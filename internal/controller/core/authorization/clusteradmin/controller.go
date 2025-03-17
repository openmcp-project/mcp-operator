package clusteradmin

import (
	"context"
	"fmt"
	"time"

	"github.tools.sap/CoLa/controller-utils/pkg/logging"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	authzconfig "github.tools.sap/CoLa/mcp-operator/internal/controller/core/authorization/config"
	apiserverutils "github.tools.sap/CoLa/mcp-operator/internal/utils/apiserver"
)

const (
	ControllerName = "ClusterAdmin"
)

// ClusterAdminReconciler reconciles a ClusterAdmin object
type ClusterAdminReconciler struct {
	Client           client.Client
	Config           *authzconfig.ClusterAdmin
	APIServerAccess  apiserverutils.APIServerAccess
	EventBroadcaster record.EventBroadcaster
	EventRecorder    record.EventRecorder
}

// NewClusterAdminReconciler creates a new ClusterAdminReconciler
func NewClusterAdminReconciler(c client.Client, config *authzconfig.AuthorizationConfig) *ClusterAdminReconciler {
	return &ClusterAdminReconciler{
		Client: c,
		Config: &config.ClusterAdmin,
		APIServerAccess: &apiserverutils.APIServerAccessImpl{
			NewClient: client.New,
		},
		EventBroadcaster: record.NewBroadcaster(),
	}
}

func (car *ClusterAdminReconciler) SetAPIServerAccess(apiServerAccess apiserverutils.APIServerAccess) *ClusterAdminReconciler {
	car.APIServerAccess = apiServerAccess
	return car
}

