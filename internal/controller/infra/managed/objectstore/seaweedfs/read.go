package seaweedfs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const seaweedProbeTimeout = 5 * time.Second

func ReadState(
	ctx context.Context,
	k8sClient client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule ctrlcommon.OnDeleteRule,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.ObjectStore)
	log := logx.GetSlog(ctx)

	var actualResource = &seaweedv1.Seaweed{}
	conditions := make([]metav1.Condition, 0)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, k8sClient, nsnBuilder.SpecNsName(), ResourceTypeName, actualResource,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   SeaweedCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}
	}
	if !found {
		actualResource = nil
		if onDeleteRule.Policy == ctrlcommon.Purge {
			log.Debug(
				"Attempting to purge associated seaweedfs resources after deletion",
				"seaweedName", SeaweedName(specNamespacedName.Name),
			)
			err = purgeAssociatedResources(ctx, k8sClient, specNamespacedName.Namespace, onDeleteRule.Selector)
			if err != nil {
				conditions = append(
					conditions,
					metav1.Condition{
						Type:   SeaweedCustomResourceType,
						Status: metav1.ConditionUnknown,
						Reason: ctrlcommon.ApiErrorReason,
					},
				)
			} else {
				conditions = append(conditions, metav1.Condition{
					Type:   SeaweedReportedReadyType,
					Status: metav1.ConditionFalse,
					Reason: ctrlcommon.PendingDeleteReason,
				},
				)
			}
		}
	}

	if actualResource != nil {
		readyConditions := computeSeaweedReportedReadyCondition(ctx, actualResource)
		conditions = append(conditions, readyConditions...)
		if readyConditions[0].Status == metav1.ConditionTrue {
			conditions = append(conditions, computeSeaweedWritableCondition(ctx, actualResource))
			conditions = append(conditions, computeSeaweedS3ReachableCondition(ctx, actualResource))
		}
	}
	log.Debug("read", "seaweedName", nsnBuilder.SpecNsName().Name, "namespace", nsnBuilder.SpecNsName().Namespace, "rule", onDeleteRule.Policy)
	return conditions
}

type seaweedAssignResponse struct {
	FID   string `json:"fid"`
	Error string `json:"error"`
}

func computeSeaweedWritableCondition(ctx context.Context, cr *seaweedv1.Seaweed) metav1.Condition {
	scheme := "http"
	transport := http.DefaultTransport
	if cr.Spec.TLS != nil && cr.Spec.TLS.Enabled {
		scheme = "https"
		tlsTransport := http.DefaultTransport.(*http.Transport).Clone()
		// The Seaweed operator generates an internal certificate whose CA is not
		// mounted into this controller. The request never leaves the cluster DNS
		// name and only verifies that the master can allocate writable storage.
		tlsTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402
		transport = tlsTransport
	}

	endpoint := url.URL{
		Scheme: scheme,
		Host: fmt.Sprintf(
			"%s-master.%s.svc.cluster.local:%d",
			cr.Name,
			cr.Namespace,
			seaweedv1.MasterHTTPPort,
		),
		Path: "/dir/assign",
	}
	query := endpoint.Query()
	query.Set("count", "1")
	if cr.Spec.Master != nil && cr.Spec.Master.DefaultReplication != nil {
		query.Set("replication", *cr.Spec.Master.DefaultReplication)
	}
	endpoint.RawQuery = query.Encode()

	return probeSeaweedAllocation(ctx, &http.Client{Transport: transport, Timeout: seaweedProbeTimeout}, endpoint.String())
}

func computeSeaweedS3ReachableCondition(ctx context.Context, cr *seaweedv1.Seaweed) metav1.Condition {
	scheme := "http"
	transport := http.DefaultTransport
	if cr.Spec.TLS != nil && cr.Spec.TLS.Enabled {
		scheme = "https"
		tlsTransport := http.DefaultTransport.(*http.Transport).Clone()
		// The generated internal CA is not mounted into this controller. This
		// unauthenticated probe stays on the cluster-local service address.
		tlsTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402
		transport = tlsTransport
	}
	endpoint := url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s-s3.%s.svc.cluster.local:%s", cr.Name, cr.Namespace, S3Port),
		Path:   "/",
	}
	return probeSeaweedS3(ctx, &http.Client{Transport: transport, Timeout: seaweedProbeTimeout}, endpoint.String())
}

func probeSeaweedS3(ctx context.Context, client *http.Client, endpoint string) metav1.Condition {
	condition := metav1.Condition{Type: SeaweedS3ReachableType, Status: metav1.ConditionFalse, Reason: "EndpointUnavailable"}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		condition.Message = err.Error()
		return condition
	}
	response, err := client.Do(req)
	if err != nil {
		condition.Message = err.Error()
		return condition
	}
	defer response.Body.Close()
	// An unauthenticated S3 service commonly returns 403. Any non-5xx response
	// proves cluster DNS resolved and the S3 API accepted the connection.
	if response.StatusCode >= http.StatusInternalServerError {
		condition.Message = fmt.Sprintf("S3 endpoint returned HTTP %d", response.StatusCode)
		return condition
	}
	condition.Status = metav1.ConditionTrue
	condition.Reason = "EndpointReachable"
	return condition
}

func probeSeaweedAllocation(ctx context.Context, client *http.Client, endpoint string) metav1.Condition {
	condition := metav1.Condition{
		Type:   SeaweedWritableType,
		Status: metav1.ConditionFalse,
		Reason: "AllocationFailed",
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		condition.Message = err.Error()
		return condition
	}
	response, err := client.Do(req)
	if err != nil {
		condition.Message = err.Error()
		return condition
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		condition.Message = err.Error()
		return condition
	}
	var assign seaweedAssignResponse
	if err := json.Unmarshal(body, &assign); err != nil {
		condition.Message = fmt.Sprintf("invalid allocation response: %v", err)
		return condition
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		condition.Message = fmt.Sprintf("allocation returned HTTP %d: %s", response.StatusCode, assign.Error)
		return condition
	}
	if assign.Error != "" {
		condition.Message = assign.Error
		return condition
	}
	if assign.FID == "" {
		condition.Message = "allocation response did not include a file ID"
		return condition
	}

	condition.Status = metav1.ConditionTrue
	condition.Reason = "AllocationSucceeded"
	return condition
}

func computeSeaweedReportedReadyCondition(_ context.Context, cr *seaweedv1.Seaweed) []metav1.Condition {
	if cr == nil {
		return []metav1.Condition{}
	}

	allReady := true
	anyRunning := false

	components := []struct {
		name   string
		status seaweedv1.ComponentStatus
	}{
		{"master", cr.Status.Master},
		{"volume", cr.Status.Volume},
		{"filer", cr.Status.Filer},
	}

	for _, c := range components {
		if c.status.Replicas == 0 {
			continue
		}
		if c.status.ReadyReplicas > 0 {
			anyRunning = true
		}
		if c.status.ReadyReplicas < c.status.Replicas {
			allReady = false
		}
	}

	var status metav1.ConditionStatus
	var reason string

	switch {
	case cr.Status.Filer.Replicas > 0 && cr.Status.Filer.ReadyReplicas == 0:
		status = metav1.ConditionFalse
		reason = "red"
	case allReady && anyRunning:
		status = metav1.ConditionTrue
		reason = "green"
	case anyRunning:
		status = metav1.ConditionFalse
		reason = "yellow"
	default:
		status = metav1.ConditionUnknown
		reason = ctrlcommon.UnknownReason
	}

	return []metav1.Condition{
		{
			Type:   SeaweedReportedReadyType,
			Status: status,
			Reason: reason,
		},
	}
}
