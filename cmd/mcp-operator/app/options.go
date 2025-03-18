package app

import (
	goflag "flag"
	"fmt"
	"strings"
	"time"

	"github.com/openmcp-project/mcp-operator/internal/components"

	"github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"
	configauthn "github.com/openmcp-project/mcp-operator/internal/controller/core/authentication/config"
	configauthz "github.com/openmcp-project/mcp-operator/internal/controller/core/authorization/config"

	colactrlutil "github.com/openmcp-project/controller-utils/pkg/controller"
	"github.com/openmcp-project/controller-utils/pkg/init/crds"
	"github.com/openmcp-project/controller-utils/pkg/init/webhooks"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	ctrlrun "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

const (
	ControllerIDManagedControlPlane = "managedcontrolplane"
)

var (
	ControllerIDAPIServer         = strings.ToLower(string(openmcpv1alpha1.APIServerComponent))
	ControllerIDLandscaper        = strings.ToLower(string(openmcpv1alpha1.LandscaperComponent))
	ControllerIDCloudOrchestrator = strings.ToLower(string(openmcpv1alpha1.CloudOrchestratorComponent))
	ControllerIDAuthentication    = strings.ToLower(string(openmcpv1alpha1.AuthenticationComponent))
	ControllerIDAuthorization     = strings.ToLower(string(openmcpv1alpha1.AuthorizationComponent))
)

// rawOptions contains the options specified directly via the command line.
// The Options struct then contains these as embedded struct and additionally some options that were derived from the raw options (e.g. by loading files or interpreting raw options).
type rawOptions struct {
	// init
	Init bool `json:"init"`

	// controller-runtime stuff
	MetricsAddr          string `json:"metricsAddress"`
	EnableLeaderElection bool   `json:"enableLeaderElection"`
	LeaseNamespace       string `json:"leaseNamespace"`
	ProbeAddr            string `json:"healthProbeAddress"`

	// raw options that need to be evaluated
	APIServerConfigPath          string `json:"apiServerConfigPath"`
	LaaSClusterPath              string `json:"laasClusterConfigPath"`
	CrateClusterPath             string `json:"crateClusterConfigPath"`
	CloudOrchestratorClusterPath string `json:"cloudOrchestratorClusterConfigPath"`
	AuthConfigPath               string `json:"authConfigPath"`
	AuthzConfigPath              string `json:"authzConfigPath"`
	ControllerList               string `json:"controllers"`

	APIServerWorkerCount    int           `json:"apiServerWorkerCount"`
	APIServerWorkerInterval time.Duration `json:"apiServerWorkerInterval"`

	// raw options that are final
	DryRun bool `json:"dryRun"`
}

func writeHeader(sb *strings.Builder, includeHeader bool, header string) {
	if includeHeader {
		sb.WriteString("########## ")
		sb.WriteString(header)
		sb.WriteString(" ##########\n")
	}
}

func (ro *rawOptions) String(includeHeader bool) (string, error) {
	sb := strings.Builder{}
	writeHeader(&sb, includeHeader, "RAW OPTIONS")
	printableRawOptions, err := yaml.Marshal(ro)
	if err != nil {
		return "", fmt.Errorf("unable to marshal raw options to yaml: %w", err)
	}
	sb.WriteString(string(printableRawOptions))
	writeHeader(&sb, includeHeader, "END RAW OPTIONS")
	return sb.String(), nil
}

// Options describes the options to configure the Landscaper controller.
type Options struct {
	rawOptions

	// completed options from raw options
	APIServerConfig                *config.APIServerProviderConfiguration
	Log                            logging.Logger
	LaaSClusterConfig              *rest.Config
	CrateClusterConfig             *rest.Config
	CloudOrchestratorClusterConfig *rest.Config
	HostClusterConfig              *rest.Config
	AuthConfig                     *configauthn.AuthenticationConfig
	AuthzConfig                    *configauthz.AuthorizationConfig
	ActiveControllers              sets.Set[string]
	WebhooksFlags                  *webhooks.Flags
	CRDFlags                       *crds.Flags
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) String(includeHeader bool, includeRawOptions bool) (string, error) {
	sb := strings.Builder{}
	writeHeader(&sb, includeHeader, "OPTIONS")

	if includeRawOptions {
		rawOpts, err := o.rawOptions.String(false)
		if err != nil {
			return "", err
		}
		sb.WriteString(rawOpts)
	}

	opts := map[string]any{}
	// API server config
	opts["apiServerConfig"] = o.APIServerConfig

	// clusters
	opts["crateClusterHost"] = nil
	if o.CrateClusterConfig != nil {
		opts["crateClusterHost"] = o.CrateClusterConfig.Host
	}
	opts["laasClusterHost"] = nil
	if o.LaaSClusterConfig != nil {
		opts["laasClusterHost"] = o.LaaSClusterConfig.Host
	}
	opts["cloudOrchestratorClusterHost"] = nil
	if o.CloudOrchestratorClusterConfig != nil {
		opts["cloudOrchestratorClusterHost"] = o.CloudOrchestratorClusterConfig.Host
	}

	hostClusterConfig, err := ctrlrun.GetConfig()
	if err != nil {
		return "", fmt.Errorf("error getting host cluster config: %w", err)
	}
	o.HostClusterConfig = hostClusterConfig

	opts["authConfig"] = o.AuthConfig
	opts["authzConfig"] = o.AuthzConfig

	// controllers
	opts["activeControllers"] = sets.List(o.ActiveControllers)

	// convert to yaml
	optsString, err := yaml.Marshal(opts)
	if err != nil {
		return "", fmt.Errorf("error converting options map to yaml: %w", err)
	}
	sb.WriteString(string(optsString))

	webhooksString, err := yaml.Marshal(o.WebhooksFlags)
	if err != nil {
		return "", fmt.Errorf("error converting webhooks flags to yaml: %w", err)
	}

	writeHeader(&sb, includeHeader, "WEBHOOKS")
	sb.WriteString(string(webhooksString))

	crdsString, err := yaml.Marshal(o.CRDFlags)
	if err != nil {
		return "", fmt.Errorf("error converting CRD flags to yaml: %w", err)
	}

	writeHeader(&sb, includeHeader, "CRDs")
	sb.WriteString(string(crdsString))

	writeHeader(&sb, includeHeader, "END OPTIONS")

	return sb.String(), nil
}

func (o *Options) AddFlags(fs *flag.FlagSet) {
	// decide if init process or main controller
	fs.BoolVar(&o.Init, "init", false, "If true, the init process is started, which creates the necessary CRDs and webhooks configuration.")

	// standard stuff
	fs.StringVar(&o.MetricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to.")
	fs.StringVar(&o.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(&o.EnableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&o.LeaseNamespace, "lease-namespace", "default", "Namespace in which the controller manager's leader election lease will be created.")

	// APIServer
	fs.StringVar(&o.APIServerConfigPath, "apiserver-config", "", "Path to the APIServer provider config file.")
	fs.IntVar(&o.APIServerWorkerCount, "apiserver-workers", 10, "Number of max workers in the APIServer worker pool.")
	fs.DurationVar(&o.APIServerWorkerInterval, "apiserver-worker-interval", time.Second*1, "Interval at which the APIServer worker runs the tasks.")

	// landscaper
	fs.StringVar(&o.LaaSClusterPath, "laas-cluster", "", "Path to the LaaS core cluster kubeconfig file or directory containing either a kubeconfig or host, token, and ca file. Leave empty to use in-cluster config.")

	// cloudorchestrator
	fs.StringVar(&o.CloudOrchestratorClusterPath, "co-cluster", "", "Path to the CloudOrchestrator core cluster kubeconfig file or directory containing either a kubeconfig or host, token, and ca file. Leave empty to use in-cluster config.")

	// authentication
	fs.StringVar(&o.AuthConfigPath, "auth-config", "", "Path to the authentication config file.")

	// authorization
	fs.StringVar(&o.AuthzConfigPath, "authz-config", "", "Path to the authorization config file.")

	// common
	fs.BoolVar(&o.DryRun, "dry-run", false, "If true, the CLI args are evaluated as usual, but the program exits before the controllers are started.")
	fs.StringVar(&o.CrateClusterPath, "crate-cluster", "", "Path to the crate cluster kubeconfig file or directory containing either a kubeconfig or host, token, and ca file. Leave empty to use in-cluster config.")
	fs.StringVar(&o.ControllerList, "controllers", strings.Join([]string{ControllerIDManagedControlPlane, ControllerIDAPIServer, ControllerIDLandscaper, ControllerIDCloudOrchestrator}, ","), "Comma-separated list of controllers that should be active.")
	logging.InitFlags(fs)
	// because webhooks.BindFlags only supports Go's native flag package, we have to convert
	fsHelper := &goflag.FlagSet{}
	o.WebhooksFlags = webhooks.BindFlags(fsHelper)
	o.CRDFlags = crds.BindFlags(fsHelper)
	fs.AddGoFlagSet(fsHelper)
}

// Complete parses all Options and flags and initializes the basic functions
func (o *Options) Complete() error {
	// build logger
	log, err := logging.GetLogger()
	if err != nil {
		return err
	}
	o.Log = log
	ctrlrun.SetLogger(o.Log.Logr())
	olog := log.WithName("options")

	// print raw options
	rawOptsString, err := o.rawOptions.String(true)
	if err != nil {
		olog.Error(err, "error computing raw options string for printing")
	} else {
		fmt.Print(rawOptsString)
	}

	// determine active controllers
	o.ActiveControllers = sets.New(strings.Split(o.ControllerList, ",")...)
	// remove empty string, if part of active controllers
	delete(o.ActiveControllers, "")
	// unregister components for inactive controllers
	for ct := range components.Registry.GetKnownComponents() {
		if !o.ActiveControllers.Has(strings.ToLower(string(ct))) {
			// controller for component is not active, remove registration
			components.Registry.Register(ct, nil)
		}
	}

	// load kubeconfigs
	o.CrateClusterConfig, err = colactrlutil.LoadKubeconfig(o.CrateClusterPath)
	if err != nil {
		return fmt.Errorf("unable to load crate cluster kubeconfig: %w", err)
	}
	if o.ActiveControllers.Has(ControllerIDLandscaper) {
		o.LaaSClusterConfig, err = colactrlutil.LoadKubeconfig(o.LaaSClusterPath)
		if err != nil {
			return fmt.Errorf("unable to load laas cluster kubeconfig: %w", err)
		}
	}
	if o.ActiveControllers.Has(ControllerIDCloudOrchestrator) {
		o.CloudOrchestratorClusterConfig, err = colactrlutil.LoadKubeconfig(o.CloudOrchestratorClusterPath)
		if err != nil {
			return fmt.Errorf("unable to load core cluster kubeconfig: %w", err)
		}
	}

	// load APIServer provider config
	if o.ActiveControllers.Has(ControllerIDAPIServer) {
		if o.APIServerConfigPath == "" {
			return fmt.Errorf("no (or empty) path to API server config file given, please specify --apiserver-config argument")
		}
		o.APIServerConfig, err = config.LoadConfig(o.APIServerConfigPath)
		if err != nil {
			return err
		}
		err = config.Validate(o.APIServerConfig)
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
	}

	// load authentication config
	if o.ActiveControllers.Has(ControllerIDAuthentication) {
		if o.AuthConfigPath == "" {
			return fmt.Errorf("no (or empty) path to authentication config file given, please specify --auth-config argument")
		}
		o.AuthConfig, err = configauthn.LoadConfig(o.AuthConfigPath)
		if err != nil {
			return err
		}

		err = configauthn.Validate(o.AuthConfig)
		if err != nil {
			return fmt.Errorf("invalid authentication config: %w", err)
		}
	}

	// load authorization config
	if o.ActiveControllers.Has(ControllerIDAuthorization) {
		if o.AuthzConfigPath == "" {
			return fmt.Errorf("no (or empty) path to authorization config file given, please specify --authz-config argument")
		}
		o.AuthzConfig, err = configauthz.LoadConfig(o.AuthzConfigPath)
		if err != nil {
			return err
		}

		err = configauthz.Validate(o.AuthzConfig)
		if err != nil {
			return fmt.Errorf("invalid authorization config: %w", err)
		}
	}

	// print options
	optsString, err := o.String(true, false)
	if err != nil {
		olog.Error(err, "error computing options string for printing")
	} else {
		fmt.Print(optsString)
	}

	return nil
}
