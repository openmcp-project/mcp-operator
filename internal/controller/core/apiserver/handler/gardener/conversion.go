package gardener

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.tools.sap/CoLa/controller-utils/pkg/collections/maps"
	"github.tools.sap/CoLa/controller-utils/pkg/logging"
	authenticationv1 "k8s.io/api/authentication/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	authenticationv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/authentication/v1alpha1"
	gardenv1beta1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
	gardenconstants "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1/constants"
	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/config"
	"github.tools.sap/CoLa/mcp-operator/internal/utils"
	"github.tools.sap/CoLa/mcp-operator/internal/utils/region"
)

const (
	auditlogExtensionServiceName = "shoot-auditlog-service"
	auditlogCredentialName       = "auditlog-credentials"
)

// Shoot_v1beta1_from_APIServer_v1alpha1 updates a v1beta1.Shoot based on a v1alpha1.APIServer.
// Since most fields are immutable, the values of an existing shoot will be preserved in the most cases.
// For the k8s version, there is a special logic in place which prevents downgrades and allows upgrades.
func (gc *GardenerConnector) Shoot_v1beta1_from_APIServer_v1alpha1(ctx context.Context, as *openmcpv1alpha1.APIServer, sh *gardenv1beta1.Shoot) error {
	log := logging.FromContextOrPanic(ctx).WithName("ShootConversion")

	lc := ""
	if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil {
		lc = as.Spec.Internal.GardenerConfig.LandscapeConfiguration
	}
	_, gcfg, err := gc.LandscapeConfiguration(lc)
	if err != nil {
		return fmt.Errorf("error resolving landscape and configuration: %w", err)
	}

	sh.SetName(gc.GetShootName(sh, as, gcfg))

	if sh.Namespace == "" {
		if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil && as.Spec.Internal.GardenerConfig.ShootOverwrite != nil {
			sh.SetNamespace(as.Spec.Internal.GardenerConfig.ShootOverwrite.Namespace)
		} else {
			sh.SetNamespace(gcfg.ProjectNamespace)
		}
	}
	log = log.WithValues("shoot", client.ObjectKeyFromObject(sh).String())

	enforcedAnnotations := maps.Merge(gcfg.ShootTemplate.Annotations, map[string]string{
		"shoot.gardener.cloud/cleanup-extended-apis-finalize-grace-period-seconds": "30",
		gardenconstants.AnnotationAuthenticationIssuer:                             gardenconstants.AnnotationAuthenticationIssuerManaged,
	})
	existingAnnotations := sh.GetAnnotations()
	if existingAnnotations == nil {
		sh.SetAnnotations(enforcedAnnotations)
	} else {
		for k, v := range enforcedAnnotations {
			val, exists := existingAnnotations[k]
			if !exists || val != v {
				sh.SetAnnotations(maps.Merge(existingAnnotations, enforcedAnnotations))
				break
			}
		}
	}

	enforcedLabels := maps.Merge(gcfg.ShootTemplate.Labels, map[string]string{
		openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName:      as.Name,
		openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace: as.Namespace,
	})
	if project, ok := as.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject]; ok {
		enforcedLabels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject] = project
	}
	if workspace, ok := as.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace]; ok {
		enforcedLabels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace] = workspace
	}
	existingLabels := sh.GetLabels()
	if existingLabels == nil {
		sh.SetLabels(enforcedLabels)
	} else {
		for k, v := range enforcedLabels {
			val, exists := existingLabels[k]
			if !exists || val != v {
				sh.SetLabels(maps.Merge(existingLabels, enforcedLabels))
				break
			}
		}
	}

	if sh.Spec.Purpose == nil {
		log.Debug("Setting shoot.Spec.Purpose", "value", string(gardenv1beta1.ShootPurposeProduction))
		sh.Spec.Purpose = ptr.To(gardenv1beta1.ShootPurposeProduction)
	}
	if sh.Spec.CloudProfile == nil {
		log.Debug("Setting shoot.Spec.CloudProfile", "value_kind", gardenconstants.CloudProfileReferenceKindCloudProfile, "value_name", gcfg.CloudProfile)
		sh.Spec.CloudProfile = &gardenv1beta1.CloudProfileReference{
			Kind: gardenconstants.CloudProfileReferenceKindCloudProfile,
			Name: gcfg.CloudProfile,
		}
	}
	if sh.Spec.Provider.Type == "" {
		log.Debug("Setting shoot.Spec.Provider.Type", "value", gcfg.ProviderType)
		sh.Spec.Provider.Type = gcfg.ProviderType
	}
	if sh.Spec.Hibernation == nil {
		sh.Spec.Hibernation = &gardenv1beta1.Hibernation{}
	}
	if sh.Spec.Hibernation.Enabled == nil || *sh.Spec.Hibernation.Enabled {
		log.Debug("Setting shoot.Spec.Hibernation.Enabled", "value", false)
		sh.Spec.Hibernation.Enabled = ptr.To(false)
	}
	h := HashAsNumber(as.Name, as.Namespace)
	if sh.Spec.Region == "" {
		log.Debug("Setting shoot.Spec.Region")
		if as.Spec.GardenerConfig != nil && as.Spec.GardenerConfig.Region != "" {
			sh.Spec.Region = as.Spec.GardenerConfig.Region
			log.Debug("Using shoot region specified in APIServer's Gardener config", "region", sh.Spec.Region)
		} else {
			// try to derive region from the common configuration
			if as.Spec.DesiredRegion != nil && as.Spec.DesiredRegion.Name != "" {
				dr := as.Spec.DesiredRegion.DeepCopy()
				if dr.Direction == "" {
					dr.Direction = openmcpv1alpha1.CENTRAL
				}
				mapper := region.GetPredefinedMapperByCloudprovider(gcfg.ProviderType)
				if mapper != nil {
					regions, err := region.GetClosestRegions(*dr, mapper, sets.KeySet(gcfg.ValidRegions).UnsortedList(), true)
					if err != nil {
						// log, but don't break
						log.Error(err, "error finding closest regions", "region", dr.Name, "direction", dr.Direction)
					} else if len(regions) > 0 {
						if len(regions) == 1 {
							sh.Spec.Region = regions[0]
						} else {
							sh.Spec.Region = regions[h%len(regions)]
						}
						log.Debug("Resolved shoot region from APIServer's desiredRegion field", "region", sh.Spec.Region)
					}
				}
			}
			// fallback to specified default region
			if sh.Spec.Region == "" {
				sh.Spec.Region = gcfg.DefaultRegion
				log.Debug("Neither desired region nor explicit Gardener region specified in APIServer, using fallback from global configuration", "region", sh.Spec.Region)
			}
		}
	}
	configuredVersion := ""
	if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil && as.Spec.Internal.GardenerConfig.K8SVersionOverwrite != "" {
		log.Debug("Found internal k8s version overwrite", "version", as.Spec.Internal.GardenerConfig.K8SVersionOverwrite)
		configuredVersion = as.Spec.Internal.GardenerConfig.K8SVersionOverwrite
	}
	newK8sVersion := computeK8sVersion(configuredVersion, sh.Spec.Kubernetes.Version)
	if sh.Spec.Kubernetes.Version != newK8sVersion {
		log.Debug("Setting shoot.Spec.Kubernetes.Version", "value", newK8sVersion)
		sh.Spec.Kubernetes.Version = newK8sVersion
	}
	if sh.Spec.Kubernetes.KubeAPIServer == nil {
		sh.Spec.Kubernetes.KubeAPIServer = &gardenv1beta1.KubeAPIServerConfig{}
	}
	if sh.Spec.Kubernetes.KubeAPIServer.RuntimeConfig == nil {
		sh.Spec.Kubernetes.KubeAPIServer.RuntimeConfig = map[string]bool{}
	}
	if val, ok := sh.Spec.Kubernetes.KubeAPIServer.RuntimeConfig["apps/v1"]; !ok || !val {
		log.Debug("Setting shoot.Spec.Kubernetes.KubeAPIServer.RuntimeConfig[apps/v1]", "value", true)
		sh.Spec.Kubernetes.KubeAPIServer.RuntimeConfig["apps/v1"] = true
	}
	if val, ok := sh.Spec.Kubernetes.KubeAPIServer.RuntimeConfig["batch/v1"]; !ok || !val {
		log.Debug("Setting shoot.Spec.Kubernetes.KubeAPIServer.RuntimeConfig[batch/v1]", "value", true)
		sh.Spec.Kubernetes.KubeAPIServer.RuntimeConfig["batch/v1"] = true
	}

	addOIDCExtension(log, sh)

	// add all audit log configurations
	if as.Spec.GardenerConfig != nil && as.Spec.GardenerConfig.AuditLog != nil {
		log.Debug("Adding audit log configuration to shoot")
		sh.Spec.Kubernetes.KubeAPIServer.AuditConfig = &gardenv1beta1.AuditConfig{
			AuditPolicy: &gardenv1beta1.AuditPolicy{
				ConfigMapRef: &corev1.ObjectReference{
					Name: utils.PrefixWithNamespace(sh.Name, "auditlog-policy"),
				},
			},
		}

		err := addAuditLogServiceExtension(sh, as)
		if err != nil {
			return err
		}
		addAuditLogCredentialResource(sh)
	}

	// remove all audit log configurations when nothing is configured in the ManagedControlPlane
	if as.Spec.GardenerConfig == nil || as.Spec.GardenerConfig.AuditLog == nil {
		sh.Spec.Kubernetes.KubeAPIServer.AuditConfig = nil
		removeAuditLogServiceExtension(sh)
		removeAuditLogCredentialResource(sh)
	}

	if gc.APIServerType == openmcpv1alpha1.GardenerDedicated {
		log.Debug("APIServer type is GardenerDedicated, configuring shoot workers")
		region := gcfg.ValidRegions[sh.Spec.Region]
		var highAvailabilityConfig *openmcpv1alpha1.HighAvailabilityConfig
		if as.Spec.GardenerConfig != nil {
			highAvailabilityConfig = as.Spec.GardenerConfig.HighAvailabilityConfig
		}
		builder, err := getShootBuilderByCloudProvider(log, sh, h, gcfg.ProviderType, gcfg.ShootTemplate, &region, highAvailabilityConfig)
		if err != nil {
			return fmt.Errorf("error constructing shoot builder: %w", err)
		}

		if sh.Spec.Networking == nil {
			sh.Spec.Networking = &gardenv1beta1.Networking{}
		}
		if sh.Spec.Networking.Type == nil {
			log.Debug("Setting shoot.Spec.Networking.Type", "value", gcfg.ShootTemplate.Spec.Networking.Type)
			sh.Spec.Networking.Type = gcfg.ShootTemplate.Spec.Networking.Type
		}
		if sh.Spec.Networking.Nodes == nil {
			log.Debug("Setting shoot.Spec.Networking.Nodes", "value", gcfg.ShootTemplate.Spec.Networking.Nodes)
			sh.Spec.Networking.Nodes = gcfg.ShootTemplate.Spec.Networking.Nodes
		}

		if sh.Spec.Provider.InfrastructureConfig == nil {
			log.Debug("Setting shoot.Spec.Provider.InfrastructureConfig")
			sh.Spec.Provider.InfrastructureConfig, err = builder.newInfrastructureConfig(log)
			if err != nil {
				return err
			}
		}

		if sh.Spec.Provider.ControlPlaneConfig == nil {
			log.Debug("Setting shoot.Spec.Provider.ControlPlaneConfig")
			controlPlaneConfig, err := builder.newControlPlaneConfig(log)
			if err != nil {
				return err
			}
			sh.Spec.Provider.ControlPlaneConfig = controlPlaneConfig
		}

		builder.adjustWorkers(log, &sh.Spec.Provider)

		if sh.Spec.SecretBindingName == nil {
			log.Debug("Setting shoot.Spec.SecretBindingName", "value", gcfg.ShootTemplate.Spec.SecretBindingName)
			sh.Spec.SecretBindingName = gcfg.ShootTemplate.Spec.SecretBindingName
		}
	}

	if as.Spec.GardenerConfig != nil && as.Spec.GardenerConfig.HighAvailabilityConfig != nil {
		if sh.Spec.ControlPlane == nil {
			sh.Spec.ControlPlane = &gardenv1beta1.ControlPlane{}
		}

		log.Debug("Setting shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type", "value", as.Spec.GardenerConfig.HighAvailabilityConfig.FailureToleranceType)
		sh.Spec.ControlPlane.HighAvailability = &gardenv1beta1.HighAvailability{
			FailureTolerance: gardenv1beta1.FailureTolerance{
				Type: gardenv1beta1.FailureToleranceType(as.Spec.GardenerConfig.HighAvailabilityConfig.FailureToleranceType),
			},
		}
	}

	if as.Spec.GardenerConfig != nil {
		log.Debug("Setting shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig")
		if as.Spec.GardenerConfig.EncryptionConfig == nil {
			sh.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = nil
		} else {
			sh.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &gardenv1beta1.EncryptionConfig{
				Resources: as.Spec.GardenerConfig.EncryptionConfig.Resources,
			}
		}
	}

	return nil
}

