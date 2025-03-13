package apiserver

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cconst "github.tools.sap/CoLa/mcp-operator/api/constants"
	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

// Task is a function that is executed for each APIServer in the cluster
// The first client is for the Crate cluster
// The second client is for the APIServer cluster
type Task func(context.Context, *openmcpv1alpha1.APIServer, client.Client, client.Client) error

// OnExit is a channel that is used to signal when the worker is stopped
type OnExit chan bool

// OnNextInterval is a channel that is used to signal when the next interval is executed
type OnNextInterval chan bool

// Worker has a pool of workers which execute all tasks for each APIServer in the cluster
type Worker interface {
	// RegisterTask registers a new task with the given name
	// The name must be unique
	RegisterTask(name string, task Task)
	// UnregisterTask removes the task with the given name
	UnregisterTask(name string)
	// Start starts the worker
	// This function will not block.
	// The worker will be stopped when the context is canceled
	// The OnExit channel will be used to signal when the worker is stopped (when not nil)
	// The OnNextInterval channel will be used to signal when the next interval is executed (when not nil)
	Start(ctx context.Context, onExit OnExit, onNextInterval OnNextInterval, waitFor <-chan struct{}) error
}

// WorkerImpl is the implementation of the Worker interface
type WorkerImpl struct {
	crateClient client.Client
	interval    time.Duration
	maxWorkers  int
	taskList    sync.Map
	taskListLen atomic.Int32
	scheme      *runtime.Scheme
	NewClient   client.NewClientFunc
}

// Options is used to configure the Worker
type Options struct {
	// MaxWorkers is the maximum number of workers that can be executed concurrently
	MaxWorkers *int
	// Interval is the time between each execution of the tasks
	Interval *time.Duration
	// NewClient is the function to create a new client for the APIServer
	NewClient client.NewClientFunc
}

// SetDefaultsIfNotSet sets the default values for the options if they are not set
func (o *Options) SetDefaultsIfNotSet() {
	if o.MaxWorkers == nil {
		o.MaxWorkers = &maxWorkersDefault
	}

	if o.Interval == nil {
		o.Interval = &intervalDefault
	}

	if o.NewClient == nil {
		o.NewClient = client.New
	}
}

var (
	// maxWorkersDefault is the default value for the maximum number of workers
	maxWorkersDefault = 1
	// intervalDefault is the default value for the interval between each execution of the tasks
	intervalDefault = time.Second * 10
)

// NewWorker creates a new Worker instance
// crateClient is the client for the crate cluster
// options is used to configure the Worker. If nil, the default values will be used.
func NewWorker(crateClient client.Client, options *Options) (Worker, error) {
	sc := runtime.NewScheme()

	if err := clientgoscheme.AddToScheme(sc); err != nil {
		return nil, fmt.Errorf("error adding client-go scheme to runtime scheme: %w", err)
	}

	if options == nil {
		options = &Options{}
	}

	options.SetDefaultsIfNotSet()

	res := &WorkerImpl{
		crateClient: crateClient,
		maxWorkers:  *options.MaxWorkers,
		interval:    *options.Interval,
		taskList:    sync.Map{},
		taskListLen: atomic.Int32{},
		scheme:      sc,
		NewClient:   options.NewClient,
	}

	res.taskListLen.Store(0)
	return res, nil
}

// RegisterTask implements Worker.RegisterTask
// Thread-safe
func (w *WorkerImpl) RegisterTask(name string, task Task) {
	// check if the task is already stored
	if _, ok := w.taskList.Load(name); ok {
		return
	}

	w.taskList.Store(name, task)
	w.taskListLen.Add(1)
}

// UnregisterTask implements Worker.UnregisterTask
// Thread-safe
func (w *WorkerImpl) UnregisterTask(name string) {
	if _, ok := w.taskList.Load(name); ok {
		w.taskList.Delete(name)
		w.taskListLen.Add(-1)
	}
}

// Start implements Worker.Start
func (w *WorkerImpl) Start(ctx context.Context, onExit OnExit, onNextInterval OnNextInterval, waitFor <-chan struct{}) error {
	log := logging.FromContextOrPanic(ctx)
	pool := pond.NewPool(w.maxWorkers, pond.WithContext(ctx))
	group := pool.NewGroup()

	go func() {
		if waitFor != nil {
			// wait for the given channel to be closed
			<-waitFor
		}

		log.Info("Worker started")

		for {
			if w.taskListLen.Load() > 0 {
				// get all existing APIServers from the crate cluster
				log.Debug("Listing APIServers")
				apiServers, err := w.listAPIServers(ctx)
				if err != nil {
					log.Error(err, "error listing APIServers")
				} else {
					for i := range apiServers.Items {
						as := &apiServers.Items[i]
						log := log.WithValues("apiserver", fmt.Sprintf("%s/%s", as.Namespace, as.Name))
						// create the kubernetes client for the APIServer
						apiServerClient, err := w.createAPIServerClient(ctx, as)

						if err != nil {
							log.Error(err, "error creating APIServer client", cconst.KeyResource, as.Name)
							continue
						}

						// execute all tasks for the current APIServer
						w.taskList.Range(func(key, value interface{}) bool {
							task := value.(Task)
							taskName := key.(string)
							log := log.WithValues("task", taskName)
							log.Debug("Executing task")
							ctx := logging.NewContext(ctx, log)
							group.SubmitErr(func() error {
								if err = task(ctx, as, w.crateClient, apiServerClient); err != nil {
									log.Error(err, "error executing task")
								}
								return nil
							})

							return true
						})
					}
				}
			}

			// wait for next interval
			select {
			case <-time.After(w.interval):
			case <-ctx.Done():
				log.Info("APIServer worker stopped")
				if onExit != nil {
					onExit <- true
				}
				return
			}

			if onNextInterval != nil {
				onNextInterval <- true
			}
		}
	}()

	return nil
}

// listAPIServers lists all APIServers in the crate cluster
func (w *WorkerImpl) listAPIServers(ctx context.Context) (*openmcpv1alpha1.APIServerList, error) {
	apiServerList := &openmcpv1alpha1.APIServerList{}
	if err := w.crateClient.List(ctx, apiServerList); err != nil {
		return nil, fmt.Errorf("error listing APIServers: %w", err)
	}

	return apiServerList, nil
}

// createAPIServerClient creates a new client for the given APIServer
func (w *WorkerImpl) createAPIServerClient(_ context.Context, as *openmcpv1alpha1.APIServer) (client.Client, error) {
	if as.Status.AdminAccess == nil {
		return nil, fmt.Errorf("no admin access found in APIServer status")
	}

	// create a new client for the APIServer kubeconfig string
	apiServerConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(as.Status.AdminAccess.Kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("error creating REST config from kubeconfig: %w", err)
	}

	return w.NewClient(apiServerConfig, client.Options{
		Scheme: w.scheme,
	})
}
