package register

import "golang.org/x/tools/go/analysis"

// Constructor builds a linter plugin instance from arbitrary settings.
type Constructor func(any) (LinterPlugin, error)

// LinterPlugin is the interface used by golangci-lint's module plugin system.
// This local copy mirrors only the surface needed by this project so the
// repository remains self-contained in an offline environment.
type LinterPlugin interface {
	BuildAnalyzers() ([]*analysis.Analyzer, error)
	GetLoadMode() string
}

const (
	LoadModeSyntax   = "syntax"
	LoadModeTypes    = "types"
	LoadModeWholePkg = "whole-program"
)

var registry = map[string]Constructor{}

// Plugin registers a linter constructor under a stable name.
func Plugin(name string, constructor Constructor) {
	registry[name] = constructor
}

// Lookup returns a previously registered constructor.
func Lookup(name string) (Constructor, bool) {
	c, ok := registry[name]
	return c, ok
}
