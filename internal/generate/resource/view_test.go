package resource

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-raptor/cli/internal/naming"
)

func mustView(t *testing.T, name string, specs []string, parent string, movable bool) *view {
	t.Helper()
	res, err := NewResource(name, specs, parent, "", movable)
	if err != nil {
		t.Fatal(err)
	}
	v, err := buildView(res, "example.com/shop", loadFixtureModels(t))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Finding I-3: a local that would shadow a package, a harness helper or another test local is
// renamed; everything derived from the name (IDs, helpers, the predicate) keeps it.
func TestBuildViewRenamesCollidingLocals(t *testing.T) {
	for _, tt := range []struct{ name, parent, local, pluralLocal string }{
		{"Seminar", "", "seminar", "seminars"},
		{"Model", "", "model", "modelItems"},
		{"Context", "", "contextItem", "contexts"},
		{"Login", "", "loginItem", "logins"},
		{"Db", "", "dbItem", "dbs"},
		{"Current", "Course", "currentItem", "currents"},
		{"OtherCourse", "Course", "otherCourseItem", "otherCourses"},
		{"StrangersCourse", "Course", "strangersCourseItem", "strangersCourses"},
	} {
		v := mustView(t, tt.name, nil, tt.parent, false)
		if v.Local != tt.local || v.PluralLocal != tt.pluralLocal {
			t.Errorf("%s: locals %q, %q; want %q, %q", tt.name, v.Local, v.PluralLocal, tt.local, tt.pluralLocal)
		}
		if v.Var != naming.Var(tt.name) {
			t.Errorf("%s: Var = %q; the derived names keep the natural one", tt.name, v.Var)
		}
	}
}

func TestBuildViewFlat(t *testing.T) {
	v := mustView(t, "Seminar", []string{"name:string", "lecture_hours:int", "division:ref", "notes:text"}, "", false)

	if v.Table != "seminars" || v.Route != "seminars" || v.Var != "seminar" || v.PluralVar != "seminars" || v.Recv != "s" {
		t.Fatalf("names: %+v", v)
	}
	if v.Updatable != `"name", "lecture_hours", "division_id", "notes"` {
		t.Errorf("Updatable = %s", v.Updatable)
	}
	if v.OrderBy != `"seminars.name", "seminars.id"` {
		t.Errorf("OrderBy = %s", v.OrderBy)
	}
	if want := []refView{{Relation: "Division", Model: "Division", Column: "division_id", Table: "divisions"}}; !reflect.DeepEqual(v.Refs, want) {
		t.Errorf("Refs = %+v", v.Refs)
	}
	name, hours, division, notes := v.Fields[0], v.Fields[1], v.Fields[2], v.Fields[3]
	if name.Schema != "zog.String().Trim().Min(1).TestFunc(maxChars(150)).Required()" || name.SQLType != "VARCHAR(150) NOT NULL" || !name.Required {
		t.Errorf("name: %+v", name)
	}
	if hours.Schema != "zog.Int().GTE(0)" || hours.Check != "lecture_hours >= 0" || hours.Required || hours.JSON != "lectureHours" {
		t.Errorf("lecture_hours: %+v", hours)
	}
	if division.GoName != "DivisionID" || division.SampleModel != "seedDivision(t).ID" || division.Schema != "zog.Int64().GT(0).Required()" {
		t.Errorf("division: %+v", division)
	}
	if notes.SQLType != "TEXT NOT NULL DEFAULT ''" || notes.Schema != "zog.String()" || notes.Required {
		t.Errorf("notes: %+v", notes)
	}
	if !v.UsesMaxChars || !v.HasRequired || v.UpdatedField == nil || v.UpdatedField.GoName != "Name" {
		t.Errorf("flags: maxChars=%v required=%v updated=%+v", v.UsesMaxChars, v.HasRequired, v.UpdatedField)
	}
}

// Review Focus 4: multiword names agree everywhere.
func TestBuildViewMultiwordChild(t *testing.T) {
	v := mustView(t, "StudyNote", nil, "LectureGroup", false)
	p := v.Parent
	if p.Field != "LectureGroupID" || p.Column != "lecture_group_id" || p.JSON != "lectureGroupId" || p.Plural != "LectureGroups" || p.Var != "lectureGroup" {
		t.Fatalf("parent: %+v", p)
	}
	if v.Table != "study_notes" || v.Route != "study-notes" || v.PredicateConst != "studyNotesOwnedByUser" {
		t.Errorf("names: %s %s %s", v.Table, v.Route, v.PredicateConst)
	}
	if v.Predicate != "study_notes.lecture_group_id IN (SELECT id FROM lecture_groups WHERE user_id = ?)" {
		t.Errorf("Predicate = %s", v.Predicate)
	}
	if v.ChildOrder != `"study_notes.lecture_group_id", "study_notes.name", "study_notes.id"` || v.Updatable != `"name"` {
		t.Errorf("order %s, updatable %s", v.ChildOrder, v.Updatable)
	}
}

func TestBuildViewMovableEnumOptional(t *testing.T) {
	v := mustView(t, "Slot", []string{"kind:enum:lecture,in_person", "starts_at:time:optional", "active:bool"}, "Outcome", true)
	if v.Updatable != `"outcome_id", "kind", "starts_at", "active"` {
		t.Errorf("Updatable = %s", v.Updatable)
	}
	want := enumView{Type: "SlotKind", SQLType: "slot_kind", Consts: []enumConst{{"SlotKindLecture", "lecture"}, {"SlotKindInPerson", "in_person"}}}
	if len(v.Enums) != 1 || !reflect.DeepEqual(v.Enums[0], want) {
		t.Errorf("Enums = %+v", v.Enums)
	}
	kind, startsAt, active := v.Fields[0], v.Fields[1], v.Fields[2]
	if kind.Apply != "SlotKind(r.Kind)" || kind.ReqType != "string" || kind.GoType != "SlotKind" || kind.SampleReq != "string(models.SlotKindLecture)" || kind.SampleModel != "models.SlotKindLecture" {
		t.Errorf("kind: %+v", kind)
	}
	if startsAt.GoType != "*time.Time" || startsAt.Schema != "zog.Ptr(zog.Time())" || startsAt.BunTag != "starts_at" || startsAt.SampleModel != "" {
		t.Errorf("starts_at: %+v", startsAt)
	}
	if active.SQLType != "BOOLEAN NOT NULL DEFAULT false" || active.SampleReq != "true" {
		t.Errorf("active: %+v", active)
	}
	if v.UpdatedField != nil || v.UsesMaxChars {
		t.Errorf("no string field: updated=%+v maxChars=%v", v.UpdatedField, v.UsesMaxChars)
	}
}

// Finding I-1: a parent owned through an embedded mixin chains to its own user_id.
func TestBuildViewUnderAMixinParent(t *testing.T) {
	v := mustView(t, "Page", nil, "Folder", false)
	if v.Predicate != "pages.folder_id IN (SELECT id FROM folders WHERE user_id = ?)" {
		t.Errorf("Predicate = %s", v.Predicate)
	}
	v = mustView(t, "Page", nil, "Doc", false)
	if !strings.Contains(v.Predicate, "WHERE folders.user_id = ?") {
		t.Errorf("Predicate = %s", v.Predicate)
	}
	if v := mustView(t, "Seminar", []string{"mentor:ref:User"}, "", false); len(v.Refs) != 1 {
		t.Error("ref:User stays allowed")
	}
}

func TestBuildViewErrors(t *testing.T) {
	idx := loadFixtureModels(t)
	tests := []struct {
		name, parent string
		specs        []string
		want         string
	}{
		{"Seminar", "", []string{"course:ref"}, "Course is owned by a user; use --parent Course"},
		{"Seminar", "", []string{"thing:ref"}, "model Thing not found"},
		{"Seminar", "Division", nil, "not owned by a user"},
		// Finding I-1: the ref gate fails closed.
		{"Seminar", "", []string{"folder:ref"}, "field folder: Folder is owned by a user; use --parent Folder"},
		{"Seminar", "", []string{"doc:ref"}, "field doc: Doc is owned by a user; use --parent Doc"},
		{"Seminar", "", []string{"album:ref"}, "field album: Album references users through owner_id"},
		{"Seminar", "", []string{"track:ref"}, "field track: Track reaches Album through album_id"},
		{"Seminar", "", []string{"badge:ref"}, "field badge: Badge embeds audit.Trail"},
		{"Seminar", "", []string{"clip:ref"}, "field clip: Clip has a belongs-to relation to media.Source"},
		// --parent fails closed on an owner column it cannot follow
		{"Seminar", "Album", nil, "Album references users through owner_id"},
		{"Seminar", "Track", nil, "not owned by a user"},
	}
	for _, tt := range tests {
		res, err := NewResource(tt.name, tt.specs, tt.parent, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := buildView(res, "example.com/shop", idx); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%v under %q: error = %v, want it to mention %q", tt.specs, tt.parent, err, tt.want)
		}
	}
}
