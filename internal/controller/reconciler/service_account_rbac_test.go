package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func serviceAccountRBACClient(t *testing.T) ctrlClient.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		rbacv1.AddToScheme,
		apiv2.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func serviceAccountRBACCR(name string, annotations map[string]string) *apiv2.WeightsAndBiases {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
	}
	wandb.Spec.Wandb.ServiceAccount = apiv2.ServiceAccountSpec{
		Create:             ptr.To(true),
		ServiceAccountName: name,
		Annotations:        annotations,
	}
	return wandb
}

// TestServiceAccountRBACFollowsCarriedName pins the guarantee the v1 → v2
// ServiceAccount mapping depends on: an account carried over from v1 gets the
// same Role and RoleBinding the operator's own default account would have, so
// carrying a name never costs the workload its permissions.
func TestServiceAccountRBACFollowsCarriedName(t *testing.T) {
	const carried = "my-irsa-sa"

	ctx := context.Background()
	client := serviceAccountRBACClient(t)
	wandb := serviceAccountRBACCR(carried, map[string]string{
		"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/wandb",
	})

	if err := createOrUpdateServiceAccount(ctx, client, wandb, carried); err != nil {
		t.Fatal(err)
	}
	if err := createOrUpdateRole(ctx, client, wandb, carried); err != nil {
		t.Fatal(err)
	}
	if err := createOrUpdateRoleBinding(ctx, client, wandb, carried); err != nil {
		t.Fatal(err)
	}

	key := types.NamespacedName{Name: carried, Namespace: wandb.Namespace}

	serviceAccount := &corev1.ServiceAccount{}
	if err := client.Get(ctx, key, serviceAccount); err != nil {
		t.Fatalf("expected a ServiceAccount named %q: %v", carried, err)
	}
	if got := serviceAccount.Annotations["eks.amazonaws.com/role-arn"]; got == "" {
		t.Fatal("expected the carried cloud IAM annotation on the ServiceAccount")
	}

	role := &rbacv1.Role{}
	if err := client.Get(ctx, key, role); err != nil {
		t.Fatalf("expected a Role named %q: %v", carried, err)
	}
	if len(role.Rules) == 0 {
		t.Fatal("expected the Role to carry the default account's rules")
	}

	roleBinding := &rbacv1.RoleBinding{}
	if err := client.Get(ctx, key, roleBinding); err != nil {
		t.Fatalf("expected a RoleBinding named %q: %v", carried, err)
	}
	if roleBinding.RoleRef.Name != carried {
		t.Fatalf("expected the RoleBinding to reference Role %q, got %q", carried, roleBinding.RoleRef.Name)
	}
	if len(roleBinding.Subjects) != 1 ||
		roleBinding.Subjects[0].Name != carried ||
		roleBinding.Subjects[0].Namespace != wandb.Namespace {
		t.Fatalf("expected the RoleBinding to bind ServiceAccount %s/%s, got %+v",
			wandb.Namespace, carried, roleBinding.Subjects)
	}
}

// TestServiceAccountRBACMatchesDefaultAccount: the rules must not depend on the
// name, or a carried account would silently get a different permission set than
// the operator's own wandb-app.
func TestServiceAccountRBACMatchesDefaultAccount(t *testing.T) {
	ctx := context.Background()

	rulesFor := func(name string) []rbacv1.PolicyRule {
		client := serviceAccountRBACClient(t)
		wandb := serviceAccountRBACCR(name, nil)
		if err := createOrUpdateRole(ctx, client, wandb, name); err != nil {
			t.Fatal(err)
		}
		role := &rbacv1.Role{}
		if err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: wandb.Namespace}, role); err != nil {
			t.Fatal(err)
		}
		return role.Rules
	}

	defaultRules := rulesFor("wandb-app")
	carriedRules := rulesFor("my-irsa-sa")

	if len(defaultRules) != len(carriedRules) {
		t.Fatalf("rule count differs: default %d, carried %d", len(defaultRules), len(carriedRules))
	}
	for i := range defaultRules {
		if defaultRules[i].String() != carriedRules[i].String() {
			t.Fatalf("rule %d differs:\n default: %s\n carried: %s",
				i, defaultRules[i].String(), carriedRules[i].String())
		}
	}
}
