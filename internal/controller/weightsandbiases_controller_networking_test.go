package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/reconciler"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var _ = Describe("WeightsAndBiases Networking", func() {
	const wandbNamespace = "default"

	It("reconciles a managed Gateway and cross-namespace infra HTTPRoutes", func() {
		ctx := context.Background()
		wandbName := "network-gateway-managed"
		infraNamespace := "network-gateway-managed-infra"
		listenerName := "https"
		gatewayClassName := "test-gateway-class"

		createNamespaceIfMissing(ctx, infraNamespace)
		wandb, objectStoreService := newNetworkingWandb(wandbName, infraNamespace)
		wandb.Spec.Networking = apiv2.NetworkingSpec{
			Mode: apiv2.NetworkingModeGatewayAPI,
			GatewayAPI: &apiv2.GatewayAPIConfig{
				ListenerName: &listenerName,
				Gateway: apiv2.GatewayConfig{
					Managed:          true,
					GatewayClassName: &gatewayClassName,
					Listeners: []apiv2.GatewayListener{{
						Name:     listenerName,
						Port:     80,
						Protocol: string(gatewayv1.HTTPProtocolType),
					}},
				},
			},
		}

		Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
		Expect(k8sClient.Create(ctx, objectStoreService)).To(Succeed())
		DeferCleanup(deleteIfPresent, ctx, wandb)

		wandb = markWandbReadyForNetworking(ctx, wandbName, wandbNamespace)
		reconcileNetworkingManifest(ctx, wandb)

		gatewayName := fmt.Sprintf("%s-gateway", wandbName)
		gateway := &gatewayv1.Gateway{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: wandbNamespace}, gateway)).To(Succeed())
		Expect(gateway.Spec.Listeners).To(HaveLen(1))
		Expect(gateway.Spec.Listeners[0].AllowedRoutes).NotTo(BeNil())
		Expect(gateway.Spec.Listeners[0].AllowedRoutes.Namespaces).NotTo(BeNil())
		Expect(gateway.Spec.Listeners[0].AllowedRoutes.Namespaces.From).NotTo(BeNil())
		Expect(*gateway.Spec.Listeners[0].AllowedRoutes.Namespaces.From).To(Equal(gatewayv1.NamespacesFromAll))

		infraRoute := &gatewayv1.HTTPRoute{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      fmt.Sprintf("%s-bucket-default", wandbName),
			Namespace: infraNamespace,
		}, infraRoute)).To(Succeed())
		Expect(infraRoute.Spec.ParentRefs).To(HaveLen(1))
		Expect(infraRoute.Spec.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName(gatewayName)))
		Expect(infraRoute.Spec.ParentRefs[0].Namespace).NotTo(BeNil())
		Expect(*infraRoute.Spec.ParentRefs[0].Namespace).To(Equal(gatewayv1.Namespace(wandbNamespace)))

		gateway.Status.Addresses = []gatewayv1.GatewayStatusAddress{{Value: "10.0.0.5"}}
		gateway.Status.Conditions = []metav1.Condition{{
			Type:               string(gatewayv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.GatewayReasonProgrammed),
			LastTransitionTime: metav1.Now(),
		}}
		Expect(k8sClient.Status().Update(ctx, gateway)).To(Succeed())

		wandb = getWandb(ctx, wandbName, wandbNamespace)
		reconcileNetworkingManifest(ctx, wandb)
		wandb = getWandb(ctx, wandbName, wandbNamespace)

		Expect(wandb.Status.GatewayStatus).NotTo(BeNil())
		Expect(wandb.Status.GatewayStatus.GatewayRef).NotTo(BeNil())
		Expect(wandb.Status.GatewayStatus.GatewayRef.Name).To(Equal(gatewayName))
		Expect(wandb.Status.GatewayStatus.Ready).To(BeTrue())
		Expect(wandb.Status.GatewayStatus.Addresses).To(ContainElement("10.0.0.5"))
	})

	It("reconciles applications against an external Gateway reference", func() {
		ctx := context.Background()
		wandbName := "network-gateway-external"
		gatewayNamespace := "network-gateway-external-shared"
		gatewayName := "shared-gateway"

		createNamespaceIfMissing(ctx, gatewayNamespace)
		externalGateway := &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gatewayName,
				Namespace: gatewayNamespace,
			},
			Spec: gatewayv1.GatewaySpec{
				GatewayClassName: gatewayv1.ObjectName("shared-class"),
				Listeners: []gatewayv1.Listener{{
					Name:     gatewayv1.SectionName("http"),
					Port:     80,
					Protocol: gatewayv1.HTTPProtocolType,
				}},
			},
		}
		Expect(k8sClient.Create(ctx, externalGateway)).To(Succeed())
		DeferCleanup(deleteIfPresent, ctx, externalGateway)

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, externalGateway)).To(Succeed())
		wandb, objectStoreService := newNetworkingWandb(wandbName, "")
		wandb.Spec.Networking = apiv2.NetworkingSpec{
			Mode: apiv2.NetworkingModeGatewayAPI,
			GatewayAPI: &apiv2.GatewayAPIConfig{
				Gateway: apiv2.GatewayConfig{
					Managed: false,
					GatewayRef: &apiv2.GatewayReference{
						Name:      gatewayName,
						Namespace: gatewayNamespace,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
		Expect(k8sClient.Create(ctx, objectStoreService)).To(Succeed())
		DeferCleanup(deleteIfPresent, ctx, wandb)

		externalGateway.Status.Addresses = []gatewayv1.GatewayStatusAddress{{Value: "10.0.0.6"}}
		externalGateway.Status.Conditions = []metav1.Condition{{
			Type:               string(gatewayv1.GatewayConditionAccepted),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayv1.GatewayReasonAccepted),
			LastTransitionTime: metav1.Now(),
		}}
		Expect(k8sClient.Status().Update(ctx, externalGateway)).To(Succeed())

		wandb = markWandbReadyForNetworking(ctx, wandbName, wandbNamespace)
		reconcileNetworkingManifest(ctx, wandb)
		wandb = getWandb(ctx, wandbName, wandbNamespace)

		managedGateway := &gatewayv1.Gateway{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-gateway", wandbName), Namespace: wandbNamespace}, managedGateway)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		Expect(wandb.Status.GatewayStatus).NotTo(BeNil())
		Expect(wandb.Status.GatewayStatus.GatewayRef).NotTo(BeNil())
		Expect(wandb.Status.GatewayStatus.GatewayRef.Name).To(Equal(gatewayName))
		Expect(wandb.Status.GatewayStatus.GatewayRef.Namespace).To(Equal(gatewayNamespace))
		Expect(wandb.Status.GatewayStatus.Ready).To(BeTrue())
		Expect(wandb.Status.GatewayStatus.Addresses).To(ContainElement("10.0.0.6"))

		app := &apiv2.Application{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "api", Namespace: wandbNamespace}, app)).To(Succeed())
		Expect(app.Spec.HTTPRouteTemplate).NotTo(BeNil())
		Expect(app.Spec.HTTPRouteTemplate.ParentRefs).To(HaveLen(1))
		Expect(app.Spec.HTTPRouteTemplate.ParentRefs[0].Name).To(Equal(gatewayv1.ObjectName(gatewayName)))
		Expect(app.Spec.HTTPRouteTemplate.ParentRefs[0].Namespace).NotTo(BeNil())
		Expect(*app.Spec.HTTPRouteTemplate.ParentRefs[0].Namespace).To(Equal(gatewayv1.Namespace(gatewayNamespace)))
	})

	It("reconciles a consolidated Ingress and mirrors its load balancer status", func() {
		ctx := context.Background()
		wandbName := "network-ingress"
		ingressClassName := "nginx"

		wandb, service := newNetworkingWandb(wandbName, "")
		wandb.Spec.Networking = apiv2.NetworkingSpec{
			Mode: apiv2.NetworkingModeIngress,
			Ingress: &apiv2.IngressConfig{
				IngressClassName: &ingressClassName,
			},
			TLS: &apiv2.TLSConfig{
				SecretName: "wandb-tls",
			},
			Annotations: map[string]string{
				"example.com/ingress": "enabled",
			},
		}
		Expect(k8sClient.Create(ctx, wandb)).To(Succeed())
		Expect(k8sClient.Create(ctx, service)).To(Succeed())
		DeferCleanup(deleteIfPresent, ctx, wandb)

		wandb = markWandbReadyForNetworking(ctx, wandbName, wandbNamespace)
		reconcileNetworkingManifest(ctx, wandb)

		ingress := &networkingv1.Ingress{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      wandbName,
			Namespace: wandbNamespace,
		}, ingress)).To(Succeed())
		Expect(ingress.Spec.IngressClassName).NotTo(BeNil())
		Expect(*ingress.Spec.IngressClassName).To(Equal(ingressClassName))
		Expect(ingress.Annotations).To(HaveKeyWithValue("example.com/ingress", "enabled"))
		Expect(ingress.Spec.TLS).To(HaveLen(1))
		Expect(ingress.Spec.TLS[0].SecretName).To(Equal("wandb-tls"))
		Expect(ingress.Spec.Rules).NotTo(BeEmpty())

		ingress.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{
			IP: "34.118.10.1",
		}}
		Expect(k8sClient.Status().Update(ctx, ingress)).To(Succeed())

		wandb = getWandb(ctx, wandbName, wandbNamespace)
		reconcileNetworkingManifest(ctx, wandb)
		wandb = getWandb(ctx, wandbName, wandbNamespace)

		Expect(wandb.Status.IngressStatus).NotTo(BeNil())
		Expect(wandb.Status.IngressStatus.Name).To(Equal(wandbName))
		Expect(wandb.Status.IngressStatus.LoadBalancerIngress).To(HaveLen(1))
		Expect(wandb.Status.IngressStatus.LoadBalancerIngress[0].IP).To(Equal("34.118.10.1"))
	})
})

