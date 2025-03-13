package utils

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	colactrlutil "github.com/openmcp-project/controller-utils/pkg/controller"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

const DefaultAdminAccessValidityTime = 180 * 24 * time.Hour

// GetAdminAccess creates a ServiceAccount (if it does not exist), binds it to the cluster-admin role and returns a kubeconfig for it.
func GetAdminAccess(ctx context.Context, c client.Client, cfg *rest.Config, saName, saNamespace string) (*openmcpv1alpha1.APIServerAccess, error) {
	sa, err := EnsureServiceAccount(ctx, c, saName, saNamespace)
	if err != nil {
		return nil, err
	}

	_, err = BindToClusterRole(ctx, c, "cluster-admin", rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      sa.Name,
		Namespace: sa.Namespace,
	})
	if err != nil {
		return nil, err
	}

	tr, err := CreateTokenForServiceAccount(ctx, c, sa, ptr.To(DefaultAdminAccessValidityTime))
	if err != nil {
		return nil, err
	}

	kcfgBytes, err := CreateTokenKubeconfig(saName, cfg.Host, cfg.CAData, tr.Status.Token)
	if err != nil {
		return nil, err
	}

	return &openmcpv1alpha1.APIServerAccess{
		Kubeconfig:          string(kcfgBytes),
		CreationTimestamp:   ptr.To(metav1.Now()),
		ExpirationTimestamp: &tr.Status.ExpirationTimestamp,
	}, nil
}

// BindToClusterRole creates/updates a ClusterRoleBinding that binds the given subject to the given ClusterRole.
// It returns the created/updated ClusterRoleBinding.
func BindToClusterRole(ctx context.Context, c client.Client, clusterRoleName string, subject rbacv1.Subject) (*rbacv1.ClusterRoleBinding, error) {
	crb := &rbacv1.ClusterRoleBinding{}
	crb.SetName(fmt.Sprintf("%s--%s", subject.Name, clusterRoleName))
	if err := FailIfNotManaged(ctx, c, crb); err != nil {
		return nil, fmt.Errorf("error updating ClusterRoleBinding '%s': %w", crb.Name, err)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c, crb, func() error {
		// set managed-by label
		labels := crb.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[openmcpv1alpha1.ManagedByAPIServerLabel] = "true"
		crb.SetLabels(labels)

		// set role ref
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		}

		// set subject
		if crb.Subjects == nil {
			crb.Subjects = []rbacv1.Subject{}
		}

		for _, sub := range crb.Subjects {
			if reflect.DeepEqual(sub, subject) {
				// ClusterRoleBinding is up-to-date, nothing to do
				return nil
			}
		}
		crb.Subjects = append(crb.Subjects, subject)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error creating/updating ClusterRoleBinding '%s': %w", crb.Name, err)
	}

	return crb, nil
}

// EnsureServiceAccount creates a ServiceAccount, if required.
// It returns the ServiceAccount.
func EnsureServiceAccount(ctx context.Context, c client.Client, saName, saNamespace string) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	sa.SetName(saName)
	sa.SetNamespace(saNamespace)
	if err := FailIfNotManaged(ctx, c, sa); err != nil {
		return nil, fmt.Errorf("error updating ServiceAccount '%s/%s': %w", sa.Namespace, sa.Name, err)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c, sa, func() error {
		// set managed-by label
		labels := sa.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[openmcpv1alpha1.ManagedByAPIServerLabel] = "true"
		sa.SetLabels(labels)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error creating/updating ServiceAccount '%s/%s': %w", sa.Namespace, sa.Name, err)
	}

	return sa, nil
}

// EnsureNamespace creates a Namespace, if required.
// It returns the Namespace.
func EnsureNamespace(ctx context.Context, c client.Client, nsName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	ns.SetName(nsName)
	if err := FailIfNotManaged(ctx, c, ns); err != nil {
		return nil, fmt.Errorf("error updating Namespace '%s': %w", ns.Name, err)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c, ns, func() error {
		// set managed-by label
		labels := ns.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[openmcpv1alpha1.ManagedByAPIServerLabel] = "true"
		ns.SetLabels(labels)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error creating/updating Namespace '%s': %w", ns.Name, err)
	}

	return ns, nil
}

// EnsureUserClusterRole creates/updates a ClusterRole that has permissions for namespaces, secrets, and configmaps.
func EnsureUserClusterRole(ctx context.Context, c client.Client, crName string) (*rbacv1.ClusterRole, error) {
	cr := &rbacv1.ClusterRole{}
	cr.SetName(crName)
	if err := FailIfNotManaged(ctx, c, cr); err != nil {
		return nil, fmt.Errorf("error updating ClusterRole '%s': %w", cr.Name, err)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c, cr, func() error {
		// set managed-by label
		labels := cr.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[openmcpv1alpha1.ManagedByAPIServerLabel] = "true"
		cr.SetLabels(labels)

		cr.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"namespaces",
					"secrets",
					"configmaps",
				},
				Verbs: []string{rbacv1.VerbAll},
			},
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error creating/updating ClusterRole '%s': %w", cr.Name, err)
	}

	return cr, nil
}

// CreateTokenForServiceAccount generates a token for the given ServiceAccount.
// Returns a TokenRequest object whose status contains the token and its expiration timestamp.
func CreateTokenForServiceAccount(ctx context.Context, c client.Client, sa *corev1.ServiceAccount, desiredDuration *time.Duration) (*authenticationv1.TokenRequest, error) {
	tr := &authenticationv1.TokenRequest{}
	if desiredDuration != nil {
		tr.Spec.ExpirationSeconds = ptr.To((int64)(desiredDuration.Seconds()))
	}

	if err := c.SubResource("token").Create(ctx, sa, tr); err != nil {
		return nil, fmt.Errorf("error creating token for ServiceAccount '%s/%s': %w", sa.Namespace, sa.Name, err)
	}

	return tr, nil
}

