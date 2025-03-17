// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"context"
	"fmt"
	"reflect"

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

	updateValidators := []func(*ManagedControlPlane, *ManagedControlPlane) error{
		validateUpdateDesiredRegion,
		validateUpdateAPIServerUpdate,
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

// validateUpdateDesiredRegion ensures that DesiredRegion is immutable
func validateUpdateDesiredRegion(newMCP, oldMcp *ManagedControlPlane) error {
	if oldMcp.Spec.CommonConfig == nil || oldMcp.Spec.CommonConfig.DesiredRegion == nil {
		return nil
	}
	if !reflect.DeepEqual(newMCP.Spec.DesiredRegion, oldMcp.Spec.DesiredRegion) {
		return fmt.Errorf("spec.desiredRegion is immutable")
	}
	return nil
}

// validateUpdateAPIServerUpdate ensure that APIServer is immutable while allowing delete
func validateUpdateAPIServerUpdate(newMCP, oldMcp *ManagedControlPlane) error {
	if oldMcp.Spec.Components.APIServer == nil { // APIServer is already unset, nothing to do here
		return nil
	}

	if newMCP.Spec.Components.APIServer == nil { // APIServer is being deleted.
		return newMCP.validateUpdateAPIServerRemove(oldMcp)
	}
	if !reflect.DeepEqual(newMCP.Spec.Components.APIServer, oldMcp.Spec.Components.APIServer) {
		return fmt.Errorf("spec.components.apiServer is immutable")
	}
	return nil
}

// validateUpdateAPIServerRemove ensures that APIserver is not removed before all other components are removed
func (r *ManagedControlPlane) validateUpdateAPIServerRemove(oldMcp *ManagedControlPlane) error {
	if oldMcp.Spec.Components.APIServer != nil && r.Spec.Components.APIServer == nil {
		if r.Spec.Components.Landscaper != nil ||
			r.Spec.Components.CloudOrchestratorConfiguration.Flux != nil ||
			r.Spec.Components.CloudOrchestratorConfiguration.Kyverno != nil ||
			r.Spec.Components.CloudOrchestratorConfiguration.Crossplane != nil ||
			r.Spec.Components.CloudOrchestratorConfiguration.BTPServiceOperator != nil ||
			r.Spec.Components.CloudOrchestratorConfiguration.ExternalSecretsOperator != nil {
			return fmt.Errorf("spec.components.apiServer can't be removed while other components are configured")
		}
	}
	return nil
}
