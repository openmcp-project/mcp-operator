package apiserver

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/openmcp-project/controller-utils/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/controller-utils/pkg/clusteraccess"
	ctrlutils "github.com/openmcp-project/controller-utils/pkg/controller"
	"github.com/openmcp-project/controller-utils/pkg/resources"

	gcpv1alpha1 "github.com/openmcp-project/cluster-provider-gardener/api/core/v1alpha1"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	clustersconst "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1/constants"

	gardenv1beta1 "github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
	handler "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler"
)

func v2HandleCreateOrUpdate(ctx context.Context, as *openmcpv1alpha1.APIServer, platformClient client.Client) (ctrl.Result, handler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx).WithName(openmcpv1alpha1.ArchitectureV2)
	ctx = logging.NewContext(ctx, log)

	// instead of calling a handler, create a ClusterRequest and an AccessRequest
	// ensure namespace, because this is created on the platform cluster
	nsName := fmt.Sprintf("mcp-%s", ctrlutils.K8sNameHash(as.Namespace))
	nsm := resources.NewNamespaceMutator(nsName)
	nsm.MetadataMutator().WithLabels(map[string]string{
		openmcpv1alpha1.V1MCPReferenceLabelNamespace: as.Namespace,
	})
	if err := resources.CreateOrUpdateResource(ctx, platformClient, nsm); err != nil {
		return ctrl.Result{}, nil, nil, openmcperrors.WithReason(fmt.Errorf("failed to create or update namespace %s: %w", nsName, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}

	// create or update ClusterRequest
	var purpose string
	switch as.Spec.Type {
	case openmcpv1alpha1.Gardener:
		purpose = "mcp"
	case openmcpv1alpha1.GardenerDedicated:
		purpose = "mcp-worker"
	default:
		return ctrl.Result{}, nil, nil, openmcperrors.WithReason(fmt.Errorf("unknown APIServer type %s", as.Spec.Type), clustersconst.ReasonConfigurationProblem)
	}
	cr := &clustersv1alpha1.ClusterRequest{}
	cr.Name = as.Name
	cr.Namespace = nsName
	crm := NewClusterRequestMutator(cr.Name, cr.Namespace, purpose)
	crm.MetadataMutator().WithLabels(map[string]string{
		openmcpv1alpha1.V1MCPReferenceLabelName:      as.Name,
		openmcpv1alpha1.V1MCPReferenceLabelNamespace: as.Namespace,
	})
	if err := resources.CreateOrUpdateResource(ctx, platformClient, crm); err != nil {
		return ctrl.Result{}, nil, nil, openmcperrors.WithReason(fmt.Errorf("failed to create or update ClusterRequest %s/%s: %w", cr.Namespace, cr.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}

	// if the ClusterRequest is granted, fetch the corresponding cluster
	if err := platformClient.Get(ctx, client.ObjectKeyFromObject(cr), cr); err != nil {
		return ctrl.Result{}, nil, nil, openmcperrors.WithReason(fmt.Errorf("failed to get ClusterRequest %s/%s: %w", cr.Namespace, cr.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}
	var setShootInStatus handler.UpdateStatusFunc
	if cr.Status.Phase == clustersv1alpha1.REQUEST_GRANTED && cr.Status.Cluster != nil {
		// fetch Cluster resource
		cluster := &clustersv1alpha1.Cluster{}
		cluster.Name = cr.Status.Cluster.Name
		cluster.Namespace = cr.Status.Cluster.Namespace
		if err := platformClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
			return ctrl.Result{}, nil, nil, openmcperrors.WithReason(fmt.Errorf("failed to get Cluster %s/%s: %w", cluster.Namespace, cluster.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
		}
		// check if there is a shoot manifest in the Cluster status
		// if so, copy it into the APIServer status
		if cluster.Status.ProviderStatus != nil {
			cs := &gcpv1alpha1.ClusterStatus{}
			if err := cluster.Status.GetProviderStatus(cs); err != nil {
				return ctrl.Result{}, nil, nil, openmcperrors.WithReason(fmt.Errorf("error unmarshalling provider status: %w", err), clustersconst.ReasonInternalError)
			}
			log.Debug("Provider status found, checking for shoot manifest")
			if cs.Shoot != nil {
				log.Debug("Found shoot in provider status", "shootName", cs.Shoot.GetName(), "shootNamespace", cs.Shoot.GetNamespace())
				setShootInStatus = func(status *openmcpv1alpha1.APIServerStatus) error {
					status.GardenerStatus = &openmcpv1alpha1.GardenerStatus{}
					uShoot := &unstructured.Unstructured{}
					data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cs.Shoot)
					if err != nil {
						return fmt.Errorf("unable to convert shoot to unstructured object: %w", err)
					}
					uShoot.SetUnstructuredContent(data)
					// ensure type information is set
					uShoot.SetAPIVersion(gardenv1beta1.SchemeGroupVersion.String())
					uShoot.SetKind("Shoot")
					// delete fields that should not be part of the shoot manifest in the status
					uShoot.SetFinalizers(nil)
					uShoot.SetResourceVersion("")
					uShoot.SetCreationTimestamp(metav1.Time{})
					uShoot.SetGenerateName("")
					uShoot.SetGeneration(0)
					uShoot.SetManagedFields(nil)
					uShoot.SetDeletionGracePeriodSeconds(nil)
					uShoot.SetDeletionTimestamp(nil)
					uShoot.SetOwnerReferences(nil)

					status.GardenerStatus.Shoot = &runtime.RawExtension{Object: uShoot}
					return nil
				}

			}
		}
	}

	// build the UpdateStatusFunc
	apiAccess := &openmcpv1alpha1.APIServerAccess{}
	var usf handler.UpdateStatusFunc = func(status *openmcpv1alpha1.APIServerStatus) error {
		if status.ExternalAPIServerStatus == nil {
			status.ExternalAPIServerStatus = &openmcpv1alpha1.ExternalAPIServerStatus{}
		}
		if setShootInStatus != nil {
			if err := setShootInStatus(status); err != nil {
				return fmt.Errorf("error setting shoot in status: %w", err)
			}
		}
		if apiAccess != nil {
			status.AdminAccess = apiAccess
		}
		return nil
	}

	// ensure AccessRequest
	ar := &clustersv1alpha1.AccessRequest{}
	ar.Name = as.Name
	ar.Namespace = nsName
	arm := NewAccessRequestMutator(ar.Name, ar.Namespace, cr.Name, cr.Namespace, false, []clustersv1alpha1.PermissionsRequest{
		{
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		},
	})
	arm.MetadataMutator().WithLabels(map[string]string{
		openmcpv1alpha1.V1MCPReferenceLabelName:      as.Name,
		openmcpv1alpha1.V1MCPReferenceLabelNamespace: as.Namespace,
	})
	if err := resources.CreateOrUpdateResource(ctx, platformClient, arm); err != nil {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("failed to create or update AccessRequest %s/%s: %w", ar.Namespace, ar.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}
	// if the AccessRequest is granted, fetch the corresponding access
	if err := platformClient.Get(ctx, client.ObjectKeyFromObject(ar), ar); err != nil {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("failed to get AccessRequest %s/%s: %w", ar.Namespace, ar.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}
	if ar.Status.Phase != clustersv1alpha1.REQUEST_GRANTED && ar.Status.SecretRef == nil {
		// todo return condition
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("AccessRequest %s/%s is not granted yet", ar.Namespace, ar.Name), clustersconst.ReasonInternalError)
	}

	// fetch the secret containing the kubeconfig
	secret := &corev1.Secret{}
	secret.Name = ar.Status.SecretRef.Name
	secret.Namespace = ar.Status.SecretRef.Namespace
	if err := platformClient.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("failed to get Secret %s/%s: %w", secret.Namespace, secret.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}
	kcfg, ok := secret.Data["kubeconfig"]
	if !ok {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("kubeconfig not found in secret %s/%s", secret.Namespace, secret.Name), clustersconst.ReasonInternalError)
	}
	rawCreationTime, ok := secret.Data["creationTimestamp"]
	if !ok {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("creationTimestamp not found in secret %s/%s", secret.Namespace, secret.Name), clustersconst.ReasonInternalError)
	}
	creationSeconds, err := strconv.ParseInt(string(rawCreationTime), 10, 64)
	if err != nil {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("error parsing creationTimestamp from secret %s/%s to int64: %w", secret.Namespace, secret.Name, err), clustersconst.ReasonInternalError)
	}
	expirationTime, ok := secret.Data["expirationTimestamp"]
	if !ok {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("expirationTimestamp not found in secret %s/%s", secret.Namespace, secret.Name), clustersconst.ReasonInternalError)
	}
	expirationSeconds, err := strconv.ParseInt(string(expirationTime), 10, 64)
	if err != nil {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("error parsing expirationTimestamp from secret %s/%s to int64: %w", secret.Namespace, secret.Name, err), clustersconst.ReasonInternalError)
	}
	apiAccess.Kubeconfig = string(kcfg)
	apiAccess.CreationTimestamp = &metav1.Time{Time: time.Unix(creationSeconds, 0)}
	apiAccess.ExpirationTimestamp = &metav1.Time{Time: time.Unix(expirationSeconds, 0)}
	rr := ctrl.Result{
		RequeueAfter: time.Until(clusteraccess.ComputeTokenRenewalTimeWithRatio(apiAccess.CreationTimestamp.Time, apiAccess.ExpirationTimestamp.Time, 0.85)),
	}

	return rr, usf, nil, nil
}

func v2HandleDelete(ctx context.Context, as *openmcpv1alpha1.APIServer, platformClient client.Client) (ctrl.Result, handler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx).WithName(openmcpv1alpha1.ArchitectureV2)
	ctx = logging.NewContext(ctx, log)

	// instead of calling a handler, remove AccessRequest and ClusterRequest
	nsName := fmt.Sprintf("mcp-%s", ctrlutils.K8sNameHash(as.Namespace))

	// remove AccessRequest
	ar := &clustersv1alpha1.AccessRequest{}
	ar.Name = as.Name
	ar.Namespace = nsName
	if err := platformClient.Delete(ctx, ar); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, nil, nil, openmcperrors.WithReason(fmt.Errorf("failed to delete AccessRequest %s/%s: %w", ar.Namespace, ar.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}

	var usf handler.UpdateStatusFunc = func(status *openmcpv1alpha1.APIServerStatus) error {
		status.AdminAccess = nil
		return nil
	}

	// remove ClusterRequest
	cr := &clustersv1alpha1.ClusterRequest{}
	cr.Name = as.Name
	cr.Namespace = nsName
	if err := platformClient.Delete(ctx, cr); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, usf, nil, openmcperrors.WithReason(fmt.Errorf("failed to delete ClusterRequest %s/%s: %w", cr.Namespace, cr.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
	}

	usf = func(status *openmcpv1alpha1.APIServerStatus) error {
		status.AdminAccess = nil
		status.GardenerStatus = nil
		return nil
	}

	return ctrl.Result{}, usf, nil, nil
}

type ClusterRequestMutator struct {
	name      string
	namespace string
	purpose   string
	meta      resources.MetadataMutator
}

var _ resources.Mutator[*clustersv1alpha1.ClusterRequest] = &ClusterRequestMutator{}

func NewClusterRequestMutator(name, namespace, purpose string) *ClusterRequestMutator {
	return &ClusterRequestMutator{
		name:      name,
		namespace: namespace,
		purpose:   purpose,
		meta:      resources.NewMetadataMutator(),
	}
}

// Empty implements resources.Mutator.
func (m *ClusterRequestMutator) Empty() *clustersv1alpha1.ClusterRequest {
	return &clustersv1alpha1.ClusterRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clustersv1alpha1.GroupVersion.String(),
			Kind:       "ClusterRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.name,
			Namespace: m.namespace,
		},
	}
}

