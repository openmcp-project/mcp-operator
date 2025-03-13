package gardener

import (
	"fmt"
	"strings"

	"math/rand"

	"github.com/openmcp-project/controller-utils/pkg/logging"
	"sigs.k8s.io/yaml"

	"github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	gardenv1beta1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
)

type shootBuilder interface {
	newControlPlaneConfig(log logging.Logger) (*runtime.RawExtension, error)
	newInfrastructureConfig(log logging.Logger) (*runtime.RawExtension, error)
	adjustWorkers(log logging.Logger, provider *gardenv1beta1.Provider)
}

func getShootBuilderByCloudProvider(
	log logging.Logger,
	existingShoot *gardenv1beta1.Shoot,
	nameNamespaceHash int,
	provider string,
	shootTemplate *gardenv1beta1.ShootTemplate,
	region *gardenv1beta1.Region,
	haConfig *v1alpha1.HighAvailabilityConfig,
) (shootBuilder, error) {
	// build a numeric hash from shoot name and namespace
	// this allows to choose a random element from a slice (e.g. the region) in a deterministic way
	bsb := baseShootBuilder{
		shootTemplate: shootTemplate,
		region:        region,
		workerZones:   region.Zones,
		haConfig:      haConfig,
	}
	// check if a control plane zone is already set in the shoot
	// it's immutable and tied to the worker zones, so we shouldn't change it
	cpc := existingShoot.Spec.Provider.ControlPlaneConfig
	if cpc != nil && cpc.Raw != nil {
		cpcData := map[string]any{}
		if err := yaml.Unmarshal(cpc.Raw, &cpcData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal control plane config: %w", err)
		}
		rawCpcZone, ok := cpcData["zone"]
		if ok {
			cpcZone, ok := rawCpcZone.(string)
			if ok {
				log.Debug("Using control plane zone from existing shoot", "zone", cpcZone)
				bsb.controlPlaneZone = cpcZone
			}
		}
	}
	if bsb.controlPlaneZone == "" && len(region.Zones) > 0 {
		bsb.controlPlaneZone = region.Zones[nameNamespaceHash%len(region.Zones)].Name
	}

	switch strings.ToLower(provider) {
	case "gcp":
		return &shootBuilderGCP{baseShootBuilder: bsb}, nil
	case "aws":
		return &shootBuilderAWS{baseShootBuilder: bsb}, nil
	}
	return nil, fmt.Errorf("unsupported cloud provider: %s", provider)
}

// baseShootBuilder provides methods which don't depend on the cloud provider
type baseShootBuilder struct {
	shootTemplate *gardenv1beta1.ShootTemplate
	region        *gardenv1beta1.Region
	// randomly selected controlPlaneZone of the region, used for the controlplane config
	controlPlaneZone string
	workerZones      []gardenv1beta1.AvailabilityZone
	haConfig         *v1alpha1.HighAvailabilityConfig
}

func (b *baseShootBuilder) newInfrastructureConfig(log logging.Logger) (*runtime.RawExtension, error) {
	return b.shootTemplate.Spec.Provider.InfrastructureConfig, nil
}

// adjustWorkers adjusts the shoot's workers based on the shoot template and the HA configuration.
// The reason why this is so complex is that some parts of a worker spec, e.g. the zones, are immutable.
// Instead of changing it, one has to create a new worker spec (with a new name) and remove the old one.
func (b *baseShootBuilder) adjustWorkers(log logging.Logger, provider *gardenv1beta1.Provider) {
	// generate workers based on shoot template
	desiredWorkers := b.shootTemplate.Spec.Provider.DeepCopy().Workers
	for i := 0; i < len(desiredWorkers); i++ {
		worker := &desiredWorkers[i]
		worker.Name = fmt.Sprintf("worker-%s", randString(5))

		if b.haConfig != nil {
			// Consensus-based software components depend on maintaining a quorum of (n/2)+1.
			// Therefore, at least 3 zones are needed to tolerate the outage of 1 zone.
			// For all non-consensus-based software components, 2 nodes are sufficient to tolerate the outage of 1 node.
			// To be safe, use 3 zones as the minimum number of zones.
			// If the region has less than 3 zones, all available zones are used.
			worker.Minimum = int32(min(3, len(b.workerZones)))
			// The maximum number of workers should be at least the minimum number of workers.
			// If the maximum is configured higher, use the configured maximum.
			worker.Maximum = int32(max(int(worker.Minimum), int(worker.Maximum)))

			if b.haConfig.FailureToleranceType == v1alpha1.HighAvailabilityFailureToleranceZone {
				// place workers in all available zones, up to the minimum number of workers
				worker.Zones = make([]string, 0, worker.Minimum)
				for i := 0; i < int(worker.Minimum); i++ {
					worker.Zones = append(worker.Zones, b.workerZones[i].Name)
				}
			}
			if b.haConfig.FailureToleranceType == v1alpha1.HighAvailabilityFailureToleranceNode {
				// place all workers in one zone
				worker.Zones = []string{b.controlPlaneZone}
			}
		} else {
			// If the control plane is not HA or only node-tolerant, all workers are placed in the control plane zone.
			worker.Zones = []string{b.controlPlaneZone}
		}
	}

	keepFromOld := sets.New[string]()
	takeFromNew := sets.New[string]()
	for idx, dw := range desiredWorkers {
		found := false
		for _, w := range provider.Workers {
			if keepFromOld.Has(w.Name) {
				// this worker spec is already kept because it matches another desired spec
				continue
			}
			diffs := workerEquals(&dw, &w)
			if len(diffs) == 0 {
				// there exists a worker spec that matches one of our desired specs, so let's keep it
				log.Debug("Keeping existing worker group, as its specs match desired worker group", "workerGroup", w.Name, "desiredWorkerGroupIndex", idx)
				keepFromOld.Insert(w.Name)
				found = true
				break
			} else {
				log.Debug("Existing worker group does not match desired worker group", "workerGroup", w.Name, "desiredWorkerGroupIndex", idx, "differences", diffs.String())
			}
		}
		if !found {
			// no worker spec matches the current desired one, so we need to create a new one
			takeFromNew.Insert(dw.Name)
		}
	}

	newWorkers := make([]gardenv1beta1.Worker, 0, keepFromOld.Len()+takeFromNew.Len())
	for _, w := range provider.Workers {
		if keepFromOld.Has(w.Name) {
			newWorkers = append(newWorkers, w)
		} else {
			log.Debug("Discarding existing worker group", "workerGroup", w.Name)
		}
	}
	for _, dw := range desiredWorkers {
		if takeFromNew.Has(dw.Name) {
			log.Debug("Adding new worker group", "workerGroup", dw.Name)
			newWorkers = append(newWorkers, dw)
		}
	}
	provider.Workers = newWorkers
}

const randStringChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = randStringChars[rand.Intn(len(randStringChars))]
	}
	return string(b)
}

// workerEquals compares two worker specs for equality, ignoring the name.
// Note that only selected fields are compared, since a DeepEqual causes continuous reconciliation loops
// (probably due to Gardener injecting some information into the worker spec).
// The return value is a slice of fields that are different. If it is nil or empty, the worker specs are equal.
func workerEquals(a, b *gardenv1beta1.Worker) diffList {
	if a == nil && b == nil {
		return nil
	}
	if a == nil || b == nil {
		aName := "nil"
		bName := "nil"
		if a != nil {
			aName = a.Name
		} else if b != nil {
			bName = b.Name
		}
		return []diff{newDiff("nil", aName, bName)}
	}
	res := []diff{}
	if a.Machine.Image.Name != b.Machine.Image.Name {
		res = append(res, newDiff("Machine.Image.Name", a.Machine.Image.Name, b.Machine.Image.Name))
	}
	if a.Machine.Image.Version != nil && b.Machine.Image.Version != nil && *a.Machine.Image.Version != *b.Machine.Image.Version {
		res = append(res, newDiff("Machine.Image.Version", a.Machine.Image.Version, b.Machine.Image.Version))
	}
	if a.Machine.Type != b.Machine.Type {
		res = append(res, newDiff("Machine.Type", a.Machine.Type, b.Machine.Type))
	}
	if a.Machine.Architecture != nil && b.Machine.Architecture != nil && *a.Machine.Architecture != *b.Machine.Architecture {
		res = append(res, newDiff("Machine.Architecture", a.Machine.Architecture, b.Machine.Architecture))
	}
	if a.Minimum != b.Minimum {
		res = append(res, newDiff("Minimum", a.Minimum, b.Minimum))
	}
	if a.Maximum != b.Maximum {
		res = append(res, newDiff("Maximum", a.Maximum, b.Maximum))
	}
	if a.MaxSurge != nil && b.MaxSurge != nil && *a.MaxSurge != *b.MaxSurge {
		res = append(res, newDiff("MaxSurge", a.MaxSurge, b.MaxSurge))
	}
	if a.MaxUnavailable != nil && b.MaxUnavailable != nil && *a.MaxUnavailable != *b.MaxUnavailable {
		res = append(res, newDiff("MaxUnavailable", a.MaxUnavailable, b.MaxUnavailable))
	}
	if !sets.New(a.Zones...).Equal(sets.New(b.Zones...)) {
		res = append(res, newDiff("Zones", fmt.Sprintf("[%s]", strings.Join(a.Zones, ", ")), fmt.Sprintf("[%s]", strings.Join(b.Zones, ", "))))
	}
	return res
}

func newDiff(id string, a, b any) diff {
	return diff{id: id, a: fmt.Sprint(a), b: fmt.Sprint(b)}
}

type diff struct {
	id string
	a  string
	b  string
}

func (d diff) String() string {
	return fmt.Sprintf("%s: %s != %s", d.id, d.a, d.b)
}

type diffList []diff

func (d diffList) String() string {
	res := make([]string, len(d))
	for i, diff := range d {
		res[i] = diff.String()
	}
	return fmt.Sprintf("[%s]", strings.Join(res, ", "))
}
