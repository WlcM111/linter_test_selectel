package detect

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

type LoggerKind string

const (
	LoggerUnknown    LoggerKind = ""
	LoggerSlog       LoggerKind = "slog"
	LoggerZap        LoggerKind = "zap"
	LoggerZapSugared LoggerKind = "zap-sugared"
)

// Symbol describes a resolved variable, field, or expression that may represent
// either a supported logger or a user-defined struct.
type Symbol struct {
	Logger LoggerKind
	Struct string
}

func (s Symbol) IsLogger() bool { return s.Logger != LoggerUnknown }

type StructuredArg struct {
	Key       string
	KeyExpr   ast.Expr
	ValueExpr ast.Expr
}

type Invocation struct {
	LoggerKind     LoggerKind
	Method         string
	Call           *ast.CallExpr
	MessageExpr    ast.Expr
	StructuredArgs []StructuredArg
	File           *ast.File
}

type packageEnv struct {
	structs map[string]map[string]Symbol
	globals map[string]Symbol
}

type fileEnv struct {
	pkg     *packageEnv
	imports map[string]string
}

type funcEnv struct {
	file   *fileEnv
	locals map[string]Symbol
}

// Collect walks all files in the package and returns supported slog and zap
// log invocations discovered in function bodies.
func Collect(pass *analysis.Pass) []Invocation {
	pkgEnv := buildPackageEnv(pass.Files)
	invocations := make([]Invocation, 0, 16)
	for _, file := range pass.Files {
		fenv := &fileEnv{pkg: pkgEnv, imports: buildImportMap(file)}
		for _, decl := range file.Decls {
			if d, ok := decl.(*ast.FuncDecl); ok {
				f := newFuncEnv(fenv, d.Type, d.Recv)
				collectBlock(f, d.Body, file, &invocations)
			}
		}
	}
	return invocations
}

// buildPackageEnv collects package-level struct field information and global
// variables so logger-typed fields and globals can be resolved later.
func buildPackageEnv(files []*ast.File) *packageEnv {
	env := &packageEnv{structs: map[string]map[string]Symbol{}, globals: map[string]Symbol{}}
	for _, file := range files {
		imports := buildImportMap(file)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			switch gen.Tok {
			case token.TYPE:
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					fields := map[string]Symbol{}
					for _, field := range st.Fields.List {
						sym := symbolFromTypeExpr(field.Type, imports, env)
						if len(field.Names) == 0 {
							if ident, ok := unwrapIdent(field.Type); ok {
								fields[ident] = sym
							}
							continue
						}
						for _, name := range field.Names {
							fields[name.Name] = sym
						}
					}
					env.structs[ts.Name.Name] = fields
				}
			case token.VAR:
				for _, spec := range gen.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						assignSymbols(env.globals, vs.Names, vs.Values, vs.Type, imports, env)
					}
				}
			}
		}
	}
	return env
}

// newFuncEnv initializes a function environment with receiver and parameter symbols.
func newFuncEnv(fenv *fileEnv, ft *ast.FuncType, recv *ast.FieldList) *funcEnv {
	locals := map[string]Symbol{}
	if recv != nil {
		for _, field := range recv.List {
			sym := symbolFromTypeExpr(field.Type, fenv.imports, fenv.pkg)
			for _, name := range field.Names {
				locals[name.Name] = sym
			}
		}
	}
	if ft != nil && ft.Params != nil {
		for _, field := range ft.Params.List {
			sym := symbolFromTypeExpr(field.Type, fenv.imports, fenv.pkg)
			for _, name := range field.Names {
				locals[name.Name] = sym
			}
		}
	}
	return &funcEnv{file: fenv, locals: locals}
}

// collectBlock traverses a function body, tracks local assignments, and
// records all supported log invocations.
func collectBlock(fenv *funcEnv, body *ast.BlockStmt, file *ast.File, out *[]Invocation) {
	if body == nil {
		return
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncLit:
			child := newFuncEnv(fenv.file, node.Type, nil)
			collectBlock(child, node.Body, file, out)
			return false
		case *ast.AssignStmt:
			consumeAssign(fenv, node)
		case *ast.DeclStmt:
			if gen, ok := node.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				consumeVarDecl(fenv, gen)
			}
		case *ast.CallExpr:
			if inv, ok := detectInvocation(fenv, file, node); ok {
				*out = append(*out, inv)
			}
		}
		return true
	})
}

// consumeVarDecl updates the function environment with symbols introduced by local var declarations.
func consumeVarDecl(fenv *funcEnv, gen *ast.GenDecl) {
	for _, spec := range gen.Specs {
		if vs, ok := spec.(*ast.ValueSpec); ok {
			assignSymbols(fenv.locals, vs.Names, vs.Values, vs.Type, fenv.file.imports, fenv.file.pkg)
		}
	}
}

