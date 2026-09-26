package resource

import (
	"fmt"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

// view is the data every template renders from.
type view struct {
	Module           string
	Name             string // Course
	Plural           string // Courses
	Var              string // course
	PluralVar        string // courses
	Local            string // the variable for one row: Var, or Var+"Item" where Var would shadow
	PluralLocal      string // the variable for a list: PluralVar, or Var+"Items"
	Recv             string // mapper receiver: the name's first letter, or m where that is r
	Table            string // courses
	Route            string // courses; kebab-case for multiword names
	Label            string // Course, Lecture group
	LabelLower       string // course, lecture group
	PluralLabelLower string // courses, lecture groups
	Parent           *parentView
	Movable          bool
	Fields           []fieldView
	Enums            []enumView
	Refs             []refView
	OrderBy          string // Order() arguments for a list
	ChildOrder       string // Order() arguments for a child's unfiltered list
	PredicateConst   string
	Predicate        string
	Updatable        string // the XUpdatableColumns elements
	UsesMaxChars     bool
	HasRequired      bool
	SamplesUseTime   bool
	UpdatedField     *fieldView
}

type parentView struct {
	Name, Plural, Var, Table string // Course, Courses, course, courses
	Field, Column, JSON      string // CourseID, course_id, courseId
	LabelLower               string
}

type fieldView struct {
	GoName, GoType, ReqType, JSON, Column string
	BunTag, Schema, Apply, SQLType, Check string
	Required                              bool   // the schema calls Required()
	SampleModel                           string // Go expression in a seed; "" leaves the zero value
	SampleReq                             string // Go expression in a test request
	UpdateSample                          string // string fields: the value an update test sends
}

type enumView struct {
	Type    string // SlotKind
	SQLType string // slot_kind
	Consts  []enumConst
}

type enumConst struct{ Name, Value string }

type refView struct {
	Relation string // Division: the belongs-to field
	Model    string
	Column   string
	Table    string
	Optional bool
}

func buildView(res *Resource, module string, idx ModelIndex) (*view, error) {
	v := &view{
		Module:           module,
		Name:             res.Name,
		Plural:           res.Plural,
		Var:              naming.Var(res.Name),
		PluralVar:        naming.Var(res.Plural),
		Table:            naming.Snake(res.Plural),
		Label:            naming.Label(res.Name),
		LabelLower:       naming.LabelLower(res.Name),
		PluralLabelLower: naming.LabelLower(res.Plural),
		Movable:          res.Movable,
		Recv:             strings.ToLower(res.Name[:1]),
	}
	v.Route = naming.Kebab(v.Table)
	testLocals := []string{"current"} // a test local next to the resource's own
	if res.Parent != "" {
		testLocals = append(testLocals, "other"+res.Parent, "strangers"+res.Parent)
	}
	v.Local = localName(v.Var, v.Var+"Item", testLocals...)
	v.PluralLocal = localName(v.PluralVar, v.Var+"Items", testLocals...)
	if v.Recv == "r" {
		v.Recv = "m" // r is the request receiver
	}

	var updatable []string
	if res.Parent != "" {
		chain, err := idx.OwnerChain(res.Parent)
		if err != nil {
			return nil, fmt.Errorf("--parent %s: %w", res.Parent, err)
		}
		pm := chain.Models[0]
		column := naming.Snake(pm.Name) + "_id"
		v.Parent = &parentView{
			Name: pm.Name, Plural: naming.GoField(pm.Table), Var: naming.Var(pm.Name), Table: pm.Table,
			Field: naming.GoField(column), Column: column, JSON: naming.Camel(column),
			LabelLower: naming.LabelLower(pm.Name),
		}
		v.PredicateConst = v.PluralVar + "OwnedByUser"
		v.Predicate = chain.Predicate(v.Table, column)
		v.HasRequired = true // the parent FK is required
		if res.Movable {
			updatable = append(updatable, column)
		}
	}

	orderKey := ""
	for _, f := range res.Fields {
		fv, err := v.field(f, idx)
		if err != nil {
			return nil, err
		}
		v.HasRequired = v.HasRequired || fv.Required
		if f.Type == String && !f.Optional && orderKey == "" {
			orderKey = f.Column()
		}
		updatable = append(updatable, f.Column())
		v.Fields = append(v.Fields, fv)
	}
	for i, f := range res.Fields {
		if f.Type == String && !f.Optional {
			v.UpdatedField = &v.Fields[i]
			break
		}
	}

	v.Updatable = quoteJoin(updatable)
	var order []string
	if orderKey != "" {
		order = append(order, v.Table+"."+orderKey)
	}
	order = append(order, v.Table+".id")
	v.OrderBy = quoteJoin(order)
	if v.Parent != nil {
		v.ChildOrder = quoteJoin(append([]string{v.Table + "." + v.Parent.Column}, order...))
	}
	return v, nil
}

// testFuncs are the package-level functions the integration test and seed files declare.
func (v *view) testFuncs() []string {
	fns := []string{
		v.PluralVar + "Request", "valid" + v.Name + "Request", "seed" + v.Name,
		"Test" + v.Plural + "RequireAuth", "Test" + v.Plural + "OwnerLifecycle", "Test" + v.Plural + "StrangerGets404",
	}
	if v.HasRequired {
		fns = append(fns, "Test"+v.Plural+"CreateValidation")
	}
	if v.Parent != nil {
		fns = append(fns, "Test"+v.Plural+"UnderStrangers"+v.Parent.Name)
		if v.Movable {
			fns = append(fns, "Test"+v.Plural+"CannotMoveToStrangers"+v.Parent.Name)
		} else {
			fns = append(fns, "Test"+v.Plural+"ParentIsImmutable")
		}
	}
	return fns
}

func (v *view) field(f Field, idx ModelIndex) (fieldView, error) {
	col := f.Column()
	fv := fieldView{GoName: f.GoName(), JSON: f.JSON(), Column: col, Apply: "r." + f.GoName()}
	required := !f.Optional
	switch f.Type {
	case String:
		fv.GoType, fv.ReqType = "string", "string"
		v.UsesMaxChars = true
		if required {
			fv.BunTag, fv.SQLType = col+",notnull", fmt.Sprintf("VARCHAR(%d) NOT NULL", f.Length)
			fv.Schema = fmt.Sprintf("zog.String().Trim().Min(1).TestFunc(maxChars(%d)).Required()", f.Length)
			fv.Required = true
			fv.SampleModel = fmt.Sprintf("%q", truncate("Sample", f.Length))
			fv.SampleReq = fv.SampleModel
			fv.UpdateSample = fmt.Sprintf("%q", truncate("Updated", f.Length))
		} else {
			fv.BunTag, fv.SQLType = col+",nullzero", fmt.Sprintf("VARCHAR(%d)", f.Length)
			fv.Schema = fmt.Sprintf("zog.String().Trim().TestFunc(maxChars(%d))", f.Length)
		}
	case Text:
		fv.GoType, fv.ReqType, fv.BunTag = "string", "string", col+",notnull"
		fv.SQLType, fv.Schema = "TEXT NOT NULL DEFAULT ''", "zog.String()"
		fv.SampleModel, fv.SampleReq = `"Sample text"`, `"Sample text"`
	case Int, Int64:
		goType, sqlType, zogType := "int", "INTEGER", "Int"
		if f.Type == Int64 {
			goType, sqlType, zogType = "int64", "BIGINT", "Int64"
		}
		fv.Check = col + " >= 0" // 0 is valid, so the schema never calls Required
		if required {
			fv.GoType, fv.BunTag, fv.SQLType = goType, col+",notnull", sqlType+" NOT NULL DEFAULT 0"
			fv.Schema = fmt.Sprintf("zog.%s().GTE(0)", zogType)
			fv.SampleModel, fv.SampleReq = "1", "1"
		} else {
			fv.GoType, fv.BunTag, fv.SQLType = "*"+goType, col, sqlType
			fv.Schema = fmt.Sprintf("zog.Ptr(zog.%s().GTE(0))", zogType)
		}
		fv.ReqType = fv.GoType
	case Bool:
		if required {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "bool", col+",notnull", "BOOLEAN NOT NULL DEFAULT false", "zog.Bool()"
			fv.SampleModel, fv.SampleReq = "true", "true"
		} else {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "*bool", col, "BOOLEAN", "zog.Ptr(zog.Bool())"
		}
		fv.ReqType = fv.GoType
	case Time:
		if required {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "time.Time", col+",notnull", "TIMESTAMPTZ NOT NULL", "zog.Time().Required()"
			fv.Required = true
			fv.SampleModel = "time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)"
			fv.SampleReq = fv.SampleModel
			v.SamplesUseTime = true
		} else {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "*time.Time", col, "TIMESTAMPTZ", "zog.Ptr(zog.Time())"
		}
		fv.ReqType = fv.GoType
	case Enum:
		e := enumView{Type: enumType(v.Name, f), SQLType: naming.Snake(v.Name) + "_" + col}
		for _, value := range f.EnumValues {
			e.Consts = append(e.Consts, enumConst{Name: e.Type + naming.GoField(value), Value: value})
		}
		v.Enums = append(v.Enums, e)
		fv.GoType, fv.ReqType, fv.Apply = e.Type, "string", e.Type+"(r."+f.GoName()+")"
		if required {
			fv.BunTag, fv.SQLType = col+",notnull", e.SQLType+" NOT NULL"
			fv.Schema = fmt.Sprintf("zog.String().OneOf(%sValues).Required()", e.Type)
			fv.Required = true
			fv.SampleModel = "models." + e.Consts[0].Name
			fv.SampleReq = "string(models." + e.Consts[0].Name + ")"
		} else {
			fv.BunTag, fv.SQLType = col+",nullzero", e.SQLType
			fv.Schema = fmt.Sprintf("zog.String().OneOf(%sValues)", e.Type)
		}
	case Ref:
		target, ok := idx[f.RefModel]
		if !ok {
			return fieldView{}, fmt.Errorf("field %s: model %s not found in app/models", f.Name, f.RefModel)
		}
		if err := idx.CheckRefTarget(target.Name); err != nil {
			return fieldView{}, fmt.Errorf("field %s: %w", f.Name, err)
		}
		v.Refs = append(v.Refs, refView{Relation: naming.GoField(f.Name), Model: target.Name, Column: col, Table: target.Table, Optional: f.Optional})
		if required {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "int64", col+",notnull", "BIGINT NOT NULL", "zog.Int64().GT(0).Required()"
			fv.Required = true
			fv.SampleModel = "seed" + target.Name + "(t).ID"
			fv.SampleReq = fv.SampleModel
		} else {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "*int64", col, "BIGINT", "zog.Ptr(zog.Int64().GT(0))"
		}
		fv.ReqType = fv.GoType
	}
	return fv, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}
