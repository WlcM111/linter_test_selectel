package rules

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/WlcM111/linter_test_selectel/config"
)

const (
	MessageLowercase = "log message must start with a lowercase letter"
	MessageEnglish   = "log message must contain only English letters"
	MessageCharset   = "log message must not contain punctuation, special symbols, or emoji"
	MessageSensitive = "log call must not contain sensitive data"
)

// StringConstant tries to resolve an AST expression to a constant string.
// It supports plain string literals, concatenated string literals, and
// parenthesized expressions.
func StringConstant(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return text, true
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, ok := StringConstant(e.X)
		if !ok {
			return "", false
		}
		right, ok := StringConstant(e.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return StringConstant(e.X)
	}
	return "", false
}

// CheckLowercase reports whether a log message starts with an uppercase
func CheckLowercase(text string) bool {
	r, ok := firstVisibleRune(text)
	if !ok {
		return false
	}
	return unicode.IsUpper(r)
}

// CheckEnglish reports whether a log message contains non-ASCII letters.
func CheckEnglish(text string) bool {
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

// CheckCharset reports whether a log message contains forbidden punctuation,
// special symbols, or emoji. Letters, digits, spaces, explicitly allowed
// punctuation, and optional template syntax are permitted.
func CheckCharset(text string, allowed map[rune]struct{}, allowTemplateSyntax bool) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			continue
		}
		if allowTemplateSyntax && (r == '%' || r == '{' || r == '}') {
			continue
		}
		if _, ok := allowed[r]; ok {
			continue
		}
		return true
	}
	return false
}

// NormalizeToken reduces a token to a lowercase alphanumeric form so that
// different spellings such as "api_key", "api-key", and "api.key" can be
// matched consistently.
func NormalizeToken(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ContainsSensitiveKey reports whether a structured logging key or identifier
// should be considered sensitive, for example "password", "token", or "api_key".
func ContainsSensitiveKey(compiled config.Compiled, text string) bool {
	if text == "" {
		return false
	}
	normalized := NormalizeToken(text)
	for keyword := range compiled.SensitiveKeywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	for _, pattern := range compiled.SensitivePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// ContainsSensitiveMessageLiteral reports only explicit leak-like patterns in
// literal log messages, such as "token: ..." or "api_key=...". Benign status
// messages like "token validated" must not be treated as leaks.
func ContainsSensitiveMessageLiteral(compiled config.Compiled, text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, pattern := range compiled.SensitiveMessagePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// ExprContainsSensitive reports whether a non-constant expression appears to
// reference or construct sensitive data. It inspects string literals,
// identifiers, and selector names inside the expression tree.
func ExprContainsSensitive(compiled config.Compiled, expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		switch node := n.(type) {
		case *ast.BasicLit:
			if node.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(node.Value)
			if err == nil && ContainsSensitiveMessageLiteral(compiled, text) {
				found = true
				return false
			}
		case *ast.Ident:
			if ContainsSensitiveKey(compiled, node.Name) {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			if ContainsSensitiveKey(compiled, node.Sel.Name) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// LowercaseFix returns a safe automatic fix for lowercase violations by
// converting the first visible rune of the message to lowercase.
func LowercaseFix(text string) string {
	prefixLen := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			break
		}
		prefixLen += utf8.RuneLen(r)
	}
	prefix := text[:prefixLen]
	body := text[prefixLen:]
	if body == "" {
		return text
	}
	r, size := utf8.DecodeRuneInString(body)
	if r == utf8.RuneError && size == 0 {
		return text
	}
	return prefix + string(unicode.ToLower(r)) + body[size:]
}

// CharsetFix removes forbidden characters from a message while preserving
// letters, digits, spaces, explicitly allowed punctuation, and optional
// template syntax. Repeated spaces are collapsed to a single space.
func CharsetFix(text string, allowed map[rune]struct{}, allowTemplateSyntax bool) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range text {
		keep := unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r)
		if !keep {
			_, keep = allowed[r]
		}
		if allowTemplateSyntax && (r == '%' || r == '{' || r == '}') {
			keep = true
		}
		if !keep {
			continue
		}
		if unicode.IsSpace(r) {
			if prevSpace {
				continue
			}
			prevSpace = true
			b.WriteRune(' ')
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// firstVisibleRune returns the first non-space rune in text.
func firstVisibleRune(text string) (rune, bool) {
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		return r, true
	}
	return 0, false
}
