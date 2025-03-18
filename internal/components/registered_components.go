package components

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcpinstall "github.com/openmcp-project/mcp-operator/api/install"
)

const (
	LandscaperNamespaceScopedAdminMatchLabel      = "rbac.landscaper.gardener.cloud/aggregate-to-admin"
	LandscaperNamespaceScopedViewMatchLabel       = "rbac.landscaper.gardener.cloud/aggregate-to-view"
	CrossPlaneClusterScopedAdminMatchLabel        = "rbac.crossplane.io/aggregate-to-admin"
	CrossPlaneClusterScopedViewMatchLabel         = "rbac.crossplane.io/aggregate-to-view"
	CloudOrchestratorClusterScopedAdminMatchLabel = "core.orchestrate.cloud.sap/aggregate-to-admin"
	CloudOrchestratorClusterScopedViewMatchLabel  = "core.orchestrate.cloud.sap/aggregate-to-view"
	MatchLabelValue                               = "true"
)

var Registry *registry

func init() {
	colaScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(colaScheme))
	utilruntime.Must(apiextv1.AddToScheme(colaScheme))
	openmcpinstall.Install(colaScheme)

	Registry = newRegistry(colaScheme)
	Registry.Register(openmcpv1alpha1.APIServerComponent, func() *ComponentHandler {
		return NewComponentHandler(&openmcpv1alpha1.APIServer{}, &APIServerConverter{}, nil)
	})
	Registry.Register(openmcpv1alpha1.LandscaperComponent, func() *ComponentHandler {
		return NewComponentHandler(&openmcpv1alpha1.Landscaper{}, &LandscaperConverter{}, func(roleName string) []metav1.LabelSelector {
			if openmcpv1alpha1.IsClusterScopedRole(roleName) {
				// Landscaper uses only namespace-scoped resources
				return nil
			}
			if openmcpv1alpha1.IsAdminRole(roleName) {
				return []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							LandscaperNamespaceScopedAdminMatchLabel: MatchLabelValue,
						},
					},
				}
			}
			return []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						LandscaperNamespaceScopedViewMatchLabel: MatchLabelValue,
					},
				},
			}
		})
	})
	Registry.Register(openmcpv1alpha1.CloudOrchestratorComponent, func() *ComponentHandler {
		return NewComponentHandler(&openmcpv1alpha1.CloudOrchestrator{}, &CloudOrchestratorConverter{}, func(roleName string) []metav1.LabelSelector {
			if openmcpv1alpha1.IsClusterScopedRole(roleName) {
				if openmcpv1alpha1.IsAdminRole(roleName) {
					return []metav1.LabelSelector{
						{
							MatchLabels: map[string]string{
								CrossPlaneClusterScopedAdminMatchLabel: MatchLabelValue, // Crossplane admin role
							},
						},
						{
							MatchLabels: map[string]string{
								CloudOrchestratorClusterScopedAdminMatchLabel: MatchLabelValue, // CO Components admin role
							},
						},
					}
				}
				return []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							CrossPlaneClusterScopedViewMatchLabel: MatchLabelValue, // Crossplane view role
						},
					},
					{
						MatchLabels: map[string]string{
							CloudOrchestratorClusterScopedViewMatchLabel: MatchLabelValue, // CO Components view role
						},
					},
				}
			}
			return nil
		})
	})
	Registry.Register(openmcpv1alpha1.AuthenticationComponent, func() *ComponentHandler {
		return NewComponentHandler(&openmcpv1alpha1.Authentication{}, &AuthenticationConverter{}, nil)
	})
	Registry.Register(openmcpv1alpha1.AuthorizationComponent, func() *ComponentHandler {
		return NewComponentHandler(&openmcpv1alpha1.Authorization{}, &AuthorizationConverter{}, nil)
	})

	// add new components here

	// Note that the function argument must be an anonymous function and not be wrapped within a NewComponentHandlerFn function or similar,
	// otherwise repeated calls to Registry.GetKnownComponents() will return pointers to the same instance of the resource struct, which breaks the ManagedControlPlane controller's logic!
	// The whole idea behind registering a function instead of just a fixed ComponentHandler is that the registry will always return new ComponentHandlers, never the same one as returned before.
}

var _ ComponentRegistry[*ComponentHandler] = &registry{}
var _ ManagedComponent = &ComponentHandler{}

// Arguments:
//   - obj is an empty version of the in-cluster resource for this component
//   - conv is the components ComponentConverter
//   - aggregationLabelSelectorFunc is a function that gets a name of a (Cluster)Role and returns the LabelSelectors that should be added to that role's aggregation rules, if any.
//     This is used by the Authorization component to grant end-users permissions for the component's resources on the API Server.
//     If a component has custom resources on the API Server which the end-user has to interact with, the component itself should deploy corresponding (Cluster)Rules with aggregation labels
//     and return the fitting selectors via this function here.
func NewComponentHandler(obj Component, conv ComponentConverter, aggregationLabelSelectorFunc func(string) []metav1.LabelSelector) *ComponentHandler {
	return &ComponentHandler{
		resource:                     obj,
		converter:                    conv,
		aggregationLabelSelectorFunc: aggregationLabelSelectorFunc,
	}
}

type ComponentHandler struct {
	resource                     Component
	converter                    ComponentConverter
	aggregationLabelSelectorFunc func(string) []metav1.LabelSelector
}

// Resource implements v1alpha1.ComponentResourceGetter.
func (ch *ComponentHandler) Resource() Component {
	return ch.resource
}

func (ch *ComponentHandler) Converter() ComponentConverter {
	return ch.converter
}

func (ch *ComponentHandler) LabelSelectorsForRole(roleName string) []metav1.LabelSelector {
	if ch.aggregationLabelSelectorFunc == nil {
		return nil
	}
	return ch.aggregationLabelSelectorFunc(roleName)
}

type registry struct {
	reg map[openmcpv1alpha1.ComponentType]func() *ComponentHandler
	sc  *runtime.Scheme
}

func newRegistry(baseScheme *runtime.Scheme) *registry {
	return &registry{
		reg: map[openmcpv1alpha1.ComponentType]func() *ComponentHandler{},
		sc:  baseScheme,
	}
}

// GetComponent implements ComponentRegistry.
func (r *registry) GetComponent(ct openmcpv1alpha1.ComponentType) *ComponentHandler {
	if chp, ok := r.reg[ct]; ok {
		return chp()
	}
	return nil
}

// GetKnownComponents implements ComponentRegistry.
func (r *registry) GetKnownComponents() map[openmcpv1alpha1.ComponentType]*ComponentHandler {
	res := make(map[openmcpv1alpha1.ComponentType]*ComponentHandler, len(r.reg))
	for ct, chp := range r.reg {
		res[ct] = chp()
	}
	return res
}

// Has implements ComponentRegistry.
func (r *registry) Has(ct openmcpv1alpha1.ComponentType) bool {
	_, ok := r.reg[ct]
	return ok
}

// Register implements ComponentRegistry.
func (r *registry) Register(ct openmcpv1alpha1.ComponentType, provideCh func() *ComponentHandler) {
	if provideCh == nil {
		delete(r.reg, ct)
		return
	}
	r.reg[ct] = provideCh
}

// Scheme returns the scheme of the Registry.
func (r *registry) Scheme() *runtime.Scheme {
	return r.sc
}
