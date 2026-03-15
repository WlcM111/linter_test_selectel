package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	RuleLowercase = "lowercase"
	RuleEnglish   = "english"
	RuleCharset   = "charset"
	RuleSensitive = "sensitive"
)

// Config describes all user-facing linter settings.
type Config struct {
	EnabledRules             []string
	SensitiveKeywords        []string
	SensitivePatterns        []string
	SensitiveMessagePatterns []string
	AllowedPunctuation       string
	AllowTemplateSyntax      bool
}

// Compiled is an immutable runtime representation of Config.
type Compiled struct {
	Enabled                  map[string]bool
	SensitiveKeywords        map[string]struct{}
	SensitivePatterns        []*regexp.Regexp
	SensitiveMessagePatterns []*regexp.Regexp
	AllowedPunctuation       map[rune]struct{}
	AllowTemplateSyntax      bool
}

// Default returns the default linter configuration used when a setting
// is not explicitly overridden by the user.
func Default() Config {
	return Config{
		EnabledRules: []string{
			RuleLowercase,
			RuleEnglish,
			RuleCharset,
			RuleSensitive,
		},
		SensitiveKeywords: []string{
			"password",
			"passwd",
			"pwd",
			"secret",
			"api_key",
			"apikey",
			"api-key",
			"token",
			"access_token",
			"refresh_token",
			"jwt",
			"bearer",
			"credential",
			"credentials",
			"private_key",
			"private-key",
			"client_secret",
			"session",
			"cookie",
		},
		SensitivePatterns: []string{
			`(?i)pass(word|wd)?`,
			`(?i)api[._-]?key`,
			`(?i)(access|refresh)?[._-]?token`,
			`(?i)client[._-]?secret`,
			`(?i)private[._-]?key`,
			`(?i)bearer`,
			`(?i)credential`,
			`(?i)cookie`,
			`(?i)session`,
		},
		SensitiveMessagePatterns: []string{
			`(?i)\b(pass(word|wd)?|pwd|token|api[._-]?key|access[._-]?token|refresh[._-]?token|client[._-]?secret|private[._-]?key|cookie|session)\b\s*[:=]`,
			`(?i)\bbearer\b\s+`,
		},
		AllowedPunctuation:  "",
		AllowTemplateSyntax: false,
	}
}

// Compile validates configuration values, applies defaults, normalizes tokens,
// and precompiles all regular expressions into a runtime-ready representation.
func (c Config) Compile() (Compiled, error) {
	cfg := c.withDefaults()

	compiled := Compiled{
		Enabled:                  make(map[string]bool, len(cfg.EnabledRules)),
		SensitiveKeywords:        make(map[string]struct{}, len(cfg.SensitiveKeywords)),
		SensitivePatterns:        make([]*regexp.Regexp, 0, len(cfg.SensitivePatterns)),
		SensitiveMessagePatterns: make([]*regexp.Regexp, 0, len(cfg.SensitiveMessagePatterns)),
		AllowedPunctuation:       make(map[rune]struct{}, len(cfg.AllowedPunctuation)),
		AllowTemplateSyntax:      cfg.AllowTemplateSyntax,
	}

	for _, rule := range cfg.EnabledRules {
		rule = normalizeKey(rule)
		switch rule {
		case RuleLowercase, RuleEnglish, RuleCharset, RuleSensitive:
			compiled.Enabled[rule] = true
		default:
			return Compiled{}, fmt.Errorf("unknown rule %q", rule)
		}
	}

	for _, keyword := range cfg.SensitiveKeywords {
		keyword = normalizeToken(keyword)
		if keyword == "" {
			continue
		}
		compiled.SensitiveKeywords[keyword] = struct{}{}
	}

	patterns, err := compileRegexps(cfg.SensitivePatterns, "sensitive pattern")
	if err != nil {
		return Compiled{}, err
	}
	compiled.SensitivePatterns = patterns

	messagePatterns, err := compileRegexps(cfg.SensitiveMessagePatterns, "sensitive message pattern")
	if err != nil {
		return Compiled{}, err
	}
	compiled.SensitiveMessagePatterns = messagePatterns

	for _, r := range cfg.AllowedPunctuation {
		compiled.AllowedPunctuation[r] = struct{}{}
	}

	return compiled, nil
}

// compileRegexps compiles a slice of regexp patterns and returns
// a descriptive error if any pattern is invalid.
func compileRegexps(patterns []string, kind string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile %s %q: %w", kind, pattern, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// withDefaults fills missing config fields with values from Default().
func (c Config) withDefaults() Config {
	def := Default()
	out := c
	if len(out.EnabledRules) == 0 {
		out.EnabledRules = append([]string(nil), def.EnabledRules...)
	}
	if len(out.SensitiveKeywords) == 0 {
		out.SensitiveKeywords = append([]string(nil), def.SensitiveKeywords...)
	}
	if len(out.SensitivePatterns) == 0 {
		out.SensitivePatterns = append([]string(nil), def.SensitivePatterns...)
	}
	if len(out.SensitiveMessagePatterns) == 0 {
		out.SensitiveMessagePatterns = append([]string(nil), def.SensitiveMessagePatterns...)
	}
	return out
}

// normalizeKey normalizes rule and option names to a stable lookup form.
func normalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

// normalizeToken normalizes sensitive tokens so that different spellings
// such as "api_key", "api-key", and "api.key" are treated equally.
func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "", "_", "", "-", "", ".", "", "/", "", "\\", "").Replace(value)
	return value
}

// Rules returns the enabled rule names in stable sorted order.
func (c Compiled) Rules() []string {
	out := make([]string, 0, len(c.Enabled))
	for rule := range c.Enabled {
		out = append(out, rule)
	}
	sort.Strings(out)
	return out
}
