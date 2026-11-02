package controller

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	appsv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer/deployerfakes"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	"github.com/wandb/operator/pkg/wandb/spec/state/secrets"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestReconcileUserSpecPolicy(t *testing.T) {
	for _, mode := range []string{"deployer", "managed", "managed fallback"} {
		for _, enabled := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/enabled=%t", mode, enabled), func(t *testing.T) {
				user := testUserSpecSecret()
				r, wandb, deployer := newUserSpecReconciler(t, mode, user)
				r.UserSpecEnabled = enabled
				request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(wandb)}
				ctx := context.Background()
				manager := state.New(ctx, r.Client, wandb, r.Scheme, secrets.New(ctx, r.Client, wandb, r.Scheme))
				assertSpec := func(wantUser bool, call int) {
					t.Helper()
					active, err := manager.GetActive()
					if err != nil {
						t.Fatal(err)
					}
					wantImage, wantChart, wantSource := "deployer", "0.43.5", "deployer"
					wantRelease, wantLicense := "", "cr-license"
					if mode == "managed" {
						wantImage, wantSource = "managed", ""
					}
					if wantUser {
						wantImage, wantChart, wantSource = "user", "9.0.0", "user"
						wantRelease, wantLicense = "user-release", "user-license"
					}
					if got := active.Values.GetString("app.image.tag"); got != wantImage {
						t.Fatalf("image = %q, want %q", got, wantImage)
					}
					if got := active.Chart.(*charts.RepoRelease).Version; got != wantChart {
						t.Fatalf("chart version = %q, want %q", got, wantChart)
					}
					if (wantSource == "" && active.Metadata != nil) || (wantSource != "" && (active.Metadata == nil || (*active.Metadata)["source"] != wantSource)) {
						t.Fatalf("metadata = %v, want source %q", active.Metadata, wantSource)
					}
					if got := active.Values.GetString("global.host"); got != "cr.example.invalid" {
						t.Fatalf("CR host = %q", got)
					}
					if got := active.Values.GetString("_releaseId"); got != wantRelease {
						t.Fatalf("release ID = %q, want %q", got, wantRelease)
					}
					options := deployer.GetSpecArgsForCall(call)
					if options.ReleaseId != wantRelease || options.License != wantLicense {
						t.Fatalf("Deployer release/license = %q/%q, want %q/%q", options.ReleaseId, options.License, wantRelease, wantLicense)
					}
				}
				if _, err := r.Reconcile(ctx, request); err != nil {
					t.Fatal(err)
				}
				assertSpec(enabled, 0)
				preserved := &corev1.Secret{}
				if err := r.Get(ctx, client.ObjectKeyFromObject(user), preserved); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(user.Data, preserved.Data) {
					t.Fatal("reconciliation changed the user Secret")
				}
				if !enabled {
					r.UserSpecEnabled = true
					if _, err := r.Reconcile(ctx, request); err != nil {
						t.Fatal(err)
					}
					assertSpec(true, 1)
				}
			})
		}
	}
}

func TestReconcileUserSpecSecretHandling(t *testing.T) {
	for _, tc := range []struct {
		name      string
		enabled   bool
		missing   bool
		malformed bool
		forbidden bool
		wantError bool
	}{
		{name: "disabled missing", missing: true},
		{name: "disabled malformed", malformed: true},
		{name: "disabled forbidden", forbidden: true},
		{name: "enabled missing", enabled: true, missing: true},
		{name: "enabled malformed", enabled: true, malformed: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := testUserSpecSecret()
			if tc.malformed {
				user.Data["values"] = []byte("{")
			}
			objects := []client.Object{}
			if !tc.missing {
				objects = append(objects, user)
			}
			r, wandb, _ := newUserSpecReconciler(t, "deployer", objects...)
			r.UserSpecEnabled = tc.enabled
			underlying := r.Client
			reads := 0
			r.Client = interceptor.NewClient(underlying.(client.WithWatch), interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, secret := obj.(*corev1.Secret); secret && key.Name == user.Name {
						reads++
						if tc.forbidden {
							return apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, key.Name, fmt.Errorf("user Secret access denied"))
						}
					}
					return c.Get(ctx, key, obj, opts...)
				},
			})
			ctx := context.Background()
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(wandb)})
			if (err != nil) != tc.wantError {
				t.Fatalf("Reconcile error = %v, want error %t", err, tc.wantError)
			}
			if !tc.enabled && reads != 0 {
				t.Fatalf("disabled reconciliation read the user Secret %d times", reads)
			}
			after := &corev1.Secret{}
			err = underlying.Get(ctx, client.ObjectKeyFromObject(user), after)
			if tc.missing && !tc.enabled {
				if !apierrors.IsNotFound(err) {
					t.Fatalf("disabled reconciliation created a user Secret: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !tc.missing && !reflect.DeepEqual(user.Data, after.Data) {
				t.Fatal("reconciliation changed the user Secret")
			}
		})
	}
}

func newUserSpecReconciler(t *testing.T, mode string, objects ...client.Object) (*WeightsAndBiasesReconciler, *appsv1.WeightsAndBiases, *deployerfakes.FakeDeployerInterface) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	wandb := &appsv1.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "test-user-spec", Namespace: "default", UID: types.UID("test-user-spec")},
		Spec: appsv1.WeightsAndBiasesSpec{Values: appsv1.Object{Object: map[string]interface{}{
			"global": map[string]interface{}{
				"host":          "cr.example.invalid",
				"licenseSecret": map[string]interface{}{"name": "cr-license", "key": "license"},
			},
		}}},
	}
	objects = append(objects, wandb, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cr-license", Namespace: "default"},
		Data:       map[string][]byte{"license": []byte("cr-license")},
	})
	if mode == "managed" {
		cm := testManagedSpecConfigMap("default", map[string]interface{}{
			"app": map[string]interface{}{"image": map[string]interface{}{"tag": "managed"}},
		})
		objects = append(objects, cm)
	}
	deployer := &deployerfakes.FakeDeployerInterface{}
	base := testManagedSpec(map[string]interface{}{
		"app": map[string]interface{}{"image": map[string]interface{}{"tag": "deployer"}},
	})
	if mode != "managed" {
		base.Metadata = &spec.Metadata{"source": "deployer"}
	}
	deployer.GetSpecReturns(base, nil)
	return &WeightsAndBiasesReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(wandb).WithObjects(objects...).Build(),
		Scheme:             scheme,
		Recorder:           record.NewFakeRecorder(30),
		DeployerClient:     deployer,
		DryRun:             true,
		ManagedSpecEnabled: mode != "deployer",
		UserSpecEnabled:    true,
	}, wandb, deployer
}

func testUserSpecSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-user-spec-spec-user", Namespace: "default"},
		Data: map[string][]byte{
			"values":   []byte(`{"_releaseId":"user-release","global":{"host":"user.example.invalid","license":"user-license"},"app":{"image":{"tag":"user"}}}`),
			"chart":    []byte(`{"name":"operator-wandb","url":"https://charts.wandb.ai","version":"9.0.0"}`),
			"metadata": []byte(`{"source":"user"}`),
		},
	}
}
