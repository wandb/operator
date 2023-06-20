package minio

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const RootUser = "minio"
const RootPassword = "miniowandb"
const RegionName = "us-east-1"
const BucketName = "wandb"

func CreateBucketJob(endpoint string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "wandb",
			Name:      "create-bucket",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "example",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "create-bucket",
							Image: "minio/mc:latest",
							Env: []corev1.EnvVar{
								{Name: "MINIO_ACCESS_KEY", Value: RootUser},
								{Name: "MINIO_SECRET_KEY", Value: RootPassword},
								{Name: "MINIO_SERVER_ENDPOINT", Value: endpoint + ":9000"},
								{Name: "BUCKET_NAME", Value: BucketName},
							},
							Command: []string{
								"/bin/sh",
								"-c",
								`echo "Creating MinIO bucket..."
until /bin/sh -c "mc config host add local http://$MINIO_SERVER_ENDPOINT $MINIO_ACCESS_KEY $MINIO_SECRET_KEY"; do
  echo "Waiting for MinIO server to become available..."
  sleep 3
done
mc mb local/$BUCKET_NAME --ignore-existing
echo "Bucket created."`,
							},
						},
					},
					RestartPolicy: "Never",
				},
			},
			BackoffLimit: pointer.Int32(4),
		},
	}
}

func PersistanceVolumeClaim() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "wandb",
			Name:      "wandb-minio-data",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
}

func Deployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "wandb",
			Name:      "minio",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "wandb-minio",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "wandb-minio",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "pvc-minio-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "wandb-minio-data",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "minio",
							Image: "minio/minio:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 9000},
								{ContainerPort: 9090},
							},
							Command: []string{"/bin/bash", "-c"},
							Args: []string{
								"minio server /data --console-address :9090",
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/minio/health/live",
										Port: intstr.FromInt(9000),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/minio/health/ready",
										Port: intstr.FromInt(9000),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "pvc-minio-data",
									MountPath: "/data",
								},
							},
						},
					},
				},
			},
		},
	}
}

func Service() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "wandb",
			Name:      "minio-service",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "wandb-minio",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Protocol:   "TCP",
					Port:       9000,
					TargetPort: intstr.FromInt(9000),
					NodePort:   int32(31300),
				},
				{
					Name:       "console",
					Protocol:   "TCP",
					Port:       9090,
					TargetPort: intstr.FromInt(9090),
					NodePort:   int32(31400),
				},
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
}

func GetHost(ctx context.Context, c client.Client) (string, error) {
	nodes := &corev1.NodeList{}
	c.List(ctx, nodes)
	if err := c.List(ctx, nodes); err != nil {
		return "", err
	}

	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeExternalIP {
				return address.Address, nil
			}
		}
	}

	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				return address.Address, nil
			}
		}
	}

	return "", fmt.Errorf("no nodes found")
}
