package deployer

import (
	"github.com/wandb/operator/pkg/wandb/spec"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestDeployerClient_GetSpec(t *testing.T) {
	type fields struct {
		testServer func(license string) *httptest.Server
	}
	type args struct {
		license     string
		activeState *spec.Spec
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *spec.Spec
		wantErr bool
	}{
		{
			"Test the HTTP request has expected headers and returns 200",
			fields{
				testServer: func(license string) *httptest.Server {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Content-Type") != "application/json" {
							t.Errorf("Expected Content-Type: application/json header, got: %s", r.Header.Get("Accept"))
						}
						if username, _, _ := r.BasicAuth(); username != license {
							t.Errorf("Expected BasicAuth to match %s, got: %s", license, username)
						}
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{}`))
					}))
					return server
				},
			},
			args{license: "license", activeState: &spec.Spec{}},
			&spec.Spec{},
			false,
		}, {
			"Test the HTTP request fails repeatedly",
			fields{
				testServer: func(license string) *httptest.Server {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusBadGateway)
						_, _ = w.Write([]byte(`{}`))
					}))
					return server
				},
			},
			args{license: "license", activeState: &spec.Spec{}},
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.fields.testServer(tt.args.license)
			defer server.Close()
			c := &DeployerClient{
				DeployerChannelUrl: server.URL,
			}
			got, err := c.GetSpec(tt.args.license, tt.args.activeState)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != "all retries failed" {
				t.Errorf("GetSpec() error = %v, expected %v", err, "all retries failed")
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSpec() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeployerClient_getURL(t *testing.T) {
	type fields struct {
		DeployerChannelUrl string
		HttpClient         *http.Client
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"No Deployer Channel URL provided", fields{"", nil}, DeployerAPI},
		{"User Provided Deployer Channel URL", fields{"https://test-url.example.com", nil}, "https://test-url.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &DeployerClient{
				DeployerChannelUrl: tt.fields.DeployerChannelUrl,
			}
			if got := c.getURL(); got != tt.want {
				t.Errorf("getURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
