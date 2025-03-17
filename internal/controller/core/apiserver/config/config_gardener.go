package config

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/schemes"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenv1beta1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
)

const (
	// defaultGardenerLandscapeName is used to default the landscape name if a single config (old format) is provided.
	defaultGardenerLandscapeName = "default"
	// defaultGardenerConfigName is used to default the config name if a single config (old format) is provided.
	defaultGardenerConfigName = "default"
)

var (
	defaultLandscapeAndConfigurationRegex = regexp.MustCompile(`^([a-z0-9-]+/[a-z0-9-]+)$`)
)

// MultiGardenerConfiguration contains configuration for multiple Gardener landscapes.
type MultiGardenerConfiguration struct {
	// GardenerConfiguration accepts a single Gardener configuration.
	// It is mainly used for backward compatibility.
	*GardenerConfiguration `json:",inline"`

	// GardenerLandscapeWithoutConfig takes the fields which have been removed from GardenerConfiguration to preserve backward compatibility.
	*GardenerLandscapeWithoutConfig `json:",inline"`

	// DefaultLandscapeAndConfiguration is the name of the default Gardener configuration.
	// It is expected to follow the format '<landscape-name>/<config-name>'.
	// Only required in multi-config mode.
	DefaultLandscapeAndConfiguration string `json:"defaultConfig,omitempty"`

	// Landscapes is a list of supported Gardener landscapes.
	// Only required in multi-config mode.
	Landscapes []GardenerLandscape `json:"landscapes,omitempty"`
}

// GardenerLandscape represents a Gardener landscape.
type GardenerLandscape struct {
	GardenerLandscapeWithoutConfig `json:",inline"`

	// Name is the name of this Gardener landscape.
	Name string `json:"name,omitempty"`

	// Configurations is a list of Gardener configurations.
	Configurations []GardenerConfiguration `json:"configs,omitempty"`
}

// This type exists only for backward compatibility.
// It can be merged with GardenerLandscape, if we at one point decide to get rid of the single configuration option.
type GardenerLandscapeWithoutConfig struct {
	// Kubeconfig contains an inline kubeconfig.
	Kubeconfig string `json:"kubeconfig,omitempty"`

	// If not nil, this client is used instead of creating a new one from the given kubeconfig during the 'complete' method.
	// This is meant as a way to inject a fake client for testing purposes.
	gardenClusterClient client.Client
}

// GardenerConfiguration contains configuration for a Gardener.
type GardenerConfiguration struct {
	// Name is the name of this Gardener configuration.
	// Only required in multi-config mode.
	Name string `json:"name,omitempty"`

	// Project is the Gardener project which should be used to create shoot clusters in it.
	// The provided kubeconfig must have priviliges for this project.
	Project string `json:"project,omitempty"`

	// CloudProfile is the name of the Gardener CloudProfile that should be used for this shoot.
	CloudProfile string `json:"cloudProfile,omitempty"`

	// ShootTemplate contains the configuration for shoot clusters with worker nodes.
	// It is relevant for APIServers for which spec.gardener.enableWorkers is true.
	ShootTemplate *gardenv1beta1.ShootTemplate `json:"shootTemplate,omitempty"`

	// Regions contains the supported regions and their zones.
	Regions []gardenv1beta1.Region `json:"regions,omitempty"`

	// DefaultRegion is the default region for the workerless shoots.
	// If not specified, a region must be chosen in the APIServer spec.
	// If specified, this region will be used if there is none in the APIServer spec.
	DefaultRegion string `json:"defaultRegion,omitempty"`
}

// InjectGardenClusterClient can be used to inject a fake client for testing purposes.
// It has to be called before calling the 'complete' method.
// For single config mode, the landscape parameter must be an empty string.
func (cfg *MultiGardenerConfiguration) InjectGardenClusterClient(landscape string, fakeClient client.Client) {
	if landscape == "" {
		if cfg.GardenerLandscapeWithoutConfig == nil {
			cfg.GardenerLandscapeWithoutConfig = &GardenerLandscapeWithoutConfig{}
		}
		cfg.gardenClusterClient = fakeClient
	} else {
		for i := range cfg.Landscapes {
			ls := &cfg.Landscapes[i]
			if ls.Name == landscape {
				ls.gardenClusterClient = fakeClient
				return
			}
		}
	}
}

