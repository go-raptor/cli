package resource

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseField(t *testing.T) {
	tests := []struct {
		spec string
		want Field
	}{
		{"name:string", Field{Name: "name", Type: String, Length: 150}},
		{"title:string:80", Field{Name: "title", Type: String, Length: 80}},
		{"nickname:string:optional", Field{Name: "nickname", Type: String, Length: 150, Optional: true}},
		{"code:string:12:optional", Field{Name: "code", Type: String, Length: 12, Optional: true}},
		{"notes:text", Field{Name: "notes", Type: Text}},
		{"lecture_hours:int", Field{Name: "lecture_hours", Type: Int}},
		{"size:int64:optional", Field{Name: "size", Type: Int64, Optional: true}},
		{"active:bool", Field{Name: "active", Type: Bool}},
		{"starts_at:time:optional", Field{Name: "starts_at", Type: Time, Optional: true}},
		{"type:enum:lecture,lab", Field{Name: "type", Type: Enum, EnumValues: []string{"lecture", "lab"}}},
		{"division:ref", Field{Name: "division", Type: Ref, RefModel: "Division"}},
		{"mentor:ref:User:optional", Field{Name: "mentor", Type: Ref, RefModel: "User", Optional: true}},
		{"api_key:ref", Field{Name: "api_key", Type: Ref, RefModel: "APIKey"}},
	}
	for _, tt := range tests {
		got, err := ParseField(tt.spec)
		if err != nil {
			t.Errorf("ParseField(%q): %v", tt.spec, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseField(%q) = %+v, want %+v", tt.spec, got, tt.want)
		}
	}
}

func TestFieldNames(t *testing.T) {
	f := Field{Name: "division", Type: Ref, RefModel: "Division"}
	if f.Column() != "division_id" || f.GoName() != "DivisionID" || f.JSON() != "divisionId" {
		t.Fatalf("ref names: %q %q %q", f.Column(), f.GoName(), f.JSON())
	}
	f = Field{Name: "url_path", Type: String}
	if f.Column() != "url_path" || f.GoName() != "URLPath" || f.JSON() != "urlPath" {
		t.Fatalf("multiword names: %q %q %q", f.Column(), f.GoName(), f.JSON())
	}
}

func TestParseFieldErrors(t *testing.T) {
	for spec, want := range map[string]string{
		"name":                  "expected name:type",
		"Name:string":           "snake_case",
		"name:varchar":          `unknown type "varchar"`,
		"name:string:0":         "string length",
		"notes:text:optional":   "text is already optional",
		"notes:text:5":          `unexpected "5"`,
		"type:enum":             "enum needs its values",
		"type:enum:Lecture":     `enum value "Lecture"`,
		"type:enum:a,a":         `duplicate enum value "a"`,
		"division:ref:division": "PascalCase model name",
		"order:string":          `"order" is reserved`,
		"user:ref":              `"user" is reserved`,
		"created_at:time":       `"created_at" is reserved`,
		"course_id:int64":       "course:ref",
	} {
		_, err := ParseField(spec)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParseField(%q) error = %v, want it to mention %q", spec, err, want)
		}
	}
}

func TestNewResource(t *testing.T) {
	r, err := NewResource("LectureGroup", nil, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Plural != "LectureGroups" || len(r.Fields) != 1 || !reflect.DeepEqual(r.Fields[0], Field{Name: "name", Type: String, Length: 150}) {
		t.Fatalf("defaults: %+v", r)
	}
	r, err = NewResource("Criterion", []string{"name:string"}, "", "Criteria", false)
	if err != nil || r.Plural != "Criteria" {
		t.Fatalf("plural override: %+v %v", r, err)
	}
}

func TestNewResourceErrors(t *testing.T) {
	tests := []struct {
		name    string
		specs   []string
		parent  string
		plural  string
		movable bool
		want    string
	}{
		{"course", nil, "", "", false, "PascalCase"},
		{"Type", nil, "", "", false, "Go keyword"},
		{"Err", nil, "", "", false, "used by the generated code"},
		{"List", nil, "", "", false, "used by the generated code"},
		{"Course", nil, "", "", true, "--movable only applies with --parent"},
		{"Course", nil, "Course", "", false, "its own parent"},
		{"Outcome", []string{"course:ref"}, "Course", "", false, "added by --parent"},
		{"Course", []string{"name:string", "name:text"}, "", "", false, "declared twice"},
		{"Course", []string{"division:ref", "division:string"}, "", "", false, "Division is used twice"},
		{"Course", nil, "", "courses", false, "--plural"},
		// Finding I-3: field names the generated structs already use.
		{"Course", []string{"to_model:string"}, "", "", false, "field \"to_model:string\": its Go name ToModel is taken by the request's ToModel method"},
		{"Course", []string{"apply_to:string"}, "", "", false, "its Go name ApplyTo is taken by the request's ApplyTo method"},
		{"Course", []string{"base_model:string"}, "", "", false, "its Go name BaseModel is taken by the embedded bun.BaseModel"},
		{"Course", []string{"base_model:ref"}, "", "", false, "its Go name BaseModel is taken by the embedded bun.BaseModel"},
		// Finding I-3: package-level names the model file would declare twice.
		{"Course", []string{"request:enum:a,b"}, "", "", false, "models.CourseRequest would be declared twice: by the request type and by field request's enum type"},
		{"Course", []string{"kind:enum:values"}, "", "", false, "models.CourseKindValues would be declared twice"},
		{"Course", []string{"kinds:enum:a"}, "", "CourseKinds", false, "models.CourseKinds would be declared twice"},
		{"Sheep", nil, "", "Sheep", false, "models.Sheep would be declared twice: by the model and by the plural alias"},
	}
	for _, tt := range tests {
		_, err := NewResource(tt.name, tt.specs, tt.parent, tt.plural, tt.movable)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("NewResource(%q, %v, parent %q) error = %v, want it to mention %q", tt.name, tt.specs, tt.parent, err, tt.want)
		}
	}
}
