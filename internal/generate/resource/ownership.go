package resource

import (
	"fmt"
	"strings"
)

// Chain is the path from a model up to the row that carries user_id.
type Chain struct {
	Models []*Model // the model itself first, the root-owned model last
	FKs    []string // FKs[i] is the column on Models[i] that points at Models[i+1]
}

func (c Chain) Root() *Model { return c.Models[len(c.Models)-1] }

// Predicate is the ownership condition for a child table whose parentFK column points at the
// chain's first model, in the form patterns.md uses: a subquery when that model carries
// user_id itself, and a JOIN chain when it is deeper.
func (c Chain) Predicate(childTable, parentFK string) string {
	parent := c.Models[0]
	if len(c.Models) == 1 {
		return fmt.Sprintf("%s.%s IN (SELECT id FROM %s WHERE user_id = ?)", childTable, parentFK, parent.Table)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s.%s IN (\n\tSELECT %s.id FROM %s\n", childTable, parentFK, parent.Table, parent.Table)
	for i := 1; i < len(c.Models); i++ {
		prev, cur := c.Models[i-1], c.Models[i]
		fmt.Fprintf(&b, "\tJOIN %s ON %s.id = %s.%s\n", cur.Table, cur.Table, prev.Table, c.FKs[i-1])
	}
	fmt.Fprintf(&b, "\tWHERE %s.user_id = ?\n)", c.Root().Table)
	return b.String()
}

// IsOwned reports whether rows of the named model belong to a user, directly through a user_id
// column or through a parent.
func (idx ModelIndex) IsOwned(name string) bool {
	m, ok := idx[name]
	return ok && idx.owned(m, map[string]bool{})
}

func (idx ModelIndex) owned(m *Model, visiting map[string]bool) bool {
	if m.HasColumn("user_id") {
		return true
	}
	if visiting[m.Name] {
		return false
	}
	visiting[m.Name] = true
	defer delete(visiting, m.Name)
	return len(idx.ownedParents(m, visiting)) > 0
}

type fk struct {
	column string
	target *Model
}

func (idx ModelIndex) ownedParents(m *Model, visiting map[string]bool) []fk {
	var out []fk
	for _, f := range m.Fields {
		if f.Column == "" {
			continue
		}
		if target, ok := idx.FKTarget(m, f.Column); ok && target.Name != m.Name && idx.owned(target, visiting) {
			out = append(out, fk{f.Column, target})
		}
	}
	return out
}

// OwnerChain walks from the named model up to the model with a user_id column. It fails when the
// model is not owned, or when a model on the way has more than one owned parent.
func (idx ModelIndex) OwnerChain(name string) (Chain, error) {
	m, ok := idx[name]
	if !ok {
		return Chain{}, fmt.Errorf("model %s not found in app/models", name)
	}
	var c Chain
	seen := map[string]bool{}
	for {
		if seen[m.Name] {
			return Chain{}, fmt.Errorf("the ownership chain of %s loops back to %s", name, m.Name)
		}
		seen[m.Name] = true
		c.Models = append(c.Models, m)
		if m.HasColumn("user_id") {
			return c, nil
		}
		parents := idx.ownedParents(m, map[string]bool{m.Name: true})
		switch len(parents) {
		case 0:
			return Chain{}, fmt.Errorf("model %s is not owned by a user: it has no user_id column and no foreign key to an owned model", m.Name)
		case 1:
		default:
			cols := make([]string, len(parents))
			for i, p := range parents {
				cols[i] = p.column
			}
			return Chain{}, fmt.Errorf("model %s has more than one owned parent (%s); the generator cannot choose one", m.Name, strings.Join(cols, ", "))
		}
		c.FKs = append(c.FKs, parents[0].column)
		m = parents[0].target
	}
}