// SetupWithManager sets up the controller with the controller-runtime manager
func (car *ClusterAdminReconciler) SetupWithManager(mgr ctrl.Manager) error {
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	car.EventBroadcaster.StartStructuredLogging(4)
	car.EventBroadcaster.StartRecordingToSink(&v1.EventSinkImpl{
		Interface: cs.CoreV1().Events(""),
	})
	car.EventRecorder = car.EventBroadcaster.NewRecorder(mgr.GetScheme(), corev1.EventSource{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&openmcpv1alpha1.ClusterAdmin{}).
		Complete(car)
}

// Reconcile reconciles the ClusterAdmin object
func (car *ClusterAdminReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error

	// get the logger
	log := logging.FromContextOrPanic(ctx)

	ca := &openmcpv1alpha1.ClusterAdmin{}
	if err = car.Client.Get(ctx, req.NamespacedName, ca); err != nil {
		log.Error(err, "unable to fetch ClusterAdmin")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	authz := &openmcpv1alpha1.Authorization{}
	err = car.Client.Get(ctx, client.ObjectKey{Name: ca.Name, Namespace: ca.Namespace}, authz)
	if err != nil {
		log.Error(err, "unable to fetch Authorization for ClusterAdmin")
		return ctrl.Result{}, err
	}

	apiServer := &openmcpv1alpha1.APIServer{}
	err = car.Client.Get(ctx, client.ObjectKey{Name: ca.Name, Namespace: ca.Namespace}, apiServer)
	if err != nil {
		log.Error(err, "unable to fetch APIServer for ClusterAdmin")
		return ctrl.Result{}, err
	}

	if apiServer.Status.AdminAccess == nil || apiServer.Status.AdminAccess.Kubeconfig == "" {
		log.Debug("APIServer admin access not ready yet")
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	apiServerClient, err := car.APIServerAccess.GetAdminAccessClient(apiServer, client.Options{})
	if err != nil {
		log.Error(err, "unable to get APIServer admin access client")
		return ctrl.Result{}, err
	}

	if !ca.DeletionTimestamp.IsZero() {
		log.Debug("ClusterAdmin is being deleted")
		// deletion
		return ctrl.Result{}, car.handleDelete(ctx, ca, apiServerClient)
	} else {
		log.Debug("ClusterAdmin is being created/updated")
		// creation/update
		return car.handleCreateUpdate(ctx, ca, apiServerClient)
	}
}

// emitActivatedEvent emits a K8S event when the cluster admin is activated
func (car *ClusterAdminReconciler) emitActivatedEvent(ctx context.Context, ca *openmcpv1alpha1.ClusterAdmin) {
	if car.EventRecorder == nil {
		return
	}

	log := logging.FromContextOrPanic(ctx)
	log.Debug("Cluster admin activated", "validUntil", ca.Status.Expiration, "subjects", ca.Spec.Subjects)

	mcp := &openmcpv1alpha1.ManagedControlPlane{}
	err := car.Client.Get(ctx, client.ObjectKey{Name: ca.Name, Namespace: ca.Namespace}, mcp)
	if err == nil {
		car.EventRecorder.Eventf(mcp, corev1.EventTypeWarning, "ClusterAdminActivated", "Cluster admin activated (valid until %s) with subjects: %v", ca.Status.Expiration.Format(time.RFC3339), ca.Spec.Subjects)
	}
}

// emitDeactivatedEvent emits a K8S event when the cluster admin is deactivated
func (car *ClusterAdminReconciler) emitDeactivatedEvent(ctx context.Context, ca *openmcpv1alpha1.ClusterAdmin) {
	if car.EventRecorder == nil {
		return
	}

	log := logging.FromContextOrPanic(ctx)
	log.Debug("Cluster admin deactivated")

	mcp := &openmcpv1alpha1.ManagedControlPlane{}
	err := car.Client.Get(ctx, client.ObjectKey{Name: ca.Name, Namespace: ca.Namespace}, mcp)
	if err == nil {
		car.EventRecorder.Eventf(mcp, corev1.EventTypeWarning, "ClusterAdminDeactivated", "Cluster admin deactivated")
	}
}

// handleCreateUpdate handles the creation and update of the ClusterAdmin object
func (car *ClusterAdminReconciler) handleCreateUpdate(ctx context.Context, ca *openmcpv1alpha1.ClusterAdmin, apiServerClient client.Client) (reconcile.Result, error) {
	var err error

	mutateClusterRoleBinding := func(clusterRoleBinding *rbacv1.ClusterRoleBinding, subjects []openmcpv1alpha1.Subject) {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     openmcpv1alpha1.ClusterAdminRole,
		}
		clusterRoleBinding.Labels = map[string]string{
			openmcpv1alpha1.ManagedByLabel: ControllerName,
		}
		clusterRoleBinding.Subjects = make([]rbacv1.Subject, 0, len(subjects))
		for _, subject := range subjects {
			clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, rbacv1.Subject{
				Kind:      subject.Kind,
				Name:      subject.Name,
				Namespace: subject.Namespace,
				APIGroup:  subject.APIGroup,
			})
		}
	}

	if controllerutil.AddFinalizer(ca, openmcpv1alpha1.AuthorizationComponent.Finalizer()) {
		if err := car.Client.Update(ctx, ca); err != nil {
			return reconcile.Result{}, err
		}
	}

	if ca.Status.Active {
		// was activated before
		if ca.Status.Activated == nil || ca.Status.Expiration == nil {
			return reconcile.Result{}, fmt.Errorf("cluster admin status is active, but activated or expiration time is missing")
		}
		// get now
		now := metav1.Now()
		// if is activated for more than the desired duration, then deactivate
		if now.Sub(ca.Status.Activated.Time) >= car.Config.ActiveDuration.Duration {
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.ClusterAdminRoleBinding,
				},
			}

			err = apiServerClient.Delete(ctx, clusterRoleBinding)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return reconcile.Result{}, err
				}
			}

			ca.Status.Active = false

			if err = car.Client.Status().Update(ctx, ca); err != nil {
				return reconcile.Result{}, err
			}

			car.emitDeactivatedEvent(ctx, ca)

		} else {
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.ClusterAdminRoleBinding,
				},
			}
			_, err = controllerutil.CreateOrUpdate(ctx, apiServerClient, clusterRoleBinding, func() error {
				mutateClusterRoleBinding(clusterRoleBinding, ca.Spec.Subjects)
				return nil
			})

			if err != nil {
				return reconcile.Result{}, err
			}

			// reconcile before expiration
			return reconcile.Result{
				RequeueAfter: ca.Status.Expiration.Sub(now.Time),
			}, nil
		}
	}

	if ca.Status.Activated == nil {
		// was not activated before
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: openmcpv1alpha1.ClusterAdminRoleBinding,
			},
		}
		_, err = controllerutil.CreateOrUpdate(ctx, apiServerClient, clusterRoleBinding, func() error {
			mutateClusterRoleBinding(clusterRoleBinding, ca.Spec.Subjects)
			return nil
		})

		if err != nil {
			return reconcile.Result{}, err
		}

		ca.Status.Active = true
		ca.Status.Activated = ptr.To(metav1.Now())
		ca.Status.Expiration = ptr.To(metav1.Time{Time: ca.Status.Activated.Add(car.Config.ActiveDuration.Duration)})

		if err = car.Client.Status().Update(ctx, ca); err != nil {
			return reconcile.Result{}, err
		}

		car.emitActivatedEvent(ctx, ca)

		return reconcile.Result{
			RequeueAfter: car.Config.ActiveDuration.Duration,
		}, nil
	}

	return reconcile.Result{}, nil
}

// handleDelete handles the deletion of the ClusterAdmin object
func (car *ClusterAdminReconciler) handleDelete(ctx context.Context, ca *openmcpv1alpha1.ClusterAdmin, apiServerClient client.Client) error {
	var err error

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: openmcpv1alpha1.ClusterAdminRoleBinding,
		},
	}

	err = apiServerClient.Delete(ctx, clusterRoleBinding)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if controllerutil.RemoveFinalizer(ca, openmcpv1alpha1.AuthorizationComponent.Finalizer()) {
		err = car.Client.Update(ctx, ca)
		if err != nil {
			return err
		}
	}

	if ca.Status.Active {
		car.emitDeactivatedEvent(ctx, ca)
	}

	return nil
}
