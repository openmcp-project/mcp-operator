package v1alpha1

import (
	"context"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-openmcp-cloud-v1alpha1-managedcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.openmcp.cloud,resources=managedcontrolplanes,verbs=create;update,versions=v1alpha1,name=vmanagedcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &ManagedControlPlane{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (r *ManagedControlPlane) Default(ctx context.Context, obj runtime.Object) error {
	mcp, ok := obj.(*ManagedControlPlane)
	if !ok {
		return fmt.Errorf("object not supported")
	}

	managedcontrolplanelog.Info("default", "name", mcp.Name)

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	setCreatedBy(mcp, req)

	return nil
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
	updateValidators := []func(*ManagedControlPlane, *ManagedControlPlane) error{
		validateCreatedByUnchanged,
	}

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

// errCreatedByImmutable is the error that is returned when the value of the resource creator annotation has been changed by the user.
var errCreatedByImmutable = fmt.Errorf("annotation %s is immutable", CreatedByAnnotation)

// compareStringMapValue compares the value of string values identified by a key in two maps.
// Returns "true" if the value is the same.
func compareStringMapValue(a, b map[string]string, key string) bool {
	return a[key] == b[key]
}

// validateCreatedByUnchanged checks if the value of the annotation that contains the name of the resource creator has been changed.
// Returns an error if the value has been changed or "nil" if it's the same.
func validateCreatedByUnchanged(old, new *ManagedControlPlane) error {
	if compareStringMapValue(old.GetAnnotations(), new.GetAnnotations(), CreatedByAnnotation) {
		return nil
	}

	return errCreatedByImmutable
}

// setCreatedBy sets an annotation that contains the name of the user who created the resource.
// The value is only set when the "Operation" is "Create".
func setCreatedBy(obj metav1.Object, req admission.Request) {
	if req.Operation != admissionv1.Create {
		return
	}

	setMetaDataAnnotation(obj, CreatedByAnnotation, req.UserInfo.Username)
}

// setMetaDataAnnotation sets the annotation on the given object.
// If the given Object did not yet have annotations, they are initialized.
func setMetaDataAnnotation(meta metav1.Object, key, value string) {
	labels := meta.GetAnnotations()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	meta.SetAnnotations(labels)
}
