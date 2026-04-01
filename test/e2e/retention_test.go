/*
Copyright 2025.

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

package e2e

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/test/utils"
)

var retentionSuiteSetupDone bool

const (
	retentionReadyTimeout    = 15 * time.Minute
	retentionDeleteTimeout   = 5 * time.Minute
	retentionPollingInterval = 10 * time.Second

	wandbNameLabel      = "apps.wandb.com/name"
	wandbNamespaceLabel = "apps.wandb.com/namespace"
	wandbModuleLabel    = "apps.wandb.com/module"
)

// infraCRType maps a module name to its third-party Kubernetes CRD resource type.
var infraCRType = map[string]string{
	"mysql":       "innodbcluster",
	"redis":       "redis",
	"kafka":       "kafka",
	"objectStore": "tenant",
	"clickhouse":  "clickhouseinstallation",
}

var allModules = []string{"mysql", "redis", "kafka", "objectStore", "clickhouse"}

func generateUniqueName(prefix string) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	suffix := make([]byte, 6)
	for i := range suffix {
		suffix[i] = letters[rand.Intn(len(letters))] //nolint:gosec
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().Unix(), string(suffix))
}

func createNamespace(ns string) error {
	cmd := exec.Command("kubectl", "create", "namespace", ns)
	_, err := utils.Run(cmd)
	return err
}

func deleteNamespace(ns string) {
	cmd := exec.Command("kubectl", "delete", "namespace", ns, "--ignore-not-found", "--wait=false")
	_, _ = utils.Run(cmd)
}

func buildLabelSelector(wandbName, wandbNs, module string) string {
	return fmt.Sprintf("%s=%s,%s=%s,%s=%s",
		wandbNameLabel, wandbName,
		wandbNamespaceLabel, wandbNs,
		wandbModuleLabel, module,
	)
}

type manifestOpts struct {
	name                string
	ns                  string
	size                string
	specRetentionPolicy string
	componentOverrides  map[string]string
}

func createWandbManifest(opts manifestOpts) string {
	managedFieldName := map[string]string{
		"mysql":       "managedMysql",
		"redis":       "managedRedis",
		"kafka":       "managedKafka",
		"objectStore": "managedObjectStore",
		"clickhouse":  "managedClickhouse",
	}
	componentBlock := func(module string) string {
		managed := managedFieldName[module]
		override := ""
		if policy, ok := opts.componentOverrides[module]; ok {
			override = fmt.Sprintf("\n      retentionPolicy:\n        onDelete: %s", policy)
		}
		extra := ""
		if module == "mysql" {
			extra = "\n      deploymentType: mysql"
		}
		return fmt.Sprintf("  %s:\n    %s:%s%s", module, managed, extra, override)
	}

	return fmt.Sprintf(`apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: %s
  namespace: %s
spec:
  size: %s
  retentionPolicy:
    onDelete: %s
  wandb:
    features: {}
%s
%s
%s
%s
%s
`, opts.name, opts.ns, opts.size, opts.specRetentionPolicy,
		componentBlock("mysql"),
		componentBlock("redis"),
		componentBlock("kafka"),
		componentBlock("objectStore"),
		componentBlock("clickhouse"),
	)
}

func applyManifest(manifest string) error {
	f, err := os.CreateTemp("", "wandb-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(manifest); err != nil {
		return err
	}
	f.Close()
	cmd := exec.Command("kubectl", "apply", "-f", f.Name())
	_, err = utils.Run(cmd)
	return err
}

func deleteWandB(name, ns string) error {
	cmd := exec.Command("kubectl", "delete", "weightsandbiases", name, "-n", ns, "--wait=false")
	_, err := utils.Run(cmd)
	return err
}

func patchComponentDisabled(wandbName, ns, module string) error {
	patch := fmt.Sprintf(`{"spec":{%q:{"enabled":false}}}`, module)
	cmd := exec.Command("kubectl", "patch", "weightsandbiases", wandbName,
		"-n", ns, "--type=merge", "-p", patch)
	_, err := utils.Run(cmd)
	return err
}

// waitForWandbReady polls until all five component statuses report "Ready".
func waitForWandbReady(name, ns string) {
	verifyReady := func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "weightsandbiases", name, "-n", ns, "-o", "json")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())

		var obj map[string]any
		g.Expect(json.Unmarshal([]byte(output), &obj)).To(Succeed())

		status, ok := obj["status"].(map[string]any)
		g.Expect(ok).To(BeTrue(), "status field missing")

		statusKeys := map[string]string{
			"mysql":       "mysqlStatus",
			"redis":       "redisStatus",
			"kafka":       "kafkaStatus",
			"objectStore": "objectStoreStatus",
			"clickhouse":  "clickhouseStatus",
		}
		for _, key := range statusKeys {
			cs, ok := status[key].(map[string]any)
			g.Expect(ok).To(BeTrue(), "component status %s missing", key)
			g.Expect(cs["state"]).To(Equal("Healthy"), "component %s not Healthy", key)
		}
	}
	Eventually(verifyReady, retentionReadyTimeout, retentionPollingInterval).Should(Succeed())
}

func resourceItemCount(resourceType, labelSelector, ns string) int {
	cmd := exec.Command("kubectl", "get", resourceType,
		"-l", labelSelector, "-n", ns, "-o", "json")
	output, err := utils.Run(cmd)
	if err != nil {
		return -1
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return -1
	}
	items, _ := result["items"].([]any)
	return len(items)
}

// resourceItemCountInNs counts all resources of the given type in a namespace without label filtering.
// Used for infra CRs that are not labeled with WandB metadata labels.
func resourceItemCountInNs(resourceType, ns string) int {
	cmd := exec.Command("kubectl", "get", resourceType, "-n", ns, "-o", "json")
	output, err := utils.Run(cmd)
	if err != nil {
		return -1
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return -1
	}
	items, _ := result["items"].([]any)
	return len(items)
}

func verifyComponentResourcesExist(wandbName, ns, module string, g Gomega) {
	sel := buildLabelSelector(wandbName, ns, module)
	g.Expect(resourceItemCount("pvc", sel, ns)).To(BeNumerically(">", 0),
		"expected PVCs for module %s", module)
	g.Expect(resourceItemCountInNs(infraCRType[module], ns)).To(BeNumerically(">", 0),
		"expected infra CR %s for module %s", infraCRType[module], module)
}

func verifyComponentResourcesDeleted(wandbName, ns, module string, timeout time.Duration) {
	sel := buildLabelSelector(wandbName, ns, module)
	verifyGone := func(g Gomega) {
		g.Expect(resourceItemCount("pvc", sel, ns)).To(Equal(0),
			"PVCs still exist for module %s", module)
		g.Expect(resourceItemCountInNs(infraCRType[module], ns)).To(Equal(0),
			"infra CR %s still exists for module %s", infraCRType[module], module)
	}
	Eventually(verifyGone, timeout, retentionPollingInterval).Should(Succeed())
}

func verifyWandbDeleted(name, ns string) {
	verifyGone := func(g Gomega) {
		cmd := exec.Command("kubectl", "get", "weightsandbiases", name, "-n", ns)
		_, err := utils.Run(cmd)
		g.Expect(err).To(HaveOccurred(), "WandB CR should no longer exist")
	}
	Eventually(verifyGone, retentionDeleteTimeout, retentionPollingInterval).Should(Succeed())
}

var _ = Describe("Retention Policy Integration Tests", func() {

	BeforeEach(func() {
		if retentionSuiteSetupDone {
			return
		}

		By("ensuring WeightsAndBiases CRD is installed")
		cmd := exec.Command("kubectl", "get", "crd", "weightsandbiases.apps.wandb.com")
		if _, err := utils.Run(cmd); err != nil {
			By("CRD not found — installing from deploy/operator/crds")
			projectDir, err := utils.GetProjectDir()
			Expect(err).NotTo(HaveOccurred())
			cmd = exec.Command("kubectl", "apply", "-f",
				projectDir+"/deploy/operator/crds/apps.wandb.com_weightsandbiases.yaml",
				"--server-side")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to install WeightsAndBiases CRD")
		}

		By("ensuring the WandB operator controller is running")
		cmd = exec.Command("kubectl", "get", "pods",
			"-l", "control-plane=controller-manager",
			"-A",
			"--field-selector=status.phase=Running",
			"-o", "name")
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to check for operator controller pod")
		Expect(utils.GetNonEmptyLines(output)).NotTo(BeEmpty(),
			"No running operator controller pod found — deploy the WandB operator before running retention tests")

		retentionSuiteSetupDone = true
	})

	// setupWandbCR creates the namespace and applies the WandB CR manifest, returning cleanup func.
	setupWandbCR := func(wandbName, wandbNs *string, namePrefix, size, specPolicy string, overrides map[string]string) {
		BeforeEach(func() {
			*wandbName = generateUniqueName(namePrefix)
			*wandbNs = generateUniqueName("wandb-test")

			By(fmt.Sprintf("creating namespace %s", *wandbNs))
			Expect(createNamespace(*wandbNs)).To(Succeed())

			By(fmt.Sprintf("creating WandB CR %s (size=%s, specPolicy=%s)", *wandbName, size, specPolicy))
			manifest := createWandbManifest(manifestOpts{
				name:                *wandbName,
				ns:                  *wandbNs,
				size:                size,
				specRetentionPolicy: specPolicy,
				componentOverrides:  overrides,
			})
			Expect(applyManifest(manifest)).To(Succeed())
		})

		AfterEach(func() {
			deleteNamespace(*wandbNs)
		})
	}

	// --- Purge: CR deletion ---
	runPurgeCRDeletionTest := func(size string) {
		var (
			wandbName string
			wandbNs   string
		)
		setupWandbCR(&wandbName, &wandbNs, fmt.Sprintf("wandb-%s-purge", size), size, "purge", nil)

		It("should purge all resources when the CR is deleted", func() {
			By("waiting for all components to be Ready")
			waitForWandbReady(wandbName, wandbNs)

			By("verifying all component resources exist")
			for _, module := range allModules {
				verifyComponentResourcesExist(wandbName, wandbNs, module, Default)
			}

			By("deleting the WandB CR")
			Expect(deleteWandB(wandbName, wandbNs)).To(Succeed())

			By("verifying all component resources are purged")
			for _, module := range allModules {
				verifyComponentResourcesDeleted(wandbName, wandbNs, module, retentionDeleteTimeout)
			}

			By("verifying the WandB CR itself is gone")
			verifyWandbDeleted(wandbName, wandbNs)
		})
	}

	// --- Purge: component disable ---
	runPurgeComponentDisableTest := func(size, module string) {
		var (
			wandbName string
			wandbNs   string
		)
		setupWandbCR(&wandbName, &wandbNs, fmt.Sprintf("wandb-%s-%s-dis", size, module), size, "purge", nil)

		It(fmt.Sprintf("should purge %s resources when the component is disabled", module), func() {
			By("waiting for all components to be Ready")
			waitForWandbReady(wandbName, wandbNs)

			By("verifying all component resources exist")
			for _, m := range allModules {
				verifyComponentResourcesExist(wandbName, wandbNs, m, Default)
			}

			By(fmt.Sprintf("disabling the %s component", module))
			Expect(patchComponentDisabled(wandbName, wandbNs, module)).To(Succeed())

			By(fmt.Sprintf("verifying %s resources are purged", module))
			verifyComponentResourcesDeleted(wandbName, wandbNs, module, retentionDeleteTimeout)

			By("verifying remaining components are unaffected")
			for _, m := range allModules {
				if m == module {
					continue
				}
				verifyComponentResourcesExist(wandbName, wandbNs, m, Default)
			}
		})
	}

	// --- Detach: CR deletion ---
	runDetachCRDeletionTest := func(size string) {
		var (
			wandbName string
			wandbNs   string
		)
		setupWandbCR(&wandbName, &wandbNs, fmt.Sprintf("wandb-%s-detach", size), size, "detach", nil)

		It("should detach all resources when the CR is deleted", func() {
			By("waiting for all components to be Ready")
			waitForWandbReady(wandbName, wandbNs)

			By("verifying all component resources exist")
			for _, module := range allModules {
				verifyComponentResourcesExist(wandbName, wandbNs, module, Default)
			}

			By("deleting the WandB CR")
			Expect(deleteWandB(wandbName, wandbNs)).To(Succeed())

			By("verifying the WandB CR itself is gone")
			verifyWandbDeleted(wandbName, wandbNs)

			By("verifying all component resources still exist (detached, not purged)")
			for _, module := range allModules {
				verifyComponentResourcesExist(wandbName, wandbNs, module, Default)
			}
		})
	}

	// --- Detach: component disable ---
	runDetachComponentDisableTest := func(size, module string) {
		var (
			wandbName string
			wandbNs   string
		)
		setupWandbCR(&wandbName, &wandbNs, fmt.Sprintf("wandb-%s-%s-det", size, module), size, "detach", nil)

		It(fmt.Sprintf("should detach %s resources when the component is disabled", module), func() {
			By("waiting for all components to be Ready")
			waitForWandbReady(wandbName, wandbNs)

			By("verifying all component resources exist")
			for _, m := range allModules {
				verifyComponentResourcesExist(wandbName, wandbNs, m, Default)
			}

			By(fmt.Sprintf("disabling the %s component", module))
			Expect(patchComponentDisabled(wandbName, wandbNs, module)).To(Succeed())

			Consistently(func(g Gomega) {
				verifyComponentResourcesExist(wandbName, wandbNs, module, g)
			}, 30*time.Second, retentionPollingInterval).Should(Succeed(),
				"%s resources should survive disable with detach policy", module)

			By("verifying remaining components are unaffected")
			for _, m := range allModules {
				if m == module {
					continue
				}
				verifyComponentResourcesExist(wandbName, wandbNs, m, Default)
			}
		})
	}

	// --- Component-level override: spec=detach, component=purge ---
	runComponentOverridePurgeTest := func(size, module string) {
		var (
			wandbName string
			wandbNs   string
		)
		overrides := map[string]string{module: "purge"}
		setupWandbCR(&wandbName, &wandbNs, fmt.Sprintf("wandb-%s-%s-ovp", size, module), size, "detach", overrides)

		It(fmt.Sprintf("should purge %s (override) while detaching others on CR deletion", module), func() {
			By("waiting for all components to be Ready")
			waitForWandbReady(wandbName, wandbNs)

			By("verifying all component resources exist")
			for _, m := range allModules {
				verifyComponentResourcesExist(wandbName, wandbNs, m, Default)
			}

			By("deleting the WandB CR")
			Expect(deleteWandB(wandbName, wandbNs)).To(Succeed())

			By("verifying the WandB CR itself is gone")
			verifyWandbDeleted(wandbName, wandbNs)

			By(fmt.Sprintf("verifying %s resources are purged (component override)", module))
			verifyComponentResourcesDeleted(wandbName, wandbNs, module, retentionDeleteTimeout)

			By("verifying other components still exist (detached)")
			for _, m := range allModules {
				if m == module {
					continue
				}
				verifyComponentResourcesExist(wandbName, wandbNs, m, Default)
			}
		})
	}

	// --- Component-level override: spec=purge, component=detach ---
	runComponentOverrideDetachTest := func(size, module string) {
		var (
			wandbName string
			wandbNs   string
		)
		overrides := map[string]string{module: "detach"}
		setupWandbCR(&wandbName, &wandbNs, fmt.Sprintf("wandb-%s-%s-ovd", size, module), size, "purge", overrides)

		It(fmt.Sprintf("should detach %s (override) while purging others on CR deletion", module), func() {
			By("waiting for all components to be Ready")
			waitForWandbReady(wandbName, wandbNs)

			By("verifying all component resources exist")
			for _, m := range allModules {
				verifyComponentResourcesExist(wandbName, wandbNs, m, Default)
			}

			By("deleting the WandB CR")
			Expect(deleteWandB(wandbName, wandbNs)).To(Succeed())

			By("verifying the WandB CR itself is gone")
			verifyWandbDeleted(wandbName, wandbNs)

			By(fmt.Sprintf("verifying %s resources still exist (component detach override)", module))
			verifyComponentResourcesExist(wandbName, wandbNs, module, Default)

			By("verifying other components are purged")
			for _, m := range allModules {
				if m == module {
					continue
				}
				verifyComponentResourcesDeleted(wandbName, wandbNs, m, retentionDeleteTimeout)
			}
		})
	}

	for _, size := range []string{"dev", "small"} {
		size := size

		Context(fmt.Sprintf("%s size", size), func() {
			Context("spec-level purge policy", func() {
				Context("CR deletion", func() {
					runPurgeCRDeletionTest(size)
				})

				Context("component disable", func() {
					for _, module := range allModules {
						module := module
						Context(fmt.Sprintf("%s component", module), func() {
							runPurgeComponentDisableTest(size, module)
						})
					}
				})
			})

			Context("spec-level detach policy", func() {
				Context("CR deletion", func() {
					runDetachCRDeletionTest(size)
				})

				Context("component disable", func() {
					for _, module := range allModules {
						module := module
						Context(fmt.Sprintf("%s component", module), func() {
							runDetachComponentDisableTest(size, module)
						})
					}
				})
			})

			Context("component-level overrides", func() {
				Context("spec=detach with component=purge override", func() {
					for _, module := range allModules {
						module := module
						Context(fmt.Sprintf("%s component", module), func() {
							runComponentOverridePurgeTest(size, module)
						})
					}
				})

				Context("spec=purge with component=detach override", func() {
					for _, module := range allModules {
						module := module
						Context(fmt.Sprintf("%s component", module), func() {
							runComponentOverrideDetachTest(size, module)
						})
					}
				})
			})
		})
	}
})
