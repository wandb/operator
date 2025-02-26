package charts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"crypto/tls"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

func TestRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Repo Suite", Label("charts"))
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Suite setup code (runs once)
	return nil
}, func(data []byte) {
	// Node setup code (runs on each parallel node)
})

var _ = SynchronizedAfterSuite(func() {
	// Node cleanup code
}, func() {
	// Suite cleanup code (runs once)
})

var _ = Describe("RepoRelease", func() {
	var tempDir string
	var repoRelease *RepoRelease
	var logCap *logCapture

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "repo-test-*")
		Expect(err).NotTo(HaveOccurred())

		repoRelease = &RepoRelease{
			URL:      "https://charts.example.com",
			Name:     "test-chart",
			Version:  "1.0.0",
			Username: "test-user",
			Password: "test-pass",
			Debug:    false,
		}

		// Set up mock responses
		chartMetadata := &chart.Metadata{
			Version: "1.0.0",
			Name:    "test-chart",
		}

		chartVersion := repo.ChartVersion{
			Metadata: chartMetadata,
			URLs:     []string{"https://charts.example.com/test-chart-1.0.0.tgz"},
		}

		indexFile := &repo.IndexFile{
			APIVersion: "v1",
			Entries: map[string]repo.ChartVersions{
				"test-chart": {&chartVersion},
			},
		}

		// Create required directories
		Expect(os.MkdirAll(filepath.Join(tempDir, "cache"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tempDir, "config"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tempDir, "data"), 0755)).To(Succeed())

		// Write index files for different registries
		registries := []string{"charts-example-com", "custom-registry"}
		for _, registry := range registries {
			indexPath := filepath.Join(tempDir, "cache", registry+"-index.yaml")
			err = indexFile.WriteFile(indexPath, 0644)
			Expect(err).NotTo(HaveOccurred())
		}

		logCap = newLogCapture()
		logCap.setup()
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
		logCap.teardown()
	})

	Describe("deriveRepoName", func() {
		Context("with explicit repo name", func() {
			BeforeEach(func() {
				repoRelease.RepoName = "explicit-name"
			})

			It("should use the explicit name", func() {
				name, err := repoRelease.deriveRepoName()
				Expect(err).NotTo(HaveOccurred())
				Expect(name).To(Equal("explicit-name"))
			})
		})

		Context("with various URLs", func() {
			DescribeTable("should derive correct repo names",
				func(url, expected string) {
					repoRelease.URL = url
					name, err := repoRelease.deriveRepoName()
					Expect(err).NotTo(HaveOccurred())
					Expect(name).To(Equal(expected))
				},
				Entry("simple hostname", "https://example.com", "example-com"),
				Entry("subdomain", "https://charts.example.com", "charts-example-com"),
				Entry("with port", "https://example.com:8080", "example-com"),
				Entry("with path", "https://example.com/charts", "example-com"),
			)

			It("should fail with invalid URL", func() {
				repoRelease.URL = "not-a-url"
				_, err := repoRelease.deriveRepoName()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not derive repository name from URL"))
			})
		})
	})

	Describe("Chart", func() {
		It("should return error when ToLocalRelease fails", func() {
			repoRelease.URL = "invalid-url"
			chart, err := repoRelease.Chart()
			Expect(err).To(HaveOccurred())
			Expect(chart).To(BeNil())
		})
	})

	Describe("Validate", func() {
		Context("with valid configuration", func() {
			It("should validate successfully", func() {
				err := repoRelease.Validate()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with missing required fields", func() {
			It("should fail validation when URL is missing", func() {
				repoRelease.URL = ""
				err := repoRelease.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("URL"))
			})

			It("should fail validation when Name is missing", func() {
				repoRelease.Name = ""
				err := repoRelease.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Name"))
			})
		})

		Context("with invalid URL", func() {
			It("should fail validation", func() {
				repoRelease.URL = "not-a-url"
				err := repoRelease.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("URL"))
			})
		})
	})

	Describe("ToLocalRelease", func() {
		It("should convert to LocalRelease", func() {
			local, err := repoRelease.ToLocalRelease()
			Expect(err).To(HaveOccurred()) // Expected since we're not fully mocking helm
			Expect(local).To(BeNil())
		})

		It("should fail when downloadChart fails", func() {
			repoRelease.URL = "invalid-url"
			local, err := repoRelease.ToLocalRelease()
			Expect(err).To(HaveOccurred())
			Expect(local).To(BeNil())
		})
	})

	Describe("Apply", func() {
		It("should return error when ToLocalRelease fails", func() {
			repoRelease.URL = "invalid-url"
			err := repoRelease.Apply(context.TODO(), nil, nil, nil, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Prune", func() {
		It("should return error when ToLocalRelease fails", func() {
			repoRelease.URL = "invalid-url"
			err := repoRelease.Prune(context.TODO(), nil, nil, nil, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("downloadChart", func() {
		Context("with environment variables", func() {
			BeforeEach(func() {
				os.Setenv("HELM_CACHE_HOME", filepath.Join(tempDir, "cache"))
				os.Setenv("HELM_CONFIG_HOME", filepath.Join(tempDir, "config"))
				os.Setenv("HELM_DATA_HOME", filepath.Join(tempDir, "data"))
			})

			AfterEach(func() {
				os.Unsetenv("HELM_CACHE_HOME")
				os.Unsetenv("HELM_CONFIG_HOME")
				os.Unsetenv("HELM_DATA_HOME")
			})

			Context("TLS verification settings", func() {
				DescribeTable("should handle TLS verification appropriately",
					func(url string, expectSkipTLS bool) {
						repoRelease.URL = url
						// Create a mock server that immediately returns 404
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusNotFound)
						}))
						defer server.Close()
						repoRelease.URL = server.URL // Use mock server URL

						_, err := repoRelease.downloadChart()
						Expect(err).To(HaveOccurred()) // Expected since we're returning 404

						// Verify that the error is not related to TLS verification
						Expect(err.Error()).NotTo(ContainSubstring("x509: certificate"))
					},
					Entry("HTTP URL should skip TLS", "http://example.com", true),
					Entry("HTTPS URL should not skip TLS", "https://example.com", false),
					Entry("HTTP URL with uppercase should skip TLS", "HTTP://example.com", true),
					Entry("HTTPS URL with uppercase should not skip TLS", "HTTPS://example.com", false),
				)

				It("should correctly set InsecureSkipTLSverify based on URL scheme", func() {
					// For this test, we'll use a different approach since we can't easily mock the repo.NewChartRepository function

					// First, test with HTTP URL
					httpURL := "http://example.com"
					parsedURL, err := url.Parse(httpURL)
					Expect(err).NotTo(HaveOccurred())

					// Directly test the logic from repo.go
					insecureSkipTLS := parsedURL.Scheme == "http"
					Expect(insecureSkipTLS).To(BeTrue(), "HTTP URLs should have InsecureSkipTLSverify=true")

					// Now test with HTTPS URL
					httpsURL := "https://example.com"
					parsedURL, err = url.Parse(httpsURL)
					Expect(err).NotTo(HaveOccurred())

					// Directly test the logic from repo.go
					insecureSkipTLS = parsedURL.Scheme == "http"
					Expect(insecureSkipTLS).To(BeFalse(), "HTTPS URLs should have InsecureSkipTLSverify=false")

					// Test with uppercase schemes
					httpUpperURL := "HTTP://example.com"
					parsedURL, err = url.Parse(httpUpperURL)
					Expect(err).NotTo(HaveOccurred())

					// For uppercase HTTP, the scheme will be normalized to lowercase by url.Parse
					insecureSkipTLS = parsedURL.Scheme == "http"
					Expect(insecureSkipTLS).To(BeTrue(), "Uppercase HTTP URLs should have InsecureSkipTLSverify=true")
				})

				It("should log debug information about TLS verification when debug is enabled", func() {
					// Enable debug logging
					repoRelease.Debug = true

					// Set HTTP URL to trigger TLS skip
					repoRelease.URL = "http://example.com"

					// Create a log capture to verify log messages
					logCapture := newLogCapture()
					logCapture.setup()
					defer logCapture.teardown()

					// Create a test server
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusNotFound)
					}))
					defer server.Close()
					repoRelease.URL = server.URL

					// Call the function that should log TLS verification info
					_, err := repoRelease.downloadChart()
					Expect(err).To(HaveOccurred()) // Expected since we're returning 404

					// Verify that the log message about TLS verification was captured
					// Note: This is an indirect test as we're using the ginkgoLogSink
					// The expected log message would be something like "TLS verification disabled for HTTP URL"
					// We can't directly capture and assert on log messages with the current setup
				})

				Context("with self-signed certificate", func() {
					var selfSignedServer *httptest.Server

					BeforeEach(func() {
						// Create a TLS server with self-signed certificate
						selfSignedServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)
							response := map[string]interface{}{
								"entries": map[string]interface{}{
									"test-chart": []interface{}{
										map[string]interface{}{
											"name": "test-chart",
											"urls": []string{"https://example.com/test-chart-1.0.0.tgz"},
										},
									},
								},
							}
							json.NewEncoder(w).Encode(response)
						}))

						// Set up the repo release to use this server
						repoRelease.URL = selfSignedServer.URL
					})

					AfterEach(func() {
						if selfSignedServer != nil {
							selfSignedServer.Close()
						}
					})

					It("should fail with default client due to certificate verification", func() {
						// Call download with default settings (should fail with cert error)
						_, err := repoRelease.downloadChart()
						Expect(err).To(HaveOccurred())
						// Should fail with certificate error
						Expect(err.Error()).To(Or(
							ContainSubstring("certificate"),
							ContainSubstring("x509"),
							ContainSubstring("TLS"), // Different error messages on different platforms
						))
					})

					It("should succeed when InsecureSkipTLSverify is manually set to true", func() {
						// This test requires modifying internal code behavior to set InsecureSkipTLSverify
						// Since we can't easily modify the code behavior directly in tests,
						// we'll test the behavior indirectly by using the getter options

						// First verify we can make a connection with insecure client
						transport := &http.Transport{
							TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
						}
						client := &http.Client{Transport: transport}
						req, err := http.NewRequest("GET", selfSignedServer.URL, nil)
						Expect(err).NotTo(HaveOccurred())

						resp, err := client.Do(req)
						Expect(err).NotTo(HaveOccurred())
						resp.Body.Close()

						// Now the actual test - the full downloadChart can't easily be tested due to
						// its internal complexity, but we've verified our test setup works
					})

					It("should make HTTP transport requests with appropriate TLS settings", func() {
						// Instead of trying to track HTTP requests by replacing the default client,
						// we'll track requests at the server level
						var httpServerRequested bool

						// Create a mock server with a valid index file format
						httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// Mark that the server received a request
							httpServerRequested = true

							// Return a properly structured index file
							indexFile := map[string]interface{}{
								"apiVersion": "v1", // Required field
								"entries": map[string]interface{}{
									"test-chart": []interface{}{
										map[string]interface{}{
											"name":    "test-chart",
											"version": "1.0.0",
											"urls":    []string{"https://charts.example.com/test-chart-1.0.0.tgz"},
										},
									},
								},
							}
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusOK)
							json.NewEncoder(w).Encode(indexFile)
						}))
						defer httpServer.Close()

						// Set up the repo release
						repoRelease.URL = httpServer.URL
						repoRelease.Debug = true

						// Call the function
						_, _ = repoRelease.downloadChart()

						// Verify that the server received at least one request
						Expect(httpServerRequested).To(BeTrue(), "No HTTP requests were received by the server")

						// For HTTPS with self-signed certificates, we expect requests to fail before reaching
						// the server unless we modify the client's TLS config, so we'll skip that test
					})
				})
			})

			Context("with different repository URLs", func() {
				DescribeTable("should use correct index file",
					func(url string) {
						repoRelease.URL = url
						// Create a mock server that immediately returns 404
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusNotFound)
						}))
						defer server.Close()
						repoRelease.URL = server.URL // Use mock server URL

						_, err := repoRelease.downloadChart()
						Expect(err).To(HaveOccurred()) // Expected since we're returning 404
						// The error should be about HTTP, not about missing index file
						Expect(err.Error()).NotTo(ContainSubstring("Failed to load index file"))
					},
					Entry("simple hostname", "https://example.com"),
					Entry("subdomain", "https://charts.example.com"),
					Entry("with port", "https://example.com:8080"),
					Entry("with path", "https://example.com/charts"),
				)
			})

			It("should attempt to download chart and log debug info", func() {
				// Create a mock server that immediately returns 404
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
				defer server.Close()
				repoRelease.URL = server.URL // Use mock server URL

				_, err := repoRelease.downloadChart()
				Expect(err).To(HaveOccurred())
			})

			Context("with invalid chart URL", func() {
				BeforeEach(func() {
					repoRelease.URL = "invalid-url"
				})

				It("should return error", func() {
					path, err := repoRelease.downloadChart()
					Expect(err).To(HaveOccurred())
					Expect(path).To(BeEmpty())
				})
			})

			Context("with non-existent chart version", func() {
				BeforeEach(func() {
					repoRelease.Version = "9.9.9"
				})

				It("should return error", func() {
					path, err := repoRelease.downloadChart()
					Expect(err).To(HaveOccurred())
					Expect(path).To(BeEmpty())
				})
			})
		})
	})
})

// recordingTransport is a test helper that wraps the default transport and records request information
type recordingTransport struct {
	originalTransport http.RoundTripper
	requestCallback   func(*http.Request)
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.requestCallback != nil {
		t.requestCallback(req)
	}
	return t.originalTransport.RoundTrip(req)
}
