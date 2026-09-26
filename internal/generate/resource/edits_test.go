package resource

import (
	"strings"
	"testing"
)

func TestAddValidationSchema(t *testing.T) {
	src := mustRender(t, "validation_service.go.tmpl", mustView(t, "Seminar", nil, "", false))
	out, err := addValidationSchema(src, "example.com/shop", "Topic")
	if err != nil {
		t.Fatal(err)
	}
	loose(t, out, "SeminarSchema *zog.StructSchema TopicSchema *zog.StructSchema }")
	loose(t, out, "s.SeminarSchema = models.SeminarSchema() s.TopicSchema = models.TopicSchema() return nil")

	renamed := strings.ReplaceAll(src, "(s *ValidationService)", "(v *ValidationService)")
	renamed = strings.ReplaceAll(renamed, "s.SeminarSchema =", "v.SeminarSchema =")
	out, err = addValidationSchema(renamed, "example.com/shop", "Topic")
	if err != nil {
		t.Fatal(err)
	}
	loose(t, out, "v.TopicSchema = models.TopicSchema()")

	for _, broken := range []string{
		"package services\n\ntype ValidationService struct{}\n",
		strings.Replace(src, "\"example.com/shop/app/models\"\n", "", 1),
	} {
		if _, err := addValidationSchema(broken, "example.com/shop", "Topic"); err == nil {
			t.Errorf("an edit it cannot make safely must be reported:\n%s", broken)
		}
	}
}

func TestAddSchemaTest(t *testing.T) {
	src := mustRender(t, "schemas_test.go.tmpl", mustView(t, "Seminar", nil, "", false))
	out, err := addSchemaTest(src, "example.com/shop", "Topic")
	if err != nil {
		t.Fatal(err)
	}
	loose(t, out, "models.SeminarSchema().Validate(&models.SeminarRequest{}) models.TopicSchema().Validate(&models.TopicRequest{}) }")
	if _, err := addSchemaTest("package models_test\n", "example.com/shop", "Topic"); err == nil {
		t.Error("a file without TestSchemasMatchTheirRequests must be reported")
	}
}

func TestAddRoutes(t *testing.T) {
	src := "routes:\n  /api/v1:\n    /auth:\n      /login:\n        POST: Auth.Login\n  /: SPA.Index\n"
	want := "routes:\n  /api/v1:\n    /auth:\n      /login:\n        POST: Auth.Login\n" +
		"    /courses:\n      GET: Courses.Index\n      POST: Courses.Create\n      /{id}:\n        GET: Courses.Show\n        PUT: Courses.Update\n        DELETE: Courses.Destroy\n" +
		"  /: SPA.Index\n"
	got, err := addRoutes(src, "courses", "Courses")
	if err != nil || got != want {
		t.Fatalf("addRoutes:\n got %q\nwant %q\n%v", got, want, err)
	}
}

// Review Focus 3: 4-space indentation, /api/v1 as the last block, no trailing newline.
func TestAddRoutesAdaptsToTheFile(t *testing.T) {
	src := "routes:\n    /api/v1:\n        /auth:\n            /login:\n                POST: Auth.Login"
	got, err := addRoutes(src, "lecture-groups", "LectureGroups")
	if err != nil {
		t.Fatal(err)
	}
	want := "                POST: Auth.Login\n        /lecture-groups:\n            GET: LectureGroups.Index\n            POST: LectureGroups.Create\n            /{id}:\n                GET: LectureGroups.Show\n                PUT: LectureGroups.Update\n                DELETE: LectureGroups.Destroy\n"
	if !strings.HasSuffix(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestAddRoutesRefuses(t *testing.T) {
	for name, tt := range map[string]struct{ src, want string }{
		"no /api/v1": {"routes:\n  /: SPA.Index\n", "no /api/v1: key"},
		"CRLF":       {"routes:\r\n  /api/v1:\r\n", "CRLF"},
		"existing":   {"routes:\n  /api/v1:\n    /courses:\n      GET: Courses.Index\n", "/courses already exists"},
	} {
		if _, err := addRoutes(tt.src, "courses", "Courses"); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want it to mention %q", name, err, tt.want)
		}
	}
}
