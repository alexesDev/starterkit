package graph

import (
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/ast"

	"starterkit/internal/graph/generated"
)

const complexityLimit = 60

type Introspection bool

type configurable interface {
	Use(extension graphql.HandlerExtension)
	SetQueryCache(cache graphql.Cache[*ast.QueryDocument])
	SetErrorPresenter(f graphql.ErrorPresenterFunc)
}

func newExecutableSchema(env Env) graphql.ExecutableSchema {
	resolver := Resolver{Env: env}

	return generated.NewExecutableSchema(generated.Config{Resolvers: &resolver})
}

func configure(target configurable, env Env, introspection Introspection) {
	target.SetQueryCache(lru.New[*ast.QueryDocument](200))

	if introspection {
		target.Use(extension.Introspection{})
	}

	target.Use(extension.FixedComplexityLimit(complexityLimit))
	target.SetErrorPresenter(errorPresenter(env.Logger()))
}

func NewHandler(env Env, introspection Introspection) *handler.Server {
	srv := handler.New(newExecutableSchema(env))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})
	configure(srv, env, introspection)

	return srv
}
