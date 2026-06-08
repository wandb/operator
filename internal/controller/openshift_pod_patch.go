/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

// patchPodSpecForOpenShift applies OpenShift restricted-v2 compatibility
// patches to a PodSpec in-place. Idempotent: safe to call on each reconcile
// (the caller always copies a fresh template from the Application spec first).
//
// The trigger is the utils.OpenShiftNginxOverlayLabel label, which the
// reconciler sets on workloads that need it (its value names the target
// container). This keeps image-specific knowledge in the single place that
// builds the Application, rather than sniffing image names at the workload
// layer.
func patchPodSpecForOpenShift(podSpec *corev1.PodSpec, podLabels map[string]string) {
	if !utils.IsOpenShift() {
		return
	}
	if containerName, ok := podLabels[utils.OpenShiftNginxOverlayLabel]; ok {
		injectNginxWritableOverlay(podSpec, containerName)
	}
}

const (
	nginxHTMLVolumeName  = "openshift-nginx-html"
	nginxConfVolumeName  = "openshift-nginx-conf"
	nginxCacheVolumeName = "openshift-nginx-cache"
	nginxRunVolumeName   = "openshift-nginx-run"
	nginxInitName        = "openshift-nginx-html-init"
)

// injectNginxWritableOverlay overlays the read-only nginx asset and runtime
// directories of the named container with emptyDir volumes and seeds
// /usr/share/nginx/html via an initContainer using the same image. This lets
// the container start under an arbitrary UID assigned by the namespace SCC.
func injectNginxWritableOverlay(podSpec *corev1.PodSpec, containerName string) {
	containerIdx := -1
	for i, c := range podSpec.Containers {
		if c.Name == containerName {
			containerIdx = i
			break
		}
	}
	if containerIdx < 0 {
		return
	}
	c := &podSpec.Containers[containerIdx]

	addEmptyDirVolume(podSpec, nginxHTMLVolumeName)
	addEmptyDirVolume(podSpec, nginxConfVolumeName)
	addEmptyDirVolume(podSpec, nginxCacheVolumeName)
	addEmptyDirVolume(podSpec, nginxRunVolumeName)

	addVolumeMount(c, nginxHTMLVolumeName, "/usr/share/nginx/html")
	addVolumeMount(c, nginxConfVolumeName, "/etc/nginx")
	addVolumeMount(c, nginxCacheVolumeName, "/var/cache/nginx")
	addVolumeMount(c, nginxRunVolumeName, "/var/run")

	if !hasContainerNamed(podSpec.InitContainers, nginxInitName) {
		podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{
			Name:            nginxInitName,
			Image:           c.Image,
			ImagePullPolicy: c.ImagePullPolicy,
			Command: []string{"sh", "-c",
				"cp -r /usr/share/nginx/html/. /mnt/openshift-nginx-html/ && " +
					"cp -r /etc/nginx/. /mnt/openshift-nginx-conf/"},
			VolumeMounts: []corev1.VolumeMount{
				{Name: nginxHTMLVolumeName, MountPath: "/mnt/openshift-nginx-html"},
				{Name: nginxConfVolumeName, MountPath: "/mnt/openshift-nginx-conf"},
			},
		})
	}
}

func addEmptyDirVolume(podSpec *corev1.PodSpec, name string) {
	for _, v := range podSpec.Volumes {
		if v.Name == name {
			return
		}
	}
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name:         name,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	})
}

func addVolumeMount(c *corev1.Container, name, path string) {
	for _, m := range c.VolumeMounts {
		if m.Name == name {
			return
		}
	}
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: name, MountPath: path})
}

func hasContainerNamed(containers []corev1.Container, name string) bool {
	for _, c := range containers {
		if c.Name == name {
			return true
		}
	}
	return false
}
