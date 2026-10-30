package retention

import (
	"context"
	"fmt"

	gkev1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// PrepareRoute records intent before a manifest-driven route is written. Routes
// have their own instance so purging an endpoint never changes its data policy.
func (s *Session) PrepareRoute(ctx context.Context, route *gatewayv1.HTTPRoute, component, instance string) error {
	r := Resource{APIVersion: gatewayv1.GroupVersion.String(), Kind: "HTTPRoute", Namespace: route.Namespace, Name: route.Name, Component: "networking", Instance: InstanceID(component, instance), RouteFor: InstanceID(component, instance), Stage: 2}
	if old, ok := s.Ledger.Resources[r.Key()]; ok {
		if old.RouteFor != r.RouteFor {
			return fmt.Errorf("route %s cannot move between infrastructure instances", r.Key())
		}
		r.UID = old.UID
		r.RemoveWithGateway = old.RemoveWithGateway
	}
	if err := s.routePolicy(ctx, route, &r); err != nil {
		return err
	}
	s.Ledger.Resources[r.Key()] = r
	return s.save(ctx)
}

func (s *Session) PrepareRouteSupport(ctx context.Context, obj client.Object, route *gatewayv1.HTTPRoute) error {
	r, ok := s.Ledger.Resources[(Resource{APIVersion: gatewayv1.GroupVersion.String(), Kind: "HTTPRoute", Namespace: route.Namespace, Name: route.Name}).Key()]
	if !ok {
		return fmt.Errorf("route %s/%s has no retention inventory", route.Namespace, route.Name)
	}
	gvk, err := s.GroupVersionKindFor(obj)
	if err != nil {
		return err
	}
	r.APIVersion, r.Kind, r.Namespace, r.Name = gvk.GroupVersion().String(), gvk.Kind, obj.GetNamespace(), obj.GetName()
	r.UID = s.Ledger.Resources[r.Key()].UID
	s.Ledger.Resources[r.Key()] = r
	return s.save(ctx)
}

func (s *Session) routePolicy(ctx context.Context, route *gatewayv1.HTTPRoute, r *Resource) error {
	found := false
	for _, backend := range s.Ledger.Resources {
		if backend.Component != "networking" && InstanceID(backend.Component, backend.Instance) == r.RouteFor {
			r.Policy, found = backend.Policy, true
			break
		}
	}
	if !found {
		return fmt.Errorf("route %s has no verified backend instance %s", r.Key(), r.RouteFor)
	}
	remove := ""
	for _, ref := range route.Spec.ParentRefs {
		if (ref.Group != nil && *ref.Group != gatewayv1.GroupName) || (ref.Kind != nil && *ref.Kind != "Gateway") {
			continue
		}
		ns := route.Namespace
		if ref.Namespace != nil {
			ns = string(*ref.Namespace)
		}
		gw := &gatewayv1.Gateway{}
		if err := s.reader.Get(ctx, client.ObjectKey{Namespace: ns, Name: string(ref.Name)}, gw); apierrors.IsNotFound(err) {
			// GC can remove the managed Gateway before our finalizer gets its turn.
			if r.RemoveWithGateway == ns+"/"+string(ref.Name) {
				remove = r.RemoveWithGateway
			}
			continue
		} else if err != nil {
			return err
		}
		if ownedBy(gw, s.parent.UID) || gw.Labels[UIDLabel] == string(s.parent.UID) {
			remove = ns + "/" + string(ref.Name)
		}
	}
	r.RemoveWithGateway = remove
	if remove != "" {
		r.Policy = apiv2.PurgeOnDelete
	}
	return nil
}

// Legacy routes had no persisted owner because reconciliation changed a
// temporary object. Require both operator metadata and a verified backend;
// a route name or a reference to a shared Gateway is not deletion authority.
func (s *Session) legacyRouteCandidate(obj client.Object, component string) bool {
	if uid := obj.GetLabels()[UIDLabel]; uid != "" && uid != string(s.parent.UID) && uid != s.parent.Annotations[AdoptFromAnnotation] {
		return false
	}
	if ownedBy(obj, s.parent.UID) {
		return true
	}
	labels := obj.GetLabels()
	if labels["app.kubernetes.io/managed-by"] != "wandb-operator" || labels["app.kubernetes.io/instance"] != s.parent.Name {
		return false
	}
	if ns := labels[common.WandbNamespaceLabel]; ns != "" && ns != s.parent.Namespace {
		return false
	}
	if name := labels[common.WandbNameLabel]; name != "" && name != s.parent.Name {
		return false
	}
	if c := labels[common.WandbComponentLabel]; c != "" && c != component {
		return false
	}
	return true
}

func (s *Session) routeBackend(ctx context.Context, route *gatewayv1.HTTPRoute) (string, error) {
	instance := ""
	for _, rule := range route.Spec.Rules {
		for _, ref := range rule.BackendRefs {
			if (ref.Group != nil && *ref.Group != "") || (ref.Kind != nil && *ref.Kind != "Service") {
				continue
			}
			ns := route.Namespace
			if ref.Namespace != nil {
				ns = string(*ref.Namespace)
			}
			backend, ok := s.Ledger.Resources[(Resource{APIVersion: "v1", Kind: "Service", Namespace: ns, Name: string(ref.Name)}).Key()]
			if !ok || backend.UID == "" || backend.Supplied || (backend.Component != "objectstore" && backend.Component != "clickhouse") {
				continue
			}
			live := resourceObject(backend)
			if err := s.reader.Get(ctx, client.ObjectKeyFromObject(live), live); err != nil {
				return "", err
			}
			if live.GetUID() != backend.UID || !s.trusted(live, backend) {
				return "", fmt.Errorf("route backend %s has not completed retention migration", backend.Key())
			}
			id := InstanceID(backend.Component, backend.Instance)
			if instance != "" && instance != id {
				return "", fmt.Errorf("route %s/%s has backends in multiple infrastructure instances", route.Namespace, route.Name)
			}
			instance = id
		}
	}
	return instance, nil
}

func (s *Session) discoverRoutes(ctx context.Context, namespaces map[string]bool) error {
	if !s.Scheme().Recognizes(schema.GroupVersionKind{Group: gatewayv1.GroupName, Version: "v1", Kind: "HTTPRoute"}) {
		return nil
	}
	for ns := range namespaces {
		routes := &gatewayv1.HTTPRouteList{}
		if err := s.reader.List(ctx, routes, client.InNamespace(ns)); meta.IsNoMatchError(err) {
			return nil
		} else if err != nil {
			return err
		}
		for i := range routes.Items {
			route := &routes.Items[i]
			r := Resource{APIVersion: gatewayv1.GroupVersion.String(), Kind: "HTTPRoute", Namespace: ns, Name: route.Name, Component: "networking", Stage: 2}
			old, known := s.Ledger.Resources[r.Key()]
			if !known && !s.legacyRouteCandidate(route, "infra-route") {
				continue
			}
			if known {
				r = old
			} else {
				backend, err := s.routeBackend(ctx, route)
				if err != nil {
					return err
				}
				if backend == "" {
					// Application routes have their own lifecycle and are not infrastructure.
					if ownedBy(route, s.parent.UID) || route.Labels[common.WandbComponentLabel] == "infra-route" {
						return fmt.Errorf("cannot verify backend provenance for HTTPRoute %s/%s", ns, route.Name)
					}
					continue
				}
				r.RouteFor, r.Instance, r.UID = backend, backend, route.UID
			}
			if !s.Ledger.Deleting || !known {
				if err := s.routePolicy(ctx, route, &r); err != nil {
					return err
				}
			}
			if err := s.reconcileRouteObject(ctx, r, route); err != nil {
				return err
			}
			if err := s.discoverRouteHealthCheck(ctx, route, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Session) discoverRouteHealthCheck(ctx context.Context, route *gatewayv1.HTTPRoute, r Resource) error {
	gvk := schema.GroupVersionKind{Group: "networking.gke.io", Version: "v1", Kind: "HealthCheckPolicy"}
	if !s.Scheme().Recognizes(gvk) {
		return nil
	}
	policy := &gkev1.HealthCheckPolicy{}
	if err := s.reader.Get(ctx, client.ObjectKeyFromObject(route), policy); apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
		return nil
	} else if err != nil {
		return err
	}
	r.APIVersion, r.Kind = gvk.GroupVersion().String(), gvk.Kind
	if old, known := s.Ledger.Resources[r.Key()]; known {
		r.UID = old.UID
		if s.Ledger.Deleting {
			r = old
		}
	} else {
		if !s.legacyRouteCandidate(policy, "infra-healthcheck-policy") {
			return nil
		}
		matches := false
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.BackendRefs {
				if string(ref.Name) == string(policy.Spec.TargetRef.Name) && (ref.Namespace == nil || string(*ref.Namespace) == policy.Namespace) && policy.Spec.TargetRef.Kind == "Service" {
					matches = true
				}
			}
		}
		if !matches {
			return fmt.Errorf("cannot verify target provenance for HealthCheckPolicy %s/%s", policy.Namespace, policy.Name)
		}
		r.UID = policy.UID
	}
	return s.reconcileRouteObject(ctx, r, policy)
}

// A purge may wait on another controller's finalizer across restarts.
func (s *Session) reconcileRouteObject(ctx context.Context, r Resource, obj client.Object) error {
	if s.Ledger.Deleting && r.Policy == apiv2.PurgeOnDelete && !obj.GetDeletionTimestamp().IsZero() {
		if r.UID != obj.GetUID() || !s.trusted(obj, r) {
			return fmt.Errorf("refusing terminating route with unverified identity %s", r.Key())
		}
		s.Ledger.Resources[r.Key()] = r
		return s.save(ctx)
	}
	return s.reconcileObject(ctx, r, obj)
}
