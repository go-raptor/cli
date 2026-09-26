package resource

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSeedModel(t *testing.T) {
	idx := loadFixtureModels(t)
	tests := []struct {
		model  string
		owned  bool
		fields []seedFieldView
	}{
		{"Division", false, []seedFieldView{{"Name", `"Name " + suffix`}}},
		{"Course", true, []seedFieldView{{"UserID", "userID"}, {"DivisionID", "seedDivision(t).ID"}, {"Name", `"Name " + suffix`}}},
		{"Outcome", true, []seedFieldView{{"CourseID", "seedCourse(t, userID).ID"}, {"Name", `"Name " + suffix`}}},
	}
	for _, tt := range tests {
		s, err := buildSeedModel(idx, idx[tt.model], "example.com/shop")
		if err != nil {
			t.Fatalf("%s: %v", tt.model, err)
		}
		if s.Owned != tt.owned || !reflect.DeepEqual(s.Fields, tt.fields) {
			t.Errorf("%s: owned=%v fields=%+v", tt.model, s.Owned, s.Fields)
		}
	}
	if _, err := buildSeedModel(idx, idx["Unit"], "example.com/shop"); err == nil || !strings.Contains(err.Error(), "no sample value for type UnitType") {
		t.Errorf("an unknown field type must be reported: %v", err)
	}
}

// Finding I-1: a promoted field can't go in a composite literal, so the seed assigns it after.
func TestBuildSeedModelPromotedFields(t *testing.T) {
	idx := loadFixtureModels(t)
	s, err := buildSeedModel(idx, idx["Folder"], "example.com/shop")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Owned || !reflect.DeepEqual(s.Fields, []seedFieldView{{"Name", `"Name " + suffix`}}) ||
		!reflect.DeepEqual(s.Promoted, []seedFieldView{{"Owned.UserID", "userID"}}) {
		t.Errorf("Folder: owned=%v fields=%+v promoted=%+v", s.Owned, s.Fields, s.Promoted)
	}
	src := mustRender(t, "seed_model.go.tmpl", s)
	loose(t, src, "folder := &models.Folder{ Name: \"Name \" + suffix, } folder.Owned.UserID = userID mustInsert(")

	if _, err := buildSeedModel(idx, idx["Shelf"], "example.com/shop"); err == nil || !strings.Contains(err.Error(), "embedded pointer *OwnedStamped") {
		t.Errorf("a field promoted through a pointer must be reported: %v", err)
	}
}

// Finding I-3: a seed's local must not shadow a package, a helper or the seed's own locals.
func TestBuildSeedModelRenamesCollidingLocals(t *testing.T) {
	for name, want := range map[string]string{"Division": "division", "Context": "contextItem", "Db": "dbItem", "Suffix": "suffixItem", "Type": "typeItem"} {
		m := &Model{Name: name, Table: "things", Fields: []ModelField{{GoName: "Name", GoType: "string", Column: "name"}}}
		s, err := buildSeedModel(ModelIndex{name: m}, m, "example.com/shop")
		if err != nil {
			t.Fatal(err)
		}
		if s.Var != want {
			t.Errorf("%s: Var = %q, want %q", name, s.Var, want)
		}
		mustRender(t, "seed_model.go.tmpl", s)
	}
}
