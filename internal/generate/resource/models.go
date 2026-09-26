package resource

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

// ModelField is one struct field of a Bun model, as far as the generator needs it.
type ModelField struct {
	GoName string // CourseID
	GoType string // int64, *int64, string, time.Time, UnitType, …
	Column string // course_id; "" for relations and bun:"-"
	// Embed is the path of the embedded struct the field comes from ("Owned",
	// "Base.Owned"), or "" for the model's own field. A composite literal can't set it.
	Embed string
	// ViaPointer is the first pointer embed on that path ("*Base"), or "".
	ViaPointer string
}

// Model is a struct in app/models that embeds bun.BaseModel.
type Model struct {
	Name      string // Course
	Table     string // courses
	Fields    []ModelField
	BelongsTo map[string]string // join column → target model, from rel:belongs-to
	// Unresolved lists embedded types the index can't read, such as a struct from another
	// package: the columns they add are unknown.
	Unresolved []string
}

func (m *Model) HasColumn(col string) bool {
	for _, f := range m.Fields {
		if f.Column == col {
			return true
		}
	}
	return false
}

func (m *Model) Field(goName string) (ModelField, bool) {
	for _, f := range m.Fields {
		if f.GoName == goName {
			return f, true
		}
	}
	return ModelField{}, false
}

// ModelIndex maps model names to models.
type ModelIndex map[string]*Model

// LoadModels parses every non-test Go file in dir. A missing directory is an empty index.
// Every struct type is collected first, so a model's embedded structs expand wherever in the
// package they are declared.
func LoadModels(dir string) (ModelIndex, error) {
	idx := ModelIndex{}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return idx, nil
	}
	if err != nil {
		return nil, err
	}
	structs := map[string]*ast.StructType{}
	var names []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filepath.Join(dir, name), err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts := spec.(*ast.TypeSpec)
				if st, ok := ts.Type.(*ast.StructType); ok && !ts.Assign.IsValid() {
					structs[ts.Name.Name] = st
					names = append(names, ts.Name.Name)
				}
			}
		}
	}
	for _, name := range names {
		if m := modelFromStruct(name, structs); m != nil {
			idx[m.Name] = m
		}
	}
	return idx, nil
}

// modelFromStruct indexes the named struct if it embeds bun.BaseModel, with the fields of its
// embedded structs promoted the way Bun promotes them.
func modelFromStruct(name string, structs map[string]*ast.StructType) *Model {
	m := &Model{Name: name, BelongsTo: map[string]string{}}
	isModel := false
	for _, f := range structs[name].Fields.List {
		if sel, ok := f.Type.(*ast.SelectorExpr); ok && len(f.Names) == 0 && sel.Sel.Name == "BaseModel" {
			isModel = true
			for _, part := range strings.Split(bunTag(f.Tag), ",") {
				if table, ok := strings.CutPrefix(part, "table:"); ok {
					m.Table = table
				}
			}
		}
	}
	if !isModel {
		return nil
	}
	m.addFields(structs[name], structs, embedding{within: map[string]bool{name: true}})
	if m.Table == "" {
		m.Table = naming.Snake(naming.Plural(name)) // Bun's default table name
	}
	return m
}

// embedding is where addFields is inside the model: the Go path of the embedded struct, the
// column prefix of a bun:"embed:" field, and the structs already being expanded.
type embedding struct {
	path, pointer, prefix string
	within                map[string]bool
}

func (e embedding) enter(field, typ, prefix string, pointer bool) embedding {
	next := embedding{path: field, pointer: e.pointer, prefix: e.prefix + prefix, within: map[string]bool{typ: true}}
	if e.path != "" {
		next.path = e.path + "." + field
	}
	if pointer && next.pointer == "" {
		next.pointer = "*" + typ
	}
	for k := range e.within {
		next.within[k] = true
	}
	return next
}

func (m *Model) addFields(st *ast.StructType, structs map[string]*ast.StructType, at embedding) {
	for _, f := range st.Fields.List {
		tag := bunTag(f.Tag)
		if tag == "-" {
			continue
		}
		embedPrefix, embedded := bunOption(tag, "embed")
		if len(f.Names) == 0 || embedded {
			typ, pointer := f.Type, false
			if star, ok := typ.(*ast.StarExpr); ok {
				typ, pointer = star.X, true
			}
			if sel, ok := typ.(*ast.SelectorExpr); ok && sel.Sel.Name == "BaseModel" && len(f.Names) == 0 {
				continue // the model marker, or a nested one Bun ignores
			}
			ident, ok := typ.(*ast.Ident)
			if !ok || structs[ident.Name] == nil || at.within[ident.Name] {
				m.Unresolved = append(m.Unresolved, types.ExprString(f.Type))
				continue
			}
			field := ident.Name
			if len(f.Names) > 0 {
				field = f.Names[0].Name
			}
			m.addFields(structs[ident.Name], structs, at.enter(field, ident.Name, embedPrefix, pointer))
			continue
		}
		goType := types.ExprString(f.Type)
		for _, n := range f.Names {
			mf := ModelField{GoName: n.Name, GoType: goType, Embed: at.path, ViaPointer: at.pointer}
			switch col, _, _ := strings.Cut(tag, ","); {
			case strings.Contains(tag, "rel:"):
				if strings.Contains(tag, "rel:belongs-to") {
					for _, part := range strings.Split(tag, ",") {
						if join, ok := strings.CutPrefix(part, "join:"); ok {
							joinCol, _, _ := strings.Cut(join, "=")
							m.BelongsTo[joinCol] = strings.TrimPrefix(goType, "*")
						}
					}
				}
			case col == "-":
			case col == "":
				mf.Column = at.prefix + naming.Snake(n.Name) // Bun's default column name
			default:
				mf.Column = at.prefix + col
			}
			m.Fields = append(m.Fields, mf)
		}
	}
}

// bunOption reports a key:value option of a bun tag, such as embed:prefix_.
func bunOption(tag, key string) (string, bool) {
	for _, part := range strings.Split(tag, ",") {
		if value, ok := strings.CutPrefix(part, key+":"); ok {
			return value, true
		}
	}
	return "", false
}

func bunTag(lit *ast.BasicLit) string {
	if lit == nil {
		return ""
	}
	raw, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return reflect.StructTag(raw).Get("bun")
}

// FKTarget reports the model an x_id column points at: the belongs-to relation that joins on
// it, or else the model its name implies (course_id → Course).
func (idx ModelIndex) FKTarget(m *Model, col string) (*Model, bool) {
	if !strings.HasSuffix(col, "_id") {
		return nil, false
	}
	name, ok := m.BelongsTo[col]
	if !ok {
		name = naming.GoField(strings.TrimSuffix(col, "_id"))
	}
	target, ok := idx[name]
	return target, ok
}
