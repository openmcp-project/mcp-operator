package utils

import (
	"context"
	"errors"

	apiserverutils "github.com/openmcp-project/mcp-operator/internal/utils/apiserver"

	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// TestAPIServerAccess is a test implementation of APIServerAccess.
type TestAPIServerAccess struct {
	APIServerAccess apiserverutils.APIServerAccessImpl
	Client          client.Client
	Error           error
}

// GetAdminAccessClient implements APIServerAccess.GetAdminAccessClient.
func (t *TestAPIServerAccess) GetAdminAccessClient(as *openmcpv1alpha1.APIServer, _ client.Options) (client.Client, error) {
	if t.Error != nil {
		return nil, t.Error
	}
	return t.Client, nil
}

// GetAdminAccessConfig implements APIServerAccess.GetAdminAccessConfig.
func (t *TestAPIServerAccess) GetAdminAccessConfig(as *openmcpv1alpha1.APIServer) (*restclient.Config, error) {
	return t.APIServerAccess.GetAdminAccessConfig(as)
}

// GetAdminAccessRaw implements APIServerAccess.GetAdminAccessRaw.
func (t *TestAPIServerAccess) GetAdminAccessRaw(as *openmcpv1alpha1.APIServer) (string, error) {
	return t.APIServerAccess.GetAdminAccessRaw(as)
}

// TestWorker is a test implementation of Worker.
type TestWorker struct {
	taskList        map[string]apiserverutils.Task
	crateClient     client.Client
	apiServerClient client.Client
}

// NewTestWorker creates a new TestWorker.
func NewTestWorker(crateClient, apiServerClient client.Client) *TestWorker {
	return &TestWorker{
		taskList:        make(map[string]apiserverutils.Task),
		crateClient:     crateClient,
		apiServerClient: apiServerClient,
	}
}

func (w *TestWorker) RegisterTask(name string, task apiserverutils.Task) {
	w.taskList[name] = task
}

func (w *TestWorker) UnregisterTask(name string) {
	delete(w.taskList, name)
}

func (w *TestWorker) Start(_ context.Context, _ apiserverutils.OnExit, _ apiserverutils.OnNextInterval, _ <-chan struct{}) error {
	return nil
}

func (w *TestWorker) RunTasks(ctx context.Context, as *openmcpv1alpha1.APIServer) error {
	var err error
	for _, task := range w.taskList {
		err = errors.Join(err, task(ctx, as, w.crateClient, w.apiServerClient))
	}
	return err
}
