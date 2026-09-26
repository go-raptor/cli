package resource

import (
	"fmt"
	"maps"
	"slices"
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
			if col := idx.usersColumn(m); col != "" {
				return Chain{}, fmt.Errorf("model %s is not owned by a user the generator can follow: %s references users through %s, but the ownership chain needs a user_id column", m.Name, m.Name, col)
			}
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

// isUsers reports whether m is the users table itself.
func isUsers(m *Model) bool { return m.Name == "User" || m.Table == "users" }

// usersColumn returns a column of m that references users (user_id, or any column a
// belongs-to relation or its name points at the users table), or "".
func (idx ModelIndex) usersColumn(m *Model) string {
	if isUsers(m) {
		return ""
	}
	if m.HasColumn("user_id") {
		return "user_id"
	}
	for _, f := range m.Fields {
		if target, ok := idx.FKTarget(m, f.Column); ok && isUsers(target) {
			return f.Column
		}
	}
	for _, col := range slices.Sorted(maps.Keys(m.BelongsTo)) {
		if target, ok := idx[m.BelongsTo[col]]; m.BelongsTo[col] == "User" || ok && isUsers(target) {
			return col
		}
	}
	return ""
}

// CheckRefTarget refuses a ref target unless it is provably shared reference data. A ref gets no
// ownership check, so the gate fails closed. It refuses a model that is owned, one with a column
// that references users under any name, one that reaches such a model through its foreign keys,
// and one with an embedded struct or a belongs-to relation the index cannot read. The users
// table itself stays allowed (ref:User).
func (idx ModelIndex) CheckRefTarget(name string) error {
	target, ok := idx[name]
	if !ok {
		return fmt.Errorf("model %s not found in app/models", name)
	}
	if isUsers(target) {
		return nil
	}
	if idx.IsOwned(name) {
		if _, err := idx.OwnerChain(name); err == nil {
			return fmt.Errorf("%s is owned by a user; use --parent %s, or the ownership check would be skipped", name, name)
		}
		return fmt.Errorf("%s is owned by a user, and a ref would skip the ownership check", name)
	}
	type step struct {
		m   *Model
		via []string // the foreign keys followed from the target
	}
	queue, seen := []step{{target, nil}}, map[string]bool{name: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		m, reason := cur.m, ""
		if col := idx.usersColumn(m); col != "" {
			reason = fmt.Sprintf("%s references users through %s", m.Name, col)
		} else if len(m.Unresolved) > 0 {
			reason = fmt.Sprintf("%s embeds %s, which is not declared in app/models, so its columns are unknown", m.Name, strings.Join(m.Unresolved, ", "))
		} else {
			for _, col := range slices.Sorted(maps.Keys(m.BelongsTo)) {
				if _, ok := idx[m.BelongsTo[col]]; !ok {
					reason = fmt.Sprintf("%s has a belongs-to relation to %s, which is not a model in app/models", m.Name, m.BelongsTo[col])
					break
				}
			}
		}
		if reason != "" {
			if len(cur.via) > 0 {
				reason = fmt.Sprintf("%s reaches %s through %s, and %s", name, m.Name, strings.Join(cur.via, " → "), reason)
			}
			return fmt.Errorf("%s; a ref gets no ownership check, so it only takes shared reference data", reason)
		}
		for _, f := range m.Fields {
			if next, ok := idx.FKTarget(m, f.Column); ok && !seen[next.Name] {
				seen[next.Name] = true
				queue = append(queue, step{next, append(slices.Clone(cur.via), f.Column)})
			}
		}
		for _, col := range slices.Sorted(maps.Keys(m.BelongsTo)) {
			if next, ok := idx[m.BelongsTo[col]]; ok && !seen[next.Name] {
				seen[next.Name] = true
				queue = append(queue, step{next, append(slices.Clone(cur.via), col)})
			}
		}
	}
	return nil
}
