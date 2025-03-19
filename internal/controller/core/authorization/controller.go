package authorization

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/openmcp-project/controller-utils/pkg/logging"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
	"github.com/openmcp-project/mcp-operator/internal/components"
	authzconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/authorization/config"
	"github.com/openmcp-project/mcp-operator/internal/utils"
	apiserverutils "github.com/openmcp-project/mcp-operator/internal/utils/apiserver"
	componentutils "github.com/openmcp-project/mcp-operator/internal/utils/components"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const ControllerName = "Authorization"

type AuthorizationReconciler struct {
	Client          client.Client
	Config          *authzconfig.AuthorizationConfig
	APIServerAccess apiserverutils.APIServerAccess
}

func NewAuthorizationReconciler(c client.Client, config *authzconfig.AuthorizationConfig) *AuthorizationReconciler {
	config.SetDefaults()
	return &AuthorizationReconciler{
		Client: c,
		Config: config,
		APIServerAccess: &apiserverutils.APIServerAccessImpl{
			NewClient: client.New,
		},
	}
}

// SetAPIServerAccess sets the APIServerAccess implementation.
// Used for testing.
func (ar *AuthorizationReconciler) SetAPIServerAccess(apiServerAccess apiserverutils.APIServerAccess) {
	ar.APIServerAccess = apiServerAccess
}

// +kubebuilder:rbac:groups=authorization.k8s.io,resources=authorizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=authorizations/status,verbs=get;update;patch

func (ar *AuthorizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, ctx := utils.InitializeControllerLogger(ctx, ControllerName)
	log.Debug(cconst.MsgStartReconcile)

	rr := ar.reconcile(ctx, req)
	rr.LogRequeue(log, logging.DEBUG)
	if rr.Component == nil {
		return rr.Result, rr.ReconcileError
	}
	if rr.ReconcileError != nil && len(rr.Conditions) == 0 { // shortcut so we don't have to add the same ready condition to each return statement (won't work anymore if we have multiple conditions)
		rr.Conditions = authorizationConditions(false, cconst.ReasonReconciliationError, cconst.MessageReconciliationError)
	}
	return componentutils.UpdateStatus(ctx, ar.Client, rr)
}

