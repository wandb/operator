/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv2 "github.com/wandb/operator/api/v2"
)

// withConversionReader installs a fake reader for the duration of the test
// (cleaned up via t.Cleanup) so tests don't leak the package-level state.
func withConversionReader(t *testing.T, secrets ...*corev1.Secret) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, s := range secrets {
		builder = builder.WithObjects(s)
	}
	SetConversionReader(builder.Build())
	t.Cleanup(func() { SetConversionReader(nil) })
}

// activeSpecSecret builds a `<cr-name>-spec-active`-shaped Secret with the
// given values map JSON-encoded into data.values.
func activeSpecSecret(t *testing.T, namespace, crName string, values map[string]interface{}) *corev1.Secret {
	t.Helper()
	raw, err := json.Marshal(values)
	require.NoError(t, err)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName + "-spec-active",
			Namespace: namespace,
		},
		Data: map[string][]byte{"values": raw},
	}
}

func newV1(values map[string]interface{}) *WeightsAndBiases {
	return &WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
		},
		Spec: WeightsAndBiasesSpec{
			Values: Object{Object: values},
		},
	}
}

func TestConvertTo_EmptyValues(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, newV1(nil).ConvertTo(dst))
	require.Equal(t, "wandb", dst.Name)
	require.Empty(t, dst.Spec.Wandb.Hostname)
	require.Empty(t, dst.Spec.Wandb.License)
	require.Empty(t, string(dst.Spec.Size))
	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation)
}

func TestConvertTo_NoGlobalKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"mysql": map[string]interface{}{"install": true},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, dst.Spec.Wandb.Hostname)
	require.Empty(t, string(dst.Spec.Size))
}

func TestConvertTo_HostnameAndLicense(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"host":    "http://wandb.localhost",
			"license": "jwt-token-here",
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "http://wandb.localhost", dst.Spec.Wandb.Hostname)
	require.Equal(t, "jwt-token-here", dst.Spec.Wandb.License)
	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation)
}

func TestConvertTo_AllValidSizes(t *testing.T) {
	cases := []appsv2.Size{
		appsv2.SizeDev,
		appsv2.SizeMicro,
		appsv2.SizeSmall,
		appsv2.SizeMedium,
		appsv2.SizeLarge,
		appsv2.SizeXLarge,
		appsv2.SizeXXLarge,
	}
	for _, size := range cases {
		t.Run(string(size), func(t *testing.T) {
			dst := &appsv2.WeightsAndBiases{}
			src := newV1(map[string]interface{}{
				"global": map[string]interface{}{"size": string(size)},
			})
			require.NoError(t, src.ConvertTo(dst))
			require.Equal(t, size, dst.Spec.Size)
		})
	}
}

func TestConvertTo_SizeEmptyString(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"size": ""},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, string(dst.Spec.Size))
}

