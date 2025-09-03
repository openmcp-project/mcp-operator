package authentication

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openmcp-project/mcp-operator/internal/utils"
	"github.com/openmcp-project/mcp-operator/internal/utils/apiserver"
	"github.com/openmcp-project/mcp-operator/internal/utils/components"

	apiserverutils "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/utils"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/authentication/config"

	"github.com/openmcp-project/controller-utils/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

// ControllerName is the name of the controller
const ControllerName = "Authentication"

// openIdConnectGVK is the GroupVersionKind for Gardener OpenIDConnect resources
var openIdConnectGVK = schema.GroupVersionKind{
	Group:   "authentication.gardener.cloud",
	Kind:    "OpenIDConnect",
	Version: "v1alpha1",
}

// The AuthenticationReconciler reconciles Authentication resources
type AuthenticationReconciler struct {
	Client          client.Client
	Config          *config.AuthenticationConfig
	APIServerAccess apiserver.APIServerAccess
}

// NewAuthenticationReconciler creates a new AuthenticationReconciler
func NewAuthenticationReconciler(c client.Client, config *config.AuthenticationConfig) *AuthenticationReconciler {
	config.SetDefaults()
	return &AuthenticationReconciler{
		Client: c,
		Config: config,
		APIServerAccess: &apiserver.APIServerAccessImpl{
			NewClient: client.New,
		},
	}
}

// SetAPIServerAccess sets the APIServerAccess implementation.
// Used for testing.
func (ar *AuthenticationReconciler) SetAPIServerAccess(apiServerAccess apiserver.APIServerAccess) {
	ar.APIServerAccess = apiServerAccess
}

// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=authentications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=authentications/status,verbs=get;update;patch

// Reconcile reconciles authentications and updates Gardener OpenIDConnect resources
func (ar *AuthenticationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, ctx := utils.InitializeControllerLogger(ctx, ControllerName)
	log.Debug(cconst.MsgStartReconcile)

	rr := ar.reconcile(ctx, req)
	rr.LogRequeue(log, logging.DEBUG)
	if rr.Component == nil {
		return rr.Result, rr.ReconcileError
	}
	if rr.ReconcileError != nil && len(rr.Conditions) == 0 { // shortcut so we don't have to add the same ready condition to each return statement (won't work anymore if we have multiple conditions)
		rr.Conditions = authenticationConditions(false, cconst.ReasonReconciliationError, cconst.MessageReconciliationError)
	}
	return components.UpdateStatus(ctx, ar.Client, rr)
}

