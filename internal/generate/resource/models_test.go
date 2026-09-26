package resource

import (
	"reflect"
	"testing"
)

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

// Finding I-1: fields of structs embedded from app/models belong to the embedding model's table.
func TestLoadModelsPromotesEmbeddedFields(t *testing.T) {
	idx := loadFixtureModels(t)

	folder := idx["Folder"]
	if folder == nil || !folder.HasColumn("user_id") || !folder.HasColumn("name") || folder.BelongsTo["user_id"] != "User" {
		t.Fatalf("Folder must gain Owned's user_id and its relation: %+v", folder)
	}
	if f, ok := folder.Field("UserID"); !ok || f.Embed != "Owned" || f.ViaPointer != "" {
		t.Errorf("Field(UserID) = %+v, %v; want a field promoted from Owned", f, ok)
	}
	if f, _ := folder.Field("Name"); f.Embed != "" {
		t.Error("Folder's own Name is not promoted")
	}

	shelf := idx["Shelf"]
	if shelf == nil || !shelf.HasColumn("user_id") || !shelf.HasColumn("created_at") {
		t.Fatalf("Shelf must gain the nested mixins' columns through the pointer embed: %+v", shelf)
	}
	if f, _ := shelf.Field("UserID"); f.Embed != "OwnedStamped.Owned" || f.ViaPointer != "*OwnedStamped" {
		t.Errorf("Shelf.UserID = %+v; want promoted through a pointer", f)
	}

	if _, ok := idx["Owned"]; ok {
		t.Error("a mixin without bun.BaseModel is not a model")
	}
	if got := idx["Badge"].Unresolved; len(got) != 1 || got[0] != "audit.Trail" {
		t.Errorf("Badge.Unresolved = %v, want [audit.Trail]", got)
	}
	if got := idx["Folder"].Unresolved; len(got) != 0 {
		t.Errorf("Folder.Unresolved = %v, want none", got)
	}
}

// Fix round 2, I-1(a): only bun's BaseModel, under whatever name bun is imported, marks a model.
func TestLoadModelsKnowsBunsBaseModel(t *testing.T) {
	idx := loadFixtureModels(t)
	if stamp := idx["Stamp"]; stamp == nil || stamp.Table != "stamps" || len(stamp.Unresolved) != 0 {
		t.Errorf("Stamp imports bun as ub and is a model: %+v", stamp)
	}
	if safe := idx["Safe"]; safe == nil || safe.Table != "safes" || !safe.HasColumn("user_id") || len(safe.Unresolved) != 0 {
		t.Errorf("Safe gains user_id from the project's own BaseModel: %+v", safe)
	}
	if vault := idx["Vault"]; vault == nil || !reflect.DeepEqual(vault.Unresolved, []string{"common.BaseModel"}) || vault.HasColumn("user_id") {
		t.Errorf("Vault's common.BaseModel is not bun's, so it is unresolved: %+v", vault)
	}
}

func TestLoadModelsWithoutADirectory(t *testing.T) {
	idx, err := LoadModels("testdata/nope")
	if err != nil || len(idx) != 0 {
		t.Fatalf("a project without app/models has no models: %v %v", idx, err)
	}
}
