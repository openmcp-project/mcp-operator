package components_test

import (
	"fmt"

	componentutils "github.tools.sap/CoLa/mcp-operator/internal/utils/components"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

var _ = Describe("Patch", func() {

	Context("IsAnnotationAlreadyExistsError", func() {

		It("should return true if the error is of type AnnotationAlreadyExistsError", func() {
			var err error = componentutils.NewAnnotationAlreadyExistsError("test-annotation", "desired-value", "actual-value")
			Expect(componentutils.IsAnnotationAlreadyExistsError(err)).To(BeTrue())
		})

		It("should return false if the error is not of type AnnotationAlreadyExistsError", func() {
			var err error = fmt.Errorf("test-error")
			Expect(componentutils.IsAnnotationAlreadyExistsError(err)).To(BeFalse())
		})

	})

	Context("PatchAnnotation", func() {

		It("should patch the annotation on the object, if it does not exist", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-01").Build()
			ns := &corev1.Namespace{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "no-annotation"}, ns)).To(Succeed())
			Expect(componentutils.PatchAnnotation(env.Ctx, env.Client(testutils.CrateCluster), ns, "foo.bar.baz/foo", "bar")).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
			Expect(ns.GetAnnotations()).To(HaveKeyWithValue("foo.bar.baz/foo", "bar"))
		})

		It("should not fail if the annotation already exists with the desired value", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-01").Build()
			ns := &corev1.Namespace{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "foo-annotation"}, ns)).To(Succeed())
			oldNs := ns.DeepCopy()
			Expect(componentutils.PatchAnnotation(env.Ctx, env.Client(testutils.CrateCluster), ns, "foo.bar.baz/foo", "bar")).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
			Expect(ns.GetAnnotations()).To(HaveKeyWithValue("foo.bar.baz/foo", "bar"))
			Expect(ns).To(Equal(oldNs))
		})

		It("should return an AnnotationAlreadyExistsError if the annotation already exists with a different value", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-01").Build()
			ns := &corev1.Namespace{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "foo-annotation"}, ns)).To(Succeed())
			Expect(componentutils.PatchAnnotation(env.Ctx, env.Client(testutils.CrateCluster), ns, "foo.bar.baz/foo", "baz")).To(MatchError(componentutils.NewAnnotationAlreadyExistsError("foo.bar.baz/foo", "baz", "bar")))
		})

		It("should overwrite the annotation if the mode is set to ANNOTATION_OVERWRITE", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-01").Build()
			ns := &corev1.Namespace{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "foo-annotation"}, ns)).To(Succeed())
			Expect(componentutils.PatchAnnotation(env.Ctx, env.Client(testutils.CrateCluster), ns, "foo.bar.baz/foo", "baz", componentutils.ANNOTATION_OVERWRITE)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
			Expect(ns.GetAnnotations()).To(HaveKeyWithValue("foo.bar.baz/foo", "baz"))
		})

		It("should delete the annotation if the mode is set to ANNOTATION_DELETE", func() {
			env := testutils.DefaultTestSetupBuilder("testdata", "test-01").Build()
			ns := &corev1.Namespace{}
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKey{Name: "foo-annotation"}, ns)).To(Succeed())
			Expect(componentutils.PatchAnnotation(env.Ctx, env.Client(testutils.CrateCluster), ns, "foo.bar.baz/foo", "", componentutils.ANNOTATION_DELETE)).To(Succeed())
			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
			Expect(ns.GetAnnotations()).NotTo(HaveKey("foo.bar.baz/foo"))
		})

	})

})