func (ar *AuthenticationReconciler) reconcile(ctx context.Context, req ctrl.Request) components.ReconcileResult[*openmcpv1alpha1.Authentication] {
	// get the logger
	log := logging.FromContextOrPanic(ctx)

	auth := &openmcpv1alpha1.Authentication{}
	if err := ar.Client.Get(ctx, req.NamespacedName, auth); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Resource not found")
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{}
		}
		return components.ReconcileResult[*openmcpv1alpha1.Authentication]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("unable to get resource '%s' from cluster: %w", req.String(), err), cconst.ReasonCrateClusterInteractionProblem)}
	}

	// handle operation annotation
	if auth.GetAnnotations() != nil {
		op, ok := auth.GetAnnotations()[openmcpv1alpha1.OperationAnnotation]
		if ok {
			switch op {
			case openmcpv1alpha1.OperationAnnotationValueIgnore:
				log.Info("Ignoring resource due to ignore operation annotation")
				return components.ReconcileResult[*openmcpv1alpha1.Authentication]{}
			case openmcpv1alpha1.OperationAnnotationValueReconcile:
				log.Debug("Removing reconcile operation annotation from resource")
				if err := components.PatchAnnotation(ctx, ar.Client, auth, openmcpv1alpha1.OperationAnnotation, "", components.ANNOTATION_DELETE); err != nil {
					return components.ReconcileResult[*openmcpv1alpha1.Authentication]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing operation annotation: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
				}
			}
		}
	}

	// checking for APIServer component
	log.Debug("Checking for APIServer dependency")
	ownCPGeneration, ownICGeneration, _ := components.GetCreatedFromGeneration(auth)
	as := &openmcpv1alpha1.APIServer{}
	as.SetName(auth.Name)
	as.SetNamespace(auth.Namespace)

	if err := ar.Client.Get(ctx, client.ObjectKeyFromObject(as), as); err != nil {
		if !apierrors.IsNotFound(err) {
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error fetching APIServer resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
		// APIServer not found
		as = nil
	}
	if as == nil || !components.IsDependencyReady(as, ownCPGeneration, ownICGeneration) {
		log.Info("APIServer not found or it isn't ready")
		return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, Conditions: authenticationConditions(false, cconst.ReasonWaitingForDependencies, "Waiting for APIServer dependency to be ready."), Result: ctrl.Result{RequeueAfter: 60 * time.Second}}
	}

	log.Debug("APIServer dependency is ready")

	if as.Spec.Type != openmcpv1alpha1.Gardener && as.Spec.Type != openmcpv1alpha1.GardenerDedicated {
		log.Info("APIServer is not of type Gardener/GardenerDedicated")
		return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("APIServer dependency is ready, but the APIServer type is not supported"), cconst.ReasonInvalidAPIServerType)}
	}

	if as.Status.AdminAccess == nil || as.Status.AdminAccess.Kubeconfig == "" {
		return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("APIServer dependency is ready, but no kubeconfig could be found in its status"), cconst.ReasonDependencyStatusInvalid)}
	}

	apiServerClient, err := ar.APIServerAccess.GetAdminAccessClient(as, client.Options{})
	if err != nil {
		return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error creating client from APIServer kubeconfig: %w", err), cconst.ReasonDependencyStatusInvalid)}
	}

	deleteAuthentication := false
	if !auth.DeletionTimestamp.IsZero() {
		log.Info("Deleting Authentication")
		if components.HasAnyDependencyFinalizer(auth) {
			depString := strings.Join(sets.List(components.GetDependents(auth)), ", ")
			log.Info("Authentication cannot be deleted, because it still contains dependency finalizers", "dependingComponents", depString)
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, Conditions: authenticationConditions(true, cconst.ReasonDeletionWaitingForDependingComponents, fmt.Sprintf("Deletion is waiting for the following dependencies to be removed: [%s]", depString)), Result: ctrl.Result{RequeueAfter: 60 * time.Second}}
		}
		deleteAuthentication = true
	} else {
		log.Info("Triggering creation/update of Authentication")

		old := auth.DeepCopy()
		if controllerutil.AddFinalizer(auth, openmcpv1alpha1.AuthenticationComponent.Finalizer()) {
			log.Debug("Adding finalizer to Authentication resource")
			if err := ar.Client.Patch(ctx, auth, client.MergeFrom(old)); err != nil {
				return components.ReconcileResult[*openmcpv1alpha1.Authentication]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error patching finalizer on Authentication: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
			}
		}

		log.Debug("Ensuring dependency finalizer on APIServer resource")
		err := components.EnsureDependencyFinalizer(ctx, ar.Client, as, auth, true)
		if err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
	}

	old := auth.DeepCopy()
	if deleteAuthentication {
		// delete all OpenIDConnect resources if the authentication resource is being deleted
		if err = ar.deleteOpenIDConnectResources(ctx, apiServerClient, []openmcpv1alpha1.IdentityProvider{}); err != nil {
			log.Error(err, "failed to delete all OpenIDConnect resources")
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error deleting OpenIDConnect resources: %w", err), cconst.ReasonManagingOpenIDConnect)}
		}

		// delete the access secret if the authentication resource is being deleted
		if err = ar.ensureAccessSecret(ctx, false, []openmcpv1alpha1.IdentityProvider{}, auth, as); err != nil {
			log.Error(err, "failed to delete access secret")
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error deleting access secret: %w", err), cconst.ReasonManagingOpenIDConnect)}
		}

		// remove the auth dependency finalizer from the APIServer resource if the auth resource is being deleted
		err = components.EnsureDependencyFinalizer(ctx, ar.Client, as, auth, false)
		if err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}

		// remove finalizer from auth resource
		old := auth.DeepCopy()
		changed := controllerutil.RemoveFinalizer(auth, openmcpv1alpha1.AuthenticationComponent.Finalizer())
		if changed {
			if err := ar.Client.Patch(ctx, auth, client.MergeFrom(old)); err != nil {
				return components.ReconcileResult[*openmcpv1alpha1.Authentication]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing finalizer from Authentication: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
			}
		}

	} else {
		// build the list of currently enabled identity providers
		var enabledIdentityProviders []openmcpv1alpha1.IdentityProvider
		var accessSecretIdentityProviders []openmcpv1alpha1.IdentityProvider

		// add system identity provider if enabled
		if auth.IsSystemIdentityProviderEnabled() {
			enabledIdentityProviders = append(enabledIdentityProviders, ar.Config.SystemIdentityProvider)
			accessSecretIdentityProviders = append(accessSecretIdentityProviders, ar.Config.SystemIdentityProvider)
		}

		if ar.Config.CrateIdentityProvider != nil {
			// add crate identity provider if enabled
			// the crate identity provider is not added to the access secret
			enabledIdentityProviders = append(enabledIdentityProviders, *ar.Config.CrateIdentityProvider)
		}

		// add all other identity providers enabled in the control plane
		enabledIdentityProviders = append(enabledIdentityProviders, auth.Spec.IdentityProviders...)
		accessSecretIdentityProviders = append(accessSecretIdentityProviders, auth.Spec.IdentityProviders...)

		// create/update all OpenIDConnect resources that are in the list of enabled identity providers
		if len(enabledIdentityProviders) > 0 {
			if err = ar.createOrOpenIDConnectResources(ctx, apiServerClient, enabledIdentityProviders); err != nil {
				log.Error(err, "failed to create or update OpenIDConnect resources")
				return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error creating or updating OpenIDConnect resources: %w", err), cconst.ReasonManagingOpenIDConnect)}
			}
		}

		// delete all OpenIDConnect resources that are not in the list of enabled identity providers
		if err = ar.deleteOpenIDConnectResources(ctx, apiServerClient, enabledIdentityProviders); err != nil {
			log.Error(err, "failed to delete OpenIDConnect resources")
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error deleting OpenIDConnect resources: %w", err), cconst.ReasonManagingOpenIDConnect)}
		}

		// create or update the access secret
		if err = ar.ensureAccessSecret(ctx, true, accessSecretIdentityProviders, auth, as); err != nil {
			log.Error(err, "failed to ensure access secret")
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error ensuring access secret: %w", err), cconst.ReasonManagingOpenIDConnect)}
		}

		// update the external status of the Authentication resource
		if err = ar.updateExternalStatus(ctx, auth); err != nil {
			log.Error(err, "failed to update external status")
			return components.ReconcileResult[*openmcpv1alpha1.Authentication]{OldComponent: old, Component: auth, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error updating external status: %w", err), cconst.ReasonManagingOpenIDConnect)}
		}
	}

	return components.ReconcileResult[*openmcpv1alpha1.Authentication]{OldComponent: old, Component: auth, Conditions: authenticationConditions(true, "", "")}
}

