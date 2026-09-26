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
	Promoted   []seedFieldView
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
// foreign key, and a literal by Go type otherwise. Pointer fields stay nil. A field promoted from
// an embedded struct can't go in the composite literal, so the seed assigns it afterwards. A type
// it has no sample for, or a field behind an embedded pointer, is an error, which skips the
// integration tests rather than guessing.
func buildSeedModel(idx ModelIndex, m *Model, module string) (*seedModelView, error) {
	v := naming.Var(m.Name)
	s := &seedModelView{Module: module, Name: m.Name, Var: localName(v, v+"Item", "userID"), Owned: idx.IsOwned(m.Name)}
	for i, f := range m.Fields {
		expr, err := seedExpr(idx, m, f, s)
		if err != nil {
			return nil, err
		}
		switch {
		case expr == "":
		case f.ViaPointer != "":
			return nil, fmt.Errorf("cannot seed %s.%s: it comes from the embedded pointer %s, which the seed would have to allocate", m.Name, f.GoName, f.ViaPointer)
		case f.Embed != "":
			if err := selectable(m, i); err != nil {
				return nil, err
			}
			s.Promoted = append(s.Promoted, seedFieldView{f.Selector, expr})
		default:
			s.Fields = append(s.Fields, seedFieldView{f.GoName, expr})
		}
	}
	return s, nil
}

// selectable checks that the promoted field m.Fields[i] can be set as model.Selector from
// another package: no other field of that name at its depth or above, and no embedded struct the
// index can't read, which might declare one.
func selectable(m *Model, i int) error {
	f := m.Fields[i]
	if len(m.Unresolved) > 0 {
		return fmt.Errorf("cannot seed %s.%s: %s also embeds %s, which the generator cannot read and which might declare %s too", m.Name, f.GoName, m.Name, strings.Join(m.Unresolved, ", "), f.GoName)
	}
	scope := strings.TrimSuffix(f.Selector, f.GoName)
	for j, g := range m.Fields {
		if j != i && g.GoName == f.GoName && strings.TrimSuffix(g.Selector, g.GoName) == scope && g.Depth <= f.Depth {
			return fmt.Errorf("cannot seed %s.%s: more than one field is named %s at that depth, or a shallower one hides it, so Go cannot select it", m.Name, f.GoName, f.GoName)
		}
	}
	return nil
}

// seedExpr is the sample value for one field, or "" to leave it at its zero value.
func seedExpr(idx ModelIndex, m *Model, f ModelField, s *seedModelView) (string, error) {
	switch f.Column {
	case "", "id", "created_at", "updated_at":
		return "", nil
	case "user_id":
		return "userID", nil
	}
	if strings.HasPrefix(f.GoType, "*") {
		return "", nil
	}
	if target, ok := idx.FKTarget(m, f.Column); ok {
		if idx.IsOwned(target.Name) {
			return "seed" + target.Name + "(t, userID).ID", nil
		}
		return "seed" + target.Name + "(t).ID", nil
	}
	switch {
	case f.GoType == "string":
		s.UsesSuffix = true
		return fmt.Sprintf("%q + suffix", f.GoName+" "), nil
	case numericTypes[f.GoType]:
		return "1", nil
	case f.GoType == "bool":
		return "true", nil
	case f.GoType == "time.Time":
		s.UsesTime = true
		return "time.Now()", nil
	}
	return "", fmt.Errorf("cannot seed %s.%s: the generator has no sample value for type %s", m.Name, f.GoName, f.GoType)
}
