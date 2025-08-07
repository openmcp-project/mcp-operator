package apiserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openmcp-project/controller-utils/pkg/collections"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/controller-utils/pkg/clusteraccess"
	"github.com/openmcp-project/controller-utils/pkg/resources"

	gcpv1alpha1 "github.com/openmcp-project/cluster-provider-gardener/api/core/v1alpha1"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	clustersconst "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1/constants"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	openmcpclusterutils "github.com/openmcp-project/openmcp-operator/lib/utils"

	gardenv1beta1 "github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
	handler "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler"
	componentutils "github.com/openmcp-project/mcp-operator/internal/utils/components"
)

func v2HandleCreateOrUpdate(ctx context.Context, as *openmcpv1alpha1.APIServer, platformClient client.Client) (ctrl.Result, handler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx).WithName(openmcpv1alpha1.ArchitectureV2)
	ctx = logging.NewContext(ctx, log)

	clusterRequestGrantedCon := openmcpv1alpha1.ComponentCondition{
		Type:   cconst.ConditionClusterRequestGranted,
		Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
	}
	clusterReadyCon := openmcpv1alpha1.ComponentCondition{
		Type:   cconst.ConditionClusterReady,
		Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
	}
	accessRequestGrantedCon := openmcpv1alpha1.ComponentCondition{
		Type:   cconst.ConditionAccessRequestGranted,
		Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
	}

	// instead of calling a handler, create a ClusterRequest and an AccessRequest
	// ensure namespace, because this is created on the platform cluster
	nsName := openmcpclusterutils.StableRequestNamespace(as.Namespace)
	nsm := resources.NewNamespaceMutator(nsName)
	nsm.MetadataMutator().WithLabels(map[string]string{
		openmcpv1alpha1.V1MCPReferenceLabelNamespace: as.Namespace,
	})
	if err := resources.CreateOrUpdateResource(ctx, platformClient, nsm); err != nil {
		rerr := openmcperrors.WithReason(fmt.Errorf("failed to create or update namespace %s: %w", nsName, err), clustersconst.ReasonPlatformClusterInteractionProblem)
		return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
	}

	// create or update ClusterRequest
	var purpose string
	switch as.Spec.Type {
	case openmcpv1alpha1.Gardener:
		purpose = "mcp"
	case openmcpv1alpha1.GardenerDedicated:
		purpose = "mcp-worker"
	default:
		rerr := openmcperrors.WithReason(fmt.Errorf("unknown APIServer type '%s'", as.Spec.Type), clustersconst.ReasonConfigurationProblem)
		return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
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
		rerr := openmcperrors.WithReason(fmt.Errorf("failed to create or update ClusterRequest %s/%s: %w", cr.Namespace, cr.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
		return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
	}

	// if the ClusterRequest is granted, fetch the corresponding cluster
	if err := platformClient.Get(ctx, client.ObjectKeyFromObject(cr), cr); err != nil {
		rerr := openmcperrors.WithReason(fmt.Errorf("failed to get ClusterRequest %s/%s: %w", cr.Namespace, cr.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
		return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
	}
	var setShootInStatus handler.UpdateStatusFunc
	if cr.Status.Phase == clustersv1alpha1.REQUEST_GRANTED && cr.Status.Cluster != nil {
		clusterRequestGrantedCon.Status = openmcpv1alpha1.ComponentConditionStatusTrue

		// fetch Cluster resource
		cluster := &clustersv1alpha1.Cluster{}
		cluster.Name = cr.Status.Cluster.Name
		cluster.Namespace = cr.Status.Cluster.Namespace
		if err := platformClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
			rerr := openmcperrors.WithReason(fmt.Errorf("failed to get Cluster %s/%s: %w", cluster.Namespace, cluster.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
			return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}

		clusterReadyCon.Status = openmcpv1alpha1.ComponentConditionStatusFromBool(cluster.Status.Phase == clustersv1alpha1.CLUSTER_PHASE_READY)
		if clusterReadyCon.Status != openmcpv1alpha1.ComponentConditionStatusTrue {
			clusterReadyCon.Reason = cconst.ReasonClusterNotReady
			clusterReadyCon.Message = strings.Join(collections.ProjectSliceToSlice(cluster.Status.Conditions, func(con metav1.Condition) string {
				return fmt.Sprintf("[%s] %s", con.Reason, con.Message)
			}), "\n")
			if clusterReadyCon.Message == "" {
				clusterReadyCon.Message = "Cluster is not ready yet, no further information available"
			}
		}

		// check if there is a shoot manifest in the Cluster status
		// if so, copy it into the APIServer status
		if cluster.Status.ProviderStatus != nil {
			cs := &gcpv1alpha1.ClusterStatus{}
			if err := cluster.Status.GetProviderStatus(cs); err != nil {
				rerr := openmcperrors.WithReason(fmt.Errorf("error unmarshalling provider status: %w", err), clustersconst.ReasonInternalError)
				return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
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

		if clusterReadyCon.Status != openmcpv1alpha1.ComponentConditionStatusTrue {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil, clusterConditions(false, cconst.ReasonClusterNotReady, clusterReadyCon.Message, clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), nil
		}

	} else {
		clusterRequestGrantedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
		clusterRequestGrantedCon.Reason = cconst.ReasonClusterRequestNotGranted
		crReason := cconst.ReasonClusterRequestNotGranted
		crMessage := strings.Join(collections.ProjectSliceToSlice(cr.Status.Conditions, func(con metav1.Condition) string {
			return fmt.Sprintf("[%s] %s", con.Reason, con.Message)
		}), "\n")
		if crMessage == "" {
			crMessage = "<NoMessage>"
		}
		clusterRequestGrantedCon.Message = fmt.Sprintf("ClusterRequest is not granted or does not reference a cluster: [%s] %s", crReason, crMessage)

		rr := ctrl.Result{RequeueAfter: 30 * time.Second}
		if cr.Status.Phase == clustersv1alpha1.REQUEST_DENIED {
			// a denied request will never become granted (at least that's the idea), so no reason to wait for it
			rr = ctrl.Result{}
		}
		return rr, nil, clusterConditions(false, clusterRequestGrantedCon.Reason, clusterRequestGrantedCon.Message, clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), nil
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

	rr := ctrl.Result{}
	if clusterReadyCon.Status == openmcpv1alpha1.ComponentConditionStatusTrue {
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
			rerr := openmcperrors.WithReason(fmt.Errorf("failed to create or update AccessRequest %s/%s: %w", ar.Namespace, ar.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		// if the AccessRequest is granted, fetch the corresponding access
		if err := platformClient.Get(ctx, client.ObjectKeyFromObject(ar), ar); err != nil {
			rerr := openmcperrors.WithReason(fmt.Errorf("failed to get AccessRequest %s/%s: %w", ar.Namespace, ar.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		if ar.Status.Phase != clustersv1alpha1.REQUEST_GRANTED && ar.Status.SecretRef == nil {
			accessRequestGrantedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
			accessRequestGrantedCon.Reason = cconst.ReasonAccessRequestNotGranted
			arReason := cconst.ReasonAccessRequestNotGranted
			arMessage := strings.Join(collections.ProjectSliceToSlice(ar.Status.Conditions, func(con metav1.Condition) string {
				return fmt.Sprintf("[%s] %s", con.Reason, con.Message)
			}), "\n")
			if arMessage == "" {
				arMessage = "<NoMessage>"
			}
			accessRequestGrantedCon.Message = fmt.Sprintf("AccessRequest '%s/%s' is not granted or does not reference a secret: [%s] %s", ar.Namespace, ar.Name, arReason, arMessage)

			rr := ctrl.Result{RequeueAfter: 30 * time.Second}
			if ar.Status.Phase == clustersv1alpha1.REQUEST_DENIED {
				// a denied request will never become granted (at least that's the idea), so no reason to wait for it
				rr = ctrl.Result{}
			}
			return rr, usf, clusterConditions(false, accessRequestGrantedCon.Reason, accessRequestGrantedCon.Message, clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), nil
		}

		accessRequestGrantedCon.Status = openmcpv1alpha1.ComponentConditionStatusTrue

		// fetch the secret containing the kubeconfig
		secret := &corev1.Secret{}
		secret.Name = ar.Status.SecretRef.Name
		secret.Namespace = ar.Status.SecretRef.Namespace
		if err := platformClient.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			rerr := openmcperrors.WithReason(fmt.Errorf("failed to get Secret %s/%s: %w", secret.Namespace, secret.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		kcfg, ok := secret.Data["kubeconfig"]
		if !ok {
			rerr := openmcperrors.WithReason(fmt.Errorf("kubeconfig not found in secret %s/%s", secret.Namespace, secret.Name), clustersconst.ReasonInternalError)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		rawCreationTime, ok := secret.Data["creationTimestamp"]
		if !ok {
			rerr := openmcperrors.WithReason(fmt.Errorf("creationTimestamp not found in secret %s/%s", secret.Namespace, secret.Name), clustersconst.ReasonInternalError)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		creationSeconds, err := strconv.ParseInt(string(rawCreationTime), 10, 64)
		if err != nil {
			rerr := openmcperrors.WithReason(fmt.Errorf("error parsing creationTimestamp from secret %s/%s to int64: %w", secret.Namespace, secret.Name, err), clustersconst.ReasonInternalError)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		expirationTime, ok := secret.Data["expirationTimestamp"]
		if !ok {
			rerr := openmcperrors.WithReason(fmt.Errorf("expirationTimestamp not found in secret %s/%s", secret.Namespace, secret.Name), clustersconst.ReasonInternalError)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		expirationSeconds, err := strconv.ParseInt(string(expirationTime), 10, 64)
		if err != nil {
			rerr := openmcperrors.WithReason(fmt.Errorf("error parsing expirationTimestamp from secret %s/%s to int64: %w", secret.Namespace, secret.Name, err), clustersconst.ReasonInternalError)
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), rerr
		}
		apiAccess.Kubeconfig = string(kcfg)
		apiAccess.CreationTimestamp = &metav1.Time{Time: time.Unix(creationSeconds, 0)}
		apiAccess.ExpirationTimestamp = &metav1.Time{Time: time.Unix(expirationSeconds, 0)}
		rr = ctrl.Result{
			RequeueAfter: time.Until(clusteraccess.ComputeTokenRenewalTimeWithRatio(apiAccess.CreationTimestamp.Time, apiAccess.ExpirationTimestamp.Time, 0.85)),
		}
	}

	return rr, usf, clusterConditions(true, "", "", clusterRequestGrantedCon, clusterReadyCon, accessRequestGrantedCon), nil
}

func v2HandleDelete(ctx context.Context, as *openmcpv1alpha1.APIServer, platformClient client.Client) (ctrl.Result, handler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx).WithName(openmcpv1alpha1.ArchitectureV2)
	ctx = logging.NewContext(ctx, log)

	accessRequestDeletedCon := openmcpv1alpha1.ComponentCondition{
		Type:   cconst.ConditionAccessRequestDeleted,
		Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
	}
	clusterRequestDeletedCon := openmcpv1alpha1.ComponentCondition{
		Type:   cconst.ConditionClusterRequestDeleted,
		Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
	}

	// instead of calling a handler, remove AccessRequest and ClusterRequest
	nsName := openmcpclusterutils.StableRequestNamespace(as.Namespace)

	// remove AccessRequest
	ar := &clustersv1alpha1.AccessRequest{}
	ar.Name = as.Name
	ar.Namespace = nsName
	if err := platformClient.Delete(ctx, ar); client.IgnoreNotFound(err) != nil {
		rerr := openmcperrors.WithReason(fmt.Errorf("failed to delete AccessRequest %s/%s: %w", ar.Namespace, ar.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
		accessRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
		accessRequestDeletedCon.Reason = rerr.Reason()
		accessRequestDeletedCon.Message = err.Error()
		return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), accessRequestDeletedCon, clusterRequestDeletedCon), rerr
	}

	if err := platformClient.Get(ctx, client.ObjectKeyFromObject(ar), ar); err != nil {
		if !apierrors.IsNotFound(err) {
			rerr := openmcperrors.WithReason(fmt.Errorf("failed to verify deletion of AccessRequest '%s/%s': %w", ar.Namespace, ar.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
			accessRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
			accessRequestDeletedCon.Reason = rerr.Reason()
			accessRequestDeletedCon.Message = rerr.Error()
			return ctrl.Result{}, nil, clusterConditions(false, rerr.Reason(), rerr.Error(), accessRequestDeletedCon, clusterRequestDeletedCon), rerr
		}
		accessRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusTrue
	} else {
		accessRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
		accessRequestDeletedCon.Reason = cconst.ReasonAccessRequestNotDeleted
		accessRequestDeletedCon.Message = fmt.Sprintf("AccessRequest '%s/%s' has not been deleted yet", ar.Namespace, ar.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil, clusterConditions(false, accessRequestDeletedCon.Reason, accessRequestDeletedCon.Message, accessRequestDeletedCon, clusterRequestDeletedCon), nil
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
		rerr := openmcperrors.WithReason(fmt.Errorf("failed to delete ClusterRequest %s/%s: %w", cr.Namespace, cr.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
		clusterRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
		clusterRequestDeletedCon.Reason = rerr.Reason()
		clusterRequestDeletedCon.Message = err.Error()
		return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), accessRequestDeletedCon, clusterRequestDeletedCon), rerr
	}

	if err := platformClient.Get(ctx, client.ObjectKeyFromObject(cr), cr); err != nil {
		if !apierrors.IsNotFound(err) {
			rerr := openmcperrors.WithReason(fmt.Errorf("failed to verify deletion of ClusterRequest '%s/%s': %w", cr.Namespace, cr.Name, err), clustersconst.ReasonPlatformClusterInteractionProblem)
			clusterRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
			clusterRequestDeletedCon.Reason = rerr.Reason()
			clusterRequestDeletedCon.Message = rerr.Error()
			return ctrl.Result{}, usf, clusterConditions(false, rerr.Reason(), rerr.Error(), accessRequestDeletedCon, clusterRequestDeletedCon), rerr
		}
		clusterRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusTrue
	} else {
		clusterRequestDeletedCon.Status = openmcpv1alpha1.ComponentConditionStatusFalse
		clusterRequestDeletedCon.Reason = cconst.ReasonClusterRequestNotDeleted
		clusterRequestDeletedCon.Message = fmt.Sprintf("ClusterRequest '%s/%s' has not been deleted yet", cr.Namespace, cr.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, usf, clusterConditions(false, clusterRequestDeletedCon.Reason, clusterRequestDeletedCon.Message, accessRequestDeletedCon, clusterRequestDeletedCon), nil
	}

	usf = func(status *openmcpv1alpha1.APIServerStatus) error {
		status.AdminAccess = nil
		status.GardenerStatus = nil
		return nil
	}

	return ctrl.Result{}, usf, clusterConditions(true, "", "", accessRequestDeletedCon, clusterRequestDeletedCon), nil
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
		r.Spec.ClusterRef = &commonapi.ObjectReference{}
		r.Spec.ClusterRef.Name = m.refName
		r.Spec.ClusterRef.Namespace = m.refNamespace
	} else if !m.isClusterRef && r.Spec.RequestRef == nil {
		r.Spec.RequestRef = &commonapi.ObjectReference{}
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

func clusterConditions(ready bool, reason, message string, additionalConditions ...openmcpv1alpha1.ComponentCondition) []openmcpv1alpha1.ComponentCondition {
	conditions := []openmcpv1alpha1.ComponentCondition{
		componentutils.NewCondition(openmcpv1alpha1.APIServerComponent.HealthyCondition(), openmcpv1alpha1.ComponentConditionStatusFromBool(ready), reason, message),
	}
	conditions = append(conditions, additionalConditions...)
	return conditions
}
