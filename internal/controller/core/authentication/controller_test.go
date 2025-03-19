package authentication_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openmcp-project/mcp-operator/internal/components"

	"github.com/openmcp-project/mcp-operator/internal/controller/core/authentication"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/authentication/config"

	. "github.com/openmcp-project/mcp-operator/test/matchers"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

var (
	systemIdentityProvider = openmcpv1alpha1.IdentityProvider{
		Name:          "openmcp",
		IssuerURL:     "https://issuer.local",
		ClientID:      "aaa-bbb-ccc",
		UsernameClaim: "email",
		GroupsClaim:   "groups",
	}

	crateIdentityProvider = openmcpv1alpha1.IdentityProvider{
		Name:          "crate",
		IssuerURL:     "https://crate.local",
		ClientID:      "mcp",
		UsernameClaim: "sub",
	}
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	return authentication.NewAuthenticationReconciler(c[0], &config.AuthenticationConfig{
		SystemIdentityProvider: systemIdentityProvider,
	})
}

func getReconcilerWithCrateIdentityProvider(c ...client.Client) reconcile.Reconciler {
	return authentication.NewAuthenticationReconciler(c[0], &config.AuthenticationConfig{
		SystemIdentityProvider: systemIdentityProvider,
		CrateIdentityProvider:  &crateIdentityProvider,
	})
}

func getOpenIDConnect() *unstructured.Unstructured {
	openIdConnect := &unstructured.Unstructured{}
	openIdConnect.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "authentication.gardener.cloud",
		Version: "v1alpha1",
		Kind:    "OpenIDConnect",
	})
	return openIdConnect
}

func getOpenIDConnectList() *unstructured.UnstructuredList {
	openIdConnectList := &unstructured.UnstructuredList{}
	openIdConnectList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "authentication.gardener.cloud",
		Version: "v1alpha1",
		Kind:    "OpenIDConnectList",
	})
	return openIdConnectList
}

const (
	authReconciler = "auth"
)

func testEnvWithAPIServerAccess(testDataPathSegments ...string) *testing.ComplexEnvironment {
	env := testutils.DefaultTestSetupBuilder(testDataPathSegments...).WithFakeClient(testutils.APIServerCluster, testutils.Scheme).WithReconcilerConstructor(authReconciler, getReconciler, testutils.CrateCluster).Build()
	env.Reconcilers[authReconciler].(*authentication.AuthenticationReconciler).SetAPIServerAccess(&testutils.TestAPIServerAccess{Client: env.Client(testutils.APIServerCluster)})
	return env
}

func testEnvWithAPIServerAccessWithCrateIdentityProvider(testDataPathSegments ...string) *testing.ComplexEnvironment {
	env := testutils.DefaultTestSetupBuilder(testDataPathSegments...).WithFakeClient(testutils.APIServerCluster, testutils.Scheme).WithReconcilerConstructor(authReconciler, getReconcilerWithCrateIdentityProvider, testutils.CrateCluster).Build()
	env.Reconcilers[authReconciler].(*authentication.AuthenticationReconciler).SetAPIServerAccess(&testutils.TestAPIServerAccess{Client: env.Client(testutils.APIServerCluster)})
	return env
}

