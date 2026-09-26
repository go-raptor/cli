// Package components registers controllers and services in config/components.
package components

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const raptorImport = "github.com/go-raptor/raptor/v4"

// AddEntry adds &<pkg>.<structName>{} to the raptor.Controllers or raptor.Services literal in
// src, importing the package if needed. A component is already registered when the file holds
// that exact entry; then src comes back unchanged. The result is gofmt'd, and an edit that
// would not be valid Go is an error, so the caller can print the manual step instead.
func AddEntry(src, moduleName, kind, structName string) (string, error) {
	var pkg, list string
	switch kind {
	case "controller":
		pkg, list = "controllers", "Controllers"
	case "service":
		pkg, list = "services", "Services"
	default:
		return "", fmt.Errorf("unsupported component type for registration: %s", kind)
	}
	importPkg := moduleName + "/app/" + pkg

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return "", fmt.Errorf("the file does not parse: %w", err)
	}
	var lit *ast.CompositeLit
	registered := false
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CompositeLit:
			if lit == nil && isSelector(n.Type, "raptor", list) {
				lit = n
			}
		case *ast.UnaryExpr:
			if cl, ok := n.X.(*ast.CompositeLit); ok && n.Op == token.AND && isSelector(cl.Type, pkg, structName) {
				registered = true
			}
		}
		return true
	})
	if registered {
		return src, nil
	}
	if lit == nil {
		return "", fmt.Errorf("could not find raptor.%s{", list)
	}

	type insertion struct {
		at   int
		text string
	}
	offset := func(p token.Pos) int { return fset.Position(p).Offset }
	line := func(p token.Pos) int { return fset.Position(p).Line }
	entry := fmt.Sprintf("&%s.%s{}", pkg, structName)
	var ins []insertion
	switch n := len(lit.Elts); {
	case n > 0 && line(lit.Elts[n-1].End()) == line(lit.Rbrace): // the literal ends on its last entry's line
		ins = append(ins, insertion{offset(lit.Elts[n-1].End()), ", " + entry})
	case n == 0 && line(lit.Lbrace) == line(lit.Rbrace): // {}
		ins = append(ins, insertion{offset(lit.Rbrace), "\n" + entry + ",\n"})
	default: // a line of its own, before the closing brace's line
		at := offset(lit.Rbrace)
		ins = append(ins, insertion{strings.LastIndex(src[:at], "\n") + 1, "\t\t" + entry + ",\n"})
	}
	if !importsPath(file, importPkg) {
		at, text := -1, ""
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.IMPORT {
				continue
			}
			for _, spec := range gen.Specs {
				if p, err := strconv.Unquote(spec.(*ast.ImportSpec).Path.Value); err == nil && p == raptorImport {
					if gen.Lparen.IsValid() {
						at, text = offset(spec.End()), fmt.Sprintf("\n\t%q", importPkg)
					} else {
						at, text = offset(gen.End()), fmt.Sprintf("\nimport %q", importPkg)
					}
				}
			}
		}
		if at == -1 {
			return "", fmt.Errorf("could not find the raptor import")
		}
		ins = append(ins, insertion{at, text})
	}
	sort.Slice(ins, func(i, j int) bool { return ins[i].at > ins[j].at })
	out := src
	for _, in := range ins {
		out = out[:in.at] + in.text + out[in.at:]
	}
	formatted, err := format.Source([]byte(out))
	if err != nil {
		return "", fmt.Errorf("the edited file would not be valid Go: %w", err)
	}
	return string(formatted), nil
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg
}

func importsPath(file *ast.File, path string) bool {
	for _, imp := range file.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == path {
			return true
		}
	}
	return false
}

// Register adds structName to config/components/<kind>s.go.
func Register(moduleName, kind, structName string) error {
	regFile := filepath.Join("config", "components", kind+"s.go")
	content, err := os.ReadFile(regFile)
	if err != nil {
		return err
	}
	out, err := AddEntry(string(content), moduleName, kind, structName)
	if err != nil {
		return fmt.Errorf("%s: %w", regFile, err)
	}
	if out == string(content) {
		return nil
	}
	return os.WriteFile(regFile, []byte(out), 0644)
}
