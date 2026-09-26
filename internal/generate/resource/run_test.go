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

// Finding I-2: registration is judged by the exact entry, and the output says what happened.
func TestRunRegistersComponents(t *testing.T) {
	copyFixture(t)
	replaceInFile(t, "config/components/services.go", "&services.AuthService{},", "&services.AuthService{},\n\t\t&services.LectureNotesService{},")
	writeFile(t, "config/components/controllers.go", "package components\n\nimport (\n\t\"example.com/shop/app/controllers\"\n\t\"github.com/go-raptor/raptor/v4\"\n)\n\nfunc Controllers() raptor.Controllers {\n\treturn raptor.Controllers{&controllers.AuthController{}, &controllers.NotesController{}}\n}\n")
	var out bytes.Buffer
	if err := run(t, Options{Name: "Note", Out: &out}); err != nil {
		t.Fatal(err)
	}
	loose(t, readFile(t, "config/components/services.go"), "&services.LectureNotesService{}, &services.DatabaseService{}, &services.ValidationService{}, &services.NotesService{}, }")
	if got := readFile(t, "config/components/controllers.go"); strings.Count(got, "NotesController") != 1 {
		t.Errorf("an already registered controller must not be added again:\n%s", got)
	}
	for _, want := range []string{"Updated config/components/services.go", "config/components/controllers.go already registers NotesController"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	for _, unwanted := range []string{"Updated config/components/controllers.go", "Add these by hand"} {
		if strings.Contains(out.String(), unwanted) {
			t.Errorf("output has %q:\n%s", unwanted, out.String())
		}
	}
}

// Finding I-2: a one-line literal is edited into valid, gofmt'd Go.
func TestRunRegistersInAOneLineLiteral(t *testing.T) {
	copyFixture(t)
	writeFile(t, "config/components/controllers.go", "package components\n\nimport (\n\t\"example.com/shop/app/controllers\"\n\t\"github.com/go-raptor/raptor/v4\"\n)\n\nfunc Controllers() raptor.Controllers {\n\treturn raptor.Controllers{&controllers.AuthController{}}\n}\n")
	var out bytes.Buffer
	if err := run(t, Options{Name: "Course", Out: &out}); err != nil {
		t.Fatal(err)
	}
	want := "\treturn raptor.Controllers{&controllers.AuthController{}, &controllers.CoursesController{}}\n"
	if got := readFile(t, "config/components/controllers.go"); !strings.Contains(got, want) {
		t.Errorf("controllers.go:\n%s", got)
	}
	if strings.Contains(out.String(), "Add these by hand") {
		t.Errorf("the edit should have been made:\n%s", out.String())
	}
}

// Finding I-3: two outputs on one path is a clear error, and nothing is written.
func TestRunRefusesCollidingPaths(t *testing.T) {
	copyFixture(t)
	before := snapshot(t, ".")
	err := run(t, Options{Name: "Validation"})
	if err == nil || !strings.Contains(err.Error(), "app/models/validation.go would be written twice") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, ".")) {
		t.Error("a refused run must not change any file")
	}
}

// A model file Go would skip (lab_test.go is a test file, x_linux.go is built only on Linux)
// gets a name it compiles under.
func TestRunNamesTheModelFileSoItCompiles(t *testing.T) {
	copyFixture(t)
	for _, name := range []string{"LabTest", "StoreWindows"} {
		if err := run(t, Options{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"app/models/lab_test_model.go", "app/models/store_windows_model.go"} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s", path)
		}
	}
	for _, path := range []string{"app/models/lab_test.go", "app/models/store_windows.go"} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s would not compile into the package", path)
		}
	}
}

// migrationVersions maps each migration's name (after the version) to its version.
func migrationVersions(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	versions, seen := map[string]string{}, map[string]string{}
	for _, e := range entries {
		version, name, _ := strings.Cut(e.Name(), "_")
		if other, dup := seen[version]; dup {
			t.Errorf("version %s is used by both %s and %s", version, other, e.Name())
		}
		seen[version], versions[name] = e.Name(), version
	}
	return versions
}

// Finding I-4: runs chained within one second must not reuse a Goose version.
func TestRunKeepsMigrationVersionsUnique(t *testing.T) {
	copyFixture(t)
	for _, o := range []Options{{Name: "Course"}, {Name: "Outcome", Parent: "Course"}, {Name: "Unit", Parent: "Outcome"}} {
		if err := run(t, o); err != nil { // the same Now each time
			t.Fatal(err)
		}
	}
	got := migrationVersions(t)
	want := map[string]string{
		"create_users.sql": "20260101000000", "setup.sql": "20260925115959",
		"create_courses.sql": "20260925120000", "create_outcomes.sql": "20260925120001", "create_units.sql": "20260925120002",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("versions = %v, want %v", got, want)
	}
}

// Finding I-4: with a migration newer than now, the setup and table migrations both go after it,
// the setup one first.
func TestRunStampsAfterTheLatestMigration(t *testing.T) {
	copyFixture(t)
	writeFile(t, "db/migrations/20260925120030_create_things.sql", "-- +goose Up\nSELECT 1;\n")
	if err := run(t, Options{Name: "Course"}); err != nil {
		t.Fatal(err)
	}
	got := migrationVersions(t)
	if got["setup.sql"] != "20260925120031" || got["create_courses.sql"] != "20260925120032" {
		t.Errorf("versions = %v; want setup at 20260925120031 and courses at 20260925120032", got)
	}
}
