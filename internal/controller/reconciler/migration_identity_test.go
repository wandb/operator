package reconciler

import (
	"context"
	"reflect"
	"strings"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func migrationIdentityFixture(t *testing.T) (client.Client, *apiv2.WeightsAndBiases, servermanifest.Manifest) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{apiv2.AddToScheme, corev1.AddToScheme, batchv1.AddToScheme, rbacv1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	w := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "test", UID: types.UID("install-1")}, Spec: apiv2.WeightsAndBiasesSpec{Wandb: apiv2.WandbAppSpec{Version: "test", ServiceAccount: apiv2.ServiceAccountSpec{ServiceAccountName: "shared"}}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(w).WithObjects(w).Build()
	automount := true
	task := servermanifest.MigrationJob{
		Image: servermanifest.ImageRef{Repository: "example/signer", Tag: "test"}, Args: []string{"secret-generation-job"},
		ServiceAccount: &servermanifest.MigrationServiceAccount{AutomountServiceAccountToken: &automount, Permissions: []rbacv1.PolicyRule{{Resources: []string{"secrets"}, ResourceNames: []string{"wandb-internal-signer"}, Verbs: []string{"get", "update"}}}},
	}
	m := servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: "wandb-internal-signer", UseExactName: true}}, Migrations: map[string]servermanifest.MigrationJob{"internal-signer": task}}
	return c, w, m
}

func TestSignerMigrationIdentityBootstrap(t *testing.T) {
	c, w, m := migrationIdentityFixture(t)
	ctx := context.Background()
	if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	if _, err := runMigrations(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	job := &batchv1.Job{}
	if err := c.Get(ctx, client.ObjectKey{Name: "wandb-internal-signer", Namespace: w.Namespace}, job); err != nil {
		t.Fatal(err)
	}
	pod := job.Spec.Template.Spec
	if pod.ServiceAccountName != "wandb-migration-internal-signer" || pod.AutomountServiceAccountToken == nil || !*pod.AutomountServiceAccountToken {
		t.Fatalf("wrong job identity: %+v", pod)
	}
	sa := &corev1.ServiceAccount{}
	key := client.ObjectKey{Name: pod.ServiceAccountName, Namespace: w.Namespace}
	if err := c.Get(ctx, key, sa); err != nil {
		t.Fatal(err)
	}
	if sa.AutomountServiceAccountToken == nil || !*sa.AutomountServiceAccountToken {
		t.Fatal("service account automount disabled")
	}
	role := &rbacv1.Role{}
	if err := c.Get(ctx, key, role); err != nil {
		t.Fatal(err)
	}
	want := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, ResourceNames: []string{"wandb-internal-signer"}, Verbs: []string{"get", "update"}}}
	if !reflect.DeepEqual(role.Rules, want) {
		t.Fatalf("overbroad signer role: %+v", role.Rules)
	}
	binding := &rbacv1.RoleBinding{}
	if err := c.Get(ctx, key, binding); err != nil {
		t.Fatal(err)
	}
	if binding.RoleRef.Name != key.Name || len(binding.Subjects) != 1 || binding.Subjects[0].Name != sa.Name || binding.Subjects[0].Namespace != w.Namespace {
		t.Fatalf("wrong binding: %+v", binding)
	}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: "wandb-internal-signer", Namespace: w.Namespace}, secret); err != nil {
		t.Fatal(err)
	}
	if len(secret.Data) != 0 || !hasOwnerReference(secret, w) {
		t.Fatal("bootstrap Secret must be empty and owned by installation")
	}
	if len(pod.Volumes) != 0 || len(pod.Containers[0].VolumeMounts) != 0 {
		t.Fatal("signer must rely on automatic credentials rather than explicit token volumes")
	}
}

