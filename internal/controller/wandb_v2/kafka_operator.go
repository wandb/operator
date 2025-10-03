package wandb_v2

import (
	"context"
	"errors"

	apiv2 "github.com/wandb/operator/api/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesV2Reconciler) handleKafkaOperator(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	if wandb.Spec.Streaming.Type != apiv2.WBKafkaStreaming {
		log.Info("Kafka Streaming handler skipping since not kafka", "type", wandb.Spec.Streaming.Type)
		return ctrl.Result{}, nil
	}
	if !wandb.Spec.Streaming.Enabled {
		log.Info("Kafka Streaming handler for enabled", "enabled", wandb.Spec.Streaming.Enabled)
		return ctrl.Result{}, nil
	}
	streamingStatus := wandb.Status.StreamingStatus
	log.Info("Streaming Reconcile", "reconcileStatus", streamingStatus.ReconciliationStatus)
	switch streamingStatus.ReconciliationStatus {
	case "":
		log.Info("handle empty reconcile status")
		wandb.Status.StreamingStatus.ReconciliationStatus = apiv2.WBInfraStatusMissing
		if err := r.Client.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	case apiv2.WBInfraStatusMissing:
		log.Info("handle 'Missing' reconcile status")
		return r.handleKafkaMissingStatus(ctx, wandb, req)
	}
	err := errors.New("not implemented")
	log.Error(err, "reconcile status handling not implemented for", "reconcileStatus", streamingStatus.ReconciliationStatus)
	return ctrl.Result{}, nil
}

func (r *WeightsAndBiasesV2Reconciler) handleKafkaMissingStatus(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := r.Client.List(ctx, crdList); err != nil {
		log.Error(err, "failed to list CRDs")
		return ctrl.Result{}, err
	}

	for _, crd := range crdList.Items {
		if crd.Spec.Group == "kafka.strimzi.io" || crd.Spec.Group == "core.strimzi.io" {
			log.Info("Found Kafka CRD", "name", crd.Name)

			for _, version := range crd.Spec.Versions {
				gvr := schema.GroupVersionResource{
					Group:    crd.Spec.Group,
					Version:  version.Name,
					Resource: crd.Spec.Names.Plural,
				}

				list := &unstructured.UnstructuredList{}
				list.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   gvr.Group,
					Version: gvr.Version,
					Kind:    crd.Spec.Names.Kind + "List",
				})

				if err := r.Client.List(ctx, list); err != nil {
					log.Error(err, "failed to list resources", "gvr", gvr)
					continue
				}

				for _, item := range list.Items {
					log.Info("Found resource", "kind", crd.Spec.Names.Kind, "name", item.GetName(), "namespace", item.GetNamespace())
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *WeightsAndBiasesV2Reconciler) installKafkaOperator(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (ctrl.Result, error) {

	log := ctrllog.FromContext(ctx)
	log.Info("Installing Kafka Operator...")

	return ctrl.Result{}, nil
}
