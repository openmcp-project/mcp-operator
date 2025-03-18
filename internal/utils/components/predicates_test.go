package components_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openmcp-project/mcp-operator/internal/utils/components"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("Predicates", func() {

	var base *openmcpv1alpha1.ManagedControlPlane
	var changed *openmcpv1alpha1.ManagedControlPlane

	BeforeEach(func() {
		base = &openmcpv1alpha1.ManagedControlPlane{}
		base.SetName("foo")
		base.SetNamespace("bar")
		base.SetGeneration(1)
		changed = base.DeepCopy()
	})

	It("should detect changes to the generation labels", func() {
		p := components.GenerationLabelsChangedPredicate{}
		components.SetCreatedFromGeneration(base, mcpWithGeneration(1), nil)
		components.SetCreatedFromGeneration(changed, mcpWithGeneration(1), nil)
		Expect(p.Update(updateEvent(base, changed))).To(BeFalse(), "GenerationLabelsChangedPredicate should return false if the generation labels did not change")
		By("change mcp generation label")
		components.SetCreatedFromGeneration(changed, mcpWithGeneration(2), nil)
		Expect(p.Update(updateEvent(base, changed))).To(BeTrue(), "GenerationLabelsChangedPredicate should return true if the MCP generation label changed")
		By("add ic generation label")
		base = changed.DeepCopy()
		components.SetCreatedFromGeneration(changed, mcpWithGeneration(2), mcpWithGeneration(2))
		Expect(p.Update(updateEvent(base, changed))).To(BeTrue(), "GenerationLabelsChangedPredicate should return true if the IC generation label was added")
		By("change ic generation label")
		base = changed.DeepCopy()
		components.SetCreatedFromGeneration(changed, mcpWithGeneration(2), mcpWithGeneration(3))
		Expect(p.Update(updateEvent(base, changed))).To(BeTrue(), "GenerationLabelsChangedPredicate should return true if the IC generation label was changed")
		By("remove ic generation label")
		base = changed.DeepCopy()
		components.SetCreatedFromGeneration(changed, mcpWithGeneration(2), nil)
		Expect(p.Update(updateEvent(base, changed))).To(BeTrue(), "GenerationLabelsChangedPredicate should return true if the IC generation label was removed")
	})

	It("should detect changes to the status", func() {
		p := components.StatusChangedPredicate{}
		Expect(p.Update(updateEvent(base, changed))).To(BeFalse(), "StatusChangedPredicate should return false if the status did not change")
		By("change status")
		changed.Status = openmcpv1alpha1.ManagedControlPlaneStatus{
			ManagedControlPlaneMetaStatus: openmcpv1alpha1.ManagedControlPlaneMetaStatus{
				ObservedGeneration: 1,
			},
		}
		Expect(p.Update(updateEvent(base, changed))).To(BeTrue(), "StatusChangedPredicate should return true if the status changed")
	})

})

func updateEvent(old, new client.Object) event.UpdateEvent {
	return event.UpdateEvent{
		ObjectOld: old,
		ObjectNew: new,
	}
}

func mcpWithGeneration(gen int64) *openmcpv1alpha1.ManagedControlPlane {
	res := &openmcpv1alpha1.ManagedControlPlane{}
	res.SetGeneration(gen)
	return res
}
