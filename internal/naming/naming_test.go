package naming

import "testing"

func TestSnake(t *testing.T) {
	for in, want := range map[string]string{
		"Users":         "users",
		"RateLimit":     "rate_limit",
		"HTTPServer":    "http_server",
		"APIKey":        "api_key",
		"LectureGroup":  "lecture_group",
		"already_snake": "already_snake",
	} {
		if got := Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPascal(t *testing.T) {
	for in, want := range map[string]string{
		"rate_limit": "RateLimit",
		"user_api":   "UserApi", // no initialisms: existing generators name structs this way
		"users":      "Users",
	} {
		if got := Pascal(in); got != want {
			t.Errorf("Pascal(%q) = %q, want %q", in, got, want)
		}
	}
}
