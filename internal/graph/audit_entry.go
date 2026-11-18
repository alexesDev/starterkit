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
