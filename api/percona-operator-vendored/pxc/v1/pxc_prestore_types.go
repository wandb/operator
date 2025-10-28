package v1

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PerconaXtraDBClusterRestoreSpec defines the desired state of PerconaXtraDBClusterRestore
type PerconaXtraDBClusterRestoreSpec struct {
	PXCCluster       string                      `json:"pxcCluster"`
	BackupName       string                      `json:"backupName"`
	ContainerOptions *BackupContainerOptions     `json:"containerOptions,omitempty"`
	BackupSource     *PXCBackupStatus            `json:"backupSource,omitempty"`
	PITR             *PITR                       `json:"pitr,omitempty"`
	Resources        corev1.ResourceRequirements `json:"resources,omitempty"`
}

// PerconaXtraDBClusterRestoreStatus defines the observed state of PerconaXtraDBClusterRestore
type PerconaXtraDBClusterRestoreStatus struct {
	State         RestoreState `json:"state,omitempty"`
	Comments      string       `json:"comments,omitempty"`
	CompletedAt   *metav1.Time `json:"completed,omitempty"`
	LastScheduled *metav1.Time `json:"lastscheduled,omitempty"`
	PXCSize       int32        `json:"clusterSize,omitempty"`
	HAProxySize   int32        `json:"haproxySize,omitempty"`
	ProxySQLSize  int32        `json:"proxysqlSize,omitempty"`
	Unsafe        UnsafeFlags  `json:"unsafeFlags,omitempty"`
}

type PITR struct {
	BackupSource *PXCBackupStatus `json:"backupSource"`
	Type         string           `json:"type"`
	Date         string           `json:"date"`
	GTID         string           `json:"gtid"`
}

// PerconaXtraDBClusterRestore is the Schema for the perconaxtradbclusterrestores API
type PerconaXtraDBClusterRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PerconaXtraDBClusterRestoreSpec   `json:"spec,omitempty"`
	Status PerconaXtraDBClusterRestoreStatus `json:"status,omitempty"`
}

// PerconaXtraDBClusterRestoreList contains a list of PerconaXtraDBClusterRestore
type PerconaXtraDBClusterRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PerconaXtraDBClusterRestore `json:"items"`
}

type RestoreState string

const (
	RestoreNew            RestoreState = ""
	RestoreStarting       RestoreState = "Starting"
	RestoreStopCluster    RestoreState = "Stopping Cluster"
	RestoreRestore        RestoreState = "Restoring"
	RestoreStartCluster   RestoreState = "Starting Cluster"
	RestorePITR           RestoreState = "Point-in-time recovering"
	RestorePrepareCluster RestoreState = "Preparing Cluster"
	RestoreFailed         RestoreState = "Failed"
	RestoreSucceeded      RestoreState = "Succeeded"
)

const AnnotationUnsafePITR = "percona.com/unsafe-pitr"

func (cr *PerconaXtraDBClusterRestore) CheckNsetDefaults() error {
	if cr.Spec.PXCCluster == "" {
		return errors.New("pxcCluster can't be empty")
	}
	if cr.Spec.PITR != nil && cr.Spec.PITR.BackupSource != nil && cr.Spec.PITR.BackupSource.StorageName == "" && cr.Spec.PITR.BackupSource.S3 == nil && cr.Spec.PITR.BackupSource.Azure == nil {
		return errors.New("PITR.BackupSource.StorageName, PITR.BackupSource.S3 and PITR.BackupSource.Azure can't be empty simultaneously")
	}
	if cr.Spec.BackupName == "" && cr.Spec.BackupSource == nil {
		return errors.New("backupName and BackupSource can't be empty simultaneously")
	}
	if len(cr.Spec.BackupName) > 0 && cr.Spec.BackupSource != nil {
		return errors.New("backupName and BackupSource can't be specified simultaneously")
	}

	return nil
}
