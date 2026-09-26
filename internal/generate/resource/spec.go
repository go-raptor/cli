// Package resource implements `raptor g resource`: one command that scaffolds a whole entity
// the way the raptor-api-conventions skill writes one.
package resource

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

type FieldType int

const (
	String FieldType = iota
	Text
	Int
	Int64
	Bool
	Time
	Enum
	Ref
)

var fieldTypes = map[string]FieldType{
	"string": String, "text": Text, "int": Int, "int64": Int64,
	"bool": Bool, "time": Time, "enum": Enum, "ref": Ref,
}

const defaultStringLength = 150

// Field is one field spec from the command line: name:type[:arg][:optional].
type Field struct {
	Name       string // snake_case as given; a ref's name has no _id
	Type       FieldType
	Length     int      // String: VARCHAR length
	EnumValues []string // Enum
	RefModel   string   // Ref: target model, PascalCase
	Optional   bool
}

// Column is the database column; a ref's name gains _id.
func (f Field) Column() string {
	if f.Type == Ref {
		return f.Name + "_id"
	}
	return f.Name
}

func (f Field) GoName() string { return naming.GoField(f.Column()) }
func (f Field) JSON() string   { return naming.Camel(f.Column()) }

// Resource is everything the command line says about the entity to generate.
type Resource struct {
	Name    string // PascalCase singular: LectureGroup
	Plural  string // PascalCase plural: LectureGroups
	Fields  []Field
	Parent  string // PascalCase parent model; "" for a flat resource
	Movable bool
}

