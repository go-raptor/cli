package resource

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// scenarios build on one another in one project: a flat course, an outcome under it, a movable
// unit two levels down, and a record three levels down.
var scenarios = []struct {
	name string
	opts Options
}{
	{"course", Options{Name: "Course", Fields: []string{"name:string", "lecture_hours:int", "division:ref"}}},
	{"outcome", Options{Name: "Outcome", Fields: []string{"name:string", "criteria:text", "position:int"}, Parent: "Course"}},
	{"unit", Options{Name: "Unit", Fields: []string{"title:string:80", "type:enum:lecture,lab", "starts_at:time:optional", "active:bool"}, Parent: "Outcome", Movable: true}},
	{"record", Options{Name: "Record", Fields: []string{"group_number:int"}, Parent: "Unit"}},
}

// runScenarios generates every scenario into the working directory and returns, per scenario,
// the files it created or changed.
func runScenarios(t *testing.T) map[string]map[string]string {
	t.Helper()
	changed := map[string]map[string]string{}
	for i, s := range scenarios {
		opts := s.opts
		opts.Module, opts.Out = "example.com/shop", io.Discard
		opts.Now = testNow.Add(time.Duration(i) * time.Minute)
		before := snapshot(t, ".")
		if err := Run(opts); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		changed[s.name] = map[string]string{}
		for path, content := range snapshot(t, ".") {
			if before[path] != content {
				changed[s.name][path] = content
			}
		}
	}
	return changed
}

func TestGolden(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "golden"))
	if err != nil {
		t.Fatal(err)
	}
	copyFixture(t)
	for scenario, files := range runScenarios(t) {
		for path, content := range files {
			goldenPath := filepath.Join(golden, scenario, filepath.FromSlash(path)+".golden")
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Errorf("%s: no golden file for %s; run with -update and review it", scenario, path)
				continue
			}
			if string(want) != content {
				t.Errorf("%s: %s differs from %s; diff them, and run with -update only if the change is intended", scenario, path, goldenPath)
			}
		}
	}
}
