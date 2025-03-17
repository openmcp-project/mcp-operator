package components_test

import (
	"sync"
	"time"

	componentutils "github.tools.sap/CoLa/mcp-operator/internal/utils/components"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"

	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

var _ = Describe("Dependencies", func() {

	Context("GetDependents", func() {

		It("should return no dependencies if no finalizers at all are present", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-finalizers", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.GetDependents(ls)).To(BeEmpty())
		})

		It("should return no dependencies if no dependency finalizers are present", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-dependencies", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.GetDependents(ls)).To(BeEmpty())
		})

		It("should return all dependencies if dependency finalizers are present", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "with-dependencies", Namespace: "test"}, ls)).To(Succeed())
			deps := componentutils.GetDependents(ls)
			Expect(deps.UnsortedList()).To(ConsistOf("authentication", "apiserver"))
		})

	})

	Context("HasAnyDependencyFinalizer", func() {

		It("should determine existence of dependency finalizers correctly", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-finalizers", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.HasAnyDependencyFinalizer(ls)).To(BeFalse())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-dependencies", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.HasAnyDependencyFinalizer(ls)).To(BeFalse())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "with-dependencies", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.HasAnyDependencyFinalizer(ls)).To(BeTrue())
		})

	})

	Context("HasDepedencyFinalizer", func() {

		It("should correctly identify dependency finalizers", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-finalizers", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.AuthenticationComponent)).To(BeFalse())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.AuthorizationComponent)).To(BeFalse())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.APIServerComponent)).To(BeFalse())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-dependencies", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.AuthenticationComponent)).To(BeFalse())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.AuthorizationComponent)).To(BeFalse())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.APIServerComponent)).To(BeFalse())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "with-dependencies", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.AuthenticationComponent)).To(BeTrue())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.AuthorizationComponent)).To(BeFalse())
			Expect(componentutils.HasDepedencyFinalizer(ls, openmcpv1alpha1.APIServerComponent)).To(BeTrue())
		})

	})

	Context("EnsureDependencyFinalizer", func() {

		It("should add the finalizer if it does not exist and expected is true", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-finalizers", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, true)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls.GetFinalizers()).To(ConsistOf(openmcpv1alpha1.AuthenticationComponent.DependencyFinalizer()))

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-dependencies", Namespace: "test"}, ls)).To(Succeed())
			fins := ls.GetFinalizers()
			Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, true)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls.GetFinalizers()).To(ConsistOf(append(fins, openmcpv1alpha1.AuthenticationComponent.DependencyFinalizer())))
		})

		It("should not do anything if the finalizer already exists and expected is true", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "with-dependencies", Namespace: "test"}, ls)).To(Succeed())
			oldLs := ls.DeepCopy()
			Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, true)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls).To(Equal(oldLs))
		})

		It("should remove the finalizer if it exists and expected is false", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "with-dependencies", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, false)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls.GetFinalizers()).NotTo(ContainElement(openmcpv1alpha1.AuthenticationComponent.DependencyFinalizer()))
		})

		It("should not do anything if the finalizer does not exist and expected is false", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-finalizers", Namespace: "test"}, ls)).To(Succeed())
			oldLs := ls.DeepCopy()
			Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, false)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls).To(Equal(oldLs))

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-dependencies", Namespace: "test"}, ls)).To(Succeed())
			oldLs = ls.DeepCopy()
			Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, false)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls).To(Equal(oldLs))
		})

		It("should handle updating the finalizers correctly when updated from multiple controllers", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-dependencies", Namespace: "test"}, ls)).To(Succeed())

			wg := sync.WaitGroup{}
			wg.Add(4)

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, true)).To(Succeed())
				wg.Done()
			}()

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.APIServer{}, true)).To(Succeed())
				wg.Done()
			}()

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authorization{}, true)).To(Succeed())
				wg.Done()
			}()

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.CloudOrchestrator{}, true)).To(Succeed())
				wg.Done()
			}()

			Eventually(func() bool {
				wg.Wait()
				return true
			}).WithTimeout(100 * time.Millisecond).Should(BeTrue())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls.GetFinalizers()).To(ConsistOf(
				"foo.bar.baz/foobar",
				openmcpv1alpha1.APIServerComponent.DependencyFinalizer(),
				openmcpv1alpha1.AuthenticationComponent.DependencyFinalizer(),
				openmcpv1alpha1.AuthorizationComponent.DependencyFinalizer(),
				openmcpv1alpha1.CloudOrchestratorComponent.DependencyFinalizer(),
			))

			wg = sync.WaitGroup{}
			wg.Add(4)

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authentication{}, false)).To(Succeed())
				wg.Done()
			}()

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.APIServer{}, false)).To(Succeed())
				wg.Done()
			}()

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.Authorization{}, false)).To(Succeed())
				wg.Done()
			}()

			go func() {
				Expect(componentutils.EnsureDependencyFinalizer(env.Ctx, env.Client(testutils.CrateCluster), ls, &openmcpv1alpha1.CloudOrchestrator{}, false)).To(Succeed())
				wg.Done()
			}()

			Eventually(func() bool {
				wg.Wait()
				return true
			}).WithTimeout(100 * time.Millisecond).Should(BeTrue())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)).To(Succeed())
			Expect(ls.GetFinalizers()).To(ConsistOf("foo.bar.baz/foobar"))
		})
	})

	Context("IsDependencyReady", func() {

		var mcpGen int64 = 3
		var icGen int64 = -1

		It("should return false if the dependency's condition is not True", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-03").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "notready-false", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen)).To(BeFalse())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "notready-unknown", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen)).To(BeFalse())
		})

		It("should return false if the dependency's observed generations are outdated", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-03").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "notready-mcpgen", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen)).To(BeFalse())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "notready-icgen", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen)).To(BeFalse())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "notready-rgen", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen)).To(BeFalse())
		})

		It("should return false if the dependency's generations differ from the own ones", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-03").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "ready", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen+1, icGen)).To(BeFalse())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "ready", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen+1)).To(BeFalse())
		})

		It("should return false if a relevant condition does not exist", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-03").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "ready", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen, "doesNotExistCondition")).To(BeFalse())
		})

		It("should return true if all of the relevant conditions are True", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-03").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "notready-false", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen, "isReady")).To(BeTrue())
		})

		It("should return true if no conditions are specified and all existing ones are True", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-03").Build()
			ls := &openmcpv1alpha1.Landscaper{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "ready", Namespace: "test"}, ls)).To(Succeed())
			Expect(componentutils.IsDependencyReady(ls, mcpGen, icGen)).To(BeTrue())
		})

	})

})
