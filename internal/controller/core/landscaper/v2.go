package landscaper

import (
	"context"
	"fmt"
	"time"

	"github.com/openmcp-project/controller-utils/pkg/collections"
	"github.com/openmcp-project/controller-utils/pkg/logging"

	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	openmcpls "github.com/openmcp-project/service-provider-landscaper/api/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
	"github.com/openmcp-project/mcp-operator/internal/utils/components"
)

func (r *LandscaperConnector) v2HandleCreateOrUpdate(ctx context.Context, ls *openmcpv1alpha1.Landscaper) (ctrl.Result, bool, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx)
	log.Info("Creating or updating Landscaper v2 resource", "resourceName", ls.Name, "resourceNamespace", ls.Namespace)

	con := components.NewCondition(cconst.ConditionLandscaperV2ResourceCreatedOrUpdated, openmcpv1alpha1.ComponentConditionStatusUnknown, "", "")

	lsv2 := &openmcpls.Landscaper{}
	lsv2.SetName(ls.Name)
	lsv2.SetNamespace(ls.Namespace)
	if _, err := ctrl.CreateOrUpdate(ctx, r.CrateClient, lsv2, func() error {
		if lsv2.Labels == nil {
			lsv2.Labels = map[string]string{}
		}
		lsv2.Labels[openmcpv1alpha1.V1MCPReferenceLabelName] = ls.Name
		lsv2.Labels[openmcpv1alpha1.V1MCPReferenceLabelNamespace] = ls.Namespace

		return nil
	}); err != nil {
		rerr := openmcperrors.WithReason(fmt.Errorf("error creating or updating Landscaper v2 resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)
		con.Status = openmcpv1alpha1.ComponentConditionStatusFalse
		con.Reason = rerr.Reason()
		con.Message = rerr.Error()
		return ctrl.Result{}, false, []openmcpv1alpha1.ComponentCondition{con}, rerr
	}

	ready := lsv2.Status.Phase == commonapi.StatusPhaseReady && lsv2.Status.ObservedGeneration == lsv2.Generation
	cons := collections.ProjectSlice(lsv2.Status.Conditions, func(v2con metav1.Condition) openmcpv1alpha1.ComponentCondition {
		return components.NewCondition("LSv2_"+v2con.Type, components.ComponentConditionStatusFromMetav1ConditionStatus(v2con.Status), v2con.Reason, v2con.Message)
	})
	con.Status = openmcpv1alpha1.ComponentConditionStatusTrue
	cons = append(cons, con)

	return ctrl.Result{}, ready, cons, nil
}

func (r *LandscaperConnector) v2HandleDelete(ctx context.Context, ls *openmcpv1alpha1.Landscaper) (ctrl.Result, bool, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx)

	con := components.NewCondition(cconst.ConditionLandscaperV2ResourceDeleted, openmcpv1alpha1.ComponentConditionStatusUnknown, "", "")

	lsv2 := &openmcpls.Landscaper{}
	lsv2.SetName(ls.Name)
	lsv2.SetNamespace(ls.Namespace)
	if err := r.CrateClient.Get(ctx, client.ObjectKeyFromObject(lsv2), lsv2); err != nil {
		if !apierrors.IsNotFound(err) {
			rerr := openmcperrors.WithReason(fmt.Errorf("error getting Landscaper v2 resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)
			con.Status = openmcpv1alpha1.ComponentConditionStatusFalse
			con.Reason = rerr.Reason()
			con.Message = rerr.Error()
			return ctrl.Result{}, false, []openmcpv1alpha1.ComponentCondition{con}, rerr
		}
		lsv2 = nil
	}

	if lsv2 != nil {
		if lsv2.DeletionTimestamp.IsZero() {
			log.Info("Deleting Landscaper v2 resource", "resourceName", lsv2.Name, "resourceNamespace", lsv2.Namespace)
			if err := r.CrateClient.Delete(ctx, lsv2); err != nil {
				rerr := openmcperrors.WithReason(fmt.Errorf("error deleting Landscaper v2 resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)
				con.Status = openmcpv1alpha1.ComponentConditionStatusFalse
				con.Reason = rerr.Reason()
				con.Message = rerr.Error()
				return ctrl.Result{}, false, []openmcpv1alpha1.ComponentCondition{con}, rerr
			}
		} else {
			log.Info("Waiting for Landscaper v2 resource to be deleted", "resourceName", lsv2.Name, "resourceNamespace", lsv2.Namespace)
		}

		cons := collections.ProjectSlice(lsv2.Status.Conditions, func(v2con metav1.Condition) openmcpv1alpha1.ComponentCondition {
			return components.NewCondition("LSv2_"+v2con.Type, components.ComponentConditionStatusFromMetav1ConditionStatus(v2con.Status), v2con.Reason, v2con.Message)
		})
		con.Status = openmcpv1alpha1.ComponentConditionStatusFalse
		con.Reason = cconst.ReasonWaitingForLaaS
		con.Message = "Waiting for Landscaper v2 resource to be deleted"
		cons = append(cons, con)

		return ctrl.Result{RequeueAfter: 30 * time.Second}, false, cons, nil
	}

	log.Info("Landscaper v2 resource deleted", "resourceName", ls.Name, "resourceNamespace", ls.Namespace)
	con.Status = openmcpv1alpha1.ComponentConditionStatusTrue

	return ctrl.Result{}, true, []openmcpv1alpha1.ComponentCondition{con}, nil
}
