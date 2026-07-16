package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want slog.Level
	}{
		{name: "default", want: slog.LevelInfo},
		{name: "debug", raw: "debug", want: slog.LevelDebug},
		{name: "info", raw: "INFO", want: slog.LevelInfo},
		{name: "warn", raw: "warn", want: slog.LevelWarn},
		{name: "warning alias", raw: "warning", want: slog.LevelWarn},
		{name: "error", raw: " error ", want: slog.LevelError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseLogLevel(test.raw)
			if err != nil {
				t.Fatalf("parseLogLevel(%q): %v", test.raw, err)
			}
			if got != test.want {
				t.Fatalf("parseLogLevel(%q) = %s, want %s", test.raw, got, test.want)
			}
		})
	}
}

func TestParseLogLevelRejectsUnknownValue(t *testing.T) {
	if _, err := parseLogLevel("verbose"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestLoggerOmitsQueryString(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	router := chi.NewRouter()
	router.Use(requestLogger)
	router.Get("/auth/verify", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/auth/verify?token=super-secret", nil)
	router.ServeHTTP(httptest.NewRecorder(), request)

	logged := output.String()
	if !strings.Contains(logged, `"path":"/auth/verify"`) {
		t.Fatalf("log = %q, want request path", logged)
	}
	if strings.Contains(logged, "super-secret") || strings.Contains(logged, "token=") {
		t.Fatalf("log contains query string: %q", logged)
	}
}