// addOIDCExtension adds the OIDC extension to the shoot spec if it is not already present.
func addOIDCExtension(log logging.Logger, sh *gardenv1beta1.Shoot) {
	// Update configuration
	for _, extension := range sh.Spec.Extensions {
		if extension.Type == "shoot-oidc-service" {
			return
		}
	}

	// Add configuration
	log.Debug("Adding 'shoot-oidc-service' extension to shoot")
	sh.Spec.Extensions = append(sh.Spec.Extensions, gardenv1beta1.Extension{
		Type: "shoot-oidc-service",
	})

}

// addAuditLogServiceExtension adds the audit log service extension to the shoot spec if it is not already present.
func addAuditLogServiceExtension(sh *gardenv1beta1.Shoot, as *openmcpv1alpha1.APIServer) error {
	m := map[string]string{
		"apiVersion":          "service.auditlog.extensions.gardener.cloud/v1alpha1",
		"kind":                "AuditlogConfig",
		"type":                as.Spec.GardenerConfig.AuditLog.Type,
		"tenantID":            as.Spec.GardenerConfig.AuditLog.TenantID,
		"serviceURL":          as.Spec.GardenerConfig.AuditLog.ServiceURL,
		"secretReferenceName": "auditlog-credentials",
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// Update configuration
	for i := range sh.Spec.Extensions {
		extension := &sh.Spec.Extensions[i]
		if extension.Type == auditlogExtensionServiceName {
			extension.ProviderConfig = &runtime.RawExtension{
				Raw: raw,
			}
			return nil
		}
	}

	// Add configuration
	sh.Spec.Extensions = append(sh.Spec.Extensions, gardenv1beta1.Extension{
		Type:           auditlogExtensionServiceName,
		ProviderConfig: &runtime.RawExtension{Raw: raw},
	})

	return nil
}

// removeAuditLogServiceExtension removes the audit log service extension from the shoot spec if it is present.
func removeAuditLogServiceExtension(sh *gardenv1beta1.Shoot) {
	var extensions []gardenv1beta1.Extension
	for _, extension := range sh.Spec.Extensions {
		if extension.Type != auditlogExtensionServiceName {
			extensions = append(extensions, extension)
		}
	}
	sh.Spec.Extensions = extensions
}

// addAuditLogCredentialResource adds the audit log credential resource to the shoot spec if it is not already present.
func addAuditLogCredentialResource(sh *gardenv1beta1.Shoot) {
	resourceRef := autoscalingv1.CrossVersionObjectReference{
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       utils.PrefixWithNamespace(sh.Name, "auditlog-credentials"),
	}

	// add audit log credentials
	for i := range sh.Spec.Resources {
		resource := &sh.Spec.Resources[i]
		if resource.Name == auditlogCredentialName {
			resource.ResourceRef = resourceRef
			return
		}
	}

	sh.Spec.Resources = append(sh.Spec.Resources, gardenv1beta1.NamedResourceReference{
		Name:        auditlogCredentialName,
		ResourceRef: resourceRef,
	})
}

// removeAuditLogCredentialResource removes the audit log credential resource from the shoot spec if it is present.
func removeAuditLogCredentialResource(sh *gardenv1beta1.Shoot) {
	var resources []gardenv1beta1.NamedResourceReference
	for _, resource := range sh.Spec.Resources {
		if resource.Name != auditlogCredentialName {
			resources = append(resources, resource)
		}
	}
	sh.Spec.Resources = resources
}

// computeK8sVersion computes which k8s version should be rendered into the generated shoot manifest.
// It takes a k8s version which is configured and one which comes from an already existing shoot.
// The logic is as follows:
// - If both are empty, the result is empty.
// - If only one is empty, the result is the non-empty one.
// - If neither is empty, the higher one is returned. To not cause any unplanned shoot updates, a configured version without a patch number is considered to be less than an existing version with a patch number (that is otherwise identical).
func computeK8sVersion(configured, existing string) string {
	if existing == "" && configured != "" {
		return configured
	} else if existing != "" && configured != "" {
		configuredK8sVersion := semver.MustParse(configured)
		existingK8sVersion := semver.MustParse(existing)

		if configuredK8sVersion.GreaterThan(existingK8sVersion) {
			return configuredK8sVersion.Original()
		}

	}

	return existing
}

// getAdminKubeconfigForShoot uses the AdminKubeconfigRequest subresource of a shoot to get a admin kubeconfig for the given shoot.
func getAdminKubeconfigForShoot(ctx context.Context, c client.Client, shoot *gardenv1beta1.Shoot, desiredValidity time.Duration) ([]byte, error) {
	expirationSeconds := int64(desiredValidity.Seconds())
	adminKubeconfigRequest := &authenticationv1alpha1.AdminKubeconfigRequest{
		Spec: authenticationv1alpha1.AdminKubeconfigRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}
	err := c.SubResource("adminkubeconfig").Create(ctx, shoot, adminKubeconfigRequest)
	if err != nil {
		return nil, err
	}
	return adminKubeconfigRequest.Status.Kubeconfig, nil
}

// getTemporaryClientForShoot creates a client.Client for accessing the shoot cluster.
// Also returns the rest.Config used to create the client.
// The token used by the client has a validity of one hour.
func getTemporaryClientForShoot(ctx context.Context, c client.Client, shoot *gardenv1beta1.Shoot) (client.Client, *rest.Config, error) {
	kcfg, err := getAdminKubeconfigForShoot(ctx, c, shoot, time.Hour)
	if err != nil {
		return nil, nil, err
	}
	if bytes.Equal(kcfg, []byte("fake")) {
		// inject fake client for tests
		return fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
			SubResourceCreate: func(ctx context.Context, c client.Client, subResourceName string, obj, subResource client.Object, opts ...client.SubResourceCreateOption) error {
				switch subResourceName {
				case "token":
					tr, ok := subResource.(*authenticationv1.TokenRequest)
					if !ok {
						return fmt.Errorf("unexpected object type %T", subResource)
					}
					tr.Status.Token = "fake"
					tr.Status.ExpirationTimestamp = metav1.Time{Time: time.Now().Add(time.Duration(*tr.Spec.ExpirationSeconds * int64(time.Second)))}
					return nil
				}
				// use default logic
				return c.SubResource(subResourceName).Create(ctx, obj, subResource, opts...)
			},
		}).Build(), &rest.Config{}, nil
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kcfg)
	if err != nil {
		return nil, nil, err
	}
	shootClient, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, nil, err
	}
	return shootClient, cfg, nil
}

