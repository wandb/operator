package cdk8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cdk8sTar struct {
	TarPath string
	Values  *map[string]interface{}
}

func (c Cdk8sTar) Apply(
	ctx context.Context,
	client client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
) error {
	return nil
}

type Cdk8sDownloadTar struct {
	DownloadURL string
	Values      map[string]interface{}
}

func (c Cdk8sDownloadTar) Apply(
	ctx context.Context,
	client client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
) error {
	return nil
}
