package app

import (
	"context"
	"fmt"
	"os"

	"github.tools.sap/CoLa/mcp-operator/internal/components"
	"github.tools.sap/CoLa/mcp-operator/internal/releasechannel"
	"github.tools.sap/CoLa/mcp-operator/internal/utils/apiserver"

	apiservercontroller "github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver"
	authenticationcontroller "github.tools.sap/CoLa/mcp-operator/internal/controller/core/authentication"
	authorizationcontroller "github.tools.sap/CoLa/mcp-operator/internal/controller/core/authorization"
	clusteradmincontroller "github.tools.sap/CoLa/mcp-operator/internal/controller/core/authorization/clusteradmin"
	cloudorchestratorcontroller "github.tools.sap/CoLa/mcp-operator/internal/controller/core/cloudorchestrator"
	landscapercontroller "github.tools.sap/CoLa/mcp-operator/internal/controller/core/landscaper"
	mcpcontroller "github.tools.sap/CoLa/mcp-operator/internal/controller/core/managedcontrolplane"

	"sigs.k8s.io/controller-runtime/pkg/cluster"

	laasinstall "github.com/gardener/landscaper-service/pkg/apis/core/install"
	"github.com/openmcp-project/controller-utils/pkg/init/webhooks"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	"github.com/spf13/cobra"
	cocorev1beta1 "github.tools.sap/cloud-orchestration/control-plane-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcfg "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	crdinstall "github.tools.sap/CoLa/mcp-operator/api/crds"
	openmcpinstall "github.tools.sap/CoLa/mcp-operator/api/install"
)

func NewMCPOperatorCommand(ctx context.Context) *cobra.Command {
	options := NewOptions()

	cmd := &cobra.Command{
		Use:   "mcpo",
		Short: "mcpo handles ManagedControlPlanes.",

		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Print(err)
				os.Exit(1)
			}
			ctx = logging.NewContext(ctx, options.Log)
			if options.Init {
				if err := options.runInit(ctx); err != nil {
					options.Log.Error(err, "unable to run mcpo init")
					os.Exit(1)
				}
			} else {
				if err := options.run(ctx); err != nil {
					options.Log.Error(err, "unable to run mcpo controller")
					os.Exit(1)
				}
			}
		},
	}

	options.AddFlags(cmd.Flags())

	return cmd
}

func (o *Options) runInit(ctx context.Context) error {
	log := o.Log
	setupLog := log.WithName("setup")

	if o.DryRun {
		setupLog.Info("Exiting now because this is a dry run")
		return nil
	}

	setupLog.Info("Initializing mcpo")

	var err error

	sc := runtime.NewScheme()
	openmcpinstall.Install(sc)
	utilruntime.Must(clientgoscheme.AddToScheme(sc))

	hostClient, err := client.New(o.HostClusterConfig, client.Options{Scheme: components.Registry.Scheme()})
	if err != nil {
		return fmt.Errorf("error building host client: %w", err)
	}
	crateClient, err := client.New(o.CrateClusterConfig, client.Options{Scheme: components.Registry.Scheme()})
	if err != nil {
		return fmt.Errorf("error building crate client: %w", err)
	}

	if o.CRDFlags.Install {
		if err != nil {
			return fmt.Errorf("error building setup client: %w", err)
		}
		setupLog.Info("CRD installation configured, deploying CRDs ...")
		crds := crdinstall.CRDs()
		for _, crd := range crds {
			setupLog.Info("Deploying CRD", "name", crd.Name)
			desired := crd.DeepCopy()
			if _, err := ctrl.CreateOrUpdate(ctx, crateClient, crd, func() error {
				crd.Spec = desired.Spec
				return nil
			}); err != nil {
				return fmt.Errorf("error trying to apply CRD '%s' into cluster: %w", crd.Name, err)
			}
		}
	}

	// install WebHooks if configured
	if o.WebhooksFlags.Install {
		// Generate webhook certificate
		if err := webhooks.GenerateCertificate(ctx, hostClient, o.WebhooksFlags.CertOptions...); err != nil {
			return fmt.Errorf("error generating webhook certificate: %w", err)
		}

		installOptions := o.WebhooksFlags.InstallOptions
		installOptions = append(installOptions, webhooks.WithRemoteClient{Client: crateClient})

		// Install webhooks
		err = webhooks.Install(
			ctx,
			hostClient,
			sc,
			[]client.Object{
				&openmcpv1alpha1.ManagedControlPlane{},
			},
			installOptions...,
		)
		if err != nil {
			return fmt.Errorf("error installing webhooks: %w", err)
		}
	}

	return nil
}

