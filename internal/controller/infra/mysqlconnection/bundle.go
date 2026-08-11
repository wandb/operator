package mysqlconnection

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	BundleVersion = "1"

	BundleVersionAnnotation = "weightsandbiases.apps.wandb.com/mysql-bundle-version"
	InstanceAnnotation      = "weightsandbiases.apps.wandb.com/mysql-instance"
	ProviderReadyType       = "ProviderReady"
	ConnectionResolvedType  = "ConnectionResolved"
	BundleReadyType         = "BundleReady"
	DatabaseInitializedType = "DatabaseInitialized"

	URLKey      = "url"
	HostKey     = "Host"
	PortKey     = "Port"
	DatabaseKey = "Database"
	UsernameKey = "Username"
	PasswordKey = "Password"
	TLSKey      = "Tls"
	SSLCAKey    = "SslCa"
	SSLCertKey  = "SslCert"
	SSLKeyKey   = "SslKey"

	CACertFile     = "ca.pem"
	ClientCertFile = "client-cert.pem"
	ClientKeyFile  = "client-key.pem"

	bundleManagedBy = "wandb-operator"
	bundleComponent = "mysql-connection"
)

const instanceIDLength = 10

// Material is the provider-independent MySQL connection data used to create
// the application-facing bundle.
type Material struct {
	Host       string
	Port       string
	Database   string
	Username   string
	Password   string
	TLS        string
	CACert     []byte
	ClientCert []byte
	ClientKey  []byte
}

func InstanceID(instance string) string {
	if instance == "" {
		instance = apiv2.DefaultInstanceName
	}
	digest := sha256.Sum256([]byte(instance))
	return hex.EncodeToString(digest[:])[:instanceIDLength]
}

func SecretName(wandbName, instance string) string {
	base := fmt.Sprintf("%s-%s", wandbName, InstanceID(instance))
	return common.FitDefaultInfraName(base, "-mysql-connection", validation.DNS1123LabelMaxLength)
}

func MountPath(instance string) string {
	return fmt.Sprintf("/var/run/secrets/wandb/mysql/%s", InstanceID(instance))
}

func VolumeName(instance string) string {
	return "mysql-" + InstanceID(instance)
}

// Normalize validates provider material and converts the v2 TLS selector value
// into the URL modes supported by the currently deployed Gorilla connector.
func Normalize(material Material) (Material, error) {
	material.Host = strings.TrimSpace(material.Host)
	material.Port = strings.TrimSpace(material.Port)
	material.Database = strings.TrimSpace(material.Database)
	material.Username = strings.TrimSpace(material.Username)
	material.TLS = strings.ToLower(strings.TrimSpace(material.TLS))

	if material.Host == "" {
		return Material{}, fmt.Errorf("host is required")
	}
	port, err := strconv.ParseUint(material.Port, 10, 16)
	if err != nil || port == 0 {
		return Material{}, fmt.Errorf("port %q must be an integer between 1 and 65535", material.Port)
	}
	if material.Database == "" {
		return Material{}, fmt.Errorf("database is required")
	}
	if material.Username == "" {
		return Material{}, fmt.Errorf("username is required")
	}

	hasCA := len(material.CACert) > 0
	hasClientCert := len(material.ClientCert) > 0
	hasClientKey := len(material.ClientKey) > 0
	if hasClientCert != hasClientKey {
		return Material{}, fmt.Errorf("sslCert and sslKey must be configured together")
	}
	if hasClientCert && !hasCA {
		return Material{}, fmt.Errorf("sslCa is required when client certificates are configured")
	}

	switch material.TLS {
	case "":
		if hasCA {
			material.TLS = "custom"
		}
	case "true":
		if hasCA {
			material.TLS = "custom"
		}
	case "custom":
		if !hasCA {
			return Material{}, fmt.Errorf("sslCa is required when tls is custom")
		}
	case "false":
		if hasCA || hasClientCert {
			return Material{}, fmt.Errorf("TLS certificates cannot be configured when tls is false")
		}
	case "preferred", "skip-verify":
		if hasCA || hasClientCert {
			return Material{}, fmt.Errorf("TLS certificates cannot be configured when tls is %s", material.TLS)
		}
	default:
		return Material{}, fmt.Errorf("unsupported tls mode %q", material.TLS)
	}

	if hasCA {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(material.CACert) {
			return Material{}, fmt.Errorf("sslCa does not contain a valid PEM certificate")
		}
	}
	if hasClientCert {
		if _, err := tls.X509KeyPair(material.ClientCert, material.ClientKey); err != nil {
			return Material{}, fmt.Errorf("sslCert and sslKey are not a valid key pair: %w", err)
		}
	}

	return material, nil
}

