package cloudorchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1beta1 "github.tools.sap/cloud-orchestration/control-plane-operator/api/v1beta1"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

func Test_convertToControlPlaneSpec(t *testing.T) {
	apiServerStatus := &openmcpv1alpha1.APIServerStatus{
		AdminAccess: &openmcpv1alpha1.APIServerAccess{
			Kubeconfig: testKubeConfig,
		},
	}

	tests := []struct {
		name         string
		input        *openmcpv1alpha1.CloudOrchestratorSpec
		validateFunc func(*corev1beta1.ControlPlaneSpec) error
		expectedErr  error
	}{
		{
			name: "Crossplane enabled through non nil pointer - everything else disabled",
			input: &openmcpv1alpha1.CloudOrchestratorSpec{
				CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
					Crossplane: &openmcpv1alpha1.CrossplaneConfig{
						Version: "1.0.0",
					},
				},
			},
			validateFunc: func(spec *corev1beta1.ControlPlaneSpec) error {
				assert.NotNil(t, spec.Crossplane)
				assert.Nil(t, spec.Crossplane.Providers)
				assert.Nil(t, spec.ExternalSecretsOperator)
				assert.Nil(t, spec.Flux)
				assert.Nil(t, spec.BTPServiceOperator)
				assert.Nil(t, spec.Kyverno)
				assert.Nil(t, spec.CertManager)
				return nil
			},
		},
		{
			name: "All Components are enabled through non nil pointer",
			input: &openmcpv1alpha1.CloudOrchestratorSpec{
				CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
					Crossplane: &openmcpv1alpha1.CrossplaneConfig{
						Version: "1.0.0",
						Providers: []*openmcpv1alpha1.CrossplaneProviderConfig{
							{
								Name:    "provider1",
								Version: "1.0.0",
							},
						},
					},
					ExternalSecretsOperator: &openmcpv1alpha1.ExternalSecretsOperatorConfig{
						Version: "1.0.0",
					},
					Flux: &openmcpv1alpha1.FluxConfig{
						Version: "1.0.0",
					},
					BTPServiceOperator: &openmcpv1alpha1.BTPServiceOperatorConfig{
						Version: "1.0.0",
					},
					Kyverno: &openmcpv1alpha1.KyvernoConfig{
						Version: "1.0.0",
					},
				},
			},
			validateFunc: func(spec *corev1beta1.ControlPlaneSpec) error {
				assert.NotNil(t, spec.Crossplane)
				assert.NotNil(t, spec.Crossplane.Providers)
				assert.Len(t, spec.Crossplane.Providers, 1)
				assert.NotNil(t, spec.ExternalSecretsOperator)
				assert.NotNil(t, spec.Flux)
				assert.NotNil(t, spec.BTPServiceOperator)
				assert.NotNil(t, spec.Kyverno)
				assert.NotNil(t, spec.CertManager)
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := convertToControlPlaneSpec(tt.input, apiServerStatus)
			assert.ErrorIs(t, err, tt.expectedErr)
			if err := tt.validateFunc(spec); err != nil {
				t.Errorf("convertToControlPlaneSpec() = %v, want %v", err, "no error")
			}
		})

	}
}

const testKubeConfig = `
apiVersion: v1
clusters:
- name: apiserver
cluster:
  server: https://apiserver.dummy
  certificate-authority-data: ZHVtbXkK
contexts:
- name: apiserver
context:
  cluster: apiserver
  user: apiserver
current-context: apiserver
users:
- name: apiserver
user:
  client-certificate-data: ZHVtbXkK
  client-key-data: ZHVtbXkK
`
