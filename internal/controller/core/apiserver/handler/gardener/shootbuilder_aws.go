package gardener

import (
	"encoding/json"
	"fmt"
	"net"

	"sigs.k8s.io/yaml"

	"github.com/apparentlymart/go-cidr/cidr"

	"k8s.io/apimachinery/pkg/runtime"

	"github.tools.sap/CoLa/controller-utils/pkg/logging"

	gardenawsv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener-extension-provider-aws/pkg/apis/aws/v1alpha1"
)

var _ shootBuilder = &shootBuilderAWS{}

type shootBuilderAWS struct {
	baseShootBuilder
}

func (b *shootBuilderAWS) newControlPlaneConfig(log logging.Logger) (*runtime.RawExtension, error) {
	controlPlaneConfig := map[string]any{
		"apiVersion": "aws.provider.extensions.gardener.cloud/v1alpha1",
		"kind":       "ControlPlaneConfig",
	}
	controlPlaneConfigRaw, err := json.Marshal(controlPlaneConfig)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{
		Raw: controlPlaneConfigRaw,
	}, nil
}

// The AWS infrastructure config expects a list of zones with CIDR ranges for workers, public, and internal subnets.
// If this information is not provided in the shoot template, it is added based on the VPC CIDR range.
func (b *shootBuilderAWS) newInfrastructureConfig(log logging.Logger) (*runtime.RawExtension, error) {
	// get defined zone networks from shoot template
	awsInfraCfg := &gardenawsv1alpha1.InfrastructureConfig{}
	err := yaml.Unmarshal(b.shootTemplate.Spec.Provider.InfrastructureConfig.Raw, awsInfraCfg)
	if err != nil {
		return nil, err
	}
	if len(awsInfraCfg.Networks.Zones) == 0 {
		// add zones to infrastructure config if they are not defined
		// Note that this CIDR computation logic is designed for a /16 VPC CIDR range and might not work well with netmask sizes containing fewer IP addresses (= larger number behind the '/').
		vpcRaw := awsInfraCfg.Networks.VPC.CIDR
		if vpcRaw == nil {
			return nil, fmt.Errorf("networks.vpc.cidr is not defined in the AWS infrastructure config")
		}
		_, vpc, err := net.ParseCIDR(*vpcRaw)
		if err != nil {
			return nil, fmt.Errorf("networks.vpc.cidr '%s' is not a valid CIDR: %w", *vpcRaw, err)
		}
		internal, _ := cidr.PreviousSubnet(vpc, 32) // dummy to simplify the loop
		for _, zone := range b.workerZones {
			workers, overflow := cidr.NextSubnet(internal, 19)
			_, lastIP := cidr.AddressRange(workers)
			if overflow || !vpc.Contains(lastIP) {
				return nil, fmt.Errorf("unable to calculate 'workers' subnet for zone '%s': vpc CIDR range '%s' does not fully contain computed subnet '%s'", zone.Name, vpc.String(), workers.String())
			}
			public, overflow := cidr.NextSubnet(workers, 20)
			_, lastIP = cidr.AddressRange(public)
			if overflow || !vpc.Contains(lastIP) {
				return nil, fmt.Errorf("unable to calculate 'public' subnet for zone '%s': vpc CIDR range '%s' does not fully contain computed subnet '%s'", zone.Name, vpc.String(), public.String())
			}
			internal, overflow = cidr.NextSubnet(public, 20)
			_, lastIP = cidr.AddressRange(internal)
			if overflow || !vpc.Contains(lastIP) {
				return nil, fmt.Errorf("unable to calculate 'internal' subnet for zone '%s': vpc CIDR range '%s' does not fully contain computed subnet '%s'", zone.Name, vpc.String(), internal.String())
			}

			awsInfraCfg.Networks.Zones = append(awsInfraCfg.Networks.Zones, gardenawsv1alpha1.Zone{
				Name:     zone.Name,
				Workers:  workers.String(),
				Public:   public.String(),
				Internal: internal.String(),
			})
		}
	}

	awsInfraCfgRaw, err := json.Marshal(awsInfraCfg)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: awsInfraCfgRaw}, nil
}
