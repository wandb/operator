package wandb_v2

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	apiv2 "github.com/wandb/operator/api/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesV2Reconciler) handleKafkaOperator(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (ctrl.Result, error) {

	if wandb.Spec.Infra.Streaming.Type == apiv2.WBKafkaStreaming || wandb.Spec.Infra.Streaming.Enabled {

		if wandb.Status.ComponentStatus.InfraStatus.StreamingStatus.ReconciliationStatus == apiv2.WBInfraStatusMissing {
			return r.installKafkaOperator(ctx, wandb, req)
		}
	}
	return ctrl.Result{}, nil
}

func (r *WeightsAndBiasesV2Reconciler) installKafkaOperator(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (ctrl.Result, error) {

	log := ctrllog.FromContext(ctx)
	log.Info("Installing Kafka Operator...")

	if wandb.Spec.Infra.Streaming.ConfigMapRef == nil {
		log.Error(nil, "ConfigMapRef is required for Kafka operator installation")
		return ctrl.Result{}, fmt.Errorf("streaming.configMapRef is required")
	}

	templates, err := r.loadKafkaTemplates(ctx, wandb.Spec.Infra.Streaming.ConfigMapRef)
	if err != nil {
		log.Error(err, "Failed to load Kafka operator templates")
		return ctrl.Result{}, err
	}

	namespace := wandb.Spec.Infra.Streaming.ConfigMapRef.Namespace
	if namespace == "" {
		namespace = req.Namespace
	}

	for filename, templateContent := range templates {
		if err := r.applyKafkaTemplate(ctx, templateContent, namespace); err != nil {
			log.Error(err, "Failed to apply Kafka template", "filename", filename)
			return ctrl.Result{}, err
		}
		log.Info("Applied Kafka template", "filename", filename)
	}

	return ctrl.Result{}, nil
}

func (r *WeightsAndBiasesV2Reconciler) loadKafkaTemplates(
	ctx context.Context, ref *apiv2.ConfigMapReference,
) (map[string]string, error) {

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	if ref.Key != "" {
		data, exists := configMap.Data[ref.Key]
		if exists {
			return map[string]string{ref.Key: data}, nil
		}
		return nil, fmt.Errorf("key %s not found in ConfigMap %s/%s", ref.Key, ref.Namespace, ref.Name)
	}

	return configMap.Data, nil
}

func (r *WeightsAndBiasesV2Reconciler) applyKafkaTemplate(
	ctx context.Context, templateContent string, namespace string,
) error {

	tmpl, err := template.New("kafka").Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"Namespace": namespace}); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(&buf, 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(obj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("failed to decode manifest: %w", err)
		}

		if obj.GetKind() == "" {
			continue
		}

		if err := r.Client.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("wandb-operator")); err != nil {
			return fmt.Errorf("failed to apply manifest %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	return nil
}
