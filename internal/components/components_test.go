package components

import (
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
	if want := "\"github.com/go-raptor/raptor/v4\"\n\t\"example.com/shop/app/services\""; !strings.Contains(out, want) {
		t.Fatalf("import not added:\n%s", out)
	}
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
