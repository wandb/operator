package pod

import (
	"context"
	"encoding/json"
	"os"
	"time"

	v1 "github.com/wandb/operator/api/v1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func GetJobSpec(s interface{}) *JobSpec {
	if spec, ok := s.(*JobSpec); ok {
		return spec
	}
	return nil
}

type JobSpec struct {
	Type      string `default:"job"`
	Container corev1.Container
}

func (j JobSpec) Envs(wandb *v1.WeightsAndBiases, scheme *runtime.Scheme) {
	gvk, _ := apiutil.GVKForObject(wandb, scheme)
	ownerReference, _ := json.Marshal(map[string]interface{}{
		"apiVersion":         gvk.GroupVersion().String(),
		"blockOwnerDeletion": true,
		"controller":         true,
		"kind":               gvk.Kind,
		"name":               wandb.GetName(),
		"uid":                wandb.GetUID(),
	})

	j.Container.Env = append(
		j.Container.Env,

		corev1.EnvVar{Name: "OPERATOR_OWNER_REFERENCE", Value: string(ownerReference)},
		corev1.EnvVar{Name: "OPERATOR_NAMESPACE", Value: os.Getenv("OPERATOR_NAMESPACE")},

		corev1.EnvVar{Name: "WANDB_CR_NAMESPACE", Value: wandb.GetNamespace()},
		corev1.EnvVar{Name: "WANDB_CR_NAME", Value: wandb.GetName()},
	)
}

func (j JobSpec) Job(owner metav1.Object, scheme *runtime.Scheme) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      owner.GetName() + "-release",
			Namespace: owner.GetNamespace(),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes:    []corev1.Volume{},
					Containers: []corev1.Container{j.Container},
				},
			},
		},
	}
}

func waitForCompletion(ctx context.Context, job *batchv1.Job, c client.Client, delete bool) error {
	for {
		j := &batchv1.Job{}
		err := c.Get(
			ctx,
			client.ObjectKey{
				Namespace: job.GetNamespace(),
				Name:      job.GetName(),
			},
			j,
		)

		if errors.IsNotFound(err) {
			break
		}

		if j.Status.CompletionTime == nil {
			if delete {
				err := c.Delete(ctx, j)
				if err != nil {
					return err
				}
			}
			break
		}

		time.Sleep(10 * time.Second)
	}
	return nil
}

func (j JobSpec) Apply(
	ctx context.Context,
	c client.Client,
	owner *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config map[string]interface{},
) error {
	job := j.Job(owner, scheme)

	waitForCompletion(ctx, job, c, true)

	if err := controllerutil.SetControllerReference(owner, job, scheme); err != nil {
		return err
	}

	if err := c.Create(ctx, job); err != nil {
		return err
	}

	waitForCompletion(ctx, job, c, false)

	return nil
}
