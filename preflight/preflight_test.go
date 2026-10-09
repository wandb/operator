package preflight

import (
	"context"
	"net"
	"reflect"
	"testing"
)

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
		if check.Run == nil {
			t.Errorf("Checks[%q].Run is nil", name)
		}
	}
}

func TestRunExternalDBCheck_MissingHost(t *testing.T) {
	result := RunExternalMysqlCheck(context.Background(), map[string]string{})
	if result.Outcome != OutcomeFail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFail)
	}
}

func TestRunExternalDBCheck_Unreachable(t *testing.T) {
	// Grab a free port then release it so the dial is refused immediately.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(l.Addr().String())
	_ = l.Close()

	result := RunExternalMysqlCheck(context.Background(), map[string]string{
		ParamHost:     "127.0.0.1",
		ParamPort:     port,
		ParamUsername: "wandb",
		ParamPassword: "secret",
	})
	if result.Outcome != OutcomeFail {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, OutcomeFail)
	}
	if result.Check != ExternalMysqlCheck {
		t.Errorf("Check = %q, want %q", result.Check, ExternalMysqlCheck)
	}
}
