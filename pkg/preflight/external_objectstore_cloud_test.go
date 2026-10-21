package preflight

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
)

func testRSAKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestCheckGCSBucket(t *testing.T) {
	keyPEM := testRSAKeyPEM(t)
	tests := []struct {
		name        string
		tokenStatus int
		listStatus  int
		bucket      string
		want        Outcome
		contains    string
	}{
		{name: "pass", tokenStatus: http.StatusOK, listStatus: http.StatusOK, bucket: "wandb/runs", want: OutcomePass},
		{name: "token rejected", tokenStatus: http.StatusBadRequest, bucket: "wandb", want: OutcomeFail, contains: "invalid_grant"},
		{name: "no list permission", tokenStatus: http.StatusOK, listStatus: http.StatusForbidden, bucket: "wandb", want: OutcomeFail, contains: "storage.objects.list"},
		{name: "missing bucket", tokenStatus: http.StatusOK, listStatus: http.StatusNotFound, bucket: "wandb", want: OutcomeFail, contains: "does not exist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPrefix string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.tokenStatus)
					if tt.tokenStatus != http.StatusOK {
						_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"Invalid JWT Signature."}`)
						return
					}
					_, _ = io.WriteString(w, `{"access_token":"tok","token_type":"Bearer","expires_in":3600}`)
				case "/storage/v1/b/wandb/o":
					if r.Header.Get("Authorization") != "Bearer tok" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					gotPrefix = r.URL.Query().Get("prefix")
					w.WriteHeader(tt.listStatus)
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			}))
			defer srv.Close()
			origToken, origStorage := gcsTokenURL, gcsStorageURL
			gcsTokenURL, gcsStorageURL = srv.URL+"/token", srv.URL
			defer func() { gcsTokenURL, gcsStorageURL = origToken, origStorage }()

			conn := &apiv2.ObjectStoreConnection{
				Provider:  apiv2.ValueOrSecret{Value: string(apiv2.ObjectStoreProviderGCS)},
				AccessKey: apiv2.ValueOrSecret{Value: "wandb@project.iam.gserviceaccount.com"},
				SecretKey: apiv2.ValueOrSecret{Value: keyPEM},
				Bucket:    apiv2.ValueOrSecret{Value: tt.bucket},
				Path:      apiv2.ValueOrSecret{Value: "/files/"},
			}
			got := RunExternalObjectStoreCheck(context.Background(), conn, Env{Resolve: literalResolver})
			if got.Outcome != tt.want || !strings.Contains(got.Message, tt.contains) {
				t.Errorf("got %s %q, want %s containing %q", got.Outcome, got.Message, tt.want, tt.contains)
			}
			if strings.Contains(got.Message, "PRIVATE KEY") {
				t.Errorf("message leaks the key: %q", got.Message)
			}
			// The bucket's own prefix and spec.path are joined, as in external/objectstore.buildGCSURL.
			if tt.want == OutcomePass && gotPrefix != "runs/files" {
				t.Errorf("prefix = %q, want runs/files", gotPrefix)
			}
		})
	}
}

func TestCheckGCSBucket_InputValidation(t *testing.T) {
	gcs := func(accessKey, secretKey string) *apiv2.ObjectStoreConnection {
		return &apiv2.ObjectStoreConnection{
			Provider:  apiv2.ValueOrSecret{Value: string(apiv2.ObjectStoreProviderGCS)},
			AccessKey: apiv2.ValueOrSecret{Value: accessKey},
			SecretKey: apiv2.ValueOrSecret{Value: secretKey},
			Bucket:    apiv2.ValueOrSecret{Value: "wandb"},
		}
	}
	env := Env{Resolve: literalResolver}
	if got := RunExternalObjectStoreCheck(context.Background(), gcs("", ""), env); got.Outcome != OutcomeSkipped {
		t.Errorf("workload identity: %s, want skipped", got.Outcome)
	}
	if got := RunExternalObjectStoreCheck(context.Background(), gcs("", "key"), env); got.Outcome != OutcomeFail {
		t.Errorf("key without email: %s, want fail", got.Outcome)
	}
}

func TestCheckAzureContainer(t *testing.T) {
	accountKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	tests := []struct {
		name      string
		key       string
		container string
		want      Outcome
		contains  string
	}{
		{name: "pass", key: accountKey, container: "wandb/prefix", want: OutcomePass},
		{name: "wrong key", key: base64.StdEncoding.EncodeToString([]byte("wrong")), container: "wandb", want: OutcomeFail, contains: "AuthenticationFailed"},
		{name: "missing container", key: accountKey, container: "other", want: OutcomeFail, contains: "does not exist"},
		{name: "key not base64", key: "not base64!", container: "wandb", want: OutcomeFail, contains: "base64"},
		{name: "workload identity", container: "wandb", want: OutcomeSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key, _ := base64.StdEncoding.DecodeString(accountKey)
				mac := hmac.New(sha256.New, key)
				_, _ = mac.Write([]byte(azureStringToSign(r, "acct")))
				want := "SharedKey acct:" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
				switch {
				case r.Method != http.MethodHead || r.URL.Query().Get("restype") != "container":
					w.WriteHeader(http.StatusBadRequest)
				case r.Header.Get("Authorization") != want:
					w.Header().Set("X-Ms-Error-Code", "AuthenticationFailed")
					w.WriteHeader(http.StatusForbidden)
				case r.URL.Path != "/wandb":
					w.Header().Set("X-Ms-Error-Code", "ContainerNotFound")
					w.WriteHeader(http.StatusNotFound)
				default:
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer srv.Close()
			orig := azureBlobURL
			azureBlobURL = func(_, container, _ string) string { return srv.URL + "/" + container }
			defer func() { azureBlobURL = orig }()

			conn := &apiv2.ObjectStoreConnection{
				Provider:  apiv2.ValueOrSecret{Value: string(apiv2.ObjectStoreProviderAzure)},
				AccessKey: apiv2.ValueOrSecret{Value: "acct"},
				SecretKey: apiv2.ValueOrSecret{Value: tt.key},
				Bucket:    apiv2.ValueOrSecret{Value: tt.container},
			}
			got := RunExternalObjectStoreCheck(context.Background(), conn, Env{Resolve: literalResolver})
			if got.Outcome != tt.want || !strings.Contains(got.Message, tt.contains) {
				t.Errorf("got %s %q, want %s containing %q", got.Outcome, got.Message, tt.want, tt.contains)
			}
		})
	}
}

// TestAzureStringToSign pins the Shared Key layout by hand, so the format isn't only checked against itself.
func TestAzureStringToSign(t *testing.T) {
	req, err := http.NewRequest(http.MethodHead, "https://acct.blob.core.windows.net/wandb?restype=container", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-ms-version", "2021-08-06")
	req.Header.Set("x-ms-date", "Tue, 21 Oct 2026 10:00:00 GMT")

	want := "HEAD\n" + strings.Repeat("\n", 11) +
		"x-ms-date:Tue, 21 Oct 2026 10:00:00 GMT\n" +
		"x-ms-version:2021-08-06\n" +
		"/acct/wandb\nrestype:container"
	if got := azureStringToSign(req, "acct"); got != want {
		t.Errorf("string to sign:\n%q\nwant:\n%q", got, want)
	}
}

func TestJoinKeyPrefix(t *testing.T) {
	tests := []struct{ base, prefix, want string }{
		{"", "", ""},
		{"runs", "", "runs"},
		{"", "/files/", "files"},
		{"runs/", "/files", "runs/files"},
	}
	for _, tt := range tests {
		if got := joinKeyPrefix(tt.base, tt.prefix); got != tt.want {
			t.Errorf("joinKeyPrefix(%q, %q) = %q, want %q", tt.base, tt.prefix, got, tt.want)
		}
	}
}
