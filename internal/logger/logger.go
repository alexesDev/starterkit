// Package logger is the narrow logging port every use case sees.
package logger

import (
	"errors"
	"log/slog"
	"os"

	"starterkit/internal/confread"
)

type Config struct {
	Level slog.Level
}

func (c *Config) Register(r *confread.Reader) {
	*c = Config{Level: slog.LevelInfo}

	setLevel := func(raw string) error {
		err := c.Level.UnmarshalText([]byte(raw))
		if err != nil {
			return errors.New("must be debug, info, warn or error")
		}

		return nil
	}

	r.Func("log-level", "info", "debug|info|warn|error", setLevel)
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

type slogLogger struct {
	log *slog.Logger
}

func New(config Config, pretty bool) Logger {
	opts := &slog.HandlerOptions{Level: config.Level}

	var handler slog.Handler
	if pretty {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}

	return &slogLogger{log: slog.New(handler)}
}

func (l *slogLogger) Debug(msg string, args ...any) {
	l.log.Debug(msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.log.Info(msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.log.Warn(msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.log.Error(msg, args...)
}

func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{log: l.log.With(args...)}
}

func Discard() Logger {
	return &slogLogger{log: slog.New(slog.DiscardHandler)}
}