// MetadataMutator implements resources.Mutator.
func (m *ClusterRequestMutator) MetadataMutator() resources.MetadataMutator {
	return m.meta
}

// Mutate implements resources.Mutator.
func (m *ClusterRequestMutator) Mutate(r *clustersv1alpha1.ClusterRequest) error {
	r.Spec.Purpose = m.purpose
	return m.meta.Mutate(r)
}

// String implements resources.Mutator.
func (m *ClusterRequestMutator) String() string {
	return fmt.Sprintf("ClusterRequest %s/%s", m.namespace, m.name)
}

type AccessRequestMutator struct {
	name         string
	namespace    string
	refName      string
	refNamespace string
	isClusterRef bool
	permissions  []clustersv1alpha1.PermissionsRequest
	meta         resources.MetadataMutator
}

var _ resources.Mutator[*clustersv1alpha1.AccessRequest] = &AccessRequestMutator{}

func NewAccessRequestMutator(name, namespace, refName, refNamespace string, isClusterRef bool, permissions []clustersv1alpha1.PermissionsRequest) *AccessRequestMutator {
	return &AccessRequestMutator{
		name:         name,
		namespace:    namespace,
		refName:      refName,
		refNamespace: refNamespace,
		isClusterRef: isClusterRef,
		permissions:  permissions,
		meta:         resources.NewMetadataMutator(),
	}
}

