package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/WlcM111/linter_test_selectel/config"
	"github.com/WlcM111/linter_test_selectel/internal/detect"
	"github.com/WlcM111/linter_test_selectel/internal/rules"
	"golang.org/x/tools/go/analysis"
)

const (
	// Name is the public analyzer name used by golangci-lint.
	Name = "logcheck"

	// Doc describes the analyzer purpose in a short human-readable form.
	Doc = "check slog and zap log messages for style, language, and sensitive data issues"

	// URL points to the project repository and is used as analyzer metadata.
	URL = "https://github.com/WlcM111/linter_test_selectel"
)

// Analyzer is the default configured analyzer instance used by plugin entrypoints
// and tests that do not need custom settings.
var Analyzer = MustNew(config.Default())

// MustNew builds an analyzer from configuration and panics if the configuration
// is invalid. It is intended for package-level initialization.
func MustNew(cfg config.Config) *analysis.Analyzer {
	a, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return a
}

// New builds a go/analysis analyzer that validates slog and zap log messages
// according to the configured rules.
func New(cfg config.Config) (*analysis.Analyzer, error) {
	compiled, err := cfg.Compile()
	if err != nil {
		return nil, err
	}

	return &analysis.Analyzer{
		Name:             Name,
		Doc:              Doc,
		URL:              URL,
		RunDespiteErrors: true,
		Run: func(pass *analysis.Pass) (interface{}, error) {
			for _, inv := range detect.Collect(pass) {
				checkInvocation(pass, compiled, inv)
			}
			return nil, nil
		},
	}, nil
}

// checkInvocation applies all enabled rules to a single detected log call.
// Style checks are applied to constant messages, while sensitive-data checks
// also inspect dynamic expressions and structured fields.
func checkInvocation(pass *analysis.Pass, compiled config.Compiled, inv detect.Invocation) {
	text, constant := rules.StringConstant(inv.MessageExpr)

	if constant && compiled.Enabled[config.RuleLowercase] && rules.CheckLowercase(text) {
		report(pass, inv.MessageExpr, config.RuleLowercase, rules.MessageLowercase, literalFix(inv.MessageExpr, rules.LowercaseFix(text)))
	}
	if constant && compiled.Enabled[config.RuleEnglish] && rules.CheckEnglish(text) {
		report(pass, inv.MessageExpr, config.RuleEnglish, rules.MessageEnglish, nil)
	}
	if constant && compiled.Enabled[config.RuleCharset] && rules.CheckCharset(text, compiled.AllowedPunctuation, compiled.AllowTemplateSyntax) {
		fixed := rules.CharsetFix(text, compiled.AllowedPunctuation, compiled.AllowTemplateSyntax)
		report(pass, inv.MessageExpr, config.RuleCharset, rules.MessageCharset, literalFix(inv.MessageExpr, fixed))
	}
	if !compiled.Enabled[config.RuleSensitive] {
		return
	}

	// Literal messages are checked only for explicit leak-like patterns such as
	// "token: ..." or "api_key=...". Messages as "token validated"
	// must not be reported.
	if constant && rules.ContainsSensitiveMessageLiteral(compiled, text) {
		report(pass, inv.MessageExpr, config.RuleSensitive, rules.MessageSensitive, nil)
	}

	// Dynamic message expressions are inspected separately because sensitive
	// data can be introduced by concatenation or identifiers.
	if !constant && rules.ExprContainsSensitive(compiled, inv.MessageExpr) {
		report(pass, inv.Call, config.RuleSensitive, rules.MessageSensitive, nil)
	}

	// Structured fields are checked by key and value independently so that
	// sensitive keys such as "password" or "api_key" are reported precisely.
	for _, field := range inv.StructuredArgs {
		if rules.ContainsSensitiveKey(compiled, field.Key) {
			node := ast.Node(inv.Call)
			if field.KeyExpr != nil {
				node = field.KeyExpr
			}
			report(pass, node, config.RuleSensitive, rules.MessageSensitive, nil)
			continue
		}
		if field.ValueExpr != nil && rules.ExprContainsSensitive(compiled, field.ValueExpr) {
			node := ast.Node(inv.Call)
			if field.KeyExpr != nil {
				node = field.KeyExpr
			}
			report(pass, node, config.RuleSensitive, rules.MessageSensitive, nil)
		}
	}
}

// report emits a diagnostic for the specified AST node and attaches an optional
// suggested fix when one is available.
func report(pass *analysis.Pass, node ast.Node, category, message string, fix *analysis.SuggestedFix) {
	diag := analysis.Diagnostic{Pos: node.Pos(), End: node.End(), Category: category, Message: message}
	if fix != nil {
		diag.SuggestedFixes = []analysis.SuggestedFix{*fix}
	}
	pass.Report(diag)
}

// literalFix creates a SuggestedFix for plain string literals when the
// replacement is safe to apply automatically.
func literalFix(expr ast.Expr, replacement string) *analysis.SuggestedFix {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING || strings.TrimSpace(replacement) == "" {
		return nil
	}
	quoted := strconv.Quote(replacement)
	if lit.Value == quoted {
		return nil
	}
	return &analysis.SuggestedFix{
		Message: fmt.Sprintf("replace log message with %s", quoted),
		TextEdits: []analysis.TextEdit{{
			Pos:     lit.Pos(),
			End:     lit.End(),
			NewText: []byte(quoted),
		}},
	}
}
