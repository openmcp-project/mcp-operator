package gardener

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	apiserverconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"
)

// resolvedAuditLog keeps cross-namespace references internal to the operator.
// It must not be written back to the MCP or its generated APIServer spec.
type resolvedAuditLog struct {
	*openmcpv1alpha1.AuditLogConfig
	secretNamespace string
	policyNamespace string
}

// resolveAuditLog is shared by resource reconciliation and shoot conversion so
// the copied resources and the rendered endpoint always use the same precedence.
func (gc *GardenerConnector) resolveAuditLog(as *openmcpv1alpha1.APIServer) (*resolvedAuditLog, error) {
	if as.Spec.GardenerConfig != nil && as.Spec.GardenerConfig.AuditLog != nil {
		cfg := as.Spec.GardenerConfig.AuditLog
		if err := apiserverconfig.ValidateAuditLogConfig(cfg, field.NewPath("spec", "gardener", "auditLog")).ToAggregate(); err != nil {
			return nil, err
		}
		return &resolvedAuditLog{AuditLogConfig: cfg, secretNamespace: as.Namespace, policyNamespace: as.Namespace}, nil
	}
	if gc.Common != nil && gc.Common.DefaultAuditLog != nil {
		cfg := gc.Common.DefaultAuditLog
		return &resolvedAuditLog{AuditLogConfig: cfg.AuditLogConfig(), secretNamespace: cfg.SecretRef.Namespace, policyNamespace: cfg.PolicyRef.Namespace}, nil
	}
	return nil, nil
}
