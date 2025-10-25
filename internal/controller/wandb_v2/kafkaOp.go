package wandb_v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	strimziv1beta2 "github.com/wandb/operator/api/strimzi/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const kafkaFinalizer = "kafka.app.wandb.com"

type wandbKafkaWrapper struct {
	kafkaInstalled    bool
	nodePoolInstalled bool
	kafkaObj          *strimziv1beta2.Kafka
	nodePoolObj       *strimziv1beta2.KafkaNodePool
	secretInstalled   bool
	secret            *corev1.Secret
}

func (w *wandbKafkaWrapper) IsReady() bool {
	if !w.kafkaInstalled || w.kafkaObj == nil || w.nodePoolObj == nil {
		return false
	}

	kafkaReady := false
	nodePoolReady := false

	for _, condition := range w.kafkaObj.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			kafkaReady = true
			break
		}
	}

	// Check if NodePool has Ready condition set to True
	for _, condition := range w.nodePoolObj.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			nodePoolReady = true
			break
		}
	}

	// If NodePool has no conditions, consider it ready if it has replicas and observed generation matches
	if !nodePoolReady && len(w.nodePoolObj.Status.Conditions) == 0 {
		if w.nodePoolObj.Status.Replicas > 0 && w.nodePoolObj.Status.ObservedGeneration > 0 {
			nodePoolReady = true
		}
	}

	return kafkaReady && nodePoolReady
}

func (w *wandbKafkaWrapper) GetStatus() string {
	if !w.kafkaInstalled || w.kafkaObj == nil || w.nodePoolObj == nil {
		return "NotInstalled"
	}

	kafkaReady := false
	var kafkaReason string

	for _, condition := range w.kafkaObj.Status.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == metav1.ConditionTrue {
				kafkaReady = true
			} else {
				kafkaReason = condition.Reason
			}
			break
		}
	}

	if kafkaReady {
		return "ready"
	}

	if !kafkaReady && kafkaReason != "" {
		return "Kafka:" + kafkaReason
	}

	return "pending"
}

type wandbKafkaDoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState
}

