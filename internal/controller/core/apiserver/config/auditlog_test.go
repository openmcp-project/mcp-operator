package config_test

import (
	"context"
	"strings"
	"testing"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	apiserverconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"
)

func validDefaultAuditLog() *apiserverconfig.DefaultAuditLogConfiguration {
	return &apiserverconfig.DefaultAuditLogConfiguration{
		Type: "standard", TenantID: "platform-tenant", ServiceURL: "https://audit.example.com:8081",
		SecretRef: openmcpv1alpha1.NamespacedObjectReference{Name: "audit-credentials", Namespace: "operator-secrets"},
		PolicyRef: openmcpv1alpha1.NamespacedObjectReference{Name: "audit-policy", Namespace: "operator-policies"},
	}
}

func TestDefaultAuditLogValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*apiserverconfig.DefaultAuditLogConfiguration)
		field  string
	}{
		{"valid", func(_ *apiserverconfig.DefaultAuditLogConfiguration) {}, ""},
		{"type", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.Type = "unsupported" }, "type"},
		{"tenant", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.TenantID = " " }, "tenantID"},
		{"URL", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.ServiceURL = "http://audit.example.com" }, "serviceURL"},
		{"URL without host", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.ServiceURL = "https:///audit" }, "serviceURL"},
		{"secret name", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.SecretRef.Name = "" }, "secretRef.name"},
		{"secret namespace", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.SecretRef.Namespace = "" }, "secretRef.namespace"},
		{"policy name", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.PolicyRef.Name = "" }, "policyRef.name"},
		{"policy namespace", func(c *apiserverconfig.DefaultAuditLogConfiguration) { c.PolicyRef.Namespace = "Bad/Namespace" }, "policyRef.namespace"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := validDefaultAuditLog()
			tt.change(c)
			cfg := &apiserverconfig.APIServerProviderConfiguration{CommonConfig: apiserverconfig.CommonConfig{DefaultAuditLog: c}}
			err := apiserverconfig.Validate(cfg)
			_, completeErr := cfg.Complete(context.Background())
			if tt.field == "" {
				if err != nil || completeErr != nil {
					t.Fatalf("valid default rejected: %v / %v", err, completeErr)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.field) || completeErr == nil {
				t.Fatalf("expected validation and completion to reject %s: %v / %v", tt.field, err, completeErr)
			}
		})
	}
}

func TestLoadDefaultAuditLog(t *testing.T) {
	cfg, err := apiserverconfig.LoadConfigFromBytes([]byte(`defaultAuditLog:
  type: standard
  tenantID: platform-tenant
  serviceURL: https://audit.example.com
  secretRef: {name: audit-credentials, namespace: operator-secrets}
  policyRef: {name: audit-policy, namespace: operator-policies}
`))
	if err != nil {
		t.Fatal(err)
	}
	completed, err := cfg.Complete(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if completed.DefaultAuditLog == nil || *completed.DefaultAuditLog != *cfg.DefaultAuditLog {
		t.Fatal("default was not retained")
	}
	cfg.DefaultAuditLog.TenantID = "changed"
	if completed.DefaultAuditLog.TenantID != "platform-tenant" {
		t.Fatal("completed configuration aliases input")
	}
	withoutDefault, err := (&apiserverconfig.APIServerProviderConfiguration{}).Complete(context.Background())
	if err != nil || withoutDefault.DefaultAuditLog != nil {
		t.Fatalf("absent default changed behavior: %v", err)
	}
}

func TestDefaultAuditLogValidationRedactsURLCredentials(t *testing.T) {
	c := validDefaultAuditLog()
	c.ServiceURL = "https://test-user:test-only-password@audit.example.com"
	err := apiserverconfig.Validate(&apiserverconfig.APIServerProviderConfiguration{CommonConfig: apiserverconfig.CommonConfig{DefaultAuditLog: c}})
	if err == nil {
		t.Fatal("credential-bearing URL accepted")
	}
	if strings.Contains(err.Error(), "test-only-password") {
		t.Fatal("validation error disclosed URL credentials")
	}
}
