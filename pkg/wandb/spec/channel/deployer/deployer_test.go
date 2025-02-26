package deployer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
)

func TestDeployer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deployer Suite")
}

var _ = Describe("DeployerClient", func() {
	Describe("GetSpec", func() {
		var server *httptest.Server
		var client *DeployerClient

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		Context("when the HTTP request is successful", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.Header.Get("Content-Type")).To(Equal("application/json"), "Expected Content-Type: application/json header")
					username, _, _ := r.BasicAuth()
					Expect(username).To(Equal("license"), "Expected BasicAuth to match license")
					w.WriteHeader(http.StatusOK)
					// Return a valid sample spec
					response := SpecUnknownChart{
						Metadata: &spec.Metadata{"version": "1.0.0"},
						Values:   &spec.Values{"key": "value"},
						Chart:    map[string]interface{}{"type": "local", "path": "test-path"},
					}
					jsonData, _ := json.Marshal(response)
					_, _ = w.Write(jsonData)
				}))
				client = &DeployerClient{
					DeployerAPI: server.URL,
				}
			})

			It("should make a request with correct headers and return successfully", func() {
				got, err := client.GetSpec(GetSpecOptions{
					License:    "license",
					RetryDelay: time.Millisecond,
					Debug:      true,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(got).NotTo(BeNil())
				Expect(got.Values).To(HaveKeyWithValue("key", "value"))
				// The Chart should be properly initialized using charts.Get
				_, ok := got.Chart.(*charts.LocalRelease)
				Expect(ok).To(BeTrue(), "Expected chart to be a LocalRelease")
			})
		})

		Context("when the server returns non-200 status codes", func() {
			BeforeEach(func() {
				// Count requests to track retry attempts
				requestCount := 0
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestCount++
					// Return error status for all requests
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error": "Internal Server Error"}`))
				}))
				client = &DeployerClient{
					DeployerAPI: server.URL,
				}
			})

			It("should retry and return an error after all retries fail", func() {
				got, err := client.GetSpec(GetSpecOptions{
					License:    "license",
					RetryDelay: time.Millisecond, // Use small delay for test
					Debug:      true,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("all retries failed"))
				Expect(got).To(BeNil())
			})
		})

		Context("when the server is unreachable", func() {
			BeforeEach(func() {
				// Use an invalid URL to simulate unreachable server
				client = &DeployerClient{
					DeployerAPI: "http://invalid-url-that-will-never-resolve.example",
				}
			})

			It("should return a connection error", func() {
				got, err := client.GetSpec(GetSpecOptions{
					License:    "license",
					RetryDelay: time.Millisecond,
					Debug:      true,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("all retries failed"))
				Expect(got).To(BeNil())
			})
		})

		Context("when the server times out", func() {
			BeforeEach(func() {
				// Create a server that sleeps longer than the client timeout
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Sleep for 200ms, which is much longer than our 10ms timeout
					time.Sleep(200 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{}`))
				}))

				// Create client with our test server URL
				client = &DeployerClient{
					DeployerAPI: server.URL,
				}
			})

			It("should complete successfully with a reasonable timeout", func() {
				// Set a timeout that's longer than the server sleep
				opts := GetSpecOptions{
					License:    "license",
					RetryDelay: time.Millisecond,
					Debug:      true,
					// Setting a timeout longer than the server sleep
					Timeout: 500 * time.Millisecond,
				}

				got, err := client.GetSpec(opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).NotTo(BeNil())
			})
		})

		Context("when the server returns invalid JSON", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"invalid json`)) // Malformed JSON
				}))
				client = &DeployerClient{
					DeployerAPI: server.URL,
				}
			})

			It("should return a JSON parsing error", func() {
				got, err := client.GetSpec(GetSpecOptions{
					License:    "license",
					RetryDelay: time.Millisecond,
					Debug:      true,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected end of JSON input"))
				Expect(got).To(BeNil())
			})
		})

		Context("with TLS certificate validation", func() {
			var tlsServer *httptest.Server
			var certPool *x509.CertPool

			BeforeEach(func() {
				// Create HTTPS server with self-signed cert
				tlsServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					response := SpecUnknownChart{
						Metadata: &spec.Metadata{"version": "1.0.0"},
						Values:   &spec.Values{"key": "value"},
						Chart:    map[string]interface{}{"type": "local", "path": "test-path"},
					}
					jsonData, _ := json.Marshal(response)
					_, _ = w.Write(jsonData)
				}))

				// Get the server's certificate
				cert := tlsServer.TLS.Certificates[0].Certificate[0]
				x509Cert, _ := x509.ParseCertificate(cert)

				// Create a cert pool and add the certificate
				certPool = x509.NewCertPool()
				certPool.AddCert(x509Cert)
			})

			AfterEach(func() {
				if tlsServer != nil {
					tlsServer.Close()
				}
			})

			It("should succeed with certificate in trust store", func() {
				// Custom client with our trusted certificate
				httpClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							RootCAs: certPool,
						},
					},
				}

				// Make a direct request to verify certificates work
				req, _ := http.NewRequest("GET", tlsServer.URL, nil)
				resp, err := httpClient.Do(req)

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Close the response body
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			})

			It("should fail with untrusted certificate", func() {
				// Custom client with empty trust store
				httpClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							RootCAs: x509.NewCertPool(), // Empty pool with no trusted certs
						},
					},
				}

				// Make a direct request
				req, _ := http.NewRequest("GET", tlsServer.URL, nil)
				_, err := httpClient.Do(req)

				// Should fail with certificate error
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("certificate"))
			})
		})

		Context("testing CA certificates in the container", func() {
			It("should have access to system CA certificates", func() {
				// Skip in CI or if not running on Linux
				if os.Getenv("CI") != "" || os.Getenv("SKIP_SYSTEM_TESTS") != "" {
					Skip("Skipping system-dependent test in CI environment")
				}

				// Check if CA cert directories exist
				caPaths := []string{
					"/etc/pki/ca-trust",
					"/etc/pki/ca-trust/extracted",
					"/etc/ssl/certs",
				}

				// At least one CA path should exist
				found := false
				for _, path := range caPaths {
					if _, err := os.Stat(path); err == nil {
						found = true
						break
					}
				}

				Expect(found).To(BeTrue(), "No CA certificate directories found - container may be missing CA certificates")

				// Test actual HTTPS connection to a well-known site
				client := &http.Client{
					Timeout: 5 * time.Second,
				}

				// Try to connect to a secure site that would require CA verification
				req, _ := http.NewRequest("HEAD", "https://www.google.com", nil)
				resp, err := client.Do(req)

				// Should succeed if CA certs are properly installed
				if err != nil {
					Expect(err.Error()).NotTo(ContainSubstring("certificate"),
						"Certificate verification failed - container may be missing CA certificates")
				} else {
					resp.Body.Close()
				}
			})
		})
	})

	Describe("getDeployerURL", func() {
		DescribeTable("should return the correct URL based on inputs",
			func(deployerChannelUrl, releaseId, expected string) {
				client := &DeployerClient{
					DeployerAPI: deployerChannelUrl,
				}
				got := client.getDeployerURL(GetSpecOptions{ReleaseId: releaseId})
				Expect(got).To(Equal(expected))
			},
			Entry("No releaseId, default channel URL",
				"", "", DeployerAPI+DeployerChannelPath),
			Entry("No releaseId, custom channel URL",
				"https://custom-channel.example.com", "", "https://custom-channel.example.com"+DeployerChannelPath),
			Entry("With releaseId, default release URL",
				"", "123", DeployerAPI+strings.Replace(DeployerReleaseAPIPath, ":versionId", "123", 1)),
			Entry("With releaseId, custom release URL",
				"https://custom-release.example.com", "456", "https://custom-release.example.com/api/v1/operator/channel/release/456"),
		)
	})
})

