package region

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/openmcp-project/controller-utils/pkg/collections"
	"k8s.io/apimachinery/pkg/util/sets"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// DirectionProximityMapping maps directions to lists of neighboring directions.
// The lists are considered to be ordered.
// By default, all directions (except CENTRAL) are considered to be neighbored to CENTRAL first and their respective two non-opposite directions second.
// CENTRAL is neighbored to all other directions.
var DirectionProximityMapping = map[openmcpv1alpha1.Direction][]openmcpv1alpha1.Direction{
	openmcpv1alpha1.CENTRAL: {openmcpv1alpha1.NORTH, openmcpv1alpha1.EAST, openmcpv1alpha1.SOUTH, openmcpv1alpha1.WEST},
	openmcpv1alpha1.NORTH:   {openmcpv1alpha1.CENTRAL, openmcpv1alpha1.WEST, openmcpv1alpha1.EAST},
	openmcpv1alpha1.EAST:    {openmcpv1alpha1.CENTRAL, openmcpv1alpha1.NORTH, openmcpv1alpha1.SOUTH},
	openmcpv1alpha1.SOUTH:   {openmcpv1alpha1.CENTRAL, openmcpv1alpha1.EAST, openmcpv1alpha1.WEST},
	openmcpv1alpha1.WEST:    {openmcpv1alpha1.CENTRAL, openmcpv1alpha1.SOUTH, openmcpv1alpha1.NORTH},
}

var DirectionOppositeMapping = map[openmcpv1alpha1.Direction]openmcpv1alpha1.Direction{
	openmcpv1alpha1.NORTH: openmcpv1alpha1.SOUTH,
	openmcpv1alpha1.EAST:  openmcpv1alpha1.WEST,
	openmcpv1alpha1.SOUTH: openmcpv1alpha1.NORTH,
	openmcpv1alpha1.WEST:  openmcpv1alpha1.EAST,
}

type Location struct {
	Region   openmcpv1alpha1.Region
	Neighbor map[openmcpv1alpha1.Direction]*Location
}

type Geography map[openmcpv1alpha1.Region]*Location

var World Geography = Geography{
	openmcpv1alpha1.AFRICA:       {Region: openmcpv1alpha1.AFRICA, Neighbor: map[openmcpv1alpha1.Direction]*Location{}},
	openmcpv1alpha1.ASIA:         {Region: openmcpv1alpha1.ASIA, Neighbor: map[openmcpv1alpha1.Direction]*Location{}},
	openmcpv1alpha1.AUSTRALIA:    {Region: openmcpv1alpha1.AUSTRALIA, Neighbor: map[openmcpv1alpha1.Direction]*Location{}},
	openmcpv1alpha1.EUROPE:       {Region: openmcpv1alpha1.EUROPE, Neighbor: map[openmcpv1alpha1.Direction]*Location{}},
	openmcpv1alpha1.NORTHAMERICA: {Region: openmcpv1alpha1.NORTHAMERICA, Neighbor: map[openmcpv1alpha1.Direction]*Location{}},
	openmcpv1alpha1.SOUTHAMERICA: {Region: openmcpv1alpha1.SOUTHAMERICA, Neighbor: map[openmcpv1alpha1.Direction]*Location{}},
}