func URL(material Material, instance string) (string, error) {
	normalized, err := Normalize(material)
	if err != nil {
		return "", err
	}

	host := normalized.Host
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	databaseURL := url.URL{
		Scheme:  "mysql",
		Host:    net.JoinHostPort(host, normalized.Port),
		User:    url.UserPassword(normalized.Username, normalized.Password),
		Path:    "/" + normalized.Database,
		RawPath: "/" + url.PathEscape(normalized.Database),
	}
	query := databaseURL.Query()
	if normalized.TLS != "" {
		query.Set("tls", normalized.TLS)
	}
	if len(normalized.CACert) > 0 {
		query.Set("ssl-ca", MountPath(instance)+"/"+CACertFile)
	}
	if len(normalized.ClientCert) > 0 {
		query.Set("ssl-cert", MountPath(instance)+"/"+ClientCertFile)
		query.Set("ssl-key", MountPath(instance)+"/"+ClientKeyFile)
	}
	databaseURL.RawQuery = query.Encode()
	return databaseURL.String(), nil
}

func Write(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	instance string,
	material Material,
) (*apiv2.MysqlConnection, error) {
	normalized, err := Normalize(material)
	if err != nil {
		return nil, err
	}
	connectionURL, err := URL(normalized, instance)
	if err != nil {
		return nil, err
	}

	data := map[string][]byte{
		URLKey:      []byte(connectionURL),
		HostKey:     []byte(normalized.Host),
		PortKey:     []byte(normalized.Port),
		DatabaseKey: []byte(normalized.Database),
		UsernameKey: []byte(normalized.Username),
		PasswordKey: []byte(normalized.Password),
		TLSKey:      []byte(normalized.TLS),
		SSLCAKey:    normalized.CACert,
		SSLCertKey:  normalized.ClientCert,
		SSLKeyKey:   normalized.ClientKey,
	}

	nsName := types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      SecretName(wandb.Name, instance),
	}
	actual := &corev1.Secret{}
	if err := c.Get(ctx, nsName, actual); apierrors.IsNotFound(err) {
		actual = nil
	} else if err != nil {
		return nil, err
	}

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsName.Name,
			Namespace: nsName.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": bundleManagedBy,
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/component":  bundleComponent,
			},
			Annotations: map[string]string{
				BundleVersionAnnotation: BundleVersion,
				InstanceAnnotation:      instance,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := controllerutil.SetOwnerReference(wandb, desired, c.Scheme()); err != nil {
		return nil, err
	}
	if _, err := common.CrudResource(ctx, c, desired, actual); err != nil {
		return nil, err
	}

	return Connection(nsName.Name), nil
}

// DeleteStale removes application bundles for instances that are no longer in
// the W&B spec. Owner UID matching prevents a newly-created W&B resource with
// the same name from deleting bundles that belong to its predecessor.
func DeleteStale(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	desiredInstances map[string]struct{},
) error {
	desiredNames := make(map[string]struct{}, len(desiredInstances))
	for instance := range desiredInstances {
		desiredNames[SecretName(wandb.Name, instance)] = struct{}{}
	}

	secrets := &corev1.SecretList{}
	if err := c.List(
		ctx,
		secrets,
		client.InNamespace(wandb.Namespace),
		client.MatchingLabels(map[string]string{
			"app.kubernetes.io/managed-by": bundleManagedBy,
			"app.kubernetes.io/instance":   wandb.Name,
			"app.kubernetes.io/component":  bundleComponent,
		}),
	); err != nil {
		return err
	}

	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if _, ok := desiredNames[secret.Name]; ok {
			continue
		}
		ownedByCurrentWandb := false
		for _, ref := range secret.OwnerReferences {
			if ref.UID == wandb.UID {
				ownedByCurrentWandb = true
				break
			}
		}
		if !ownedByCurrentWandb {
			continue
		}
		if err := c.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func Read(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, instance string) (*apiv2.MysqlConnection, error) {
	name := SecretName(wandb.Name, instance)
	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: wandb.Namespace, Name: name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return Connection(name), nil
}

func Delete(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, instance string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: wandb.Namespace,
			Name:      SecretName(wandb.Name, instance),
		},
	}
	if err := c.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func Connection(secretName string) *apiv2.MysqlConnection {
	localRef := corev1.LocalObjectReference{Name: secretName}
	selector := func(key string, optional bool) corev1.SecretKeySelector {
		return corev1.SecretKeySelector{
			LocalObjectReference: localRef,
			Key:                  key,
			Optional:             ptr.To(optional),
		}
	}
	return &apiv2.MysqlConnection{
		URL:      selector(URLKey, false),
		Host:     selector(HostKey, false),
		Port:     selector(PortKey, false),
		Database: selector(DatabaseKey, false),
		Username: selector(UsernameKey, false),
		Password: selector(PasswordKey, false),
		Tls:      selector(TLSKey, true),
		SslCa:    selector(SSLCAKey, true),
		SslCert:  selector(SSLCertKey, true),
		SslKey:   selector(SSLKeyKey, true),
	}
}
