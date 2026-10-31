package retention

import (
	"context"
	"testing"

	gkev1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func routeFixture(t *testing.T, managed bool, policy apiv2.OnDeletePolicy) (*Session, *gatewayv1.HTTPRoute, *gatewayv1.Gateway, *corev1.Service) {
	t.Helper()
	ctx := context.Background()
	w, c, _ := lifecycleFixture(t, apiv2.DetachOnDelete)
	require.NoError(t, gatewayv1.Install(c.Scheme()))
	require.NoError(t, gkev1.Install(c.Scheme()))
	w.Annotations = map[string]string{trackingIDAnnotation: "app:apps.wandb.com/WeightsAndBiases:test/wandb"}
	s, err := Open(ctx, c, c, w)
	require.NoError(t, err)
	// This Service represents a discovered, verified vendor child in another namespace.
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "store-s3", Namespace: "infra", UID: "service-uid"}}
	backend := Resource{APIVersion: "v1", Kind: "Service", Namespace: service.Namespace, Name: service.Name, UID: service.UID, Component: "objectstore", Instance: "default", Policy: policy, Stage: 2}
	associate(service, w, backend)
	require.NoError(t, c.Create(ctx, service))
	s.Ledger.Resources[backend.Key()] = backend
	gw := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway", Namespace: w.Namespace, UID: "gateway-uid"}}
	if managed {
		gw.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases", Name: w.Name, UID: w.UID}}
	}
	require.NoError(t, c.Create(ctx, gw))
	ns := gatewayv1.Namespace(gw.Namespace)
	route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: "wandb-bucket-default", Namespace: service.Namespace, UID: "route-uid", Labels: map[string]string{"app.kubernetes.io/managed-by": "wandb-operator", "app.kubernetes.io/instance": w.Name}}, Spec: gatewayv1.HTTPRouteSpec{CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: []gatewayv1.ParentReference{{Name: gatewayv1.ObjectName(gw.Name), Namespace: &ns}}}, Rules: []gatewayv1.HTTPRouteRule{{BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(service.Name)}}}}}}}}
	require.NoError(t, c.Create(ctx, route))
	require.NoError(t, s.save(ctx))
	return s, route, gw, service
}

func finishRoutes(t *testing.T, s *Session) {
	t.Helper()
	for range 20 {
		done, err := s.Finalize(context.Background())
		require.NoError(t, err)
		if done {
			return
		}
	}
	t.Fatal("finalization did not finish")
}

func TestRouteRetentionFollowsGatewayOwnershipAndBackendPolicy(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, policy := range []apiv2.OnDeletePolicy{apiv2.DetachOnDelete, apiv2.PurgeOnDelete} {
			name := "external/" + string(policy)
			if managed {
				name = "managed/" + string(policy)
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				s, route, gw, service := routeFixture(t, managed, policy)
				require.NoError(t, s.Reconcile(ctx))
				require.NoError(t, s.CheckReady(ctx))
				require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
				require.Empty(t, route.OwnerReferences)
				require.Equal(t, s.parent.Annotations[trackingIDAnnotation], route.Annotations[trackingIDAnnotation])
				require.True(t, argoProtected(route, s.parent))
				r, ok, err := s.objectResource(route)
				require.NoError(t, err)
				require.True(t, ok)
				want := policy
				if managed {
					want = apiv2.PurgeOnDelete
				}
				require.Equal(t, want, r.Policy)
				require.Equal(t, managed, r.RemoveWithGateway != "")
				// Restart with an ownerless route: cleanup must use persisted identity.
				s, err = Open(ctx, s.Client, s.reader, s.parent)
				require.NoError(t, err)
				finishRoutes(t, s)
				err = s.Get(ctx, client.ObjectKeyFromObject(route), &gatewayv1.HTTPRoute{})
				if want == apiv2.PurgeOnDelete {
					require.True(t, apierrors.IsNotFound(err))
				} else {
					require.NoError(t, err)
				}
				err = s.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{})
				if policy == apiv2.PurgeOnDelete {
					require.True(t, apierrors.IsNotFound(err))
				} else {
					require.NoError(t, err)
				}
				require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(gw), gw), "route finalization does not delete a Gateway")
			})
		}
	}
}