func init() {
	// connect the Locations within the World
	// Africa
	World[openmcpv1alpha1.AFRICA].Neighbor[openmcpv1alpha1.NORTH] = World[openmcpv1alpha1.EUROPE]
	World[openmcpv1alpha1.AFRICA].Neighbor[openmcpv1alpha1.EAST] = World[openmcpv1alpha1.AUSTRALIA]
	World[openmcpv1alpha1.AFRICA].Neighbor[openmcpv1alpha1.WEST] = World[openmcpv1alpha1.SOUTHAMERICA]

	// Asia
	World[openmcpv1alpha1.ASIA].Neighbor[openmcpv1alpha1.EAST] = World[openmcpv1alpha1.NORTHAMERICA]
	World[openmcpv1alpha1.ASIA].Neighbor[openmcpv1alpha1.SOUTH] = World[openmcpv1alpha1.AUSTRALIA]
	World[openmcpv1alpha1.ASIA].Neighbor[openmcpv1alpha1.WEST] = World[openmcpv1alpha1.EUROPE]

	// Australia
	World[openmcpv1alpha1.AUSTRALIA].Neighbor[openmcpv1alpha1.NORTH] = World[openmcpv1alpha1.ASIA]
	World[openmcpv1alpha1.AUSTRALIA].Neighbor[openmcpv1alpha1.EAST] = World[openmcpv1alpha1.SOUTHAMERICA]
	World[openmcpv1alpha1.AUSTRALIA].Neighbor[openmcpv1alpha1.WEST] = World[openmcpv1alpha1.AFRICA]

	// Europe
	World[openmcpv1alpha1.EUROPE].Neighbor[openmcpv1alpha1.EAST] = World[openmcpv1alpha1.ASIA]
	World[openmcpv1alpha1.EUROPE].Neighbor[openmcpv1alpha1.SOUTH] = World[openmcpv1alpha1.AFRICA]
	World[openmcpv1alpha1.EUROPE].Neighbor[openmcpv1alpha1.WEST] = World[openmcpv1alpha1.NORTHAMERICA]

	// Northamerica
	World[openmcpv1alpha1.NORTHAMERICA].Neighbor[openmcpv1alpha1.EAST] = World[openmcpv1alpha1.EUROPE]
	World[openmcpv1alpha1.NORTHAMERICA].Neighbor[openmcpv1alpha1.SOUTH] = World[openmcpv1alpha1.SOUTHAMERICA]
	World[openmcpv1alpha1.NORTHAMERICA].Neighbor[openmcpv1alpha1.WEST] = World[openmcpv1alpha1.ASIA]

	// Southamerica
	World[openmcpv1alpha1.SOUTHAMERICA].Neighbor[openmcpv1alpha1.NORTH] = World[openmcpv1alpha1.NORTHAMERICA]
	World[openmcpv1alpha1.SOUTHAMERICA].Neighbor[openmcpv1alpha1.EAST] = World[openmcpv1alpha1.AFRICA]
	World[openmcpv1alpha1.SOUTHAMERICA].Neighbor[openmcpv1alpha1.WEST] = World[openmcpv1alpha1.AUSTRALIA]
}

func SortByProximity(start openmcpv1alpha1.RegionSpecification, preferSameRegion bool) [][]openmcpv1alpha1.RegionSpecification {
	return World.SortByProximity(start, preferSameRegion)
}

// SortByProximity basically sorts the elements of this Geography based on a breadth-first search starting from the given element.
// The returned list is two-dimensional, where the index of the outer elements corresponds to the relative distance of all corrensponding inner elements.
// If preferSameRegion is set to true, the result list is ordered in a way that other regions only appear after all combinations of the start region with all directions.
func (g Geography) SortByProximity(start openmcpv1alpha1.RegionSpecification, preferSameRegion bool) [][]openmcpv1alpha1.RegionSpecification {
	dummy := openmcpv1alpha1.RegionSpecification{Name: openmcpv1alpha1.Region("DUMMY"), Direction: openmcpv1alpha1.Direction("DUMMY")}
	var q collections.Queue[openmcpv1alpha1.RegionSpecification] = collections.NewLinkedList[openmcpv1alpha1.RegionSpecification](start, dummy)
	visited := sets.New[openmcpv1alpha1.RegionSpecification]()
	sameRegionPriorityMode := preferSameRegion
	res := [][]openmcpv1alpha1.RegionSpecification{}
	curRes := []openmcpv1alpha1.RegionSpecification{}
	addDummy := false
	for {
		if q.Size() == 0 {
			if sameRegionPriorityMode {
				// all preferred region/direction combinations have been added, start breadth-first search again to include other regions
				visited.Clear()
				q.Add(start, dummy)
				sameRegionPriorityMode = false
			} else {
				break
			}
		}
		cur := q.Poll()
		if cur == dummy {
			if len(curRes) > 0 {
				res = append(res, curRes)
				curRes = []openmcpv1alpha1.RegionSpecification{}
			}
			if addDummy {
				q.Add(dummy)
				addDummy = false
			}
			continue
		}
		if cur.Direction == "" {
			cur.Direction = openmcpv1alpha1.CENTRAL
		}
		if visited.Has(cur) {
			// current node has already been seen
			continue
		}
		addDummy = true
		if !(preferSameRegion && !sameRegionPriorityMode && cur.Name == start.Name) { //nolint:staticcheck
			curRes = append(curRes, cur)
		}
		visited.Insert(cur)
		loc, ok := g[cur.Name]
		if !ok {
			// no location found in Geography for current region specification
			continue
		}
		// add new elements to queue based on proximity to current location
		// first, add neighboring directions from same region
		dNeighbors, ok := DirectionProximityMapping[cur.Direction]
		if ok {
			for _, dn := range dNeighbors {
				q.Add(openmcpv1alpha1.RegionSpecification{
					Name:      cur.Name,
					Direction: dn,
				})
			}
		}
		// second, add neighboring region to queue
		// use opposite direction if known
		// skip this in sameRegionPriorityMode, as we are only interested in the starting region at that time
		if !sameRegionPriorityMode {
			rNeighbor, ok := loc.Neighbor[cur.Direction]
			if ok {
				q.Add(openmcpv1alpha1.RegionSpecification{
					Name:      rNeighbor.Region,
					Direction: DirectionOppositeMapping[cur.Direction],
				})
			}
		}
	}
	return res
}

