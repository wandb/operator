package reconciler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// migrationIdentityFixture describes the signer bootstrap scenario. Secret-only
// tests use manifestSecretTestClient instead of inheriting migration configuration.
func migrationIdentityFixture(t *testing.T) (client.WithWatch, *apiv2.WeightsAndBiases, servermanifest.Manifest) {
	t.Helper()
	c, wandb := manifestSecretTestClient(t)
	require.NoError(t, batchv1.AddToScheme(c.Scheme()))
	require.NoError(t, rbacv1.AddToScheme(c.Scheme()))
	wandb.Spec.Wandb = apiv2.WandbAppSpec{
		Version:        "test",
		ServiceAccount: apiv2.ServiceAccountSpec{ServiceAccountName: "shared"},
	}
	require.NoError(t, c.Update(context.Background(), wandb))
	automount := true
	task := servermanifest.MigrationJob{
		Image: servermanifest.ImageRef{Repository: "example/signer", Tag: "test"},
		Args:  []string{"secret-generation-job"},
		ServiceAccount: &servermanifest.MigrationServiceAccount{
			AutomountServiceAccountToken: &automount,
			Permissions: []rbacv1.PolicyRule{{
				Resources:     []string{"secrets"},
				ResourceNames: []string{"wandb-internal-signer"},
				Verbs:         []string{"get", "update"},
			}},
		},
	}
	manifest := servermanifest.Manifest{
		Secrets:    []servermanifest.Secret{{Name: "wandb-internal-signer", UseExactName: true}},
		Migrations: map[string]servermanifest.MigrationJob{"internal-signer": task},
	}
	return c, wandb, manifest
}

func TestSignerMigrationIdentityBootstrap(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	ctx := context.Background()
	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)

	_, err = runMigrations(ctx, c, wandb, manifest)
	require.NoError(t, err)

	job := &batchv1.Job{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "wandb-internal-signer", Namespace: wandb.Namespace}, job))
	pod := job.Spec.Template.Spec
	require.Equal(t, "wandb-migration-internal-signer", pod.ServiceAccountName)
	require.NotNil(t, pod.AutomountServiceAccountToken)
	require.True(t, *pod.AutomountServiceAccountToken)
	sa := &corev1.ServiceAccount{}
	key := client.ObjectKey{Name: pod.ServiceAccountName, Namespace: wandb.Namespace}
	require.NoError(t, c.Get(ctx, key, sa))
	require.NotNil(t, sa.AutomountServiceAccountToken)
	require.True(t, *sa.AutomountServiceAccountToken, "signer account needs API credentials")
	role := &rbacv1.Role{}
	require.NoError(t, c.Get(ctx, key, role))
	// The empty API group denotes core Kubernetes resources such as Secrets.
	want := []rbacv1.PolicyRule{{
		APIGroups:     []string{""},
		Resources:     []string{"secrets"},
		ResourceNames: []string{"wandb-internal-signer"},
		Verbs:         []string{"get", "update"},
	}}
	require.Equal(t, want, role.Rules, "signer Role must grant only the declared permissions")
	binding := &rbacv1.RoleBinding{}
	require.NoError(t, c.Get(ctx, key, binding))
	require.Equal(t, key.Name, binding.RoleRef.Name)
	require.Len(t, binding.Subjects, 1)
	require.Equal(t, sa.Name, binding.Subjects[0].Name)
	require.Equal(t, wandb.Namespace, binding.Subjects[0].Namespace)
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "wandb-internal-signer", Namespace: wandb.Namespace}, secret))
	require.Empty(t, secret.Data)
	require.True(t, hasOwnerReference(secret, wandb), "bootstrap Secret must belong to the installation")
	// API credentials come from automounting rather than explicit token volumes.
	require.Empty(t, pod.Volumes)
	require.NotEmpty(t, pod.Containers)
	require.Empty(t, pod.Containers[0].VolumeMounts)
}