func (ar *AuthorizationReconciler) reconcile(ctx context.Context, req ctrl.Request) componentutils.ReconcileResult[*openmcpv1alpha1.Authorization] {
	// get the logger
	log := logging.FromContextOrPanic(ctx)

	authz := &openmcpv1alpha1.Authorization{}
	if err := ar.Client.Get(ctx, req.NamespacedName, authz); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Resource not found")
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{}
		}
		return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("unable to get resource '%s' from cluster: %w", req.NamespacedName.String(), err), cconst.ReasonCrateClusterInteractionProblem)}
	}

	// handle operation annotation
	if authz.GetAnnotations() != nil {
		op, ok := authz.GetAnnotations()[openmcpv1alpha1.OperationAnnotation]
		if ok {
			switch op {
			case openmcpv1alpha1.OperationAnnotationValueIgnore:
				log.Info("Ignoring resource due to ignore operation annotation")
				return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{}
			case openmcpv1alpha1.OperationAnnotationValueReconcile:
				log.Debug("Removing reconcile operation annotation from resource")
				if err := componentutils.PatchAnnotation(ctx, ar.Client, authz, openmcpv1alpha1.OperationAnnotation, "", componentutils.ANNOTATION_DELETE); err != nil {
					return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing operation annotation: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
				}
			}
		}
	}

	// checking for APIServer component
	log.Debug("Checking for APIServer dependency")
	ownCPGeneration, ownICGeneration, _ := componentutils.GetCreatedFromGeneration(authz)
	as := &openmcpv1alpha1.APIServer{}
	as.SetName(authz.Name)
	as.SetNamespace(authz.Namespace)

	if err := ar.Client.Get(ctx, client.ObjectKeyFromObject(as), as); err != nil {
		if !apierrors.IsNotFound(err) {
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error fetching APIServer resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
		// APIServer not found
		as = nil
	}
	if as == nil || !componentutils.IsDependencyReady(as, ownCPGeneration, ownICGeneration) {
		log.Info("APIServer not found or it isn't ready")
		return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, Conditions: authorizationConditions(false, cconst.ReasonWaitingForDependencies, "Waiting for APIServer dependency to be ready"), Result: reconcile.Result{RequeueAfter: 60 * time.Second}}
	}

	log.Debug("APIServer dependency is ready")

	if as.Status.AdminAccess == nil || as.Status.AdminAccess.Kubeconfig == "" {
		return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, ReconcileError: openmcperrors.WithReason(fmt.Errorf("APIServer dependency is ready, but no kubeconfig could be found in its status"), cconst.ReasonDependencyStatusInvalid)}
	}

	apiServerClient, err := ar.APIServerAccess.GetAdminAccessClient(as, client.Options{})
	if err != nil {
		return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error creating client from APIServer kubeconfig: %w", err), cconst.ReasonDependencyStatusInvalid)}
	}

	old := authz.DeepCopy()
	if !authz.DeletionTimestamp.IsZero() {
		log.Info("Deleting Authorization")
		if componentutils.HasAnyDependencyFinalizer(authz) {
			depString := strings.Join(sets.List(componentutils.GetDependents(authz)), ", ")
			log.Info("Authorization cannot be deleted, because it still contains dependency finalizers", "dependingComponents", depString)
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, Conditions: authorizationConditions(true, cconst.ReasonDeletionWaitingForDependingComponents, fmt.Sprintf("Deletion is waiting for the following dependencies to be removed: [%s]", depString)), Result: ctrl.Result{RequeueAfter: 60 * time.Second}}
		}

		log.Info("Deleting Authorization")
		if err = ar.deleteAuthorization(ctx, apiServerClient); err != nil {
			log.Error(err, "error deleting authorization resources")
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, ReconcileError: openmcperrors.WithReason(err, cconst.ReasonManagingAuthorization)}
		}

		// remove the auth dependency finalizer from the APIServer resource if the auth resource is being deleted
		err = componentutils.EnsureDependencyFinalizer(ctx, ar.Client, as, authz, false)
		if err != nil {
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}

		// remove finalizer from authz resource
		old := authz.DeepCopy()
		changed := controllerutil.RemoveFinalizer(authz, openmcpv1alpha1.AuthorizationComponent.Finalizer())
		if changed {
			if err := ar.Client.Patch(ctx, authz, client.MergeFrom(old)); err != nil {
				return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing finalizer from Authorization: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
			}
		}
	} else {
		log.Info("Triggering creation/update of Authorization")

		if controllerutil.AddFinalizer(authz, openmcpv1alpha1.AuthorizationComponent.Finalizer()) {
			log.Debug("Adding finalizer to Authorization resource")
			if err := ar.Client.Patch(ctx, authz, client.MergeFrom(old)); err != nil {
				return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error patching finalizer on Authorization: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
			}
		}

		log.Debug("Ensuring dependency finalizer on APIServer resource")
		err = componentutils.EnsureDependencyFinalizer(ctx, ar.Client, as, authz, true)
		if err != nil {
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{Component: authz, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}

		log.Info("Creating/Updating Authorization")
		if err = ar.ensureClusterRoles(ctx, apiServerClient, authz); err != nil {
			log.Error(err, "error creating/updating cluster roles")
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{OldComponent: old, Component: authz, ReconcileError: openmcperrors.WithReason(err, cconst.ReasonManagingAuthorization)}
		}

		if err = ar.ensureClusterRoleBindings(ctx, apiServerClient, authz); err != nil {
			log.Error(err, "error ensuring cluster role bindings")
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{OldComponent: old, Component: authz, ReconcileError: openmcperrors.WithReason(err, cconst.ReasonManagingAuthorization)}
		}

		if err = ar.ensureRoleBindings(ctx, apiServerClient, authz); err != nil {
			log.Error(err, "error ensuring role bindings")
			return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{OldComponent: old, Component: authz, ReconcileError: openmcperrors.WithReason(err, cconst.ReasonManagingAuthorization)}
		}
	}

	return componentutils.ReconcileResult[*openmcpv1alpha1.Authorization]{OldComponent: old, Component: authz, Conditions: authorizationConditions(true, "", "")}
}