// consumeAssign updates local symbol bindings after assignments so chained logger
// expressions such as logger := zap.L().Named("worker") can be tracked.
func consumeAssign(fenv *funcEnv, stmt *ast.AssignStmt) {
	for idx, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		var rhs ast.Expr
		if idx < len(stmt.Rhs) {
			rhs = stmt.Rhs[idx]
		} else if len(stmt.Rhs) == 1 {
			rhs = stmt.Rhs[0]
		}
		sym := symbolFromExpr(rhs, fenv)
		if sym == (Symbol{}) {
			if existing, ok := fenv.locals[ident.Name]; ok {
				sym = existing
			}
		}
		if sym != (Symbol{}) {
			fenv.locals[ident.Name] = sym
		}
	}
}

// assignSymbols assigns inferred symbols to identifiers using either an explicit
// type or the right-hand side expression.
func assignSymbols(dst map[string]Symbol, names []*ast.Ident, values []ast.Expr, typ ast.Expr, imports map[string]string, pkg *packageEnv) {
	typed := symbolFromTypeExpr(typ, imports, pkg)
	for idx, name := range names {
		if name == nil || name.Name == "_" {
			continue
		}
		sym := typed
		if idx < len(values) {
			if fromValue := symbolFromExpr(values[idx], &funcEnv{file: &fileEnv{pkg: pkg, imports: imports}, locals: dst}); fromValue != (Symbol{}) {
				sym = fromValue
			}
		} else if len(values) == 1 {
			if fromValue := symbolFromExpr(values[0], &funcEnv{file: &fileEnv{pkg: pkg, imports: imports}, locals: dst}); fromValue != (Symbol{}) {
				sym = fromValue
			}
		}
		if sym != (Symbol{}) {
			dst[name.Name] = sym
		}
	}
}

// detectInvocation recognizes a supported slog or zap log call and extracts
// its message expression and structured arguments.
func detectInvocation(fenv *funcEnv, file *ast.File, call *ast.CallExpr) (Invocation, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return Invocation{}, false
	}
	if ident, ok := sel.X.(*ast.Ident); ok && fenv.file.imports[ident.Name] == "log/slog" {
		if msgIdx, attrs, ok := slogMethod(sel.Sel.Name); ok {
			msg, fields := extractSlogArgs(call.Args, msgIdx, attrs, fenv.file.imports)
			if msg == nil {
				return Invocation{}, false
			}
			return Invocation{LoggerKind: LoggerSlog, Method: sel.Sel.Name, Call: call, MessageExpr: msg, StructuredArgs: fields, File: file}, true
		}
	}
	receiver := symbolFromExpr(sel.X, fenv)
	if !receiver.IsLogger() {
		return Invocation{}, false
	}
	switch receiver.Logger {
	case LoggerSlog:
		if msgIdx, attrs, ok := slogMethod(sel.Sel.Name); ok {
			msg, fields := extractSlogArgs(call.Args, msgIdx, attrs, fenv.file.imports)
			if msg == nil {
				return Invocation{}, false
			}
			return Invocation{LoggerKind: LoggerSlog, Method: sel.Sel.Name, Call: call, MessageExpr: msg, StructuredArgs: fields, File: file}, true
		}
	case LoggerZap:
		if msgIdx, ok := zapMethod(sel.Sel.Name); ok {
			msg, fields := extractZapArgs(call.Args, msgIdx)
			if msg == nil {
				return Invocation{}, false
			}
			return Invocation{LoggerKind: LoggerZap, Method: sel.Sel.Name, Call: call, MessageExpr: msg, StructuredArgs: fields, File: file}, true
		}
	case LoggerZapSugared:
		if msgIdx, pairs, ok := zapSugaredMethod(sel.Sel.Name); ok {
			msg, fields := extractSugaredArgs(call.Args, msgIdx, pairs)
			if msg == nil {
				return Invocation{}, false
			}
			return Invocation{LoggerKind: LoggerZapSugared, Method: sel.Sel.Name, Call: call, MessageExpr: msg, StructuredArgs: fields, File: file}, true
		}
	}
	return Invocation{}, false
}

// buildImportMap returns a local-name-to-import-path mapping for a file.
func buildImportMap(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := importName(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		imports[name] = path
	}
	return imports
}

