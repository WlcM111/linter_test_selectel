package rules

import (
	"go/parser"
	"testing"

	"github.com/WlcM111/linter_test_selectel/config"
)

func TestSensitivePredicates(t *testing.T) {
	compiled, err := config.Default().Compile()
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !ContainsSensitiveKey(compiled, "api_key") {
		t.Fatalf("api_key must be considered sensitive")
	}
	if !ContainsSensitiveKey(compiled, "auth.token") {
		t.Fatalf("grouped key must be considered sensitive")
	}
	if ContainsSensitiveKey(compiled, "status") {
		t.Fatalf("benign key must not be considered sensitive")
	}
	if !ContainsSensitiveMessageLiteral(compiled, "token: abc") {
		t.Fatalf("explicit token leak must be considered sensitive")
	}
	if !ContainsSensitiveMessageLiteral(compiled, "api_key=secret") {
		t.Fatalf("explicit api key leak must be considered sensitive")
	}
	if ContainsSensitiveMessageLiteral(compiled, "token validated") {
		t.Fatalf("benign status must not be considered sensitive")
	}
}

func TestExprContainsSensitive(t *testing.T) {
	compiled, err := config.Default().Compile()
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	expr, err := parser.ParseExpr(`"token: " + accessToken`)
	if err != nil {
		t.Fatalf("ParseExpr() error = %v", err)
	}
	if !ExprContainsSensitive(compiled, expr) {
		t.Fatalf("expression must be considered sensitive")
	}

	expr, err = parser.ParseExpr(`"request completed" + status`)
	if err != nil {
		t.Fatalf("ParseExpr() error = %v", err)
	}
	if ExprContainsSensitive(compiled, expr) {
		t.Fatalf("expression must not be considered sensitive")
	}
}
