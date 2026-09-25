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
}

// Model is a struct in app/models that embeds bun.BaseModel.
type Model struct {
	Name      string // Course
	Table     string // courses
	Fields    []ModelField
	BelongsTo map[string]string // join column → target model, from rel:belongs-to
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
func LoadModels(dir string) (ModelIndex, error) {
	idx := ModelIndex{}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return idx, nil
	}
	if err != nil {
		return nil, err
	}
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
				if st, ok := ts.Type.(*ast.StructType); ok {
					if m := modelFromStruct(ts.Name.Name, st); m != nil {
						idx[m.Name] = m
					}
				}
			}
		}
	}
	return idx, nil
}

func modelFromStruct(name string, st *ast.StructType) *Model {
	m := &Model{Name: name, BelongsTo: map[string]string{}}
	isModel := false
	for _, f := range st.Fields.List {
		tag := bunTag(f.Tag)
		if len(f.Names) == 0 {
			if sel, ok := f.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "BaseModel" {
				isModel = true
				for _, part := range strings.Split(tag, ",") {
					if table, ok := strings.CutPrefix(part, "table:"); ok {
						m.Table = table
					}
				}
			}
			continue
		}
		goType := types.ExprString(f.Type)
		for _, n := range f.Names {
			mf := ModelField{GoName: n.Name, GoType: goType}
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
				mf.Column = naming.Snake(n.Name) // Bun's default column name
			default:
				mf.Column = col
			}
			m.Fields = append(m.Fields, mf)
		}
	}
	if !isModel {
		return nil
	}
	if m.Table == "" {
		m.Table = naming.Snake(naming.Plural(name)) // Bun's default table name
	}
	return m
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
