package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var managedcontrolplanelog = logf.Log.WithName("managedcontrolplane-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *ManagedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-core-openmcp-cloud-v1alpha1-managedcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.openmcp.cloud,resources=managedcontrolplanes,verbs=delete,versions=v1alpha1,name=vmanagedcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &ManagedControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedControlPlane) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	managedcontrolplanelog.Info("validate create - this should never be triggered", "name", r.Name)

	// no-op
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedControlPlane) ValidateUpdate(_ context.Context, old runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	oldMcp, ok := old.(*ManagedControlPlane)
	if !ok {
		return nil, fmt.Errorf("object not supported")
	}

	newMcp, ok := newObj.(*ManagedControlPlane)
	if !ok {
		return nil, fmt.Errorf("object not supported")
	}
	var errorList []error

	// Add update validators here when needed
	var updateValidators []func(*ManagedControlPlane, *ManagedControlPlane) error

	for _, validator := range updateValidators {
		if err := validator(newMcp, oldMcp); err != nil {
			managedcontrolplanelog.Error(fmt.Errorf("update validation failed"), err.Error())
			errorList = append(errorList, err)
		}
	}

	return nil, apierrors.NewAggregate(errorList)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ManagedControlPlane) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	managedcontrolplanelog.Info("validate delete", "name", r.Name)

	mcp, ok := obj.(*ManagedControlPlane)
	if !ok {
		return nil, fmt.Errorf("object not supported")
	}

	if mcp.Annotations[ManagedControlPlaneDeletionConfirmationAnnotation] == "true" {
		return nil, nil
	}
	return nil, fmt.Errorf("ManagedControlPlane %q requires annotation %q to be set to true, before it can be deleted", r.Name, ManagedControlPlaneDeletionConfirmationAnnotation)
}
