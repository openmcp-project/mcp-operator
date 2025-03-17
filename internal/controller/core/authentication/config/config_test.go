package config_test

import (
	"errors"

	"github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"

	authconfig "github.tools.sap/CoLa/mcp-operator/internal/controller/core/authentication/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
)

var _ = Describe("Auth Config", func() {
	It("should set defaults", func() {
		config := &authconfig.AuthenticationConfig{}
		config.CrateIdentityProvider = &v1alpha1.IdentityProvider{}

		config.SetDefaults()

		Expect(config.SystemIdentityProvider.Name).To(Equal(authconfig.DefaultSystemIdPName))
		Expect(config.SystemIdentityProvider.UsernameClaim).To(Equal(authconfig.DefaultSystemUsernameClaim))
		Expect(config.SystemIdentityProvider.GroupsClaim).To(Equal(authconfig.DefaultSystemGroupsClaim))

		Expect(config.CrateIdentityProvider.Name).To(Equal(authconfig.DefaultCratedIdPName))
		Expect(config.CrateIdentityProvider.ClientID).To(Equal(authconfig.DefaultCrateClientID))
		Expect(config.CrateIdentityProvider.UsernameClaim).To(Equal(authconfig.DefaultCrateUsernameClaim))
	})

	It("should validate", func() {
		config := &authconfig.AuthenticationConfig{}
		config.SetDefaults()

		config.SystemIdentityProvider.IssuerURL = "https://openmcp.local"
		config.SystemIdentityProvider.ClientID = "aaa-bbb-ccc"

		err := authconfig.Validate(config)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should not validate", func() {
		config := &authconfig.AuthenticationConfig{}

		err := authconfig.Validate(config)
		Expect(err).To(HaveOccurred())

		var aggErr k8serrors.Aggregate
		Expect(errors.As(err, &aggErr)).To(BeTrue())

		Expect(aggErr.Errors()).To(HaveLen(2))
		Expect(aggErr.Errors()[0].Error()).To(ContainSubstring("issuerURL"))
		Expect(aggErr.Errors()[1].Error()).To(ContainSubstring("clientID"))
	})

	It("should validate crate identity provider", func() {
		config := &authconfig.AuthenticationConfig{}
		config.SetDefaults()

		config.SystemIdentityProvider.IssuerURL = "https://openmcp.local"
		config.SystemIdentityProvider.ClientID = "aaa-bbb-ccc"

		config.CrateIdentityProvider = &v1alpha1.IdentityProvider{}
		config.CrateIdentityProvider.IssuerURL = "https://crate.local"
		config.CrateIdentityProvider.ClientID = "aaa-bbb-ccc"

		err := authconfig.Validate(config)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should not validate an invalid crate identity provider", func() {
		config := &authconfig.AuthenticationConfig{}
		config.SetDefaults()

		config.SystemIdentityProvider.IssuerURL = "https://openmcp.local"
		config.SystemIdentityProvider.ClientID = "aaa-bbb-ccc"

		config.CrateIdentityProvider = &v1alpha1.IdentityProvider{}

		err := authconfig.Validate(config)
		Expect(err).To(HaveOccurred())

		var aggErr k8serrors.Aggregate
		Expect(errors.As(err, &aggErr)).To(BeTrue())

		Expect(aggErr.Errors()).To(HaveLen(2))
		Expect(aggErr.Errors()[0].Error()).To(ContainSubstring("issuerURL"))
		Expect(aggErr.Errors()[1].Error()).To(ContainSubstring("clientID"))
	})
})
