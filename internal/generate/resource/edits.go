package resource

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

func importsPath(file *ast.File, path string) bool {
	for _, imp := range file.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == path {
			return true
		}
	}
	return false
}

type insertion struct {
	at   int
	text string
}

// insertAll applies insertions from the end backwards, so earlier offsets stay valid, then
// gofmt's the result.
func insertAll(src string, ins []insertion) (string, error) {
	sort.Slice(ins, func(i, j int) bool { return ins[i].at > ins[j].at })
	for _, in := range ins {
		src = src[:in.at] + in.text + src[in.at:]
	}
	out, err := format.Source([]byte(src))
	return string(out), err
}

// addValidationSchema adds <name>Schema to ValidationService: a field, and its assignment in Setup
// just before the final return.
func addValidationSchema(src, module, name string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	if !importsPath(file, "github.com/Oudwins/zog") || !importsPath(file, module+"/app/models") {
		return "", errors.New("the file does not import zog and the models package")
	}
	var fieldsEnd, setupReturn token.Pos
	recv := ""
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == "ValidationService" {
					if st, ok := ts.Type.(*ast.StructType); ok {
						fieldsEnd = st.Fields.Closing
					}
				}
			}
		case *ast.FuncDecl:
			if decl.Name.Name != "Setup" || decl.Recv == nil || decl.Body == nil || len(decl.Recv.List[0].Names) == 0 ||
				strings.TrimPrefix(types.ExprString(decl.Recv.List[0].Type), "*") != "ValidationService" {
				continue
			}
			if n := len(decl.Body.List); n > 0 {
				if ret, ok := decl.Body.List[n-1].(*ast.ReturnStmt); ok {
					setupReturn, recv = ret.Pos(), decl.Recv.List[0].Names[0].Name
				}
			}
		}
	}
	if !fieldsEnd.IsValid() || !setupReturn.IsValid() {
		return "", errors.New("no ValidationService struct with a Setup method that ends in a return")
	}
	return insertAll(src, []insertion{
		{fset.Position(fieldsEnd).Offset, fmt.Sprintf("\t%sSchema *zog.StructSchema\n", name)},
		{fset.Position(setupReturn).Offset, fmt.Sprintf("%s.%sSchema = models.%sSchema()\n\t", recv, name, name)},
	})
}

// addSchemaTest adds the zero-value Validate call to TestSchemasMatchTheirRequests.
func addSchemaTest(src, module, name string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	if !importsPath(file, module+"/app/models") {
		return "", errors.New("the file does not import the models package")
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "TestSchemasMatchTheirRequests" && fn.Body != nil {
			return insertAll(src, []insertion{{fset.Position(fn.Body.Rbrace).Offset,
				fmt.Sprintf("\tmodels.%sSchema().Validate(&models.%sRequest{})\n", name, name)}})
		}
	}
	return "", errors.New("no TestSchemasMatchTheirRequests function")
}

// routesBlock is the flat CRUD block of SKILL.md → Routes, at the given indentation.
func routesBlock(indent, step int, route, controller string) []string {
	pad := func(level int) string { return strings.Repeat(" ", indent+level*step) }
	return []string{
		pad(0) + "/" + route + ":",
		pad(1) + "GET: " + controller + ".Index",
		pad(1) + "POST: " + controller + ".Create",
		pad(1) + "/{id}:",
		pad(2) + "GET: " + controller + ".Show",
		pad(2) + "PUT: " + controller + ".Update",
		pad(2) + "DELETE: " + controller + ".Destroy",
	}
}

func leadingSpaces(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// addRoutes appends the resource's block to the end of the /api/v1: mapping, matching the file's
// indentation, and refuses anything it cannot do safely. The result must parse as YAML.
func addRoutes(src, route, controller string) (string, error) {
	if strings.Contains(src, "\r") {
		return "", errors.New("routes.yaml uses CRLF line endings")
	}
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	api := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "/api/v1:" {
			api = i
			break
		}
	}
	if api == -1 {
		return "", errors.New("no /api/v1: key in routes.yaml")
	}
	indent, child, last := leadingSpaces(lines[api]), -1, api
	for i := api + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		ind := leadingSpaces(lines[i])
		if ind <= indent {
			break
		}
		if child == -1 {
			child = ind
		}
		if ind == child && trimmed == "/"+route+":" {
			return "", fmt.Errorf("the route /%s already exists under /api/v1", route)
		}
		last = i
	}
	if child == -1 {
		child = indent + 2
	}
	block := routesBlock(child, child-indent, route, controller)
	out := append(append(append([]string{}, lines[:last+1]...), block...), lines[last+1:]...)
	result := strings.Join(out, "\n") + "\n"
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(result), &parsed); err != nil {
		return "", fmt.Errorf("the edited routes.yaml would not parse: %w", err)
	}
	return result, nil
}

// addAppSetting adds a commented key: "value" to the end of a config file's app: mapping,
// matching the file's indentation, and adds the mapping when there is none. A key the file
// already sets is the project's decision, so the file comes back unchanged. The result must
// parse as YAML.
func addAppSetting(src, comment, key, value string) (string, error) {
	if strings.Contains(src, "\r") {
		return "", errors.New("the config file uses CRLF line endings")
	}
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	indent := 2
	for _, l := range lines {
		if trimmed := strings.TrimSpace(l); trimmed != "" && !strings.HasPrefix(trimmed, "#") && leadingSpaces(l) > 0 {
			indent = leadingSpaces(l)
			break
		}
	}
	app := -1
	for i, l := range lines {
		rest, ok := strings.CutPrefix(l, "app:")
		if !ok {
			continue
		}
		if rest = strings.TrimSpace(rest); rest != "" && !strings.HasPrefix(rest, "#") {
			return "", errors.New("app: is not a block mapping")
		}
		app = i
		break
	}
	setting := func(indent int) []string {
		pad := strings.Repeat(" ", indent)
		return []string{pad + "# " + comment, pad + key + ": " + strconv.Quote(value)}
	}
	var out []string
	if app == -1 {
		out = append(append(lines, "", "app:"), setting(indent)...)
	} else {
		child, last := -1, app
		for i := app + 1; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			ind := leadingSpaces(lines[i])
			if ind == 0 {
				break
			}
			if child == -1 {
				child = ind
			}
			if ind == child && strings.HasPrefix(trimmed, key+":") {
				return src, nil
			}
			last = i
		}
		if child == -1 {
			child = indent
		}
		out = append(append(append([]string{}, lines[:last+1]...), setting(child)...), lines[last+1:]...)
	}
	result := strings.Join(out, "\n") + "\n"
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(result), &parsed); err != nil {
		return "", fmt.Errorf("the edited config file would not parse: %w", err)
	}
	return result, nil
}
