package config_test

import (
	"path"

	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/authentication/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth Config Utils", func() {
	It("should load the config from file", func() {
		authConfig, err := config.LoadConfig(path.Join("testdata", "config_valid.yaml"))
		Expect(err).ToNot(HaveOccurred())
		Expect(authConfig).ToNot(BeNil())

		Expect(authConfig.SystemIdentityProvider.Name).To(Equal("system"))
		Expect(authConfig.SystemIdentityProvider.IssuerURL).To(Equal("https://system.local"))
		Expect(authConfig.SystemIdentityProvider.ClientID).To(Equal("xxx-yyy-zzz"))
		Expect(authConfig.SystemIdentityProvider.UsernameClaim).To(Equal("email"))
		Expect(authConfig.SystemIdentityProvider.GroupsClaim).To(Equal("groups"))
	})

	It("should fail to load the config from file", func() {
		authConfig, err := config.LoadConfig(path.Join("testdata", "config_invalid.yaml"))
		Expect(err).To(HaveOccurred())
		Expect(authConfig).To(BeNil())

		authConfig, err = config.LoadConfig(path.Join("testdata", "config_missing.yaml"))
		Expect(err).To(HaveOccurred())
		Expect(authConfig).To(BeNil())
	})
})
