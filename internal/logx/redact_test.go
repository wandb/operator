package logx

import (
	"bytes"
	"fmt"
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

// TestRedactRunsBeforeCallerReplaceAttr verifies redaction happens before any
// caller-provided ReplaceAttr. A caller transform that flattens the struct to a
// string strips the type masq reflects on, so if it ran first the secret would
// be serialized as plain text and could never be redacted.
func TestRedactRunsBeforeCallerReplaceAttr(t *testing.T) {
	type conn struct {
		Host     string
		Password string `masq:"secret"`
	}

	stringify := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "conn" {
			return slog.String(a.Key, fmt.Sprintf("%+v", a.Value.Any()))
		}
		return a
	}

	var buf bytes.Buffer
	h := NewHandler(&Options{
		HandlerOptions: &slog.HandlerOptions{ReplaceAttr: stringify},
		Output:         &buf,
		Format:         JsonFormat,
	}, "")
	slog.New(h).Info("connection", slog.Any("conn", conn{Host: "db.example.com", Password: "sup3rs3cret"}))

	out := buf.String()
	if strings.Contains(out, "sup3rs3cret") {
		t.Fatalf("secret leaked past caller ReplaceAttr: %s", out)
	}
	if !strings.Contains(out, "db.example.com") {
		t.Fatalf("non-secret value should survive: %s", out)
	}
}
