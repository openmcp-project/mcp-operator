package components_test

import (
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	componentutils "github.tools.sap/CoLa/mcp-operator/internal/utils/components"

	"sigs.k8s.io/controller-runtime/pkg/client"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"

	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

var _ = Describe("Conditions", func() {

	Context("GetCondition", func() {

		It("should return the requested condition", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			con := componentutils.GetCondition(ls.Status.Conditions, "true")
			Expect(con).ToNot(BeNil())
			Expect(con.Type).To(Equal("true"))
		})

		It("should return nil if the condition does not exist", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			con := componentutils.GetCondition(ls.Status.Conditions, "doesNotExist")
			Expect(con).To(BeNil())
		})

		It("should return a pointer to the condition which allows changes to the original object", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			con := componentutils.GetCondition(ls.Status.Conditions, ls.Status.Conditions[0].Type) // fetch first condition from list
			Expect(con).ToNot(BeNil())
			con.Status = "SomethingElse"
			Expect(ls.Status.Conditions[0].Status).To(Equal(con.Status))
		})

	})

	Context("ConditionUpdater", func() {

		It("should update the condition (same value, keep other cons)", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			con := componentutils.GetCondition(ls.Status.Conditions, "true")
			Expect(con).ToNot(BeNil())
			updated := componentutils.ConditionUpdater(ls.Status.Conditions, false).UpdateCondition(con.Type, con.Status, "", "").Conditions()
			Expect(len(updated)).To(Equal(len(ls.Status.Conditions)))
			ucon := componentutils.GetCondition(updated, con.Type)
			Expect(ucon).ToNot(BeNil())
			Expect(ucon.Status).To(Equal(con.Status))
			Expect(ucon.LastTransitionTime).To(Equal(con.LastTransitionTime))
			Expect(ucon.Reason).To(Equal(""))
			Expect(ucon.Message).To(Equal(""))
		})

		It("should update the condition (different value, keep other cons)", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			con := componentutils.GetCondition(ls.Status.Conditions, "true")
			Expect(con).ToNot(BeNil())
			updated := componentutils.ConditionUpdater(ls.Status.Conditions, false).UpdateCondition(con.Type, "SomethingElse", "asdf", "foobar").Conditions()
			Expect(len(updated)).To(Equal(len(ls.Status.Conditions)))
			ucon := componentutils.GetCondition(updated, con.Type)
			Expect(ucon).ToNot(BeNil())
			Expect(ucon.Status).To(BeEquivalentTo("SomethingElse"))
			Expect(ucon.LastTransitionTime).ToNot(Equal(con.LastTransitionTime))
			Expect(ucon.Reason).To(Equal("asdf"))
			Expect(ucon.Message).To(Equal("foobar"))
		})

		It("should update the condition (same value, discard other cons)", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			con := componentutils.GetCondition(ls.Status.Conditions, "true")
			Expect(con).ToNot(BeNil())
			updated := componentutils.ConditionUpdater(ls.Status.Conditions, true).UpdateCondition(con.Type, con.Status, "", "").Conditions()
			Expect(len(updated)).To(Equal(1))
			ucon := componentutils.GetCondition(updated, con.Type)
			Expect(ucon).ToNot(BeNil())
			Expect(ucon.Status).To(Equal(con.Status))
			Expect(ucon.LastTransitionTime).To(Equal(con.LastTransitionTime))
			Expect(ucon.Reason).To(Equal(""))
			Expect(ucon.Message).To(Equal(""))
		})

		It("should update the condition (different value, discard other cons)", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			con := componentutils.GetCondition(ls.Status.Conditions, "true")
			Expect(con).ToNot(BeNil())
			updated := componentutils.ConditionUpdater(ls.Status.Conditions, true).UpdateCondition(con.Type, "SomethingElse", "asdf", "foobar").Conditions()
			Expect(len(updated)).To(Equal(1))
			ucon := componentutils.GetCondition(updated, con.Type)
			Expect(ucon).ToNot(BeNil())
			Expect(ucon.Status).To(BeEquivalentTo("SomethingElse"))
			Expect(ucon.LastTransitionTime).ToNot(Equal(con.LastTransitionTime))
			Expect(ucon.Reason).To(Equal("asdf"))
			Expect(ucon.Message).To(Equal("foobar"))
		})

		It("should sort the conditions by type", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-04").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "test", Namespace: "test"}, ls)).To(Succeed())
			oldConditions := ls.Status.Conditions.DeepCopy()
			compareConditions := func(a, b openmcpv1alpha1.ComponentCondition) int {
				return strings.Compare(a.Type, b.Type)
			}
			Expect(slices.IsSortedFunc(oldConditions, compareConditions)).To(BeFalse(), "conditions in the test object are already sorted, unable to test sorting")
			updated := componentutils.ConditionUpdater(ls.Status.Conditions, false).Conditions()
			Expect(len(updated)).To(BeNumerically(">", 1), "test object does not contain enough conditions to test sorting")
			Expect(len(updated)).To(Equal(len(ls.Status.Conditions)))
			Expect(slices.IsSortedFunc(updated, compareConditions)).To(BeTrue(), "conditions are not sorted")
		})

	})

})
