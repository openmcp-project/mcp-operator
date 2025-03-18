package apiserver_test

import (
	"context"
	"sync"
	"time"

	"github.com/openmcp-project/mcp-operator/internal/utils/apiserver"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	"github.com/openmcp-project/mcp-operator/test/utils"
)

var _ = Describe("APIServerWorker", func() {
	var (
		worker apiserver.Worker
		env    *testing.ComplexEnvironment
	)

	const (
		timeout = time.Millisecond * 500
	)

	BeforeEach(func() {
		var err error
		env = utils.DefaultTestSetupBuilder("testdata", "worker").WithFakeClient(utils.APIServerCluster, utils.Scheme).Build()
		opts := apiserver.Options{
			MaxWorkers: ptr.To(2),
			Interval:   ptr.To(time.Millisecond * 10),
			NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
				return env.Client(utils.APIServerCluster), nil
			},
		}
		worker, err = apiserver.NewWorker(env.Client(utils.CrateCluster), &opts)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should execute each task for each APIServer and stop execution", func() {
		ctx, cancel := context.WithCancel(env.Ctx)

		var (
			task1  = sync.Map{}
			task2  = sync.Map{}
			onExit = make(chan bool)
		)

		worker.RegisterTask("task1", func(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client, apiServerClient client.Client) error {
			err := apiServerClient.Get(ctx, client.ObjectKey{Name: "test", Namespace: "test"}, &v1.Secret{})
			Expect(err).ToNot(HaveOccurred())
			task1.Store(as.Name, true)
			return nil
		})

		worker.RegisterTask("task2", func(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client, apiServerClient client.Client) error {
			err := apiServerClient.Get(ctx, client.ObjectKey{Name: "test", Namespace: "test"}, &v1.Secret{})
			Expect(err).ToNot(HaveOccurred())
			task2.Store(as.Name, true)
			return nil
		})

		err := worker.Start(ctx, onExit, nil, nil)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			_, t1Ok1 := task1.Load("test1")
			_, t1Ok2 := task1.Load("test2")
			_, t2Ok1 := task2.Load("test1")
			_, t2Ok2 := task2.Load("test2")
			return t1Ok1 && t1Ok2 && t2Ok1 && t2Ok2
		}).WithTimeout(timeout).WithContext(ctx).Should(BeTrue())

		cancel()
		Eventually(onExit).WithTimeout(timeout).Should(Receive(ptr.To(true)))
	})

	It("should remove tasks", func() {
		ctx, cancel := context.WithCancel(env.Ctx)

		var (
			task1          = sync.Map{}
			task2          = sync.Map{}
			onNextInterval = make(chan bool)
		)

		worker.RegisterTask("task1", func(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client, apiServerClient client.Client) error {
			task1.Store(as.Name, true)
			return nil
		})

		worker.RegisterTask("task2", func(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client, apiServerClient client.Client) error {
			task2.Store(as.Name, true)
			return nil
		})

		err := worker.Start(ctx, nil, onNextInterval, nil)
		Expect(err).ToNot(HaveOccurred())

		worker.UnregisterTask("task1")
		Eventually(onNextInterval).WithTimeout(timeout).Should(Receive(ptr.To(true)))
		task1 = sync.Map{}
		Eventually(onNextInterval).WithTimeout(timeout).Should(Receive(ptr.To(true)))
		_, ok := task1.Load("test1")
		Expect(ok).To(BeFalse())

		cancel()
	})

	It("should not replace tasks", func() {
		ctx, cancel := context.WithCancel(env.Ctx)

		var (
			task1          = sync.Map{}
			task2          = sync.Map{}
			onNextInterval = make(chan bool)
		)

		worker.RegisterTask("task1", func(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client, apiServerClient client.Client) error {
			task1.Store(as.Name, true)
			return nil
		})

		worker.RegisterTask("task2", func(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client, apiServerClient client.Client) error {
			task2.Store(as.Name, true)
			return nil
		})

		err := worker.Start(ctx, nil, onNextInterval, nil)
		Expect(err).ToNot(HaveOccurred())

		worker.RegisterTask("task1", func(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client, apiServerClient client.Client) error {
			return nil
		})
		Eventually(onNextInterval).WithTimeout(timeout).Should(Receive(ptr.To(true)))
		task1 = sync.Map{}
		Eventually(onNextInterval).WithTimeout(timeout).Should(Receive(ptr.To(true)))
		_, ok := task1.Load("test1")
		Expect(ok).To(BeTrue())

		cancel()
	})
})
