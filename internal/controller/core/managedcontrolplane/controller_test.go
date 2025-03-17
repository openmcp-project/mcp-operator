package managedcontrolplane_test

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.tools.sap/CoLa/mcp-operator/internal/components"

	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/managedcontrolplane"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openmcptesting "github.tools.sap/CoLa/controller-utils/pkg/testing"

	. "github.tools.sap/CoLa/mcp-operator/test/matchers"

	cconst "github.tools.sap/CoLa/mcp-operator/api/constants"
	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	return managedcontrolplane.NewManagedControlPlaneController(c[0])
}

const (
	mcpReconciler = "mcp"
)

var _ = Describe("CO-1153 ManagedControlPlane Controller", func() {
	It("should create all component resources that are configured in the MCP and delete them again when they are unconfigured", func() {
		var err error
		env := testutils.DefaultTestSetupBuilder("testdata", "test-01").WithReconcilerConstructor(mcpReconciler, getReconciler, testutils.CrateCluster).Build()

		// get ManagedControlPlane
		mcp := &openmcpv1alpha1.ManagedControlPlane{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, mcp)
		Expect(err).NotTo(HaveOccurred())

		req := openmcptesting.RequestFromObject(mcp)
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// check for all component resources
		for ct, ch := range components.Registry.GetKnownComponents() {
			if ch != nil && ch.Resource() != nil && ch.Converter() != nil && ch.Converter().IsConfigured(mcp) {
				Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed(), "unable to get resource for component %s", ct)

				// verify that the resource looks like expected
				genSpec, err := ch.Converter().ConvertToResourceSpec(mcp, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(ch.Resource().GetSpec()).To(Equal(genSpec))

				// check for mcp labels
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName, mcp.Name))
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace, mcp.Namespace))
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneGenerationLabel, fmt.Sprint(mcp.Generation)))
				Expect(ch.Resource().GetLabels()).ToNot(HaveKey(openmcpv1alpha1.InternalConfigurationGenerationLabel))
			}
		}

		// unconfigure all components one by one and verify their deletion
		// CloudOrchestrator
		mcp.Spec.Components.BTPServiceOperator = nil
		mcp.Spec.Components.Crossplane = nil
		mcp.Spec.Components.ExternalSecretsOperator = nil
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch := components.Registry.GetComponent(openmcpv1alpha1.CloudOrchestratorComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeFalse(), "despite removal from spec, CloudOrchestrator is still considered to be configured - most likely, the spec was expanded without adapting this test")
		env.ShouldReconcile(mcpReconciler, req)
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// Landscaper
		mcp.Spec.Components.Landscaper = nil
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.LandscaperComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeFalse(), "despite removal from spec, Landscaper is still considered to be configured - most likely, the spec was expanded without adapting this test")
		env.ShouldReconcile(mcpReconciler, req)
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// APIServer
		mcp.Spec.Components.APIServer = nil
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.APIServerComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeFalse(), "despite removal from spec, APIServer is still considered to be configured - most likely, the spec was expanded without adapting this test")
		env.ShouldReconcile(mcpReconciler, req)
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// Authentication
		mcp.Spec.Authentication = nil
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.AuthenticationComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeFalse(), "despite removal from spec, Authentication is still considered to be configured - most likely, the spec was expanded without adapting this test")
		env.ShouldReconcile(mcpReconciler, req)
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// Authorization
		mcp.Spec.Authorization = nil
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.AuthorizationComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeFalse(), "despite removal from spec, Authorization is still considered to be configured - most likely, the spec was expanded without adapting this test")
		env.ShouldReconcile(mcpReconciler, req)
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
	})

	It("should not create unconfigured components, add/modify them if added/modified later, and delete all component resources when the MCP is deleted", func() {
		var err error
		env := testutils.DefaultTestSetupBuilder("testdata", "test-02").WithReconcilerConstructor(mcpReconciler, getReconciler, testutils.CrateCluster).Build()

		// get ManagedControlPlane
		mcp := &openmcpv1alpha1.ManagedControlPlane{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, mcp)
		Expect(err).NotTo(HaveOccurred())

		req := openmcptesting.RequestFromObject(mcp)
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// check for all component resources
		for ct, ch := range components.Registry.GetKnownComponents() {
			if ch != nil && ch.Resource() != nil && ch.Converter() != nil && ch.Converter().IsConfigured(mcp) {
				err := env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())
				Expect(err).To(HaveOccurred(), "resource for component %s should not exist", ct)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		}

		// load MCP from test-01 because it is fully configured
		fullMcp := &openmcpv1alpha1.ManagedControlPlane{}
		data, err := os.ReadFile(path.Join("testdata", "test-01", "mcp.yaml"))
		Expect(err).NotTo(HaveOccurred())
		decoder := serializer.NewCodecFactory(env.Client(testutils.CrateCluster).Scheme()).UniversalDeserializer()
		obj, _, err := decoder.Decode(data, nil, fullMcp)
		Expect(err).NotTo(HaveOccurred())
		fullMcp, ok := obj.(*openmcpv1alpha1.ManagedControlPlane)
		Expect(ok).To(BeTrue())

		// configure all components one by one and verify their creation
		// Authentication
		mcp.Spec.Authentication = fullMcp.Spec.Authentication.DeepCopy()
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch := components.Registry.GetComponent(openmcpv1alpha1.AuthenticationComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeTrue())
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// Authorization
		mcp.Spec.Authorization = fullMcp.Spec.Authorization.DeepCopy()
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.AuthorizationComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeTrue())
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// APIServer
		mcp.Spec.Components.APIServer = fullMcp.Spec.Components.APIServer.DeepCopy()
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.APIServerComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeTrue())
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// Landscaper
		mcp.Spec.Components.Landscaper = fullMcp.Spec.Components.Landscaper.DeepCopy()
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.LandscaperComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeTrue())
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// CloudOrchestrator
		mcp.Spec.Components.CloudOrchestratorConfiguration = *(&fullMcp.Spec.Components.CloudOrchestratorConfiguration).DeepCopy()
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, mcp)).To(Succeed())
		ch = components.Registry.GetComponent(openmcpv1alpha1.CloudOrchestratorComponent)
		Expect(ch.Converter().IsConfigured(mcp)).To(BeTrue())
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed())
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// delete the MCP and verify that all component resources are deleted
		Expect(env.Client(testutils.CrateCluster).Delete(env.Ctx, mcp)).To(Succeed())
		env.ShouldReconcile(mcpReconciler, req)
		for ct, ch := range components.Registry.GetKnownComponents() {
			if ch != nil && ch.Resource() != nil && ch.Converter() != nil && ch.Converter().IsConfigured(mcp) {
				err := env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())
				Expect(err).To(HaveOccurred(), "resource for component %s should not exist", ct)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		}
	})

	It("shouldn't show status conditions of CloudOrchestrator internal conditions in lower case when MCP is in deletion", func() {
		env := testutils.DefaultTestSetupBuilder("testdata", "test-05").WithReconcilerConstructor(mcpReconciler, getReconciler, testutils.CrateCluster).Build()

		// get ManagedControlPlane
		mcp := &openmcpv1alpha1.ManagedControlPlane{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, mcp)).To(Succeed())

		// get CloudOrchestrator
		co := &openmcpv1alpha1.CloudOrchestrator{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), co)).To(Succeed())

		// delete MCP
		Expect(env.Client(testutils.CrateCluster).Delete(env.Ctx, mcp)).To(Succeed())

		req := openmcptesting.RequestFromObject(mcp)
		env.ShouldReconcile(mcpReconciler, req)

		// check if CloudOrchestrator status has condition additionalUnexportedCondition starting with lower case
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)).To(Succeed())
		Expect(co.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    "additionalUnexportedCondition",
				Status:  openmcpv1alpha1.ComponentConditionStatusTrue,
				Reason:  "",
				Message: "",
			}),
		))
		Expect(co.Status.Conditions).To(HaveLen(2))

		// check if MCP status conditions to not have additionalUnexportedCondition while MCP is in deletion
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
		Expect(mcp.Status.Conditions).ToNot(ConsistOf(
			MatchFields(0, Fields{
				"ManagedBy": Equal(openmcpv1alpha1.CloudOrchestratorComponent),
				"ComponentCondition": MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   "additionalUnexportedCondition",
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			}),
		))
		Expect(mcp.Status.Conditions).To(HaveLen(2))
	})

	It("should apply InternalConfigurations correctly", func() {
		var err error
		env := testutils.DefaultTestSetupBuilder("testdata", "test-03").WithReconcilerConstructor(mcpReconciler, getReconciler, testutils.CrateCluster).Build()

		// get ManagedControlPlane
		mcp := &openmcpv1alpha1.ManagedControlPlane{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, mcp)
		Expect(err).NotTo(HaveOccurred())

		// get InternalConfiguration
		ic := &openmcpv1alpha1.InternalConfiguration{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ic)
		Expect(err).NotTo(HaveOccurred())

		req := openmcptesting.RequestFromObject(mcp)
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// check for all component resources
		for ct, ch := range components.Registry.GetKnownComponents() {
			if ch != nil && ch.Resource() != nil && ch.Converter() != nil && ch.Converter().IsConfigured(mcp) {
				Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed(), "unable to get resource for component %s", ct)

				// verify that the resource looks like expected
				genSpec, err := ch.Converter().ConvertToResourceSpec(mcp, ic)
				Expect(err).NotTo(HaveOccurred())
				Expect(ch.Resource().GetSpec()).To(Equal(genSpec))

				// check for mcp labels
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName, mcp.Name))
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace, mcp.Namespace))
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneGenerationLabel, fmt.Sprint(mcp.Generation)))
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.InternalConfigurationGenerationLabel, fmt.Sprint(ic.Generation)))
			}
		}

		// delete InternalConfiguration and verify that component resources are updated
		Expect(env.Client(testutils.CrateCluster).Delete(env.Ctx, ic)).To(Succeed())
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

		// check for all component resources
		for ct, ch := range components.Registry.GetKnownComponents() {
			if ch != nil && ch.Resource() != nil && ch.Converter() != nil && ch.Converter().IsConfigured(mcp) {
				Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed(), "unable to get resource for component %s", ct)

				// verify that the resource looks like expected
				genSpec, err := ch.Converter().ConvertToResourceSpec(mcp, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(ch.Resource().GetSpec()).To(Equal(genSpec))

				// check for mcp labels
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName, mcp.Name))
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace, mcp.Namespace))
				Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneGenerationLabel, fmt.Sprint(mcp.Generation)))
				Expect(ch.Resource().GetLabels()).ToNot(HaveKey(openmcpv1alpha1.InternalConfigurationGenerationLabel))
			}
		}
	})

	It("should sync conditions and status back to ManagedControlPlane from component resources", func() {
		env := testutils.DefaultTestSetupBuilder("testdata", "test-04").WithReconcilerConstructor(mcpReconciler, getReconciler, testutils.CrateCluster).Build()

		// get ManagedControlPlane
		mcp := &openmcpv1alpha1.ManagedControlPlane{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, mcp)).To(Succeed())

		// get Authentication
		auth := &openmcpv1alpha1.Authentication{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), auth)).To(Succeed())

		req := openmcptesting.RequestFromObject(mcp)
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
		time.Sleep(1 * time.Second) // without this, it cannot be verified that the lastTransitionTime is not updated if nothing changes, because the test is too fast for the second precision of the timestamps

		// verify status
		Expect(mcp.Status.Components.Authentication).To(Equal(auth.Status.ExternalAuthenticationStatus))

		// verify conditions
		Expect(mcp.Status.Conditions).To(ConsistOf(
			MatchFields(0, Fields{
				"ManagedBy": Equal(openmcpv1alpha1.LandscaperComponent),
				"ComponentCondition": MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   "AdditionalCondition",
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			}),
			MatchFields(0, Fields{
				"ManagedBy": BeEmpty(),
				"ComponentCondition": MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionMCPSuccessful,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
					Reason: cconst.ReasonAllComponentsReconciledSuccessfully,
				}),
			}),
			MatchFields(0, Fields{
				"ManagedBy": Equal(openmcpv1alpha1.AuthenticationComponent),
				"ComponentCondition": MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   string(openmcpv1alpha1.AuthenticationComponent),
					Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
					Reason: cconst.ReasonNoConditions,
				}),
			}),
		))

		oldConditions := map[string]*openmcpv1alpha1.ManagedControlPlaneComponentCondition{}
		for _, con := range mcp.Status.Conditions {
			oldConditions[con.Type] = con.DeepCopy()
		}

		// reconcile again to ensure that conditions' lastTransitionTime remains stable if nothing changes
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
		for _, oldCon := range oldConditions {
			Expect(mcp.Status.Conditions).To(ContainElement(MatchFields(0, Fields{
				"ManagedBy":          Equal(oldCon.ManagedBy),
				"ComponentCondition": MatchComponentCondition(oldCon.ComponentCondition),
			})), "displaying expected timestamp for better debugging: %s", oldCon.LastTransitionTime.Format(time.RFC3339))
		}
		time.Sleep(1 * time.Second) // without this, it cannot be verified that the lastTransitionTime is properly updated, because the test is too fast for the second precision of the timestamps

		// set authentication condition to false and verify again
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), auth)).To(Succeed())
		auth.Status.Conditions = append(auth.Status.Conditions, openmcpv1alpha1.ComponentCondition{
			Type:               openmcpv1alpha1.AuthenticationComponent.ReconciliationCondition(),
			Status:             openmcpv1alpha1.ComponentConditionStatusFalse,
			Reason:             "TestReason",
			Message:            "Test Error Message",
			LastTransitionTime: metav1.Now(),
		})
		Expect(env.Client(testutils.CrateCluster).Status().Update(env.Ctx, auth)).To(Succeed())
		// reconcile MCP
		env.ShouldReconcile(mcpReconciler, req)
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
		// verify conditions
		Expect(mcp.Status.Conditions).To(ConsistOf(
			MatchFields(0, Fields{
				"ManagedBy": Equal(openmcpv1alpha1.LandscaperComponent),
				"ComponentCondition": MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   "AdditionalCondition",
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			}),
			MatchFields(0, Fields{
				"ManagedBy": BeEmpty(),
				"ComponentCondition": MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:    cconst.ConditionMCPSuccessful,
					Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
					Reason:  cconst.ReasonNotAllComponentsReconciledSuccessfully,
					Message: "The following components could not be reconciled successfully:\n\tAuthentication: [TestReason] Test Error Message",
				}),
			}),
		))
		// verify that lastTransitionTime has changed for the conditions where the status changed
		for _, con := range mcp.Status.Conditions {
			if oldCon, ok := oldConditions[con.Type]; ok {
				if oldCon.Status == con.Status {
					Expect(con.LastTransitionTime).To(Equal(oldCon.LastTransitionTime))
				} else {
					Expect(oldCon.LastTransitionTime.Before(&con.LastTransitionTime)).To(BeTrue(), "lastTransitionTime '%s' of condition '%s' should have been updated", oldCon.LastTransitionTime.Format(time.RFC3339), con.Type)
				}
			}
		}
	})

	It("should add project and workspace metadata to MCP and all component resources, if present in namespace", func() {
		var err error
		env := testutils.DefaultTestSetupBuilder("testdata", "test-01").WithReconcilerConstructor(mcpReconciler, getReconciler, testutils.CrateCluster).Build()
		ns := &corev1.Namespace{}
		ns.SetName("test")
		ns.SetLabels(map[string]string{
			"core.openmcp.cloud/project": "test-project",
		})
		Expect(env.Client(testutils.CrateCluster).Create(env.Ctx, ns)).To(Succeed())

		// get ManagedControlPlane
		mcp := &openmcpv1alpha1.ManagedControlPlane{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, mcp)
		Expect(err).NotTo(HaveOccurred())

		expectProjectLabel := true
		expectWorkspaceLabel := false

		reconcileAndTest := func() {
			req := openmcptesting.RequestFromObject(mcp)
			env.ShouldReconcile(mcpReconciler, req)
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), mcp)).To(Succeed())
			if expectProjectLabel {
				Expect(mcp.GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject, "test-project"))
			} else {
				Expect(mcp.GetLabels()).ToNot(HaveKey(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject))
			}
			if expectWorkspaceLabel {
				Expect(mcp.GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace, "test-workspace"))
			} else {
				Expect(mcp.GetLabels()).ToNot(HaveKey(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace))
			}

			// check for all component resources
			for ct, ch := range components.Registry.GetKnownComponents() {
				if ch != nil && ch.Resource() != nil && ch.Converter() != nil && ch.Converter().IsConfigured(mcp) {
					Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(mcp), ch.Resource())).To(Succeed(), "unable to get resource for component %s", ct)

					// verify that the resource looks like expected
					genSpec, err := ch.Converter().ConvertToResourceSpec(mcp, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(ch.Resource().GetSpec()).To(Equal(genSpec))

					// check for mcp labels
					Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName, mcp.Name))
					Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace, mcp.Namespace))
					if expectProjectLabel {
						Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject, "test-project"))
					} else {
						Expect(ch.Resource().GetLabels()).ToNot(HaveKey(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject))
					}
					if expectWorkspaceLabel {
						Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace, "test-workspace"))
					} else {
						Expect(ch.Resource().GetLabels()).ToNot(HaveKey(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace))
					}
					Expect(ch.Resource().GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneGenerationLabel, fmt.Sprint(mcp.Generation)))
					Expect(ch.Resource().GetLabels()).ToNot(HaveKey(openmcpv1alpha1.InternalConfigurationGenerationLabel))
				}
			}
		}

		// test with project label only
		By("project label only")
		reconcileAndTest()

		// test with project and workspace label
		By("project and workspace label")
		ns.SetLabels(map[string]string{
			"core.openmcp.cloud/project":   "test-project",
			"core.openmcp.cloud/workspace": "test-workspace",
		})
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, ns)).To(Succeed())
		expectWorkspaceLabel = true
		reconcileAndTest()

		// test without project and workspace label
		By("no project or workspace label")
		ns.SetLabels(map[string]string{})
		Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, ns)).To(Succeed())
		expectProjectLabel = false
		expectWorkspaceLabel = false
		reconcileAndTest()
	})

})

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManagedControlPlane Controller Test Suite")
}
