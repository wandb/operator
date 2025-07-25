package secrets_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	"github.com/wandb/operator/pkg/wandb/spec/state/secrets"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Manager", func() {
	var state *secrets.State
	var k8sSecret *corev1.Secret
	BeforeEach(func() {
		ctx := context.Background()
		owner := &v1.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb",
				Namespace: "default",
				UID:       uuid.NewUUID(),
			},
		}
		state = secrets.New(ctx, k8sClient, owner, scheme.Scheme)
	})

	AfterEach(func() {
		if k8sSecret != nil {
			Expect(k8sClient.Delete(context.Background(), k8sSecret)).To(Succeed())
			k8sSecret = nil
		}
	})

	Describe("Get", func() {
		It("encounters an empty secret", func() {
			k8sSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "default",
				},
				Data: map[string][]byte{},
			}
			Expect(k8sClient.Create(context.Background(), k8sSecret)).To(Succeed())
			result, err := state.Get("default", "test-name")
			Expect(err.Error()).To(Equal("secret default/test-name does not have a `values` key"))
			Expect(result).To(BeNil())
		})
		It("secret has values but no chart", func() {
			k8sSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"values": []byte(`{"foo": "bar"}`),
				},
			}
			Expect(k8sClient.Create(context.Background(), k8sSecret)).To(Succeed())
			result, err := state.Get("default", "test-name")
			Expect(err.Error()).To(Equal("secret default/test-name does not have a `release` key"))
			Expect(result).To(BeNil())
		})
		It("It does not have a valid release", func() {
			k8sSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"values": []byte(`{"foo": "bar"}`),
					"chart":  []byte(`{"foo": "bar"}`),
				},
			}
			Expect(k8sClient.Create(context.Background(), k8sSecret)).To(Succeed())
			expectedSpec := &spec.Spec{
				Metadata: nil,
				Chart:    nil,
				Values:   map[string]interface{}{"foo": "bar"},
			}
			result, err := state.Get("default", "test-name")
			Expect(err).To(BeNil())
			Expect(result).To(Equal(expectedSpec))
		})
		It("should return the spec when the secret and has proper values", func() {
			k8sSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-name",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"values": []byte(`{"foo": "bar"}`),
					"chart":  []byte(`{"path": "foo/bar"}`),
				},
			}
			Expect(k8sClient.Create(context.Background(), k8sSecret)).To(Succeed())
			expectedSpec := &spec.Spec{
				Metadata: nil,
				Chart: &charts.LocalRelease{
					Path: "foo/bar",
				},
				Values: map[string]interface{}{"foo": "bar"},
			}
			result, err := state.Get("default", "test-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expectedSpec))
		})

		It("should return an error when the secret does not exist", func() {
			result, err := state.Get("default", "test-name")
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("Set", func() {
		It("should set the spec successfully", func() {
			expectedSpec := &spec.Spec{
				Metadata: nil,
				Chart: &charts.LocalRelease{
					Path: "foo/bar",
				},
				Values: map[string]interface{}{"foo": "bar"},
			}
			err := state.Set("default", "test-name", expectedSpec)
			Expect(err).NotTo(HaveOccurred())
			k8sSecret = &corev1.Secret{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "test-name"}, k8sSecret)).To(Succeed())
		})
	})
})