func newNetworkingWandb(name string, infraNamespace string) (*apiv2.WeightsAndBiases, *corev1.Service) {
	internalServiceAuthEnabled := false
	if infraNamespace == "" {
		infraNamespace = "default"
	}

	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: apiv2.WeightsAndBiasesSpec{
			Size: apiv2.SizeDev,
			Wandb: apiv2.WandbAppSpec{
				Hostname:           "http://localhost",
				Features:           map[string]bool{},
				ManifestRepository: manifestsRepository,
				Version:            "0.83.0-clickhouse-keeper.2",
				InternalServiceAuth: apiv2.InternalServiceAuth{
					Enabled: &internalServiceAuthEnabled,
				},
			},
			RetentionPolicy: apiv2.RetentionPolicy{
				OnDelete: apiv2.DetachOnDelete,
			},
			MySQL: apiv2.MySQLSpec{
				ManagedMysql: &apiv2.ManagedMysqlSpec{},
			},
			Redis: apiv2.RedisSpec{
				ManagedRedis: &apiv2.ManagedRedisSpec{},
			},
			Kafka: apiv2.KafkaSpec{
				ManagedKafka: &apiv2.ManagedKafkaSpec{},
			},
			ObjectStore: apiv2.ObjectStoreSpec{
				ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{
					Namespace: infraNamespace,
				},
			},
			ClickHouse: apiv2.ClickHouseSpec{
				ManagedClickHouse: &apiv2.ManagedClickHouseSpec{},
			},
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-seaweedfs-s3", name),
			Namespace: infraNamespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: "s3-http",
				Port: 80,
			}},
		},
	}

	return wandb, service
}

