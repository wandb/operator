package reconciler

import (
	"context"
	"fmt"

	"github.com/wandb/operator/api/v2"
	"k8s.io/api/core/v1"
	v4 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v3 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	oidcDiscoveryClusterRoleName = "system:service-account-issuer-discovery"

	// Internal service auth uses projected service-account tokens, so issuer
	// discovery only needs authenticated Kubernetes callers.
	oidcDiscoverySubjectGroup = "system:authenticated"
)

// createOrUpdateServiceAccount creates or updates the ServiceAccount for the W&B applications
func createOrUpdateServiceAccount(
	ctx context.Context,
	client client.Client,
	wandb *v2.WeightsAndBiases,
	serviceAccountName string,
) error {
	log := controllerruntime.LoggerFrom(ctx)

	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: v3.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
			Annotations: wandb.Spec.Wandb.ServiceAccount.Annotations,
		},
		AutomountServiceAccountToken: ptr.To(false),
	}

	if err := controllerutil.SetControllerReference(wandb, serviceAccount, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference on ServiceAccount: %w", err)
	}

	existingServiceAccount := &v1.ServiceAccount{}
	if err := client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: wandb.Namespace}, existingServiceAccount); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating a new ServiceAccount", "Namespace", wandb.Namespace, "Name", serviceAccountName)
			if err := client.Create(ctx, serviceAccount); err != nil {
				return fmt.Errorf("failed to create ServiceAccount: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get existing ServiceAccount: %w", err)
	}

	existingServiceAccount.Annotations = serviceAccount.Annotations
	existingServiceAccount.Labels = serviceAccount.Labels
	existingServiceAccount.OwnerReferences = serviceAccount.OwnerReferences
	existingServiceAccount.AutomountServiceAccountToken = serviceAccount.AutomountServiceAccountToken
	log.Info("Updating existing ServiceAccount", "Namespace", wandb.Namespace, "Name", serviceAccountName)
	if err := client.Update(ctx, existingServiceAccount); err != nil {
		return fmt.Errorf("failed to update ServiceAccount: %w", err)
	}

	return nil
}

// createOrUpdateRole creates or updates the Role for the W&B service account
func createOrUpdateRole(
	ctx context.Context,
	client client.Client,
	wandb *v2.WeightsAndBiases,
	serviceAccountName string,
) error {
	log := controllerruntime.LoggerFrom(ctx)

	role := &v4.Role{
		ObjectMeta: v3.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		Rules: []v4.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get"},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(wandb, role, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference on Role: %w", err)
	}

	existingRole := &v4.Role{}
	err := client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: wandb.Namespace}, existingRole)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating Role", "name", serviceAccountName, "namespace", wandb.Namespace)
			if err := client.Create(ctx, role); err != nil {
				return fmt.Errorf("failed to create Role: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Role: %w", err)
	}

	// Update existing role
	existingRole.Rules = role.Rules
	existingRole.Labels = role.Labels
	log.Info("Updating Role", "name", serviceAccountName, "namespace", wandb.Namespace)
	if err := client.Update(ctx, existingRole); err != nil {
		return fmt.Errorf("failed to update Role: %w", err)
	}

	return nil
}

// createOrUpdateRoleBinding creates or updates the RoleBinding for the W&B service account
func createOrUpdateRoleBinding(
	ctx context.Context,
	client client.Client,
	wandb *v2.WeightsAndBiases,
	serviceAccountName string,
) error {
	log := controllerruntime.LoggerFrom(ctx)

	roleBinding := &v4.RoleBinding{
		ObjectMeta: v3.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		RoleRef: v4.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     serviceAccountName,
		},
		Subjects: []v4.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: wandb.Namespace,
			},
		},
	}

	if err := controllerutil.SetOwnerReference(wandb, roleBinding, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
	}

	existingRoleBinding := &v4.RoleBinding{}
	err := client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: wandb.Namespace}, existingRoleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating RoleBinding", "name", serviceAccountName, "namespace", wandb.Namespace)
			if err := client.Create(ctx, roleBinding); err != nil {
				return fmt.Errorf("failed to create RoleBinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get RoleBinding: %w", err)
	}

	// Update existing rolebinding
	existingRoleBinding.RoleRef = roleBinding.RoleRef
	existingRoleBinding.Subjects = roleBinding.Subjects
	existingRoleBinding.Labels = roleBinding.Labels
	log.Info("Updating RoleBinding", "name", serviceAccountName, "namespace", wandb.Namespace)
	if err := client.Update(ctx, existingRoleBinding); err != nil {
		return fmt.Errorf("failed to update RoleBinding: %w", err)
	}

	return nil
}

// createOrUpdateOIDCDiscoveryClusterRoleBinding creates or updates the ClusterRoleBinding
// for OIDC discovery. This is required for JWT token validation between W&B services.
// Returns error if creation fails, but this is non-fatal for reconciliation.
func createOrUpdateOIDCDiscoveryClusterRoleBinding(
	ctx context.Context,
	client client.Client,
	wandb *v2.WeightsAndBiases,
) error {
	log := controllerruntime.LoggerFrom(ctx)

	clusterRoleBindingName := fmt.Sprintf("%s-oidc-discovery", wandb.Name)

	clusterRoleBinding := &v4.ClusterRoleBinding{
		ObjectMeta: v3.ObjectMeta{
			Name: clusterRoleBindingName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		RoleRef: v4.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     oidcDiscoveryClusterRoleName,
		},
		Subjects: []v4.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     oidcDiscoverySubjectGroup,
			},
		},
	}

	// Note: ClusterRoleBinding cannot have ownerReferences to namespaced resources
	// It will be cleaned up manually or left as cluster-scoped resource

	existingClusterRoleBinding := &v4.ClusterRoleBinding{}
	err := client.Get(ctx, types.NamespacedName{Name: clusterRoleBindingName}, existingClusterRoleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating ClusterRoleBinding for OIDC discovery", "name", clusterRoleBindingName)
			if err := client.Create(ctx, clusterRoleBinding); err != nil {
				return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ClusterRoleBinding: %w", err)
	}

	// Update existing clusterrolebinding
	existingClusterRoleBinding.RoleRef = clusterRoleBinding.RoleRef
	existingClusterRoleBinding.Subjects = clusterRoleBinding.Subjects
	existingClusterRoleBinding.Labels = clusterRoleBinding.Labels
	log.Info("Updating ClusterRoleBinding for OIDC discovery", "name", clusterRoleBindingName)
	if err := client.Update(ctx, existingClusterRoleBinding); err != nil {
		return fmt.Errorf("failed to update ClusterRoleBinding: %w", err)
	}

	return nil
}
