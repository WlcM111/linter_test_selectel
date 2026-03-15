package main

import (
	"github.com/WlcM111/linter_test_selectel/analyzer"
	"github.com/WlcM111/linter_test_selectel/config"
	"golang.org/x/tools/go/analysis"
)

// New is the entry point required by golangci-lint's Go plugin system.
func New(conf any) ([]*analysis.Analyzer, error) {
	cfg, err := config.Decode(conf)
	if err != nil {
		return nil, err
	}
	a, err := analyzer.New(cfg)
	if err != nil {
		return nil, err
	}
	return []*analysis.Analyzer{a}, nil
}

func main() {}