// GetClosestRegion tries to find the region(s) that are closest to the origin region.
// First, the SortByProximity function is used to generate multiple lists of generic regions, ordered by their 'distance' to the origin region.
// Then, for each of these lists, all of its entries are mapped using the specified mapper and extended with the specified pre- and suffix.
// For each string generated this way, the list of available regions is filtered for entries that match the resulting regular expression.
// If any matches are found, the function is aborted and returns only the matches found for the current list.
// If no match is found for any list, nil is returned.
// An error is only returned if the generated regular expression is invalid.
//
// The idea is, that all returned specific regions are considered to have the same minimal distance to the specified origin.
//
// The preferSameRegion field controls whether other directions within the same region should be prioritized over neighboring directions of neighboring regions.
// For example: GCP currently only has regions in the south(east) of Australia, so there is no exact match for the generic region specification (AUSTRALIA, NORTH).
// With preferSameRegion set to false, the function returns GCPs asia-south* regions because Asia's south is considered only one step away from Australia's north, opposed to Australia's south, which is considered to be two steps away.
// With preferSameRegion set to true, the function returns GCPs australia-south* regions because even though it is the opposite direction, they still share the same region and are therefore preferred.
// If GCP had zones in central, east, or west of Australia, all of these would be considered one step away from (AUSTRALIA, NORTH). In this case, with preferSameRegion set to true, only these regions would be returned,
// neither asia-south*, nor australia-south*. With preferSameRegion set to false, the returned selection would contain australia-central*, -east*, and -west*, as well as asia-south*, because all of them are considered the same distance away from (AUSTRALIA, NORTH).
//
// The returned list is sorted alphabetically to ensure consistent results.
func GetClosestRegions(origin openmcpv1alpha1.RegionSpecification, mapper GenericToSpecificRegionMapper, availableRegions []string, preferSameRegion bool) ([]string, error) {
	groups := SortByProximity(origin, preferSameRegion)
	for _, group := range groups {
		var groupMatches []string
		for _, region := range group {
			regex := mapper.MapGenericToSpecific(region.Name, region.Direction)
			if regex == "" {
				continue
			}
			var err error
			regionMatches, err := Filter(availableRegions, regex)
			if err != nil {
				return nil, err
			}
			if len(regionMatches) > 0 {
				groupMatches = append(groupMatches, regionMatches...)
			}
		}
		if len(groupMatches) > 0 {
			slices.Sort(groupMatches)
			return groupMatches, nil
		}
	}
	return nil, nil
}

// Filter filters a list of strings and returns a new list containing only the elements which are matched by the given regular expression.
// Returns an error if the regular expression cannot be compiled.
func Filter(data []string, regex string) ([]string, error) {
	matcher, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("error compiling regex '%s': %w", regex, err)
	}
	res := []string{}
	for _, elem := range data {
		if matcher.MatchString(elem) {
			res = append(res, elem)
		}
	}
	return res, nil
}

var DefaultRegion = openmcpv1alpha1.RegionSpecification{
	Name:      openmcpv1alpha1.EUROPE,
	Direction: openmcpv1alpha1.CENTRAL,
}

