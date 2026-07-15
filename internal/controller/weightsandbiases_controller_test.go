package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/reconciler"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var manifestsDir, _ = filepath.Abs("../../hack/testing-manifests/server-manifest")
var manifestsRepository = fmt.Sprintf("file://%s", manifestsDir)

var _ = Describe("WeightsAndBiases Controller V2", func() {
	const (
		WandbName      = "test-wandb-v2"
		WandbNamespace = "default"
		timeout        = time.Second * 10
		interval       = time.Millisecond * 250
	)

	AfterEach(func() {
		// Cleanup
		wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: WandbName, Namespace: WandbNamespace}}
		wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
		dbSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      WandbName + "-db-password",
				Namespace: WandbNamespace,
			},
		}
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      WandbName + "-mysql-init",
				Namespace: WandbNamespace,
			},
		}
		// Applications are not owned by the WeightsAndBiases CR via OwnerReference
		// in the envtest setup, so garbage collection doesn't cascade. Without
		// this explicit pass, Applications from earlier It blocks leak into the
		// next test's namespace and break assertions that count them.
		appList := &apiv2.ApplicationList{}
		if err := k8sClient.List(ctx, appList, client.InNamespace(WandbNamespace)); err == nil {
			for i := range appList.Items {
				app := &appList.Items[i]
				if len(app.Finalizers) > 0 {
					app.SetFinalizers(nil)
					_ = k8sClient.Update(ctx, app)
				}
				_ = k8sClient.Delete(ctx, app, client.PropagationPolicy(metav1.DeletePropagationBackground))
			}
		}
		Expect(k8sClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))).Should(SatisfyAny(Succeed(), MatchError(ContainSubstring("not found"))))
		Expect(k8sClient.Delete(ctx, dbSecret)).Should(SatisfyAny(Succeed(), MatchError(ContainSubstring("not found"))))
		Expect(k8sClient.Delete(ctx, wandb)).Should(SatisfyAny(Succeed(), MatchError(ContainSubstring("not found"))))
		err := k8sClient.Get(ctx, wandbLookupKey, wandb)
		if !errors.IsNotFound(err) {
			wandb.SetFinalizers([]string{})
			Expect(k8sClient.Update(ctx, wandb)).Should(Succeed())
		}
	})

	Context("When reconciling a v2 WeightsAndBiases object", func() {
		It("Should successfully reconcile and update status", func() {
			By("Creating a new WeightsAndBiases v2 object")
			ctx := context.Background()
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.SizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname: "http://localhost",
						Features: map[string]bool{
							"proxy": true,
						},
						ManifestRepository: manifestsRepository,
						Version:            "0.83.0-clickhouse-keeper.2",
					},
					MySQL: map[string]apiv2.MySQLSpec{
						apiv2.DefaultInstanceName: {
							ManagedMysql: &apiv2.ManagedMysqlSpec{
								StorageSize: "10Gi",
							},
						},
					},
					Redis: map[string]apiv2.RedisSpec{
						apiv2.DefaultInstanceName: {
							ManagedRedis: &apiv2.ManagedRedisSpec{
								StorageSize: "10Gi",
							},
						},
					},
					Kafka: apiv2.KafkaSpec{
						ManagedKafka: &apiv2.ManagedKafkaSpec{
							StorageSize: "10Gi",
						},
					},
					ObjectStore: map[string]apiv2.ObjectStoreSpec{
						apiv2.DefaultInstanceName: {
							ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{
								StorageSize: "10Gi",
							},
						},
					},
					ClickHouse: map[string]apiv2.ClickHouseSpec{
						apiv2.DefaultInstanceName: {
							ManagedClickHouse: &apiv2.ManagedClickHouseSpec{},
						},
					},
				},
				Status: apiv2.WeightsAndBiasesStatus{},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: WandbName, Namespace: WandbNamespace}

			By("Running the reconciler")
			reconciler := &WeightsAndBiasesReconciler{
				Client:   k8sClient,
				Scheme:   scheme.Scheme,
				Recorder: record.NewFakeRecorder(10),
				EnableV2: true,
			}

			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: wandbLookupKey,
			})

			Expect(err).Should(Succeed())

			By("Checking if finalizers were added")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

			Expect(utils.ContainsString(wandb.GetFinalizers(), "wandb.apps.wandb.com/cleanup")).Should(BeTrue())
		})

		It("Should create a MySQL init job when deployment type is moco", func() {
			By("Creating a new WeightsAndBiases v2 object with MySQL deployment type 'moco'")
			ctx := context.Background()
			wandbName := "test-moco-init"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      wandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Wandb: apiv2.WandbAppSpec{
						Hostname:           "http://localhost",
						Features:           map[string]bool{},
						ManifestRepository: manifestsRepository,
						Version:            "0.83.0-clickhouse-keeper.2",
					},
					MySQL: map[string]apiv2.MySQLSpec{
						apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}},
					},
					Redis:       map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}},
					Kafka:       apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{}},
					ObjectStore: map[string]apiv2.ObjectStoreSpec{apiv2.DefaultInstanceName: {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}}},
					ClickHouse:  map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			By("Setting infra to ready")
			wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
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
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

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
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

			wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.MysqlConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}
			wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.ClickHouseConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}

			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			By("Checking if Applications were NOT created yet (migrations not complete)")
			wandbManifest, err := manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
			Expect(err).Should(Succeed())
			_, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())

			By("Checking if the MySQL init job was created")
			// The init job is now per managed MySQL instance, named after the
			// instance's resource name (default instance: "<cr>-mysql").
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: wandbName + "-mysql-moco-init", Namespace: WandbNamespace}, job)
			}, timeout, interval).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("moco-init"))
		})

		It("Should create application components when infrastructure is ready", func() {
			By("Creating a new WeightsAndBiases v2 object with ready infrastructure")
			ctx := context.Background()
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.SizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname:           "http://localhost",
						Features:           map[string]bool{},
						ManifestRepository: manifestsRepository,
						Version:            "0.83.0-clickhouse-keeper.2",
					},
					MySQL: map[string]apiv2.MySQLSpec{
						apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}},
					},
					Redis: map[string]apiv2.RedisSpec{
						apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}},
					},
					Kafka: apiv2.KafkaSpec{
						ManagedKafka: &apiv2.ManagedKafkaSpec{},
					},
					ObjectStore: map[string]apiv2.ObjectStoreSpec{
						apiv2.DefaultInstanceName: {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}},
					},
					ClickHouse: map[string]apiv2.ClickHouseSpec{
						apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

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
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

			wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.MysqlConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}
			wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.ClickHouseConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}

			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			By("Checking if Applications were NOT created yet (migrations not complete)")
			wandbManifest, err := manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
			Expect(err).Should(Succeed())
			ctrlResult, err := v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeNumerically(">", 0))

			appList := &apiv2.ApplicationList{}
			Expect(k8sClient.List(ctx, appList, client.InNamespace(WandbNamespace))).Should(Succeed())
			Expect(len(appList.Items)).Should(Equal(0))

			By("Setting migration status to successful")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			wandb.Status.Wandb.Migration.Version = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.LastSuccessVersion = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.Ready = true
			wandb.Status.Wandb.Migration.Reason = "Complete"
			wandb.Status.Wandb.MySQLInit = map[string]apiv2.MigrationJobStatus{apiv2.DefaultInstanceName: {Succeeded: true}}
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			// For now test by calling ReconcileWandbManifest directly, but this will get refactored into the reconciler later
			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeZero())

			By("Checking if ServiceAccount was created")
			saLookupKey := types.NamespacedName{Name: wandb.Spec.Wandb.ServiceAccount.ServiceAccountName, Namespace: WandbNamespace}
			createdSa := &v1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, saLookupKey, createdSa)).Should(Succeed())
			Expect(createdSa.Labels["app.kubernetes.io/instance"]).To(Equal(WandbName))

			By("Checking if Applications were created")
			appList = &apiv2.ApplicationList{}
			Expect(k8sClient.List(ctx, appList, client.InNamespace(WandbNamespace))).Should(Succeed())

			// The 0.76.1.yaml manifest should have some applications defined.
			// We expect them to be created as Application CRs.
			Expect(len(appList.Items)).Should(BeNumerically("==", len(wandbManifest.Applications)-1), "Expected all non-feature flagged applications to be created")
		})

		It("Should advance status.observedGeneration only once applications are reconciled for a generation", func() {
			By("Creating a new WeightsAndBiases v2 object at the initial version")
			ctx := context.Background()
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.SizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname:           "http://localhost",
						Features:           map[string]bool{},
						ManifestRepository: manifestsRepository,
						Version:            "0.83.0-clickhouse-keeper.1",
					},
					MySQL:       map[string]apiv2.MySQLSpec{apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}}},
					Redis:       map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}},
					Kafka:       apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{}},
					ObjectStore: map[string]apiv2.ObjectStoreSpec{apiv2.DefaultInstanceName: {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}}},
					ClickHouse:  map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			Expect(wandb.Status.ObservedGeneration).Should(BeZero())

			By("Marking infrastructure, mysql init, and migrations ready")
			wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			mysqlStatus := wandb.Status.MySQLStatus[apiv2.DefaultInstanceName]
			mysqlStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			wandb.Status.MySQLStatus[apiv2.DefaultInstanceName] = mysqlStatus
			clickHouseStatus := wandb.Status.ClickHouseStatus[apiv2.DefaultInstanceName]
			clickHouseStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			wandb.Status.ClickHouseStatus[apiv2.DefaultInstanceName] = clickHouseStatus
			wandb.Status.Wandb.Migration.Version = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.LastSuccessVersion = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.Ready = true
			wandb.Status.Wandb.Migration.Reason = "Complete"
			wandb.Status.Wandb.MySQLInit = map[string]apiv2.MigrationJobStatus{apiv2.DefaultInstanceName: {Succeeded: true}}
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			By("Reconciling the manifest to completion for the initial generation")
			wandbManifest, err := manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
			Expect(err).Should(Succeed())
			ctrlResult, err := v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeZero())

			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			initialGeneration := wandb.Generation
			Expect(initialGeneration).Should(BeNumerically(">", 0))
			Expect(wandb.Status.ObservedGeneration).Should(Equal(initialGeneration))

			By("Upgrading spec.wandb.version to bump the generation")
			wandb.Spec.Wandb.Version = "0.83.0-clickhouse-keeper.2"
			Expect(k8sClient.Update(ctx, wandb)).Should(Succeed())
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			Expect(wandb.Generation).Should(BeNumerically(">", initialGeneration))
			Expect(wandb.Status.ObservedGeneration).Should(Equal(initialGeneration))

			By("Reconciling while the new version's migration is still pending")
			wandbManifest, err = manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
			Expect(err).Should(Succeed())
			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeNumerically(">", 0))

			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			Expect(wandb.Status.ObservedGeneration).Should(Equal(initialGeneration),
				"observedGeneration must not advance before applications carry the new generation's spec")

			By("Completing the migration and reconciling to completion")
			wandb.Status.Wandb.Migration.Version = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.LastSuccessVersion = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.Ready = true
			wandb.Status.Wandb.Migration.Reason = "Complete"
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeZero())

			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			Expect(wandb.Status.ObservedGeneration).Should(Equal(wandb.Generation))
			Expect(wandb.Status.ObservedGeneration).Should(BeNumerically(">", initialGeneration))
		})

		It("Should clean up legacy v1 deployments once live Deployments are ready, even with a stale status map", func() {
			By("Creating a new WeightsAndBiases v2 object")
			ctx := context.Background()
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.SizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname:           "http://localhost",
						Features:           map[string]bool{},
						ManifestRepository: manifestsRepository,
						Version:            "0.83.0-clickhouse-keeper.1",
					},
					MySQL:       map[string]apiv2.MySQLSpec{apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}}},
					Redis:       map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}},
					Kafka:       apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{}},
					ObjectStore: map[string]apiv2.ObjectStoreSpec{apiv2.DefaultInstanceName: {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}}},
					ClickHouse:  map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())
			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}

			By("Creating a legacy v1 helm Deployment left over from the upgrade")
			legacy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: WandbName + "-app-bc", Namespace: WandbNamespace},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "legacy"}},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "legacy"}},
						Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "app", Image: "wandb/local:latest"}}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, legacy)).Should(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, legacy)
			})

			By("Marking infrastructure, mysql init, and migrations ready")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.MysqlConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}
			wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.ClickHouseConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}
			wandb.Status.Wandb.Migration.Version = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.LastSuccessVersion = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.Ready = true
			wandb.Status.Wandb.Migration.Reason = "Complete"
			wandb.Status.Wandb.MySQLInit = map[string]apiv2.MigrationJobStatus{apiv2.DefaultInstanceName: {Succeeded: true}}
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			By("Reconciling the manifest to create the Applications")
			wandbManifest, err := manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
			Expect(err).Should(Succeed())
			_, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())

			appList := &apiv2.ApplicationList{}
			Expect(k8sClient.List(ctx, appList, client.InNamespace(WandbNamespace))).Should(Succeed())
			Expect(appList.Items).ShouldNot(BeEmpty())

			By("Verifying Applications carry a WeightsAndBiases owner reference for the MatchEveryOwner watch")
			var ownerKinds []string
			for _, ref := range appList.Items[0].OwnerReferences {
				ownerKinds = append(ownerKinds, ref.Kind)
			}
			Expect(ownerKinds).To(ContainElement("WeightsAndBiases"),
				"the parent's Owns(Application, MatchEveryOwner) watch maps events through this owner ref")

			By("Verifying cleanup is deferred while application Deployments are absent")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: legacy.Name, Namespace: WandbNamespace}, &appsv1.Deployment{})).Should(Succeed(),
				"legacy deployment must survive until the estate is ready")

			By("Simulating the Application controller: rolled-out Deployments while the status map stays stale-false")
			for i := range appList.Items {
				app := appList.Items[i]
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: WandbNamespace},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": app.Name}},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": app.Name}},
							Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "app", Image: "wandb/local:latest"}}},
						},
					},
				}
				Expect(k8sClient.Create(ctx, dep)).Should(Succeed())
				DeferCleanup(func() {
					_ = k8sClient.Delete(ctx, dep)
				})
				dep.Status = appsv1.DeploymentStatus{
					ObservedGeneration: dep.Generation,
					Replicas:           1,
					ReadyReplicas:      1,
				}
				Expect(k8sClient.Status().Update(ctx, dep)).Should(Succeed())
			}

			By("Reconciling again: the gate must pass on live Deployments even though the status map says not-ready")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			_, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())

			err = k8sClient.Get(ctx, types.NamespacedName{Name: legacy.Name, Namespace: WandbNamespace}, &appsv1.Deployment{})
			Expect(errors.IsNotFound(err)).To(BeTrue(),
				"legacy -bc deployment must be deleted once live Deployments are ready")

			By("Verifying the status map refreshes from live Application status on the next pass")
			refreshed := &apiv2.Application{}
			appName := appList.Items[0].Name
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appName, Namespace: WandbNamespace}, refreshed)).Should(Succeed())
			refreshed.Status.Ready = true
			Expect(k8sClient.Status().Update(ctx, refreshed)).Should(Succeed())

			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			_, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			Expect(wandb.Status.Wandb.Applications[appName].Ready).To(BeTrue(),
				"the parent status map must reflect the Application's current status")
		})

		It("Should handle various migration states correctly", func() {
			By("Creating a new WeightsAndBiases v2 object")
			ctx := context.Background()
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.SizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname:           "http://localhost",
						Features:           map[string]bool{},
						ManifestRepository: manifestsRepository,
						Version:            "0.83.0-clickhouse-keeper.2",
					},
					MySQL:       map[string]apiv2.MySQLSpec{apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}}},
					Redis:       map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}},
					Kafka:       apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{}},
					ObjectStore: map[string]apiv2.ObjectStoreSpec{apiv2.DefaultInstanceName: {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}}},
					ClickHouse:  map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

			// Mark infra as ready
			wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			wandbManifest, err := manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
			Expect(err).Should(Succeed())

			By("Simulating migration in Running state")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			wandb.Status.Wandb.Migration.Version = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.Migration.Ready = false
			wandb.Status.Wandb.Migration.Reason = "Running"
			mysqlStatus := wandb.Status.MySQLStatus[apiv2.DefaultInstanceName]
			mysqlStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			wandb.Status.MySQLStatus[apiv2.DefaultInstanceName] = mysqlStatus
			clickHouseStatus := wandb.Status.ClickHouseStatus[apiv2.DefaultInstanceName]
			clickHouseStatus.Connection.URL = v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}
			wandb.Status.ClickHouseStatus[apiv2.DefaultInstanceName] = clickHouseStatus
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			ctrlResult, err := v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeNumerically(">", 0), "Expected requeue when migration is running")

			By("Simulating migration in Failed state")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			wandb.Status.Wandb.Migration.Ready = false
			wandb.Status.Wandb.Migration.Reason = "Failed"
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeNumerically(">", 0), "Expected requeue when migration failed")

			By("Simulating migration Complete")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			wandb.Status.Wandb.Migration.Ready = true
			wandb.Status.Wandb.Migration.Reason = "Complete"
			wandb.Status.Wandb.Migration.LastSuccessVersion = wandb.Spec.Wandb.Version
			wandb.Status.Wandb.MySQLInit = map[string]apiv2.MigrationJobStatus{apiv2.DefaultInstanceName: {Succeeded: true}}
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			ctrlResult, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())
			Expect(ctrlResult.RequeueAfter).Should(BeZero(), "Expected no requeue when migration is complete")
		})

		It("Should trigger new migrations on version upgrade", func() {
			By("Creating a new WeightsAndBiases v2 object with an old version")
			ctx := context.Background()
			oldVersion := "0.83.0-clickhouse-keeper.1"
			newVersion := "0.83.0-clickhouse-keeper.2"
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      WandbName,
					Namespace: WandbNamespace,
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: apiv2.SizeDev,
					Wandb: apiv2.WandbAppSpec{
						Hostname:           "http://localhost",
						Features:           map[string]bool{},
						ManifestRepository: manifestsRepository,
						Version:            oldVersion,
					},
					MySQL:       map[string]apiv2.MySQLSpec{apiv2.DefaultInstanceName: {ManagedMysql: &apiv2.ManagedMysqlSpec{}}},
					Redis:       map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {ManagedRedis: &apiv2.ManagedRedisSpec{}}},
					Kafka:       apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{}},
					ObjectStore: map[string]apiv2.ObjectStoreSpec{apiv2.DefaultInstanceName: {ManagedObjectStore: &apiv2.ManagedObjectStoreSpec{}}},
					ClickHouse:  map[string]apiv2.ClickHouseSpec{apiv2.DefaultInstanceName: {ManagedClickHouse: &apiv2.ManagedClickHouseSpec{}}},
				},
			}
			Expect(k8sClient.Create(ctx, wandb)).Should(Succeed())

			wandbLookupKey := types.NamespacedName{Name: wandb.Name, Namespace: wandb.Namespace}
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

			// Mark infra as ready and migration as complete for old version
			wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.MysqlConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}
			wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.KafkaStatus.Ready = true
			wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{apiv2.DefaultInstanceName: {WBInfraStatus: apiv2.WBInfraStatus{Ready: true}}}
			wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{apiv2.DefaultInstanceName: {
				WBInfraStatus: apiv2.WBInfraStatus{Ready: true},
				Connection:    apiv2.ClickHouseConnection{URL: v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: WandbName}, Key: "test"}},
			}}
			wandb.Status.Wandb.Migration.Version = oldVersion
			wandb.Status.Wandb.Migration.LastSuccessVersion = oldVersion
			wandb.Status.Wandb.Migration.Ready = true
			wandb.Status.Wandb.Migration.Reason = "Complete"
			wandb.Status.Wandb.MySQLInit = map[string]apiv2.MigrationJobStatus{apiv2.DefaultInstanceName: {Succeeded: true}}
			Expect(k8sClient.Status().Update(ctx, wandb)).Should(Succeed())

			By("Upgrading the version in the spec")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			wandb.Spec.Wandb.Version = newVersion
			Expect(k8sClient.Update(ctx, wandb)).Should(Succeed())

			By("Running ReconcileWandbManifest and verifying it triggers migrations")
			wandbManifest, err := manifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, newVersion)
			Expect(err).Should(Succeed())

			// Re-fetch to get updated Spec and Status
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())

			// This call to ReconcileWandbManifest should trigger runMigrations,
			// which sees version mismatch and starts migrations.
			_, err = v2.ReconcileWandbManifest(ctx, k8sClient, wandb, wandbManifest, v2.DefaultTelemetryRuntimeConfig())
			Expect(err).Should(Succeed())

			By("Verifying migration status was reset for the new version")
			Expect(k8sClient.Get(ctx, wandbLookupKey, wandb)).Should(Succeed())
			Expect(wandb.Status.Wandb.Migration.Version).Should(Equal(newVersion))
			Expect(wandb.Status.Wandb.Migration.Ready).Should(BeFalse())
			Expect(wandb.Status.Wandb.Migration.Reason).Should(Equal("Running"))
		})
	})
})
