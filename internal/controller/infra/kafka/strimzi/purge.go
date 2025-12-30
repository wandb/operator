package strimzi

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PostDeletePurge(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
) error {

}
