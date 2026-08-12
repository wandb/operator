package reconciler

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func watchtowerTestClient(t *testing.T, objects ...ctrlClient.Object) ctrlClient.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(objects) > 0 {
		builder.WithObjects(objects...)
	}
	return builder.Build()
}

func newWandbForWatchtower(install bool) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
			UID:       "wandb-uid",
		},
		Spec: apiv2.WeightsAndBiasesSpec{
			Wandb: apiv2.WandbAppSpec{Hostname: "https://wandb.example.com"},
			Watchtower: apiv2.WatchtowerSpec{
				Install: ptr.To(install),
			},
		},
	}
}

// watchtowerTestManifest mirrors the published manifest's shape: the api
// application owns /oidc and therefore the auth sub-request endpoint.
func watchtowerTestManifest() serverManifest.Manifest {
	return serverManifest.Manifest{
		Applications: map[string]serverManifest.Application{
			"api": {
				Name: "api",
				Service: &serverManifest.ServiceSpec{
					Ports: []corev1.ServicePort{{Name: "api", Port: 8080}},
				},
				Ingress: &serverManifest.AppIngressSpec{
					Paths:       []string{"/api", "/graphql", "/oidc"},
					ServicePort: "8080",
					PathType:    "Prefix",
				},
			},
			"frontend": {
				Name: "frontend",
				Service: &serverManifest.ServiceSpec{
					Ports: []corev1.ServicePort{{Name: "frontend", Port: 8080}},
				},
				Ingress: &serverManifest.AppIngressSpec{
					Paths:       []string{"/"},
					ServicePort: "8080",
					PathType:    "Prefix",
				},
			},
		},
	}
}

func TestWatchtowerAuthServiceDerivedFromManifest(t *testing.T) {
	wandb := newWandbForWatchtower(true)

	authService, err := watchtowerAuthService(wandb, watchtowerTestManifest())
	require.NoError(t, err)
	require.Equal(t, "api:8080", authService)
}

func TestWatchtowerAuthServiceResolvesNamedServicePort(t *testing.T) {
	manifest := watchtowerTestManifest()
	api := manifest.Applications["api"]
	api.Ingress.ServicePort = "api"
	api.Service.Ports = []corev1.ServicePort{{Name: "api", Port: 8081}}
	manifest.Applications["api"] = api

	authService, err := watchtowerAuthService(newWandbForWatchtower(true), manifest)
	require.NoError(t, err)
	require.Equal(t, "api:8081", authService)
}

func TestWatchtowerAuthServiceSpecOverrideWins(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	wandb.Spec.Watchtower.AuthService = "wandb-api:8081"

	authService, err := watchtowerAuthService(wandb, watchtowerTestManifest())
	require.NoError(t, err)
	require.Equal(t, "wandb-api:8081", authService)
}

// Failing closed matters here: a Watchtower with no auth service would serve
// cluster administration to unauthenticated callers.
func TestWatchtowerAuthServiceErrorsWhenUnderivable(t *testing.T) {
	manifest := watchtowerTestManifest()
	delete(manifest.Applications, "api")

	_, err := watchtowerAuthService(newWandbForWatchtower(true), manifest)
	require.ErrorContains(t, err, "cannot derive spec.watchtower.authService")
}

func TestWatchtowerAuthServiceSkipsDisabledFeatureApps(t *testing.T) {
	manifest := watchtowerTestManifest()
	api := manifest.Applications["api"]
	api.Features = []string{"unavailable"}
	manifest.Applications["api"] = api

	_, err := watchtowerAuthService(newWandbForWatchtower(true), manifest)
	require.ErrorContains(t, err, "cannot derive spec.watchtower.authService")
}

