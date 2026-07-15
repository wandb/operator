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

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/wandb/operator/internal/crdinstaller"
	"github.com/wandb/operator/internal/logx"
)

// crd-installer installs Custom Resource Definitions used by the wandb
// operator and its optional subcharts. It is invoked as a pre-install /
// pre-upgrade Helm hook from the operator chart.
//
// Subcommands:
//
//	render   compose the CRDs and emit YAML to stdout (debugging / dry-run)
//	apply    compose the CRDs and server-side apply them to the cluster
//
// Every flag also accepts the matching UPPER_SNAKE_CASE env var. Required
// flags differ per subcommand — see usage.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "render":
		os.Exit(runRender(args))
	case "apply":
		os.Exit(runApply(args))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", sub)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `crd-installer — install operator CRDs (server-side apply) or render them to stdout.

usage:
  crd-installer render [flags]
  crd-installer apply  [flags]

flags (each also accepts the matching UPPER_SNAKE_CASE env var, e.g. CERT_INJECT_REFERENCE):
  --cert-inject-reference     value for cert-manager.io/inject-ca-from on operator CRDs (required)
  --webhook-service-name      conversion webhook service name on operator CRDs (required)
  --webhook-service-namespace conversion webhook service namespace on operator CRDs (required)
  --groups                    comma-separated optional CRD groups to include (e.g. "redis,kafka")
`)
}

type cliOpts struct {
	opts      crdinstaller.Options
	groupsRaw string
	logLevel  string
	logFormat string
}

func registerFlags(fs *flag.FlagSet, c *cliOpts) {
	fs.StringVar(&c.opts.CertInjectReference, "cert-inject-reference", "", "value for cert-manager.io/inject-ca-from on operator CRDs")
	fs.StringVar(&c.opts.WebhookServiceName, "webhook-service-name", "", "conversion webhook service name on operator CRDs")
	fs.StringVar(&c.opts.WebhookServiceNamespace, "webhook-service-namespace", "", "conversion webhook service namespace on operator CRDs")
	fs.StringVar(&c.groupsRaw, "groups", "", `comma-separated optional CRD groups to include (e.g. "redis,kafka")`)
	fs.StringVar(&c.logLevel, "log-level", "info", "log level: debug, info, warn, error")
	fs.StringVar(&c.logFormat, "log-format", "text", "log format: text or json")
}

// applyEnvDefaults populates any flag whose user-supplied value is empty
// from the matching UPPER_SNAKE_CASE env var. Mirrors the manager binary's
// setFlagsFromEnvironment but only fills empties, so explicit CLI flags win.
func applyEnvDefaults(fs *flag.FlagSet) error {
	var firstErr error
	fs.VisitAll(func(f *flag.Flag) {
		if firstErr != nil {
			return
		}
		// Only override if the flag is still at its zero/default value.
		if f.Value.String() != f.DefValue {
			return
		}
		envKey := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v, ok := os.LookupEnv(envKey); ok {
			if err := fs.Set(f.Name, v); err != nil {
				firstErr = fmt.Errorf("setting --%s from %s: %w", f.Name, envKey, err)
			}
		}
	})
	return firstErr
}

func parseSubcommand(name string, args []string) (*cliOpts, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	c := &cliOpts{}
	registerFlags(fs, c)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if err := applyEnvDefaults(fs); err != nil {
		return nil, err
	}
	groups, err := crdinstaller.ParseGroups(c.groupsRaw)
	if err != nil {
		return nil, err
	}
	c.opts.Groups = groups
	return c, nil
}

func setupLogger(c *cliOpts) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(c.logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logx.SetOptions(&logx.Options{
		HandlerOptions: &slog.HandlerOptions{Level: level},
		// Logs go to stderr so `render`'s stdout stays a clean YAML stream.
		Output: os.Stderr,
		Format: logx.LogFormat(c.logFormat),
	})
	return logx.NewSlogLogger("crd-installer")
}

func runRender(args []string) int {
	c, err := parseSubcommand("render", args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx, stop := signalContext()
	defer stop()
	if err := crdinstaller.Render(ctx, c.opts, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "render failed: %v\n", err)
		return 1
	}
	return 0
}

func runApply(args []string) int {
	c, err := parseSubcommand("apply", args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	logger := setupLogger(c)
	ctx, stop := signalContext()
	defer stop()

	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error("loading kubeconfig", "err", err)
		return 1
	}
	client, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		logger.Error("constructing apiextensions client", "err", err)
		return 1
	}
	if err := crdinstaller.Apply(ctx, c.opts, client, logger); err != nil {
		logger.Error("apply failed", "err", err)
		return 1
	}
	return 0
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
