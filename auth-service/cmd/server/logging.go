package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func configureLogging(service string) error {
	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", service)
	slog.SetDefault(logger)
	// Existing log.Printf calls report failures. Route them through the
	// structured logger as errors while they are migrated incrementally.
	slog.SetLogLoggerLevel(slog.LevelError)
	slog.Info("logging configured", "level", level.String())
	return nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, or error")
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		started := time.Now()

		defer func() {
			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
			}

			level := slog.LevelInfo
			switch {
			case r.URL.Path == "/healthz":
				level = slog.LevelDebug
			case status >= http.StatusInternalServerError:
				level = slog.LevelError
			case status >= http.StatusBadRequest:
				level = slog.LevelWarn
			}
			path := chi.RouteContext(r.Context()).RoutePattern()
			if path == "" {
				path = "<unmatched>"
			}

			slog.LogAttrs(r.Context(), level, "request completed",
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", path),
				slog.Int("status", status),
				slog.Int("bytes", wrapped.BytesWritten()),
				slog.Duration("duration", time.Since(started)),
			)
		}()

		next.ServeHTTP(wrapped, r)
	})
}
