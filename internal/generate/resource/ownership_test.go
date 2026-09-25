package resource

import (
	"strings"
	"testing"
)

func TestIsOwned(t *testing.T) {
	idx := loadFixtureModels(t)
	for name, want := range map[string]bool{
		"Course": true, "Outcome": true, "Unit": true, "Lesson": true, "LectureGroup": true,
		"Division": false, "User": false, "Tag": false, "Missing": false,
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
	} {
		_, err := idx.OwnerChain(name)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("OwnerChain(%s) error = %v, want it to mention %q", name, err, want)
		}
	}
}
