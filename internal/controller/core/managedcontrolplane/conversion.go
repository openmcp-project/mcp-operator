package managedcontrolplane

import (
	"fmt"
	"maps"
	"slices"

	"github.com/openmcp-project/mcp-operator/internal/components"
	mcpocfg "github.com/openmcp-project/mcp-operator/internal/config"
	componentutils "github.com/openmcp-project/mcp-operator/internal/utils/components"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// ManagedControlPlaneToSplitInternalResources converts the given v1alpha1.ManagedControlPlane into multiple internal resources.
// The returned map contains only those components for which the ManagedControlPlane contains configuration.
func (*ManagedControlPlaneController) ManagedControlPlaneToSplitInternalResources(mcp *openmcpv1alpha1.ManagedControlPlane, icfg *openmcpv1alpha1.InternalConfiguration, ns *corev1.Namespace, scheme *runtime.Scheme, addReconcileAnnotation bool) (map[openmcpv1alpha1.ComponentType]*components.ComponentHandler, error) {
	if mcp == nil {
		return nil, nil
	}

	labels := map[string]string{
		openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName:      mcp.Name,
		openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace: mcp.Namespace,
	}
	if ns != nil && ns.Labels != nil {
		if project, ok := ns.Labels[openmcpv1alpha1.ProjectWorkspaceOperatorProjectLabel]; ok {
			labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject] = project
		}
		if workspace, ok := ns.Labels[openmcpv1alpha1.ProjectWorkspaceOperatorWorkspaceLabel]; ok {
			labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace] = workspace
		}
	}

	res := map[openmcpv1alpha1.ComponentType]*components.ComponentHandler{}
	allCompHandlers := components.Registry.GetKnownComponents()
	for ct, ch := range allCompHandlers {
		if ch != nil && ch.Resource() != nil && ch.Converter() != nil && ch.Converter().IsConfigured(mcp) {
			ch.Resource().SetName(mcp.Name)
			ch.Resource().SetNamespace(mcp.Namespace)

			spec, err := ch.Converter().ConvertToResourceSpec(mcp, icfg)
			if err != nil {
				return nil, fmt.Errorf("error converting configuration for component '%s' into spec for that component's resource: %w", string(ct), err)
			}
			if err := ch.Resource().SetSpec(spec); err != nil {
				return nil, fmt.Errorf("internal error: the spec for component '%s' cannot be passed into the resource for this component", string(ct))
			}

			if componentIsDisabled(mcp, ct) {
				ch.Resource().SetAnnotations(map[string]string{
					openmcpv1alpha1.OperationAnnotation: openmcpv1alpha1.OperationAnnotationValueIgnore,
				})
			} else if addReconcileAnnotation {
				ch.Resource().SetAnnotations(map[string]string{
					openmcpv1alpha1.OperationAnnotation: openmcpv1alpha1.OperationAnnotationValueReconcile,
				})
			}

			// take over architecture version label from the MCP resource, if override is allowed for the component
			bridgeConfig := mcpocfg.Config.Architecture.GetBridgeConfigForComponent(ct)
			cLabels := make(map[string]string, len(labels)+1)
			maps.Copy(cLabels, labels)
			v, found := mcp.Labels[ct.ArchitectureVersionLabel()]
			if found {
				// check if version override is allowed for this component
				if !bridgeConfig.AllowOverride {
					return nil, fmt.Errorf("architecture version override is not allowed for component '%s', remove the '%s' label", string(ct), ct.ArchitectureVersionLabel())
				}
				if !bridgeConfig.IsAllowedVersion(v) {
					return nil, fmt.Errorf("architecture version '%s' is not allowed for component '%s'", v, string(ct))
				}
				cLabels[openmcpv1alpha1.ArchitectureVersionLabel] = v
			} else {
				cLabels[openmcpv1alpha1.ArchitectureVersionLabel] = bridgeConfig.Version
			}

			ch.Resource().SetLabels(cLabels)

			componentutils.SetCreatedFromGeneration(ch.Resource(), mcp, icfg)
			if err := controllerutil.SetControllerReference(mcp, ch.Resource(), scheme); err != nil {
				return nil, fmt.Errorf("unable to set owner reference: %w", err)
			}
			res[ct] = ch
		}
	}

	return res, nil
}

// componentIsDisabled returns true if the given component type is disabled in the given managedcontrolplane's spec.
func componentIsDisabled(mcp *openmcpv1alpha1.ManagedControlPlane, ct openmcpv1alpha1.ComponentType) bool {
	if mcp == nil || len(mcp.Spec.DisabledComponents) == 0 {
		return false
	}

	return slices.Contains(mcp.Spec.DisabledComponents, ct)
}
