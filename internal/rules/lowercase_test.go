package rules

import "testing"

func TestCheckLowercase(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "lowercase", text: "starting server", want: false},
		{name: "uppercase", text: "Starting server", want: true},
		{name: "space then uppercase", text: "  Failed request", want: true},
		{name: "spaces only", text: "   ", want: false},
		{name: "digit prefix", text: "8080 started", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CheckLowercase(tc.text); got != tc.want {
				t.Fatalf("CheckLowercase(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestLowercaseFix(t *testing.T) {
	got := LowercaseFix("  Starting server")
	if got != "  starting server" {
		t.Fatalf("LowercaseFix() = %q", got)
	}
	got = LowercaseFix("already fine")
	if got != "already fine" {
		t.Fatalf("LowercaseFix() must preserve lowercase text, got %q", got)
	}
}
