package app

import (
	"context"
	"fmt"
	"os"

	"github.com/openmcp-project/mcp-operator/internal/components"
	"github.com/openmcp-project/mcp-operator/internal/releasechannel"
	"github.com/openmcp-project/mcp-operator/internal/utils/apiserver"

	mcpocfg "github.com/openmcp-project/mcp-operator/internal/config"
	apiservercontroller "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver"
	authenticationcontroller "github.com/openmcp-project/mcp-operator/internal/controller/core/authentication"
	authorizationcontroller "github.com/openmcp-project/mcp-operator/internal/controller/core/authorization"
	clusteradmincontroller "github.com/openmcp-project/mcp-operator/internal/controller/core/authorization/clusteradmin"
	cloudorchestratorcontroller "github.com/openmcp-project/mcp-operator/internal/controller/core/cloudorchestrator"
	landscapercontroller "github.com/openmcp-project/mcp-operator/internal/controller/core/landscaper"
	mcpcontroller "github.com/openmcp-project/mcp-operator/internal/controller/core/managedcontrolplane"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	v2install "github.com/openmcp-project/openmcp-operator/api/install"
	lsv2install "github.com/openmcp-project/service-provider-landscaper/api/install"

	laasinstall "github.com/gardener/landscaper-service/pkg/apis/core/install"
	cocorev1beta1 "github.com/openmcp-project/control-plane-operator/api/v1beta1"
	"github.com/openmcp-project/controller-utils/pkg/init/webhooks"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	"github.com/openmcp-project/controller-utils/pkg/resources"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcfg "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	crdinstall "github.com/openmcp-project/mcp-operator/api/crds"
	openmcpinstall "github.com/openmcp-project/mcp-operator/api/install"
)