type CompletedMultiGardenerConfiguration struct {
	// DefaultConfiguration is the name of the default Gardener configuration.
	DefaultConfiguration string

	// DefaultLandscape is the name of the default Gardener landscape.
	DefaultLandscape string

	// Landscapes is a map of Gardener landscapes.
	Landscapes map[string]CompletedGardenerLandscape
}

// LandscapeConfiguration returns the Gardener configuration with the given name, or an error if the configuration is unknown.
// This function can either be called with two separate arguments, or a single combined argument in the format '<landscape-name>/<config-name>'.
//
// In the two-argument case, the first argument is the landscape name and the second argument is the configuration name.
// An empty landscape or configuration string is interpreted as the respective default value.
// Note that the default config belongs to the default landscape, so a non-empty landscape string in combination with an empty config string is invalid.
//
// In the single-argument case, the argument is a combined string in the format '<landscape-name>/<config-name>'.
// A single empty string will be interpreted as default landscape and configuration.
// Apart from that, neither landscape nor configuration may be empty in this case.
//
// Will default landscape and configuration if called without any arguments.
//
// Any other amount of arguments will result in an error.
func (ccfg *CompletedMultiGardenerConfiguration) LandscapeConfiguration(data ...string) (*CompletedGardenerLandscape, *CompletedGardenerConfiguration, error) {
	switch len(data) {
	case 0:
		return ccfg.configurationFromLandscapeAndConfigFields(ccfg.DefaultLandscape, ccfg.DefaultConfiguration)
	case 1:
		return ccfg.configurationFromCombinedField(data[0])
	case 2:
		return ccfg.configurationFromLandscapeAndConfigFields(data[0], data[1])
	default:
		return nil, nil, fmt.Errorf("invalid arguments: expected either one combined argument or two separate arguments, got %d", len(data))
	}
}

func (ccfg *CompletedMultiGardenerConfiguration) configurationFromCombinedField(lc string) (*CompletedGardenerLandscape, *CompletedGardenerConfiguration, error) {
	if lc == "" {
		return ccfg.configurationFromLandscapeAndConfigFields(ccfg.DefaultLandscape, ccfg.DefaultConfiguration)
	}
	fields := strings.Split(lc, "/")
	if len(fields) != 2 {
		return nil, nil, fmt.Errorf("expected format is '<landscape-name>/<config-name>', but got '%s'", lc)
	}
	if fields[0] == "" || fields[1] == "" {
		return nil, nil, fmt.Errorf("invalid arguments: landscape and config must not be empty in combined format")
	}
	return ccfg.configurationFromLandscapeAndConfigFields(fields[0], fields[1])
}

func (ccfg *CompletedMultiGardenerConfiguration) configurationFromLandscapeAndConfigFields(landscape, config string) (*CompletedGardenerLandscape, *CompletedGardenerConfiguration, error) {
	if landscape == "" {
		landscape = ccfg.DefaultLandscape
	} else if landscape != ccfg.DefaultLandscape && config == "" {
		return nil, nil, fmt.Errorf("invalid arguments: config can only be defaulted if landscape is also default (default landscape '%s', given landscape '%s')", ccfg.DefaultLandscape, landscape)
	}
	ls, ok := ccfg.Landscapes[landscape]
	if !ok {
		return nil, nil, fmt.Errorf("unable to get Gardener landscape: unknown landscape '%s'", landscape)
	}
	if config == "" {
		config = ccfg.DefaultConfiguration
	}
	cfg, ok := ls.Configurations[config]
	if !ok {
		return nil, nil, fmt.Errorf("unable to get Gardener configuration: unknown configuration '%s'", config)
	}
	return &ls, &cfg, nil
}

type CompletedGardenerLandscape struct {
	// Client for accessing the Gardener landscape.
	Client client.Client

	// Kubeconfig contains the kubeconfig the client was constructed from (unless injected for test scenarios).
	Kubeconfig string

	// Configurations is a map of Gardener configurations.
	Configurations map[string]CompletedGardenerConfiguration
}

type CompletedGardenerConfiguration struct {
	GardenerConfiguration

	// ProjectNamespace is the namespace belonging to the configured project.
	ProjectNamespace string

	// ProviderType is the provider type. It is extracted from the given CloudProfile.
	ProviderType string

	// ValidRegions is the set of valid regions. It contains the regions from the given CloudProfile
	// whose names are listed in the APIServer config.
	ValidRegions map[string]gardenv1beta1.Region

	// ValidK8SVersions is the set of valid k8s versions. It is extracted from the given CloudProfile.
	ValidK8SVersions sets.Set[string]
}

