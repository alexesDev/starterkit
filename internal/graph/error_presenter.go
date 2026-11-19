package graph

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"starterkit/internal/db"
	"starterkit/internal/logger"
	"starterkit/internal/model"
	"starterkit/internal/page"
)

func errorPresenter(log logger.Logger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		if errors.Is(err, model.ErrNotAdmin) {
			return gqlerror.WrapPath(graphql.GetPath(ctx), model.ErrNotAdmin)
		}

		if errors.Is(err, page.ErrInvalidCursor) {
			return gqlerror.WrapPath(graphql.GetPath(ctx), page.ErrInvalidCursor)
		}

		var gqlErr *gqlerror.Error
		if errors.As(err, &gqlErr) && errors.Unwrap(gqlErr) == nil {
			return graphql.DefaultErrorPresenter(ctx, err)
		}

		if isMetaField(ctx) {
			return gqlerror.WrapPath(graphql.GetPath(ctx), err)
		}

		ref := errorRef()

		if db.IsBusy(err) {
			log.Warn("resolver found the database busy", "error", err, "ref", ref, "path", graphql.GetPath(ctx))

			busy := gqlerror.WrapPath(graphql.GetPath(ctx), errors.New("the database is busy, retry, ref "+ref))
			busy.Extensions = map[string]any{"code": codeRetry}

			return busy
		}

		log.Error("resolver failed", "error", err, "ref", ref, "path", graphql.GetPath(ctx))

		return gqlerror.WrapPath(graphql.GetPath(ctx), errors.New("internal error, ref "+ref))
	}
}

const codeRetry = "RETRY"

func isMetaField(ctx context.Context) bool {
	field := graphql.GetFieldContext(ctx)

	return field != nil && strings.HasPrefix(field.Field.Name, "__")
}

func errorRef() string {
	buf := make([]byte, 6)

	_, err := rand.Read(buf)
	if err != nil {
		return "unavailable"
	}

	return hex.EncodeToString(buf)
}