func TestSignerIdentityPreservesExistingKeys(t *testing.T) {
	tests := []struct {
		name         string
		alreadyOwned bool
	}{
		{name: "legacy Secret is adopted", alreadyOwned: false},
		{name: "owned Secret retains ownership", alreadyOwned: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, wandb, manifest := migrationIdentityFixture(t)
			ctx := context.Background()
			owner := metav1.OwnerReference{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: wandb.Name, UID: wandb.UID}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "wandb-internal-signer", Namespace: wandb.Namespace},
				Data:       map[string][]byte{"key.pem": []byte("existing signing material")},
			}
			if test.alreadyOwned {
				secret.OwnerReferences = []metav1.OwnerReference{owner}
			}
			require.NoError(t, c.Create(ctx, secret))
			originalData := secret.DeepCopy().Data

			_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
			require.NoError(t, err)
			_, err = reconcileManifestSecrets(ctx, c, wandb, manifest)
			require.NoError(t, err, "repeated reconciliation must preserve the adopted Secret")

			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), secret))
			require.Equal(t, originalData, secret.Data, "adoption must preserve signing material")
			require.Equal(t, []metav1.OwnerReference{owner}, secret.OwnerReferences)
			require.Equal(t, map[string]string{manifestSecretLabel: "true"}, secret.Labels)
		})
	}
}

func TestSignerIdentityRepairsRBACDrift(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	ctx := context.Background()
	require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest))
	role := &rbacv1.Role{}
	key := client.ObjectKey{Name: "wandb-migration-internal-signer", Namespace: wandb.Namespace}
	require.NoError(t, c.Get(ctx, key, role))
	require.Len(t, role.Rules, 1)
	// Simulate permission drift: the signer must never retain permission to delete Secrets.
	role.Rules[0].Verbs = append(role.Rules[0].Verbs, "delete")
	require.NoError(t, c.Update(ctx, role))

	require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest))
	require.NoError(t, c.Get(ctx, key, role))
	require.Len(t, role.Rules, 1)
	require.Equal(t, []string{"get", "update"}, role.Rules[0].Verbs)
}

func TestMigrationWithoutDedicatedIdentityUsesSharedAccount(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	task := manifest.Migrations["internal-signer"]
	task.ServiceAccount = nil
	manifest.Migrations["internal-signer"] = task
	ctx := context.Background()

	_, err := runMigrations(ctx, c, wandb, manifest)
	require.NoError(t, err)
	jobs := &batchv1.JobList{}
	require.NoError(t, c.List(ctx, jobs))
	require.Len(t, jobs.Items, 1)
	pod := jobs.Items[0].Spec.Template.Spec
	require.Equal(t, "shared", pod.ServiceAccountName)
	require.Nil(t, pod.AutomountServiceAccountToken, "legacy migrations must retain the existing automount default")
}

func TestMigrationWithEmptyDedicatedIdentityHasNoAPICredentials(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	task := manifest.Migrations["internal-signer"]
	// An explicit empty identity requests a dedicated account with credentials disabled.
	task.ServiceAccount = &servermanifest.MigrationServiceAccount{}
	manifest.Migrations["internal-signer"] = task
	ctx := context.Background()

	_, err := runMigrations(ctx, c, wandb, manifest)
	require.NoError(t, err)
	jobs := &batchv1.JobList{}
	require.NoError(t, c.List(ctx, jobs))
	require.Len(t, jobs.Items, 1)
	pod := jobs.Items[0].Spec.Template.Spec
	require.Equal(t, "wandb-migration-internal-signer", pod.ServiceAccountName)
	require.Empty(t, pod.Volumes)
	require.NotNil(t, pod.AutomountServiceAccountToken)
	require.False(t, *pod.AutomountServiceAccountToken)
}

func TestCompletedMigrationDoesNotRunAgain(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	wandb.Status.Wandb.Migration = apiv2.WandbMigrationStatus{Version: wandb.Spec.Wandb.Version, Ready: true}
	ctx := context.Background()

	_, err := runMigrations(ctx, c, wandb, manifest)
	require.NoError(t, err)
	jobs := &batchv1.JobList{}
	require.NoError(t, c.List(ctx, jobs))
	require.Empty(t, jobs.Items, "completed migration must not run again")
}

func TestRunningMigrationRetainsExistingIdentity(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	ctx := context.Background()
	// The running Job predates dedicated identities and must keep its shared account.
	existing := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-internal-signer", Namespace: wandb.Namespace},
		Spec:       batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{ServiceAccountName: "shared"}}},
	}
	require.NoError(t, c.Create(ctx, existing))

	_, err := runMigrations(ctx, c, wandb, manifest)
	require.NoError(t, err)
	jobs := &batchv1.JobList{}
	require.NoError(t, c.List(ctx, jobs))
	require.Len(t, jobs.Items, 1)
	pod := jobs.Items[0].Spec.Template.Spec
	require.Equal(t, "shared", pod.ServiceAccountName)
	require.Nil(t, pod.AutomountServiceAccountToken)
}

