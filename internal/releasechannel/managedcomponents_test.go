package releasechannel

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openmcp-project/controller-utils/pkg/testing"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

func testEnvSetup(crateObjectsPath, coObjectsPath string, crDynamicObjects ...client.Object) *testing.ComplexEnvironment {
	builder := testutils.DefaultTestSetupBuilder(crateObjectsPath).WithFakeClient(testutils.COCoreCluster, testutils.Scheme)
	if coObjectsPath != "" {
		builder.WithInitObjectPath(testutils.COCoreCluster, coObjectsPath)
	}
	if len(crDynamicObjects) > 0 {
		builder.WithDynamicObjectsWithStatus(testutils.CrateCluster, crDynamicObjects...)
	}
	return builder.Build()
}

var _ = Describe("CO-1153 ReleasechannelRunnable", func() {
	It("Should create managedcomponents", func() {
		env := testEnvSetup("", "testdata/core")

		var crateClient client.Client
		var coreClient client.Client
		for key, client := range env.Clusters {
			if key == testutils.COCoreCluster {
				coreClient = client
			}
			if key == testutils.CrateCluster {
				crateClient = client
			}
		}

		Expect(coreClient).ToNot(BeNil())
		Expect(crateClient).ToNot(BeNil())

		runnable := NewReleasechannelRunnable(crateClient, coreClient)
		err := runnable.loop(env.Ctx)
		if err != nil {
			Fail(err.Error())
		}

		managedComponents := v1alpha1.ManagedComponentList{}
		err = crateClient.List(env.Ctx, &managedComponents)
		if err != nil {
			Fail(err.Error())
		}

		Expect(len(managedComponents.Items)).To(Equal(23))

	})
	It("Should update managedcomponents", func() {
		managedComponent := v1alpha1.ManagedComponent{
			ObjectMeta: v1.ObjectMeta{Name: "crossplane"},
			Status: v1alpha1.ManagedComponentStatus{
				Versions: []string{
					"0.14.0",
				},
			},
		}
		env := testEnvSetup("testdata/crate", "testdata/core", &managedComponent)

		var crateClient client.Client
		var coreClient client.Client
		for key, client := range env.Clusters {
			if key == testutils.COCoreCluster {
				coreClient = client
			}
			if key == testutils.CrateCluster {
				crateClient = client
			}
		}

		//err := crateClient.Create(env.Ctx, &managedComponent)
		//if err != nil {
		//	Fail(err.Error())
		//}

		Expect(coreClient).ToNot(BeNil())
		Expect(crateClient).ToNot(BeNil())

		managedComponents := v1alpha1.ManagedComponentList{}
		err := crateClient.List(env.Ctx, &managedComponents)
		if err != nil {
			Fail(err.Error())
		}

		Expect(len(managedComponents.Items)).To(Equal(1))

		runnable := NewReleasechannelRunnable(crateClient, coreClient)
		err = runnable.loop(env.Ctx)
		if err != nil {
			Fail(err.Error())
		}

		managedCrossplaneComponent := v1alpha1.ManagedComponent{}
		err = crateClient.Get(env.Ctx, client.ObjectKey{Name: "crossplane"}, &managedCrossplaneComponent)
		if err != nil {
			Fail(err.Error())
		}

		Expect(len(managedCrossplaneComponent.Status.Versions), 1)
	})
	It("Should delete managedcomponents", func() {
		env := testEnvSetup("testdata/crate", "")

		var crateClient client.Client
		var coreClient client.Client
		for key, client := range env.Clusters {
			if key == testutils.COCoreCluster {
				coreClient = client
			}
			if key == testutils.CrateCluster {
				crateClient = client
			}
		}

		Expect(coreClient).ToNot(BeNil())
		Expect(crateClient).ToNot(BeNil())

		managedComponents := v1alpha1.ManagedComponentList{}
		err := crateClient.List(env.Ctx, &managedComponents)
		if err != nil {
			Fail(err.Error())
		}

		Expect(len(managedComponents.Items)).To(Equal(1))

		runnable := NewReleasechannelRunnable(crateClient, coreClient)
		err = runnable.loop(env.Ctx)
		if err != nil {
			Fail(err.Error())
		}

		managedComponents = v1alpha1.ManagedComponentList{}
		err = crateClient.List(env.Ctx, &managedComponents)
		if err != nil {
			Fail(err.Error())
		}

		Expect(len(managedComponents.Items)).To(Equal(0))
	})
})
