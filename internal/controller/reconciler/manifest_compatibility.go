package reconciler

import (
	"context"
	"errors"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"github.com/wandb/operator/pkg/wandb/manifest/registryauth"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const manifestCompatibleCondition = "ManifestCompatible"

func loadCompatibleManifest(ctx context.Context, c client.Client, recorder record.EventRecorder, wandb *apiv2.WeightsAndBiases) (manifest.Manifest, ctrl.Result, error) {
	var auth *manifest.RegistryAuth
	var m manifest.Manifest
	var err error
	if !manifest.IsFileRepository(wandb.Spec.Wandb.ManifestRepository) {
		allowAmbient := wandb.Spec.Wandb.ManifestRepository != apiv2.DefaultManifestRepository
		auth, err = registryauth.Resolve(ctx, c, wandb.Namespace, wandb.Spec.Global.ImagePullSecrets, allowAmbient)
	}
	if err == nil {
		m, err = manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version, auth)
	}
	result, reportErr := reportManifestCompatibility(ctx, c, recorder, wandb, m, err)
	return m, result, reportErr
}

// reportManifestCompatibility persists the loader result before any manifest-driven
// writes. A permanent version error waits for a spec change or bounded retry;
// retrieval and status-write failures retain normal controller error retries.
func reportManifestCompatibility(ctx context.Context, c client.Client, recorder record.EventRecorder, wandb *apiv2.WeightsAndBiases, m manifest.Manifest, loadErr error) (ctrl.Result, error) {
	before := wandb.DeepCopy().Status
	previous := apimeta.FindStatusCondition(before.Conditions, manifestCompatibleCondition)
	condition := metav1.Condition{
		Type: manifestCompatibleCondition, Status: metav1.ConditionTrue,
		ObservedGeneration: wandb.Generation, Reason: "SupportedManifestVersion",
		Message: fmt.Sprintf("Server manifest %s:%s uses supported manifestVersion %d", wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version, m.ManifestVersion),
	}
	if loadErr == nil && !m.VersionExplicit() {
		condition.Message += " (defaulted from an omitted declaration)"
	}
	var unsupported *manifest.UnsupportedManifestVersionError
	var invalid *manifest.InvalidManifestVersionError
	var decode *manifest.ManifestDecodeError
	permanent := false
	if loadErr != nil {
		condition.Message = fmt.Sprintf("Server manifest %s:%s: %v", wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version, loadErr)
		switch {
		case errors.As(loadErr, &unsupported):
			condition.Status, condition.Reason = metav1.ConditionFalse, "UnsupportedManifestVersion"
			condition.Message += ". Install an operator supporting this manifest version or select a supported server manifest."
			permanent = true
		case errors.As(loadErr, &invalid):
			condition.Status, condition.Reason = metav1.ConditionFalse, "InvalidManifestVersion"
			permanent = true
		case errors.As(loadErr, &decode):
			condition.Status, condition.Reason = metav1.ConditionUnknown, "InvalidManifest"
		default:
			condition.Status, condition.Reason = metav1.ConditionUnknown, "ManifestUnavailable"
		}
		setReadyStatus(wandb, false, condition.Reason, condition.Message)
	} else if previous != nil && previous.Status != metav1.ConditionTrue {
		setReadyStatus(wandb, false, "Reconciling", "Manifest compatibility restored; reconciling the requested configuration")
	}
	changed := previous == nil || previous.Status != condition.Status || previous.Reason != condition.Reason || previous.Message != condition.Message
	apimeta.SetStatusCondition(&wandb.Status.Conditions, condition)
	if err := updateWandbStatusIfChanged(ctx, c, wandb, before); err != nil {
		return ctrl.Result{}, err
	}
	if changed && recorder != nil {
		eventType := corev1.EventTypeNormal
		if loadErr != nil {
			eventType = corev1.EventTypeWarning
		}
		recorder.Event(wandb, eventType, condition.Reason, condition.Message)
	}
	if permanent {
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}
	return ctrl.Result{}, loadErr
}
