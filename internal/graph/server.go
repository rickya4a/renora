package graph

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/renora/renora-api/internal/apperr"
	"github.com/renora/renora-api/internal/config"
	"github.com/renora/renora-api/internal/graph/generated"
)

type ServerOptions struct {
	Config   config.GraphQL
	Logger   *slog.Logger
	APQCache graphql.Cache[string] // nil disables automatic persisted queries
}

// NewHandler builds the /graphql handler with production limits applied.
func NewHandler(r *Resolver, opts ServerOptions) http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))

	// Uploads go straight to object storage through presigned URLs, so there is
	// no multipart transport. WebSocket subscriptions are added with their first use.
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	if *opts.Config.Introspection {
		srv.Use(extension.Introspection{})
	}
	if opts.APQCache != nil {
		srv.Use(extension.AutomaticPersistedQuery{Cache: opts.APQCache})
	}
	srv.Use(extension.FixedComplexityLimit(opts.Config.ComplexityLimit))
	srv.Use(DepthLimit{Max: opts.Config.DepthLimit})

	srv.SetErrorPresenter(errorPresenter(opts.Logger))
	srv.SetRecoverFunc(func(ctx context.Context, v any) error {
		opts.Logger.ErrorContext(ctx, "graphql resolver panic", "panic", v, "stack", string(debug.Stack()))
		return errors.New("internal panic")
	})
	return srv
}

// errorPresenter shows *apperr.Error values to the client with their code,
// passes GraphQL's own validation errors through, and masks everything else.
func errorPresenter(logger *slog.Logger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		if ae, ok := apperr.As(err); ok {
			gqlErr := graphql.DefaultErrorPresenter(ctx, err)
			gqlErr.Message = ae.Message
			gqlErr.Extensions = map[string]any{"code": string(ae.Code)}
			if len(ae.Field) > 0 {
				gqlErr.Extensions["field"] = ae.Field
			}
			if ae.Code == apperr.CodeInternal {
				logger.ErrorContext(ctx, "graphql internal error", "error", err, "path", gqlErr.Path.String())
			}
			return gqlErr
		}

		var gqlErr *gqlerror.Error
		if errors.As(err, &gqlErr) && gqlErr.Err == nil {
			return graphql.DefaultErrorPresenter(ctx, err)
		}

		masked := graphql.DefaultErrorPresenter(ctx, err)
		logger.ErrorContext(ctx, "graphql internal error", "error", err, "path", masked.Path.String())
		masked.Message = "internal server error"
		masked.Extensions = map[string]any{"code": string(apperr.CodeInternal)}
		return masked
	}
}
