package resource

import (
	"fmt"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

// seedModelView renders a seed function for a model the generator didn't write: a parent or a
// ref target that a generated test needs rows of.
type seedModelView struct {
	Module     string
	Name, Var  string
	Owned      bool // the seed takes userID
	Fields     []seedFieldView
	UsesTime   bool
	UsesSuffix bool
}

type seedFieldView struct{ GoName, Expr string }

var numericTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

// buildSeedModel picks a sample value for every column: userID for the owner, another seed for a
// foreign key, and a literal by Go type otherwise. Pointer fields stay nil. A type it has no
// sample for is an error, which skips the integration tests rather than guessing.
func buildSeedModel(idx ModelIndex, m *Model, module string) (*seedModelView, error) {
	s := &seedModelView{Module: module, Name: m.Name, Var: naming.Var(m.Name), Owned: idx.IsOwned(m.Name)}
	for _, f := range m.Fields {
		switch f.Column {
		case "", "id", "created_at", "updated_at":
			continue
		case "user_id":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "userID"})
			continue
		}
		if strings.HasPrefix(f.GoType, "*") {
			continue
		}
		if target, ok := idx.FKTarget(m, f.Column); ok {
			expr := "seed" + target.Name + "(t).ID"
			if idx.IsOwned(target.Name) {
				expr = "seed" + target.Name + "(t, userID).ID"
			}
			s.Fields = append(s.Fields, seedFieldView{f.GoName, expr})
			continue
		}
		switch {
		case f.GoType == "string":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, fmt.Sprintf("%q + suffix", f.GoName+" ")})
			s.UsesSuffix = true
		case numericTypes[f.GoType]:
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "1"})
		case f.GoType == "bool":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "true"})
		case f.GoType == "time.Time":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "time.Now()"})
			s.UsesTime = true
		default:
			return nil, fmt.Errorf("cannot seed %s.%s: the generator has no sample value for type %s", m.Name, f.GoName, f.GoType)
		}
	}
	return s, nil
}
