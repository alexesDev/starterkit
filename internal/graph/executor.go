package graph

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"

	"starterkit/internal/appreq"
)

type Request struct {
	Query         string
	OperationName string
	Variables     map[string]any
}

type Executor struct {
	exec    *executor.Executor
	surface surface
}

func NewExecutor(env Env, introspection Introspection) *Executor {
	schema := newExecutableSchema(env)
	exec := executor.New(schema)
	configure(exec, env, introspection)

	return &Executor{exec: exec, surface: newSurface(schema.Schema())}
}

func (e *Executor) ExecuteGraphQL(ctx context.Context, request Request) *graphql.Response {
	ctx = graphql.StartOperationTrace(ctx)
	start := graphql.Now()

	params := &graphql.RawParams{
		Query:         request.Query,
		OperationName: request.OperationName,
		Variables:     request.Variables,
		ReadTime:      graphql.TraceTiming{Start: start, End: start},
	}

	opCtx, opErr := e.exec.CreateOperationContext(ctx, params)
	if opErr != nil {
		return e.exec.DispatchError(graphql.WithOperationContext(ctx, opCtx), opErr)
	}

	responses, respCtx := e.exec.DispatchOperation(ctx, opCtx)

	return responses(respCtx)
}

func (e *Executor) GraphQLSurface(ctx context.Context) *SchemaView {
	req, err := appreq.FromCtx(ctx)
	if err == nil && req.IsAdmin() {
		return e.surface.admin
	}

	return e.surface.user
}
