package preflight

import (
	"context"
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
)

// RunExternalMysqlInClusterCheck dials the MySQL host from a pod in the CR's namespace.  externalMysqlCheck runs
// from wsm's machine, which can't prove the app's network path (cluster DNS, VPC-only hosts, egress policies).
func RunExternalMysqlInClusterCheck(ctx context.Context, conn *apiv2.MysqlConnection, env Env) Result {
	res := func(o Outcome, msg string) Result {
		return Result{Check: ExternalMysqlInClusterCheck, Outcome: o, Message: msg}
	}
	if env.Client == nil {
		return res(OutcomeSkipped, "creating probe pods is not enabled")
	}
	// Only host and port: the pod spec is readable by anyone with pod get, so no credentials go in it.
	v, err := resolveFields(ctx, env.Resolve, map[string]apiv2.ValueOrSecret{"host": conn.Host, "port": conn.Port})
	if err != nil {
		return resolveFailure(ExternalMysqlInClusterCheck, err)
	}
	if v["host"] == "" || !validPort(v["port"]) {
		return res(OutcomeFail, "host and a valid port are required")
	}

	// Values go through env vars, never into the shell string, so a crafted host can't inject commands.
	pod := newProbePod(env.Namespace, ExternalMysqlInClusterCheck, []string{"sh", "-c", `nc -z -w 5 "$HOST" "$PORT"`}, []corev1.EnvVar{{Name: "HOST", Value: v["host"]}, {Name: "PORT", Value: v["port"]}})
	phase, msg, err := runProbePod(ctx, env.Client, pod)
	target := fmt.Sprintf("%s:%s", v["host"], v["port"])
	switch {
	case err != nil:
		return res(OutcomeUnknown, fmt.Sprintf("probe pod in namespace %q: %v", env.Namespace, err))
	case phase == corev1.PodSucceeded:
		return res(OutcomePass, fmt.Sprintf("%s reachable from a pod in namespace %q", target, env.Namespace))
	default:
		return res(OutcomeFail, fmt.Sprintf("%s not reachable from a pod in namespace %q: %s", target, env.Namespace, strings.TrimSpace(msg)))
	}
}
