package preflight

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
)

func literalResolver(_ context.Context, v apiv2.ValueOrSecret) (string, error) {
	return v.Value, nil
}

func TestParseSkipList(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		want       map[string]bool
	}{
		{name: "empty runs everything", annotation: "", want: map[string]bool{}},
		{name: "single check", annotation: ExternalMysqlCheck, want: map[string]bool{ExternalMysqlCheck: true}},
		{name: "trims whitespace and drops empty entries", annotation: " externalMysqlCheck , other ,,", want: map[string]bool{ExternalMysqlCheck: true, "other": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseSkipList(tt.annotation); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSkipList(%q) = %v, want %v", tt.annotation, got, tt.want)
			}
		})
	}
}

func TestRegistryConsistency(t *testing.T) {
	for name, check := range Checks {
		if check.Name != name {
			t.Errorf("Checks[%q].Name = %q", name, check.Name)
		}
		if check.CRField == "" {
			t.Errorf("Checks[%q].CRField is empty", name)
		}
		if check.NewSpec == nil || check.Run == nil {
			t.Errorf("Checks[%q] is missing NewSpec or Run", name)
		}
	}
}

func TestCheckRejectsWrongSpecType(t *testing.T) {
	result := Checks[ExternalMysqlCheck].Run(context.Background(), &apiv2.RedisConnection{}, literalResolver)
	if result.Outcome != OutcomeUnknown {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeUnknown)
	}
}

func TestRunExternalMysqlCheck_MissingHost(t *testing.T) {
	result := RunExternalMysqlCheck(context.Background(), &apiv2.MysqlConnection{}, literalResolver)
	if result.Outcome != OutcomeFail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFail)
	}
}

func TestRunExternalMysqlCheck_MissingPort(t *testing.T) {
	conn := &apiv2.MysqlConnection{Host: apiv2.ValueOrSecret{Value: "db.example.com"}}
	result := RunExternalMysqlCheck(context.Background(), conn, literalResolver)
	if result.Outcome != OutcomeFail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFail)
	}
}

func TestRunExternalMysqlCheck_ResolveError(t *testing.T) {
	failing := func(context.Context, apiv2.ValueOrSecret) (string, error) {
		return "", errors.New("secret not found")
	}
	result := RunExternalMysqlCheck(context.Background(), &apiv2.MysqlConnection{}, failing)
	if result.Outcome != OutcomeUnknown {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeUnknown)
	}
}

func TestRunExternalMysqlCheck_Unreachable(t *testing.T) {
	// Grab a free port then release it so the dial is refused immediately.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(l.Addr().String())
	_ = l.Close()

	conn := &apiv2.MysqlConnection{
		Host:     apiv2.ValueOrSecret{Value: "127.0.0.1"},
		Port:     apiv2.ValueOrSecret{Value: port},
		Username: apiv2.ValueOrSecret{Value: "wandb"},
		Password: apiv2.ValueOrSecret{Value: "secret"},
	}
	result := Checks[ExternalMysqlCheck].Run(context.Background(), conn, literalResolver)
	if result.Outcome != OutcomeFail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFail)
	}
	if result.Check != ExternalMysqlCheck {
		t.Errorf("Check = %q, want %q", result.Check, ExternalMysqlCheck)
	}
}

func TestMysqlTLSConfig(t *testing.T) {
	tests := []struct {
		name                string
		mode, ca, cert, key string
		wantNil, wantErr    bool
	}{
		{name: "no certificate material leaves tls to the driver", mode: "true", wantNil: true},
		{name: "tls disabled ignores certificate material", mode: "false", ca: "not-pem", wantNil: true},
		{name: "invalid CA", ca: "not-pem", wantErr: true},
		{name: "invalid client key pair", cert: "not-pem", key: "not-pem", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := mysqlTLSConfig("db.example.com", tt.mode, tt.ca, tt.cert, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (cfg == nil) != tt.wantNil {
				t.Fatalf("cfg = %v, wantNil %v", cfg, tt.wantNil)
			}
		})
	}
}

func TestRunExternalMysqlCheck_InvalidCA(t *testing.T) {
	conn := &apiv2.MysqlConnection{
		Host:  apiv2.ValueOrSecret{Value: "db.example.com"},
		Port:  apiv2.ValueOrSecret{Value: "3306"},
		SslCa: apiv2.ValueOrSecret{Value: "not-pem"},
	}
	result := RunExternalMysqlCheck(context.Background(), conn, literalResolver)
	if result.Outcome != OutcomeFail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFail)
	}
}

func TestRedact(t *testing.T) {
	if got := redact("auth failed for hunter2", "hunter2"); got != "auth failed for ***" {
		t.Errorf("redact = %q", got)
	}
	if got := redact("auth failed", ""); got != "auth failed" {
		t.Errorf("redact with empty secret = %q", got)
	}
}
