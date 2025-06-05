package controller

import (
	"context"
	"fmt"
	"time"

	apiserverutils "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/utils"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

// UpdateStatusFunc is expected to update all component-specific fields in the status.
type UpdateStatusFunc func(*openmcpv1alpha1.APIServerStatus) error

// APIServerHandler is an interface for the handlers for the different APIServer types.
type APIServerHandler interface {
	// HandleCreateOrUpdate handles creation/update of the APIServer.
	// It returns a reconcile result, an update function to update the status with, conditions that determine health/readiness of the cluster, and potentially an error that occurred.
	// The status' condition will be overwritten based on the other returned values.
	HandleCreateOrUpdate(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (ctrl.Result, UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError)

	// HandleDelete handles the deletion of the APIServer.
	// It returns a reconcile result, an update function to update the status with, conditions that determine health/readiness of the cluster (deletion), and potentially an error that occurred.
	HandleDelete(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (ctrl.Result, UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError)
}

// ClusterAccessEnabler is a helper interface.
// It is used to initially access the cluster in order to create serviceaccounts and generate kubeconfigs for them.
type ClusterAccessEnabler interface {
	// Init is called before Client and RESTConfig. It is called only if access to the cluster is actually required.
	// This can be used to put expensive operations into which should not be executed always - which would happen if they were in the constructor - but only when actually needed.
	Init(ctx context.Context) error
	// Client returns a client for accessing the cluster.
	Client() client.Client
	// RESTConfig returns the rest config for the cluster. The information from here is used for kubeconfig construction.
	RESTConfig() *rest.Config
}

// GetClusterAccess is a helper function to get admin and user kubeconfigs for an APIServer.
// It takes a possible existing admin and user access (or nil), as well as a ClusterAccessEnabler which provides initial access to the cluster.
// It returns an admin access, a user access, and the computed duration after which the APIServer should be reconciled to renew the kubeconfigs, if required.
func GetClusterAccess(ctx context.Context, serviceAccountNamespace, adminServiceAccount string, adminAccess *openmcpv1alpha1.APIServerAccess, cae ClusterAccessEnabler) (*openmcpv1alpha1.APIServerAccess, time.Duration, error) {
	// generate kubeconfig/check token validity
	// check if admin kubeconfig already exists
	adminAccessExists := false
	var adminRenewalAt time.Time
	if adminAccess != nil {
		adminAccessExists = true
		adminRenewalAt = computeTokenRenewalTime(adminAccess)
	}
	renewAdminAccess := !adminAccessExists || (!adminRenewalAt.IsZero() && adminRenewalAt.Before(time.Now()))

	var requeueAfter time.Duration
	if renewAdminAccess {
		err := cae.Init(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("error initializing ClusterAccessEnabler: %w", err)
		}
		c := cae.Client()
		rc := cae.RESTConfig()

		ns, err := apiserverutils.EnsureNamespace(ctx, c, serviceAccountNamespace)
		if err != nil {
			return nil, 0, err
		}

		if renewAdminAccess {
			adminAccess, err = apiserverutils.GetAdminAccess(ctx, c, rc, adminServiceAccount, ns.Name)
			if err != nil {
				return nil, 0, fmt.Errorf("error creating/renewing admin access for APIServer shoot cluster: %w", err)
			}
			requeueAfter = time.Until(computeTokenRenewalTime(adminAccess))
		}
	}

	return adminAccess, requeueAfter, nil
}

// computeTokenRenewalTime computes the time at which the given access should be renewed.
// Can only be computed if CreationTimestamp and ExpirationTimestamp are non-nil, otherwise the zero time is returned.
// The returned time is when 80% of the validity duration are reached.
func computeTokenRenewalTime(acc *openmcpv1alpha1.APIServerAccess) time.Time {
	if acc == nil || acc.CreationTimestamp == nil || acc.ExpirationTimestamp == nil {
		return time.Time{}
	}
	// validity is how long the token was valid in the first place
	validity := acc.ExpirationTimestamp.Sub(acc.CreationTimestamp.Time)
	// renewalAfter is 80% of the validity
	renewalAfter := time.Duration(float64(validity) * 0.8)
	// renewalAt is the point in time at which the token should be renewed
	renewalAt := acc.CreationTimestamp.Add(renewalAfter)
	return renewalAt
}
