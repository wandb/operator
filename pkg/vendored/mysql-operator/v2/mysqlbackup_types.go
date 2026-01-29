/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MySQLBackupSpec defines the desired state of MySQLBackup
// +kubebuilder:object:generate=true
type MySQLBackupSpec struct {
	ClusterName                   string         `json:"clusterName"`
	BackupProfileName             string         `json:"backupProfileName,omitempty"`
	BackupProfile                 *BackupProfile `json:"backupProfile,omitempty"`
	Incremental                   bool           `json:"incremental,omitempty"`
	IncrementalBase               string         `json:"incrementalBase,omitempty"`
	AddTimestampToBackupDirectory bool           `json:"addTimestampToBackupDirectory,omitempty"`
	DeleteBackupData              bool           `json:"deleteBackupData,omitempty"`
}

// MySQLBackupStatus defines the observed state of MySQLBackup
// +kubebuilder:object:generate=true
type MySQLBackupStatus struct {
	Status         string `json:"status,omitempty"`
	StartTime      string `json:"startTime,omitempty"`
	CompletionTime string `json:"completionTime,omitempty"`
	ElapsedTime    string `json:"elapsedTime,omitempty"`
	Output         string `json:"output,omitempty"`
	Method         string `json:"method,omitempty"`
	Source         string `json:"source,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	OCITenancy     string `json:"ociTenancy,omitempty"`
	Container      string `json:"container,omitempty"`
	SpaceAvailable string `json:"spaceAvailable,omitempty"`
	Size           string `json:"size,omitempty"`
	Message        string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MySQLBackup is the Schema for the mysqlbackups API
// +kubebuilder:object:generate=true
type MySQLBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MySQLBackupSpec   `json:"spec,omitempty"`
	Status MySQLBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MySQLBackupList contains a list of MySQLBackup
// +kubebuilder:object:generate=true
type MySQLBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MySQLBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MySQLBackup{}, &MySQLBackupList{})
}
