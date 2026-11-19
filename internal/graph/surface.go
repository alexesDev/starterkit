package graph

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"

	"starterkit/internal/textsearch"
)

const (
	adminField      = "admin"
	maxTypesPerCall = 5
	maxGrepTypes    = 15
	maxHitsPerType  = 10
)

var (
	ErrUnknownType  = errors.New("unknown type")
	ErrTooManyTypes = fmt.Errorf("at most %d types per call", maxTypesPerCall)
)

type surface struct {
	admin *SchemaView
	user  *SchemaView
}

type SchemaView struct {
	schema *ast.Schema
	names  []string
	sdl    map[string][]string
	paths  map[string]string
}

func newSurface(schema *ast.Schema) surface {
	return surface{
		admin: NewSchemaView(schema),
		user:  NewSchemaView(schema, adminField),
	}
}

func NewSchemaView(full *ast.Schema, hiddenRootFields ...string) *SchemaView {
	hidden := map[string]bool{}
	for _, name := range hiddenRootFields {
		hidden[name] = true
	}

	schema := visibleSchema(full, hidden)
	view := &SchemaView{schema: schema, sdl: map[string][]string{}}

	for name, definition := range schema.Types {
		single := &ast.Schema{Types: map[string]*ast.Definition{name: definition}, Directives: map[string]*ast.DirectiveDefinition{}}

		var out strings.Builder

		formatter.NewFormatter(&out, formatter.WithIndent("  ")).FormatSchema(single)

		view.names = append(view.names, name)
		view.sdl[name] = strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	}

	slices.Sort(view.names)

	view.paths = pathsFromRoots(schema)

	return view
}

func pathsFromRoots(schema *ast.Schema) map[string]string {
	paths := map[string]string{}
	queue := make([]string, 0, len(schema.Types))

	for _, root := range []*ast.Definition{schema.Query, schema.Mutation} {
		paths[root.Name] = root.Name
		queue = append(queue, root.Name)
	}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		for _, field := range schema.Types[name].Fields {
			step := paths[name] + "." + field.Name
			reached := []struct{ name, path string }{{field.Type.Name(), step}}

			for _, argument := range field.Arguments {
				reached = append(reached, struct{ name, path string }{argument.Type.Name(), step + "(" + argument.Name + ")"})
			}

			for _, next := range reached {
				_, seen := paths[next.name]
				_, visible := schema.Types[next.name]

				if seen || !visible {
					continue
				}

				paths[next.name] = next.path
				queue = append(queue, next.name)
			}
		}
	}

	return paths
}

func (v *SchemaView) header(name string) string {
	path, ok := v.paths[name]
	if !ok || path == name {
		return "## " + name
	}

	return "## " + name + " (reached by " + path + ")"
}

func (v *SchemaView) EntryPoints() string {
	var out strings.Builder

	for _, root := range []*ast.Definition{v.schema.Query, v.schema.Mutation} {
		fmt.Fprintf(&out, "%s (fields: %d)\n", root.Name, len(root.Fields))

		for _, field := range root.Fields {
			namespace := v.schema.Types[field.Type.Name()]
			if len(field.Arguments) > 0 || namespace == nil || namespace.Kind != ast.Object {
				continue
			}

			fmt.Fprintf(&out, "  %s.%s -> %s (fields: %d)\n", root.Name, field.Name, namespace.Name, len(namespace.Fields))
		}
	}

	return out.String()
}

func (v *SchemaView) Types(names []string) (string, error) {
	if len(names) > maxTypesPerCall {
		return "", ErrTooManyTypes
	}

	parts := make([]string, 0, len(names))

	for _, name := range names {
		lines, ok := v.sdl[name]
		if !ok {
			return "", fmt.Errorf("%w %q: search with query, or call graphql_introspection() for the entry points", ErrUnknownType, name)
		}

		parts = append(parts, strings.Join(lines, "\n"))
	}

	return strings.Join(parts, "\n\n"), nil
}

func (v *SchemaView) Grep(matcher textsearch.Matcher, before, after int) string {
	var out strings.Builder

	first := 1
	matched := 0

	for _, name := range v.names {
		lines := v.sdl[name]
		window := textsearch.Window{Before: before, After: after, Hits: maxHitsPerType, FirstLine: first}
		first += len(lines)

		excerpt := textsearch.Cut(lines, matcher, window)
		if excerpt.Hits == 0 {
			continue
		}

		matched++

		if matched > maxGrepTypes {
			continue
		}

		fmt.Fprintf(&out, "%s\n%s", v.header(name), excerpt.Text)

		if excerpt.Omitted > 0 {
			fmt.Fprintf(&out, "... %d more in %s\n", excerpt.Omitted, name)
		}
	}

	if matched == 0 {
		return "No line matches. Try other words, query_op=\"or\", or graphql_introspection() for the entry points."
	}

	if matched > maxGrepTypes {
		fmt.Fprintf(&out, "... %d more types match; narrow the query.\n", matched-maxGrepTypes)
	}

	return out.String()
}