// GenericToSpecificRegionMapper maps between the generic region specification and a specific implementation.
type GenericToSpecificRegionMapper interface {
	// MapGenericToSpecific takes a generic region and direction and maps them to an implementation-specific regex string.
	// This regex can be used to filter a list of existing regions for matches.
	// The behavior if (parts of) the arguments are empty or no mapping can be found depends on the implementation of the interface.
	MapGenericToSpecific(openmcpv1alpha1.Region, openmcpv1alpha1.Direction) string
}

var _ GenericToSpecificRegionMapper = &BasicRegionMapper{}

// BasicRegionMapper is a basic implementation of the GenericToSpecificRegionMapper interface.
type BasicRegionMapper struct {
	// In format string, %R will be replaced by the region and %D will be replaced by the direction.
	Format                     string
	RegionGenericToSpecific    map[openmcpv1alpha1.Region]string
	DirectionGenericToSpecific map[openmcpv1alpha1.Direction]string
	DefaultMissingDirection    bool
}

// NewRegionMapper creates a new BasicRegionMapper.
// format is a regex string where %R represents the region and %D represents the direction.
// The mappings map from generic regions and directions to the specific ones.
// If defaultMissingDirection is set to true, the direction is defaulted to CENTRAL if not specified. Otherwise, it is kept empty.
// The region is always defaulted to the value from DefaultRegion if left empty.
//
// Example: If EUROPE maps to 'eu' and CENTRAL maps to 'central' and the format string is '%R-%D-[0-9]+', the mapper would map (EUROPE, CENTRAL) to 'eu-central-[0-9]+'.
// This regex could then be used to filter e.g. ['eu-central-1', 'eu-central-2', 'eu-central-3'] out of a list of available specific regions.
func NewRegionMapper(format string, regionMapping map[openmcpv1alpha1.Region]string, directionMapping map[openmcpv1alpha1.Direction]string, defaultMissingDirection bool) *BasicRegionMapper {
	res := &BasicRegionMapper{
		Format:                     format,
		RegionGenericToSpecific:    make(map[openmcpv1alpha1.Region]string, len(regionMapping)),
		DirectionGenericToSpecific: make(map[openmcpv1alpha1.Direction]string, len(directionMapping)),
		DefaultMissingDirection:    defaultMissingDirection,
	}

	for g, s := range regionMapping {
		res.RegionGenericToSpecific[g] = s
	}
	for g, s := range directionMapping {
		res.DirectionGenericToSpecific[g] = s
	}

	return res
}

func (m *BasicRegionMapper) MapGenericToSpecific(reg openmcpv1alpha1.Region, dir openmcpv1alpha1.Direction) string {
	if reg == "" {
		reg = DefaultRegion.Name
	}
	mReg, ok := m.RegionGenericToSpecific[reg]
	if !ok {
		// no mapping specified for region, return empty string
		return ""
	}
	mDir := ""
	if dir == "" && m.DefaultMissingDirection {
		dir = DefaultRegion.Direction
	}
	mDir = m.DirectionGenericToSpecific[dir]
	rep := strings.NewReplacer("%R", mReg, "%D", mDir)
	return rep.Replace(m.Format)
}

// GetPredefinedMapperByCloudprovider returns a predefined mapper for the specified cloud provider, if any exists.
// Otherwise, nil is returned.
// The name of the cloudprovider is case-insensitive.
//
// Currently supported: aws, gcp
func GetPredefinedMapperByCloudprovider(provider string) GenericToSpecificRegionMapper {
	switch strings.ToLower(provider) {
	case "aws":
		return AWSMapper()
	case "gcp":
		return GCPMapper()
	}
	return nil
}

// AWSMapper returns a pre-configured mapper for AWS regions.
func AWSMapper() *BasicRegionMapper {
	return NewRegionMapper("^%R-%D[a-z]{0,4}-[0-9]+[a-z]*$", map[openmcpv1alpha1.Region]string{
		openmcpv1alpha1.AFRICA:       "af",
		openmcpv1alpha1.ASIA:         "ap",
		openmcpv1alpha1.AUSTRALIA:    "ap",
		openmcpv1alpha1.EUROPE:       "eu",
		openmcpv1alpha1.NORTHAMERICA: "us",
		openmcpv1alpha1.SOUTHAMERICA: "sa",
	}, map[openmcpv1alpha1.Direction]string{
		openmcpv1alpha1.CENTRAL: "central",
		openmcpv1alpha1.NORTH:   "north",
		openmcpv1alpha1.EAST:    "east",
		openmcpv1alpha1.SOUTH:   "south",
		openmcpv1alpha1.WEST:    "west",
	}, true)
}

