package apiserver

import (
	"fmt"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

// APIServerAccess provides access to an APIServer's admin kubeconfig.
type APIServerAccess interface {
	// GetAdminAccessClient returns the admin access kubeconfig for the given APIServer.
	GetAdminAccessClient(as *openmcpv1alpha1.APIServer, options client.Options) (client.Client, error)
	// GetAdminAccessConfig returns the admin access kubeconfig for the given APIServer.
	GetAdminAccessConfig(as *openmcpv1alpha1.APIServer) (*restclient.Config, error)
	// GetAdminAccessRaw returns the admin access kubeconfig for the given APIServer.
	GetAdminAccessRaw(as *openmcpv1alpha1.APIServer) (string, error)
}

// APIServerAccessImpl is the default implementation of APIServerAccess.
type APIServerAccessImpl struct {
	NewClient client.NewClientFunc
}

// GetAdminAccessClient implements APIServerAccess.GetAdminAccessClient.
func (a *APIServerAccessImpl) GetAdminAccessClient(apiServer *openmcpv1alpha1.APIServer, options client.Options) (client.Client, error) {
	if a.NewClient == nil {
		return nil, fmt.Errorf("NewClient function not set")
	}

	config, err := a.GetAdminAccessConfig(apiServer)
	if err != nil {
		return nil, err
	}

	apiServerClient, err := a.NewClient(config, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return apiServerClient, nil
}

// GetAdminAccessConfig implements APIServerAccess.GetAdminAccessConfig.
func (a *APIServerAccessImpl) GetAdminAccessConfig(apiServer *openmcpv1alpha1.APIServer) (*restclient.Config, error) {
	if apiServer.Status.AdminAccess == nil || apiServer.Status.AdminAccess.Kubeconfig == "" {
		return nil, fmt.Errorf("admin access kubeconfig not available")
	}

	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(apiServer.Status.AdminAccess.Kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config from kubeconfig: %w", err)
	}

	return config, nil
}

// GetAdminAccessRaw implements APIServerAccess.GetAdminAccessRaw.
func (a *APIServerAccessImpl) GetAdminAccessRaw(apiServer *openmcpv1alpha1.APIServer) (string, error) {
	if apiServer.Status.AdminAccess == nil || apiServer.Status.AdminAccess.Kubeconfig == "" {
		return "", fmt.Errorf("admin access kubeconfig not available")
	}

	return apiServer.Status.AdminAccess.Kubeconfig, nil
}