var (
	pascalRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	snakeRe  = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	enumRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// reservedColumns are columns every generated table already has, plus words PostgreSQL reserves.
var reservedColumns = map[string]bool{
	"id": true, "user_id": true, "created_at": true, "updated_at": true,
	"user": true, "order": true, "group": true, "table": true, "select": true, "default": true,
	"check": true, "references": true, "limit": true, "offset": true, "desc": true, "asc": true,
	"primary": true, "foreign": true, "unique": true, "end": true, "from": true, "where": true, "to": true,
}

// generatedIdents are local names the generated code already uses, so a resource's variable
// name must not take one.
var generatedIdents = map[string]bool{
	"s": true, "c": true, "r": true, "m": true, "t": true, "i": true, "x": true, "err": true,
	"res": true, "exists": true, "id": true, "ctx": true, "user": true, "req": true, "rec": true,
	"raw": true, "path": true, "list": true, "session": true, "owner": true, "stranger": true,
	"strangerSession": true, "suffix": true, "created": true, "updated": true, "listed": true,
	"body": true, "reader": true, "opts": true, "encoded": true,
}

// packageIdents are the packages and package-level helpers the generated files refer to. A
// resource whose variable would shadow one keeps its name, and its locals get renamed.
var packageIdents = map[string]bool{
	"models": true, "services": true, "controllers": true, "components": true, "config": true,
	"context": true, "json": true, "http": true, "httptest": true, "fmt": true, "time": true,
	"bytes": true, "io": true, "slices": true, "testing": true, "raptor": true, "errs": true,
	"strconv": true, "zog": true, "bun": true, "mime": true, "errors": true, "utf8": true,
	"os": true, "atomic": true, "bcrypt": true, "sql": true, "pgconn": true,
	"app": true, "db": true, "mustInsert": true, "newUser": true, "login": true, "withSession": true,
	"newClient": true, "testPassword": true, "clientIPs": true, "bindJSON": true,
	"validationFailed": true, "pathID": true, "maxChars": true,
}

// localName is ident, or fallback where ident as a variable would be a Go identifier, a name the
// generated code already uses, a package or helper it calls, or one of avoid.
func localName(ident, fallback string, avoid ...string) string {
	if naming.Reserved(ident) || generatedIdents[ident] || packageIdents[ident] || slices.Contains(avoid, ident) {
		return fallback
	}
	return ident
}

// takenGoNames are struct members the generated model or request already has.
var takenGoNames = map[string]string{
	"BaseModel": "the embedded bun.BaseModel",
	"ToModel":   "the request's ToModel method",
	"ApplyTo":   "the request's ApplyTo method",
}

// ParseField parses one name:type[:arg][:optional] spec.
func ParseField(spec string) (Field, error) {
	parts := strings.Split(spec, ":")
	if len(parts) < 2 {
		return Field{}, fmt.Errorf("field %q: expected name:type", spec)
	}
	f := Field{Name: parts[0]}
	if !snakeRe.MatchString(f.Name) {
		return Field{}, fmt.Errorf("field %q: the name must be snake_case, starting with a letter", spec)
	}
	typ, ok := fieldTypes[parts[1]]
	if !ok {
		return Field{}, fmt.Errorf("field %q: unknown type %q (use string, text, int, int64, bool, time, enum or ref)", spec, parts[1])
	}
	f.Type = typ
	args := parts[2:]
	if n := len(args); n > 0 && args[n-1] == "optional" {
		f.Optional = true
		args = args[:n-1]
	}
	switch typ {
	case String:
		f.Length = defaultStringLength
		if len(args) == 1 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 || n > 10485760 {
				return Field{}, fmt.Errorf("field %q: the string length must be a number from 1 to 10485760", spec)
			}
			f.Length, args = n, nil
		}
	case Text:
		if f.Optional {
			return Field{}, fmt.Errorf("field %q: text is already optional", spec)
		}
	case Enum:
		if len(args) != 1 || args[0] == "" {
			return Field{}, fmt.Errorf("field %q: enum needs its values, e.g. %s:enum:draft,published", spec, f.Name)
		}
		seen := map[string]bool{}
		for _, v := range strings.Split(args[0], ",") {
			if !enumRe.MatchString(v) {
				return Field{}, fmt.Errorf("field %q: enum value %q must be lowercase letters, digits and underscores", spec, v)
			}
			if seen[v] {
				return Field{}, fmt.Errorf("field %q: duplicate enum value %q", spec, v)
			}
			seen[v] = true
			f.EnumValues = append(f.EnumValues, v)
		}
		args = nil
	case Ref:
		f.RefModel = naming.GoField(f.Name)
		if len(args) == 1 {
			if !pascalRe.MatchString(args[0]) {
				return Field{}, fmt.Errorf("field %q: the ref target must be a PascalCase model name", spec)
			}
			f.RefModel, args = args[0], nil
		}
	}
	if len(args) > 0 {
		return Field{}, fmt.Errorf("field %q: unexpected %q", spec, strings.Join(args, ":"))
	}
	if reservedColumns[f.Name] || reservedColumns[f.Column()] {
		return Field{}, fmt.Errorf("field %q: %q is reserved", spec, f.Name)
	}
	if f.Type != Ref && strings.HasSuffix(f.Name, "_id") {
		return Field{}, fmt.Errorf("field %q: a foreign key is %s:ref, not %s", spec, strings.TrimSuffix(f.Name, "_id"), f.Name)
	}
	return f, nil
}