// AWSRegions contains a list of all known AWS regions. May not be up-to-date.
var AWSRegions = []string{
	"us-east-2",
	"us-east-1",
	"us-west-1",
	"us-west-2",
	"af-south-1",
	"ap-east-1",
	"ap-south-2",
	"ap-southeast-3",
	"ap-southeast-4",
	"ap-south-1",
	"ap-northeast-3",
	"ap-northeast-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-northeast-1",
	"ca-central-1",
	"ca-west-1",
	"eu-central-1",
	"eu-west-1",
	"eu-west-2",
	"eu-south-1",
	"eu-west-3",
	"eu-south-2",
	"eu-north-1",
	"eu-central-2",
	"il-central-1",
	"me-south-1",
	"me-central-1",
	"sa-east-1",
	"us-gov-east-1",
	"us-gov-west-1",
}

// AWSZones contains a list of all known AWS availability zones. May not be up-to-date.
// Note that special zones or zones belonging to special regions are excluded, so not every region from the AWSRegions list has matching zones in here.
var AWSZones = []string{
	"us-east-2a",
	"us-east-2b",
	"us-east-2c",
	"us-east-1a",
	"us-east-1b",
	"us-east-1c",
	"us-east-1d",
	"us-east-1e",
	"us-east-1f",
	"us-west-1a",
	"us-west-1b",
	"us-west-2a",
	"us-west-2b",
	"us-west-2c",
	"us-west-2d",
	"ap-south-1a",
	"ap-south-1b",
	"ap-south-1c",
	"ap-northeast-3a",
	"ap-northeast-3b",
	"ap-northeast-3c",
	"ap-northeast-2a",
	"ap-northeast-2b",
	"ap-northeast-2c",
	"ap-northeast-2d",
	"ap-southeast-1a",
	"ap-southeast-1b",
	"ap-southeast-1c",
	"ap-southeast-2a",
	"ap-southeast-2b",
	"ap-southeast-2c",
	"ap-northeast-1a",
	"ap-northeast-1c",
	"ap-northeast-1d",
	"ca-central-1a",
	"ca-central-1b",
	"ca-central-1d",
	"eu-central-1a",
	"eu-central-1b",
	"eu-central-1c",
	"eu-west-1a",
	"eu-west-1b",
	"eu-west-1c",
	"eu-west-2a",
	"eu-west-2b",
	"eu-west-2c",
	"eu-west-3a",
	"eu-west-3b",
	"eu-west-3c",
	"eu-north-1a",
	"eu-north-1b",
	"eu-north-1c",
	"sa-east-1a",
	"sa-east-1b",
	"sa-east-1c",
}

// GCPMapper returns a pre-configured mapper for GCP regions.
func GCPMapper() *BasicRegionMapper {
	return NewRegionMapper("^%R-%D[a-z]{0,4}[0-9]+(-[a-z]+)?$", map[openmcpv1alpha1.Region]string{
		openmcpv1alpha1.AFRICA:       "me", // not really, but close enough
		openmcpv1alpha1.ASIA:         "asia",
		openmcpv1alpha1.AUSTRALIA:    "australia",
		openmcpv1alpha1.EUROPE:       "europe",
		openmcpv1alpha1.NORTHAMERICA: "us",
		openmcpv1alpha1.SOUTHAMERICA: "southamerica",
	}, map[openmcpv1alpha1.Direction]string{
		openmcpv1alpha1.CENTRAL: "central",
		openmcpv1alpha1.NORTH:   "north",
		openmcpv1alpha1.EAST:    "east",
		openmcpv1alpha1.SOUTH:   "south",
		openmcpv1alpha1.WEST:    "west",
	}, true)
}

