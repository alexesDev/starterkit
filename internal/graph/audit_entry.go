package graph

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

const (
	maxDetail       = 4000
	maxActionFields = 8
)

// adminAction names the admin fields actually selected, so the value in
// audit_log.action and in the Prometheus label is the schema field rather than
// whatever the client called its operation.
func adminAction(ctx context.Context, operation ast.Operation) string {
	kind := "query"
	if operation == ast.Mutation {
		kind = "mutation"
	}

	fields := selectedFieldNames(ctx)
	if len(fields) == 0 {
		return kind
	}

	return kind + ":" + strings.Join(fields, ",")
}

// selectedFieldNames is deduplicated, sorted and capped: aliases let a caller
// select the same field any number of times, and the result becomes a metric
// label value, so it must not be attacker-shaped.
func selectedFieldNames(ctx context.Context) []string {
	fieldCtx := graphql.GetFieldContext(ctx)
	if fieldCtx == nil {
		return nil
	}

	seen := map[string]bool{}

	var names []string

	for _, selected := range graphql.CollectFields(graphql.GetOperationContext(ctx), fieldCtx.Field.SelectionSet, nil) {
		if strings.HasPrefix(selected.Name, "__") || seen[selected.Name] {
			continue
		}

		seen[selected.Name] = true

		names = append(names, selected.Name)
	}

	slices.Sort(names)

	if len(names) > maxActionFields {
		names = names[:maxActionFields]
	}

	return names
}

// auditVariables renders the operation's variables as JSON. Nothing in this
// schema carries a secret; the first field that does must be redacted before it
// reaches the audit detail. See docs/graphql.md.
func auditVariables(ctx context.Context) string {
	opCtx := graphql.GetOperationContext(ctx)
	if opCtx == nil || opCtx.Operation == nil || len(opCtx.Variables) == 0 {
		return ""
	}

	encoded, err := json.Marshal(opCtx.Variables)
	if err != nil {
		return ""
	}

	return truncate(string(encoded))
}

// truncate cuts on a rune boundary, so the column never receives invalid UTF-8.
func truncate(s string) string {
	if len(s) <= maxDetail {
		return s
	}

	cut := maxDetail
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}

	return s[:cut] + "..."
}