// Worker is the base definition of a worker group.
type Worker struct {
	Name string
	// Machine contains information about the machine type and image.
	Machine Machine
	// Maximum is the maximum number of machines to create.
	// This value is divided by the number of configured zones for a fair distribution.
	Maximum int32
	// Minimum is the minimum number of machines to create.
	// This value is divided by the number of configured zones for a fair distribution.
	Minimum int32
}

// Machine contains information about the machine type and image.
type Machine struct {
	// Type is the machine type of the worker group.
	Type string
	// Image holds information about the machine image to use for all nodes of this pool. It will default to the
	// latest version of the first image stated in the referenced CloudProfile if no value has been provided.
	// +optional
	Image *ShootMachineImage
	// Architecture is CPU architecture of machines in this worker pool.
	// +optional
	Architecture *string
}

// ShootMachineImage defines the name and the version of the shoot's machine image in any environment. Has to be
// defined in the respective CloudProfile.
type ShootMachineImage struct {
	// Name is the name of the image.
	Name string
	// ProviderConfig is the shoot's individual configuration passed to an extension resource.
	// +optional
	ProviderConfig *runtime.RawExtension
	// Version is the version of the shoot's image.
	// If version is not provided, it will be defaulted to the latest version from the CloudProfile.
	// +optional
	Version *string
}

func (cfg *MultiGardenerConfiguration) complete(ctx context.Context) (*CompletedMultiGardenerConfiguration, error) {
	if cfg == nil {
		return nil, nil
	}

	res := &CompletedMultiGardenerConfiguration{}

	if cfg.DefaultLandscapeAndConfiguration == "" && len(cfg.Landscapes) == 0 {
		// old single config mode
		// transform to multi config mode
		cfg.DefaultLandscapeAndConfiguration = fmt.Sprintf("%s/%s", defaultGardenerLandscapeName, defaultGardenerConfigName)
		cfg.Landscapes = []GardenerLandscape{
			{
				Name:                           defaultGardenerLandscapeName,
				GardenerLandscapeWithoutConfig: *cfg.GardenerLandscapeWithoutConfig,
				Configurations: []GardenerConfiguration{
					{
						Name:          defaultGardenerConfigName,
						CloudProfile:  cfg.CloudProfile,
						DefaultRegion: cfg.DefaultRegion,
						Project:       cfg.Project,
						Regions:       cfg.Regions,
						ShootTemplate: cfg.ShootTemplate,
					},
				},
			},
		}
	}

	// split default landscape and configuration into separate fields
	defaults := strings.Split(cfg.DefaultLandscapeAndConfiguration, "/")
	if len(defaults) != 2 {
		return nil, fmt.Errorf("default landscape and configuration '%s' does not follow the expected format '<landscape-name>/<config-name>'", cfg.DefaultLandscapeAndConfiguration)
	}
	res.DefaultLandscape = defaults[0]
	res.DefaultConfiguration = defaults[1]

	// complete landscapes
	res.Landscapes = map[string]CompletedGardenerLandscape{}
	for _, ls := range cfg.Landscapes {
		cls := CompletedGardenerLandscape{}

		// use injected client or build from given kubeconfig
		cls.Kubeconfig = ls.Kubeconfig
		if ls.gardenClusterClient != nil {
			// use fake client for testing
			cls.Client = ls.gardenClusterClient
		} else {
			// build client from kubeconfig
			restCfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(cls.Kubeconfig))
			if err != nil {
				return nil, fmt.Errorf("error building rest config from kubeconfig for landscape '%s': %w", ls.Name, err)
			}
			cls.Client, err = client.New(restCfg, client.Options{
				Scheme: schemes.GardenerScheme,
			})
			if err != nil {
				return nil, fmt.Errorf("error creating client for landscape '%s': %w", ls.Name, err)
			}
		}

		cls.Configurations = map[string]CompletedGardenerConfiguration{}
		for _, lscfg := range ls.Configurations {
			clscfg := CompletedGardenerConfiguration{
				GardenerConfiguration: lscfg,
			}

			// fetch project to get project namespace
			pr := &gardenv1beta1.Project{}
			pr.SetName(lscfg.Project)
			if err := cls.Client.Get(ctx, client.ObjectKeyFromObject(pr), pr); err != nil {
				return nil, fmt.Errorf("[%s/%s] error fetching Project '%s': %w", ls.Name, lscfg.Name, lscfg.Project, err)
			}
			if pr.Spec.Namespace == nil {
				return nil, fmt.Errorf("[%s/%s] project namespace is not set", ls.Name, lscfg.Name)
			}
			clscfg.ProjectNamespace = *pr.Spec.Namespace

			// fetch cloudprofile
			cp := &gardenv1beta1.CloudProfile{}
			cp.SetName(lscfg.CloudProfile)
			if err := cls.Client.Get(ctx, client.ObjectKeyFromObject(cp), cp); err != nil {
				return nil, fmt.Errorf("[%s/%s] error fetching CloudProfile '%s': %w", ls.Name, lscfg.Name, lscfg.CloudProfile, err)
			}

			// set provider type
			clscfg.ProviderType = cp.Spec.Type

			// set valid regions: select all regions from the cloud profile whose name is contained in the configured regions
			clscfg.ValidRegions = map[string]gardenv1beta1.Region{}
			for _, cpRegion := range cp.Spec.Regions {
				for _, cfgRegion := range lscfg.Regions {
					if cpRegion.Name == cfgRegion.Name {
						clscfg.ValidRegions[cpRegion.Name] = cpRegion
						break
					}
				}
			}

			// set valid k8s versions
			clscfg.ValidK8SVersions = sets.New[string]()
			for _, version := range cp.Spec.Kubernetes.Versions {
				semver := strings.Split(version.Version, ".")
				minorVersion := fmt.Sprintf("%s.%s", semver[0], semver[1])
				clscfg.ValidK8SVersions.Insert(version.Version, minorVersion)
			}

			// if a default region is given, check that it is valid
			if lscfg.DefaultRegion != "" {
				_, ok := clscfg.ValidRegions[lscfg.DefaultRegion]
				if !ok {
					return nil, fmt.Errorf("[%s/%s] the specified default region '%s' is not valid for the chosen cloudprofile '%s'", ls.Name, lscfg.Name, lscfg.DefaultRegion, lscfg.CloudProfile)
				}
			}

			cls.Configurations[lscfg.Name] = clscfg
		}

		res.Landscapes[ls.Name] = cls
	}

	return res, nil
}

