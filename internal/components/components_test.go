package components

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const servicesSrc = `package components

import (
	"github.com/go-raptor/raptor/v4"
	"example.com/shop/app/services"
)

func Services() raptor.Services {
	return raptor.Services{
		&services.HelloService{},
	}
}
`

func TestAddEntryAppendsToTheSlice(t *testing.T) {
	out, err := AddEntry(servicesSrc, "example.com/shop", "service", "CoursesService")
	if err != nil {
		t.Fatal(err)
	}
	if want := "\t\t&services.HelloService{},\n\t\t&services.CoursesService{},\n\t}"; !strings.Contains(out, want) {
		t.Fatalf("entry not appended:\n%s", out)
	}
}

func TestAddEntryAddsTheImport(t *testing.T) {
	src := strings.Replace(servicesSrc, "\t\"example.com/shop/app/services\"\n", "", 1)
	out, err := AddEntry(src, "example.com/shop", "service", "CoursesService")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\t\"example.com/shop/app/services\"\n") || !strings.Contains(out, "&services.CoursesService{},") {
		t.Fatalf("import not added:\n%s", out)
	}
	mustParse(t, out)
}

func TestAddEntryIsIdempotent(t *testing.T) {
	out, err := AddEntry(servicesSrc, "example.com/shop", "service", "HelloService")
	if err != nil || out != servicesSrc {
		t.Fatalf("an already registered component must leave the file unchanged: %v\n%s", err, out)
	}
}

func TestAddEntryWithoutTheSliceFails(t *testing.T) {
	src := "package components\n\nimport (\n\t\"github.com/go-raptor/raptor/v4\"\n)\n"
	if _, err := AddEntry(src, "example.com/shop", "service", "CoursesService"); err == nil {
		t.Fatal("a file without raptor.Services{ must be reported, not guessed at")
	}
}

func mustParse(t *testing.T, src string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "", src, 0); err != nil {
		t.Fatalf("the edited file does not parse: %v\n%s", err, src)
	}
}

// Finding I-2: a name that merely contains the struct name is not a registration.
func TestAddEntrySubstringIsNotRegistered(t *testing.T) {
	src := strings.Replace(servicesSrc, "HelloService", "LectureNotesService", 1)
	out, err := AddEntry(src, "example.com/shop", "service", "NotesService")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "&services.LectureNotesService{},\n\t\t&services.NotesService{},\n\t}") {
		t.Fatalf("NotesService must be registered next to LectureNotesService:\n%s", out)
	}
	mustParse(t, out)
}

// Finding I-2: the exact entry counts as registered, however it is spaced.
func TestAddEntryAlreadyRegistered(t *testing.T) {
	src := strings.Replace(servicesSrc, "&services.HelloService{},", "& services.HelloService{ },", 1)
	out, err := AddEntry(src, "example.com/shop", "service", "HelloService")
	if err != nil || out != src {
		t.Fatalf("an already registered component must leave the file unchanged: %v\n%s", err, out)
	}
}

// Finding I-2: literals on one line are edited into valid Go, never corrupted.
func TestAddEntryOneLineLiterals(t *testing.T) {
	for name, tt := range map[string]struct{ literal, want string }{
		"one entry":      {"return raptor.Controllers{&controllers.AuthController{}}", "return raptor.Controllers{&controllers.AuthController{}, &controllers.CoursesController{}}"},
		"trailing comma": {"return raptor.Controllers{&controllers.AuthController{}, }", "return raptor.Controllers{&controllers.AuthController{}, &controllers.CoursesController{}}"},
		"empty":          {"return raptor.Controllers{}", "return raptor.Controllers{\n\t\t&controllers.CoursesController{},\n\t}"},
	} {
		src := "package components\n\nimport (\n\t\"example.com/shop/app/controllers\"\n\t\"github.com/go-raptor/raptor/v4\"\n)\n\nfunc Controllers() raptor.Controllers {\n\t" + tt.literal + "\n}\n"
		out, err := AddEntry(src, "example.com/shop", "controller", "CoursesController")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(out, tt.want) {
			t.Errorf("%s: want %q in:\n%s", name, tt.want, out)
		}
		mustParse(t, out)
	}
}

// Finding I-2: an edit that would not be valid Go is an error, so the caller prints the manual
// step and leaves the file alone.
func TestAddEntryRefusesWhatItCannotFormat(t *testing.T) {
	src := servicesSrc + "\nfunc broken( {\n"
	if out, err := AddEntry(src, "example.com/shop", "service", "CoursesService"); err == nil {
		t.Fatalf("a file that does not parse must be reported, not edited:\n%s", out)
	}
}

// Finding I-2: the import goes in validly whether or not the file groups its imports.
func TestAddEntryAddsTheImportToASingleImport(t *testing.T) {
	src := "package components\n\nimport \"github.com/go-raptor/raptor/v4\"\n\nfunc Services() raptor.Services {\n\treturn raptor.Services{}\n}\n"
	out, err := AddEntry(src, "example.com/shop", "service", "CoursesService")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "import \"example.com/shop/app/services\"\n") || !strings.Contains(out, "&services.CoursesService{},") {
		t.Fatalf("import or entry missing:\n%s", out)
	}
	mustParse(t, out)
}
