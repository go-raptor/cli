package resource

import (
	"strings"
	"testing"
)

func TestIsOwned(t *testing.T) {
	idx := loadFixtureModels(t)
	for name, want := range map[string]bool{
		"Course": true, "Outcome": true, "Unit": true, "Lesson": true, "LectureGroup": true,
		"Folder": true, "Shelf": true, "Doc": true, "Binder": true,
		"Division": false, "User": false, "Tag": false, "Missing": false,
		"Album": false, "Track": false, "Badge": false, "Clip": false,
	} {
		if got := idx.IsOwned(name); got != want {
			t.Errorf("IsOwned(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestOwnerChainPredicate(t *testing.T) {
	idx := loadFixtureModels(t)
	tests := []struct{ parent, childTable, childFK, want string }{
		{"Course", "outcomes", "course_id",
			"outcomes.course_id IN (SELECT id FROM courses WHERE user_id = ?)"},
		{"Outcome", "units", "outcome_id",
			"units.outcome_id IN (\n\tSELECT outcomes.id FROM outcomes\n\tJOIN courses ON courses.id = outcomes.course_id\n\tWHERE courses.user_id = ?\n)"},
		{"Unit", "records", "unit_id",
			"records.unit_id IN (\n\tSELECT units.id FROM units\n\tJOIN outcomes ON outcomes.id = units.outcome_id\n\tJOIN courses ON courses.id = outcomes.course_id\n\tWHERE courses.user_id = ?\n)"},
		// Review Focus 2: Lesson's division_id points at reference data and is not a parent.
		{"Lesson", "notes", "lesson_id",
			"notes.lesson_id IN (\n\tSELECT lessons.id FROM lessons\n\tJOIN outcomes ON outcomes.id = lessons.outcome_id\n\tJOIN courses ON courses.id = outcomes.course_id\n\tWHERE courses.user_id = ?\n)"},
		// Finding I-1: user_id from an embedded mixin makes the model the root of the chain.
		{"Folder", "pages", "folder_id",
			"pages.folder_id IN (SELECT id FROM folders WHERE user_id = ?)"},
		{"Binder", "sheets", "binder_id",
			"sheets.binder_id IN (SELECT id FROM binders WHERE user_id = ?)"},
		{"Doc", "pages", "doc_id",
			"pages.doc_id IN (\n\tSELECT docs.id FROM docs\n\tJOIN folders ON folders.id = docs.folder_id\n\tWHERE folders.user_id = ?\n)"},
	}
	for _, tt := range tests {
		chain, err := idx.OwnerChain(tt.parent)
		if err != nil {
			t.Errorf("OwnerChain(%s): %v", tt.parent, err)
			continue
		}
		if got := chain.Predicate(tt.childTable, tt.childFK); got != tt.want {
			t.Errorf("predicate under %s:\n got %q\nwant %q", tt.parent, got, tt.want)
		}
	}
}

func TestOwnerChainErrors(t *testing.T) {
	idx := loadFixtureModels(t)
	for name, want := range map[string]string{
		"Division": "not owned by a user",
		"Link":     "more than one owned parent (course_id, outcome_id)",
		"Missing":  "not found in app/models",
		// Finding I-1: an owner under another column name fails closed, and says why.
		"Album": "Album references users through owner_id",
		"Track": "not owned by a user",
	} {
		_, err := idx.OwnerChain(name)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("OwnerChain(%s) error = %v, want it to mention %q", name, err, want)
		}
	}
}

// Finding I-1: a ref gets no ownership check, so the gate refuses anything it cannot prove is
// shared reference data.
func TestCheckRefTarget(t *testing.T) {
	idx := loadFixtureModels(t)
	for _, name := range []string{"Division", "Tag", "User"} {
		if err := idx.CheckRefTarget(name); err != nil {
			t.Errorf("CheckRefTarget(%s): %v", name, err)
		}
	}
	for name, want := range map[string]string{
		"Course": "Course is owned by a user; use --parent Course, or the ownership check would be skipped",
		"Lesson": "Lesson is owned by a user; use --parent Lesson",
		// the mixin, and a target owned through the mixin model
		"Folder": "Folder is owned by a user; use --parent Folder",
		"Doc":    "Doc is owned by a user; use --parent Doc",
		// an owner column under another name, directly and through a foreign key
		"Album": "Album references users through owner_id",
		"Track": "Track reaches Album through album_id, and Album references users through owner_id",
		// what the index cannot see
		"Badge": "Badge embeds audit.Trail, which is not declared in app/models",
		"Clip":  "Clip has a belongs-to relation to media.Source, which is not a model in app/models",
	} {
		err := idx.CheckRefTarget(name)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("CheckRefTarget(%s) error = %v, want it to mention %q", name, err, want)
			continue
		}
		if _, chainErr := idx.OwnerChain(name); chainErr != nil && strings.Contains(err.Error(), "--parent") {
			t.Errorf("CheckRefTarget(%s) suggests --parent, which fails: %v", name, chainErr)
		}
	}
}
