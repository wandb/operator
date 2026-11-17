package preflight

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"github.com/wandb/operator/pkg/wandb/sizing"
	corev1 "k8s.io/api/core/v1"
)

// RunClusterCapacityCheck compares the manifest's requests for spec.size against the  allocatable of the nodes
// the CR can schedule on. It ignores pods already running, so a pass is necessary, not sufficient.
func RunClusterCapacityCheck(ctx context.Context, spec *apiv2.WeightsAndBiasesSpec, env Env) Result {
	res := func(o Outcome, msg string) Result {
		return Result{Check: ClusterCapacityCheck, Outcome: o, Message: msg}
	}
	if env.Reader == nil {
		return res(OutcomeSkipped, "cluster reads are not available")
	}

	mctx, cancel := context.WithTimeout(ctx, serverManifestTimeout)
	defer cancel()
	mfst, err := manifest.GetServerManifest(mctx, effectiveManifestRepository(spec),
		strings.TrimSpace(spec.Wandb.Version), env.RegistryAuth)
	if err != nil {
		// serverManifestCheck reports why; this check just can't be evaluated.
		return res(OutcomeSkipped, "server manifest not loaded; see serverManifestCheck")
	}
	demand := sizing.EstimateDemand(&apiv2.WeightsAndBiases{Spec: *spec}, mfst)

	nodes := &corev1.NodeList{}
	if err := env.Reader.List(ctx, nodes); err != nil {
		// Namespace-scoped users get forbidden on Nodes: that's unverified, not missing capacity.
		return res(OutcomeUnknown, fmt.Sprintf("list nodes: %v", err))
	}

	var tolerations []corev1.Toleration
	if spec.Tolerations != nil {
		tolerations = *spec.Tolerations
	}
	var usable []corev1.Node
	var haveCPU, haveMem int64
	for _, n := range nodes.Items {
		if schedulableFor(n, tolerations) {
			usable = append(usable, n)
			haveCPU += n.Status.Allocatable.Cpu().MilliValue()
			haveMem += n.Status.Allocatable.Memory().Value()
		}
	}
	if len(usable) == 0 {
		return res(OutcomeFail, fmt.Sprintf("none of %d nodes is ready, schedulable and tolerated by spec.tolerations", len(nodes.Items)))
	}

	var needCPU, needMem int64
	var pods int32
	var misfits []string
	for _, d := range demand {
		needCPU += d.CPU.MilliValue() * int64(d.Replicas)
		needMem += d.Memory.Value() * int64(d.Replicas)
		pods += d.Replicas
		if !fitsSomeNode(d, usable) {
			misfits = append(misfits, fmt.Sprintf("%s (%s CPU, %s)",
				d.Component, d.CPU.String(), d.Memory.String()))
		}
	}

	size := spec.Size
	if size == "" {
		size = "default"
	}
	summary := fmt.Sprintf("size %q needs %.1f CPU / %.1fGi across %d pods; %d nodes  allocate %.1f CPU / %.1fGi", size, float64(needCPU)/1000, gib(needMem), pods, len(usable), float64(haveCPU)/1000, gib(haveMem))
	switch {
	case len(misfits) > 0:
		return res(OutcomeFail, summary+"; no single node fits: "+strings.Join(misfits, ", "))
	case needCPU > haveCPU || needMem > haveMem:
		return res(OutcomeFail, summary+"; add nodes or choose a smaller spec.size")
	default:
		return res(OutcomePass, summary)
	}
}

func fitsSomeNode(d sizing.PodDemand, nodes []corev1.Node) bool {
	return slices.ContainsFunc(nodes, func(n corev1.Node) bool {
		return d.CPU.MilliValue() <= n.Status.Allocatable.Cpu().MilliValue() &&
			d.Memory.Value() <= n.Status.Allocatable.Memory().Value()
	})
}

func schedulableFor(n corev1.Node, tolerations []corev1.Toleration) bool {
	if n.Spec.Unschedulable {
		return false
	}
	ready := slices.ContainsFunc(n.Status.Conditions, func(c corev1.NodeCondition) bool {
		return c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue
	})
	if !ready {
		return false
	}
	for _, t := range n.Spec.Taints {
		if t.Effect == corev1.TaintEffectPreferNoSchedule {
			continue
		}
		if !slices.ContainsFunc(tolerations, func(tol corev1.Toleration) bool {
			return tolerates(tol, t)
		}) {
			return false
		}
	}
	return true
}

func tolerates(tol corev1.Toleration, t corev1.Taint) bool {
	if tol.Effect != "" && tol.Effect != t.Effect {
		return false
	}
	if tol.Operator == corev1.TolerationOpExists {
		return tol.Key == "" || tol.Key == t.Key
	}
	return tol.Key == t.Key && tol.Value == t.Value
}
func gib(b int64) float64 { return float64(b) / (1 << 30) }
