package components

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/openmcp-project/mcp-operator/internal/components"

	"github.com/openmcp-project/controller-utils/pkg/logging"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

var (
	mutex *sync.Mutex
)

func init() {
	mutex = &sync.Mutex{}
}

// GetDependents returns all dependency finalizers present on the given component, with the dependency finalizer prefix removed.
func GetDependents(requirement components.Component) sets.Set[string] {
	res := sets.New[string]()
	for _, fin := range requirement.GetFinalizers() {
		if strings.HasPrefix(fin, openmcpv1alpha1.DependencyFinalizerPrefix) {
			res.Insert(strings.TrimPrefix(fin, openmcpv1alpha1.DependencyFinalizerPrefix))
		}
	}
	return res
}

// HasAnyDependencyFinalizer returns true if the given resource has a dependency finalizer from any other component.
func HasAnyDependencyFinalizer(obj client.Object) bool {
	if obj == nil {
		return false
	}
	finalizers := obj.GetFinalizers()
	if len(finalizers) == 0 {
		return false
	}
	for _, fin := range finalizers {
		if strings.HasPrefix(fin, openmcpv1alpha1.DependencyFinalizerPrefix) {
			return true
		}
	}
	return false
}

// HasDepedencyFinalizer returns true if the given resource has a dependency finalizer from the given component.
func HasDepedencyFinalizer(obj client.Object, ct openmcpv1alpha1.ComponentType) bool {
	if obj == nil {
		return false
	}
	finalizers := obj.GetFinalizers()
	if len(finalizers) == 0 {
		return false
	}
	cFin := ct.DependencyFinalizer()
	for _, fin := range finalizers {
		if fin == cFin {
			return true
		}
	}
	return false
}

// EnsureDependencyFinalizer ensures that the dependency finalizer of component 'depComp' either exists or doesn't exist (based on argument 'expected') on the resource of component 'reqComp'.
func EnsureDependencyFinalizer(ctx context.Context, c client.Client, reqComp components.Component, depComp components.Component, expected bool) error {
	// since this function is called from multiple controller goroutines, we need to lock the access to the resource
	// otherwise, we might end up with an inconsistent list of finalizers
	mutex.Lock()
	defer mutex.Unlock()

	// get the latest version of the resource so that we only remove or add the requested dependency finalizer
	if err := c.Get(ctx, client.ObjectKeyFromObject(reqComp), reqComp); err != nil {
		return fmt.Errorf("error getting resource %s/%s: %w", reqComp.GetNamespace(), reqComp.GetName(), err)
	}

	// log finalizers before changing them
	log, err := logging.FromContext(ctx)
	if err == nil {
		logFinalizers(log, reqComp)
	}

	exists := HasDepedencyFinalizer(reqComp, depComp.Type())
	if exists == expected {
		// either finalizer exists and should be there or doesn't exist and should not
		return nil
	}

	fins := reqComp.GetFinalizers()
	if fins == nil {
		fins = []string{}
	}

	cFin := depComp.Type().DependencyFinalizer()
	if expected {
		fins = append(fins, cFin)
	} else {
		for i := len(fins) - 1; i >= 0; i-- {
			if fins[i] == cFin {
				fins = append(fins[:i], fins[i+1:]...)
			}
		}
	}

	finsj, err := json.Marshal(fins)
	if err != nil {
		return fmt.Errorf("error converting list of finalizers to JSON: %w", err)
	}
	if err := c.Patch(ctx, reqComp, client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"metadata":{"finalizers":%s}}`, finsj)))); err != nil {
		return err
	}
	return nil
}

// IsDependencyReady checks if a dependency is ready.
// The first argument is the resource (components.Component).
// The second argument is the generation of the ManagedControlPlane the current component was generated from.
// The third argument is the generation of the InternalConfiguration the current component was generated from, or -1 if no InternalConfiguration exists.
// Further arguments can be used to specify which conditions should be checked for readiness. If none are specified, all existing ones have to be "True".
// In addition to the IsComponentReady/IsComponentReadyRaw functions, this one only returns true if the depended on component was created from the same generation of the ManagedControlPlane as the current component.
func IsDependencyReady(dep components.Component, ownCPGeneration, ownICGeneration int64, relevantConditions ...string) bool {
	if dep == nil {
		return false
	}
	return IsComponentReady(dep, relevantConditions...) &&
		dep.GetCommonStatus().ObservedGenerations.ManagedControlPlane == ownCPGeneration &&
		dep.GetCommonStatus().ObservedGenerations.InternalConfiguration == ownICGeneration
}

func logFinalizers(log logging.Logger, comp components.Component) {
	if comp == nil {
		return
	}
	finalizers := comp.GetFinalizers()
	compName := client.ObjectKeyFromObject(comp).String()
	compType := comp.GetObjectKind().GroupVersionKind().String()
	if len(finalizers) == 0 {
		log.Debug("Finalizers", cconst.KeyResource, compName, cconst.KeyReconciledResourceKind, compType, "finalizers", "none")
	} else {
		log.Debug("Finalizers", cconst.KeyResource, compName, cconst.KeyReconciledResourceKind, compType, "finalizers", strings.Join(finalizers, ","))
	}
}
