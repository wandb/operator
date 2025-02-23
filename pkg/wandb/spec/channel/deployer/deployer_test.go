package deployer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/pkg/wandb/spec"
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
					_, _ = w.Write([]byte(`{}`))
				}))
				client = &DeployerClient{
					DeployerAPI: server.URL,
				}
			})

			It("should make a request with correct headers and return successfully", func() {
				got, err := client.GetSpec(GetSpecOptions{
					License:     "license",
					ActiveState: &spec.Spec{},
					RetryDelay:  time.Millisecond,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(&spec.Spec{}))
			})
		})

		Context("when the HTTP request fails repeatedly", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{}`))
				}))
				client = &DeployerClient{
					DeployerAPI: server.URL,
				}
			})

			It("should return an error after all retries fail", func() {
				got, err := client.GetSpec(GetSpecOptions{
					License:     "license",
					ActiveState: &spec.Spec{},
					RetryDelay:  10 * time.Millisecond,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("all retries failed"))
				Expect(got).To(BeNil())
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
