package graph

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"starterkit/internal/logger"
	"starterkit/internal/model"
)

func errorPresenter(log logger.Logger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		if errors.Is(err, model.ErrNotAdmin) {
			return gqlerror.WrapPath(graphql.GetPath(ctx), model.ErrNotAdmin)
		}

		var gqlErr *gqlerror.Error
		if errors.As(err, &gqlErr) && errors.Unwrap(gqlErr) == nil {
			return graphql.DefaultErrorPresenter(ctx, err)
		}

		ref := errorRef()
		log.Error("resolver failed", "error", err, "ref", ref, "path", graphql.GetPath(ctx))

		return gqlerror.WrapPath(graphql.GetPath(ctx), errors.New("internal error, ref "+ref))
	}
}

func errorRef() string {
	buf := make([]byte, 6)

	_, err := rand.Read(buf)
	if err != nil {
		return "unavailable"
	}

	return hex.EncodeToString(buf)
}
