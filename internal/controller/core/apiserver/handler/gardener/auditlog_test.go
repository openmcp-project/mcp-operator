package gardener

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	apiserverconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"
)

func TestResolveAuditLog(t *testing.T) {
	fallback := &apiserverconfig.DefaultAuditLogConfiguration{
		Type: "standard", TenantID: "platform-tenant", ServiceURL: "https://platform.example.com",
		SecretRef: openmcpv1alpha1.NamespacedObjectReference{Name: "central-secret", Namespace: "operator-secrets"},
		PolicyRef: openmcpv1alpha1.NamespacedObjectReference{Name: "central-policy", Namespace: "operator-policies"},
	}
	explicit := &openmcpv1alpha1.AuditLogConfig{
		Type: "standard", TenantID: "customer-tenant", ServiceURL: "https://customer.example.com",
		SecretRef: corev1.LocalObjectReference{Name: "customer-secret"},
		PolicyRef: corev1.LocalObjectReference{Name: "customer-policy"},
	}
	for _, tt := range []struct {
		name                                     string
		gardener                                 *openmcpv1alpha1.GardenerConfiguration
		defaultConfig                            *apiserverconfig.DefaultAuditLogConfiguration
		tenant, secretNamespace, policyNamespace string
		wantError                                bool
	}{
		{"absent", nil, nil, "", "", "", false},
		{"default with no gardener block", nil, fallback, "platform-tenant", "operator-secrets", "operator-policies", false},
		{"default with no audit block", &openmcpv1alpha1.GardenerConfiguration{}, fallback, "platform-tenant", "operator-secrets", "operator-policies", false},
		{"explicit wins", &openmcpv1alpha1.GardenerConfiguration{AuditLog: explicit}, fallback, "customer-tenant", "customer", "customer", false},
		{"explicit without default", &openmcpv1alpha1.GardenerConfiguration{AuditLog: explicit}, nil, "customer-tenant", "customer", "customer", false},
		{"invalid explicit does not fall back", &openmcpv1alpha1.GardenerConfiguration{AuditLog: &openmcpv1alpha1.AuditLogConfig{}}, fallback, "", "", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			as := &openmcpv1alpha1.APIServer{ObjectMeta: metav1.ObjectMeta{Namespace: "customer"}, Spec: openmcpv1alpha1.APIServerSpec{APIServerConfiguration: openmcpv1alpha1.APIServerConfiguration{GardenerConfig: tt.gardener}}}
			before := as.DeepCopy()
			gc := &GardenerConnector{Common: &apiserverconfig.CompletedCommonConfig{DefaultAuditLog: tt.defaultConfig}}
			actual, err := gc.resolveAuditLog(as)
			if (err != nil) != tt.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.tenant == "" {
				if actual != nil {
					t.Fatal("unexpected audit configuration")
				}
			} else if actual == nil || actual.TenantID != tt.tenant || actual.secretNamespace != tt.secretNamespace || actual.policyNamespace != tt.policyNamespace {
				t.Fatalf("unexpected resolution: %+v", actual)
			}
			if !reflect.DeepEqual(before, as) {
				t.Fatal("resolution mutated the APIServer")
			}
		})
	}
}