func validateGardenerConfig(cfg *MultiGardenerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg == nil {
		allErrs = append(allErrs, field.Required(fldPath, "Gardener config must not be empty"))
		return allErrs
	}

	if cfg.DefaultLandscapeAndConfiguration == "" && len(cfg.Landscapes) == 0 {
		// single config mode (old format)
		if cfg.GardenerConfiguration == nil || cfg.Project == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("project"), "project name must not be empty"))
		}
		if cfg.GardenerConfiguration == nil || cfg.CloudProfile == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("cloudProfile"), "cloudprofile name must not be empty"))
		}
		if cfg.GardenerLandscapeWithoutConfig == nil || len(cfg.Kubeconfig) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("kubeconfig"), "kubeconfig must not be empty"))
		}
		if cfg.GardenerConfiguration == nil || len(cfg.Regions) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("regions"), "regions must not be empty"))
		}
		if cfg.GardenerConfiguration == nil || cfg.ShootTemplate == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("shootTemplate"), "shootTemplate must not be empty"))
		} else {
			allErrs = append(allErrs, validateShootTemplate(cfg.ShootTemplate, fldPath.Child("shootTemplate"))...)
		}
	} else {
		// multi config mode
		if cfg.DefaultLandscapeAndConfiguration == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("defaultConfig"), "default landscape/configuration name must not be empty"))
		} else if !defaultLandscapeAndConfigurationRegex.MatchString(cfg.DefaultLandscapeAndConfiguration) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("defaultConfig"), cfg.DefaultLandscapeAndConfiguration, "default configuration name must follow the format '<landscape-name>/<config-name>'"))
		}
		if len(cfg.Landscapes) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("landscapes"), "landscapes must not be empty"))
		}

		knownLandscapeNames := sets.New[string]()
		for i, ls := range cfg.Landscapes {
			landscapePath := fldPath.Child("landscapes").Index(i)
			if ls.Name == "" {
				allErrs = append(allErrs, field.Required(landscapePath.Child("name"), "landscape name must not be empty"))
			}
			if knownLandscapeNames.Has(ls.Name) {
				allErrs = append(allErrs, field.Duplicate(landscapePath.Child("name"), ls.Name))
			}
			if len(ls.Configurations) == 0 {
				allErrs = append(allErrs, field.Required(landscapePath.Child("configs"), "configurations must not be empty"))
			}
			if len(ls.Kubeconfig) == 0 {
				allErrs = append(allErrs, field.Required(landscapePath.Child("kubeconfig"), "kubeconfig must not be empty"))
			}

			knownLandscapeNames.Insert(ls.Name)
			knownConfigNames := sets.New[string]()

			for j, lscfg := range ls.Configurations {
				configPath := landscapePath.Child("configs").Index(j)
				if lscfg.Name == "" {
					allErrs = append(allErrs, field.Required(configPath.Child("name"), "configuration name must not be empty"))
				}
				if knownConfigNames.Has(lscfg.Name) {
					allErrs = append(allErrs, field.Duplicate(configPath.Child("name"), lscfg.Name))
				}

				knownConfigNames.Insert(lscfg.Name)

				if lscfg.Project == "" {
					allErrs = append(allErrs, field.Required(configPath.Child("project"), "project name must not be empty"))
				}
				if lscfg.CloudProfile == "" {
					allErrs = append(allErrs, field.Required(configPath.Child("cloudProfile"), "cloudprofile name must not be empty"))
				}
				if len(lscfg.Regions) == 0 {
					allErrs = append(allErrs, field.Required(configPath.Child("regions"), "regions must not be empty"))
				}

				allErrs = append(allErrs, validateShootTemplate(lscfg.ShootTemplate, configPath.Child("shootTemplate"))...)
			}
		}
	}

	return allErrs
}

