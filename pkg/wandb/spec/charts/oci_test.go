package charts

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("OCIRelease", func() {
	var ociRelease *OCIRelease

	BeforeEach(func() {
		ociRelease = &OCIRelease{
			URL:     "oci://ghcr.io/wandb/helm-charts/operator-wandb",
			Version: "1.0.0",
			Debug:   false,
		}
	})

	Describe("Validate", func() {
		Context("with valid OCI URL", func() {
			It("should validate successfully", func() {
				err := ociRelease.Validate()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with various valid OCI URLs", func() {
			DescribeTable("should validate successfully",
				func(url string) {
					ociRelease.URL = url
					err := ociRelease.Validate()
					Expect(err).NotTo(HaveOccurred())
				},
				Entry("ghcr.io", "oci://ghcr.io/wandb/charts/wandb"),
				Entry("docker.io", "oci://docker.io/library/nginx"),
				Entry("custom registry with port", "oci://registry.example.com:5000/charts/wandb"),
				Entry("without version", "oci://ghcr.io/wandb/charts/wandb"),
			)
		})

		Context("with missing URL", func() {
			It("should fail validation", func() {
				ociRelease.URL = ""
				err := ociRelease.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("URL"))
			})
		})

		Context("with non-OCI URL", func() {
			DescribeTable("should fail validation",
				func(url string) {
					ociRelease.URL = url
					err := ociRelease.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("URL"))
				},
				Entry("https URL", "https://charts.example.com"),
				Entry("http URL", "http://charts.example.com"),
				Entry("plain hostname", "ghcr.io/wandb/charts/wandb"),
				Entry("empty scheme", "://ghcr.io/wandb/charts/wandb"),
			)
		})
	})

	Describe("Chart dispatcher", func() {
		It("should return OCIRelease for OCI URL", func() {
			input := map[string]interface{}{
				"url":     "oci://ghcr.io/wandb/charts/wandb",
				"version": "1.0.0",
			}
			result := Get(input)
			Expect(result).NotTo(BeNil())
			Expect(result).To(BeAssignableToTypeOf(&OCIRelease{}))
		})

		It("should return RepoRelease for HTTPS URL", func() {
			input := map[string]interface{}{
				"url":  "https://charts.example.com",
				"name": "wandb",
			}
			result := Get(input)
			Expect(result).NotTo(BeNil())
			Expect(result).To(BeAssignableToTypeOf(&RepoRelease{}))
		})

		It("should return LocalRelease for path", func() {
			input := map[string]interface{}{
				"path": "/opt/charts/wandb.tgz",
			}
			result := Get(input)
			Expect(result).NotTo(BeNil())
			Expect(result).To(BeAssignableToTypeOf(&LocalRelease{}))
		})

		It("should not match OCI URL as RepoRelease", func() {
			input := map[string]interface{}{
				"url":     "oci://ghcr.io/wandb/charts/wandb",
				"version": "1.0.0",
			}
			release := new(RepoRelease)
			err := Is(release, input)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("pullChart", func() {
		It("should return error for unreachable registry", func() {
			ociRelease.URL = "oci://localhost:1/nonexistent/chart"
			ociRelease.PlainHTTP = true
			_, err := ociRelease.pullChart()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to pull chart"))
		})
	})

	Describe("Apply", func() {
		It("should return error when pullChart fails", func() {
			ociRelease.URL = "oci://localhost:1/nonexistent/chart"
			ociRelease.PlainHTTP = true
			err := ociRelease.Apply(context.TODO(), nil, &v1.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}, nil, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Prune", func() {
		It("should return error when actionable chart creation fails", func() {
			// Empty name will fail release name validation
			err := ociRelease.Prune(context.TODO(), nil, &v1.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
			}, nil, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CredentialSecret", func() {
		var (
			fakeClient client.Client
			scheme     *runtime.Scheme
			wandb      *v1.WeightsAndBiases
			config     spec.Values
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(v1.AddToScheme(scheme)).To(Succeed())
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

			wandb = &v1.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: "test-namespace",
				},
			}

			config = spec.Values{}

			os.Setenv("HELM_CACHE_HOME", filepath.Join(os.TempDir(), "oci-test-cache"))
			os.Setenv("HELM_CONFIG_HOME", filepath.Join(os.TempDir(), "oci-test-config"))
			os.Setenv("HELM_DATA_HOME", filepath.Join(os.TempDir(), "oci-test-data"))
		})

		AfterEach(func() {
			os.Unsetenv("HELM_CACHE_HOME")
			os.Unsetenv("HELM_CONFIG_HOME")
			os.Unsetenv("HELM_DATA_HOME")
		})

		Context("with valid credential secret", func() {
			BeforeEach(func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-oci-credentials",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"HELM_USERNAME": []byte("secret-user"),
						"HELM_PASSWORD": []byte("secret-pass"),
					},
				}
				Expect(fakeClient.Create(context.TODO(), secret)).To(Succeed())

				ociRelease.URL = "oci://localhost:1/nonexistent/chart"
				ociRelease.PlainHTTP = true
				ociRelease.CredentialSecret = &CredentialSecret{
					Name:        "test-oci-credentials",
					UsernameKey: "HELM_USERNAME",
					PasswordKey: "HELM_PASSWORD",
				}
				ociRelease.Username = ""
				ociRelease.Password = ""
			})

			It("should retrieve credentials from secret before pull", func() {
				err := ociRelease.Apply(context.TODO(), fakeClient, wandb, scheme, config)
				// Should fail at pull step, not credential retrieval
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("Failed to get credentials from secret"))
			})
		})

		Context("with default credential keys", func() {
			BeforeEach(func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-oci-defaults",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"HELM_USERNAME": []byte("default-user"),
						"HELM_PASSWORD": []byte("default-pass"),
					},
				}
				Expect(fakeClient.Create(context.TODO(), secret)).To(Succeed())

				ociRelease.URL = "oci://localhost:1/nonexistent/chart"
				ociRelease.PlainHTTP = true
				ociRelease.CredentialSecret = &CredentialSecret{
					Name: "test-oci-defaults",
				}
			})

			It("should use default credential keys", func() {
				err := ociRelease.Apply(context.TODO(), fakeClient, wandb, scheme, config)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("Failed to get credentials from secret"))
			})
		})

		Context("with missing credential secret", func() {
			BeforeEach(func() {
				ociRelease.URL = "oci://localhost:1/nonexistent/chart"
				ociRelease.PlainHTTP = true
				ociRelease.CredentialSecret = &CredentialSecret{
					Name:        "non-existent-secret",
					UsernameKey: "HELM_USERNAME",
					PasswordKey: "HELM_PASSWORD",
				}
			})

			It("should fail when secret does not exist", func() {
				err := ociRelease.Apply(context.TODO(), fakeClient, wandb, scheme, config)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("non-existent-secret"))
			})
		})

		Context("without credential secret", func() {
			BeforeEach(func() {
				ociRelease.URL = "oci://localhost:1/nonexistent/chart"
				ociRelease.PlainHTTP = true
				ociRelease.CredentialSecret = nil
				ociRelease.Username = "direct-user"
				ociRelease.Password = "direct-pass"
			})

			It("should use direct credentials", func() {
				err := ociRelease.Apply(context.TODO(), fakeClient, wandb, scheme, config)
				// Should fail at pull step, not credential setup
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("Failed to get credentials from secret"))
			})
		})
	})
})
