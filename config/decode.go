package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Decode converts golangci-lint plugin settings into Config.
func Decode(raw any) (Config, error) {
	cfg := Default()
	if raw == nil {
		return cfg, nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return Config{}, fmt.Errorf("plugin settings must be a map, got %T", raw)
	}

	for key, value := range m {
		switch normalizeKey(key) {
		case "enabled-rules", "rules", "enabled":
			items, err := asStringSlice(value)
			if err != nil {
				return Config{}, fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.EnabledRules = items
		case "sensitive-keywords", "keywords":
			items, err := asStringSlice(value)
			if err != nil {
				return Config{}, fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.SensitiveKeywords = items
		case "sensitive-patterns", "patterns", "custom-patterns":
			items, err := asStringSlice(value)
			if err != nil {
				return Config{}, fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.SensitivePatterns = items
		case "sensitive-message-patterns", "message-patterns":
			items, err := asStringSlice(value)
			if err != nil {
				return Config{}, fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.SensitiveMessagePatterns = items
		case "allowed-punctuation", "allowed-chars":
			text, err := asString(value)
			if err != nil {
				return Config{}, fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.AllowedPunctuation = text
		case "allow-template-syntax", "allow-templates":
			flag, err := asBool(value)
			if err != nil {
				return Config{}, fmt.Errorf("decode %q: %w", key, err)
			}
			cfg.AllowTemplateSyntax = flag
		default:
			return Config{}, fmt.Errorf("unknown config key %q", key)
		}
	}

	return cfg, nil
}

func asStringSlice(value any) ([]string, error) {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...), nil
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			text, err := asString(item)
			if err != nil {
				return nil, err
			}
			items = append(items, text)
		}
		return items, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return []string{v}, nil
	default:
		return nil, fmt.Errorf("expected string slice, got %T", value)
	}
}

func asString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("expected string-compatible value, got %T", value)
	}
}

func asBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, fmt.Errorf("expected bool, got %T", value)
	}
}