func markWandbReadyForNetworking(ctx context.Context, name, namespace string) *apiv2.WeightsAndBiases {
	wandb := getWandb(ctx, name, namespace)
	wandb.Status.MySQLStatus.Ready = true
	wandb.Status.RedisStatus.Ready = true
	wandb.Status.KafkaStatus.Ready = true
	wandb.Status.ObjectStoreStatus.Ready = true
	wandb.Status.ClickHouseStatus.Ready = true
	wandb.Status.MySQLStatus.Connection.URL = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  "mysql-url",
	}
	wandb.Status.ClickHouseStatus.Connection.URL = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  "clickhouse-url",
	}
	wandb.Status.Wandb.Migration.Version = wandb.Spec.Wandb.Version
	wandb.Status.Wandb.Migration.LastSuccessVersion = wandb.Spec.Wandb.Version
	wandb.Status.Wandb.Migration.Ready = true
	wandb.Status.Wandb.Migration.Reason = "Complete"
	wandb.Status.Wandb.MySQLInit.Succeeded = true
	Expect(k8sClient.Status().Update(ctx, wandb)).To(Succeed())

	return getWandb(ctx, name, namespace)
}

func reconcileNetworkingManifest(ctx context.Context, wandb *apiv2.WeightsAndBiases) {
	wandbManifest, err := manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
	Expect(err).NotTo(HaveOccurred())

	_, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
	Expect(err).NotTo(HaveOccurred())
}

func getWandb(ctx context.Context, name, namespace string) *apiv2.WeightsAndBiases {
	wandb := &apiv2.WeightsAndBiases{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, wandb)).To(Succeed())
	return wandb
}

func createNamespaceIfMissing(ctx context.Context, name string) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := k8sClient.Create(ctx, ns)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func deleteIfPresent(ctx context.Context, obj client.Object) {
	_ = k8sClient.Delete(ctx, obj)
}
