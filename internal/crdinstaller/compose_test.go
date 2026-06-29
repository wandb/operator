/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package crdinstaller

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

var validOpts = Options{
	CertInjectReference:     "wandb/test-serving-cert",
	WebhookServiceName:      "test-wandb-operator",
	WebhookServiceNamespace: "wandb",
}

func TestParseGroupsValid(t *testing.T) {
	got, err := ParseGroups("redis,clickhouse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "redis" || got[1] != "clickhouse" {
		t.Errorf("got %v, want [redis clickhouse]", got)
	}
}

func TestParseGroupsRejectsUnknown(t *testing.T) {
	_, err := ParseGroups("redis,bogus")
	if err == nil {
		t.Fatal("expected error for unknown group")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("expected error mentioning %q, got %v", "bogus", err)
	}
}

func TestParseGroupsEmpty(t *testing.T) {
	got, err := ParseGroups("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestOptionsValidate(t *testing.T) {
	if err := validOpts.Validate(); err != nil {
		t.Fatalf("valid opts rejected: %v", err)
	}
	bad := validOpts
	bad.CertInjectReference = ""
	if err := bad.Validate(); err == nil {
		t.Error("expected error for missing CertInjectReference")
	}
}

func TestComposeOperatorOnly(t *testing.T) {
	crds, err := compose(validOpts)
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if len(crds) != 2 {
		t.Fatalf("expected 2 operator CRDs, got %d", len(crds))
	}
	for _, crd := range crds {
		if got := crd.Annotations["cert-manager.io/inject-ca-from"]; got != validOpts.CertInjectReference {
			t.Errorf("%s: cert-manager annotation = %q, want %q", crd.Name, got, validOpts.CertInjectReference)
		}
		if conv := crd.Spec.Conversion; conv != nil && conv.Webhook != nil && conv.Webhook.ClientConfig != nil && conv.Webhook.ClientConfig.Service != nil {
			svc := conv.Webhook.ClientConfig.Service
			if svc.Name != validOpts.WebhookServiceName || svc.Namespace != validOpts.WebhookServiceNamespace {
				t.Errorf("%s: webhook service = %s/%s, want %s/%s",
					crd.Name, svc.Namespace, svc.Name, validOpts.WebhookServiceNamespace, validOpts.WebhookServiceName)
			}
		}
	}
}

func TestComposeIncludesOptionalGroup(t *testing.T) {
	opts := validOpts
	opts.Groups = []string{"redis"}
	crds, err := compose(opts)
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if len(crds) <= 2 {
		t.Fatalf("expected >2 CRDs when redis group included, got %d", len(crds))
	}
	// Redis CRDs must NOT have the cert-manager annotation we inject for operator CRDs.
	for _, crd := range crds {
		if strings.HasSuffix(crd.Name, ".redis.redis.opstreelabs.in") {
			if _, ok := crd.Annotations["cert-manager.io/inject-ca-from"]; ok {
				t.Errorf("redis CRD %s should not have cert-manager annotation", crd.Name)
			}
		}
	}
}

func TestComposeIncludesClickHouseGroup(t *testing.T) {
	opts := validOpts
	opts.Groups = []string{"clickhouse"}
	crds, err := compose(opts)
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	names := make(map[string]bool, len(crds))
	for _, crd := range crds {
		names[crd.Name] = true
	}
	for _, name := range []string{
		"clickhouseinstallations.clickhouse.altinity.com",
		"clickhouseinstallationtemplates.clickhouse.altinity.com",
		"clickhouseoperatorconfigurations.clickhouse.altinity.com",
		"clickhousekeeperinstallations.clickhouse-keeper.altinity.com",
	} {
		if !names[name] {
			t.Errorf("expected ClickHouse CRD %s to be included", name)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	opts := validOpts
	opts.Groups = []string{"redis", "clickhouse"}
	var a, b bytes.Buffer
	if err := Render(context.Background(), opts, &a); err != nil {
		t.Fatalf("first render: %v", err)
	}
	if err := Render(context.Background(), opts, &b); err != nil {
		t.Fatalf("second render: %v", err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("render output differs between invocations")
	}
}