var _ = Describe("CO-1153 Authentication Controller", func() {
	It("should set the ready condition to false when there is no APIServer available", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-01")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		res := env.ShouldReconcile(authReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())

		Expect(auth.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForDependencies,
			}),
		))
	})

	It("should set the status condition to false when APIServer is not ready", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-02")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		res := env.ShouldReconcile(authReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())

		Expect(auth.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForDependencies,
			}),
		))
	})

	It("should fail to reconcile and set the status condition to false when APIServer has an unsupported type", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-03")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldNotReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())

		Expect(auth.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonReconciliationError,
			}),
		))
	})

	It("should fail to reconcile and set the status condition to false when APIServer status has no access kubeconfig", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-04")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldNotReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())

		Expect(auth.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonReconciliationError,
			}),
		))
	})

	It("should create an OpenIDConnect resource on the APIServer and an access secret", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-05")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Finalizers).To(ContainElements(openmcpv1alpha1.AuthenticationComponent.Finalizer()))

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		authComp := components.Component(auth)
		Expect(as.Finalizers).To(ContainElements(authComp.Type().DependencyFinalizer()))

		Expect(auth.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))

		Expect(auth.Status.ExternalAuthenticationStatus).NotTo(BeNil())
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess).ToNot(BeNil())
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess.Key).To(Equal("kubeconfig"))
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess.Name).To(Equal("test.kubeconfig"))
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess.Namespace).To(Equal("test"))

		accessSecret := &corev1.Secret{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test.kubeconfig", Namespace: "test"}, accessSecret)
		Expect(err).NotTo(HaveOccurred())

		Expect(accessSecret.StringData).To(HaveKey("kubeconfig"))
		config, err := clientcmd.NewClientConfigFromBytes([]byte(accessSecret.StringData["kubeconfig"]))
		Expect(err).NotTo(HaveOccurred())

		rawConfig, err := config.RawConfig()
		Expect(err).NotTo(HaveOccurred())

		Expect(rawConfig.AuthInfos).To(HaveKey(systemIdentityProvider.Name))

		systemIdP := rawConfig.AuthInfos[systemIdentityProvider.Name]
		Expect(systemIdP.Exec).ToNot(BeNil())
		Expect(systemIdP.Exec.Command).To(Equal("kubectl"))
		Expect(systemIdP.Exec.Args).To(ContainElements("oidc-login"))
		Expect(systemIdP.Exec.Args).To(ContainElements("get-token"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-issuer-url=" + systemIdentityProvider.IssuerURL))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-client-id=" + systemIdentityProvider.ClientID))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-extra-scope=email"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-extra-scope=profile"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-extra-scope=offline_access"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-use-pkce"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--grant-type=auto"))

		openIdConnect := getOpenIDConnect()

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: systemIdentityProvider.Name}, openIdConnect)
		Expect(err).NotTo(HaveOccurred())

		Expect(openIdConnect.Object).To(HaveKey("spec"))
		spec := openIdConnect.Object["spec"]
		Expect(spec).To(HaveKeyWithValue("issuerURL", systemIdentityProvider.IssuerURL))
		Expect(spec).To(HaveKeyWithValue("clientID", systemIdentityProvider.ClientID))
		Expect(spec).To(HaveKeyWithValue("usernameClaim", systemIdentityProvider.UsernameClaim))
		Expect(spec).To(HaveKeyWithValue("groupsClaim", systemIdentityProvider.GroupsClaim))
		Expect(spec).To(HaveKeyWithValue("usernamePrefix", systemIdentityProvider.Name+":"))
		Expect(spec).To(HaveKeyWithValue("groupsPrefix", systemIdentityProvider.Name+":"))

		Expect(rawConfig.AuthInfos).To(HaveKey("customer"))
		customerIdP := rawConfig.AuthInfos["customer"]
		Expect(customerIdP.Exec.Args).To(ContainElements("--oidc-issuer-url=https://customer.local"))
		Expect(customerIdP.Exec.Args).To(ContainElements("--oidc-client-id=xxx-yyy-zzz"))

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: "customer"}, openIdConnect)
		Expect(err).NotTo(HaveOccurred())

		Expect(openIdConnect.Object).To(HaveKey("spec"))
		spec = openIdConnect.Object["spec"]
		Expect(spec).To(HaveKeyWithValue("issuerURL", "https://customer.local"))
		Expect(spec).To(HaveKeyWithValue("clientID", "xxx-yyy-zzz"))
		Expect(spec).To(HaveKeyWithValue("usernameClaim", "u_name"))
		Expect(spec).To(HaveKeyWithValue("groupsClaim", "grp"))
		Expect(spec).To(HaveKeyWithValue("usernamePrefix", "customer:"))
		Expect(spec).To(HaveKeyWithValue("groupsPrefix", "customer:"))
	})

	It("should update/delete OpenIDConnect resource for spec updates", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-06")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())

		openIDConnectList := getOpenIDConnectList()
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, openIDConnectList)
		Expect(err).NotTo(HaveOccurred())

		Expect(openIDConnectList.Items).To(HaveLen(3))

		// remove last element from auth spec identity providers
		auth.Spec.IdentityProviders = auth.Spec.IdentityProviders[:len(auth.Spec.IdentityProviders)-1]

		err = env.Client(testutils.CrateCluster).Update(env.Ctx, auth)
		Expect(err).NotTo(HaveOccurred())

		_ = env.ShouldReconcile(authReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)).To(Succeed())

		err = env.Client(testutils.APIServerCluster).List(env.Ctx, openIDConnectList)
		Expect(err).NotTo(HaveOccurred())
		Expect(openIDConnectList.Items).To(HaveLen(2))

		auth.Spec.EnableSystemIdentityProvider = ptr.To(false)
		err = env.Client(testutils.CrateCluster).Update(env.Ctx, auth)
		Expect(err).NotTo(HaveOccurred())

		_ = env.ShouldReconcile(authReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)).To(Succeed())

		err = env.Client(testutils.APIServerCluster).List(env.Ctx, openIDConnectList)
		Expect(err).NotTo(HaveOccurred())
		Expect(openIDConnectList.Items).To(HaveLen(1))

		auth.Spec.IdentityProviders = append(auth.Spec.IdentityProviders, openmcpv1alpha1.IdentityProvider{
			Name:          "new",
			IssuerURL:     "https://new.local",
			ClientID:      "new-client-id",
			UsernameClaim: "new-username",
			GroupsClaim:   "new-groups",
		})

		err = env.Client(testutils.CrateCluster).Update(env.Ctx, auth)
		Expect(err).NotTo(HaveOccurred())

		_ = env.ShouldReconcile(authReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)).To(Succeed())

		err = env.Client(testutils.APIServerCluster).List(env.Ctx, openIDConnectList)
		Expect(err).NotTo(HaveOccurred())
		Expect(openIDConnectList.Items).To(HaveLen(2))

		accessSecret := &corev1.Secret{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test.kubeconfig", Namespace: "test"}, accessSecret)
		Expect(err).NotTo(HaveOccurred())

		Expect(accessSecret.StringData).To(HaveKey("kubeconfig"))
		config, err := clientcmd.NewClientConfigFromBytes([]byte(accessSecret.StringData["kubeconfig"]))
		Expect(err).NotTo(HaveOccurred())

		rawConfig, err := config.RawConfig()
		Expect(err).NotTo(HaveOccurred())

		Expect(rawConfig.AuthInfos).To(HaveLen(2))
		Expect(rawConfig.AuthInfos).To(HaveKey("new"))
		Expect(rawConfig.AuthInfos).To(HaveKey("customer"))
	})

	It("should delete authentication", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-07")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		authComp := components.Component(auth)

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())

		openIDConnectList := getOpenIDConnectList()
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, openIDConnectList)
		Expect(err).NotTo(HaveOccurred())
		Expect(openIDConnectList.Items).To(HaveLen(2))

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, auth)
		Expect(err).NotTo(HaveOccurred())

		req = testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.APIServerCluster).List(env.Ctx, openIDConnectList)
		Expect(err).NotTo(HaveOccurred())
		Expect(openIDConnectList.Items).To(HaveLen(0))

		accessSecret := &corev1.Secret{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test.kubeconfig", Namespace: "test"}, accessSecret)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		Expect(as.Finalizers).ToNot(ContainElement(authComp.Type().DependencyFinalizer()))
	})

	It("reconcile should handle when authentication is not found", func() {
		env := testEnvWithAPIServerAccess()

		env.ShouldReconcile(authReconciler, testing.RequestFromObject(&openmcpv1alpha1.Authentication{
			ObjectMeta: v1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}))
	})

	It("should handle the ignore annotation", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-08")
		apiServerAccess := &testutils.TestAPIServerAccess{Client: env.Client(testutils.APIServerCluster)}
		env.Reconcilers[authReconciler].(*authentication.AuthenticationReconciler).SetAPIServerAccess(apiServerAccess)

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Status.Conditions).To(HaveLen(0))

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: systemIdentityProvider.Name}, getOpenIDConnect())
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should handle the reconcile annotation", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-09")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Annotations).ToNot(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile))

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: systemIdentityProvider.Name}, getOpenIDConnect())
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not be deleted when it has a dependency finalizer", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-10")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, auth)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Finalizers).To(ContainElement(openmcpv1alpha1.AuthenticationComponent.Finalizer()))
		Expect(auth.Finalizers).To(ContainElement("dependency." + openmcpv1alpha1.BaseDomain + "/other_comp"))
	})

	It("should create a client kubeconfig with client secret and own extra scopes", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-11")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Finalizers).To(ContainElements(openmcpv1alpha1.AuthenticationComponent.Finalizer()))

		Expect(auth.Status.ExternalAuthenticationStatus).NotTo(BeNil())
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess).ToNot(BeNil())
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess.Key).To(Equal("kubeconfig"))
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess.Name).To(Equal("test.kubeconfig"))
		Expect(auth.Status.ExternalAuthenticationStatus.UserAccess.Namespace).To(Equal("test"))

		accessSecret := &corev1.Secret{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test.kubeconfig", Namespace: "test"}, accessSecret)
		Expect(err).NotTo(HaveOccurred())

		Expect(accessSecret.StringData).To(HaveKey("kubeconfig"))
		config, err := clientcmd.NewClientConfigFromBytes([]byte(accessSecret.StringData["kubeconfig"]))
		Expect(err).NotTo(HaveOccurred())

		rawConfig, err := config.RawConfig()
		Expect(err).NotTo(HaveOccurred())

		Expect(rawConfig.AuthInfos).To(HaveKey("customer"))

		systemIdP := rawConfig.AuthInfos["customer"]
		Expect(systemIdP.Exec).ToNot(BeNil())
		Expect(systemIdP.Exec.Command).To(Equal("kubectl"))
		Expect(systemIdP.Exec.Args).To(ContainElements("oidc-login"))
		Expect(systemIdP.Exec.Args).To(ContainElements("get-token"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-issuer-url=https://customer.local"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-client-id=xxx-yyy-zzz"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-client-secret=myclientsecret"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-extra-scope=scope1"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-extra-scope=scope2"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--oidc-use-pkce"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--grant-type=auto"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--extra-param=foo"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--extra-repeatable=bar1"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--extra-repeatable=bar2"))
		Expect(systemIdP.Exec.Args).To(ContainElements("--no-value-param"))

		openIdConnect := getOpenIDConnect()
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: "customer"}, openIdConnect)
		Expect(err).NotTo(HaveOccurred())

		Expect(openIdConnect.Object).To(HaveKey("spec"))
		spec := openIdConnect.Object["spec"]
		Expect(spec).To(HaveKeyWithValue("issuerURL", "https://customer.local"))
		Expect(spec).To(HaveKeyWithValue("clientID", "xxx-yyy-zzz"))
		Expect(spec).To(HaveKeyWithValue("usernameClaim", "u_name"))
		Expect(spec).To(HaveKeyWithValue("groupsClaim", "grp"))
		Expect(spec).To(HaveKeyWithValue("usernamePrefix", "customer:"))
		Expect(spec).To(HaveKeyWithValue("groupsPrefix", "customer:"))
		Expect(spec).To(HaveKeyWithValue("caBundle", "-----BEGIN CERTIFICATE-----\nLi4u\n-----END CERTIFICATE-----\n"))
		Expect(spec).To(HaveKeyWithValue("signingAlgs", []interface{}{"RS256", "RS384", "RS512"}))
		Expect(spec).To(HaveKeyWithValue("requiredClaims", map[string]interface{}{"myClaimKey": "myClaimValue"}))
	})

	It("should create the crate openid connect resource with the correct values", func() {
		var err error

		env := testEnvWithAPIServerAccessWithCrateIdentityProvider("testdata", "test-05")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Finalizers).To(ContainElements(openmcpv1alpha1.AuthenticationComponent.Finalizer()))

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		authComp := components.Component(auth)
		Expect(as.Finalizers).To(ContainElements(authComp.Type().DependencyFinalizer()))

		Expect(auth.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))

		openIdConnect := getOpenIDConnect()

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: crateIdentityProvider.Name}, openIdConnect)
		Expect(err).NotTo(HaveOccurred())

		Expect(openIdConnect.Object).To(HaveKey("spec"))
		spec := openIdConnect.Object["spec"]
		Expect(spec).To(HaveKeyWithValue("issuerURL", crateIdentityProvider.IssuerURL))
		Expect(spec).To(HaveKeyWithValue("clientID", crateIdentityProvider.ClientID))
		Expect(spec).To(HaveKeyWithValue("usernameClaim", crateIdentityProvider.UsernameClaim))
		Expect(spec).To(HaveKeyWithValue("usernamePrefix", crateIdentityProvider.Name+":"))

		// the crate identity provider should not be contained in the user access secret
		Expect(auth.Status.UserAccess).ToNot(BeNil())

		accessSecret := &corev1.Secret{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: auth.Status.UserAccess.Name, Namespace: auth.Status.UserAccess.Namespace}, accessSecret)
		Expect(err).NotTo(HaveOccurred())

		Expect(accessSecret.StringData).To(HaveKey("kubeconfig"))
		config, err := clientcmd.NewClientConfigFromBytes([]byte(accessSecret.StringData["kubeconfig"]))
		Expect(err).NotTo(HaveOccurred())

		rawConfig, err := config.RawConfig()
		Expect(err).NotTo(HaveOccurred())

		Expect(rawConfig.AuthInfos).ToNot(HaveKey(crateIdentityProvider.Name))
	})

	It("should not accept duplicate identity provider names", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-12")

		auth := &openmcpv1alpha1.Authentication{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldNotReconcile(authReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())

		Expect(auth.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthenticationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonReconciliationError,
			}),
		))
	})
})