// create or updates a list of Gardener OpenIDConnect resources for the given control plane
func (ar *AuthenticationReconciler) createOrOpenIDConnectResources(ctx context.Context, apiServerClient client.Client, enabledIdentityProviders []openmcpv1alpha1.IdentityProvider) error {
	log, ctx := logging.FromContextOrNew(ctx, []interface{}{cconst.KeyMethod, "createOrOpenIDConnectResources"})

	initializeOpenIDConnect := func(oidc *unstructured.Unstructured, name string) {
		oidc.SetGroupVersionKind(openIdConnectGVK)
		oidc.SetName(name)
	}

	// check if the enabled identity providers contain duplicate names
	identityProviderNames := make(map[string]struct{})

	for _, idp := range enabledIdentityProviders {
		if _, ok := identityProviderNames[idp.Name]; ok {
			return fmt.Errorf("duplicate identity provider name: %s", idp.Name)
		}

		identityProviderNames[idp.Name] = struct{}{}

		oidc := &unstructured.Unstructured{}
		initializeOpenIDConnect(oidc, idp.Name)

		operationResult, err := controllerutil.CreateOrUpdate(ctx, apiServerClient, oidc, func() error {
			isSystemIdentityProvider := idp.Name == ar.Config.SystemIdentityProvider.Name
			isCrateIdentityProvider := ar.Config.CrateIdentityProvider != nil && idp.Name == ar.Config.CrateIdentityProvider.Name

			var idpTypeLabel string

			if isSystemIdentityProvider {
				idpTypeLabel = "system"
			} else if isCrateIdentityProvider {
				idpTypeLabel = "crate"
			} else {
				idpTypeLabel = "user"
			}

			labels := oidc.GetLabels()

			if labels == nil {
				labels = map[string]string{}
			}

			labels[openmcpv1alpha1.ManagedByLabel] = ControllerName
			labels[openmcpv1alpha1.BaseDomain+"/idp-type"] = idpTypeLabel

			oidc.SetLabels(labels)

			oidc.Object["spec"] = map[string]interface{}{
				"issuerURL":      idp.IssuerURL,
				"clientID":       idp.ClientID,
				"usernameClaim":  idp.UsernameClaim,
				"groupsClaim":    idp.GroupsClaim,
				"usernamePrefix": idp.Name + ":",
				"groupsPrefix":   idp.Name + ":",
			}

			spec := oidc.Object["spec"].(map[string]interface{})

			if len(idp.CABundle) > 0 {
				spec["caBundle"] = idp.CABundle
			}

			if len(idp.SigningAlgs) > 0 {
				spec["signingAlgs"] = make([]interface{}, len(idp.SigningAlgs))

				for i, alg := range idp.SigningAlgs {
					spec["signingAlgs"].([]interface{})[i] = alg
				}
			}

			if len(idp.RequiredClaims) > 0 {
				spec["requiredClaims"] = make(map[string]interface{})

				for key, value := range idp.RequiredClaims {
					spec["requiredClaims"].(map[string]interface{})[key] = value
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to create or update Gardener OpenIDConnect resource: %w", err)
		}

		log.Debug("OpenIDConnect resource created or updated", cconst.KeyResource, idp.Name, "operationResult", operationResult)
	}

	return nil
}

// deleteOpenIDConnectResources deletes all Gardener OpenIDConnect resources that are not in the list of enabled identity providers
func (ar *AuthenticationReconciler) deleteOpenIDConnectResources(ctx context.Context, apiServerClient client.Client, enabledIdentityProviders []openmcpv1alpha1.IdentityProvider) error {
	log, ctx := logging.FromContextOrNew(ctx, []interface{}{cconst.KeyMethod, "deleteOpenIDConnectResources"})

	// list all openIDConnect resources with label managed by this controller
	oidcList := &unstructured.UnstructuredList{}
	oidcList.SetGroupVersionKind(openIdConnectGVK)

	err := apiServerClient.List(ctx, oidcList, client.MatchingLabels{openmcpv1alpha1.ManagedByLabel: ControllerName})
	if err != nil {
		return fmt.Errorf("failed to list Gardener OpenIDConnect resources: %w", err)
	}

	containsIdentityProvider := func(idps []openmcpv1alpha1.IdentityProvider, name string) bool {
		for _, idp := range idps {
			if idp.Name == name {
				return true
			}
		}
		return false
	}

	// Compare the oidc list with the enabled identity providers.
	// If an oidc resource is not in the enabled identity providers, delete it.
	for _, oidc := range oidcList.Items {
		if !containsIdentityProvider(enabledIdentityProviders, oidc.GetName()) {
			if err = apiServerClient.Delete(ctx, &oidc); err != nil {
				return fmt.Errorf("failed to delete Gardener OpenIDConnect resource: %w", err)
			}

			log.Debug("OpenIDConnect resource deleted", cconst.KeyResource, oidc.GetName())
		}
	}

	return nil
}

// ensureAccessSecret ensures that the access secret exists or does not exist (based on argument 'expected')
// and that the finalizer is removed from the secret if it exists.
// If the access secret is created or updated, it is populated with the kubeconfig containing the OIDC configuration for the user access to the APIServer cluster.
func (ar *AuthenticationReconciler) ensureAccessSecret(ctx context.Context, expected bool, enabledIdentityProviders []openmcpv1alpha1.IdentityProvider, auth *openmcpv1alpha1.Authentication, as *openmcpv1alpha1.APIServer) error {
	log, ctx := logging.FromContextOrNew(ctx, []interface{}{cconst.KeyMethod, "ensureAccessSecret"})

	accessSecret := getSecretAccessor(auth)

	if !expected {
		// get the secret if it exists
		if err := ar.Client.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error getting access secret: %w", err)
			}
		}

		// remove the finalizer from the secret
		if components.HasAnyDependencyFinalizer(accessSecret) {
			accessSecret.Finalizers = sets.NewString(accessSecret.Finalizers...).Delete(openmcpv1alpha1.AuthenticationComponent.DependencyFinalizer()).List()

			if err := ar.Client.Update(ctx, accessSecret); err != nil {
				return fmt.Errorf("error removing finalizer from access secret: %w", err)
			}
		}

		log.Debug("deleting auth access secret", cconst.KeyResource, client.ObjectKeyFromObject(accessSecret).String())

		// delete the secret if it exists
		if err := ar.Client.Delete(ctx, accessSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error deleting access secret: %w", err)
			}
		}

		return nil
	}

	// check if the access secret exists
	if as.Status.AdminAccess == nil {
		return fmt.Errorf("admin access is not available in APIServer status")
	}

	// create or update the secret
	defaultIDP := ""

	if len(auth.Spec.IdentityProviders) > 0 {
		defaultIDP = auth.Spec.IdentityProviders[0].Name
	} else if auth.IsSystemIdentityProviderEnabled() {
		defaultIDP = ar.Config.SystemIdentityProvider.Name
	}

	var oidcKubeconfig []byte

	if len(defaultIDP) >= 0 {
		restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(as.Status.AdminAccess.Kubeconfig))
		if err != nil {
			return fmt.Errorf("error creating REST config from kubeconfig: %w", err)
		}

		oidcKubeconfig, err = apiserverutils.CreateOIDCKubeconfig(ctx, ar.Client, as.Name, as.Namespace, restConfig.Host, defaultIDP, restConfig.CAData, enabledIdentityProviders)
		if err != nil {
			return fmt.Errorf("error creating OIDC kubeconfig: %w", err)
		}
	}

	result, err := controllerutil.CreateOrUpdate(ctx, ar.Client, accessSecret, func() error {
		if len(accessSecret.Finalizers) == 0 {
			accessSecret.Finalizers = make([]string, 0)
		}

		accessSecret.Finalizers = sets.NewString(accessSecret.Finalizers...).Insert(openmcpv1alpha1.AuthenticationComponent.DependencyFinalizer()).List()
		accessSecret.Data = map[string][]byte{}

		if len(oidcKubeconfig) > 0 {
			accessSecret.StringData = map[string]string{
				kubeconfigSecretValueKey: string(oidcKubeconfig),
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error creating or updating secret: %w", err)
	}

	log.Debug("access secret created or updated", cconst.KeyResource, client.ObjectKeyFromObject(accessSecret).String(), "result", result)

	return nil
}

// updateExternalStatus updates the external status of the Authentication resource
// by creating a kubeconfig for the user access to the APIServer
func (ar *AuthenticationReconciler) updateExternalStatus(ctx context.Context, auth *openmcpv1alpha1.Authentication) error {
	// try to get access secret
	accessSecret := getSecretAccessor(auth)

	secretExists := true
	if err := ar.Client.Get(ctx, client.ObjectKeyFromObject(accessSecret), accessSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting access secret: %w", err)
		}
		secretExists = false
	}

	if secretExists {
		auth.Status.UserAccess = &openmcpv1alpha1.SecretReference{
			NamespacedObjectReference: openmcpv1alpha1.NamespacedObjectReference{
				Name:      accessSecret.Name,
				Namespace: accessSecret.Namespace,
			},
			Key: kubeconfigSecretValueKey,
		}
	} else {
		auth.Status.UserAccess = nil
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (ar *AuthenticationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openmcpv1alpha1.Authentication{}, builder.WithPredicates(components.DefaultComponentControllerPredicates())).
		Watches(&openmcpv1alpha1.APIServer{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(components.StatusChangedPredicate{})).
		Complete(ar)
}

const kubeconfigSecretValueKey = "kubeconfig"

func getSecretAccessor(auth *openmcpv1alpha1.Authentication) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%s", auth.Name, kubeconfigSecretValueKey),
			Namespace: auth.Namespace,
		},
	}
}

func authenticationConditions(ready bool, reason, message string) []openmcpv1alpha1.ComponentCondition {
	return []openmcpv1alpha1.ComponentCondition{
		components.NewCondition(openmcpv1alpha1.AuthenticationComponent.HealthyCondition(), openmcpv1alpha1.ComponentConditionStatusFromBool(ready), reason, message),
	}
}
