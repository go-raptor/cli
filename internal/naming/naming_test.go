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

func TestGoField(t *testing.T) {
	for in, want := range map[string]string{
		"division_id":   "DivisionID",
		"url_path":      "URLPath",
		"lecture_hours": "LectureHours",
		"starts_at":     "StartsAt",
		"api_key":       "APIKey",
	} {
		if got := GoField(in); got != want {
			t.Errorf("GoField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamel(t *testing.T) {
	for in, want := range map[string]string{
		"division_id":      "divisionId",
		"lecture_group_id": "lectureGroupId",
		"name":             "name",
		"url_path":         "urlPath",
	} {
		if got := Camel(in); got != want {
			t.Errorf("Camel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVarKebabPlural(t *testing.T) {
	for in, want := range map[string]string{"Course": "course", "LectureGroups": "lectureGroups", "APIKey": "apiKey"} {
		if got := Var(in); got != want {
			t.Errorf("Var(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Kebab("lecture_groups"); got != "lecture-groups" {
		t.Errorf("Kebab = %q", got)
	}
	for in, want := range map[string]string{
		"Course": "Courses", "Status": "Statuses", "Box": "Boxes", "Match": "Matches",
		"Category": "Categories", "Day": "Days", "LectureGroup": "LectureGroups",
	} {
		if got := Plural(in); got != want {
			t.Errorf("Plural(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLabels(t *testing.T) {
	for in, want := range map[string][2]string{
		"Course":       {"Course", "course"},
		"LectureGroup": {"Lecture group", "lecture group"},
		"APIKey":       {"API key", "API key"},
	} {
		if got := Label(in); got != want[0] {
			t.Errorf("Label(%q) = %q, want %q", in, got, want[0])
		}
		if got := LabelLower(in); got != want[1] {
			t.Errorf("LabelLower(%q) = %q, want %q", in, got, want[1])
		}
	}
}

func TestReserved(t *testing.T) {
	for _, ident := range []string{"type", "range", "string", "len", "error", "new"} {
		if !Reserved(ident) {
			t.Errorf("Reserved(%q) = false", ident)
		}
	}
	if Reserved("course") {
		t.Error(`Reserved("course") = true`)
	}
}