func TestManagedRouteCleanupSurvivesGatewayGCAndFinalizerRetry(t *testing.T) {
	ctx := context.Background()
	s, route, gw, service := routeFixture(t, true, apiv2.DetachOnDelete)
	route.Finalizers = []string{"test/wait"}
	require.NoError(t, s.Client.Update(ctx, route))
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, s.Client.Delete(ctx, gw))
	done, err := s.Finalize(ctx)
	require.NoError(t, err)
	require.False(t, done)
	s, err = Open(ctx, s.Client, s.reader, s.parent)
	require.NoError(t, err)
	done, err = s.Finalize(ctx)
	require.NoError(t, err)
	require.False(t, done)
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
	require.False(t, route.DeletionTimestamp.IsZero())
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(service), service))
	route.Finalizers = nil
	require.NoError(t, s.Client.Update(ctx, route))
	finishRoutes(t, s)
}

func TestRoutesRejectReplacedUIDsAndIgnoreUnrelatedRoutes(t *testing.T) {
	ctx := context.Background()
	s, route, _, _ := routeFixture(t, false, apiv2.PurgeOnDelete)
	for _, name := range []string{"customer-route", "other-installation"} {
		other := route.DeepCopy()
		other.Name = name
		other.ResourceVersion = ""
		other.UID = types.UID(name)
		if name == "customer-route" {
			other.Labels = nil
		} else {
			other.Labels[UIDLabel] = "other-uid"
		}
		require.NoError(t, s.Client.Create(ctx, other))
	}
	require.NoError(t, s.Reconcile(ctx))
	for _, r := range s.Ledger.Resources {
		require.NotEqual(t, "customer-route", r.Name)
		require.NotEqual(t, "other-installation", r.Name)
	}
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
	require.NoError(t, s.Client.Delete(ctx, route))
	route.UID = "replacement"
	route.ResourceVersion = ""
	require.NoError(t, s.Client.Create(ctx, route))
	_, err := s.Finalize(ctx)
	require.ErrorContains(t, err, "unverified")
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
	require.True(t, route.DeletionTimestamp.IsZero())
}

func TestExternalRoutesUpdatePoliciesIndependently(t *testing.T) {
	ctx := context.Background()
	s, route, _, service := routeFixture(t, false, apiv2.DetachOnDelete)
	require.NoError(t, s.Reconcile(ctx))
	second := route.DeepCopy()
	second.Name = "wandb-clickhouse-default"
	second.ResourceVersion = ""
	second.UID = "second-route"
	second.Spec.Rules[0].BackendRefs[0].Name = "clickhouse"
	require.NoError(t, s.Client.Create(ctx, second))
	ch := service.DeepCopy()
	ch.Name = "clickhouse"
	ch.UID = "clickhouse-service"
	ch.ResourceVersion = ""
	backend := Resource{APIVersion: "v1", Kind: "Service", Namespace: ch.Namespace, Name: ch.Name, UID: ch.UID, Component: "clickhouse", Instance: "default", Policy: apiv2.PurgeOnDelete, Stage: 2}
	associate(ch, s.parent, backend)
	require.NoError(t, s.Client.Create(ctx, ch))
	s.Ledger.Resources[backend.Key()] = backend
	require.NoError(t, s.Reconcile(ctx))
	r, _, _ := s.objectResource(route)
	require.Equal(t, apiv2.DetachOnDelete, r.Policy)
	r, _, _ = s.objectResource(second)
	require.Equal(t, apiv2.PurgeOnDelete, r.Policy)
	finishRoutes(t, s)
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
	require.True(t, apierrors.IsNotFound(s.Get(ctx, client.ObjectKeyFromObject(second), second)))
}

