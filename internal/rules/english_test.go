package rules

import "testing"

func TestCheckEnglish(t *testing.T) {
	if CheckEnglish("starting server") {
		t.Fatalf("ASCII message must pass")
	}
	if !CheckEnglish("запуск сервера") {
		t.Fatalf("Cyrillic message must fail")
	}
	if !CheckEnglish("server запущен") {
		t.Fatalf("mixed message must fail")
	}
	if CheckEnglish("12345 !!!") {
		t.Fatalf("digits and punctuation must pass english rule")
	}
}