// ensureClusterRoles creates or updates the cluster roles as defined in the configuration
func (ar *AuthorizationReconciler) ensureClusterRoles(ctx context.Context, apiServerClient client.Client, authz *openmcpv1alpha1.Authorization) error {
	log, ctx := logging.FromContextOrNew(ctx, []interface{}{})

	createOrUpdateClusterRole := func(name string, cfg *authzconfig.RulesConfig, setLabels, setAggregation bool) error {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		result, err := controllerutil.CreateOrUpdate(ctx, apiServerClient, clusterRole, func() error {
			clusterRole.Rules = make([]rbacv1.PolicyRule, len(cfg.Rules))
			copy(clusterRole.Rules, cfg.Rules)

			if clusterRole.Name == openmcpv1alpha1.AdminClusterScopeStandardRulesRole && len(authz.Status.UserNamespaces) > 0 {
				nsRule := rbacv1.PolicyRule{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					Verbs:         []string{"update", "patch", "delete"},
					ResourceNames: make([]string, len(authz.Status.UserNamespaces)),
				}
				copy(nsRule.ResourceNames, authz.Status.UserNamespaces)

				clusterRole.Rules = append(clusterRole.Rules, nsRule)
			}

			clusterRole.Labels = map[string]string{
				openmcpv1alpha1.ManagedByLabel: ControllerName,
			}

			if setLabels {
				for key, val := range cfg.Labels {
					clusterRole.Labels[key] = val
				}
			}

			if setAggregation {
				clusterRole.AggregationRule = &rbacv1.AggregationRule{
					ClusterRoleSelectors: make([]metav1.LabelSelector, len(cfg.ClusterRoleSelectors)),
				}
				copy(clusterRole.AggregationRule.ClusterRoleSelectors, cfg.ClusterRoleSelectors)
				for _, comp := range components.Registry.GetKnownComponents() {
					ls := comp.LabelSelectorsForRole(name)
					if ls != nil {
						clusterRole.AggregationRule.ClusterRoleSelectors = append(clusterRole.AggregationRule.ClusterRoleSelectors, ls...)
					}
				}
			} else {
				clusterRole.AggregationRule = nil
			}
			return nil
		})

		if err != nil {
			return err
		}

		log.Debug("Cluster role created/updated", "result", result, cconst.KeyResource, clusterRole.Name)
		return nil
	}

	allErrs := field.ErrorList{}
	clusterRoleNames := openmcpv1alpha1.GetClusterRoleNames()

	for _, name := range clusterRoleNames {
		isAggregatedRole := openmcpv1alpha1.IsAggregatedRole(name)
		rulesConfig := ar.Config.GetRulesConfig(name)

		if err := createOrUpdateClusterRole(name, rulesConfig, !isAggregatedRole, isAggregatedRole); err != nil {
			allErrs = append(allErrs, field.InternalError(field.NewPath(name), err))
		}
	}

	return allErrs.ToAggregate()
}

// updateClusterRoleBindingSubjects updates the subjects of a cluster role binding
func updateClusterRoleBindingSubjects(clusterRoleBinding *rbacv1.ClusterRoleBinding, staticSubjects []rbacv1.Subject, dynamicSubjects []openmcpv1alpha1.Subject) {
	numSubjects := len(staticSubjects) + len(dynamicSubjects)

	if numSubjects == 0 {
		clusterRoleBinding.Subjects = nil
		return
	}

	clusterRoleBinding.Subjects = make([]rbacv1.Subject, 0, numSubjects)
	clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, staticSubjects...)

	for _, subject := range dynamicSubjects {
		clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, rbacv1.Subject{
			Kind:      subject.Kind,
			Name:      subject.Name,
			Namespace: subject.Namespace,
			APIGroup:  subject.APIGroup,
		})
	}
}

// ensureClusterRoleBindings is a cyclic task that creates or updates the admin and view cluster role bindings
func (ar *AuthorizationReconciler) ensureClusterRoleBindings(ctx context.Context, apiServerClient client.Client, authz *openmcpv1alpha1.Authorization) error {
	adminClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: openmcpv1alpha1.AdminClusterRoleBinding,
		},
	}

	allErrs := field.ErrorList{}

	adminRole := authz.Spec.GetRoleForName(openmcpv1alpha1.RoleBindingRoleAdmin)
	_, err := controllerutil.CreateOrUpdate(ctx, apiServerClient, adminClusterRoleBinding, func() error {
		adminClusterRoleBinding.Labels = map[string]string{
			openmcpv1alpha1.ManagedByLabel: ControllerName,
		}

		adminClusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     openmcpv1alpha1.AdminClusterScopeRole,
		}

		var dynamicSubjects []openmcpv1alpha1.Subject
		if adminRole != nil {
			dynamicSubjects = adminRole.Subjects
		}
		updateClusterRoleBindingSubjects(adminClusterRoleBinding, ar.Config.Admin.AdditionalSubjects, dynamicSubjects)

		return nil
	})

	if err != nil {
		allErrs = append(allErrs, field.InternalError(field.NewPath("admin"), err))
	}

	viewClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: openmcpv1alpha1.ViewClusterRoleBinding,
		},
	}

	viewRole := authz.Spec.GetRoleForName(openmcpv1alpha1.RoleBindingRoleView)
	_, err = controllerutil.CreateOrUpdate(ctx, apiServerClient, viewClusterRoleBinding, func() error {
		viewClusterRoleBinding.Labels = map[string]string{
			openmcpv1alpha1.ManagedByLabel: ControllerName,
		}

		viewClusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     openmcpv1alpha1.ViewClusterScopeRole,
		}

		var dynamicSubjects []openmcpv1alpha1.Subject
		if viewRole != nil {
			dynamicSubjects = viewRole.Subjects
		}
		updateClusterRoleBindingSubjects(viewClusterRoleBinding, ar.Config.View.AdditionalSubjects, dynamicSubjects)

		return nil
	})

	if err != nil {
		allErrs = append(allErrs, field.InternalError(field.NewPath("view"), err))
	}

	return allErrs.ToAggregate()
}

