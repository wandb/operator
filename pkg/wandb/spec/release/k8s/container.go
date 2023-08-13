package k8s

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ContainerRelease struct {
	Image   string            `json:"image" validate:"required"`
	Envs    map[string]string `json:"envs"`
	Command []string          `json:"command"`
}

func (c *ContainerRelease) Validate() error {
	return validator.New().Struct(c)
}

func (s ContainerRelease) Apply(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config map[string]interface{},
) error {
	serviceAccount := os.Getenv("SERVICE_ACCOUNT_NAME")
	if serviceAccount == "" {
		serviceAccount = createAdminServiceAccount(ctx, c, wandb)
	}

	envs := []corev1.EnvVar{}
	if s.Envs != nil {
		for k, v := range s.Envs {
			envs = append(envs, corev1.EnvVar{Name: k, Value: v})
		}
	}

	tru := true
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wandb.GetName() + "-apply",
			Namespace: wandb.GetNamespace(),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName:           serviceAccount,
					AutomountServiceAccountToken: &tru,
					Containers: []corev1.Container{
						{
							Name:            "apply",
							Image:           s.Image,
							Command:         s.Command,
							ImagePullPolicy: corev1.PullAlways,
							Env:             envs,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	if err := WaitForJobCompletion(ctx, wandb, c); err != nil {
		fmt.Println(err)
	}

	deletePolicy := metav1.DeletePropagationBackground
	deleteOptions := &client.DeleteOptions{PropagationPolicy: &deletePolicy}
	if err := c.Delete(ctx, job, deleteOptions); err != nil {
		fmt.Println(err)
	}

	if err := controllerutil.SetControllerReference(wandb, job, scheme); err != nil {
		return err
	}

	if err := c.Create(ctx, job); err != nil {
		return err
	}

	// Don't delete so we can debug better
	if err := WaitForJobCompletion(ctx, wandb, c); err != nil {
		fmt.Println(err)
	}

	return nil
}

func WaitForJobCompletion(ctx context.Context, wandb *v1.WeightsAndBiases, c client.Client) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wandb.GetName() + "-apply",
			Namespace: wandb.GetNamespace(),
		},
	}
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

		if j.Status.CompletionTime != nil || j.Status.Failed > 0 {
			break
		}

		time.Sleep(10 * time.Second)
	}
	return nil
}

func createAdminServiceAccount(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
) string {
	serviceAccount := "controller-admin"
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccount,
			Namespace: wandb.GetNamespace(), // change this to your desired namespace
		},
	}
	client.Create(ctx, sa)

	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceAccount + "-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Verbs:     []string{"*"},
				Resources: []string{"*"},
			},
		},
	}
	client.Create(ctx, clusterRole)

	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceAccount + "-rolebinding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name, // This is the clusterrole we created in the previous step
		},
	}
	client.Create(ctx, roleBinding)
	return serviceAccount
}
