package charts

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"helm.sh/helm/v3/pkg/registry"
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
				Entry("tagged reference without version", "oci://ghcr.io/wandb/charts/wandb:1.0.0"),
				Entry("digest reference without version", "oci://ghcr.io/wandb/charts/wandb@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888"),
			)
		})

		Context("with missing version", func() {
			It("should fail validation for unqualified repository URLs", func() {
				ociRelease.Version = ""
				err := ociRelease.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Version"))
			})

			It("should allow tagged OCI URLs", func() {
				ociRelease.URL = "oci://ghcr.io/wandb/charts/wandb:1.0.0"
				ociRelease.Version = ""
				err := ociRelease.Validate()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow digest OCI URLs", func() {
				ociRelease.URL = "oci://ghcr.io/wandb/charts/wandb@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888"
				ociRelease.Version = ""
				err := ociRelease.Validate()
				Expect(err).NotTo(HaveOccurred())
			})
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

		It("should not match OCI URL as RepoRelease even with name field", func() {
			input := map[string]interface{}{
				"url":     "oci://ghcr.io/wandb/charts/wandb",
				"name":    "wandb",
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

	Describe("pullReference", func() {
		var registryClient *registry.Client

		BeforeEach(func() {
			var err error
			registryClient, err = registry.NewClient()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve an explicit tag from the URL", func() {
			ociRelease.URL = "oci://ghcr.io/wandb/charts/wandb:1.2.3"
			ociRelease.Version = ""

			ref, err := ociRelease.pullReference(registryClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(ref).To(Equal("oci://ghcr.io/wandb/charts/wandb:1.2.3"))
		})

		It("should preserve an explicit digest from the URL", func() {
			ociRelease.URL = "oci://ghcr.io/wandb/charts/wandb@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888"
			ociRelease.Version = ""

			ref, err := ociRelease.pullReference(registryClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(ref).To(Equal("oci://ghcr.io/wandb/charts/wandb@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888"))
		})

		It("should combine a repository URL with the version field", func() {
			ref, err := ociRelease.pullReference(registryClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(ref).To(Equal("oci://ghcr.io/wandb/helm-charts/operator-wandb:1.0.0"))
		})

		It("should reject mismatched tagged URLs and version fields", func() {
			ociRelease.URL = "oci://ghcr.io/wandb/charts/wandb:1.2.3"
			ociRelease.Version = "2.0.0"

			_, err := ociRelease.pullReference(registryClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("chart reference and version mismatch"))
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

			GinkgoT().Setenv("HELM_CACHE_HOME", filepath.Join(os.TempDir(), "oci-test-cache"))
			GinkgoT().Setenv("HELM_CONFIG_HOME", filepath.Join(os.TempDir(), "oci-test-config"))
			GinkgoT().Setenv("HELM_DATA_HOME", filepath.Join(os.TempDir(), "oci-test-data"))
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