func kafkaNamespacedName(req ctrl.Request) types.NamespacedName {
	return types.NamespacedName{
		Name:      "wandb-kafka",
		Namespace: req.Namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleKafka(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredKafka wandbKafkaWrapper
	var actualKafka wandbKafkaWrapper
	var reconciliation wandbKafkaDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := kafkaNamespacedName(req)

	if !wandb.Spec.Kafka.Enabled {
		log.Info("Kafka not enabled, skipping")
		return CtrlContinue()
	}

	log.Info("Handling Kafka")

	if actualKafka, err = getActualKafka(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual Kafka resources")
		return CtrlError(err)
	}

	if ctrlState := actualKafka.maybeHandleDeletion(ctx, wandb, actualKafka, r); ctrlState.shouldExit(HandlerScope) {
		return ctrlState
	}

	if desiredKafka, err = getDesiredKafka(ctx, wandb, namespacedName, actualKafka); err != nil {
		log.Error(err, "Failed to get desired Kafka configuration")
		return CtrlError(err)
	}

	if reconciliation, err = computeKafkaReconcileDrift(ctx, wandb, desiredKafka, actualKafka, r); err != nil {
		log.Error(err, "Failed to compute Kafka reconcile drift")
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func getActualKafka(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbKafkaWrapper, error,
) {
	result := wandbKafkaWrapper{
		kafkaInstalled:    false,
		nodePoolInstalled: false,
		kafkaObj:          nil,
		nodePoolObj:       nil,
		secretInstalled:   false,
		secret:            nil,
	}

	obj := &strimziv1beta2.Kafka{}
	err := reconciler.Get(ctx, namespacedName, obj)
	if err == nil {
		result.kafkaObj = obj
		result.kafkaInstalled = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	nodePoolNamespacedName := types.NamespacedName{
		Name:      "wandb-kafka-pool",
		Namespace: namespacedName.Namespace,
	}
	nodePoolObj := &strimziv1beta2.KafkaNodePool{}
	err = reconciler.Get(ctx, nodePoolNamespacedName, nodePoolObj)
	if err == nil {
		result.nodePoolObj = nodePoolObj
		result.nodePoolInstalled = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	secretNamespacedName := types.NamespacedName{
		Name:      "wandb-kafka-connection",
		Namespace: namespacedName.Namespace,
	}
	secret := &corev1.Secret{}
	err = reconciler.Get(ctx, secretNamespacedName, secret)
	if err == nil {
		result.secret = secret
		result.secretInstalled = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	return result, nil
}

func getDesiredKafka(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbKafkaWrapper,
) (
	wandbKafkaWrapper, error,
) {
	result := wandbKafkaWrapper{
		kafkaInstalled:    false,
		kafkaObj:          nil,
		nodePoolObj:       nil,
		secretInstalled:   false,
		secret:            nil,
		nodePoolInstalled: false,
	}

	if !wandb.Spec.Kafka.Enabled {
		return result, nil
	}

	result.kafkaInstalled = true
	result.nodePoolInstalled = true

	storageSize := wandb.Spec.Kafka.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	//storageQuantity, err := resource.ParseQuantity(storageSize)
	//if err != nil {
	//	return result, errors.New("invalid storage size: " + storageSize)
	//}

	replicas := wandb.Spec.Kafka.Replicas
	if replicas == 0 {
		replicas = 1
	}

	kafka := &strimziv1beta2.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"app": "wandb-kafka",
			},
			Annotations: map[string]string{
				"strimzi.io/node-pools": "enabled",
			},
		},
		Spec: strimziv1beta2.KafkaSpec{
			Kafka: strimziv1beta2.KafkaClusterSpec{
				Version:         "4.1.0",
				MetadataVersion: "4.1-IV0",
				Replicas:        0, // critical when in KRaft mode
				Listeners: []strimziv1beta2.GenericKafkaListener{
					{
						Name: "plain",
						Port: 9092,
						Type: "internal",
						Tls:  false,
					},
					{
						Name: "tls",
						Port: 9093,
						Type: "internal",
						Tls:  true,
					},
				},
				Config: map[string]string{
					"offsets.topic.replication.factor":         "1",
					"transaction.state.log.replication.factor": "1",
					"transaction.state.log.min.isr":            "1",
					"default.replication.factor":               "1",
					"min.insync.replicas":                      "1",
				},
			},
		},
	}

	nodePool := &strimziv1beta2.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-kafka-pool",
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"strimzi.io/cluster": namespacedName.Name,
			},
		},
		Spec: strimziv1beta2.KafkaNodePoolSpec{
			Replicas: replicas,
			Roles:    []string{"broker", "controller"},
			Storage: strimziv1beta2.KafkaStorage{
				Type: "jbod",
				Volumes: []strimziv1beta2.StorageVolume{
					{
						ID:          0,
						Type:        "persistent-claim",
						Size:        storageSize,
						DeleteClaim: true,
					},
				},
			},
		},
	}

	wandbBackupSpec := wandb.Spec.Kafka.Backup
	if wandbBackupSpec.Enabled {
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for Kafka")
		}
	}

	result.kafkaObj = kafka
	result.nodePoolObj = nodePool

	if actual.IsReady() && actual.kafkaObj != nil && len(actual.kafkaObj.Status.Listeners) > 0 {
		var bootstrapServers string
		for _, listener := range actual.kafkaObj.Status.Listeners {
			if listener.Name == "plain" {
				bootstrapServers = listener.BootstrapServers
				break
			}
		}

		if bootstrapServers != "" {
			namespace := namespacedName.Namespace
			connectionSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wandb-kafka-connection",
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"KAFKA_BOOTSTRAP_SERVERS": bootstrapServers,
				},
			}

			result.secret = connectionSecret
			result.secretInstalled = true
		}
	}

	return result, nil
}

