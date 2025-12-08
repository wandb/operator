package translator

import corev1 "k8s.io/api/core/v1"

type InfraConnection struct {
	URL corev1.SecretKeySelector
}
