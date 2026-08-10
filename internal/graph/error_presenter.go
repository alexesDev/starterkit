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

// errorPresenter decides what a caller is allowed to read.
//
// A resolver error wraps whatever it was given — a driver error naming a
// column, an HTTP failure naming a URL with a credential in its path — so the
// default is to log the cause and hand back a reference the caller can quote.
// Two things pass through unchanged, because they carry no information the
// caller did not already have: parser and validator errors, which arrive as a
// *gqlerror.Error wrapping nothing, and ErrNotAdmin, which says only that the
// boundary refused and which the admin namespace already recorded.
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