// updateRoleBindingSubjects updates the subjects of a role binding
func updateRoleBindingSubjects(roleBinding *rbacv1.RoleBinding, staticSubjects []rbacv1.Subject, dynamicSubjects []openmcpv1alpha1.Subject) {
	numSubjects := len(staticSubjects) + len(dynamicSubjects)

	if numSubjects == 0 {
		roleBinding.Subjects = nil
		return
	}

	roleBinding.Subjects = make([]rbacv1.Subject, 0, numSubjects)
	roleBinding.Subjects = append(roleBinding.Subjects, staticSubjects...)

	for _, subject := range dynamicSubjects {
		roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
			Kind:      subject.Kind,
			Name:      subject.Name,
			Namespace: subject.Namespace,
			APIGroup:  subject.APIGroup,
		})
	}
}

// ensureRoleBindings creates or updates the admin and view role bindings
func (ar *AuthorizationReconciler) ensureRoleBindings(ctx context.Context, apiServerClient client.Client, authz *openmcpv1alpha1.Authorization) error {
	allErrs := field.ErrorList{}
	adminRole := authz.Spec.GetRoleForName(openmcpv1alpha1.RoleBindingRoleAdmin)
	viewRole := authz.Spec.GetRoleForName(openmcpv1alpha1.RoleBindingRoleView)

	for _, ns := range authz.Status.UserNamespaces {
		namespace := &corev1.Namespace{}
		if err := apiServerClient.Get(ctx, client.ObjectKey{Name: ns}, namespace); err != nil {
			allErrs = append(allErrs, field.InternalError(field.NewPath(ns), err))
			continue
		}

		// ignore namespace with deletion timestamp
		if namespace.DeletionTimestamp != nil {
			continue
		}

		adminRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openmcpv1alpha1.AdminRoleBinding,
				Namespace: ns,
			},
		}

		_, err := controllerutil.CreateOrUpdate(ctx, apiServerClient, adminRoleBinding, func() error {
			adminRoleBinding.Labels = map[string]string{
				openmcpv1alpha1.ManagedByLabel: ControllerName,
			}

			adminRoleBinding.RoleRef = rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     openmcpv1alpha1.AdminNamespaceScopeRole,
			}

			var dynamicSubjects []openmcpv1alpha1.Subject
			if adminRole != nil {
				dynamicSubjects = adminRole.Subjects
			}
			updateRoleBindingSubjects(adminRoleBinding, ar.Config.Admin.AdditionalSubjects, dynamicSubjects)

			return nil
		})

		if err != nil {
			allErrs = append(allErrs, field.InternalError(field.NewPath(ns).Child("admin"), err))
		}

		viewRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openmcpv1alpha1.ViewRoleBinding,
				Namespace: ns,
			},
		}

		_, err = controllerutil.CreateOrUpdate(ctx, apiServerClient, viewRoleBinding, func() error {
			viewRoleBinding.Labels = map[string]string{
				openmcpv1alpha1.ManagedByLabel: ControllerName,
			}

			viewRoleBinding.RoleRef = rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     openmcpv1alpha1.ViewNamespaceScopeRole,
			}

			var dynamicSubjects []openmcpv1alpha1.Subject
			if viewRole != nil {
				dynamicSubjects = viewRole.Subjects
			}
			updateRoleBindingSubjects(viewRoleBinding, ar.Config.View.AdditionalSubjects, dynamicSubjects)

			return nil
		})

		if err != nil {
			allErrs = append(allErrs, field.InternalError(field.NewPath(ns).Child("view"), err))
		}

	}

	return allErrs.ToAggregate()
}