// FailIfNotManaged fetches the given object from the cluster and returns an error if it does not contain the managed-by label set to 'true'.
// Also returns an error if fetching the object doesn't work, unless the reason is that it doesn't exist, then nil is returned.
func FailIfNotManaged(ctx context.Context, c client.Client, obj client.Object) error {
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if !colactrlutil.HasLabelWithValue(obj, openmcpv1alpha1.ManagedByAPIServerLabel, "true") {
		return fmt.Errorf("resource already exists, but does not have label '%s' with value 'true'", openmcpv1alpha1.ManagedByAPIServerLabel)
	}
	return nil
}

// PatchManagedByLabel adds the managed-by label to the given resource via a patch.
func PatchManagedByLabel(ctx context.Context, c client.Client, obj client.Object) error {
	return c.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"metadata":{"labels":{"%s":"true"}}}`, openmcpv1alpha1.ManagedByAPIServerLabel))))
}

// CreateTokenKubeconfig generates a kubeconfig based on the given values.
// The 'user' arg is used as key for the auth configuration and can be chosen freely.
func CreateTokenKubeconfig(user, host string, caData []byte, token string) ([]byte, error) {
	id := "cluster"
	kcfg := clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			id: {
				Server:                   host,
				CertificateAuthorityData: caData,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			id: {
				Cluster:  id,
				AuthInfo: user,
			},
		},
		CurrentContext: id,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			user: {
				Token: token,
			},
		},
	}

	kcfgBytes, err := clientcmd.Write(kcfg)
	if err != nil {
		return nil, fmt.Errorf("error converting converting generated kubeconfig into yaml: %w", err)
	}
	return kcfgBytes, nil
}

// CreateOIDCKubeconfig generates a kubeconfig for a cluster that uses OIDC for authentication.
// For each identity provider, a user is created that uses the 'oidc-login' plugin to get a token.
// The cluster name is prefixed with 'mcp-<namespace>-' and the context name is clusterName--idpName.
func CreateOIDCKubeconfig(ctx context.Context, crateClient client.Client, clusterName, namespace, host, defaultIdp string, caData []byte, identityProviders []openmcpv1alpha1.IdentityProvider) ([]byte, error) {
	contextName := func(clusterName, idpName string) string {
		return clusterName + "--" + idpName
	}
	createParameter := func(key, value string) string {
		if value == "" {
			return "--" + key
		}
		return "--" + key + "=" + value
	}

	clusterName = "mcp-" + namespace + "-" + clusterName

	users := make(map[string]*clientcmdapi.AuthInfo)
	contexts := make(map[string]*clientcmdapi.Context)

	flags := map[string]openmcpv1alpha1.SingleOrMultiStringValue{
		openmcpv1alpha1.OIDCParameterUsePKCE:    {},
		openmcpv1alpha1.OIDCParameterGrantType:  {Value: openmcpv1alpha1.OIDCDefaultGrantType},
		openmcpv1alpha1.OIDCParameterExtraScope: {Values: strings.Split(openmcpv1alpha1.OIDCDefaultExtraScopes, ",")},
	}

	for _, idp := range identityProviders {
		if idp.ClientConfig.ExtraConfig != nil {
			for key, value := range idp.ClientConfig.ExtraConfig {
				flags[key] = value
			}
		}

		user := &clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{
				APIVersion:         "client.authentication.k8s.io/v1beta1",
				Command:            "kubectl",
				Env:                nil,
				ProvideClusterInfo: false,
				Args: []string{
					"oidc-login",
					"get-token",
					createParameter(openmcpv1alpha1.OIDCParameterIssuerURL, idp.IssuerURL),
					createParameter(openmcpv1alpha1.OIDCParameterClientID, idp.ClientID),
				},
			},
		}

		for key, value := range flags {
			if len(value.Values) > 0 {
				// repeatable parameter
				for _, v := range value.Values {
					user.Exec.Args = append(user.Exec.Args, createParameter(key, v))
				}
				continue
			}
			if len(value.Value) > 0 {
				// single parameter
				user.Exec.Args = append(user.Exec.Args, createParameter(key, value.Value))

				continue
			}
			// flag without value
			user.Exec.Args = append(user.Exec.Args, createParameter(key, ""))
		}

		if idp.ClientConfig.ClientSecret != nil {
			ref := idp.ClientConfig.ClientSecret
			secret := &corev1.Secret{}
			err := crateClient.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret)
			if err != nil {
				return nil, fmt.Errorf("error getting idp clientSecret secret '%s': %w", ref.Name, err)
			}

			clientSecret, ok := secret.Data[ref.Key]
			if !ok {
				return nil, fmt.Errorf("clientSecret key '%s' not found in secret '%s'", ref.Key, ref.Name)
			}
			user.Exec.Args = append(user.Exec.Args, createParameter("oidc-client-secret", string(clientSecret)))
		}

		context := &clientcmdapi.Context{
			Cluster:   clusterName,
			AuthInfo:  idp.Name,
			Namespace: "default",
		}

		users[idp.Name] = user
		contexts[contextName(clusterName, idp.Name)] = context
	}

	kcfg := clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   host,
				CertificateAuthorityData: caData,
			},
		},
		Contexts:       contexts,
		CurrentContext: contextName(clusterName, defaultIdp),
		AuthInfos:      users,
	}

	kcfgBytes, err := clientcmd.Write(kcfg)
	if err != nil {
		return nil, fmt.Errorf("error converting converting generated kubeconfig into yaml: %w", err)
	}
	return kcfgBytes, nil
}
