package main

import (
	"log/slog"
	"testing"
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