// NewResource validates the name and flags and parses the field specs. With no specs, the
// resource gets a single name:string.
func NewResource(name string, specs []string, parent, plural string, movable bool) (*Resource, error) {
	if !pascalRe.MatchString(name) {
		return nil, fmt.Errorf("resource name %q must be PascalCase, e.g. Course or LectureGroup", name)
	}
	r := &Resource{Name: name, Plural: naming.Plural(name), Parent: parent, Movable: movable}
	if plural != "" {
		if !pascalRe.MatchString(plural) {
			return nil, fmt.Errorf("--plural %q must be PascalCase, e.g. Criteria", plural)
		}
		r.Plural = plural
	}
	for _, ident := range []string{naming.Var(r.Name), naming.Var(r.Plural)} {
		if naming.Reserved(ident) {
			return nil, fmt.Errorf("resource name %q: %q is a Go keyword or predeclared identifier", name, ident)
		}
		if generatedIdents[ident] {
			return nil, fmt.Errorf("resource name %q: %q is used by the generated code; choose another name", name, ident)
		}
	}
	if parent != "" && !pascalRe.MatchString(parent) {
		return nil, fmt.Errorf("--parent %q must be a PascalCase model name", parent)
	}
	if movable && parent == "" {
		return nil, errors.New("--movable only applies with --parent")
	}
	if parent == name {
		return nil, errors.New("a resource cannot be its own parent")
	}
	if len(specs) == 0 {
		specs = []string{"name:string"}
	}

	parentColumn := ""
	goNames := map[string]bool{"ID": true, "CreatedAt": true, "UpdatedAt": true}
	if parent == "" {
		goNames["UserID"], goNames["User"] = true, true
	} else {
		parentColumn = naming.Snake(parent) + "_id"
		goNames[naming.GoField(parentColumn)], goNames[parent] = true, true
	}
	claim := func(goName string) error {
		if goNames[goName] {
			return fmt.Errorf("the Go name %s is used twice; rename a field", goName)
		}
		goNames[goName] = true
		return nil
	}

	columns := map[string]bool{}
	for _, spec := range specs {
		f, err := ParseField(spec)
		if err != nil {
			return nil, err
		}
		if f.Column() == parentColumn {
			return nil, fmt.Errorf("field %q: %s is the parent's column, added by --parent", spec, parentColumn)
		}
		if columns[f.Column()] {
			return nil, fmt.Errorf("field %q is declared twice", f.Column())
		}
		columns[f.Column()] = true
		if by, taken := takenGoNames[f.GoName()]; taken {
			return nil, fmt.Errorf("field %q: its Go name %s is taken by %s; rename the field", spec, f.GoName(), by)
		}
		if err := claim(f.GoName()); err != nil {
			return nil, err
		}
		if f.Type == Ref {
			relation := naming.GoField(f.Name)
			if relation == "BaseModel" {
				return nil, fmt.Errorf("field %q: its Go name %s is taken by %s; rename the field", spec, relation, takenGoNames[relation])
			}
			if err := claim(relation); err != nil { // the belongs-to relation
				return nil, err
			}
		}
		r.Fields = append(r.Fields, f)
	}
	declaredBy := map[string]string{}
	for _, d := range r.modelDecls() {
		if by, twice := declaredBy[d.name]; twice {
			return nil, fmt.Errorf("models.%s would be declared twice: by %s and by %s; rename the field or choose another name or --plural", d.name, by, d.by)
		}
		declaredBy[d.name] = d.by
	}
	return r, nil
}

// enumType is the Go type of an enum field: the model's name and the field's (UnitType).
func enumType(model string, f Field) string { return model + f.GoName() }

type declaration struct{ name, by string }

// modelDecls lists the package-level names the model file declares, with what declares each.
func (r *Resource) modelDecls() []declaration {
	n := r.Name
	out := []declaration{
		{n, "the model"}, {r.Plural, "the plural alias"}, {n + "Request", "the request type"},
		{n + "Schema", "the schema"}, {n + "UpdatableColumns", "the update allowlist"},
		{n + "Response", "the response type"}, {n + "Responses", "the responses alias"},
		{"New" + n + "Response", "the response constructor"}, {"New" + n + "Responses", "the responses constructor"},
	}
	for _, f := range r.Fields {
		if f.Type != Enum {
			continue
		}
		typ := enumType(r.Name, f)
		out = append(out,
			declaration{typ, fmt.Sprintf("field %s's enum type", f.Name)},
			declaration{typ + "Values", fmt.Sprintf("field %s's allow-list", f.Name)})
		for _, value := range f.EnumValues {
			out = append(out, declaration{typ + naming.GoField(value), fmt.Sprintf("field %s's constant for %q", f.Name, value)})
		}
	}
	return out
}
