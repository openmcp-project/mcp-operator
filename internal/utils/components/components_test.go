package components_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/openmcp-project/mcp-operator/internal/components"
	componentutils "github.com/openmcp-project/mcp-operator/internal/utils/components"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/openmcp-project/mcp-operator/test/matchers"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
	"github.com/openmcp-project/mcp-operator/test/utils"
)

type TestManagedComponent struct {
	Component components.Component
}

func (tmc TestManagedComponent) Resource() components.Component {
	return tmc.Component
}

// TestComponentRegistry is a simple testing implementation of the ComponentRegistry interface.
type TestComponentRegistry struct {
	Components map[openmcpv1alpha1.ComponentType]TestManagedComponent
}

func (tcr *TestComponentRegistry) GetKnownComponents() map[openmcpv1alpha1.ComponentType]TestManagedComponent {
	return tcr.Components
}

func (tcr *TestComponentRegistry) GetComponent(ct openmcpv1alpha1.ComponentType) TestManagedComponent {
	return tcr.Components[ct]
}

func (tcr *TestComponentRegistry) Register(ct openmcpv1alpha1.ComponentType, f func() TestManagedComponent) {
	if f == nil {
		delete(tcr.Components, ct)
		return
	}
	tcr.Components[ct] = f()
}

func (tcr *TestComponentRegistry) Has(ct openmcpv1alpha1.ComponentType) bool {
	_, ok := tcr.Components[ct]
	return ok
}