func validateShootTemplate(st *gardenv1beta1.ShootTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if st == nil {
		allErrs = append(allErrs, field.Required(fldPath, "shootTemplate must not be empty"))
		return allErrs
	}

	specPath := fldPath.Child("spec")

	if st.Spec.Networking == nil {
		allErrs = append(allErrs, field.Required(specPath.Child("networking"), "networking must not be empty"))
	} else {
		networkingPath := specPath.Child("networking")
		if st.Spec.Networking.Type == nil {
			allErrs = append(allErrs, field.Required(networkingPath.Child("type"), "networking type must not be empty"))
		}
		if st.Spec.Networking.Nodes == nil {
			allErrs = append(allErrs, field.Required(networkingPath.Child("nodes"), "networking nodes must not be empty"))
		}
	}

	if len(st.Spec.Provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(specPath.Child("provider", "type"), "provider type must not be empty"))
	}

	if st.Spec.Provider.InfrastructureConfig == nil {
		allErrs = append(allErrs, field.Required(specPath.Child("provider", "infrastructureConfig"), "infrastructureConfig must not be empty"))
	}

	if len(st.Spec.Provider.Workers) == 0 {
		allErrs = append(allErrs, field.Required(specPath.Child("provider", "workers"), "workers must not be empty"))
	}

	for i, worker := range st.Spec.Provider.Workers {
		workerPath := specPath.Child("provider", "workers").Index(i)
		if worker.Machine.Architecture == nil {
			allErrs = append(allErrs, field.Required(workerPath.Child("machine", "architecture"), "architecture must not be empty"))
		}
		if worker.Machine.Image == nil {
			allErrs = append(allErrs, field.Required(workerPath.Child("machine", "image"), "machine image must not be empty"))
		} else {
			if worker.Machine.Image.Version == nil {
				allErrs = append(allErrs, field.Required(workerPath.Child("machine", "image", "version"), "machine image version must not be empty"))
			}
		}
		if len(worker.Machine.Type) == 0 {
			allErrs = append(allErrs, field.Required(workerPath.Child("machine", "type"), "machine type must not be empty"))
		}
		if worker.Volume == nil {
			allErrs = append(allErrs, field.Required(workerPath.Child("volume"), "volume must not be empty"))
		} else {
			if worker.Volume.Type == nil {
				allErrs = append(allErrs, field.Required(workerPath.Child("volume", "type"), "volume type must not be empty"))
			}
			if len(worker.Volume.VolumeSize) == 0 {
				allErrs = append(allErrs, field.Required(workerPath.Child("volume", "size"), "volume size must not be empty"))
			}
		}
	}

	if st.Spec.SecretBindingName == nil {
		allErrs = append(allErrs, field.Required(specPath.Child("secretBindingName"), "secretBindingName must not be empty"))
	}

	return allErrs
}