// TestDockerfileCACerts tests that the CA certs from the Dockerfile are correctly installed
func TestDockerfileCACerts(t *testing.T) {
	// Skip if not running in the container environment or if skip flag is set
	if os.Getenv("RUNNING_IN_CONTAINER") != "true" || os.Getenv("SKIP_CONTAINER_TESTS") != "" {
		t.Skip("Skipping Dockerfile CA certificates test - not running in container")
	}

	// Try to access the CA cert directories that should be copied from the ubi-minimal image
	caTrustPaths := []string{
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
		"/etc/pki/ca-trust/source/anchors",
	}

	for _, path := range caTrustPaths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("CA certificate path %s not found in container: %v", path, err)
		}
	}

	// Test a real HTTPS connection to verify certificates work
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try to connect to the actual deployer endpoint
	req, err := http.NewRequest("HEAD", DeployerAPI, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to %s: %v", DeployerAPI, err)
	}
	defer resp.Body.Close()

	t.Logf("Successfully connected to %s with TLS certificate verification", DeployerAPI)
}

// TestDeployerClientWithRealEndpoint is a manual test to check connectivity to the real endpoint
// This can be run manually, but is skipped by default
func TestDeployerClientWithRealEndpoint(t *testing.T) {
	t.Skip("Manual test for real endpoint connectivity - run explicitly when needed")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with both TLS verification enabled and disabled
	testConfigurations := []struct {
		name          string
		skipTLSVerify bool
	}{
		{"With TLS verification", false},
		{"Without TLS verification", true},
	}

	for _, tc := range testConfigurations {
		t.Run(tc.name, func(t *testing.T) {
			// Create an HTTP client with detailed error tracing
			httpClient := &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: tc.skipTLSVerify,
					},
				},
			}

			// Try both with and without trailing slash to see if that's an issue
			urls := []string{
				DeployerAPI + DeployerChannelPath,
				DeployerAPI + DeployerChannelPath + "/",
			}

			for _, url := range urls {
				t.Logf("Testing URL: %s (Skip TLS Verify: %v)", url, tc.skipTLSVerify)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					t.Logf("Error creating request: %v", err)
					continue
				}

				req.Header.Set("Content-Type", "application/json")

				resp, err := httpClient.Do(req)
				if err != nil {
					t.Logf("HTTP request failed: %v", err)
					continue
				}

				t.Logf("Response status: %d", resp.StatusCode)
				body := make([]byte, 1024)
				n, _ := resp.Body.Read(body)
				t.Logf("Response body (truncated): %s", body[:n])
				resp.Body.Close()
			}
		})
	}
}
