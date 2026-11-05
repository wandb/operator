package redact

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestValues(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "DSN in env map",
			in: map[string]interface{}{"parquet": map[string]interface{}{"env": map[string]interface{}{
				"GORILLA_HISTORY_STORAGE_ENGINE_ADDRESS": "clickhouse://wandb:hunter2@clickhouse:9000/db",
				"GORILLA_DEBUG":                          "true",
			}}},
			want: map[string]interface{}{"parquet": map[string]interface{}{"env": map[string]interface{}{
				"GORILLA_HISTORY_STORAGE_ENGINE_ADDRESS": Mask,
				"GORILLA_DEBUG":                          "true",
			}}},
		},
		{
			name: "env list entries",
			in: map[string]interface{}{"extraEnv": []interface{}{
				map[string]interface{}{"name": "MYSQL_PASSWORD", "value": "hunter2"},
				map[string]interface{}{"name": "BUCKET_ADDRESS", "value": "s3://AKIA:hunter2@bucket"},
				map[string]interface{}{"name": "LOG_LEVEL", "value": "debug"},
			}},
			want: map[string]interface{}{"extraEnv": []interface{}{
				map[string]interface{}{"name": "MYSQL_PASSWORD", "value": Mask},
				map[string]interface{}{"name": "BUCKET_ADDRESS", "value": Mask},
				map[string]interface{}{"name": "LOG_LEVEL", "value": "debug"},
			}},
		},
		{
			name: "nested lists",
			in: map[string]interface{}{"a": []interface{}{
				[]interface{}{map[string]interface{}{"password": "hunter2", "user": "wandb"}},
				"redis://:hunter2@redis:6379",
			}},
			want: map[string]interface{}{"a": []interface{}{
				[]interface{}{map[string]interface{}{"password": Mask, "user": "wandb"}},
				"redis://***@redis:6379",
			}},
		},
		{
			name: "sensitive key names ignore case and separators",
			in: map[string]interface{}{
				"Password": "a", "DB_PASSWORD": "b", "secretKey": "c", "X-Api-Key": "d",
				"accessKey": "e", "license": "f", "SENTRY_DSN": "g", "connectionString": "h",
				"GITHUB_TOKEN": "i", "credentials": map[string]interface{}{"user": "j"},
			},
			want: map[string]interface{}{
				"Password": Mask, "DB_PASSWORD": Mask, "secretKey": Mask, "X-Api-Key": Mask,
				"accessKey": Mask, "license": Mask, "SENTRY_DSN": Mask, "connectionString": Mask,
				"GITHUB_TOKEN": Mask, "credentials": map[string]interface{}{"user": Mask},
			},
		},
		{
			name: "URL userinfo under an arbitrary key",
			in: map[string]interface{}{
				"engine":   "clickhouse://wandb:hunter2@clickhouse:9000",
				"unescape": "mysql://wandb:hun@ter/2@mysql:3306/wandb",
				"repo":     "https://ghp_abc@github.com/wandb/operator",
				"notes":    "primary mysql://u:hunter2@db:3306 replica mysql://u:hunter2@db2:3306",
			},
			want: map[string]interface{}{
				"engine":   "clickhouse://***@clickhouse:9000",
				"unescape": "mysql://***@mysql:3306/wandb",
				"repo":     "https://***@github.com/wandb/operator",
				"notes":    "primary mysql://***@db:3306 replica mysql://***@db2:3306",
			},
		},
		{
			name: "non-sensitive values preserved",
			in: map[string]interface{}{"global": map[string]interface{}{
				"host":     "https://wandb.example.com",
				"replicas": 3,
				"enabled":  true,
				"unset":    nil,
				"password": "",
				"image":    map[string]interface{}{"repository": "wandb/local", "tag": "0.70.0"},
				"chart":    "oci://registry.local:5000/charts/wandb@sha256:abc",
				"source":   "https://github.com/wandb/operator@v1",
			}},
			want: map[string]interface{}{"global": map[string]interface{}{
				"host":     "https://wandb.example.com",
				"replicas": json.Number("3"),
				"enabled":  true,
				"unset":    nil,
				"password": "",
				"image":    map[string]interface{}{"repository": "wandb/local", "tag": "0.70.0"},
				"chart":    "oci://registry.local:5000/charts/wandb@sha256:abc",
				"source":   "https://github.com/wandb/operator@v1",
			}},
		},
		{
			name: "numbers under sensitive keys",
			in:   map[string]interface{}{"mysql": map[string]interface{}{"password": 123456, "port": 3306}},
			want: map[string]interface{}{"mysql": map[string]interface{}{"password": Mask, "port": json.Number("3306")}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Values(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Values() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestValuesDoesNotMutateInput(t *testing.T) {
	in := map[string]interface{}{
		"password": "hunter2",
		"env":      []interface{}{map[string]interface{}{"name": "API_TOKEN", "value": "hunter2"}},
		"url":      "redis://:hunter2@redis:6379",
	}
	before := runtime.DeepCopyJSON(in)

	out := Values(in).(map[string]interface{})
	out["env"].([]interface{})[0].(map[string]interface{})["value"] = "changed"

	if !reflect.DeepEqual(in, before) {
		t.Fatalf("Values() mutated its input: %#v", in)
	}
}

func TestValuesFailsClosed(t *testing.T) {
	if got := Values(map[string]interface{}{"f": func() {}}); got != Mask {
		t.Fatalf("Values() = %#v, want %q", got, Mask)
	}
}

func TestString(t *testing.T) {
	tests := []struct{ in, want string }{
		{"--db-password=hunter2 --verbose", "--db-password=*** --verbose"},
		{"password: hunter2\nhost: db", "password: ***\nhost: db"},
		{`{"apiKey":"hunter2"}`, `{"apiKey":"***"}`},
		{"Authorization=Bearer hunter2", "Authorization=***"},
		{"https://api.example.com/v1?token=hunter2&page=2", "https://api.example.com/v1?token=***&page=2"},
		{"license eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ3YW5kYiJ9.c2lnbmF0dXJl", "license ***"},
		{"-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----", Mask},
		{"https://charts.wandb.ai", "https://charts.wandb.ai"},
	}
	for _, tt := range tests {
		if got := String(tt.in); got != tt.want {
			t.Errorf("String(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestError(t *testing.T) {
	cause := errors.New("dial failed")
	err := Error(fmt.Errorf("pull oci://robot:hunter2@registry/charts/wandb: %w", cause))
	if got, want := err.Error(), "pull oci://***@registry/charts/wandb: dial failed"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Fatal("Error() does not unwrap to its cause")
	}
	if Error(nil) != nil {
		t.Fatal("Error(nil) != nil")
	}
}
