package v1alpha1

import (
	"unicode"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

// Default sets the default values for the AuthenticationSpec.
// This modifies the receiver object.
func (as *AuthenticationSpec) Default() {
	if as.EnableSystemIdentityProvider == nil {
		as.EnableSystemIdentityProvider = ptr.To(true)
	}
}

// Validate validates the AuthenticationSpec
func (as *AuthenticationSpec) Validate(path string, morePaths ...string) error {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath(path, morePaths...)

	uniqueness := make(map[string]interface{})

	for _, idp := range as.IdentityProviders {
		if _, ok := uniqueness[idp.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("identityProviders").Child(idp.Name), idp.Name))
		} else {
			uniqueness[idp.Name] = nil
		}

		allErrs = append(allErrs, ValidateIdp(idp, fldPath.Child("identityProviders"))...)
	}

	return allErrs.ToAggregate()
}

// ValidateIdp validates the IdentityProvider
func ValidateIdp(idp IdentityProvider, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	idpPath := fldPath.Child(idp.Name)

	if idp.IssuerURL == "" {
		allErrs = append(allErrs, field.Required(idpPath.Child("issuerURL"), "issuerURL must be set"))
	}

	if idp.ClientID == "" {
		allErrs = append(allErrs, field.Required(idpPath.Child("clientID"), "clientID must be set"))
	}

	if !isLowerCaseLetter(idp.Name) {
		allErrs = append(allErrs, field.Invalid(idpPath.Child("name"), idp.Name, "name must only contain lowercase letters"))
	}

	if len(idp.Name) > 63 {
		allErrs = append(allErrs, field.TooLong(idpPath.Child("name"), idp.Name, 63))
	}

	if idp.ClientConfig.ExtraConfig != nil {
		fldPath = idpPath.Child("client").Child("extraConfig")

		if _, ok := idp.ClientConfig.ExtraConfig["oidc-issuer-url"]; ok {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key("oidc-issuer-url"), "oidc-issuer-url is a reserved key"))
		}

		if _, ok := idp.ClientConfig.ExtraConfig["oidc-client-id"]; ok {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key("oidc-client-id"), "oidc-client-id is a reserved key"))
		}

		if _, ok := idp.ClientConfig.ExtraConfig["oidc-client-secret"]; ok {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key("oidc-client-secret"), "oidc-client-secret is a reserved key"))
		}
	}

	return allErrs
}

// isLowerCaseLetter checks if the given string is a lowercase letter.
func isLowerCaseLetter(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsLower(r) {
			return false
		}
	}
	return true
}
