package buildsys

import (
	"context"

	"github.com/rs/zerolog"
)

type logKey struct{}

func log(ctx context.Context) *zerolog.Logger {
	logger := ctx.Value(logKey{})
	if logger == nil {
		panic("logger is missing in context")
	}

	zlogger, ok := logger.(*zerolog.Logger)
	if !ok {
		panic("logger has wrong type")
	}

	return zlogger
}

// WithLogger attaches the given logger to the context
func WithLogger(ctx context.Context, logger *zerolog.Logger) context.Context {
	return context.WithValue(ctx, logKey{}, logger)
}
