package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/gregperez/hachikapp-backend/internal/infra/logging"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"nope":    slog.LevelInfo,
	}
	for in, want := range cases {
		if got := logging.ParseLevel(in); got != want {
			t.Fatalf("ParseLevel(%q)=%v want %v", in, got, want)
		}
	}
}

func TestJSONHandlerEmitsAttrs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	lg.Info("http",
		"request_id", "rid-1",
		"status", 200,
		"method", "GET",
		"path", "/v1/users",
		"latency_ms", 3,
	)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json: %v body=%s", err, buf.String())
	}
	if m["msg"] != "http" {
		t.Fatalf("msg=%v", m["msg"])
	}
	if m["request_id"] != "rid-1" {
		t.Fatalf("request_id=%v", m["request_id"])
	}
	if m["status"] != float64(200) {
		t.Fatalf("status=%v", m["status"])
	}
	if m["method"] != "GET" || m["path"] != "/v1/users" {
		t.Fatalf("method/path=%v/%v", m["method"], m["path"])
	}
}

func TestOrDefault(t *testing.T) {
	t.Parallel()
	if logging.OrDefault(nil) != slog.Default() {
		t.Fatal("nil should fall back to Default")
	}
	custom := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	if logging.OrDefault(custom) != custom {
		t.Fatal("non-nil should be returned as-is")
	}
}