// deleteAuthorization deletes all cluster roles, cluster role bindings and role bindings
func (ar *AuthorizationReconciler) deleteAuthorization(ctx context.Context, apiServerClient client.Client) error {
	allErrs := field.ErrorList{}

	path := field.NewPath("roleBindings")
	roleBindings := rbacv1.RoleBindingList{}
	if err := apiServerClient.List(ctx, &roleBindings, client.MatchingLabels{
		openmcpv1alpha1.ManagedByLabel: ControllerName,
	}); err != nil {
		allErrs = append(allErrs, field.InternalError(path, err))
	}

	for _, roleBinding := range roleBindings.Items {
		if err := apiServerClient.Delete(ctx, &roleBinding); err != nil {
			allErrs = append(allErrs, field.InternalError(path.Child(roleBinding.Name), err))
		}
	}

	path = field.NewPath("clusterRoleBindings")
	clusterRoleBindings := rbacv1.ClusterRoleBindingList{}
	if err := apiServerClient.List(ctx, &clusterRoleBindings, client.MatchingLabels{
		openmcpv1alpha1.ManagedByLabel: ControllerName,
	}); err != nil {
		allErrs = append(allErrs, field.InternalError(path, err))
	}

	for _, clusterRoleBinding := range clusterRoleBindings.Items {
		if err := apiServerClient.Delete(ctx, &clusterRoleBinding); err != nil {
			allErrs = append(allErrs, field.InternalError(path.Child(clusterRoleBinding.Name), err))
		}
	}

	path = field.NewPath("clusterRoles")
	clusterRoles := rbacv1.ClusterRoleList{}
	if err := apiServerClient.List(ctx, &clusterRoles, client.MatchingLabels{
		openmcpv1alpha1.ManagedByLabel: ControllerName,
	}); err != nil {
		allErrs = append(allErrs, field.InternalError(path, err))
	}

	for _, clusterRole := range clusterRoles.Items {
		if err := apiServerClient.Delete(ctx, &clusterRole); err != nil {
			allErrs = append(allErrs, field.InternalError(path.Child(clusterRole.Name), err))
		}
	}

	return allErrs.ToAggregate()
}

// namespacesTask is a cyclic task that updates the Authorization object with the list of user namespaces from the APIServer
func (ar *AuthorizationReconciler) namespacesTask(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient, apiServerClient client.Client) error {
	// get the authorization object
	authz := &openmcpv1alpha1.Authorization{}
	if err := crateClient.Get(ctx, client.ObjectKeyFromObject(as), authz); err != nil {
		// the APIServer has no corresponding Authorization object
		return nil
	}

	old := authz.DeepCopy()

	// get the namespaces from the APIServer
	nsList := &corev1.NamespaceList{}
	if err := apiServerClient.List(ctx, nsList); err != nil {
		return fmt.Errorf("error fetching namespaces from APIServer: %w", err)
	}

	userNamespaces := make([]string, 0, len(nsList.Items))

	// filter out the namespaces that are not allowed
	for _, ns := range nsList.Items {
		if !ar.Config.IsAllowedNamespaceName(ns.Name) {
			continue
		}

		// ignore namespace with deletion timestamp
		if ns.DeletionTimestamp != nil {
			continue
		}

		userNamespaces = append(userNamespaces, ns.Name)
	}

	if len(userNamespaces) > 0 {
		authz.Status.UserNamespaces = userNamespaces
	} else {
		authz.Status.UserNamespaces = nil
	}

	if !reflect.DeepEqual(old.Status, authz.Status) {
		if err := crateClient.Status().Patch(ctx, authz, client.MergeFrom(old)); err != nil {
			return fmt.Errorf("error updating Authorization status: %w", err)
		}

		if authz.GetAnnotations() == nil {
			authz.SetAnnotations(make(map[string]string))
		}
		authz.Annotations[openmcpv1alpha1.OperationAnnotation] = openmcpv1alpha1.OperationAnnotationValueReconcile
		if err := crateClient.Update(ctx, authz); err != nil {
			return fmt.Errorf("error updating Authorization object: %w", err)
		}
	}

	return nil
}

func (ar *AuthorizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openmcpv1alpha1.Authorization{}, builder.WithPredicates(componentutils.DefaultComponentControllerPredicates())).
		Watches(&openmcpv1alpha1.APIServer{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(componentutils.StatusChangedPredicate{})).
		Complete(ar)
}

// RegisterTasks registers the cyclic tasks for the AuthorizationReconciler
func (ar *AuthorizationReconciler) RegisterTasks(worker apiserverutils.Worker) *AuthorizationReconciler {
	worker.RegisterTask("authz_namespaces", ar.namespacesTask)
	return ar
}

func authorizationConditions(ready bool, reason, message string) []openmcpv1alpha1.ComponentCondition {
	return []openmcpv1alpha1.ComponentCondition{
		componentutils.NewCondition(openmcpv1alpha1.AuthorizationComponent.HealthyCondition(), openmcpv1alpha1.ComponentConditionStatusFromBool(ready), reason, message),
	}
}