func TestSignerIdentityPreservesKeysAndRepairsRBAC(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy secret", true: "owned secret"}[owned], func(t *testing.T) {
			c, w, m := migrationIdentityFixture(t)
			ctx := context.Background()
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wandb-internal-signer", Namespace: w.Namespace}, Data: map[string][]byte{"key.pem": []byte("existing signing material")}}
			if owned {
				secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: w.Name, UID: w.UID}}
			}
			if err := c.Create(ctx, secret); err != nil {
				t.Fatal(err)
			}
			original := secret.DeepCopy()
			if !owned {
				original.OwnerReferences = []metav1.OwnerReference{{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: w.Name, UID: w.UID}}
			}
			if original.Labels == nil {
				original.Labels = map[string]string{}
			}
			original.Labels[manifestSecretLabel] = "true"
			if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
				t.Fatal(err)
			}
			if err := reconcileMigrationServiceAccounts(ctx, c, w, m); err != nil {
				t.Fatal(err)
			}
			role := &rbacv1.Role{}
			key := client.ObjectKey{Name: migrationServiceAccountName(w, "internal-signer", m.Migrations["internal-signer"].ServiceAccount), Namespace: w.Namespace}
			if err := c.Get(ctx, key, role); err != nil {
				t.Fatal(err)
			}
			role.Rules[0].Verbs = append(role.Rules[0].Verbs, "delete")
			if err := c.Update(ctx, role); err != nil {
				t.Fatal(err)
			}
			if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
				t.Fatal(err)
			}
			if err := reconcileMigrationServiceAccounts(ctx, c, w, m); err != nil {
				t.Fatal(err)
			}
			if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(secret.Data, original.Data) || !reflect.DeepEqual(secret.OwnerReferences, original.OwnerReferences) || !reflect.DeepEqual(secret.Labels, original.Labels) {
				t.Fatal("existing signing Secret data or adopted ownership was incorrect")
			}
			if err := c.Get(ctx, key, role); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(role.Rules[0].Verbs, []string{"get", "update"}) {
				t.Fatalf("RBAC drift not repaired: %+v", role.Rules)
			}
		})
	}
}

func TestMigrationIdentityCompatibility(t *testing.T) {
	for _, mode := range []string{"legacy", "unprivileged", "completed", "running"} {
		t.Run(mode, func(t *testing.T) {
			c, w, m := migrationIdentityFixture(t)
			ctx := context.Background()
			task := m.Migrations["internal-signer"]
			switch mode {
			case "legacy":
				task.ServiceAccount = nil
			case "unprivileged":
				task.ServiceAccount = &servermanifest.MigrationServiceAccount{}
				task.VolumeMounts = nil
			case "completed":
				w.Status.Wandb.Migration = apiv2.WandbMigrationStatus{Version: w.Spec.Wandb.Version, Ready: true}
			case "running":
				old := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "wandb-internal-signer", Namespace: w.Namespace}, Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{ServiceAccountName: "shared"}}}}
				if err := c.Create(ctx, old); err != nil {
					t.Fatal(err)
				}
			}
			m.Migrations["internal-signer"] = task
			if _, err := runMigrations(ctx, c, w, m); err != nil {
				t.Fatal(err)
			}
			jobs := &batchv1.JobList{}
			if err := c.List(ctx, jobs); err != nil {
				t.Fatal(err)
			}
			if mode == "completed" {
				if len(jobs.Items) != 0 {
					t.Fatal("completed migration reran")
				}
				return
			}
			if len(jobs.Items) != 1 {
				t.Fatalf("got %d jobs", len(jobs.Items))
			}
			pod := jobs.Items[0].Spec.Template.Spec
			if mode == "legacy" || mode == "running" {
				if pod.ServiceAccountName != "shared" || pod.AutomountServiceAccountToken != nil {
					t.Fatal("existing identity changed")
				}
			} else if pod.ServiceAccountName == "shared" || len(pod.Volumes) != 0 || pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
				t.Fatal("unprivileged identity gained API credentials")
			}
		})
	}
}

func TestMigrationIdentityRejectsResourceTakeover(t *testing.T) {
	c, w, m := migrationIdentityFixture(t)
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: migrationServiceAccountName(w, "internal-signer", m.Migrations["internal-signer"].ServiceAccount), Namespace: w.Namespace}}
	if err := c.Create(context.Background(), sa); err != nil {
		t.Fatal(err)
	}
	if _, err := runMigrations(context.Background(), c, w, m); err == nil || !strings.Contains(err.Error(), "refusing to adopt") {
		t.Fatalf("expected ownership error, got %v", err)
	}
	jobs := &batchv1.JobList{}
	if err := c.List(context.Background(), jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 0 {
		t.Fatal("job created before identity was ready")
	}
}

