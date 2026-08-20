package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestRedactsSecretTaggedFields verifies that a struct field tagged
// `masq:"secret"` is redacted in the log output while non-secret fields survive.
func TestRedactsSecretTaggedFields(t *testing.T) {
	type conn struct {
		Host     string
		Password string `masq:"secret"`
	}

	var buf bytes.Buffer
	h := NewHandler(&Options{
		HandlerOptions: &slog.HandlerOptions{},
		Output:         &buf,
		Format:         JsonFormat,
	}, "")
	slog.New(h).Info("connection", slog.Any("conn", conn{Host: "db.example.com", Password: "sup3rs3cret"}))

	out := buf.String()
	if strings.Contains(out, "sup3rs3cret") {
		t.Fatalf("secret leaked into log output: %s", out)
	}
	if !strings.Contains(out, "db.example.com") {
		t.Fatalf("non-secret value should be present: %s", out)
	}
}
