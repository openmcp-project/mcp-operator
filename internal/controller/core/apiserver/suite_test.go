package apiserver_test

import (
	"context"
	"path"
	"testing"

	apiserverconfig "github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/config"
	apiserverhandler "github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/handler"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.tools.sap/CoLa/controller-utils/pkg/collections"
	"github.tools.sap/CoLa/controller-utils/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openmcptesting "github.tools.sap/CoLa/controller-utils/pkg/testing"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	"github.tools.sap/CoLa/mcp-operator/api/errors"
	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIServer Controller Test Suite")
}

var defaultConfig *apiserverconfig.APIServerProviderConfiguration
var completedDefaultConfig *apiserverconfig.CompletedAPIServerProviderConfiguration
var constructorContext context.Context
var testObjs []client.Object
var fakeHandler *FakeHandler

var _ = BeforeSuite(func() {
	var err error

	// create a context with logger
	log, err := logging.GetLogger()
	Expect(err).NotTo(HaveOccurred())
	constructorContext = logging.NewContext(context.Background(), log)

	// load test objects
	testObjs, err = openmcptesting.LoadObjects(path.Join("testdata", "garden_cluster"), testutils.Scheme)
	Expect(err).ToNot(HaveOccurred())

	// generate default config
	defaultConfig, err = apiserverconfig.LoadConfig(path.Join("testdata", "default_config.yaml"))
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	// complete the config
	var err error
	completedDefaultConfig, err = defaultConfig.Complete(constructorContext)
	Expect(err).NotTo(HaveOccurred())
	fakeHandler = NewFakeHandler()
})

var _ = AfterEach(func() {
	cou, d := fakeHandler.ExpectedCalls()
	Expect(cou).To(BeZero(), "expected %d more calls to HandleCreateOrUpdate", cou)
	Expect(d).To(BeZero(), "expected %d more calls to HandleDelete", d)
})

// FakeHandler is a fake implementation of the APIServerHandler interface.
type FakeHandler struct {
	MockedHandleCreateOrUpdateCalls collections.Queue[func(context.Context, *openmcpv1alpha1.APIServer, client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError)]
	MockedHandleDeleteCalls         collections.Queue[func(context.Context, *openmcpv1alpha1.APIServer, client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError)]
}

// NewFakeHandler creates a new FakeHandler.
// Use this fake handler's Mock<...>Call methods to instruct it which actions to perform.
func NewFakeHandler() *FakeHandler {
	return &FakeHandler{
		MockedHandleCreateOrUpdateCalls: collections.NewLinkedList[func(context.Context, *openmcpv1alpha1.APIServer, client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError)](),
		MockedHandleDeleteCalls:         collections.NewLinkedList[func(context.Context, *openmcpv1alpha1.APIServer, client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError)](),
	}
}

// MockHandleCreateOrUpdateCall adds a mocked HandleCreateOrUpdate call to the queue.
// Each time this handler's HandleCreateOrUpdate method is called, the next call in the queue will be executed.
func (f *FakeHandler) MockHandleCreateOrUpdateCall(call func(context.Context, *openmcpv1alpha1.APIServer, client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError)) {
	Expect(f.MockedHandleCreateOrUpdateCalls.Push(call)).To(Succeed())
}

// MockHandleDeleteCall adds a mocked HandleDelete call to the queue.
// Each time this handler's HandleDelete method is called, the next call in the queue will be executed.
func (f *FakeHandler) MockHandleDeleteCall(call func(context.Context, *openmcpv1alpha1.APIServer, client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError)) {
	Expect(f.MockedHandleDeleteCalls.Push(call)).To(Succeed())
}

func (f *FakeHandler) ExpectedCalls() (int, int) {
	return f.MockedHandleCreateOrUpdateCalls.Size(), f.MockedHandleDeleteCalls.Size()
}

var _ apiserverhandler.APIServerHandler = &FakeHandler{}

// HandleCreateOrUpdate implements controller.APIServerHandler.
func (f *FakeHandler) HandleCreateOrUpdate(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError) {
	call := f.MockedHandleCreateOrUpdateCalls.Poll()
	if call == nil {
		panic("unexpected call to HandleCreateOrUpdate")
	}
	return call(ctx, as, crateClient)
}

// HandleDelete implements controller.APIServerHandler.
func (f *FakeHandler) HandleDelete(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, errors.ReasonableError) {
	call := f.MockedHandleDeleteCalls.Poll()
	if call == nil {
		panic("unexpected call to HandleDelete")
	}
	return call(ctx, as, crateClient)
}