func computeKafkaReconcileDrift(
	_ context.Context, wandb *apiv2.WeightsAndBiases, desiredKafka, actualKafka wandbKafkaWrapper, _ client.Reader,
) (
	wandbKafkaDoReconcile, error,
) {
	desiredNodePoolInstalled := desiredKafka.kafkaInstalled && desiredKafka.nodePoolObj != nil
	actualNodePoolInstalled := actualKafka.nodePoolObj != nil

	// Check NodePool first (must exist before Kafka)
	if !desiredNodePoolInstalled && actualNodePoolInstalled {
		return &wandbNodePoolDelete{
			actual: actualKafka,
			wandb:  wandb,
		}, nil
	}
	if desiredNodePoolInstalled && !actualNodePoolInstalled {
		return &wandbNodePoolCreate{
			desired: desiredKafka,
			wandb:   wandb,
		}, nil
	}

	// Check Kafka CR (requires NodePool to exist)
	if !desiredKafka.kafkaInstalled && actualKafka.kafkaInstalled {
		if actualKafka.secretInstalled {
			return &wandbKafkaConnInfoDelete{
				wandb: wandb,
			}, nil
		}
		return &wandbKafkaDelete{
			actual: actualKafka,
			wandb:  wandb,
		}, nil
	}
	if desiredKafka.kafkaInstalled && !actualKafka.kafkaInstalled {
		return &wandbKafkaCreate{
			desired: desiredKafka,
			wandb:   wandb,
		}, nil
	}

	if desiredKafka.secretInstalled && !actualKafka.secretInstalled {
		return &wandbKafkaConnInfoCreate{
			desired: desiredKafka,
			wandb:   wandb,
		}, nil
	}

	// Update status if both resources exist but status differs
	if actualKafka.kafkaInstalled && actualNodePoolInstalled {
		if actualKafka.GetStatus() != wandb.Status.KafkaStatus.State ||
			actualKafka.IsReady() != wandb.Status.KafkaStatus.Ready {
			return &wandbKafkaStatusUpdate{
				wandb:  wandb,
				status: actualKafka.GetStatus(),
				ready:  actualKafka.IsReady(),
			}, nil
		}
	}

	return nil, nil
}

type wandbNodePoolCreate struct {
	desired wandbKafkaWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbNodePoolCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Kafka NodePool")
	wandb := c.wandb

	if err = controllerutil.SetOwnerReference(wandb, c.desired.nodePoolObj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Kafka NodePool")
		return CtrlError(err)
	}

	if err = r.Create(ctx, c.desired.nodePoolObj); err != nil {
		log.Error(err, "Failed to create Kafka NodePool")
		return CtrlError(err)
	}

	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating Kafka NodePool"
	wandb.Status.KafkaStatus.State = "pending"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating Kafka NodePool")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbNodePoolDelete struct {
	actual wandbKafkaWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbNodePoolDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Kafka NodePool")

	if err = r.Delete(ctx, d.actual.nodePoolObj); err != nil {
		log.Error(err, "Failed to delete Kafka NodePool")
		return CtrlError(err)
	}

	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting Kafka NodePool"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting Kafka NodePool")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbKafkaCreate struct {
	desired wandbKafkaWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbKafkaCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Kafka")
	wandb := c.wandb

	if err = controllerutil.SetOwnerReference(wandb, c.desired.kafkaObj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Kafka")
		return CtrlError(err)
	}

	if err = r.Create(ctx, c.desired.kafkaObj); err != nil {
		log.Error(err, "Failed to create Kafka")
		return CtrlError(err)
	}

	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating Kafka"
	wandb.Status.KafkaStatus.State = "pending"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating Kafka")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbKafkaDelete struct {
	actual wandbKafkaWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbKafkaDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Kafka")

	if err = r.Delete(ctx, d.actual.kafkaObj); err != nil {
		log.Error(err, "Failed to delete Kafka")
		return CtrlError(err)
	}

	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting Kafka"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting Kafka")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbKafkaStatusUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbKafkaStatusUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating Kafka status", "status", s.status, "ready", s.ready)
	s.wandb.Status.KafkaStatus.State = s.status
	s.wandb.Status.KafkaStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		log.Error(err, "Failed to update Kafka status")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

