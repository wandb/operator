package preflight

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	probeTimeout = 90 * time.Second
	// Lets leftovers be found with: kubectl delete pod -l app.kubernetes.io/managed-by=wandb-preflight
	probeManagedByLabel = "wandb-preflight"
	// Public image; air-gapped installs will need it mirrored (follow-up: prefix spec.global.imageRegistry).
	probeImage = "busybox:1.36"
)

func newProbePod(namespace, checkName string, command []string, env []corev1.EnvVar) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wandb-preflight-",
			Namespace:    namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": probeManagedByLabel,
				"preflight.wandb.com/check":    checkName,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			ActiveDeadlineSeconds:         ptr.To[int64](60),
			TerminationGracePeriodSeconds: ptr.To[int64](0),
			AutomountServiceAccountToken:  ptr.To(false),
			// Restricted PodSecurity namespaces reject anything less.
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   ptr.To(true),
				RunAsUser:      ptr.To[int64](65534),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name:    "probe",
				Image:   probeImage,
				Command: command,
				Env:     env,
				// The probe's stderr becomes the termination message, so failures explain themselves.
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}
}

// runProbePod creates pod, waits for it to finish, and always deletes it. It returns the final phase and the
// container's termination message.
func runProbePod(ctx context.Context, c client.Client, pod *corev1.Pod) (corev1.PodPhase, string,
	error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	if err := c.Create(ctx, pod); err != nil {
		return "", "", fmt.Errorf("create probe pod: %w", err)
	}
	defer func() {
		// Fresh context: ctx may have expired, and the pod must not be left behind.
		delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer delCancel()
		_ = c.Delete(delCtx, pod, client.GracePeriodSeconds(0))
	}()

	key := client.ObjectKeyFromObject(pod)
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool,
		error) {
		if err := c.Get(ctx, key, pod); err != nil {
			return false, err
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			return true, nil
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if w := cs.State.Waiting; w != nil && (w.Reason == "ErrImagePull" || w.Reason == "ImagePullBackOff") {
				return false, fmt.Errorf("probe image %s: %s", probeImage, w.Reason)
			}
		}
		return false, nil
	})
	if err != nil {
		return pod.Status.Phase, "", err
	}
	var msg string
	for _, cs := range pod.Status.ContainerStatuses {
		if t := cs.State.Terminated; t != nil {
			msg = t.Message
		}
	}
	return pod.Status.Phase, msg, nil
}