func TestConvertTo_SizeUnrecognized(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"size": "testing"},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"testing"`)
}

func TestConvertTo_CustomCACerts(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"customCACerts":    []interface{}{"---cert-one---", "---cert-two---"},
			"caCertsConfigMap": "corp-ca-certs",
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, []string{"---cert-one---", "---cert-two---"}, dst.Spec.Global.CustomCACerts)
	require.Equal(t, "corp-ca-certs", dst.Spec.Global.CACertsConfigMap)
}

func TestConvertTo_VersionFromAppImageTag(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.80.1"},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "0.80.1", dst.Spec.Wandb.Version)
}

func TestConvertTo_VersionFallsBackToApiImageTag(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"api": map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.79.2"},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "0.79.2", dst.Spec.Wandb.Version)
}

func TestConvertTo_VersionAppWinsOverApi(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.80.1"},
		},
		"api": map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.79.2"},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "0.80.1", dst.Spec.Wandb.Version, "app.image.tag takes precedence over api.image.tag")
}

func TestConvertTo_VersionEmptyAppFallsBackToApi(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"image": map[string]interface{}{"tag": ""},
		},
		"api": map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.79.2"},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "0.79.2", dst.Spec.Wandb.Version, "empty app tag should not consume the slot; api fallback applies")
}

func TestConvertTo_VersionAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, dst.Spec.Wandb.Version)
}

func TestConvertTo_VersionWithoutGlobal(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.80.1"},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "0.80.1", dst.Spec.Wandb.Version,
		"version mapping should run even when global is absent")
}

func TestConvertTo_ServiceAccountAnnotationsFromApp(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"serviceAccount": map[string]interface{}{
				"annotations": map[string]interface{}{
					"iam.gke.io/gcp-service-account": "wandb-app@project.iam.gserviceaccount.com",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, map[string]string{
		"iam.gke.io/gcp-service-account": "wandb-app@project.iam.gserviceaccount.com",
	}, dst.Spec.Wandb.ServiceAccount.Annotations)
}

func TestConvertTo_ServiceAccountAnnotationsFallsBackToApi(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"api": map[string]interface{}{
			"serviceAccount": map[string]interface{}{
				"annotations": map[string]interface{}{
					"iam.gke.io/gcp-service-account": "wandb-api@project.iam.gserviceaccount.com",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "wandb-api@project.iam.gserviceaccount.com",
		dst.Spec.Wandb.ServiceAccount.Annotations["iam.gke.io/gcp-service-account"])
}

func TestConvertTo_ServiceAccountAnnotationsAppWinsOverApi(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"serviceAccount": map[string]interface{}{
				"annotations": map[string]interface{}{
					"iam.gke.io/gcp-service-account": "from-app",
				},
			},
		},
		"api": map[string]interface{}{
			"serviceAccount": map[string]interface{}{
				"annotations": map[string]interface{}{
					"iam.gke.io/gcp-service-account": "from-api",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "from-app",
		dst.Spec.Wandb.ServiceAccount.Annotations["iam.gke.io/gcp-service-account"],
		"app annotations should take precedence over api when both present")
}

func TestConvertTo_ServiceAccountAnnotationsEmptyAppFallsBackToApi(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"serviceAccount": map[string]interface{}{
				"annotations": map[string]interface{}{},
			},
		},
		"api": map[string]interface{}{
			"serviceAccount": map[string]interface{}{
				"annotations": map[string]interface{}{
					"iam.gke.io/gcp-service-account": "from-api",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "from-api",
		dst.Spec.Wandb.ServiceAccount.Annotations["iam.gke.io/gcp-service-account"],
		"empty app annotations should not consume the slot; api fallback applies")
}

func TestConvertTo_ServiceAccountAnnotationsAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, dst.Spec.Wandb.ServiceAccount.Annotations)
}

func TestConvertTo_ServiceAccountAnnotationsPreservedMultipleKeys(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"serviceAccount": map[string]interface{}{
				"annotations": map[string]interface{}{
					"iam.gke.io/gcp-service-account":    "wandb-app@p.iam.gserviceaccount.com",
					"eks.amazonaws.com/role-arn":        "arn:aws:iam::123:role/wandb",
					"azure.workload.identity/client-id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Len(t, dst.Spec.Wandb.ServiceAccount.Annotations, 3)
	require.Equal(t, "wandb-app@p.iam.gserviceaccount.com",
		dst.Spec.Wandb.ServiceAccount.Annotations["iam.gke.io/gcp-service-account"])
	require.Equal(t, "arn:aws:iam::123:role/wandb",
		dst.Spec.Wandb.ServiceAccount.Annotations["eks.amazonaws.com/role-arn"])
	require.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		dst.Spec.Wandb.ServiceAccount.Annotations["azure.workload.identity/client-id"])
}

func TestConvertTo_InternalJWTIssuerFromGlobal(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"internalJWTMap": []interface{}{
				map[string]interface{}{
					"issuer":  "https://gke-issuer.example.com",
					"subject": "system:serviceaccount:default:wandb-weave-trace",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "https://gke-issuer.example.com", dst.Spec.Wandb.InternalServiceAuth.OIDCIssuer)
}

func TestConvertTo_InternalJWTIssuerFallsBackToApp(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"app": map[string]interface{}{
			"internalJWTMap": []interface{}{
				map[string]interface{}{
					"issuer":  "https://app-issuer.example.com",
					"subject": "system:serviceaccount:default:wandb-app",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "https://app-issuer.example.com", dst.Spec.Wandb.InternalServiceAuth.OIDCIssuer)
}

func TestConvertTo_InternalJWTIssuerGlobalWinsOverApp(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"internalJWTMap": []interface{}{
				map[string]interface{}{"issuer": "from-global"},
			},
		},
		"app": map[string]interface{}{
			"internalJWTMap": []interface{}{
				map[string]interface{}{"issuer": "from-app"},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "from-global", dst.Spec.Wandb.InternalServiceAuth.OIDCIssuer)
}

func TestConvertTo_InternalJWTIssuerFirstEntryWins(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"internalJWTMap": []interface{}{
				map[string]interface{}{"issuer": "first-issuer"},
				map[string]interface{}{"issuer": "second-issuer"},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "first-issuer", dst.Spec.Wandb.InternalServiceAuth.OIDCIssuer)
}

func TestConvertTo_InternalJWTIssuerEmptyGlobalFallsBackToApp(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"internalJWTMap": []interface{}{},
		},
		"app": map[string]interface{}{
			"internalJWTMap": []interface{}{
				map[string]interface{}{"issuer": "from-app"},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "from-app", dst.Spec.Wandb.InternalServiceAuth.OIDCIssuer)
}

func TestConvertTo_InternalJWTIssuerAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, dst.Spec.Wandb.InternalServiceAuth.OIDCIssuer)
}

func TestConvertTo_IngressEnabledByChartDefaults(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, appsv2.NetworkingModeIngress, dst.Spec.Networking.Mode,
		"ingress.install and ingress.create both default to true; mode should be ingress")
}

func TestConvertTo_IngressDisabledByInstallFalse(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{"install": false},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, string(dst.Spec.Networking.Mode))
}

func TestConvertTo_IngressDisabledByCreateFalse(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{"create": false},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, string(dst.Spec.Networking.Mode))
}

func TestConvertTo_IngressClassMaps(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{"class": "nginx"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotNil(t, dst.Spec.Networking.Ingress)
	require.NotNil(t, dst.Spec.Networking.Ingress.IngressClassName)
	require.Equal(t, "nginx", *dst.Spec.Networking.Ingress.IngressClassName)
}

func TestConvertTo_IngressAnnotationsMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{
			"annotations": map[string]interface{}{
				"kubernetes.io/ingress.class":               "gce",
				"ingress.gcp.kubernetes.io/pre-shared-cert": "wandb-cert",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "gce", dst.Spec.Networking.Annotations["kubernetes.io/ingress.class"])
	require.Equal(t, "wandb-cert",
		dst.Spec.Networking.Annotations["ingress.gcp.kubernetes.io/pre-shared-cert"])
}

func TestConvertTo_IngressAdditionalHostsMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{
			"additionalHosts": []interface{}{"alt1.example.com", "alt2.example.com"},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, []string{"alt1.example.com", "alt2.example.com"}, dst.Spec.Wandb.AdditionalHostnames)
}

func TestConvertTo_IngressTLSFirstEntryWins(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{
			"tls": []interface{}{
				map[string]interface{}{
					"secretName": "wandb-tls",
					"hosts":      []interface{}{"wandb.example.com"},
				},
				map[string]interface{}{
					"secretName": "wandb-tls-secondary",
					"hosts":      []interface{}{"alt.example.com"},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotNil(t, dst.Spec.Networking.TLS)
	require.Equal(t, "wandb-tls", dst.Spec.Networking.TLS.SecretName,
		"v2 carries only the first TLS entry's secretName")
}

func TestConvertTo_IngressNameOverrideMaps(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{"nameOverride": "dpanzella-test-gcp"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotNil(t, dst.Spec.Networking.Ingress)
	require.Equal(t, "dpanzella-test-gcp", dst.Spec.Networking.Ingress.Name)
}

func TestConvertTo_IngressNameOverrideEmpty(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{"nameOverride": ""},
	})
	require.NoError(t, src.ConvertTo(dst))
	if dst.Spec.Networking.Ingress != nil {
		require.Empty(t, dst.Spec.Networking.Ingress.Name)
	}
}

func TestConvertTo_IngressNoModeOverrideWhenSet(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{
		Spec: appsv2.WeightsAndBiasesSpec{
			Networking: appsv2.NetworkingSpec{
				Mode: appsv2.NetworkingModeGatewayAPI,
			},
		},
	}
	src := newV1(map[string]interface{}{
		"ingress": map[string]interface{}{"class": "nginx"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, appsv2.NetworkingModeGatewayAPI, dst.Spec.Networking.Mode,
		"already-set mode should not be overridden by ingress chart defaults")
}

func TestConvertTo_OIDCAllLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId":   "abc",
					"secret":     "shh",
					"authMethod": "client_secret_post",
					"issuer":     "https://example.com",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw, ok := dst.Annotations[OIDCPendingAnnotation]
	require.True(t, ok, "expected oidc-pending annotation to be set")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "abc", decoded["clientId"])
	require.Equal(t, "shh", decoded["secret"])
	require.Equal(t, "client_secret_post", decoded["authMethod"])
	require.Equal(t, "https://example.com", decoded["issuer"])
	require.NotContains(t, decoded, "oidcSecret")

	require.Empty(t, dst.Spec.Wandb.OIDC.ClientId.Name, "no ref-shaped values, so spec.wandb.oidc stays unset")
	require.Empty(t, dst.Spec.Wandb.OIDC.ClientSecret.Name)
}

func TestConvertTo_OIDCLegacyOidcSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId": "abc",
					"secret":   "shh",
					"oidcSecret": map[string]interface{}{
						"name":      "user-oidc-secret",
						"secretKey": "MY_KEY",
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "user-oidc-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name)
	require.Equal(t, "MY_KEY", dst.Spec.Wandb.OIDC.ClientSecret.Key)

	raw := dst.Annotations[OIDCPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "abc", decoded["clientId"])
	require.NotContains(t, decoded, "secret", "literal secret must not be stashed when oidcSecret ref took over")
	require.NotContains(t, decoded, "oidcSecret")
}

func TestConvertTo_OIDCLegacyOidcSecretDefaultKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"oidcSecret": map[string]interface{}{
						"name": "user-oidc-secret",
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "user-oidc-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name)
	require.Equal(t, "OIDC_SECRET", dst.Spec.Wandb.OIDC.ClientSecret.Key)
}

func TestConvertTo_OIDCValueFromRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "oidc-settings",
								"key":  "clientId",
							},
						},
					},
					"secret": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "oidc-settings",
								"key":  "clientSecret",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	oidc := dst.Spec.Wandb.OIDC
	require.Equal(t, "oidc-settings", oidc.ClientId.Name)
	require.Equal(t, "clientId", oidc.ClientId.Key)
	require.Equal(t, "oidc-settings", oidc.ClientSecret.Name)
	require.Equal(t, "clientSecret", oidc.ClientSecret.Key)

	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation,
		"no literals provided, so no annotation should be created")
}

func TestConvertTo_OIDCMixedLiteralsAndRefs(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId":   "abc",
					"authMethod": "client_secret_post",
					"secret": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "oidc-secret",
								"key":  "clientSecret",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "oidc-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name)
	require.Empty(t, dst.Spec.Wandb.OIDC.ClientId.Name)

	raw := dst.Annotations[OIDCPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "abc", decoded["clientId"])
	require.Equal(t, "client_secret_post", decoded["authMethod"])
	require.NotContains(t, decoded, "secret")
}

func TestConvertTo_OIDCValueFromWinsOverLegacyOidcSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"secret": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "valueFrom-secret",
								"key":  "clientSecret",
							},
						},
					},
					"oidcSecret": map[string]interface{}{
						"name":      "legacy-secret",
						"secretKey": "LEGACY_KEY",
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "valueFrom-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name,
		"secret.valueFrom should win over the legacy oidcSecret block")
}

func TestConvertTo_OIDCMalformedRefMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "no-key-here",
							},
						},
					},
				},
			},
		},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clientId")
}

func TestConvertTo_OIDCAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"host": "http://wandb.localhost",
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation)
	require.Empty(t, dst.Spec.Wandb.OIDC.ClientId.Name)
}

func TestConvertTo_MySQLAllLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host":     "mysql.example.com",
				"port":     int64(3306),
				"database": "wandb_local",
				"user":     "wandb",
				"password": "shh",
				"caCert":   "---cert---",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw, ok := dst.Annotations[MySQLPendingAnnotation]
	require.True(t, ok, "expected mysql-pending annotation")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "mysql.example.com", decoded["host"])
	require.Equal(t, "wandb_local", decoded["database"])
	require.Equal(t, "wandb", decoded["user"])
	require.Equal(t, "shh", decoded["password"])
	require.Equal(t, "---cert---", decoded["caCert"])
	require.NotContains(t, decoded, "passwordSecret")

	require.NotNil(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql, "externalMysql is always allocated; reconciler fills selectors from the annotation")
	require.Empty(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Host.Name)
	require.Empty(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Password.Name)
}

func TestConvertTo_MySQLLegacyPasswordSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host":     "mysql.example.com",
				"password": "shh",
				"passwordSecret": map[string]interface{}{
					"name":            "mysql-creds",
					"rootPasswordKey": "MYSQL_ROOT_PASSWORD",
					"passwordKey":     "MYSQL_PASSWORD",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql)
	require.Equal(t, "mysql-creds", dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Password.Name)
	require.Equal(t, "MYSQL_PASSWORD", dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Password.Key)

	raw := dst.Annotations[MySQLPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "mysql.example.com", decoded["host"])
	require.NotContains(t, decoded, "password", "literal password must not be stashed when passwordSecret took over")
	require.NotContains(t, decoded, "passwordSecret")
}

func TestConvertTo_MySQLLegacyPasswordSecretDefaultKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"passwordSecret": map[string]interface{}{
					"name": "mysql-creds",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql)
	require.Equal(t, "mysql-creds", dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Password.Name)
	require.Equal(t, "MYSQL_PASSWORD", dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Password.Key)
}

func TestConvertTo_MySQLValueFromRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "mysql-settings",
							"key":  "endpoint",
						},
					},
				},
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "mysql-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql)
	conn := dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql
	require.Equal(t, "mysql-settings", conn.Host.Name)
	require.Equal(t, "endpoint", conn.Host.Key)
	require.Equal(t, "mysql-secret", conn.Password.Name)
	require.Equal(t, "password", conn.Password.Key)

	require.NotContains(t, dst.Annotations, MySQLPendingAnnotation,
		"no literals provided, so no annotation should be created")
}

func TestConvertTo_MySQLMixedLiteralsAndRefs(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host": "mysql.example.com",
				"port": int64(3306),
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "mysql-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql)
	require.Equal(t, "mysql-secret", dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Password.Name)
	require.Empty(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Host.Name)

	raw := dst.Annotations[MySQLPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "mysql.example.com", decoded["host"])
	require.Contains(t, decoded, "port")
	require.NotContains(t, decoded, "password")
}

func TestConvertTo_MySQLValueFromWinsOverPasswordSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "valueFrom-secret",
							"key":  "password",
						},
					},
				},
				"passwordSecret": map[string]interface{}{
					"name":        "legacy-secret",
					"passwordKey": "LEGACY_KEY",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql)
	require.Equal(t, "valueFrom-secret", dst.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Password.Name,
		"password.valueFrom should win over the legacy passwordSecret block")
}

func TestConvertTo_MySQLMalformedRefMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "no-key-here",
						},
					},
				},
			},
		},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "host")
}

func TestConvertTo_MySQLAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, MySQLPendingAnnotation)
}

func TestConvertTo_MySQLEmptyMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, MySQLPendingAnnotation)
}

func TestConvertTo_RedisAllLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host":     "redis.example.com",
				"port":     int64(6379),
				"password": "shh",
				"external": true,
				"caCert":   "----BEGIN CERT----",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	raw, ok := dst.Annotations[RedisPendingAnnotation]
	require.True(t, ok, "expected redis-pending annotation")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "redis.example.com", decoded["host"])
	require.Equal(t, "shh", decoded["password"])
	require.Equal(t, "----BEGIN CERT----", decoded["caCert"])
	require.NotContains(t, decoded, "external", "fields outside the known v2 mapping must be dropped")
	require.NotContains(t, decoded, "secret")

	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis, "externalRedis is always allocated; reconciler fills selectors from the annotation")
	require.Empty(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Host.Name)
}

func TestConvertTo_RedisLegacySecretRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host":     "redis.example.com",
				"password": "shh",
				"secret": map[string]interface{}{
					"secretName": "redis-creds",
					"secretKey":  "REDIS_PASSWORD",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis)
	require.Equal(t, "redis-creds", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Password.Name)
	require.Equal(t, "REDIS_PASSWORD", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Password.Key)

	raw := dst.Annotations[RedisPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "redis.example.com", decoded["host"])
	require.NotContains(t, decoded, "password", "literal password must not be stashed when secret ref took over")
}

func TestConvertTo_RedisLegacySecretRefDefaultKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "redis-creds",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis)
	require.Equal(t, "redis-creds", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Password.Name)
	require.Equal(t, "REDIS_PASSWORD", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Password.Key)
}

func TestConvertTo_RedisValueFromRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "redis-settings",
							"key":  "endpoint",
						},
					},
				},
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "redis-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis)
	conn := dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis
	require.Equal(t, "redis-settings", conn.Host.Name)
	require.Equal(t, "endpoint", conn.Host.Key)
	require.Equal(t, "redis-secret", conn.Password.Name)
	require.Equal(t, "password", conn.Password.Key)

	require.NotContains(t, dst.Annotations, RedisPendingAnnotation,
		"no literals provided, so no annotation should be created")
}

func TestConvertTo_RedisMixedLiteralsAndRefs(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host": "redis.example.com",
				"port": int64(6379),
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "redis-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis)
	require.Equal(t, "redis-secret", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Password.Name)
	require.Empty(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Host.Name)

	raw := dst.Annotations[RedisPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "redis.example.com", decoded["host"])
	require.Contains(t, decoded, "port")
	require.NotContains(t, decoded, "password")
}

func TestConvertTo_RedisValueFromWinsOverLegacySecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "valueFrom-secret",
							"key":  "password",
						},
					},
				},
				"secret": map[string]interface{}{
					"secretName": "legacy-secret",
					"secretKey":  "LEGACY_KEY",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis)
	require.Equal(t, "valueFrom-secret", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Password.Name,
		"password.valueFrom should win over the legacy secret block")
}

func TestConvertTo_RedisMalformedRefMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "no-key-here",
						},
					},
				},
			},
		},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "host")
}

func TestConvertTo_RedisTLSValueFromInParams(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"params": map[string]interface{}{
					"tls": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "redis-tls",
								"key":  "enabled",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis)
	require.Equal(t, "redis-tls", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Name)
	require.Equal(t, "enabled", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Key)
}

func TestConvertTo_RedisTLSValueFromInParameters(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"parameters": map[string]interface{}{
					"tls": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "redis-tls",
								"key":  "enabled",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotNil(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis)
	require.Equal(t, "redis-tls", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Name)
	require.Equal(t, "enabled", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Key)
}

func TestConvertTo_RedisTLSParamsWinsOverParameters(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"params": map[string]interface{}{
					"tls": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "from-params",
								"key":  "tls",
							},
						},
					},
				},
				"parameters": map[string]interface{}{
					"tls": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "from-parameters",
								"key":  "tls",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "from-params", dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Name,
		"params should be checked before parameters")
}

func TestConvertTo_RedisTLSLiteralStashedInAnnotation(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"params": map[string]interface{}{
					"tls": "true",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw, ok := dst.Annotations[RedisPendingAnnotation]
	require.True(t, ok, "expected redis-pending annotation for the literal tls")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "true", decoded["tls"])

	require.Empty(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Name,
		"literal tls should not be set on the spec; reconciler materializes it")
}

func TestConvertTo_RedisTLSAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host": "redis.example.com",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Name)
	require.Empty(t, dst.Spec.Redis[appsv2.DefaultInstanceName].ExternalRedis.Tls.Key)
}

// TestConvertTo_RedisTLSBooleanStashedAsString locks in that a YAML boolean
// tls toggle is stringified before being stashed. Without this the
// reconciler's string-typed payload field fails to decode a JSON boolean.
func TestConvertTo_RedisTLSBooleanStashedAsString(t *testing.T) {
	for _, tc := range []struct {
		name string
		tls  interface{}
		want string
	}{
		{"true", true, "true"},
		{"false", false, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dst := &appsv2.WeightsAndBiases{}
			src := newV1(map[string]interface{}{
				"global": map[string]interface{}{
					"redis": map[string]interface{}{
						"params": map[string]interface{}{"tls": tc.tls},
					},
				},
			})
			require.NoError(t, src.ConvertTo(dst))

			raw := dst.Annotations[RedisPendingAnnotation]
			// The stashed value must be a JSON string, not a JSON boolean.
			require.Contains(t, raw, `"tls":"`+tc.want+`"`)

			var decoded map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
			require.Equal(t, tc.want, decoded["tls"])
		})
	}
}

// TestConvertTo_RedisNumericPortStashedAsString confirms numeric scalars are
// also stringified at stash time.
func TestConvertTo_RedisNumericPortStashedAsString(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"port": float64(6379),
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw := dst.Annotations[RedisPendingAnnotation]
	require.Contains(t, raw, `"port":"6379"`)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "6379", decoded["port"])
}

func TestConvertTo_RedisAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, RedisPendingAnnotation)
}

func TestConvertTo_RedisEmptyMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, RedisPendingAnnotation)
}

func TestConvertTo_BucketSecretRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName":    "bucket-creds",
					"accessKeyName": "MY_ACCESS",
					"secretKeyName": "MY_SECRET",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore)
	ext := dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore
	require.Equal(t, "bucket-creds", ext.AccessKey.Name)
	require.Equal(t, "MY_ACCESS", ext.AccessKey.Key)
	require.Equal(t, "bucket-creds", ext.SecretKey.Name)
	require.Equal(t, "MY_SECRET", ext.SecretKey.Key)

	require.NotContains(t, dst.Annotations, BucketPendingAnnotation,
		"no literals besides the secret block, so no annotation should be created")
}

func TestConvertTo_BucketSecretRefDefaultKeys(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "bucket-creds",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore)
	ext := dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore
	require.Equal(t, "ACCESS_KEY", ext.AccessKey.Key)
	require.Equal(t, "SECRET_KEY", ext.SecretKey.Key)
}

func TestConvertTo_BucketSecretRefEmptyName(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "",
				},
				"provider": "s3",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore,
		"externalObjectStore is always allocated; reconciler fills selectors from the annotation")
	require.Empty(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore.AccessKey.Name,
		"empty secretName should not produce an AccessKey selector")
	require.Empty(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore.SecretKey.Name)

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.NotContains(t, decoded, "secret", "secret block must be stripped even when its secretName is empty")
	require.Equal(t, "s3", decoded["provider"])
}

func TestConvertTo_BucketLiteralsOnlyBucket(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider":  "s3",
				"name":      "wandb-bucket",
				"region":    "us-east-1",
				"accessKey": "AKIA...",
				"secretKey": "secret",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotNil(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore,
		"externalObjectStore is always allocated; literals stay in the annotation")
	require.Empty(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore.AccessKey.Name)

	raw, ok := dst.Annotations[BucketPendingAnnotation]
	require.True(t, ok)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "s3", decoded["provider"])
	require.Equal(t, "wandb-bucket", decoded["name"])
	require.Equal(t, "us-east-1", decoded["region"])
	require.Equal(t, "AKIA...", decoded["accessKey"])
	require.Equal(t, "secret", decoded["secretKey"])
}

func TestConvertTo_BucketLiteralsOnlyDefaultBucket(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"defaultBucket": map[string]interface{}{
				"provider": "s3",
				"name":     "wandb-bucket",
				"region":   "us-east-1",
				"kmsKey":   "arn:aws:kms:...",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw, ok := dst.Annotations[BucketPendingAnnotation]
	require.True(t, ok)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "s3", decoded["provider"])
	require.Equal(t, "wandb-bucket", decoded["name"])
	require.Equal(t, "us-east-1", decoded["region"])
	require.Equal(t, "arn:aws:kms:...", decoded["kmsKey"])
}

func TestConvertTo_BucketLiteralsMerged_BucketWins(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider": "gcs",
				"name":     "bucket-name",
			},
			"defaultBucket": map[string]interface{}{
				"provider": "s3",
				"name":     "default-name",
				"region":   "us-east-1",
				"kmsKey":   "kms-fallback",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "gcs", decoded["provider"], "bucket overrides defaultBucket")
	require.Equal(t, "bucket-name", decoded["name"], "bucket overrides defaultBucket")
	require.Equal(t, "us-east-1", decoded["region"], "defaultBucket fills in fields bucket didn't set")
	require.Equal(t, "kms-fallback", decoded["kmsKey"])
}

func TestConvertTo_BucketEmptyValueDoesNotOverride(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider": "",
			},
			"defaultBucket": map[string]interface{}{
				"provider": "s3",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "s3", decoded["provider"], "empty bucket value must not erase the defaultBucket value")
}

func TestConvertTo_BucketSecretRefAndLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "bucket-creds",
				},
				"provider": "s3",
				"name":     "wandb-bucket",
			},
			"defaultBucket": map[string]interface{}{
				"region": "us-east-1",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore)
	require.Equal(t, "bucket-creds", dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore.AccessKey.Name)

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.NotContains(t, decoded, "secret")
	require.Equal(t, "s3", decoded["provider"])
	require.Equal(t, "wandb-bucket", decoded["name"])
	require.Equal(t, "us-east-1", decoded["region"])
}

func TestConvertTo_BucketAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, BucketPendingAnnotation)
	require.Nil(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore)
}

func TestConvertTo_BucketEmptyMaps(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket":        map[string]interface{}{},
			"defaultBucket": map[string]interface{}{},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, BucketPendingAnnotation)
	require.Nil(t, dst.Spec.ObjectStore[appsv2.DefaultInstanceName].ExternalObjectStore)
}

func TestConvertTo_BucketAllEmptyValues(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider": "",
				"name":     "",
			},
			"defaultBucket": map[string]interface{}{
				"region": "",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, BucketPendingAnnotation,
		"every value is empty, so the merged annotation should be skipped")
}

func TestConvertTo_GlobalNotAMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": "not-a-map",
	})
	require.Error(t, src.ConvertTo(dst))
}

func TestConvertTo_PreservesObjectMeta(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(nil)
	src.Labels = map[string]string{"app.kubernetes.io/name": "weightsandbiases"}
	src.Annotations = map[string]string{"existing": "value"}
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "weightsandbiases", dst.Labels["app.kubernetes.io/name"])
	require.Equal(t, "value", dst.Annotations["existing"])
}

// TestConvertRoundTrip locks in the v1 → v2 → v1 → v2 lossless round-trip
// behavior. kube-apiserver bounces objects through this sequence during
// admission, so any data ConvertFrom can't recover would be silently dropped
// from the persisted v2 spec on the next pass.
func TestConvertRoundTrip(t *testing.T) {
	original := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"host": "http://wandb.localhost",
			"mysql": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "mysql-creds",
							"key":  "Host",
						},
					},
				},
			},
		},
	})
	original.Spec.Chart.Object = map[string]interface{}{
		"name":    "operator-wandb",
		"version": "0.37.1",
	}

	// First conversion (apiserver applies the v1 manifest).
	firstV2 := &appsv2.WeightsAndBiases{}
	require.NoError(t, original.ConvertTo(firstV2))
	require.Equal(t, "http://wandb.localhost", firstV2.Spec.Wandb.Hostname)
	require.NotNil(t, firstV2.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql)
	require.Equal(t, "mysql-creds", firstV2.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Host.Name)

	// Apiserver bounces through ConvertFrom internally.
	roundTripped := &WeightsAndBiases{}
	require.NoError(t, roundTripped.ConvertFrom(firstV2))

	// And then ConvertTo again. The second v2 must match the first; otherwise
	// the round-trip silently erases data.
	secondV2 := &appsv2.WeightsAndBiases{}
	require.NoError(t, roundTripped.ConvertTo(secondV2))

	require.Equal(t, firstV2.Spec.Wandb.Hostname, secondV2.Spec.Wandb.Hostname)
	require.NotNil(t, secondV2.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql)
	require.Equal(t, firstV2.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Host, secondV2.Spec.MySQL[appsv2.DefaultInstanceName].ExternalMysql.Host)
}

func TestConvertFrom_NoAnnotations(t *testing.T) {
	dst := &WeightsAndBiases{}
	src := &appsv2.WeightsAndBiases{}
	require.NoError(t, dst.ConvertFrom(src))
	require.NotNil(t, dst.Spec.Chart.Object)
	require.Empty(t, dst.Spec.Chart.Object)
	require.NotNil(t, dst.Spec.Values.Object)
	require.Empty(t, dst.Spec.Values.Object)
}

func TestConvertTo_ActiveSpecSecretOverridesCRValues(t *testing.T) {
	withConversionReader(t, activeSpecSecret(t, "default", "wandb", map[string]interface{}{
		"global": map[string]interface{}{
			"host":    "http://wandb.from-active-spec",
			"license": "active-spec-license",
		},
		"app": map[string]interface{}{
			"image": map[string]interface{}{"tag": "0.80.5"},
		},
	}))

	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"host":    "http://wandb.from-cr",
			"license": "cr-license",
		},
	})
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "http://wandb.from-active-spec", dst.Spec.Wandb.Hostname,
		"active-spec Secret should override CR values")
	require.Equal(t, "active-spec-license", dst.Spec.Wandb.License)
	require.Equal(t, "0.80.5", dst.Spec.Wandb.Version,
		"version derives from app.image.tag in the active-spec values")
}

func TestConvertTo_ActiveSpecAbsentFallsBackToCRValues(t *testing.T) {
	// Reader is wired up but the active-spec Secret isn't present.
	withConversionReader(t)

	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://wandb.from-cr"},
	})
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "http://wandb.from-cr", dst.Spec.Wandb.Hostname)
}

func TestConvertTo_ActiveSpecMissingValuesKeyFallsBackToCR(t *testing.T) {
	withConversionReader(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-spec-active",
			Namespace: "default",
		},
		// Only chart, no values — should still fall back to CR.
		Data: map[string][]byte{"chart": []byte(`{}`)},
	})

	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://wandb.from-cr"},
	})
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "http://wandb.from-cr", dst.Spec.Wandb.Hostname)
}

func TestConvertTo_ActiveSpecMalformedJSONErrors(t *testing.T) {
	withConversionReader(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-spec-active",
			Namespace: "default",
		},
		Data: map[string][]byte{"values": []byte("not json")},
	})

	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	dst := &appsv2.WeightsAndBiases{}
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wandb-spec-active")
}

func TestConvertTo_ActiveSpecScopedByCRName(t *testing.T) {
	// Active-spec Secret exists but for a different CR name.
	withConversionReader(t, activeSpecSecret(t, "default", "some-other-cr", map[string]interface{}{
		"global": map[string]interface{}{"host": "http://wandb.from-other-cr"},
	}))

	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://wandb.from-cr"},
	})
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "http://wandb.from-cr", dst.Spec.Wandb.Hostname,
		"active spec lookup must be scoped to this CR's name")
}

// TestConvertTo_StashedAnnotationsReflectCRNotActiveSpec confirms that the
// round-trip annotations carry the raw v1 CR values (not the merged active
// spec). This preserves lossless round-trips even when the active-spec Secret
// is mutated between consecutive ConvertFrom/ConvertTo bounces.
func TestConvertTo_StashedAnnotationsReflectCRNotActiveSpec(t *testing.T) {
	withConversionReader(t, activeSpecSecret(t, "default", "wandb", map[string]interface{}{
		"global": map[string]interface{}{"host": "http://wandb.from-active-spec"},
	}))

	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://wandb.from-cr"},
	})
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, src.ConvertTo(dst))

	stashed := dst.Annotations[v1ValuesAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stashed), &decoded))
	global := decoded["global"].(map[string]interface{})
	require.Equal(t, "http://wandb.from-cr", global["host"],
		"stashed annotation must preserve the CR's raw values for round-trip")
}
