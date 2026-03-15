package rules

import "testing"

func TestCheckCharset(t *testing.T) {
	allowed := map[rune]struct{}{'-': {}}
	if CheckCharset("server started", allowed, false) {
		t.Fatalf("plain text must pass")
	}
	if !CheckCharset("server started!!!", allowed, false) {
		t.Fatalf("punctuation must fail")
	}
	if !CheckCharset("server started 🚀", allowed, false) {
		t.Fatalf("emoji must fail")
	}
	if CheckCharset("cache-hit", allowed, false) {
		t.Fatalf("allowed punctuation must pass")
	}
	if CheckCharset("request %{id}", nil, true) {
		t.Fatalf("template syntax must pass when enabled")
	}
}

func TestCharsetFix(t *testing.T) {
	got := CharsetFix("warning: something went wrong...", nil, false)
	if got != "warning something went wrong" {
		t.Fatalf("CharsetFix() = %q", got)
	}
}
