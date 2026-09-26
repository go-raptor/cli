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

// Finding I-1: a promoted field can't go in a composite literal, so the seed assigns it after,
// through the promoted selector (fix round 2, N-1).
func TestBuildSeedModelPromotedFields(t *testing.T) {
	idx := loadFixtureModels(t)
	s, err := buildSeedModel(idx, idx["Folder"], "example.com/shop")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Owned || !reflect.DeepEqual(s.Fields, []seedFieldView{{"Name", `"Name " + suffix`}}) ||
		!reflect.DeepEqual(s.Promoted, []seedFieldView{{"UserID", "userID"}}) {
		t.Errorf("Folder: owned=%v fields=%+v promoted=%+v", s.Owned, s.Fields, s.Promoted)
	}
	src := mustRender(t, "seed_model.go.tmpl", s)
	loose(t, src, "folder := &models.Folder{ Name: \"Name \" + suffix, } folder.UserID = userID mustInsert(")

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

// Fix round 2, N-1: the seed lives in another package, so it assigns a promoted field through
// the selector Go promotes, which stays legal when an embedded type is unexported.
func TestBuildSeedModelSelectsPromotedFields(t *testing.T) {
	idx := loadFixtureModels(t)
	for model, want := range map[string][]seedFieldView{
		"Room":   {{"Code", `"Code " + suffix`}},
		"Box":    {{"UserID", "userID"}},
		"Locker": {{"Own.UserID", "userID"}},
	} {
		s, err := buildSeedModel(idx, idx[model], "example.com/shop")
		if err != nil {
			t.Errorf("%s: %v", model, err)
			continue
		}
		if !reflect.DeepEqual(s.Promoted, want) {
			t.Errorf("%s: promoted = %+v, want %+v", model, s.Promoted, want)
		}
		for _, f := range append(s.Fields, s.Promoted...) {
			if strings.Contains(f.GoName, "note") || strings.Contains(f.GoName, "coded") || strings.Contains(f.GoName, "ownedInner") {
				t.Errorf("%s: the seed names an unexported field: %+v", model, f)
			}
		}
	}
	if _, err := buildSeedModel(idx, idx["Kiosk"], "example.com/shop"); err == nil || !strings.Contains(err.Error(), "cannot seed Kiosk.Code: more than one field is named Code at that depth") {
		t.Errorf("an ambiguous promoted field must be reported: %v", err)
	}
	crate := &Model{Name: "Crate", Unresolved: []string{"audit.Trail"}, Fields: []ModelField{
		{GoName: "UserID", GoType: "int64", Column: "user_id", Embed: "Owned", Selector: "UserID", Depth: 1},
	}}
	if _, err := buildSeedModel(ModelIndex{"Crate": crate}, crate, "example.com/shop"); err == nil || !strings.Contains(err.Error(), "which the generator cannot read and which might declare UserID too") {
		t.Errorf("a promoted field beside an unreadable embed must be reported: %v", err)
	}
}