func TestMigrationIdentityRejectsResourceTakeover(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	// An account with the desired name but no installation owner must not be adopted.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-migration-internal-signer", Namespace: wandb.Namespace},
	}
	require.NoError(t, c.Create(context.Background(), sa))
	_, err := runMigrations(context.Background(), c, wandb, manifest)
	require.ErrorContains(t, err, "refusing to adopt")
	jobs := &batchv1.JobList{}
	require.NoError(t, c.List(context.Background(), jobs))
	require.Empty(t, jobs.Items, "Job must not be created before identity is ready")
}

func TestMigrationIdentityRevokesRemovedPermissions(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	ctx := context.Background()
	require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest))
	task := manifest.Migrations["internal-signer"]
	// Remove permissions while keeping the account and credential configuration.
	task.ServiceAccount.Permissions = nil
	manifest.Migrations["internal-signer"] = task
	require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest))
	roles := &rbacv1.RoleList{}
	bindings := &rbacv1.RoleBindingList{}
	require.NoError(t, c.List(ctx, roles))
	require.NoError(t, c.List(ctx, bindings))
	require.Empty(t, roles.Items, "removing permissions must remove the Role")
	require.Empty(t, bindings.Items, "removing permissions must remove the RoleBinding")
}

func TestMigrationIdentityCleansUpStaleResources(t *testing.T) {
	for _, change := range []string{"removed migration", "shared account", "renamed account"} {
		t.Run(change, func(t *testing.T) {
			c, wandb, manifest := migrationIdentityFixture(t)
			ctx := context.Background()
			require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest))
			task := manifest.Migrations["internal-signer"]
			switch change {
			case "removed migration":
				delete(manifest.Migrations, "internal-signer")
			case "shared account":
				task.ServiceAccount = nil
				manifest.Migrations["internal-signer"] = task
			case "renamed account":
				task.ServiceAccount.Name = "new-signer"
				manifest.Migrations["internal-signer"] = task
			}

			require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest))
			for _, obj := range []client.Object{&corev1.ServiceAccount{}, &rbacv1.Role{}, &rbacv1.RoleBinding{}} {
				key := client.ObjectKey{Name: "wandb-migration-internal-signer", Namespace: wandb.Namespace}
				require.True(t, apierrors.IsNotFound(c.Get(ctx, key, obj)), "stale %T must be removed", obj)
				if change == "renamed account" {
					key.Name = "new-signer"
					require.NoError(t, c.Get(ctx, key, obj), "reconciled %T must be retained", obj)
				}
			}
			require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest), "cleanup must be idempotent")
		})
	}
}

func TestMigrationIdentityCleanupPreservesOtherResources(t *testing.T) {
	c, wandb, manifest := migrationIdentityFixture(t)
	ctx := context.Background()
	owner := metav1.OwnerReference{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: wandb.Name, UID: wandb.UID}
	otherOwner := owner
	otherOwner.UID = "other-installation"
	var preserved []client.Object
	for _, test := range []struct {
		name      string
		namespace string
		component string
		owners    []metav1.OwnerReference
	}{
		{name: "unowned", namespace: wandb.Namespace, component: "migration-identity"},
		{name: "other-owner", namespace: wandb.Namespace, component: "migration-identity", owners: []metav1.OwnerReference{otherOwner}},
		{name: "unlabeled", namespace: wandb.Namespace, owners: []metav1.OwnerReference{owner}},
		{name: "other-component", namespace: wandb.Namespace, component: "server", owners: []metav1.OwnerReference{owner}},
		{name: "other-namespace", namespace: "other-namespace", component: "migration-identity", owners: []metav1.OwnerReference{owner}},
	} {
		for _, obj := range []client.Object{&corev1.ServiceAccount{}, &rbacv1.Role{}, &rbacv1.RoleBinding{}} {
			obj.SetName(test.name)
			obj.SetNamespace(test.namespace)
			obj.SetOwnerReferences(test.owners)
			if test.component != "" {
				obj.SetLabels(map[string]string{"app.kubernetes.io/component": test.component})
			}
			require.NoError(t, c.Create(ctx, obj))
			preserved = append(preserved, obj)
		}
	}

	require.NoError(t, reconcileMigrationServiceAccounts(ctx, c, wandb, manifest))
	for _, obj := range preserved {
		require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), obj), "%T %s must be retained", obj, obj.GetName())
	}
}

