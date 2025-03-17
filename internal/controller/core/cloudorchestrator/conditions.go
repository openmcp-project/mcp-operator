package cloudorchestrator

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1beta1 "github.tools.sap/cloud-orchestration/control-plane-operator/api/v1beta1"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	componentutils "github.tools.sap/CoLa/mcp-operator/internal/utils/components"
)

func mcpConditionStatusFromCOConditionStatus(coStatus metav1.ConditionStatus) openmcpv1alpha1.ComponentConditionStatus {
	switch coStatus {
	case metav1.ConditionTrue:
		return openmcpv1alpha1.ComponentConditionStatusTrue
	case metav1.ConditionFalse:
		return openmcpv1alpha1.ComponentConditionStatusFalse
	default:
		return openmcpv1alpha1.ComponentConditionStatusUnknown
	}
}

// cloudOrchestratorConditions builds up the conditions for the CloudOrchestrator
// It copies the passed in conditions and adds the conditions from the ControlPlane resource (if not nil),
// as well as an aggregated 'CloudOrchestratorHealthy' condition.
func cloudOrchestratorConditions(ready bool, reason, message string, cocp *corev1beta1.ControlPlane, cons ...openmcpv1alpha1.ComponentCondition) []openmcpv1alpha1.ComponentCondition {
	resLen := len(cons) + 1
	if cocp != nil {
		resLen += len(cocp.Status.Conditions)
	}
	res := make([]openmcpv1alpha1.ComponentCondition, len(cons), resLen)
	copy(res, cons)

	// iterate over all conditions from the ControlPlane resource and add them to the list
	// additionally, they are all aggregated into the 'CloudOrchestratorHealthy' condition
	// the reason is that some of them are lower-case conditions that are not propagated to the MCP status
	healthy := ready
	healthyMsg := strings.Builder{}
	if cocp != nil {
		for _, con := range cocp.Status.Conditions {
			res = append(res, componentutils.NewCondition(con.Type, mcpConditionStatusFromCOConditionStatus(con.Status), con.Reason, con.Message))
			if con.Status != metav1.ConditionTrue {
				healthy = false
				if healthyMsg.Len() == 0 {
					healthyMsg.WriteString("The following ControlPlane conditions are not 'True':\n")
				}
				healthyMsg.WriteString("\t")
				healthyMsg.WriteString(con.Type)
				healthyMsg.WriteString(": ")
				healthyMsg.WriteString(con.Message)
				healthyMsg.WriteString("\n")
			}
		}
	}
	if !healthy {
		if reason == "" {
			reason = "UnhealthyControlPlaneConditions"
		}
		if healthyMsg.Len() > 0 {
			if message == "" {
				message = healthyMsg.String()
			} else {
				message = fmt.Sprintf("%s\n%s", message, healthyMsg.String())
			}
		}
	}
	res = append(res, componentutils.NewCondition(openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(), openmcpv1alpha1.ComponentConditionStatusFromBool(healthy), reason, message))

	return res
}