func visibleSchema(schema *ast.Schema, hiddenRootFields map[string]bool) *ast.Schema {
	visible := &ast.Schema{
		Types:      map[string]*ast.Definition{},
		Directives: map[string]*ast.DirectiveDefinition{},
	}

	var pending []string

	visible.Query = addDefinition(visible, schema.Query, hiddenRootFields, &pending)
	visible.Mutation = addDefinition(visible, schema.Mutation, hiddenRootFields, &pending)

	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		_, seen := visible.Types[name]
		definition := schema.Types[name]

		if seen || definition.BuiltIn {
			continue
		}

		addDefinition(visible, definition, nil, &pending)
	}

	return visible
}

func addDefinition(visible *ast.Schema, definition *ast.Definition, hiddenFields map[string]bool, pending *[]string) *ast.Definition {
	stripped := stripDefinition(definition, hiddenFields)
	visible.Types[definition.Name] = stripped
	*pending = append(*pending, referencedTypes(stripped)...)

	return stripped
}

func stripDefinition(definition *ast.Definition, hiddenFields map[string]bool) *ast.Definition {
	stripped := *definition
	stripped.Directives = nil
	stripped.Fields = nil

	for _, field := range definition.Fields {
		if hiddenFields[field.Name] || strings.HasPrefix(field.Name, "__") {
			continue
		}

		fieldCopy := *field
		fieldCopy.Directives = nil
		stripped.Fields = append(stripped.Fields, &fieldCopy)
	}

	return &stripped
}

func referencedTypes(definition *ast.Definition) []string {
	names := append([]string{}, definition.Types...)
	names = append(names, definition.Interfaces...)

	for _, field := range definition.Fields {
		names = append(names, field.Type.Name())

		for _, argument := range field.Arguments {
			names = append(names, argument.Type.Name())
		}
	}

	return names
}

var (
	quotedName   = regexp.MustCompile(`"([A-Za-z_][A-Za-z0-9_]*)(?:\.[A-Za-z_][A-Za-z0-9_]*)?"`)
	missingField = regexp.MustCompile(`Cannot query field "([A-Za-z0-9_]+)" on type "([A-Za-z0-9_]+)"`)
	missingArg   = regexp.MustCompile(`Unknown argument "([A-Za-z0-9_]+)" on field "([A-Za-z0-9_]+)\.([A-Za-z0-9_]+)"`)
	missingType  = regexp.MustCompile(`Unknown type "([A-Za-z0-9_]+)"`)
)

const (
	maxSuggestions  = 3
	maxEditDistance = 3
)

func (v *SchemaView) Explain(messages []string) string {
	var types []string

	var hints []string

	for _, message := range messages {
		for _, match := range quotedName.FindAllStringSubmatch(message, -1) {
			_, ok := v.sdl[match[1]]
			if ok && !slices.Contains(types, match[1]) {
				types = append(types, match[1])
			}
		}

		hints = append(hints, v.suggest(message)...)
	}

	if len(types) > maxTypesPerCall {
		types = types[:maxTypesPerCall]
	}

	var out strings.Builder

	for _, hint := range hints {
		out.WriteString(hint + "\n")
	}

	if len(types) > 0 {
		definitions, _ := v.Types(types)
		out.WriteString("The schema around this error:\n" + definitions + "\n")
	}

	return out.String()
}

func (v *SchemaView) suggest(message string) []string {
	var hints []string

	if found := missingField.FindStringSubmatch(message); found != nil {
		hints = append(hints, nearestHint(found[1], found[2], v.fieldNames(found[2])))
	}

	if found := missingArg.FindStringSubmatch(message); found != nil {
		hints = append(hints, nearestHint(found[1], found[2]+"."+found[3], v.argumentNames(found[2], found[3])))
	}

	if found := missingType.FindStringSubmatch(message); found != nil {
		hints = append(hints, nearestHint(found[1], "the schema", v.names))
	}

	return hints
}

func (v *SchemaView) fieldNames(typeName string) []string {
	definition, ok := v.schema.Types[typeName]
	if !ok {
		return nil
	}

	names := make([]string, 0, len(definition.Fields))
	for _, field := range definition.Fields {
		names = append(names, field.Name)
	}

	return names
}

func (v *SchemaView) argumentNames(typeName, fieldName string) []string {
	definition, ok := v.schema.Types[typeName]
	if !ok {
		return nil
	}

	field := definition.Fields.ForName(fieldName)
	if field == nil {
		return nil
	}

	names := make([]string, 0, len(field.Arguments))
	for _, argument := range field.Arguments {
		names = append(names, argument.Name)
	}

	return names
}

func nearestHint(wrong, where string, candidates []string) string {
	type scored struct {
		name     string
		distance int
	}

	var near []scored

	for _, candidate := range candidates {
		distance := levenshtein.ComputeDistance(strings.ToLower(wrong), strings.ToLower(candidate))
		if distance <= maxEditDistance {
			near = append(near, scored{name: candidate, distance: distance})
		}
	}

	if len(near) == 0 {
		return fmt.Sprintf("%q is not in %s, and no name there is spelled like it.", wrong, where)
	}

	slices.SortStableFunc(near, func(a, b scored) int {
		return a.distance - b.distance
	})

	names := make([]string, 0, maxSuggestions)
	for _, found := range near[:min(len(near), maxSuggestions)] {
		names = append(names, found.name)
	}

	return fmt.Sprintf("%q is not in %s. Nearest by spelling: %s.", wrong, where, strings.Join(names, ", "))
}
