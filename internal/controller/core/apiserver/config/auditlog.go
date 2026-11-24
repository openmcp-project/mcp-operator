package config

import (
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// DefaultAuditLogConfiguration is an operator-only fallback for Gardener API servers
// without explicit audit logging. Its source resources live on the crate cluster.
// Unlike customer references, these references may select an operator-controlled namespace.
type DefaultAuditLogConfiguration struct {
	Type       string                                    `json:"type"`
	TenantID   string                                    `json:"tenantID"`
	ServiceURL string                                    `json:"serviceURL"`
	SecretRef  openmcpv1alpha1.NamespacedObjectReference `json:"secretRef"`
	PolicyRef  openmcpv1alpha1.NamespacedObjectReference `json:"policyRef"`
}

// AuditLogConfig returns the endpoint and local resource names without changing the default.
func (cfg *DefaultAuditLogConfiguration) AuditLogConfig() *openmcpv1alpha1.AuditLogConfig {
	return &openmcpv1alpha1.AuditLogConfig{
		Type: cfg.Type, TenantID: cfg.TenantID, ServiceURL: cfg.ServiceURL,
		SecretRef: corev1.LocalObjectReference{Name: cfg.SecretRef.Name},
		PolicyRef: corev1.LocalObjectReference{Name: cfg.PolicyRef.Name},
	}
}

func (cfg *DefaultAuditLogConfiguration) validate(fldPath *field.Path) field.ErrorList {
	if cfg == nil {
		return nil
	}
	errs := ValidateAuditLogConfig(cfg.AuditLogConfig(), fldPath)
	for name, namespace := range map[string]string{"secretRef": cfg.SecretRef.Namespace, "policyRef": cfg.PolicyRef.Namespace} {
		path := fldPath.Child(name, "namespace")
		if namespace == "" {
			errs = append(errs, field.Required(path, "source namespace must be specified"))
		} else if problems := validation.IsDNS1123Label(namespace); len(problems) > 0 {
			errs = append(errs, field.Invalid(path, namespace, strings.Join(problems, ", ")))
		}
	}
	return errs
}

// ValidateAuditLogConfig rejects incomplete explicit configurations as well as
// invalid defaults. An explicit configuration must never silently fall back.
func ValidateAuditLogConfig(cfg *openmcpv1alpha1.AuditLogConfig, fldPath *field.Path) field.ErrorList {
	if cfg == nil {
		return nil
	}
	var errs field.ErrorList
	if cfg.Type != "standard" {
		errs = append(errs, field.NotSupported(fldPath.Child("type"), cfg.Type, []string{"standard"}))
	}
	if strings.TrimSpace(cfg.TenantID) == "" {
		errs = append(errs, field.Required(fldPath.Child("tenantID"), "tenant ID must be specified"))
	}
	u, err := url.Parse(cfg.ServiceURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		// Do not include a potentially credential-bearing URL in validation errors.
		errs = append(errs, field.Invalid(fldPath.Child("serviceURL"), "<redacted>", "must be an absolute HTTPS URL without credentials or fragment"))
	}
	for name, ref := range map[string]corev1.LocalObjectReference{"secretRef": cfg.SecretRef, "policyRef": cfg.PolicyRef} {
		path := fldPath.Child(name, "name")
		if ref.Name == "" {
			errs = append(errs, field.Required(path, "source resource name must be specified"))
		} else if problems := validation.IsDNS1123Subdomain(ref.Name); len(problems) > 0 {
			errs = append(errs, field.Invalid(path, ref.Name, strings.Join(problems, ", ")))
		}
	}
	return errs
}