// importName returns the default local import name derived from an import path.
func importName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// symbolFromTypeExpr resolves logger-related information from a type expression.
func symbolFromTypeExpr(expr ast.Expr, imports map[string]string, pkg *packageEnv) Symbol {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return symbolFromTypeExpr(t.X, imports, pkg)
	case *ast.SelectorExpr:
		ident, ok := t.X.(*ast.Ident)
		if !ok {
			return Symbol{}
		}
		switch imports[ident.Name] {
		case "log/slog":
			if t.Sel.Name == "Logger" {
				return Symbol{Logger: LoggerSlog}
			}
		case "go.uber.org/zap":
			switch t.Sel.Name {
			case "Logger":
				return Symbol{Logger: LoggerZap}
			case "SugaredLogger":
				return Symbol{Logger: LoggerZapSugared}
			}
		}
	case *ast.Ident:
		if pkg != nil {
			if _, ok := pkg.structs[t.Name]; ok {
				return Symbol{Struct: t.Name}
			}
		}
	}
	return Symbol{}
}

// symbolFromExpr resolves logger-related information from arbitrary expressions,
// including local variables, struct fields, constructor calls, and chaining
// methods such as With, Named, Sugar, or WithGroup.
func symbolFromExpr(expr ast.Expr, fenv *funcEnv) Symbol {
	switch e := expr.(type) {
	case nil:
		return Symbol{}
	case *ast.ParenExpr:
		return symbolFromExpr(e.X, fenv)
	case *ast.StarExpr:
		return symbolFromExpr(e.X, fenv)
	case *ast.UnaryExpr:
		return symbolFromExpr(e.X, fenv)
	case *ast.Ident:
		if sym, ok := fenv.locals[e.Name]; ok {
			return sym
		}
		if sym, ok := fenv.file.pkg.globals[e.Name]; ok {
			return sym
		}
	case *ast.SelectorExpr:
		base := symbolFromExpr(e.X, fenv)
		if base.Struct != "" {
			if field, ok := fenv.file.pkg.structs[base.Struct][e.Sel.Name]; ok {
				return field
			}
		}
		if base.Logger == LoggerZap && e.Sel.Name == "Sugar" {
			return Symbol{Logger: LoggerZapSugared}
		}
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				switch fenv.file.imports[ident.Name] {
				case "log/slog":
					switch sel.Sel.Name {
					case "Default", "New", "With":
						return Symbol{Logger: LoggerSlog}
					}
				case "go.uber.org/zap":
					switch sel.Sel.Name {
					case "L", "New", "NewProduction", "NewDevelopment", "NewExample", "NewNop", "Must":
						return Symbol{Logger: LoggerZap}
					case "S":
						return Symbol{Logger: LoggerZapSugared}
					}
				}
			}
			receiver := symbolFromExpr(sel.X, fenv)
			switch receiver.Logger {
			case LoggerSlog:
				switch sel.Sel.Name {
				case "With", "WithGroup":
					return Symbol{Logger: LoggerSlog}
				}
			case LoggerZap:
				switch sel.Sel.Name {
				case "With", "Named":
					return Symbol{Logger: LoggerZap}
				case "Sugar":
					return Symbol{Logger: LoggerZapSugared}
				}
			case LoggerZapSugared:
				if sel.Sel.Name == "Desugar" {
					return Symbol{Logger: LoggerZap}
				}
			}
		}
	}
	return Symbol{}
}

// unwrapIdent extracts an identifier name from plain or pointer types.
func unwrapIdent(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, true
	case *ast.StarExpr:
		return unwrapIdent(t.X)
	}
	return "", false
}

// slogMethod returns the message argument index, whether the call accepts only
// slog.Attr-style arguments, and whether the method is supported.
func slogMethod(name string) (int, bool, bool) {
	switch name {
	case "Debug", "Info", "Warn", "Error":
		return 0, false, true
	case "DebugContext", "InfoContext", "WarnContext", "ErrorContext":
		return 1, false, true
	case "Log":
		return 2, false, true
	case "LogAttrs":
		return 2, true, true
	default:
		return 0, false, false
	}
}

// zapMethod returns the message argument index for supported zap.Logger methods.
func zapMethod(name string) (int, bool) {
	switch name {
	case "Debug", "Info", "Warn", "Error", "DPanic", "Panic", "Fatal":
		return 0, true
	default:
		return 0, false
	}
}

// zapSugaredMethod returns the message argument index, whether the method uses
// key/value pairs, and whether the method is supported.
func zapSugaredMethod(name string) (int, bool, bool) {
	switch name {
	case "Debugw", "Infow", "Warnw", "Errorw", "DPanicw", "Panicw", "Fatalw":
		return 0, true, true
	case "Debugf", "Infof", "Warnf", "Errorf", "DPanicf", "Panicf", "Fatalf":
		return 0, false, true
	case "Debug", "Info", "Warn", "Error", "DPanic", "Panic", "Fatal":
		return 0, false, true
	default:
		return 0, false, false
	}
}