// ComputeShootName computes the name of a shoot based on the given APIServer metadata and the project name.
func ComputeShootName(apiServerMeta *metav1.ObjectMeta, project string) string {
	// Gardener enforces a length limit on shoot names which is at max 21 characters for project name + shoot name.
	shootMaxLength := 21 - len(project)
	shootName := utils.ScopeToControlPlane(apiServerMeta)
	return shootName[:shootMaxLength]
}

// HashAsNumber takes any number of strings and returns a hash value as an integer.
// Note that this function is not cyptographically secure.
func HashAsNumber(data ...string) int {
	h := fnv.New32a()
	for _, s := range data {
		h.Write([]byte(s))
	}
	return int(h.Sum32())
}

func (gc *GardenerConnector) GetShootNameNoConfig(sh *gardenv1beta1.Shoot, as *openmcpv1alpha1.APIServer) (string, error) {
	lc := ""
	if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil {
		lc = as.Spec.Internal.GardenerConfig.LandscapeConfiguration
	}
	_, gcfg, err := gc.LandscapeConfiguration(lc)
	if err != nil {
		return "", fmt.Errorf("error resolving landscape and configuration: %w", err)
	}

	return gc.GetShootName(sh, as, gcfg), nil
}

func (gc *GardenerConnector) GetShootName(sh *gardenv1beta1.Shoot, as *openmcpv1alpha1.APIServer, gcfg *config.CompletedGardenerConfiguration) string {
	shootName := ""

	if sh == nil || sh.Name == "" {
		if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil && as.Spec.Internal.GardenerConfig.ShootOverwrite != nil {
			shootName = as.Spec.Internal.GardenerConfig.ShootOverwrite.Name
		} else {
			shootName = ComputeShootName(&as.ObjectMeta, gcfg.Project)
		}
	} else {
		shootName = sh.Name
	}

	return shootName
}
