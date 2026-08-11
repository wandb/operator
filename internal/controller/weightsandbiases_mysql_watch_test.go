package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMySQLSecretDependencies(t *testing.T) {
	selector := func(name, key string) corev1.SecretKeySelector {
		return corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
		}
	}
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "app"},
		Spec: apiv2.WeightsAndBiasesSpec{MySQL: map[string]apiv2.MySQLSpec{
			apiv2.DefaultInstanceName: {ExternalMysql: &apiv2.MysqlConnection{
				Host:     selector("mysql-address", "host"),
				Port:     selector("mysql-address", "port"),
				Database: selector("mysql-credentials", "database"),
				Username: selector("mysql-credentials", "username"),
				Password: selector("mysql-credentials", "password"),
				SslCa:    selector("mysql-tls", "ca"),
			}},
			"analytics": {ManagedMysql: &apiv2.ManagedMysqlSpec{
				Name:      "analytics",
				Namespace: "database",
			}},
		}},
	}

	assert.Equal(t, []string{
		"app/mysql-address",
		"app/mysql-credentials",
		"app/mysql-tls",
		"database/moco-analytics",
	}, mysqlSecretDependencies(wandb))
}

func TestMySQLSecretDependenciesDerivesManagedDefaults(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "app"},
		Spec: apiv2.WeightsAndBiasesSpec{MySQL: map[string]apiv2.MySQLSpec{
			apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}},
		}},
	}

	assert.Equal(t, []string{"app/moco-wandb-mysql"}, mysqlSecretDependencies(wandb))
}

func TestMapMySQLSecretToEveryDependentWandb(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	selector := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "shared-mysql"}, Key: "value",
	}
	dependent := func(name string) *apiv2.WeightsAndBiases {
		return &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "app"},
			Spec: apiv2.WeightsAndBiasesSpec{MySQL: map[string]apiv2.MySQLSpec{
				apiv2.DefaultInstanceName: {ExternalMysql: &apiv2.MysqlConnection{
					Host: selector, Port: selector, Database: selector, Username: selector, Password: selector,
				}},
			}},
		}
	}
	first := dependent("first")
	second := dependent("second")
	unrelated := dependent("unrelated")
	unrelated.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql.Host.Name = "another-secret"
	unrelated.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql.Port.Name = "another-secret"
	unrelated.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql.Database.Name = "another-secret"
	unrelated.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql.Username.Name = "another-secret"
	unrelated.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql.Password.Name = "another-secret"

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(first, second, unrelated).
		WithIndex(&apiv2.WeightsAndBiases{}, mysqlSecretDependencyIndex, mysqlSecretDependencies).
		Build()
	reconciler := &WeightsAndBiasesReconciler{Client: client}
	requests := reconciler.mapMySQLSecretToWandb(t.Context(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-mysql", Namespace: "app"},
	})

	requestNames := make([]string, 0, len(requests))
	for _, request := range requests {
		requestNames = append(requestNames, request.Name)
	}
	assert.ElementsMatch(t, []string{"first", "second"}, requestNames)
}

func TestMapMocoToWandbUsesProvenanceLabels(t *testing.T) {
	cluster := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
		common.WandbNameLabel:      "wandb",
		common.WandbNamespaceLabel: "app",
	}}}

	requests := mapMocoToWandb(context.Background(), cluster)
	if assert.Len(t, requests, 1) {
		assert.Equal(t, types.NamespacedName{Name: "wandb", Namespace: "app"}, requests[0].NamespacedName)
	}
}