func TestMigrationIdentityPermissionsDoNotImplyCredentialsOrSecrets(t *testing.T) {
	for _, test := range []struct {
		name      string
		automount bool
	}{
		{name: "automatic credentials disabled", automount: false},
		{name: "automatic credentials enabled", automount: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, wandb, manifest := migrationIdentityFixture(t)
			task := manifest.Migrations["internal-signer"]
			// Permissions alone must not create Secrets or explicit credential volumes.
			manifest.Secrets = nil
			task.ServiceAccount = &servermanifest.MigrationServiceAccount{
				Name:                         "custom-worker",
				AutomountServiceAccountToken: &test.automount,
				Permissions:                  []rbacv1.PolicyRule{{APIGroups: []string{"example.com"}, Resources: []string{"tasks"}, Verbs: []string{"get", "patch"}}},
			}
			manifest.Migrations["internal-signer"] = task
			_, err := runMigrations(context.Background(), c, wandb, manifest)
			require.NoError(t, err)
			job := &batchv1.Job{}
			require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "wandb-internal-signer", Namespace: wandb.Namespace}, job))
			pod := job.Spec.Template.Spec
			require.Equal(t, "custom-worker", pod.ServiceAccountName)
			require.NotNil(t, pod.AutomountServiceAccountToken)
			require.Equal(t, test.automount, *pod.AutomountServiceAccountToken)
			require.Empty(t, pod.Volumes, "permissions must not introduce explicit credential volumes")
			role := &rbacv1.Role{}
			require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: pod.ServiceAccountName, Namespace: wandb.Namespace}, role))
			require.Equal(t, task.ServiceAccount.Permissions, role.Rules)
			secrets := &corev1.SecretList{}
			require.NoError(t, c.List(context.Background(), secrets))
			require.Empty(t, secrets.Items, "permissions must not implicitly create Secrets")
		})
	}
}

func TestMigrationIdentityRejectsConflictingOrInvalidDeclarations(t *testing.T) {
	tests := []struct {
		name              string
		accountName       string
		sharedAccountName string
		duplicateAccount  bool
		permissions       []rbacv1.PolicyRule
	}{
		{
			name:              "shared account collision",
			sharedAccountName: "wandb-migration-internal-signer",
			permissions:       []rbacv1.PolicyRule{{Resources: []string{"secrets"}, Verbs: []string{"get", "update"}}},
		},
		{
			name: "duplicate account name", accountName: "duplicate", duplicateAccount: true,
			permissions: []rbacv1.PolicyRule{{Resources: []string{"secrets"}, Verbs: []string{"get", "update"}}},
		},
		{
			name: "invalid account name", accountName: "invalid/name",
			permissions: []rbacv1.PolicyRule{{Resources: []string{"secrets"}, Verbs: []string{"get", "update"}}},
		},
		{
			name:        "non-resource URL cannot be granted by namespaced Role",
			permissions: []rbacv1.PolicyRule{{NonResourceURLs: []string{"/metrics"}, Resources: []string{"secrets"}, Verbs: []string{"get", "update"}}},
		},
		{name: "missing resources", permissions: []rbacv1.PolicyRule{{Verbs: []string{"get", "update"}}}},
		{name: "missing verbs", permissions: []rbacv1.PolicyRule{{Resources: []string{"secrets"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, wandb, manifest := migrationIdentityFixture(t)
			if test.sharedAccountName != "" {
				wandb.Spec.Wandb.ServiceAccount.ServiceAccountName = test.sharedAccountName
			}
			task := manifest.Migrations["internal-signer"]
			task.ServiceAccount.Name = test.accountName
			task.ServiceAccount.Permissions = test.permissions
			manifest.Migrations["internal-signer"] = task
			if test.duplicateAccount {
				manifest.Migrations["another"] = task
			}

			_, err := runMigrations(context.Background(), c, wandb, manifest)
			require.Error(t, err, "invalid declaration must be rejected")
			accounts := &corev1.ServiceAccountList{}
			require.NoError(t, c.List(context.Background(), accounts))
			require.Empty(t, accounts.Items, "all declarations must be validated before creating resources")
		})
	}
}
