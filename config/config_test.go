package config

import "testing"

func TestDecode(t *testing.T) {
	raw := map[string]any{
		"enabled-rules":              []any{"lowercase", "sensitive"},
		"sensitive-patterns":         []any{"(?i)topsecret"},
		"sensitive-message-patterns": []any{`(?i)token\s*[:=]`},
		"sensitive-keywords":         []any{"Password", "jwt", "auth.token"},
		"allowed-punctuation":        "-_",
		"allow-template-syntax":      true,
	}

	cfg, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	compiled, err := cfg.Compile()
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !compiled.Enabled[RuleLowercase] || !compiled.Enabled[RuleSensitive] {
		t.Fatalf("expected lowercase and sensitive to be enabled: %#v", compiled.Enabled)
	}
	if compiled.Enabled[RuleEnglish] || compiled.Enabled[RuleCharset] {
		t.Fatalf("unexpected extra enabled rules: %#v", compiled.Enabled)
	}
	if _, ok := compiled.SensitiveKeywords["password"]; !ok {
		t.Fatalf("password keyword was not normalized")
	}
	if _, ok := compiled.SensitiveKeywords["authtoken"]; !ok {
		t.Fatalf("auth.token keyword was not normalized")
	}
	if len(compiled.SensitivePatterns) != 1 {
		t.Fatalf("expected one custom pattern, got %d", len(compiled.SensitivePatterns))
	}
	if len(compiled.SensitiveMessagePatterns) != 1 {
		t.Fatalf("expected one custom message pattern, got %d", len(compiled.SensitiveMessagePatterns))
	}
	if _, ok := compiled.AllowedPunctuation['-']; !ok {
		t.Fatalf("allowed punctuation was not preserved")
	}
	if !compiled.AllowTemplateSyntax {
		t.Fatalf("allow template syntax flag was lost")
	}
}

func TestDecodeUnknownKey(t *testing.T) {
	_, err := Decode(map[string]any{"unknown-key": true})
	if err == nil {
		t.Fatalf("expected error for unknown config key")
	}
}

func TestCompileUnknownRule(t *testing.T) {
	_, err := Config{EnabledRules: []string{"unknown"}}.Compile()
	if err == nil {
		t.Fatalf("expected error for unknown rule")
	}
}

func TestCompileInvalidRegexp(t *testing.T) {
	_, err := Config{SensitiveMessagePatterns: []string{"("}}.Compile()
	if err == nil {
		t.Fatalf("expected error for invalid regexp")
	}
}
