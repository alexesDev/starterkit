package graph

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/ast"

	"starterkit/internal/graph/generated"
)

func NewHandler(env Env) *handler.Server {
	resolver := Resolver{DefaultEnv: env}
	executable := generated.NewExecutableSchema(generated.Config{Resolvers: &resolver})
	srv := handler.New(executable)

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](200))
	srv.Use(extension.Introspection{})
	srv.Use(extension.FixedComplexityLimit(60))

	srv.SetErrorPresenter(errorPresenter(env.Logger()))

	return srv
}
