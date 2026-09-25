package resource

import "testing"

// loadFixtureModels indexes testdata/models, shared by the model, ownership and view tests.
func loadFixtureModels(t *testing.T) ModelIndex {
	t.Helper()
	idx, err := LoadModels("testdata/models")
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestLoadModels(t *testing.T) {
	idx := loadFixtureModels(t)

	course := idx["Course"]
	if course == nil || course.Table != "courses" {
		t.Fatalf("Course: %+v", course)
	}
	if !course.HasColumn("user_id") || !course.HasColumn("division_id") || course.HasColumn("division") {
		t.Errorf("Course columns: %+v", course.Fields)
	}
	if course.BelongsTo["division_id"] != "Division" {
		t.Errorf("belongs-to relations: %+v", course.BelongsTo)
	}
	if f, ok := course.Field("DivisionID"); !ok || f.GoType != "int64" || f.Column != "division_id" {
		t.Errorf("Field(DivisionID) = %+v, %v", f, ok)
	}

	unit := idx["Unit"]
	if !unit.HasColumn("title") {
		t.Error("an empty bun name must fall back to the snake-cased field name")
	}
	if target, ok := idx.FKTarget(unit, "outcome_id"); !ok || target.Name != "Outcome" {
		t.Error("outcome_id without a relation must resolve to Outcome by name")
	}
	if _, ok := idx["CourseRequest"]; ok {
		t.Error("a struct without bun.BaseModel is not a model")
	}
	if idx["Tag"] == nil || idx["LectureGroup"].Table != "lecture_groups" {
		t.Error("grouped declarations and multiword models must be indexed")
	}
}

func TestLoadModelsWithoutADirectory(t *testing.T) {
	idx, err := LoadModels("testdata/nope")
	if err != nil || len(idx) != 0 {
		t.Fatalf("a project without app/models has no models: %v %v", idx, err)
	}
}
