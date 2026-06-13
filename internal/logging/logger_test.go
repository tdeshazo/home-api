package logging_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/tdeshazo/home-api/internal/logging"
)

func TestNewLoggerWritesBuildAndErrorAttributes(t *testing.T) {
	var buf bytes.Buffer

	logger, closeLogs, err := logging.New(logging.Options{
		Level:  slog.LevelDebug,
		Env:    "test",
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer closeLogs()

	logger.Error("operation failed", "error", logging.WithAttrs(errors.New("boom"), "task_id", "1"))

	line := buf.String()
	for _, want := range []string{
		`"git_sha"`,
		`"build_time"`,
		`"env":"test"`,
		`"msg":"operation failed"`,
		`"message":"boom"`,
		`"task_id":"1"`,
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected log to contain %s, got %s", want, line)
		}
	}
}

func TestAttrs(t *testing.T) {
	err := logging.WithAttrs(errors.New("boom"), "resource", "task", slog.Int("attempt", 2))
	attrs := logging.Attrs(err)

	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}
	if attrs[0].Key != "resource" || attrs[0].Value.String() != "task" {
		t.Fatalf("unexpected first attr: %#v", attrs[0])
	}
	if attrs[1].Key != "attempt" {
		t.Fatalf("unexpected second attr: %#v", attrs[1])
	}
}
