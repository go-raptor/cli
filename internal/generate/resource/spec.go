// Package resource implements `raptor g resource`: one command that scaffolds a whole entity
// the way the raptor-api-conventions skill writes one.
package resource

import (
	"errors"
	"fmt"
	"regexp"
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
		if err := claim(f.GoName()); err != nil {
			return nil, err
		}
		if f.Type == Ref {
			if err := claim(naming.GoField(f.Name)); err != nil { // the belongs-to relation
				return nil, err
			}
		}
		r.Fields = append(r.Fields, f)
	}
	return r, nil
}
