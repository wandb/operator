package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/v2"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WeightsAndBiases Controller V2", func() {
	const (
		WandbName      = "test-wandb-v2"
		WandbNamespace = "default"
		timeout        = time.Second * 10
		duration       = time.Second * 10
		interval       = time.Millisecond * 250
	)

	Context("When reconciling a v2 WeightsAndBiases object", func() {
		It("Should successfully reconcile and update status", func() {
			By("Creating a new WeightsAndBiases v2 object")
			ctx := context.Background()
			manifest.Path = "../../hack/testing-manifests/server-manifest/0.76.1.yaml"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.WBSizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Features: map[string]bool{
							"proxy": true,
						},
					},
					MySQL: apiv2.WBMySQLSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
						StorageSize: "10Gi",
					},
					Redis: apiv2.WBRedisSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
						StorageSize: "10Gi",
					},
					Kafka: apiv2.WBKafkaSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
						StorageSize: "10Gi",
					},
					Minio: apiv2.WBMinioSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
						StorageSize: "10Gi",
					},
					ClickHouse: apiv2.WBClickHouseSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
					},
				},
				Status: apiv2.WeightsAndBiasesStatus{},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: WandbName, Namespace: WandbNamespace}
			createdWandb := &apiv2.WeightsAndBiases{}

			By("Running the reconciler")
			reconciler := &WeightsAndBiasesReconciler{
				Client:   k8sClient,
				Scheme:   scheme.Scheme,
				Recorder: record.NewFakeRecorder(10),
				EnableV2: true,
			}

			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: wandbLookupKey,
			})

			Expect(err).Should(Succeed())

			By("Checking if finalizers were added")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			Expect(utils.ContainsString(createdWandb.GetFinalizers(), "wandb.apps.wandb.com/cleanup")).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, createdWandb)).Should(Succeed())
		})

		It("Should create a MySQL init job when deployment type is mysql", func() {
			By("Creating a new WeightsAndBiases v2 object with MySQL deployment type 'mysql'")
			ctx := context.Background()
			manifest.Path = "../../hack/testing-manifests/server-manifest/0.76.1.yaml"
			wandbVersion := "0.76.1"
			wandbName := "test-mysql-init"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      wandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Features: map[string]bool{},
					},
					MySQL: apiv2.WBMySQLSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
						DeploymentType: apiv2.MySQLTypeMysql,
					},
					Redis:      apiv2.WBRedisSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Kafka:      apiv2.WBKafkaSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Minio:      apiv2.WBMinioSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					ClickHouse: apiv2.WBClickHouseSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			By("Setting infra to ready")
			wandb.Status.MySQLStatus.Ready = true
			wandb.Status.RedisStatus.Ready = true
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.MinioStatus.Ready = true
			wandb.Status.ClickHouseStatus.Ready = true
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			By("Creating the db-password secret")
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      wandbName + "-db-password",
					Namespace: WandbNamespace,
				},
				Data: map[string][]byte{
					"rootPassword": []byte("root-pass"),
					"password":     []byte("user-pass"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			createdWandb := &apiv2.WeightsAndBiases{}
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			By("Running the reconciler")
			reconciler := &WeightsAndBiasesReconciler{
				Client:   k8sClient,
				Scheme:   scheme.Scheme,
				Recorder: record.NewFakeRecorder(10),
				EnableV2: true,
			}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: wandbLookupKey,
			})
			Expect(err).Should(Succeed())

			By("Setting infrastructure status to ready")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			createdWandb.Status.MySQLStatus.Ready = true
			createdWandb.Status.RedisStatus.Ready = true
			createdWandb.Status.KafkaStatus.Ready = true
			createdWandb.Status.MinioStatus.Ready = true
			createdWandb.Status.ClickHouseStatus.Ready = true
			createdWandb.Status.MySQLStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			createdWandb.Status.ClickHouseStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}

			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			By("Checking if Applications were NOT created yet (migrations not complete)")
			wandbManifest, err := manifest.GetServerManifest(wandbVersion)
			Expect(err).Should(Succeed())
			_, err = v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())

			By("Checking if the MySQL init job was created")
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: wandbName + "-mysql-init", Namespace: WandbNamespace}, job)
			}, timeout, interval).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("mysql-init"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))).Should(Succeed())
			Expect(k8sClient.Delete(ctx, wandb)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
		})

		It("Should create application components when infrastructure is ready", func() {
			By("Creating a new WeightsAndBiases v2 object with ready infrastructure")
			ctx := context.Background()
			manifest.Path = "../../hack/testing-manifests/server-manifest/0.76.1.yaml"
			wandbVersion := "0.76.1"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName + "-ready",
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.WBSizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Version:  wandbVersion,
						Features: map[string]bool{},
					},
					MySQL: apiv2.WBMySQLSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
					},
					Redis: apiv2.WBRedisSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
					},
					Kafka: apiv2.WBKafkaSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
					},
					Minio: apiv2.WBMinioSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
					},
					ClickHouse: apiv2.WBClickHouseSpec{
						WBInfraSpec: apiv2.WBInfraSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			createdWandb := &apiv2.WeightsAndBiases{}
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			By("Running the reconciler")
			reconciler := &WeightsAndBiasesReconciler{
				Client:   k8sClient,
				Scheme:   scheme.Scheme,
				Recorder: record.NewFakeRecorder(10),
				EnableV2: true,
			}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: wandbLookupKey,
			})
			Expect(err).Should(Succeed())

			By("Setting infrastructure status to ready")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			createdWandb.Status.MySQLStatus.Ready = true
			createdWandb.Status.RedisStatus.Ready = true
			createdWandb.Status.KafkaStatus.Ready = true
			createdWandb.Status.MinioStatus.Ready = true
			createdWandb.Status.ClickHouseStatus.Ready = true
			createdWandb.Status.MySQLStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			createdWandb.Status.ClickHouseStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}

			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			By("Checking if Applications were NOT created yet (migrations not complete)")
			wandbManifest, err := manifest.GetServerManifest(wandbVersion)
			Expect(err).Should(Succeed())
			ctrlResult, err := v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeNumerically(">", 0))

			appList := &apiv2.ApplicationList{}
			Expect(k8sClient.List(ctx, appList, client.InNamespace(WandbNamespace))).Should(Succeed())
			Expect(len(appList.Items)).Should(Equal(0))

			By("Setting migration status to successful")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())
			createdWandb.Status.Wandb.Migration.Version = wandbVersion
			createdWandb.Status.Wandb.Migration.LastSuccessVersion = wandbVersion
			createdWandb.Status.Wandb.Migration.Ready = true
			createdWandb.Status.Wandb.Migration.Reason = "Complete"
			createdWandb.Status.Wandb.MySQLInit.Succeeded = true
			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			// For now test by calling ReconcileWandbManifest directly, but this will get refactored into the reconciler later
			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeZero())

			By("Checking if Applications were created")
			appList = &apiv2.ApplicationList{}
			Expect(k8sClient.List(ctx, appList, client.InNamespace(WandbNamespace))).Should(Succeed())

			// The 0.76.1.yaml manifest should have some applications defined.
			// We expect them to be created as Application CRs.
			Expect(len(appList.Items)).Should(BeNumerically("==", len(wandbManifest.Applications)-2), "Expected all non-feature flagged applications to be created")

			// Cleanup
			Expect(k8sClient.Delete(ctx, createdWandb)).Should(Succeed())
		})

		It("Should handle various migration states correctly", func() {
			By("Creating a new WeightsAndBiases v2 object")
			ctx := context.Background()
			manifest.Path = "../../hack/testing-manifests/server-manifest/0.76.1.yaml"
			wandbVersion := "0.76.1"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName + "-states",
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.WBSizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Version:  wandbVersion,
						Features: map[string]bool{},
					},
					MySQL:      apiv2.WBMySQLSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Redis:      apiv2.WBRedisSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Kafka:      apiv2.WBKafkaSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Minio:      apiv2.WBMinioSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					ClickHouse: apiv2.WBClickHouseSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			createdWandb := &apiv2.WeightsAndBiases{}
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			// Mark infra as ready
			createdWandb.Status.MySQLStatus.Ready = true
			createdWandb.Status.RedisStatus.Ready = true
			createdWandb.Status.KafkaStatus.Ready = true
			createdWandb.Status.MinioStatus.Ready = true
			createdWandb.Status.ClickHouseStatus.Ready = true
			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			wandbManifest, err := manifest.GetServerManifest(wandbVersion)
			Expect(err).Should(Succeed())

			By("Simulating migration in Running state")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())
			createdWandb.Status.Wandb.Migration.Version = wandbVersion
			createdWandb.Status.Wandb.Migration.Ready = false
			createdWandb.Status.Wandb.Migration.Reason = "Running"
			createdWandb.Status.MySQLStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			createdWandb.Status.ClickHouseStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			ctrlResult, err := v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeNumerically(">", 0), "Expected requeue when migration is running")

			By("Simulating migration in Failed state")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())
			createdWandb.Status.Wandb.Migration.Ready = false
			createdWandb.Status.Wandb.Migration.Reason = "Failed"
			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeNumerically(">", 0), "Expected requeue when migration failed")

			By("Simulating migration Complete")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())
			createdWandb.Status.Wandb.Migration.Ready = true
			createdWandb.Status.Wandb.Migration.Reason = "Complete"
			createdWandb.Status.Wandb.Migration.LastSuccessVersion = wandbVersion
			createdWandb.Status.Wandb.MySQLInit.Succeeded = true
			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeZero(), "Expected no requeue when migration is complete")

			// Cleanup
			Expect(k8sClient.Delete(ctx, createdWandb)).Should(Succeed())
		})

		It("Should trigger new migrations on version upgrade", func() {
			By("Creating a new WeightsAndBiases v2 object with an old version")
			ctx := context.Background()
			manifest.Path = "../../hack/testing-manifests/server-manifest/0.76.1.yaml"
			oldVersion := "0.76.0"
			newVersion := "0.76.1"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName + "-upgrade",
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.WBSizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Version:  oldVersion,
						Features: map[string]bool{},
					},
					MySQL:      apiv2.WBMySQLSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Redis:      apiv2.WBRedisSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Kafka:      apiv2.WBKafkaSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					Minio:      apiv2.WBMinioSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
					ClickHouse: apiv2.WBClickHouseSpec{WBInfraSpec: apiv2.WBInfraSpec{Enabled: true}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			createdWandb := &apiv2.WeightsAndBiases{}
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			// Mark infra as ready and migration as complete for old version
			createdWandb.Status.MySQLStatus.Ready = true
			createdWandb.Status.RedisStatus.Ready = true
			createdWandb.Status.KafkaStatus.Ready = true
			createdWandb.Status.MinioStatus.Ready = true
			createdWandb.Status.ClickHouseStatus.Ready = true
			createdWandb.Status.MySQLStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			createdWandb.Status.ClickHouseStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			createdWandb.Status.Wandb.Migration.Version = oldVersion
			createdWandb.Status.Wandb.Migration.LastSuccessVersion = oldVersion
			createdWandb.Status.Wandb.Migration.Ready = true
			createdWandb.Status.Wandb.Migration.Reason = "Complete"
			createdWandb.Status.Wandb.MySQLInit.Succeeded = true
			Expect(k8sClient.Status().Update(ctx, createdWandb)).Should(Succeed())

			By("Upgrading the version in the spec")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())
			createdWandb.Spec.Wandb.Version = newVersion
			Expect(k8sClient.Update(ctx, createdWandb)).Should(Succeed())

			By("Running ReconcileWandbManifest and verifying it triggers migrations")
			wandbManifest, err := manifest.GetServerManifest(newVersion)
			Expect(err).Should(Succeed())

			// Re-fetch to get updated Spec and Status
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())

			// This call to ReconcileWandbManifest should trigger runMigrations,
			// which sees version mismatch and starts migrations.
			_, err = v2.ReconcileWandbManifest(ctx, k8sClient, createdWandb, wandbManifest)
			Expect(err).Should(Succeed())

			By("Verifying migration status was reset for the new version")
			Expect(k8sClient.Get(ctx, wandbLookupKey, createdWandb)).Should(Succeed())
			Expect(createdWandb.Status.Wandb.Migration.Version).Should(Equal(newVersion))
			Expect(createdWandb.Status.Wandb.Migration.Ready).Should(BeFalse())
			Expect(createdWandb.Status.Wandb.Migration.Reason).Should(Equal("Running"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, createdWandb)).Should(Succeed())
		})
	})
})
