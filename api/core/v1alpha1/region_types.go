package v1alpha1

import "fmt"

// Region represents a supported region.
// +kubebuilder:validation:Enum=northamerica;southamerica;europe;asia;africa;australia
type Region string

// Direction represents a direction within a region.
// +kubebuilder:validation:Enum=north;east;south;west;central
type Direction string

const (
	AFRICA       Region = "africa"
	ASIA         Region = "asia"
	AUSTRALIA    Region = "australia"
	EUROPE       Region = "europe"
	NORTHAMERICA Region = "northamerica"
	SOUTHAMERICA Region = "southamerica"
)

var AllRegions = []Region{AFRICA, ASIA, AUSTRALIA, EUROPE, NORTHAMERICA, SOUTHAMERICA}

const (
	NORTH   Direction = "north"
	EAST    Direction = "east"
	SOUTH   Direction = "south"
	WEST    Direction = "west"
	CENTRAL Direction = "central"
)

var AllDirections = []Direction{NORTH, EAST, SOUTH, WEST, CENTRAL}

type RegionSpecification struct {
	// Name is the name of the region.
	Name Region `json:"name,omitempty"`

	// Direction is the direction within the region.
	Direction Direction `json:"direction,omitempty"`
}

func (r RegionSpecification) String() string {
	return fmt.Sprintf("%s-%s", r.Name, r.Direction)
}
