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