func (w *wandbKafkaWrapper) maybeHandleDeletion(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, actualKafka wandbKafkaWrapper, reconciler *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrllog.FromContext(ctx)

	//requeueSeconds := wandb.Status.KafkaStatus.BackupStatus.RequeueAfter
	//if requeueSeconds == 0 {
	//	requeueSeconds = 30
	//}
	//requeueDuration := time.Duration(requeueSeconds) * time.Second

	var deletionPaused = wandb.Status.State == apiv2.WBStateDeletionPaused
	var backupEnabled = wandb.Spec.Kafka.Backup.Enabled
	var flaggedForDeletion = !wandb.ObjectMeta.DeletionTimestamp.IsZero()
	var hasKafkaFinalizer = ctrlqueue.ContainsString(wandb.GetFinalizers(), kafkaFinalizer)

	if flaggedForDeletion && !backupEnabled {
		log.Info("Kafka backup is disabled.")
		controllerutil.RemoveFinalizer(wandb, kafkaFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove Kafka finalizer")
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if deletionPaused && backupEnabled {
		log.Info("Deletion paused for Kafka Backup; disable backups to continue with deletion")
		return CtrlContinue()
	}

	if !hasKafkaFinalizer && !flaggedForDeletion {
		wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, kafkaFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to add Kafka finalizer")
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if flaggedForDeletion {
		if err := w.handleKafkaBackup(ctx, wandb, reconciler); err != nil {
			log.Info("Failed to backup Kafka, pausing deletion")
			wandb.ObjectMeta.DeletionTimestamp = nil
			if err = reconciler.Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update WeightsAndBiases during backup failure")
				return CtrlError(err)
			}
			wandb.Status.State = apiv2.WBStateDeletionPaused
			wandb.Status.Message = "Kafka backup before deletion failed, deletion paused. Disable backups to continue with deletion."
			if err = reconciler.Status().Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update status to deletion paused")
				return CtrlError(err)
			}
			return CtrlDone(HandlerScope)
		}

		if wandb.Status.KafkaStatus.BackupStatus.State == "InProgress" {
			log.Info("Backup in progress, requeuing", "backup", wandb.Status.KafkaStatus.BackupStatus.BackupName)
			if wandb.Status.State != apiv2.WBStateDeleting {
				wandb.Status.State = apiv2.WBStateDeleting
				wandb.Status.KafkaStatus.State = "stopping"
				wandb.Status.Message = "Waiting for Kafka backup to complete before deletion"
				if err := reconciler.Status().Update(ctx, wandb); err != nil {
					log.Error(err, "Failed to update status while backup in progress")
					return CtrlError(err)
				}
			}
			return CtrlDone(HandlerScope)
		}

		controllerutil.RemoveFinalizer(wandb, kafkaFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove Kafka finalizer after backup")
			return CtrlError(err)
		}

		if actualKafka.kafkaObj != nil {
			if err := reconciler.Client.Delete(ctx, actualKafka.kafkaObj); err != nil {
				log.Error(err, "Failed to delete Kafka resource during cleanup")
				return CtrlError(err)
			}
		}

		if actualKafka.nodePoolObj != nil {
			if err := reconciler.Client.Delete(ctx, actualKafka.nodePoolObj); err != nil {
				log.Error(err, "Failed to delete Kafka NodePool during cleanup")
				return CtrlError(err)
			}
		}

		return CtrlDone(HandlerScope)
	}
	return CtrlContinue()
}

func (w *wandbKafkaWrapper) handleKafkaBackup(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) error {
	log := ctrl.LoggerFrom(ctx)

	if w.kafkaObj == nil {
		log.Info("Kafka object is nil, skipping backup")
		return nil
	}

	if !wandb.Spec.Kafka.Enabled {
		log.Info("Kafka not enabled, skipping backup")
		return nil
	}

	if !wandb.Spec.Kafka.Backup.Enabled {
		log.Info("Kafka backup not enabled, skipping backup")
		return nil
	}

	log.Info("Executing Kafka backup before deletion")

	storageName := wandb.Spec.Kafka.Backup.StorageName
	if storageName == "" {
		storageName = "default-backup"
	}

	executor := &KafkaBackupExecutor{
		Client:         reconciler.Client,
		ClusterName:    w.kafkaObj.GetName(),
		Namespace:      w.kafkaObj.Namespace,
		StorageName:    storageName,
		TimeoutSeconds: wandb.Spec.Kafka.Backup.TimeoutSeconds,
	}

	currentBackupState := &BackupState{
		BackupName:  wandb.Status.KafkaStatus.BackupStatus.BackupName,
		StartedAt:   wandb.Status.KafkaStatus.BackupStatus.StartedAt,
		CompletedAt: wandb.Status.KafkaStatus.BackupStatus.CompletedAt,
		State:       wandb.Status.KafkaStatus.BackupStatus.State,
	}

	newState, result, err := executor.EnsureKafkaBackup(ctx, currentBackupState)

	if newState != nil {
		wandb.Status.KafkaStatus.BackupStatus = apiv2.WBBackupStatus{
			BackupName:     newState.BackupName,
			StartedAt:      newState.StartedAt,
			CompletedAt:    newState.CompletedAt,
			LastBackupTime: newState.CompletedAt,
			State:          newState.State,
			Message:        result.Message,
		}

		if result.InProgress {
			wandb.Status.KafkaStatus.BackupStatus.State = "InProgress"
			wandb.Status.KafkaStatus.BackupStatus.RequeueAfter = int64(result.RequeueAfter.Seconds())
		}

		if statusErr := reconciler.Client.Status().Update(ctx, wandb); statusErr != nil {
			log.Error(statusErr, "Failed to update backup status")
		}
	}

	if err != nil && !result.InProgress {
		reconciler.updateKafkaBackupStatus(ctx, wandb, "Failed", err.Error())
		return err
	}

	if result.Completed {
		return nil
	}

	if result.InProgress {
		return nil
	}

	return err
}

func (r *WeightsAndBiasesV2Reconciler) updateKafkaBackupStatus(ctx context.Context, wandb *apiv2.WeightsAndBiases, state, message string) {
	log := ctrl.LoggerFrom(ctx)
	now := metav1.Now()

	wandb.Status.KafkaStatus.BackupStatus = apiv2.WBBackupStatus{
		LastBackupTime: &now,
		State:          state,
		Message:        message,
	}

	if err := r.Client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update Kafka backup status")
	}
}