func TestMigrationIdentityRevokesRemovedPermissions(t *testing.T) {
	c, w, m := migrationIdentityFixture(t)
	ctx := context.Background()
	if err := reconcileMigrationServiceAccounts(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	task := m.Migrations["internal-signer"]
	task.ServiceAccount = &servermanifest.MigrationServiceAccount{Name: task.ServiceAccount.Name}
	m.Migrations["internal-signer"] = task
	if err := reconcileMigrationServiceAccounts(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	roles := &rbacv1.RoleList{}
	bindings := &rbacv1.RoleBindingList{}
	if err := c.List(ctx, roles); err != nil {
		t.Fatal(err)
	}
	if err := c.List(ctx, bindings); err != nil {
		t.Fatal(err)
	}
	if len(roles.Items) != 0 || len(bindings.Items) != 0 {
		t.Fatal("removed permissions were retained")
	}
}

func TestMigrationIdentityPermissionsDoNotImplyCredentialsOrSecrets(t *testing.T) {
	for _, automount := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic credentials disabled", true: "automatic credentials"}[automount], func(t *testing.T) {
			c, w, m := migrationIdentityFixture(t)
			task := m.Migrations["internal-signer"]
			m.Secrets = nil
			task.VolumeMounts = nil
			task.ServiceAccount = &servermanifest.MigrationServiceAccount{
				Name: "custom-worker", AutomountServiceAccountToken: &automount,
				Permissions: []rbacv1.PolicyRule{{APIGroups: []string{"example.com"}, Resources: []string{"tasks"}, Verbs: []string{"get", "patch"}}},
			}
			m.Migrations["internal-signer"] = task
			if _, err := runMigrations(context.Background(), c, w, m); err != nil {
				t.Fatal(err)
			}
			job := &batchv1.Job{}
			if err := c.Get(context.Background(), client.ObjectKey{Name: "wandb-internal-signer", Namespace: w.Namespace}, job); err != nil {
				t.Fatal(err)
			}
			pod := job.Spec.Template.Spec
			if pod.ServiceAccountName != "custom-worker" || pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken != automount || len(pod.Volumes) != 0 {
				t.Fatalf("unexpected identity or implicit credentials: %+v", pod)
			}
			role := &rbacv1.Role{}
			if err := c.Get(context.Background(), client.ObjectKey{Name: pod.ServiceAccountName, Namespace: w.Namespace}, role); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(role.Rules, task.ServiceAccount.Permissions) {
				t.Fatalf("declared rules changed: %+v", role.Rules)
			}
			secrets := &corev1.SecretList{}
			if err := c.List(context.Background(), secrets); err != nil {
				t.Fatal(err)
			}
			if len(secrets.Items) != 0 {
				t.Fatal("permissions implicitly created Secrets")
			}
		})
	}
}

func TestMigrationIdentityRejectsConflictingOrInvalidDeclarations(t *testing.T) {
	for _, problem := range []string{"shared account", "duplicate name", "invalid name", "non-resource URL", "missing resources", "missing verbs"} {
		t.Run(problem, func(t *testing.T) {
			c, w, m := migrationIdentityFixture(t)
			task := m.Migrations["internal-signer"]
			switch problem {
			case "shared account":
				w.Spec.Wandb.ServiceAccount.ServiceAccountName = migrationServiceAccountName(w, "internal-signer", task.ServiceAccount)
			case "duplicate name":
				task.ServiceAccount.Name = "duplicate"
				m.Migrations["another"] = task
			case "invalid name":
				task.ServiceAccount.Name = "invalid/name"
			case "non-resource URL":
				task.ServiceAccount.Permissions[0].NonResourceURLs = []string{"/metrics"}
			case "missing resources":
				task.ServiceAccount.Permissions[0].Resources = nil
			case "missing verbs":
				task.ServiceAccount.Permissions[0].Verbs = nil
			}
			m.Migrations["internal-signer"] = task
			if _, err := runMigrations(context.Background(), c, w, m); err == nil {
				t.Fatal("invalid declaration accepted")
			}
			accounts := &corev1.ServiceAccountList{}
			if err := c.List(context.Background(), accounts); err != nil {
				t.Fatal(err)
			}
			if len(accounts.Items) != 0 {
				t.Fatal("created resources before validating manifest")
			}
		})
	}
}
