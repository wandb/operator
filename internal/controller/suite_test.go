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

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv1 "github.com/wandb/operator/api/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	clickhousev1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	kafkav1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	webhookv2 "github.com/wandb/operator/internal/webhook/v2"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
	mgr       manager.Manager
	mgrErrCh  chan error
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctrllog.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error

	err = apiv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = apiv2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = clickhousev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = miniov2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = pxcv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = redisv1beta2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = redisreplicationv1beta2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = redissentinelv1beta2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = kafkav1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "config", "crd", "bases"),
				filepath.Join("..", "..", "internal", "vendored", "altinity-clickhouse", "crds"),
				filepath.Join("..", "..", "internal", "vendored", "minio-operator", "crds"),
				filepath.Join("..", "..", "internal", "vendored", "percona-operator", "crds"),
				filepath.Join("..", "..", "internal", "vendored", "redis-operator", "crds"),
				filepath.Join("..", "..", "internal", "vendored", "strimzi-kafka", "crds"),
			},
		},
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
		ErrorIfCRDPathMissing: true,
		Scheme:                scheme.Scheme,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()

	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err = manager.New(cfg, manager.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
	})
	Expect(err).NotTo(HaveOccurred())

	err = webhookv2.SetupWeightsAndBiasesWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())
	err = webhookv2.SetupApplicationWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		err = mgr.Start(ctx)
		if err != nil && ctx.Err() == nil {
			// Only fail if not cancelled
			Fail(fmt.Sprintf("manager failed to start: %v", err))
		}
	}()

	// Wait for webhook server to be ready
	Eventually(func() error {
		return mgr.GetWebhookServer().StartedChecker()(nil)
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		ctrllog.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}
