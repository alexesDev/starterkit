// Package logger is the narrow logging port every use case sees. zerolog is
// behind it: JSON lines to stderr, or a console format in dev mode.
package logger

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"starterkit/internal/confread"
)

type Config struct {
	Level zerolog.Level
}

func DefaultConfig() Config {
	return Config{Level: zerolog.InfoLevel}
}

func (c *Config) Register(r *confread.Reader) {
	*c = DefaultConfig()

	setLevel := func(raw string) error {
		level, err := zerolog.ParseLevel(raw)
		if err != nil || raw == "" {
			return errors.New("must be debug, info, warn or error")
		}

		c.Level = level

		return nil
	}

	r.Func("log-level", c.Level.String(), "debug|info|warn|error", setLevel)
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

type zeroLogger struct {
	log zerolog.Logger
}

func New(config Config, pretty bool) Logger {
	var out io.Writer = os.Stderr
	if pretty {
		out = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}
	}

	return build(out, config)
}

func build(out io.Writer, config Config) Logger {
	log := zerolog.New(out).Level(config.Level).With().Timestamp().Logger()

	return &zeroLogger{log: log}
}

func (l *zeroLogger) Debug(msg string, args ...any) {
	l.log.Debug().Fields(args).Msg(msg)
}

func (l *zeroLogger) Info(msg string, args ...any) {
	l.log.Info().Fields(args).Msg(msg)
}

func (l *zeroLogger) Warn(msg string, args ...any) {
	l.log.Warn().Fields(args).Msg(msg)
}

func (l *zeroLogger) Error(msg string, args ...any) {
	l.log.Error().Fields(args).Msg(msg)
}

func (l *zeroLogger) With(args ...any) Logger {
	return &zeroLogger{log: l.log.With().Fields(args).Logger()}
}

func Discard() Logger {
	return &zeroLogger{log: zerolog.Nop()}
}