// extractSlogArgs extracts the message expression together with structured
// fields from slog calls using either mixed arguments or pure slog.Attr style.
func extractSlogArgs(args []ast.Expr, msgIdx int, attrs bool, imports map[string]string) (ast.Expr, []StructuredArg) {
	if msgIdx >= len(args) {
		return nil, nil
	}
	msg := args[msgIdx]
	if attrs {
		return msg, extractSlogAttrs(args[msgIdx+1:], imports)
	}
	return msg, extractSlogMixedArgs(args[msgIdx+1:], imports)
}

// extractSlogMixedArgs extracts structured fields from slog methods that accept
// either alternating key/value pairs or slog.Attr constructor expressions.
func extractSlogMixedArgs(args []ast.Expr, imports map[string]string) []StructuredArg {
	fields := make([]StructuredArg, 0)
	for i := 0; i < len(args); {
		if attrs := structuredFromSlogExpr(args[i], imports, ""); len(attrs) > 0 {
			fields = append(fields, attrs...)
			i++
			continue
		}
		if i+1 < len(args) {
			fields = append(fields, StructuredArg{Key: stringLiteral(args[i]), KeyExpr: args[i], ValueExpr: args[i+1]})
			i += 2
			continue
		}
		i++
	}
	return fields
}

// extractSlogAttrs extracts structured fields from slog methods that accept
// only slog.Attr-style arguments.
func extractSlogAttrs(args []ast.Expr, imports map[string]string) []StructuredArg {
	fields := make([]StructuredArg, 0)
	for _, arg := range args {
		fields = append(fields, structuredFromSlogExpr(arg, imports, "")...)
	}
	return fields
}

// structuredFromSlogExpr converts slog attribute constructor expressions such as
// slog.String("password", value) or slog.Group("auth", ...) into flattened
// structured arguments. Nested groups are flattened using dot notation.
func structuredFromSlogExpr(expr ast.Expr, imports map[string]string, prefix string) []StructuredArg {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || imports[ident.Name] != "log/slog" || len(call.Args) == 0 {
		return nil
	}
	key := stringLiteral(call.Args[0])
	if key == "" {
		return nil
	}
	fullKey := key
	if prefix != "" {
		fullKey = prefix + "." + key
	}
	if sel.Sel.Name == "Group" {
		out := make([]StructuredArg, 0)
		for _, child := range call.Args[1:] {
			out = append(out, structuredFromSlogExpr(child, imports, fullKey)...)
		}
		return out
	}
	var value ast.Expr
	if len(call.Args) > 1 {
		value = call.Args[1]
	}
	return []StructuredArg{{Key: fullKey, KeyExpr: call.Args[0], ValueExpr: value}}
}

// extractZapArgs extracts the message expression and structured zap.Field values
// from zap.Logger methods.
func extractZapArgs(args []ast.Expr, msgIdx int) (ast.Expr, []StructuredArg) {
	if msgIdx >= len(args) {
		return nil, nil
	}
	msg := args[msgIdx]
	fields := make([]StructuredArg, 0)
	for _, arg := range args[msgIdx+1:] {
		fields = append(fields, structuredFromZapField(arg))
	}
	return msg, fields
}

// extractSugaredArgs extracts the message expression and optional key/value
// pairs from zap.SugaredLogger methods.
func extractSugaredArgs(args []ast.Expr, msgIdx int, pairs bool) (ast.Expr, []StructuredArg) {
	if msgIdx >= len(args) {
		return nil, nil
	}
	msg := args[msgIdx]
	if !pairs {
		return msg, nil
	}
	fields := make([]StructuredArg, 0)
	for i := msgIdx + 1; i+1 < len(args); i += 2 {
		fields = append(fields, StructuredArg{Key: stringLiteral(args[i]), KeyExpr: args[i], ValueExpr: args[i+1]})
	}
	return msg, fields
}

// structuredFromZapField converts a zap.Field constructor call into a structured
// argument representation used by later sensitive-data checks.
func structuredFromZapField(expr ast.Expr) StructuredArg {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return StructuredArg{ValueExpr: expr}
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return StructuredArg{ValueExpr: expr}
	}
	if sel.Sel.Name == "Error" {
		return StructuredArg{Key: "error", ValueExpr: call.Args[0]}
	}
	key := stringLiteral(call.Args[0])
	var value ast.Expr
	if len(call.Args) > 1 {
		value = call.Args[1]
	}
	return StructuredArg{Key: key, KeyExpr: call.Args[0], ValueExpr: value}
}

// stringLiteral returns the unquoted value of a string literal or an empty string
// if the expression is not a valid string literal.
func stringLiteral(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	text, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return text
}

// FirstVisibleRune returns the first non-space rune in text.
func FirstVisibleRune(text string) (rune, bool) {
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		return r, true
	}
	return 0, false
}
