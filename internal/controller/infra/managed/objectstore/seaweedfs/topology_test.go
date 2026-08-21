package seaweedfs

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("SeaweedFS topology migration", func() {
	const (
		namespace = "wandb"
		name      = "wandb-seaweedfs"
	)

	owner := func() *apiv2.WeightsAndBiases {
		return &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: namespace},
		}
	}
	seaweed := func(replicas, ready int32, replication, stable string) *seaweedv1.Seaweed {
		annotations := map[string]string{}
		if stable != "" {
			annotations[stableReplicationAnnotation] = stable
		}
		return &seaweedv1.Seaweed{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: annotations},
			Spec: seaweedv1.SeaweedSpec{
				Image: "chrislusf/seaweedfs:4.35",
				Master: &seaweedv1.MasterSpec{
					Replicas:           1,
					DefaultReplication: ptr.To(replication),
				},
				Volume: &seaweedv1.VolumeSpec{Replicas: replicas},
				Filer:  &seaweedv1.FilerSpec{Replicas: 1},
				S3:     &seaweedv1.S3GatewaySpec{Replicas: 1},
			},
			Status: seaweedv1.SeaweedStatus{
				Master: seaweedv1.ComponentStatus{Replicas: 1, ReadyReplicas: 1},
				Volume: seaweedv1.ComponentStatus{Replicas: replicas, ReadyReplicas: ready},
				Filer:  seaweedv1.ComponentStatus{Replicas: 1, ReadyReplicas: 1},
				S3:     seaweedv1.ComponentStatus{Replicas: 1, ReadyReplicas: 1},
			},
		}
	}
	readyWorkloads := func(volumeReplicas int32) []client.Object {
		statefulSet := func(suffix string, replicas int32) *appsv1.StatefulSet {
			return &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name + "-" + suffix,
					Namespace:  namespace,
					Generation: 1,
				},
				Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(replicas)},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 1,
					Replicas:           replicas,
					CurrentReplicas:    replicas,
					UpdatedReplicas:    replicas,
					ReadyReplicas:      replicas,
					CurrentRevision:    "ready",
					UpdateRevision:     "ready",
				},
			}
		}
		return []client.Object{
			statefulSet("master", 1),
			statefulSet("volume", volumeReplicas),
			statefulSet("filer", 1),
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: name + "-s3", Namespace: namespace, Generation: 1},
				Spec:       appsv1.DeploymentSpec{Replicas: ptr.To[int32](1)},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: 1,
					Replicas:           1,
					UpdatedReplicas:    1,
					ReadyReplicas:      1,
					AvailableReplicas:  1,
				},
			},
		}
	}
	completedJob := func(stage, target string) *batchv1.Job {
		job := topologyJob(
			seaweed(3, 3, target, "000"),
			topologyJobName(name, stage, target),
			stage,
			target,
			"true",
		)
		job.Status.Conditions = []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}}
		return job
	}
	newClient := func(objects ...client.Object) client.Client {
		return fake.NewClientBuilder().
			WithScheme(writeScheme()).
			WithObjects(objects...).
			Build()
	}

	It("initializes a new cluster directly at its requested topology", func() {
		desired := seaweed(3, 0, "001", "")
		conditions, err := reconcileTopology(context.Background(), newClient(), desired, nil, owner())

		Expect(err).NotTo(HaveOccurred())
		Expect(conditions).To(ContainElement(SatisfyAll(
			HaveField("Type", SeaweedTopologyReadyType),
			HaveField("Status", metav1.ConditionTrue),
		)))
		Expect(desired.Annotations).To(HaveKeyWithValue(stableReplicationAnnotation, "001"))
	})

	It("records a healthy legacy cluster before changing its topology", func() {
		actual := seaweed(1, 1, "000", "")
		desired := seaweed(3, 0, "001", "")
		cl := newClient(readyWorkloads(1)...)

		conditions, err := reconcileTopology(context.Background(), cl, desired, actual, owner())

		Expect(err).NotTo(HaveOccurred())
		Expect(conditions[0].Reason).To(Equal("TopologyAdopted"))
		Expect(desired.Annotations).To(HaveKeyWithValue(stableReplicationAnnotation, "000"))
		Expect(defaultReplication(desired)).To(Equal("000"))
		Expect(desired.Spec.Volume.Replicas).To(Equal(int32(1)))
		jobs := &batchv1.JobList{}
		Expect(cl.List(context.Background(), jobs)).To(Succeed())
		Expect(jobs.Items).To(BeEmpty())
	})

	It("adds volume servers without restarting the master into the new policy", func() {
		actual := seaweed(1, 1, "000", "000")
		desired := seaweed(3, 0, "001", "")

		conditions, err := reconcileTopology(context.Background(), newClient(), desired, actual, owner())

		Expect(err).NotTo(HaveOccurred())
		Expect(defaultReplication(desired)).To(Equal("000"))
		Expect(desired.Spec.Volume.Replicas).To(Equal(int32(3)))
		Expect(conditions[0].Reason).To(Equal("ScalingVolumeServers"))
	})

	It("verifies data before changing the replication policy", func() {
		actual := seaweed(3, 3, "000", "000")
		desired := seaweed(3, 0, "001", "")
		cl := newClient(readyWorkloads(3)...)

		conditions, err := reconcileTopology(context.Background(), cl, desired, actual, owner())

		Expect(err).NotTo(HaveOccurred())
		Expect(defaultReplication(desired)).To(Equal("000"))
		Expect(conditions[0].Reason).To(Equal("PreMigrationVerificationRunning"))
		jobs := &batchv1.JobList{}
		Expect(cl.List(context.Background(), jobs)).To(Succeed())
		Expect(jobs.Items).To(HaveLen(1))
		Expect(jobs.Items[0].Spec.Template.Spec.Containers[0].Args[0]).To(ContainSubstring("fs.verify"))
	})

	It("waits for the volume StatefulSet rollout before verifying data", func() {
		actual := seaweed(3, 3, "000", "000")
		desired := seaweed(3, 0, "001", "")
		workloads := readyWorkloads(3)
		volume := workloads[1].(*appsv1.StatefulSet)
		volume.Status.ReadyReplicas = 2
		cl := newClient(workloads...)

		conditions, err := reconcileTopology(context.Background(), cl, desired, actual, owner())

		Expect(err).NotTo(HaveOccurred())
		Expect(conditions[0].Reason).To(Equal("WaitingForSeaweedComponents"))
		jobs := &batchv1.JobList{}
		Expect(cl.List(context.Background(), jobs)).To(Succeed())
		Expect(jobs.Items).To(BeEmpty())
	})

	It("changes the master policy only after pre-verification succeeds", func() {
		actual := seaweed(3, 3, "000", "000")
		desired := seaweed(3, 0, "001", "")
		preVerify := completedJob(topologyJobPreVerify, "001")

		conditions, err := reconcileTopology(
			context.Background(),
			newClient(append(readyWorkloads(3), preVerify)...),
			desired,
			actual,
			owner(),
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(defaultReplication(desired)).To(Equal("001"))
		Expect(conditions[0].Reason).To(Equal("UpdatingReplicationPolicy"))
	})

	It("rolls the S3 gateway after replication and before post-verification", func() {
		actual := seaweed(3, 3, "001", "000")
		desired := seaweed(3, 0, "001", "")
		preVerify := completedJob(topologyJobPreVerify, "001")
		replicate := completedJob(topologyJobReplicate, "001")
		workloads := readyWorkloads(3)
		cl := newClient(append(workloads, preVerify, replicate)...)

		conditions, err := reconcileTopology(
			context.Background(),
			cl,
			desired,
			actual,
			owner(),
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(conditions[0].Reason).To(Equal("RestartingS3Gateway"))
		s3 := &appsv1.Deployment{}
		Expect(cl.Get(context.Background(), client.ObjectKey{Name: name + "-s3", Namespace: namespace}, s3)).To(Succeed())
		Expect(s3.Spec.Template.Annotations).To(HaveKeyWithValue(s3RolloutAnnotation, "001"))
	})

	It("replicates old volumes and verifies every filer chunk before completing", func() {
		actual := seaweed(3, 3, "001", "000")
		desired := seaweed(3, 0, "001", "")
		preVerify := completedJob(topologyJobPreVerify, "001")
		replicate := completedJob(topologyJobReplicate, "001")
		postVerify := completedJob(topologyJobPostVerify, "001")
		workloads := readyWorkloads(3)
		workloads[3].(*appsv1.Deployment).Spec.Template.Annotations = map[string]string{
			s3RolloutAnnotation: "001",
		}

		conditions, err := reconcileTopology(
			context.Background(),
			newClient(append(workloads, preVerify, replicate, postVerify)...),
			desired,
			actual,
			owner(),
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(conditions[0].Reason).To(Equal("TopologyMigrated"))
		Expect(desired.Annotations).To(HaveKeyWithValue(stableReplicationAnnotation, "001"))
	})

	It("waits for the S3 rollout generation before post-verification", func() {
		actual := seaweed(3, 3, "001", "000")
		desired := seaweed(3, 0, "001", "")
		preVerify := completedJob(topologyJobPreVerify, "001")
		replicate := completedJob(topologyJobReplicate, "001")
		workloads := readyWorkloads(3)
		s3 := workloads[3].(*appsv1.Deployment)
		s3.Spec.Template.Annotations = map[string]string{s3RolloutAnnotation: "001"}
		s3.Generation = 2
		s3.Status.ObservedGeneration = 1
		cl := newClient(append(workloads, preVerify, replicate)...)

		conditions, err := reconcileTopology(context.Background(), cl, desired, actual, owner())

		Expect(err).NotTo(HaveOccurred())
		Expect(conditions[0].Reason).To(Equal("WaitingForSeaweedComponents"))
		jobs := &batchv1.JobList{}
		Expect(cl.List(context.Background(), jobs)).To(Succeed())
		Expect(jobs.Items).To(HaveLen(2))
	})
})
