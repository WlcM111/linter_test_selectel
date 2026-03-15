package module

import (
	"github.com/WlcM111/linter_test_selectel/analyzer"
	"github.com/WlcM111/linter_test_selectel/config"
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin(analyzer.Name, New)
}

// New is the entry point for golangci-lint's module plugin system.
// It decodes plugin settings and returns a configured plugin instance.
func New(settings any) (register.LinterPlugin, error) {
	cfg, err := config.Decode(settings)
	if err != nil {
		return nil, err
	}
	return plugin{cfg: cfg}, nil
}

// plugin is a lightweight adapter that exposes the logcheck analyzer through
// golangci-lint's module plugin interface.
type plugin struct {
	cfg config.Config
}

var _ register.LinterPlugin = plugin{}

// BuildAnalyzers constructs the configured analyzers exported by this plugin.
func (p plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	a, err := analyzer.New(p.cfg)
	if err != nil {
		return nil, err
	}
	return []*analysis.Analyzer{a}, nil
}

// GetLoadMode reports the minimum package loading mode required by the plugin.
// Syntax mode is sufficient because logcheck works on AST structure and does
// not require full type information.
func (plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}