func TestReconcileWatchtowerCreatesApplicationAndRBAC(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	c := watchtowerTestClient(t)

	require.NoError(t, reconcileWatchtower(context.Background(), c, wandb, watchtowerTestManifest()))

	app := &apiv2.Application{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, app))

	require.Equal(t, "Deployment", app.Spec.Kind)
	require.Equal(t, int32(1), *app.Spec.Replicas)
	require.Equal(t, watchtowerComponent, app.Labels[common.WandbComponentLabel])
	require.Len(t, app.Spec.PodTemplate.Spec.Containers, 1)

	container := app.Spec.PodTemplate.Spec.Containers[0]
	require.Equal(t, apiv2.DefaultWatchtowerImageRepository+":"+apiv2.DefaultWatchtowerImageTag, container.Image)
	require.Equal(t, "/watchtower/healthz", container.LivenessProbe.HTTPGet.Path)
	require.Equal(t, "/watchtower/ready", container.ReadinessProbe.HTTPGet.Path)

	env := map[string]string{}
	for _, e := range container.Env {
		env[e.Name] = e.Value
	}
	require.Equal(t, "cluster", env["WATCHTOWER_MODE"])
	require.Equal(t, "/watchtower", env["WATCHTOWER_BASE_PATH"])
	require.Equal(t, "api:8080", env["WATCHTOWER_AUTH_SERVICE"])

	require.Equal(t, apiv2.DefaultWatchtowerServiceAccountName, app.Spec.PodTemplate.Spec.ServiceAccountName)
	require.NotNil(t, app.Spec.ServiceTemplate)
	require.Equal(t, watchtowerContainerPort, app.Spec.ServiceTemplate.Ports[0].Port)

	sa := &corev1.ServiceAccount{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: apiv2.DefaultWatchtowerServiceAccountName, Namespace: "default"}, sa))
	require.True(t, *sa.AutomountServiceAccountToken)

	role := &rbacv1.Role{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, role))

	clusterRole := &rbacv1.ClusterRole{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: "default-wandb-watchtower"}, clusterRole))

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: "default-wandb-watchtower"}, clusterRoleBinding))
	require.Equal(t, apiv2.DefaultWatchtowerServiceAccountName, clusterRoleBinding.Subjects[0].Name)

	// The apiserver rejects a binding that grants more than the operator holds,
	// and the operator only has "get" on these subresources.
	for _, rule := range clusterRole.Rules {
		if slices.Contains(rule.Resources, "weightsandbiases/status") {
			require.Equal(t, []string{"get"}, rule.Verbs)
		}
	}

	require.NotNil(t, wandb.Status.WatchtowerStatus)
	require.Equal(t, "https://wandb.example.com/watchtower", wandb.Status.WatchtowerStatus.URL)
	require.Equal(t, "api:8080", wandb.Status.WatchtowerStatus.AuthService)
}

func TestReconcileWatchtowerIsIdempotent(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	c := watchtowerTestClient(t)
	ctx := context.Background()

	require.NoError(t, reconcileWatchtower(ctx, c, wandb, watchtowerTestManifest()))
	first := &apiv2.Application{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, first))

	require.NoError(t, reconcileWatchtower(ctx, c, wandb, watchtowerTestManifest()))
	second := &apiv2.Application{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, second))

	require.Equal(t, first.ResourceVersion, second.ResourceVersion)
}

func TestReconcileWatchtowerHonorsGlobalImageRegistry(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	wandb.Spec.Global.ImageRegistry = "registry.internal"
	c := watchtowerTestClient(t)

	require.NoError(t, reconcileWatchtower(context.Background(), c, wandb, watchtowerTestManifest()))

	app := &apiv2.Application{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, app))
	require.Equal(t,
		"registry.internal/"+apiv2.DefaultWatchtowerImageRepository+":"+apiv2.DefaultWatchtowerImageTag,
		app.Spec.PodTemplate.Spec.Containers[0].Image,
	)
}

func TestReconcileWatchtowerBuildsHTTPRouteInGatewayMode(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	wandb.Spec.Networking.Mode = apiv2.NetworkingModeGatewayAPI
	wandb.Spec.Networking.GatewayAPI = &apiv2.GatewayAPIConfig{}
	wandb.Status.GatewayStatus = &apiv2.GatewayStatusSummary{
		GatewayRef: &apiv2.GatewayReference{Name: "wandb-gateway"},
	}
	c := watchtowerTestClient(t)

	require.NoError(t, reconcileWatchtower(context.Background(), c, wandb, watchtowerTestManifest()))

	app := &apiv2.Application{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, app))

	route := app.Spec.HTTPRouteTemplate
	require.NotNil(t, route)
	require.Equal(t, []string{"/watchtower"}, route.Paths)
	require.Equal(t, []gatewayv1.Hostname{"wandb.example.com"}, route.Hostnames)
	require.Equal(t, gatewayv1.PortNumber(watchtowerContainerPort), *route.ServicePort)
}

func TestReconcileWatchtowerOmitsHTTPRouteInIngressMode(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	wandb.Spec.Networking.Mode = apiv2.NetworkingModeIngress
	c := watchtowerTestClient(t)

	require.NoError(t, reconcileWatchtower(context.Background(), c, wandb, watchtowerTestManifest()))

	app := &apiv2.Application{}
	require.NoError(t, c.Get(context.Background(),
		types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, app))
	require.Nil(t, app.Spec.HTTPRouteTemplate)
}