var _ = Describe("Components", func() {
	Context("GetComponents", func() {
		It("should get an existing component", func() {
			obj := &openmcpv1alpha1.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}
			component := components.Component(obj)

			registry := &TestComponentRegistry{
				Components: make(map[openmcpv1alpha1.ComponentType]TestManagedComponent),
			}
			registry.Register("apiserver", func() TestManagedComponent {
				return TestManagedComponent{
					Component: component,
				}
			})

			env := testing.NewEnvironmentBuilder().WithFakeClient(utils.Scheme).WithInitObjects(obj).Build()

			receivedComponent, err := componentutils.GetComponent[TestManagedComponent](registry, context.Background(), env.Client(), "apiserver", "test", "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(receivedComponent.Resource()).To(Equal(component))
		})

		It("should return a nil component if it does not exist", func() {
			obj := &openmcpv1alpha1.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}
			component := components.Component(obj)

			registry := &TestComponentRegistry{
				Components: make(map[openmcpv1alpha1.ComponentType]TestManagedComponent),
			}
			registry.Register("apiserver", func() TestManagedComponent {
				return TestManagedComponent{
					Component: component,
				}
			})

			env := testing.NewEnvironmentBuilder().WithFakeClient(utils.Scheme).Build()

			receivedComponent, err := componentutils.GetComponent[TestManagedComponent](registry, context.Background(), env.Client(), "apiserver", "test", "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(receivedComponent.Component).To(BeNil())
		})
	})

	Context("GetComponents", func() {
		It("should get all existing components", func() {
			objAPIServer := &openmcpv1alpha1.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}
			componentAPIServer := components.Component(objAPIServer)

			objAuth := &openmcpv1alpha1.Authentication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}
			componentAuth := components.Component(objAuth)

			objAuthz := &openmcpv1alpha1.Authorization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}
			componentAuthz := components.Component(objAuthz)

			registry := &TestComponentRegistry{
				Components: make(map[openmcpv1alpha1.ComponentType]TestManagedComponent),
			}
			registry.Register("apiserver", func() TestManagedComponent {
				return TestManagedComponent{
					Component: componentAPIServer,
				}
			})
			registry.Register("authentication", func() TestManagedComponent {
				return TestManagedComponent{
					Component: componentAuth,
				}
			})
			registry.Register("authorization", func() TestManagedComponent {
				return TestManagedComponent{
					Component: componentAuthz,
				}
			})

			env := testing.NewEnvironmentBuilder().WithFakeClient(utils.Scheme).WithInitObjects([]client.Object{
				objAPIServer,
				objAuth,
			}...).Build()

			components, err := componentutils.GetComponents[TestManagedComponent](registry, context.Background(), env.Client(), "test", "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(components).To(HaveLen(2))
			Expect(components).To(HaveKey(openmcpv1alpha1.ComponentType("apiserver")))
			Expect(components).To(HaveKey(openmcpv1alpha1.ComponentType("authentication")))
		})
	})

	Context("InvalidGenerationLabelValueError", func() {
		It("should return a correct error message", func() {
			err := componentutils.NewInvalidGenerationLabelValueError("test", true)
			Expect(err.Error()).To(Equal(fmt.Sprintf("value 'test' of label '%s' cannot be parsed into an int64", openmcpv1alpha1.InternalConfigurationGenerationLabel)))

			err = componentutils.NewInvalidGenerationLabelValueError("test", false)
			Expect(err.Error()).To(Equal(fmt.Sprintf("value 'test' of label '%s' cannot be parsed into an int64", openmcpv1alpha1.ManagedControlPlaneGenerationLabel)))
		})

		It("should return a correct error reason", func() {
			err := componentutils.NewInvalidGenerationLabelValueError("test", true)
			Expect(err.Reason()).To(Equal(componentutils.ErrorReasonInvalidManagedControlPlaneLabels))

			err = componentutils.NewInvalidGenerationLabelValueError("test", false)
			Expect(err.Reason()).To(Equal(componentutils.ErrorReasonInvalidManagedControlPlaneLabels))
		})
	})

	Context("GetCreatedFromGeneration", func() {
		It("should return the correct values", func() {
			obj := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						openmcpv1alpha1.ManagedControlPlaneGenerationLabel:   "1",
						openmcpv1alpha1.InternalConfigurationGenerationLabel: "2",
					},
				},
			}

			valCP, valIC, err := componentutils.GetCreatedFromGeneration(obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(valCP).To(Equal(int64(1)))
			Expect(valIC).To(Equal(int64(2)))
		})
	})

	Context("SetCreatedFromGeneration", func() {
		It("should set the correct labels", func() {
			obj := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			}

			cp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
			}

			ic := &openmcpv1alpha1.InternalConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 2,
				},
			}

			componentutils.SetCreatedFromGeneration(obj, cp, ic)
			labels := obj.GetLabels()
			Expect(labels).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneGenerationLabel, "1"))
			Expect(labels).To(HaveKeyWithValue(openmcpv1alpha1.InternalConfigurationGenerationLabel, "2"))
		})
	})

	Context("GenerateCreatedFromGenerationPatch", func() {
		It("returns patch with correct generation labels when both cp and ic are not nil", func() {
			cp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
			}
			ic := &openmcpv1alpha1.InternalConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 2,
				},
			}
			patch := componentutils.GenerateCreatedFromGenerationPatch(cp, ic, false)
			Expect(patch.Type()).To(Equal(types.MergePatchType))

			cpPatch, err := patch.Data(cp)
			Expect(err).ToNot(HaveOccurred())
			icPatch, err := patch.Data(ic)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(cpPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":"1"`, openmcpv1alpha1.ManagedControlPlaneGenerationLabel)))
			Expect(string(cpPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":"2"`, openmcpv1alpha1.InternalConfigurationGenerationLabel)))
			Expect(string(icPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":"1"`, openmcpv1alpha1.ManagedControlPlaneGenerationLabel)))
			Expect(string(icPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":"2"`, openmcpv1alpha1.InternalConfigurationGenerationLabel)))
		})

		It("returns patch with correct generation labels when ic is nil", func() {
			cp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
			}

			patch := componentutils.GenerateCreatedFromGenerationPatch(cp, nil, false)
			Expect(patch.Type()).To(Equal(types.MergePatchType))

			cpPatch, err := patch.Data(cp)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(cpPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":"1"`, openmcpv1alpha1.ManagedControlPlaneGenerationLabel)))
			Expect(string(cpPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":null`, openmcpv1alpha1.InternalConfigurationGenerationLabel)))
		})

		It("returns patch with correct generation labels and reconcile annotation when addReconcileAnnotation is true", func() {
			cp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
			}

			patch := componentutils.GenerateCreatedFromGenerationPatch(cp, nil, true)
			Expect(patch.Type()).To(Equal(types.MergePatchType))

			cpPatch, err := patch.Data(cp)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(cpPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":"1"`, openmcpv1alpha1.ManagedControlPlaneGenerationLabel)))
			Expect(string(cpPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":null`, openmcpv1alpha1.InternalConfigurationGenerationLabel)))
			Expect(string(cpPatch)).To(ContainSubstring(fmt.Sprintf(`"%s":"%s"`, openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile)))
		})
	})

	Context("UpdateStatus", func() {
		It("should update the status of the object", func() {
			as := &openmcpv1alpha1.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					Labels: map[string]string{
						openmcpv1alpha1.ManagedControlPlaneGenerationLabel: "1",
					},
				},
				Status: openmcpv1alpha1.APIServerStatus{
					CommonComponentStatus: openmcpv1alpha1.CommonComponentStatus{
						Conditions: openmcpv1alpha1.ComponentConditionList{
							{
								Type:   openmcpv1alpha1.APIServerComponent.ReconciliationCondition(),
								Status: openmcpv1alpha1.ComponentConditionStatusTrue,
								Reason: "All Good",
							},
						},
						ObservedGenerations: openmcpv1alpha1.ObservedGenerations{
							Resource:              0,
							ManagedControlPlane:   1,
							InternalConfiguration: -1,
						},
					},
				},
			}

			env := testing.NewEnvironmentBuilder().WithFakeClient(utils.Scheme).WithInitObjects(as).Build()

			rr := componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{
				Component:      as,
				Message:        "Internal Error",
				Reason:         "InternalError",
				ReconcileError: openmcperrors.NewReasonableErrorList(fmt.Errorf("quota exceeded")).Aggregate(),
				Conditions: []openmcpv1alpha1.ComponentCondition{
					componentutils.NewCondition("additionalCondition", openmcpv1alpha1.ComponentConditionStatusTrue, "DummyReason", "This is a dummy message."),
				},
			}

			var err error
			result, err := componentutils.UpdateStatus(context.Background(), env.Client(), rr)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, rr.ReconcileError)).To(BeTrue())
			Expect(result.Requeue).To(BeFalse())

			err = env.Client().Get(context.Background(), client.ObjectKeyFromObject(as), as)
			Expect(err).ToNot(HaveOccurred())
			Expect(as.Status.Conditions).To(ConsistOf(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:    openmcpv1alpha1.APIServerComponent.HealthyCondition(),
					Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
					Reason:  cconst.ReasonReconciliationError,
					Message: cconst.MessageReconciliationError,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:    openmcpv1alpha1.APIServerComponent.ReconciliationCondition(),
					Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
					Reason:  "InternalError",
					Message: "Internal Error\nquota exceeded",
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:    "additionalCondition",
					Status:  openmcpv1alpha1.ComponentConditionStatusTrue,
					Reason:  "DummyReason",
					Message: "This is a dummy message.",
				}),
			))
		})
	})
})
