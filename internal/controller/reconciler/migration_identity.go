package reconciler

import (
	"context"
	"fmt"
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func migrationServiceAccountName(wandb *apiv2.WeightsAndBiases, migration string, identity *servermanifest.MigrationServiceAccount) string {
	if identity.Name != "" {
		return identity.Name
	}
	return common.FitDefaultInfraName(wandb.Name+"-migration-"+migration, "", 63)
}

func migrationPermissions(identity *servermanifest.MigrationServiceAccount) []rbacv1.PolicyRule {
	rules := make([]rbacv1.PolicyRule, len(identity.Permissions))
	for i := range identity.Permissions {
		identity.Permissions[i].DeepCopyInto(&rules[i])
		if len(rules[i].APIGroups) == 0 {
			rules[i].APIGroups = []string{""}
		}
	}
	return rules
}

func validateMigrationServiceAccounts(wandb *apiv2.WeightsAndBiases, manifest servermanifest.Manifest) error {
	names := map[string]string{}
	for migration, task := range manifest.Migrations {
		identity := task.ServiceAccount
		if identity == nil {
			continue
		}
		if name := identity.Name; name != "" {
			if problems := validation.IsDNS1123Subdomain(name); len(problems) != 0 {
				return fmt.Errorf("migration %s: invalid service account name %q: %v", migration, name, problems)
			}
		}
		name := migrationServiceAccountName(wandb, migration, identity)
		if name == wandb.Spec.Wandb.ServiceAccount.ServiceAccountName {
			return fmt.Errorf("migration %s: dedicated account collides with shared account %s", migration, name)
		}
		if previous, exists := names[name]; exists {
			return fmt.Errorf("migrations %s and %s declare the same service account %s", previous, migration, name)
		}
		names[name] = migration
		for _, rule := range identity.Permissions {
			if len(rule.NonResourceURLs) != 0 || len(rule.Resources) == 0 || len(rule.Verbs) == 0 {
				return fmt.Errorf("migration %s: permissions require resources and verbs; nonResourceURLs cannot be granted by a namespaced Role", migration)
			}
		}
	}
	return nil
}

func reconcileMigrationServiceAccounts(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, manifest servermanifest.Manifest) error {
	if err := validateMigrationServiceAccounts(wandb, manifest); err != nil {
		return err
	}
	var reconciledNames []string
	for migration, task := range manifest.Migrations {
		identity := task.ServiceAccount
		if identity == nil {
			continue
		}
		name := migrationServiceAccountName(wandb, migration, identity)
		reconciledNames = append(reconciledNames, name)
		sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace}}
		if err := reconcileMigrationIdentityObject(ctx, c, wandb, sa, func() {
			sa.AutomountServiceAccountToken = ptr.To(ptr.Deref(identity.AutomountServiceAccountToken, false))
			sa.ImagePullSecrets = wandb.Spec.Global.ImagePullSecrets
		}); err != nil {
			return fmt.Errorf("migration %s service account: %w", migration, err)
		}
		role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace}}
		binding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace}}
		if len(identity.Permissions) == 0 {
			for _, obj := range []client.Object{binding, role} {
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); apierrors.IsNotFound(err) {
					continue
				} else if err != nil {
					return err
				}
				if !hasOwnerReference(obj, wandb) || obj.GetLabels()["app.kubernetes.io/component"] != "migration-identity" {
					return fmt.Errorf("refusing to delete unowned migration RBAC %s", name)
				}
				if err := c.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			}
			continue
		}
		if err := reconcileMigrationIdentityObject(ctx, c, wandb, role, func() { role.Rules = migrationPermissions(identity) }); err != nil {
			return err
		}
		if err := reconcileMigrationIdentityObject(ctx, c, wandb, binding, func() {
			binding.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}
			binding.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: name, Namespace: wandb.Namespace}}
		}); err != nil {
			return err
		}
	}
	return cleanupMigrationServiceAccounts(ctx, c, wandb, reconciledNames)
}

func cleanupMigrationServiceAccounts(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, reconciledNames []string) error {
	for _, list := range []client.ObjectList{&corev1.ServiceAccountList{}, &rbacv1.RoleList{}, &rbacv1.RoleBindingList{}} {
		if err := c.List(ctx, list, client.InNamespace(wandb.Namespace), client.MatchingLabels{"app.kubernetes.io/component": "migration-identity"}); err != nil {
			return err
		}
		objects, err := meta.ExtractList(list)
		if err != nil {
			return err
		}
		for _, item := range objects {
			obj := item.(client.Object)
			if !hasOwnerReference(obj, wandb) || slices.Contains(reconciledNames, obj.GetName()) {
				continue
			}
			if err := c.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func reconcileMigrationIdentityObject(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, obj client.Object, mutate func()) error {
	_, err := ctrl.CreateOrUpdate(ctx, c, obj, func() error {
		if obj.GetResourceVersion() != "" && (!hasOwnerReference(obj, wandb) || obj.GetLabels()["app.kubernetes.io/component"] != "migration-identity") {
			return fmt.Errorf("refusing to adopt unowned migration identity resource %s", obj.GetName())
		}
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["app.kubernetes.io/managed-by"] = "wandb-operator"
		labels["app.kubernetes.io/instance"] = wandb.Name
		labels["app.kubernetes.io/component"] = "migration-identity"
		obj.SetLabels(labels)
		mutate()
		return controllerutil.SetControllerReference(wandb, obj, c.Scheme())
	})
	return err
}

func applyMigrationServiceAccount(wandb *apiv2.WeightsAndBiases, migration string, identity *servermanifest.MigrationServiceAccount, pod *corev1.PodSpec) {
	if identity == nil {
		return
	}
	pod.ServiceAccountName = migrationServiceAccountName(wandb, migration, identity)
	pod.AutomountServiceAccountToken = new(ptr.Deref(identity.AutomountServiceAccountToken, false))
}