func TestReconcileWatchtowerDisabledRemovesResources(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	c := watchtowerTestClient(t)
	ctx := context.Background()

	require.NoError(t, reconcileWatchtower(ctx, c, wandb, watchtowerTestManifest()))

	wandb.Spec.Watchtower.Install = ptr.To(false)
	require.NoError(t, reconcileWatchtower(ctx, c, wandb, watchtowerTestManifest()))

	err := c.Get(ctx, types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, &apiv2.Application{})
	require.True(t, apiErrors.IsNotFound(err))

	err = c.Get(ctx, types.NamespacedName{Name: "default-wandb-watchtower"}, &rbacv1.ClusterRole{})
	require.True(t, apiErrors.IsNotFound(err), "cluster-scoped RBAC has no owner ref, so it must be deleted explicitly")

	err = c.Get(ctx, types.NamespacedName{Name: "default-wandb-watchtower"}, &rbacv1.ClusterRoleBinding{})
	require.True(t, apiErrors.IsNotFound(err))

	err = c.Get(ctx, types.NamespacedName{
		Name: apiv2.DefaultWatchtowerServiceAccountName, Namespace: "default"}, &corev1.ServiceAccount{})
	require.True(t, apiErrors.IsNotFound(err))

	require.Nil(t, wandb.Status.WatchtowerStatus)
}

func TestReconcileWatchtowerDisabledIsANoopWhenAbsent(t *testing.T) {
	wandb := newWandbForWatchtower(false)
	c := watchtowerTestClient(t)

	require.NoError(t, reconcileWatchtower(context.Background(), c, wandb, watchtowerTestManifest()))
	require.Nil(t, wandb.Status.WatchtowerStatus)
}

func TestReconcileWatchtowerKeepsUserSuppliedServiceAccount(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	wandb.Spec.Watchtower.ServiceAccount = apiv2.ManagedServiceAccountSpec{
		Create:             ptr.To(false),
		ServiceAccountName: "byo-watchtower",
	}
	c := watchtowerTestClient(t)
	ctx := context.Background()

	require.NoError(t, reconcileWatchtower(ctx, c, wandb, watchtowerTestManifest()))

	err := c.Get(ctx, types.NamespacedName{Name: "byo-watchtower", Namespace: "default"}, &corev1.ServiceAccount{})
	require.True(t, apiErrors.IsNotFound(err), "the operator must not create an account it was told not to manage")

	app := &apiv2.Application{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: watchtowerAppName, Namespace: "default"}, app))
	require.Equal(t, "byo-watchtower", app.Spec.PodTemplate.Spec.ServiceAccountName)
}

func TestWatchtowerIngressPathDisabled(t *testing.T) {
	require.Nil(t, watchtowerIngressPath(newWandbForWatchtower(false)))
}

func TestWatchtowerIngressPathHonorsBasePath(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	wandb.Spec.Watchtower.BasePath = "/admin/"

	path := watchtowerIngressPath(wandb)
	require.NotNil(t, path)
	require.Equal(t, "/admin", path.Path)
	require.Equal(t, watchtowerAppName, path.Backend.Service.Name)
	require.Equal(t, watchtowerContainerPort, path.Backend.Service.Port.Number)
}

func TestConsolidatedIngressIncludesWatchtowerPath(t *testing.T) {
	wandb := newWandbForWatchtower(true)
	wandb.Spec.Networking.Mode = apiv2.NetworkingModeIngress
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, networkingv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	require.NoError(t, reconcileConsolidatedIngress(ctx, c, wandb, watchtowerTestManifest()))

	ingress := &networkingv1.Ingress{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "wandb", Namespace: "default"}, ingress))

	backends := map[string]string{}
	for _, path := range ingress.Spec.Rules[0].HTTP.Paths {
		backends[path.Path] = path.Backend.Service.Name
	}
	require.Equal(t, watchtowerAppName, backends["/watchtower"])
	require.Equal(t, "frontend", backends["/"], "the W&B frontend must keep serving the root path")
}

func TestConsolidatedIngressOmitsWatchtowerPathWhenDisabled(t *testing.T) {
	wandb := newWandbForWatchtower(false)
	wandb.Spec.Networking.Mode = apiv2.NetworkingModeIngress
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, networkingv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	require.NoError(t, reconcileConsolidatedIngress(ctx, c, wandb, watchtowerTestManifest()))

	ingress := &networkingv1.Ingress{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "wandb", Namespace: "default"}, ingress))
	for _, path := range ingress.Spec.Rules[0].HTTP.Paths {
		require.NotEqual(t, "/watchtower", path.Path)
	}
}

func TestWatchtowerResolvedBasePathNormalizes(t *testing.T) {
	require.Equal(t, "/watchtower", apiv2.WatchtowerSpec{}.ResolvedBasePath())
	require.Equal(t, "/admin", apiv2.WatchtowerSpec{BasePath: "admin"}.ResolvedBasePath())
	require.Equal(t, "/admin", apiv2.WatchtowerSpec{BasePath: "/admin/"}.ResolvedBasePath())
}

func TestWatchtowerGetImagePrefersDigest(t *testing.T) {
	spec := apiv2.WatchtowerSpec{
		Image: apiv2.WatchtowerImageSpec{
			Repository: "wandb/watchtower",
			Tag:        "0.11.0",
			Digest:     "sha256:abc",
		},
	}
	require.Equal(t, "wandb/watchtower@sha256:abc", spec.GetImage(""))
}
