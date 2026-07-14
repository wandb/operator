package seaweedfs

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SeaweedFS writable storage probe", func() {
	It("reports writable storage after an allocation succeeds", func() {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			Expect(request.URL.Path).To(Equal("/dir/assign"))
			_, err := response.Write([]byte(`{"fid":"3,01637037d6","url":"volume:8444","count":1}`))
			Expect(err).NotTo(HaveOccurred())
		}))
		DeferCleanup(server.Close)

		condition := probeSeaweedAllocation(context.Background(), server.Client(), server.URL+"/dir/assign")

		Expect(condition.Type).To(Equal(SeaweedWritableType))
		Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		Expect(condition.Reason).To(Equal("AllocationSucceeded"))
	})

	It("reports allocation errors as not writable", func() {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusInternalServerError)
			_, err := response.Write([]byte(`{"error":"No writable volumes and no free volumes left"}`))
			Expect(err).NotTo(HaveOccurred())
		}))
		DeferCleanup(server.Close)

		condition := probeSeaweedAllocation(context.Background(), server.Client(), server.URL)

		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal("AllocationFailed"))
		Expect(condition.Message).To(ContainSubstring("No writable volumes"))
	})

	It("rejects successful responses without an allocation", func() {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_, err := response.Write([]byte(`{"count":0}`))
			Expect(err).NotTo(HaveOccurred())
		}))
		DeferCleanup(server.Close)

		condition := probeSeaweedAllocation(context.Background(), server.Client(), server.URL)

		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Message).To(ContainSubstring("file ID"))
	})

	It("reports an unauthenticated S3 API response as reachable", func() {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusForbidden)
		}))
		DeferCleanup(server.Close)

		condition := probeSeaweedS3(context.Background(), server.Client(), server.URL)

		Expect(condition.Type).To(Equal(SeaweedS3ReachableType))
		Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		Expect(condition.Reason).To(Equal("EndpointReachable"))
	})

	It("rejects an unavailable S3 API", func() {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusServiceUnavailable)
		}))
		DeferCleanup(server.Close)

		condition := probeSeaweedS3(context.Background(), server.Client(), server.URL)

		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal("EndpointUnavailable"))
	})
})