// Empty implements resources.Mutator.
func (m *AccessRequestMutator) Empty() *clustersv1alpha1.AccessRequest {
	return &clustersv1alpha1.AccessRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clustersv1alpha1.GroupVersion.String(),
			Kind:       "ClusterRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.name,
			Namespace: m.namespace,
		},
	}
}

// MetadataMutator implements resources.Mutator.
func (m *AccessRequestMutator) MetadataMutator() resources.MetadataMutator {
	return m.meta
}

// Mutate implements resources.Mutator.
func (m *AccessRequestMutator) Mutate(r *clustersv1alpha1.AccessRequest) error {
	if m.isClusterRef && r.Spec.ClusterRef == nil {
		r.Spec.ClusterRef = &clustersv1alpha1.NamespacedObjectReference{}
		r.Spec.ClusterRef.Name = m.refName
		r.Spec.ClusterRef.Namespace = m.refNamespace
	} else if !m.isClusterRef && r.Spec.RequestRef == nil {
		r.Spec.RequestRef = &clustersv1alpha1.NamespacedObjectReference{}
		r.Spec.RequestRef.Name = m.refName
		r.Spec.RequestRef.Namespace = m.refNamespace
	}
	r.Spec.Permissions = make([]clustersv1alpha1.PermissionsRequest, len(m.permissions))
	for i, perm := range m.permissions {
		r.Spec.Permissions[i] = *perm.DeepCopy()
	}
	return m.meta.Mutate(r)
}

// String implements resources.Mutator.
func (m *AccessRequestMutator) String() string {
	return fmt.Sprintf("AccessRequest %s/%s", m.namespace, m.name)
}
