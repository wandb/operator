package v2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var _ = Describe("Networking Route Builders", func() {
	It("keeps managed gateway listeners namespace-scoped when infra stays local", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ObjectStore: apiv2.ObjectStoreSpec{
					ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{Namespace: "wandb-ns"},
				},
			},
		}

		allowedRoutes := buildAllowedRoutes(wandb)
		Expect(allowedRoutes).NotTo(BeNil())
		Expect(allowedRoutes.Namespaces).NotTo(BeNil())
		Expect(allowedRoutes.Namespaces.From).NotTo(BeNil())
		Expect(*allowedRoutes.Namespaces.From).To(Equal(gatewayv1.NamespacesFromSame))
	})

	It("widens managed gateway listeners when infra routes are cross-namespace", func() {
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ObjectStore: apiv2.ObjectStoreSpec{
					ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{Namespace: "infra-ns"},
				},
			},
		}

		allowedRoutes := buildAllowedRoutes(wandb)
		Expect(allowedRoutes).NotTo(BeNil())
		Expect(allowedRoutes.Namespaces).NotTo(BeNil())
		Expect(allowedRoutes.Namespaces.From).NotTo(BeNil())
		Expect(*allowedRoutes.Namespaces.From).To(Equal(gatewayv1.NamespacesFromAll))
	})

	It("builds infra HTTPRoutes with the resolved backend, labels, and parent refs", func() {
		parentNamespace := gatewayv1.Namespace("gateway-ns")
		parentRef := gatewayv1.ParentReference{
			Name:      gatewayv1.ObjectName("shared-gateway"),
			Namespace: &parentNamespace,
		}
		hostnames := []gatewayv1.Hostname{"wandb.example.com", "alt.example.com"}
		entry := infraRouteEntry{
			name:        "wandb-bucket-default",
			namespace:   "infra-ns",
			serviceName: "wandb-minio-hl",
			servicePort: gatewayv1.PortNumber(9000),
			ingress: &serverManifest.AppIngressSpec{
				Paths:    []string{"/bucket"},
				PathType: "Exact",
			},
		}
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"},
		}

		route := buildInfraHTTPRoute(wandb, parentRef, hostnames, entry)

		Expect(route.Namespace).To(Equal("infra-ns"))
		Expect(route.Labels).To(HaveKeyWithValue(common.WandbNameLabel, "wandb"))
		Expect(route.Labels).To(HaveKeyWithValue(common.WandbNamespaceLabel, "wandb-ns"))
		Expect(route.Labels).To(HaveKeyWithValue(common.WandbComponentLabel, infraHTTPRouteComponent))
		Expect(route.Spec.ParentRefs).To(HaveLen(1))
		Expect(route.Spec.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName("shared-gateway")))
		Expect(route.Spec.ParentRefs[0].Namespace).NotTo(BeNil())
		Expect(*route.Spec.ParentRefs[0].Namespace).To(Equal(parentNamespace))
		Expect(route.Spec.Hostnames).To(Equal(hostnames))
		Expect(route.Spec.Rules).To(HaveLen(1))
		Expect(route.Spec.Rules[0].Matches).To(HaveLen(1))
		Expect(route.Spec.Rules[0].Matches[0].Path).NotTo(BeNil())
		Expect(*route.Spec.Rules[0].Matches[0].Path.Type).To(Equal(gatewayv1.PathMatchExact))
		Expect(*route.Spec.Rules[0].Matches[0].Path.Value).To(Equal("/bucket"))
		Expect(route.Spec.Rules[0].BackendRefs).To(HaveLen(1))
		Expect(route.Spec.Rules[0].BackendRefs[0].Name).To(Equal(gatewayv1.ObjectName("wandb-minio-hl")))
		Expect(route.Spec.Rules[0].BackendRefs[0].Port).NotTo(BeNil())
		Expect(*route.Spec.Rules[0].BackendRefs[0].Port).To(Equal(gatewayv1.PortNumber(9000)))
	})

	It("builds application HTTPRoute templates for external gateways", func() {
		listenerName := "https"
		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Wandb: apiv2.WandbAppSpec{
					Hostname:            "https://wandb.example.com",
					AdditionalHostnames: []string{"alt.example.com"},
				},
				Networking: apiv2.NetworkingSpec{
					Mode: apiv2.NetworkingModeGatewayAPI,
					GatewayAPI: &apiv2.GatewayAPIConfig{
						ListenerName: &listenerName,
					},
				},
			},
			Status: apiv2.WeightsAndBiasesStatus{
				GatewayStatus: &apiv2.GatewayStatusSummary{
					GatewayRef: &apiv2.GatewayReference{
						Name:      "shared-gateway",
						Namespace: "gateway-ns",
					},
				},
			},
		}
		app := serverManifest.Application{
			Ingress: &serverManifest.AppIngressSpec{
				Paths:       []string{"/api"},
				PathType:    "Prefix",
				ServicePort: "8080",
			},
		}

		template := buildHTTPRouteTemplate(wandb, app)

		Expect(template).NotTo(BeNil())
		Expect(template.ParentRefs).To(HaveLen(1))
		Expect(template.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName("shared-gateway")))
		Expect(template.ParentRefs[0].Namespace).NotTo(BeNil())
		Expect(*template.ParentRefs[0].Namespace).To(Equal(gatewayv1.Namespace("gateway-ns")))
		Expect(template.ParentRefs[0].SectionName).NotTo(BeNil())
		Expect(*template.ParentRefs[0].SectionName).To(Equal(gatewayv1.SectionName("https")))
		Expect(template.Hostnames).To(ConsistOf(
			gatewayv1.Hostname("wandb.example.com"),
			gatewayv1.Hostname("alt.example.com"),
		))
		Expect(template.Paths).To(Equal([]string{"/api"}))
		Expect(template.PathType).To(Equal("Prefix"))
		Expect(template.ServicePort).NotTo(BeNil())
		Expect(*template.ServicePort).To(Equal(gatewayv1.PortNumber(8080)))
	})

	It("routes /bucket through nginx-proxy when the proxy feature is enabled", func() {
		manifest := serverManifest.Manifest{
			Features: map[string]bool{"proxy": true},
			Bucket: map[string]serverManifest.InfraConfig{
				"default": {
					Ingress: &serverManifest.AppIngressSpec{
						Paths:       []string{"/bucket"},
						ServicePort: "http-minio",
						PathType:    "Prefix",
					},
				},
			},
			Applications: map[string]serverManifest.Application{
				"nginx-proxy": {
					Name: "nginx-proxy",
					Service: &serverManifest.ServiceSpec{
						Ports: []corev1.ServicePort{{Port: 8080}},
					},
				},
			},
		}
		app := manifest.Applications["nginx-proxy"]

		app = applyProxyIngress(app, manifest)

		Expect(app.Ingress).NotTo(BeNil())
		Expect(app.Ingress.Paths).To(ContainElement("/bucket"))
		Expect(app.Ingress.ServicePort).To(Equal("8080"))

		wandb := &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"},
			Spec: apiv2.WeightsAndBiasesSpec{
				ObjectStore: apiv2.ObjectStoreSpec{
					ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{Namespace: "infra-ns"},
				},
			},
		}
		entries, err := resolveInfraRoutes(nil, nil, wandb, manifest)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

})
