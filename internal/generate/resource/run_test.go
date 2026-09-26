package resource

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

func run(t *testing.T, o Options) error {
	t.Helper()
	o.Module, o.Now = "example.com/shop", testNow
	if o.Out == nil {
		o.Out = io.Discard
	}
	return Run(o)
}

func TestRunGeneratesAFlatResource(t *testing.T) {
	copyFixture(t)
	var out bytes.Buffer
	if err := run(t, Options{Name: "Course", Fields: []string{"name:string", "division:ref"}, Out: &out}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"app/models/course.go", "app/services/courses_service.go", "app/controllers/courses_controller.go",
		"app/services/database_service.go", "app/services/validation_service.go", "app/controllers/helpers.go",
		"app/models/validation.go", "app/models/schemas_test.go",
		"app/controllers/courses_controller_test.go", "app/controllers/seed_course_test.go",
		"app/controllers/seed_division_test.go", "app/controllers/setup_test.go", "app/controllers/harness_test.go",
		"db/migrations/20260925115959_setup.sql", "db/migrations/20260925120000_create_courses.sql",
	} {
		if _, err := os.Stat(filepath.FromSlash(path)); err != nil {
			t.Errorf("missing %s", path)
		}
	}
	services := readFile(t, "config/components/services.go")
	dbAt, validationAt, coursesAt := strings.Index(services, "DatabaseService"), strings.Index(services, "ValidationService"), strings.Index(services, "CoursesService")
	if !(0 < dbAt && dbAt < validationAt && validationAt < coursesAt) {
		t.Errorf("DatabaseService, ValidationService and CoursesService must register in that order:\n%s", services)
	}
	loose(t, readFile(t, "config/components/controllers.go"), "&controllers.CoursesController{},")
	loose(t, readFile(t, "config/routes.yaml"), "/courses: GET: Courses.Index")
	if strings.Contains(out.String(), "Add these by hand") {
		t.Errorf("no edit should have failed:\n%s", out.String())
	}
}

func TestRunAddsAChildToAGeneratedParent(t *testing.T) {
	copyFixture(t)
	if err := run(t, Options{Name: "Course", Fields: []string{"name:string"}}); err != nil {
		t.Fatal(err)
	}
	if err := run(t, Options{Name: "Outcome", Fields: []string{"name:string"}, Parent: "Course"}); err != nil {
		t.Fatal(err)
	}
	loose(t, readFile(t, "app/services/validation_service.go"), "s.OutcomeSchema = models.OutcomeSchema()")
	loose(t, readFile(t, "app/models/schemas_test.go"), "models.OutcomeSchema().Validate(&models.OutcomeRequest{})")
	loose(t, readFile(t, "app/controllers/seed_outcome_test.go"), "CourseID: seedCourse(t, userID).ID,")
	loose(t, readFile(t, "config/routes.yaml"), "/outcomes: GET: Outcomes.Index")
}

// Review Focus 5: a refused run changes nothing.
func TestRunRefusesWithoutWriting(t *testing.T) {
	copyFixture(t)
	writeFile(t, "app/controllers/courses_controller.go", "package controllers\n")
	before := snapshot(t, ".")
	err := run(t, Options{Name: "Course"})
	if err == nil || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, ".")) {
		t.Error("a refused run must not change any file")
	}

	copyFixture(t)
	if err := run(t, Options{Name: "Course"}); err != nil {
		t.Fatal(err)
	}
	before = snapshot(t, ".")
	if err := run(t, Options{Name: "Course"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a second run must be refused: %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, ".")) {
		t.Error("the second run must not change any file")
	}
}

func TestRunPrintsTheEditsItCannotMake(t *testing.T) {
	copyFixture(t)
	writeFile(t, "config/routes.yaml", "routes:\n  /: SPA.Index\n")
	var out bytes.Buffer
	if err := run(t, Options{Name: "Course", Out: &out}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Add these by hand", "/courses:", "No integration tests were generated"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if readFile(t, "config/routes.yaml") != "routes:\n  /: SPA.Index\n" {
		t.Error("a failed edit must leave the file unchanged")
	}
}
