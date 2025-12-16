package tenant

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	integrationTestEnv     *envtest.Environment
	integrationTestCfg     *rest.Config
	integrationTestClient  client.Client
	integrationTestCtx     context.Context
	integrationTestCancel  context.CancelFunc
	integrationTestCounter int
)

var _ = BeforeSuite(func() {
	ctrllog.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	integrationTestCtx, integrationTestCancel = context.WithCancel(context.TODO())

	err := miniov2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	integrationTestEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths:              []string{},
			ErrorIfPathMissing: false,
		},
		Scheme: scheme.Scheme,
	}

	if binDir := os.Getenv("KUBEBUILDER_ASSETS"); binDir != "" {
		integrationTestEnv.BinaryAssetsDirectory = binDir
	} else if binDir := getEnvTestBinaryDir(); binDir != "" {
		integrationTestEnv.BinaryAssetsDirectory = binDir
	}

	integrationTestCfg, err = integrationTestEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(integrationTestCfg).NotTo(BeNil())

	integrationTestClient, err = client.New(integrationTestCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(integrationTestClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	integrationTestCancel()
	err := integrationTestEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func getEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

var _ = Describe("Minio Config and Connection Integration", func() {
	var (
		ctx           context.Context
		testNamespace string
		specNsName    types.NamespacedName
		tenantOwner   *miniov2.Tenant
		nsNameBldr    *NsNameBuilder
		envConfig     MinioEnvConfig
		wandbOwner    *corev1.ConfigMap
	)

	BeforeEach(func() {
		ctx = integrationTestCtx
		integrationTestCounter++
		testNamespace = fmt.Sprintf("test-minio-%d", integrationTestCounter)

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		err := integrationTestClient.Create(ctx, ns)
		Expect(err).NotTo(HaveOccurred())

		specNsName = types.NamespacedName{
			Namespace: testNamespace,
			Name:      "test-tenant",
		}

		nsNameBldr = CreateNsNameBuilder(specNsName)

		tenantOwner = createMinimalTenant(specNsName)
		tenantOwner.ObjectMeta = createProxyObjectMeta(ctx, integrationTestClient, testNamespace, "test-tenant-owner")

		envConfig = MinioEnvConfig{
			RootUser:            "admin",
			MinioBrowserSetting: "on",
		}

		wandbOwner = &corev1.ConfigMap{
			ObjectMeta: createProxyObjectMeta(ctx, integrationTestClient, testNamespace, "test-wandb-owner"),
		}
	})

	AfterEach(func() {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		_ = integrationTestClient.Delete(ctx, ns)
	})

	Context("password generation and preservation", func() {
		It("should generate a password on first call and preserve it on subsequent calls", func() {
			By("calling writeMinioConfig when no secret exists")
			firstConnInfo, err := writeMinioConfig(
				ctx,
				integrationTestClient,
				tenantOwner,
				nsNameBldr,
				envConfig,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstConnInfo).NotTo(BeNil())
			Expect(firstConnInfo.RootPassword).NotTo(BeEmpty())

			By("verifying the generated password is correct format")
			configSecret := &corev1.Secret{}
			err = integrationTestClient.Get(
				ctx,
				nsNameBldr.ConfigNsName(),
				configSecret,
			)
			Expect(err).NotTo(HaveOccurred())

			configContents := string(configSecret.Data["config.env"])
			firstParsedConfig := parseMinioConfigFile(configContents)

			Expect(firstParsedConfig.rootPassword).NotTo(BeEmpty())
			Expect(len(firstParsedConfig.rootPassword)).To(Equal(20))
			Expect(firstParsedConfig.rootPassword).To(MatchRegexp("^[a-zA-Z]+$"))
			Expect(firstParsedConfig.rootUser).To(Equal("admin"))
			Expect(firstParsedConfig.minioBrowserSetting).To(Equal("on"))
			Expect(firstConnInfo.RootPassword).To(Equal(firstParsedConfig.rootPassword))

			generatedPassword := firstParsedConfig.rootPassword

			By("calling writeMinioConfig again with the secret present")
			secondConnInfo, err := writeMinioConfig(
				ctx,
				integrationTestClient,
				tenantOwner,
				nsNameBldr,
				envConfig,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(secondConnInfo).NotTo(BeNil())

			By("verifying the password was preserved (not regenerated)")
			updatedSecret := &corev1.Secret{}
			err = integrationTestClient.Get(
				ctx,
				nsNameBldr.ConfigNsName(),
				updatedSecret,
			)
			Expect(err).NotTo(HaveOccurred())

			updatedContents := string(updatedSecret.Data["config.env"])
			secondParsedConfig := parseMinioConfigFile(updatedContents)

			Expect(secondParsedConfig.rootPassword).To(Equal(generatedPassword))
			Expect(secondConnInfo.RootPassword).To(Equal(generatedPassword))
			Expect(secondParsedConfig.rootUser).To(Equal("admin"))
			Expect(secondParsedConfig.minioBrowserSetting).To(Equal("on"))
		})
	})

	Context("full resource creation", func() {
		It("should create all expected secrets and return valid connection info", func() {
			By("creating minio config secret")
			connInfo, err := writeMinioConfig(
				ctx,
				integrationTestClient,
				tenantOwner,
				nsNameBldr,
				envConfig,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(connInfo).NotTo(BeNil())

			By("verifying config secret exists with correct content")
			configSecret := &corev1.Secret{}
			err = integrationTestClient.Get(
				ctx,
				nsNameBldr.ConfigNsName(),
				configSecret,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.env"))

			configContents := string(configSecret.Data["config.env"])
			parsedConfig := parseMinioConfigFile(configContents)
			Expect(parsedConfig.rootUser).To(Equal("admin"))
			Expect(parsedConfig.rootPassword).NotTo(BeEmpty())
			Expect(parsedConfig.minioBrowserSetting).To(Equal("on"))

			By("verifying config secret has correct owner reference")
			Expect(configSecret.OwnerReferences).To(HaveLen(1))
			ownerRef := configSecret.OwnerReferences[0]
			Expect(ownerRef.UID).To(Equal(tenantOwner.UID))
			Expect(ownerRef.Name).To(Equal(tenantOwner.Name))

			By("creating wandb connection secret")
			connection, err := writeWandbConnInfo(
				ctx,
				integrationTestClient,
				wandbOwner,
				nsNameBldr,
				connInfo,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(connection).NotTo(BeNil())

			By("verifying connection secret exists with correct URL")
			connSecret := &corev1.Secret{}
			err = integrationTestClient.Get(
				ctx,
				nsNameBldr.ConnectionNsName(),
				connSecret,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(connSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(connSecret.Data).To(HaveKey("url"))

			urlString := string(connSecret.Data["url"])
			Expect(urlString).To(ContainSubstring("s3://"))
			Expect(urlString).To(ContainSubstring("admin:"))
			Expect(urlString).To(ContainSubstring(parsedConfig.rootPassword))
			Expect(urlString).To(ContainSubstring(nsNameBldr.ServiceName()))
			Expect(urlString).To(ContainSubstring(testNamespace))
			Expect(urlString).To(ContainSubstring("?tls=true"))

			By("verifying connection secret has correct owner reference")
			Expect(connSecret.OwnerReferences).To(HaveLen(1))
			connOwnerRef := connSecret.OwnerReferences[0]
			Expect(connOwnerRef.Kind).To(Equal("ConfigMap"))
			Expect(connOwnerRef.Name).To(Equal(wandbOwner.Name))
			Expect(connOwnerRef.UID).To(Equal(wandbOwner.UID))

			By("verifying returned InfraConnection has correct structure")
			Expect(connection.URL.Name).To(Equal(nsNameBldr.ConnectionName()))
			Expect(connection.URL.Key).To(Equal("url"))
			Expect(connection.URL.Optional).NotTo(BeNil())
			Expect(*connection.URL.Optional).To(BeFalse())
		})
	})
})

func createProxyObjectMeta(ctx context.Context, client client.Client, namespace, name string) metav1.ObjectMeta {
	proxy := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := client.Create(ctx, proxy)
	if err != nil {
		panic(fmt.Sprintf("failed to create proxy object: %v", err))
	}

	err = client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, proxy)
	if err != nil {
		panic(fmt.Sprintf("failed to get proxy object: %v", err))
	}

	return proxy.ObjectMeta
}

func createMinimalTenant(nsName types.NamespacedName) *miniov2.Tenant {
	return &miniov2.Tenant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "minio.min.io/v2",
			Kind:       "Tenant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsName.Name,
			Namespace: nsName.Namespace,
		},
		Spec: miniov2.TenantSpec{
			Pools: []miniov2.Pool{
				{
					Name:             PoolName(nsName.Name),
					Servers:          1,
					VolumesPerServer: 1,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
		},
	}
}
