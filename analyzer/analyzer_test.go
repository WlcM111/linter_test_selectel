package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/WlcM111/linter_test_selectel/config"
	"golang.org/x/tools/go/analysis"
)

var wantPattern = regexp.MustCompile(`//\s*want\s+"([^"]+)"`)

type diagnostic struct {
	line    int
	message string
}

func TestAnalyzerIntegration(t *testing.T) {
	a := MustNew(config.Default())
	entries, err := os.ReadDir(filepath.Join("..", "testdata", "integration"))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join("..", "testdata", "integration", entry.Name())
			runIntegrationCase(t, a, path)
		})
	}
}

func runIntegrationCase(t *testing.T, a *analysis.Analyzer, path string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}

	wants := expectedDiagnostics(fset, file)
	got := make([]diagnostic, 0)
	pass := &analysis.Pass{
		Analyzer: a,
		Fset:     fset,
		Files:    []*ast.File{file},
		Report: func(d analysis.Diagnostic) {
			got = append(got, diagnostic{line: fset.Position(d.Pos).Line, message: d.Message})
		},
	}

	if _, err := a.Run(pass); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	sort.Slice(got, func(i, j int) bool {
		if got[i].line == got[j].line {
			return got[i].message < got[j].message
		}
		return got[i].line < got[j].line
	})

	if len(got) != len(wants) {
		t.Fatalf("diagnostic count mismatch\n got: %#v\nwant: %#v", got, wants)
	}
	for idx, want := range wants {
		if got[idx].line != want.line || !strings.Contains(got[idx].message, want.message) {
			t.Fatalf("diagnostic[%d] = %#v, want %#v", idx, got[idx], want)
		}
	}
}

func expectedDiagnostics(fset *token.FileSet, file *ast.File) []diagnostic {
	wants := make([]diagnostic, 0)
	for _, cg := range file.Comments {
		for _, comment := range cg.List {
			matches := wantPattern.FindStringSubmatch(comment.Text)
			if len(matches) != 2 {
				continue
			}
			line := fset.Position(comment.Slash).Line
			wants = append(wants, diagnostic{line: line, message: matches[1]})
		}
	}
	sort.Slice(wants, func(i, j int) bool {
		if wants[i].line == wants[j].line {
			return wants[i].message < wants[j].message
		}
		return wants[i].line < wants[j].line
	})
	return wants
}
