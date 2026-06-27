// Package log provides zerolog initialization with traceId in context,
// mirroring Java's logback-spring.xml + TraceIdFilter. See SPEC §4, §16.
package log

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level               string
	LogHome             string
	File                string
	MaxSizeMB           int
	MaxAgeDays          int
	TotalSizeCapGB      int
	CleanHistoryOnStart bool
	PrettyConsole       bool
}

var logger zerolog.Logger

type ctxKey struct{}

// Init builds a multi-writer logger (console + rolling file) and stores it as
// the package-level logger used by FromContext.
func Init(cfg Config) zerolog.Logger {
	var writers []io.Writer
	if cfg.PrettyConsole {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"})
	} else {
		writers = append(writers, os.Stdout)
	}
	if cfg.File != "" {
		_ = os.MkdirAll(cfg.LogHome, 0o755)
		lj := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.LogHome, cfg.File),
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxAgeDays,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   true,
			LocalTime:  true,
		}
		// NOTE: logback cleanHistoryOnStart (purge stale logs at boot) is not
		// directly supported by lumberjack; MaxAge purges on rotation instead.
		// Acceptable for Phase 1.
		writers = append(writers, lj)
	}
	lvl := zerolog.InfoLevel
	if l, err := zerolog.ParseLevel(strings.ToLower(cfg.Level)); err == nil {
		lvl = l
	}
	logger = zerolog.New(zerolog.MultiLevelWriter(writers...)).Level(lvl).With().Timestamp().Logger()
	return logger
}

// WithTraceID returns ctx carrying the traceId.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, traceID)
}

// FromContext returns a logger enriched with traceId (or "NONE" if absent),
// mirroring logback's [%X{traceId:-NONE}].
func FromContext(ctx context.Context) zerolog.Logger {
	if tid, ok := ctx.Value(ctxKey{}).(string); ok && tid != "" {
		return logger.With().Str("traceId", tid).Logger()
	}
	return logger.With().Str("traceId", "NONE").Logger()
}
