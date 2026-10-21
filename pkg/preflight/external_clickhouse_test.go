package preflight

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
)

// The app gets url verbatim, so a ClickHouse behind a proxy path with settings in the query must be probed there.
func TestRunExternalClickHouseCheck_URLWithProxyPath(t *testing.T) {
	var gotUser, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clickhouse" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("database") != "wandb" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "Code: 81. DB::Exception: Database default is not the one expected. (UNKNOWN_DATABASE)")
			return
		}
		gotUser, gotKey = r.Header.Get("X-ClickHouse-User"), r.Header.Get("X-ClickHouse-Key")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "version()") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, "25.8.1\n")
	}))
	defer srv.Close()

	u := strings.Replace(srv.URL, "http://", "http://wandb:pw@", 1) + "/clickhouse?database=wandb"
	conn := &apiv2.ClickHouseConnection{URL: apiv2.ValueOrSecret{Value: u}}

	got := RunExternalClickHouseCheck(context.Background(), conn, Env{Resolve: literalResolver})
	if got.Outcome != OutcomePass {
		t.Fatalf("got %s %q, want pass", got.Outcome, got.Message)
	}
	if gotUser != "wandb" || gotKey != "pw" {
		t.Errorf("credentials from the url = (%q, %q), want (wandb, pw) sent as headers", gotUser, gotKey)
	}
}

func TestClickHouseError(t *testing.T) {
	body := "Code: 516. DB::Exception: wandb: Authentication failed: password is incorrect. (AUTHENTICATION_FAILED) (version 25.8.1.1)"
	if got := clickHouseError(http.StatusUnauthorized, body).Error(); got != "HTTP 401: code 516 (AUTHENTICATION_FAILED)" {
		t.Errorf("ClickHouse body = %q", got)
	}
	// Anything that isn't a ClickHouse error, e.g. an internal service reached through a user-set url, stays out of status.
	if got := clickHouseError(http.StatusOK, `{"token":"secret"}`).Error(); got != "HTTP 200" {
		t.Errorf("foreign body = %q, want only the status", got)
	}
}
