package db

import (
	"strings"
	"unicode"
)

func toSnakeCase(s string) string {
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					result = append(result, '_')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					result = append(result, '_')
				}
			}
			result = append(result, unicode.ToLower(r))
		} else if r == ' ' || r == '-' {
			result = append(result, '_')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