type KafkaBackupExecutor struct {
	Client         client.Client
	ClusterName    string
	Namespace      string
	StorageName    string
	TimeoutSeconds int
}

func (k *KafkaBackupExecutor) EnsureKafkaBackup(ctx context.Context, currentState *BackupState) (*BackupState, BackupResult, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Kafka backup requested", "cluster", k.ClusterName, "storage", k.StorageName)

	backupName := fmt.Sprintf("%s-backup-%d", k.ClusterName, time.Now().Unix())
	now := metav1.Now()

	state := &BackupState{
		BackupName:  backupName,
		StartedAt:   &now,
		CompletedAt: &now,
		State:       "completed",
	}

	result := BackupResult{
		Completed:    true,
		InProgress:   false,
		Failed:       false,
		Message:      "Kafka backup completed (placeholder implementation)",
		RequeueAfter: 0,
	}

	return state, result, nil
}

type wandbKafkaConnInfoCreate struct {
	desired wandbKafkaWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbKafkaConnInfoCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Creating Kafka connection secret")

	if c.desired.secret == nil {
		log.Error(nil, "Desired secret is nil")
		return CtrlError(errors.New("desired secret is nil"))
	}

	if err := controllerutil.SetOwnerReference(c.wandb, c.desired.secret, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for Kafka connection secret")
		return CtrlError(err)
	}

	if err := r.Create(ctx, c.desired.secret); err != nil {
		log.Error(err, "Failed to create Kafka connection secret")
		return CtrlError(err)
	}

	log.Info("Kafka connection secret created successfully")
	return CtrlDone(HandlerScope)
}

type wandbKafkaConnInfoDelete struct {
	wandb *apiv2.WeightsAndBiases
}

func (d *wandbKafkaConnInfoDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Deleting Kafka connection secret")

	namespacedName := types.NamespacedName{
		Name:      "wandb-kafka-connection",
		Namespace: d.wandb.Namespace,
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, namespacedName, secret)
	if err != nil {
		if machErrors.IsNotFound(err) {
			log.Info("Kafka connection secret already deleted")
			return CtrlContinue()
		}
		log.Error(err, "Failed to get Kafka connection secret for deletion")
		return CtrlError(err)
	}

	if err := r.Delete(ctx, secret); err != nil {
		log.Error(err, "Failed to delete Kafka connection secret")
		return CtrlError(err)
	}

	log.Info("Kafka connection secret deleted successfully")
	return CtrlDone(HandlerScope)
}