func (o *Options) run(ctx context.Context) error {
	log := o.Log
	// ctx = logging.NewContext(ctx, log)
	setupLog := log.WithName("setup")

	if o.DryRun {
		setupLog.Info("Exiting now because this is a dry run")
		return nil
	}
	if len(o.ActiveControllers) == 0 {
		setupLog.Info("All controllers deactivated, nothing to do")
		return nil
	}

	setupLog.Info("Starting controllers")
	sc := runtime.NewScheme()
	openmcpinstall.Install(sc)
	utilruntime.Must(clientgoscheme.AddToScheme(sc))
	mgr, err := ctrl.NewManager(o.CrateClusterConfig, ctrl.Options{
		Scheme: sc,
		Metrics: server.Options{
			BindAddress: o.MetricsAddr,
		},
		Controller: ctrlcfg.Controller{
			RecoverPanic: ptr.To(true),
		},
		HealthProbeBindAddress:  o.ProbeAddr,
		LeaderElection:          o.EnableLeaderElection,
		LeaderElectionID:        "mcpo.openmcp.cloud",
		LeaderElectionNamespace: o.LeaseNamespace,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	apiServerWorkerOptions := &apiserver.Options{
		MaxWorkers: ptr.To(o.APIServerWorkerCount),
		Interval:   ptr.To(o.APIServerWorkerInterval),
	}
	apiServerWorker, err := apiserver.NewWorker(mgr.GetClient(), apiServerWorkerOptions)
	if err != nil {
		return fmt.Errorf("unable to create APIServer worker: %w", err)
	}

	// run WebHooks if configured
	if o.WebhooksFlags.Install {
		if err := (&openmcpv1alpha1.ManagedControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("failed to setup webhook: %w", err)
		}
	}

	if o.ActiveControllers.Has(ControllerIDManagedControlPlane) {
		// ManagedControlPlane controller
		cpc := mcpcontroller.NewManagedControlPlaneController(mgr.GetClient())
		if err := cpc.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error adding controller '%s' to manager: %w", mcpcontroller.ControllerName, err)
		}
	}

	if o.ActiveControllers.Has(ControllerIDAPIServer) {
		// APIServer controller
		apiServerProvider, err := apiservercontroller.NewAPIServerProvider(ctx, mgr.GetClient(), o.APIServerConfig)
		if err != nil {
			return fmt.Errorf("error creating %s: %w", apiservercontroller.ControllerName, err)
		}
		if err := apiServerProvider.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error adding controller '%s' to manager: %w", apiservercontroller.ControllerName, err)
		}
	}

	if o.ActiveControllers.Has(ControllerIDLandscaper) {
		// Landscaper controller
		// build laas scheme
		laasScheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(laasScheme))
		laasinstall.Install(laasScheme)
		// build laas cluster client
		laasClient, err := client.New(o.LaaSClusterConfig, client.Options{
			Scheme: laasScheme,
		})
		if err != nil {
			return fmt.Errorf("error creating LaaS cluster client: %w", err)
		}
		// add controller
		if err := landscapercontroller.NewLandscaperConnector(mgr.GetClient(), laasClient).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error adding controller '%s' to manager: %w", landscapercontroller.ControllerName, err)
		}
	}

	if o.ActiveControllers.Has(ControllerIDCloudOrchestrator) {
		// CloudOrchestrator controller
		// build cloudOrchestrator cluster client
		coScheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(coScheme))
		utilruntime.Must(cocorev1beta1.AddToScheme(coScheme))
		cloudOrchestratorClient, err := client.New(o.CloudOrchestratorClusterConfig, client.Options{
			Scheme: coScheme,
		})
		if err != nil {
			return fmt.Errorf("error creating cloudOrchestrator cluster client: %w", err)
		}
		coreCluster, err := cluster.New(o.CloudOrchestratorClusterConfig, func(o *cluster.Options) {
			o.Scheme = coScheme
		})
		if err != nil {
			return fmt.Errorf("error creating core cluster: %w", err)
		}
		if err := mgr.Add(coreCluster); err != nil {
			return fmt.Errorf("error adding core cluster to manager: %w", err)
		}
		// add controller
		if err := cloudorchestratorcontroller.NewCloudOrchestratorController(mgr.GetClient(), cloudOrchestratorClient, coreCluster).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error adding controller '%s' to manager: %w", cloudorchestratorcontroller.ControllerName, err)
		}

		// Add releasechannel sync runnable
		runnable := releasechannel.NewReleasechannelRunnable(mgr.GetClient(), cloudOrchestratorClient)
		if err := mgr.Add(&runnable); err != nil {
			return fmt.Errorf("unable to add releasechannel sync runnable: %w", err)
		}
	}

	if o.ActiveControllers.Has(ControllerIDAuthentication) {
		// Authentication controller
		// add controller
		if err := authenticationcontroller.NewAuthenticationReconciler(mgr.GetClient(), o.AuthConfig).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error adding controller '%s' to manager: %w", authenticationcontroller.ControllerName, err)
		}
	}

	if o.ActiveControllers.Has(ControllerIDAuthorization) {
		// Authorization controller
		// add controller
		if err := authorizationcontroller.NewAuthorizationReconciler(mgr.GetClient(), o.AuthzConfig).RegisterTasks(apiServerWorker).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error adding controller '%s' to manager: %w", authorizationcontroller.ControllerName, err)
		}

		if err := clusteradmincontroller.NewClusterAdminReconciler(mgr.GetClient(), o.AuthzConfig).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("error adding controller '%s' to manager: %w", clusteradmincontroller.ControllerName, err)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	signalHandler := ctrl.SetupSignalHandler()
	signalHandler = logging.NewContext(signalHandler, log.WithName("apiServerWorker"))

	setupLog.Info("Starting APIServer worker")
	if err := apiServerWorker.Start(signalHandler, nil, nil, mgr.Elected()); err != nil {
		return fmt.Errorf("problem running APIServer worker: %w", err)
	}

	setupLog.Info("Starting manager")

	if err = mgr.Start(signalHandler); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}

	return nil
}
