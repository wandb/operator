package bufstream

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

// NsNameBuilder derives the names of all resources that make up a managed
// Bufstream deployment from the base Kafka spec name/namespace.
type NsNameBuilder struct {
	baseNsName types.NamespacedName
}

func CreateNsNameBuilder(baseNsName types.NamespacedName) *NsNameBuilder {
	return &NsNameBuilder{baseNsName: baseNsName}
}

func (n *NsNameBuilder) Namespace() string {
	return n.baseNsName.Namespace
}

func (n *NsNameBuilder) SpecName() string {
	return n.baseNsName.Name
}

// BufstreamName is the broker Deployment/Service name and the Kafka bootstrap host.
func (n *NsNameBuilder) BufstreamName() string {
	return n.SpecName()
}

func (n *NsNameBuilder) BufstreamNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.BufstreamName()}
}

// BufstreamHost is the in-cluster DNS name clients use to reach the brokers.
func (n *NsNameBuilder) BufstreamHost() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", n.BufstreamName(), n.Namespace())
}

// ServiceAccountName is the shared etcd/Bufstream identity for the SCC grant.
func (n *NsNameBuilder) ServiceAccountName() string {
	return n.SpecName()
}

func (n *NsNameBuilder) ServiceAccountNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.ServiceAccountName()}
}

// SccRoleBindingName grants the Kafka SA use of nonroot-v2 (OpenShift only).
func (n *NsNameBuilder) SccRoleBindingName() string {
	return fmt.Sprintf("%s-scc-nonroot-v2", n.SpecName())
}

func (n *NsNameBuilder) SccRoleBindingNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.SccRoleBindingName()}
}

func (n *NsNameBuilder) ConfigMapName() string {
	return fmt.Sprintf("%s-config", n.SpecName())
}

func (n *NsNameBuilder) ConfigMapNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.ConfigMapName()}
}

func (n *NsNameBuilder) CredentialsName() string {
	return fmt.Sprintf("%s-storage-credentials", n.SpecName())
}

func (n *NsNameBuilder) CredentialsNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.CredentialsName()}
}

func (n *NsNameBuilder) EtcdName() string {
	return fmt.Sprintf("%s-etcd", n.SpecName())
}

func (n *NsNameBuilder) EtcdNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.EtcdName()}
}

// EtcdHost is the headless Service domain that governs the etcd StatefulSet.
// Individual members are addressed as <pod>.<EtcdHost>.
func (n *NsNameBuilder) EtcdHost() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", n.EtcdName(), n.Namespace())
}

// EtcdPodName is the stable StatefulSet pod name for member ordinal i. It
// doubles as the etcd member name (ETCD_NAME).
func (n *NsNameBuilder) EtcdPodName(i int) string {
	return fmt.Sprintf("%s-%d", n.EtcdName(), i)
}

// EtcdPodFQDN is the per-pod stable DNS name for member ordinal i.
func (n *NsNameBuilder) EtcdPodFQDN(i int) string {
	return fmt.Sprintf("%s.%s", n.EtcdPodName(i), n.EtcdHost())
}

// EtcdInitialCluster renders the static ETCD_INITIAL_CLUSTER membership list
// (member=peerURL pairs) for a cluster of the given size.
func (n *NsNameBuilder) EtcdInitialCluster(replicas, peerPort int) string {
	members := make([]string, 0, replicas)
	for i := 0; i < replicas; i++ {
		members = append(members, fmt.Sprintf("%s=http://%s:%d", n.EtcdPodName(i), n.EtcdPodFQDN(i), peerPort))
	}
	return strings.Join(members, ",")
}

// EtcdClientEndpoints lists the per-member client URLs that the broker connects
// to, giving it every endpoint for failover rather than a single VIP.
func (n *NsNameBuilder) EtcdClientEndpoints(replicas, clientPort int) []string {
	endpoints := make([]string, 0, replicas)
	for i := 0; i < replicas; i++ {
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", n.EtcdPodFQDN(i), clientPort))
	}
	return endpoints
}

func (n *NsNameBuilder) ConnectionName() string {
	return fmt.Sprintf("%s-connection", n.SpecName())
}

func (n *NsNameBuilder) ConnectionNsName() types.NamespacedName {
	return types.NamespacedName{Namespace: n.Namespace(), Name: n.ConnectionName()}
}

func createNsNameBuilder(baseNsName types.NamespacedName) *NsNameBuilder {
	return CreateNsNameBuilder(baseNsName)
}
