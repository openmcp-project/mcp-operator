package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	GroupName          = "rbac.authorization.k8s.io"
	GroupKind          = "Group"
	ServiceAccountKind = "ServiceAccount"
	UserKind           = "User"
)

// Default sets the default values for the AuthorizationSpec
func (as *AuthorizationSpec) Default() {
	for _, role := range as.RoleBindings {
		for i, subject := range role.Subjects {
			if (subject.Kind == GroupKind || subject.Kind == UserKind) && subject.APIGroup == "" {
				role.Subjects[i].APIGroup = GroupName
			}
		}
	}
}

// Validate validates the AuthorizationSpec
func (as *AuthorizationSpec) Validate(path string, morePaths ...string) error {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath(path, morePaths...)

	for _, role := range as.RoleBindings {
		if role.Role != RoleBindingRoleAdmin && role.Role != RoleBindingRoleView {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("role"), role.Role, "role must be either admin or view"))
		}

		fldPath = fldPath.Child("subjects")

		for i, subject := range role.Subjects {
			fldPath = fldPath.Index(i)

			if subject.Kind != GroupKind && subject.Kind != UserKind && subject.Kind != ServiceAccountKind {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), subject.Kind, "kind must be either ServiceAccount, User or Group"))
			}

			if (subject.Kind == GroupKind || subject.Kind == UserKind) && subject.APIGroup != GroupName {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("apiGroup"), subject.APIGroup, "apiGroup must be set to "+GroupName))
			}

			if subject.Name == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name must be set"))
			}

			if subject.Namespace == "" && subject.Kind == ServiceAccountKind {
				allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "namespace must be set"))
			}
		}
	}

	return allErrs.ToAggregate()
}