func TestHealthCheckPolicyMigratesWithRoute(t *testing.T) {
	ctx := context.Background()
	s, route, _, service := routeFixture(t, false, apiv2.DetachOnDelete)
	policy := &gkev1.HealthCheckPolicy{ObjectMeta: metav1.ObjectMeta{Name: route.Name, Namespace: route.Namespace, UID: "healthcheck", Labels: map[string]string{"app.kubernetes.io/managed-by": "wandb-operator", "app.kubernetes.io/instance": s.parent.Name, common.WandbComponentLabel: "infra-healthcheck-policy"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases", Name: s.parent.Name, UID: s.parent.UID}}}}
	policy.Spec.TargetRef.Kind = "Service"
	policy.Spec.TargetRef.Name = gatewayv1.ObjectName(service.Name)
	require.NoError(t, s.Client.Create(ctx, policy))
	require.NoError(t, s.Reconcile(ctx))
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(policy), policy))
	require.Empty(t, policy.OwnerReferences)
	require.True(t, argoProtected(policy, s.parent))
	finishRoutes(t, s)
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(policy), policy))
}

func TestPrepareRoutePersistsSafeIdentityBeforeCreate(t *testing.T) {
	ctx := context.Background()
	s, route, _, _ := routeFixture(t, false, apiv2.DetachOnDelete)
	require.NoError(t, s.Client.Delete(ctx, route))
	route.ResourceVersion = ""
	require.NoError(t, s.PrepareRoute(ctx, route, "objectstore", "default"))
	require.NoError(t, s.Create(ctx, route))
	require.NoError(t, s.Reconcile(ctx))
	r, _, err := s.objectResource(route)
	require.NoError(t, err)
	require.Equal(t, route.UID, r.UID)
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
	require.Empty(t, route.OwnerReferences)
	require.True(t, argoProtected(route, s.parent))
}

func TestRoutePolicyChangesBeforeDeletion(t *testing.T) {
	ctx := context.Background()
	s, route, _, service := routeFixture(t, false, apiv2.DetachOnDelete)
	require.NoError(t, s.Reconcile(ctx))
	backend, _, err := s.objectResource(service)
	require.NoError(t, err)
	backend.Policy = apiv2.PurgeOnDelete
	s.Ledger.Resources[backend.Key()] = backend
	require.NoError(t, s.Reconcile(ctx))
	r, _, err := s.objectResource(route)
	require.NoError(t, err)
	require.Equal(t, apiv2.PurgeOnDelete, r.Policy)
	finishRoutes(t, s)
	require.True(t, apierrors.IsNotFound(s.Get(ctx, client.ObjectKeyFromObject(route), route)))
}

func TestLegacyRouteRequiresVerifiedLiveBackend(t *testing.T) {
	ctx := context.Background()
	s, route, _, service := routeFixture(t, false, apiv2.PurgeOnDelete)
	require.NoError(t, s.Client.Delete(ctx, service))
	service.UID = "replacement"
	service.ResourceVersion = ""
	require.NoError(t, s.Client.Create(ctx, service))
	err := s.discoverRoutes(ctx, map[string]bool{route.Namespace: true})
	require.ErrorContains(t, err, "backend")
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
	require.Empty(t, route.Labels[UIDLabel])
}

func TestExternalRouteCanBeExplicitlyReadopted(t *testing.T) {
	ctx := context.Background()
	s, route, _, service := routeFixture(t, false, apiv2.DetachOnDelete)
	require.NoError(t, s.Reconcile(ctx))
	finishRoutes(t, s)
	backend, _, err := s.objectResource(service)
	require.NoError(t, err)
	old := s.parent.DeepCopy()
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(old), old))
	old.Finalizers = nil
	require.NoError(t, s.Client.Update(ctx, old))
	require.NoError(t, s.Client.Delete(ctx, old))
	next := s.parent.DeepCopy()
	next.UID = "new-installation"
	next.ResourceVersion = ""
	next.Annotations[AdoptFromAnnotation] = string(s.parent.UID)
	require.NoError(t, s.Client.Create(ctx, next))
	adopter, err := Open(ctx, s.Client, s.reader, next)
	require.NoError(t, err)
	adopter.Ledger.Resources[backend.Key()] = backend
	require.NoError(t, adopter.Reconcile(ctx))
	require.NoError(t, adopter.CheckReady(ctx))
	require.NoError(t, s.Get(ctx, client.ObjectKeyFromObject(route), route))
	require.Equal(t, string(next.UID), route.Labels[UIDLabel])
	require.Equal(t, types.UID("route-uid"), route.UID)
	require.Empty(t, route.OwnerReferences)
}