// GCPRegions contains a list of all known GCP regions. May not be up-to-date.
var GCPRegions = []string{
	"asia-east1",
	"asia-east2",
	"asia-northeast1",
	"asia-northeast2",
	"asia-northeast3",
	"asia-south1",
	"asia-south2",
	"asia-southeast1",
	"asia-southeast2",
	"australia-southeast1",
	"australia-southeast2",
	"europe-central2",
	"europe-north1",
	"europe-southwest1",
	"europe-west1",
	"europe-west10",
	"europe-west12",
	"europe-west2",
	"europe-west3",
	"europe-west4",
	"europe-west6",
	"europe-west8",
	"europe-west9",
	"me-central1",
	"me-central2",
	"me-west1",
	"northamerica-northeast1",
	"northamerica-northeast2",
	"southamerica-east1",
	"southamerica-west1",
	"us-central1",
	"us-east1",
	"us-east4",
	"us-east5",
	"us-south1",
	"us-west1",
	"us-west2",
	"us-west3",
	"us-west4",
}

// GCPZones contains a list of all known GCP availability zones. May not be up-to-date.
var GCPZones = []string{
	"asia-east1-a",
	"asia-east1-b",
	"asia-east1-c",
	"asia-east2-a",
	"asia-east2-b",
	"asia-east2-c",
	"asia-northeast1-a",
	"asia-northeast1-b",
	"asia-northeast1-c",
	"asia-northeast2-a",
	"asia-northeast2-b",
	"asia-northeast2-c",
	"asia-northeast3-a",
	"asia-northeast3-b",
	"asia-northeast3-c",
	"asia-south1-a",
	"asia-south1-b",
	"asia-south1-c",
	"asia-south2-a",
	"asia-south2-b",
	"asia-south2-c",
	"asia-southeast1-a",
	"asia-southeast1-b",
	"asia-southeast1-c",
	"asia-southeast2-a",
	"asia-southeast2-b",
	"asia-southeast2-c",
	"australia-southeast1-a",
	"australia-southeast1-b",
	"australia-southeast1-c",
	"australia-southeast2-a",
	"australia-southeast2-b",
	"australia-southeast2-c",
	"europe-central2-a",
	"europe-central2-b",
	"europe-central2-c",
	"europe-north1-a",
	"europe-north1-b",
	"europe-north1-c",
	"europe-southwest1-a",
	"europe-southwest1-b",
	"europe-southwest1-c",
	"europe-west1-b",
	"europe-west1-c",
	"europe-west1-d",
	"europe-west10-a",
	"europe-west10-b",
	"europe-west10-c",
	"europe-west12-a",
	"europe-west12-b",
	"europe-west12-c",
	"europe-west2-a",
	"europe-west2-b",
	"europe-west2-c",
	"europe-west3-a",
	"europe-west3-b",
	"europe-west3-c",
	"europe-west4-a",
	"europe-west4-b",
	"europe-west4-c",
	"europe-west6-a",
	"europe-west6-b",
	"europe-west6-c",
	"europe-west8-a",
	"europe-west8-b",
	"europe-west8-c",
	"europe-west9-a",
	"europe-west9-b",
	"europe-west9-c",
	"me-central1-a",
	"me-central1-b",
	"me-central1-c",
	"me-central2-a",
	"me-central2-b",
	"me-central2-c",
	"me-west1-a",
	"me-west1-b",
	"me-west1-c",
	"northamerica-northeast1-a",
	"northamerica-northeast1-b",
	"northamerica-northeast1-c",
	"northamerica-northeast2-a",
	"northamerica-northeast2-b",
	"northamerica-northeast2-c",
	"southamerica-east1-a",
	"southamerica-east1-b",
	"southamerica-east1-c",
	"southamerica-west1-a",
	"southamerica-west1-b",
	"southamerica-west1-c",
	"us-central1-a",
	"us-central1-b",
	"us-central1-c",
	"us-central1-f",
	"us-east1-b",
	"us-east1-c",
	"us-east1-d",
	"us-east4-a",
	"us-east4-b",
	"us-east4-c",
	"us-east5-a",
	"us-east5-b",
	"us-east5-c",
	"us-south1-a",
	"us-south1-b",
	"us-south1-c",
	"us-west1-a",
	"us-west1-b",
	"us-west1-c",
	"us-west2-a",
	"us-west2-b",
	"us-west2-c",
	"us-west3-a",
	"us-west3-b",
	"us-west3-c",
	"us-west4-a",
	"us-west4-b",
	"us-west4-c",
}
