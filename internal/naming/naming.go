// Package naming converts identifiers between the case styles the generators emit.
package naming

import (
	"strings"
	"unicode"
)

// Snake converts PascalCase or camelCase to snake_case, keeping runs of capitals together:
// "HTTPServer" → "http_server", "LectureGroup" → "lecture_group".
func Snake(s string) string {
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
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// Pascal converts snake_case to PascalCase without initialisms: "user_api" → "UserApi".
func Pascal(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// initialisms are the words Go spells in capitals inside identifiers.
var initialisms = map[string]string{
	"id": "ID", "url": "URL", "api": "API", "ip": "IP", "uuid": "UUID",
	"html": "HTML", "http": "HTTP", "json": "JSON",
}

// GoField converts snake_case to an exported Go identifier with initialisms:
// "division_id" → "DivisionID", "url_path" → "URLPath".
func GoField(snake string) string {
	var b strings.Builder
	for _, part := range strings.Split(snake, "_") {
		if part == "" {
			continue
		}
		if up, ok := initialisms[part]; ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// Camel converts snake_case to the lowerCamel JSON member style the conventions use, with no
// initialisms: "division_id" → "divisionId".
func Camel(snake string) string {
	var b strings.Builder
	for _, part := range strings.Split(snake, "_") {
		if part == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString(part)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// Var is the local variable name for a PascalCase type name: "LectureGroups" → "lectureGroups".
func Var(pascal string) string { return Camel(Snake(pascal)) }

// Kebab converts snake_case to the kebab-case of a route segment.
func Kebab(snake string) string { return strings.ReplaceAll(snake, "_", "-") }

// Plural applies the regular English rules; irregular nouns take --plural instead.
func Plural(word string) string {
	lower := strings.ToLower(word)
	for _, suffix := range []string{"s", "x", "z", "ch", "sh"} {
		if strings.HasSuffix(lower, suffix) {
			return word + "es"
		}
	}
	if n := len(lower); n > 1 && lower[n-1] == 'y' && !strings.ContainsRune("aeiou", rune(lower[n-2])) {
		return word[:len(word)-1] + "ies"
	}
	return word + "s"
}

// Label is a type name as words for messages: "LectureGroup" → "Lecture group".
func Label(pascal string) string {
	words := strings.Split(Snake(pascal), "_")
	for i, w := range words {
		if up, ok := initialisms[w]; ok {
			words[i] = up
		} else if i == 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// LabelLower is Label mid-sentence: "LectureGroup" → "lecture group".
func LabelLower(pascal string) string {
	words := strings.Split(Snake(pascal), "_")
	for i, w := range words {
		if up, ok := initialisms[w]; ok {
			words[i] = up
		}
	}
	return strings.Join(words, " ")
}

var reserved = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
	"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true,
	"var": true,
	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true,
	"complex128": true, "error": true, "float32": true, "float64": true, "int": true, "int8": true,
	"int16": true, "int32": true, "int64": true, "rune": true, "string": true, "uint": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true, "true": true,
	"false": true, "iota": true, "nil": true, "append": true, "cap": true, "clear": true,
	"close": true, "complex": true, "copy": true, "delete": true, "imag": true, "len": true,
	"make": true, "max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
}

// Reserved reports whether ident is a Go keyword or predeclared identifier, which a generated
// variable must not use.
func Reserved(ident string) bool { return reserved[ident] }