const OperatorName = "ManagedControlPlaneOperator"

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

		webhookTypes := []webhooks.APITypes{
			{
				Obj:       &openmcpv1alpha1.ManagedControlPlane{},
				Validator: true,
				Defaulter: true,
			},
		}

		// Install webhooks
		err = webhooks.Install(
			ctx,
			hostClient,
			sc,
			webhookTypes,
			installOptions...,
		)
		if err != nil {
			return fmt.Errorf("error installing webhooks: %w", err)
		}
	}

	// manage architecture immutability
	labelSelector := client.MatchingLabels{
		openmcpv1alpha1.ManagedByLabel:      OperatorName,
		openmcpv1alpha1.ManagedPurposeLabel: openmcpv1alpha1.ManagedPurposeArchitectureImmutability,
	}
	evapbs := &admissionv1.ValidatingAdmissionPolicyBindingList{}
	if err := crateClient.List(ctx, evapbs, labelSelector); err != nil {
		return fmt.Errorf("error listing ValidatingAdmissionPolicyBindings: %w", err)
	}
	for _, evapb := range evapbs.Items {
		if mcpocfg.Config.Architecture.Immutability.Disabled || evapb.Name != mcpocfg.Config.Architecture.Immutability.PolicyName {
			setupLog.Info("Deleting existing ValidatingAdmissionPolicyBinding with architecture immutability purpose", "name", evapb.Name)
			if err := crateClient.Delete(ctx, &evapb); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("error deleting ValidatingAdmissionPolicyBinding '%s': %w", evapb.Name, err)
			}
		}
	}
	evaps := &admissionv1.ValidatingAdmissionPolicyList{}
	if err := crateClient.List(ctx, evaps, labelSelector); err != nil {
		return fmt.Errorf("error listing ValidatingAdmissionPolicies: %w", err)
	}
	for _, evap := range evaps.Items {
		if mcpocfg.Config.Architecture.Immutability.Disabled || evap.Name != mcpocfg.Config.Architecture.Immutability.PolicyName {
			setupLog.Info("Deleting existing ValidatingAdmissionPolicy with architecture immutability purpose", "name", evap.Name)
			if err := crateClient.Delete(ctx, &evap); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("error deleting ValidatingAdmissionPolicy '%s': %w", evap.Name, err)
			}
		}
	}
	if !mcpocfg.Config.Architecture.Immutability.Disabled {
		setupLog.Info("Architecture immutability validation enabled, creating/updating ValidatingAdmissionPolicies ...")
		vapm := resources.NewValidatingAdmissionPolicyMutator(mcpocfg.Config.Architecture.Immutability.PolicyName, admissionv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: ptr.To(admissionv1.Fail),
			MatchConstraints: &admissionv1.MatchResources{
				ResourceRules: []admissionv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionv1.RuleWithOperations{
							Operations: []admissionv1.OperationType{
								admissionv1.Create,
								admissionv1.Update,
							},
							Rule: admissionv1.Rule{ // match all resources, actual restriction happens in the binding
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
							},
						},
					},
				},
			},
			Variables: []admissionv1.Variable{
				{
					Name:       "archLabel",
					Expression: fmt.Sprintf(`(has(object.metadata.labels) && "%s" in object.metadata.labels) ? object.metadata.labels["%s"] : ""`, openmcpv1alpha1.ArchitectureVersionLabel, openmcpv1alpha1.ArchitectureVersionLabel),
				},
				{
					Name:       "oldArchLabel",
					Expression: fmt.Sprintf(`(oldObject != null && has(oldObject.metadata.labels) && "%s" in oldObject.metadata.labels) ? oldObject.metadata.labels["%s"] : ""`, openmcpv1alpha1.ArchitectureVersionLabel, openmcpv1alpha1.ArchitectureVersionLabel),
				},
			},
			Validations: []admissionv1.Validation{
				{
					Expression: fmt.Sprintf(`variables.archLabel == "%s" || variables.archLabel == "%s"`, openmcpv1alpha1.ArchitectureV1, openmcpv1alpha1.ArchitectureV2),
					Message:    fmt.Sprintf(`The label "%s" must be set and its value must be either "%s" or "%s".`, openmcpv1alpha1.ArchitectureVersionLabel, openmcpv1alpha1.ArchitectureV1, openmcpv1alpha1.ArchitectureV2),
				},
				{
					Expression: fmt.Sprintf(`request.operation == "CREATE" || (variables.oldArchLabel == "" && variables.archLabel == "%s") || (variables.oldArchLabel == variables.archLabel)`, openmcpv1alpha1.ArchitectureV1),
					Message:    fmt.Sprintf(`The label "%s" is immutable, it may not be changed or removed once set. Adding it to existing resources is only allowed with "%s" as value.`, openmcpv1alpha1.ArchitectureVersionLabel, openmcpv1alpha1.ArchitectureV1),
				},
			},
		})
		vapm.MetadataMutator().WithLabels(map[string]string{
			openmcpv1alpha1.ManagedByLabel:      OperatorName,
			openmcpv1alpha1.ManagedPurposeLabel: openmcpv1alpha1.ManagedPurposeArchitectureImmutability,
		})
		if err := resources.CreateOrUpdateResource(ctx, crateClient, vapm); err != nil {
			return fmt.Errorf("error creating/updating ValidatingAdmissionPolicy for architecture immutability: %w", err)
		}

		vapbm := resources.NewValidatingAdmissionPolicyBindingMutator(mcpocfg.Config.Architecture.Immutability.PolicyName, admissionv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: mcpocfg.Config.Architecture.Immutability.PolicyName,
			ValidationActions: []admissionv1.ValidationAction{
				admissionv1.Deny,
			},
			MatchResources: &admissionv1.MatchResources{
				ResourceRules: []admissionv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionv1.RuleWithOperations{
							Operations: []admissionv1.OperationType{
								admissionv1.Create,
								admissionv1.Update,
							},
							Rule: admissionv1.Rule{
								APIGroups:   []string{openmcpv1alpha1.GroupVersion.Group},
								APIVersions: []string{openmcpv1alpha1.GroupVersion.Version},
								Resources: []string{
									"apiservers",
									"landscapers",
									"cloudorchestrators",
									"authentications",
									"authorizations",
								},
							},
						},
					},
				},
			},
		})
		vapbm.MetadataMutator().WithLabels(map[string]string{
			openmcpv1alpha1.ManagedByLabel:      OperatorName,
			openmcpv1alpha1.ManagedPurposeLabel: openmcpv1alpha1.ManagedPurposeArchitectureImmutability,
		})
		if err := resources.CreateOrUpdateResource(ctx, crateClient, vapbm); err != nil {
			return fmt.Errorf("error creating/updating ValidatingAdmissionPolicyBinding for architecture immutability: %w", err)
		}

		setupLog.Info("ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding for architecture immutability created/updated")
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
	lsv2install.InstallProviderAPIs(sc)
	mgr, err := ctrl.NewManager(o.CrateClusterConfig, ctrl.Options{
		Scheme: sc,
		Metrics: server.Options{
			BindAddress: o.MetricsAddr,
		},
		Controller: ctrlcfg.Controller{
			RecoverPanic: ptr.To(true),
		},
		HealthProbeBindAddress:  o.ProbeAddr,
		PprofBindAddress:        o.PprofAddr,
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
		// build platform cluster client for v2 path
		v2scheme := v2install.InstallOperatorAPIsPlatform(runtime.NewScheme())
		platformClient, err := client.New(o.LaaSClusterConfig, client.Options{
			Scheme: v2scheme,
		})
		if err != nil {
			return fmt.Errorf("error creating platform cluster client: %w", err)
		}
		apiServerProvider, err := apiservercontroller.NewAPIServerProvider(ctx, mgr.GetClient(), platformClient, o.APIServerConfig)
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
