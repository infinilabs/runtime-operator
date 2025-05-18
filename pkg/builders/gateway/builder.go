// pkg/builders/gateway/builder.go
// Contains the concrete builder strategy implementation for the Gateway component type.
package gateway

import (
	"context"
	"fmt"

	// K8s Types needed for building objects
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	// policyv1 "k8s.io/api/policy/v1" // Import if PDB is used
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	// Import App types and Common types
	appv1 "github.com/infinilabs/operator/api/app/v1"                // App types
	"github.com/infinilabs/operator/pkg/apis/common"                 // Common types
	commonutil "github.com/infinilabs/operator/pkg/apis/common/util" // Common utils

	// Import other builders needed to construct nested/related objects
	builders "github.com/infinilabs/operator/pkg/builders/k8s" // Import generic K8s builders

	// Import strategy package for the interface definition and registry access
	"github.com/infinilabs/operator/pkg/strategy"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log" // Use controller-runtime logger
)

const (
	StatefulSetType = "StatefulSet"
)

// --- Gateway Builder Strategy Implementation ---

// Ensure our builder implementation complies with the strategy interface
var _ strategy.AppBuilderStrategy = &GatewayBuilderStrategy{}

// GatewayBuilderStrategy is the concrete implementation of AppBuilderStrategy for the "gateway" type.
type GatewayBuilderStrategy struct{}

// Define the expected GVK for Gateway's primary workload (assuming StatefulSet).
var gatewayWorkloadGVK = schema.FromAPIVersionAndKind("apps/v1", StatefulSetType)

// init registers this specific builder strategy with the central registry.
// This function runs automatically when this package is imported.
func init() {
	// Register this builder using the component type name as the key ("gateway").
	// 默认注册operator策略
	strategy.RegisterAppBuilderStrategy("operator", &GatewayBuilderStrategy{})
}

// GetWorkloadGVK implements the AppBuilderStrategy interface.
func (b *GatewayBuilderStrategy) GetWorkloadGVK() schema.GroupVersionKind {
	return gatewayWorkloadGVK
}

func (b *GatewayBuilderStrategy) verifyParameters(gatewayConfig *common.ResourceConfig, appComp *appv1.ApplicationComponent) error {
	if gatewayConfig == nil {
		return fmt.Errorf("gateway configuration (properties) is mandatory but missing or empty for component '%s'", appComp.Name)
	}

	// Perform validation of the specific ResourceConfig structure.
	if gatewayConfig.Replicas == nil {
		return fmt.Errorf("gateway config missing required 'replicas' for component '%s'", appComp.Name)
	}
	if gatewayConfig.Image == nil || (gatewayConfig.Image.Repository == "" && gatewayConfig.Image.Tag == "") {
		return fmt.Errorf("gateway config is missing required 'image' for component '%s'", appComp.Name)
	}
	if gatewayConfig.Ports == nil || len(gatewayConfig.Ports) == 0 {
		return fmt.Errorf("gateway config is missing required 'ports' configuration for component '%s'", appComp.Name)
	}
	isStatefulSet := b.GetWorkloadGVK().Kind == StatefulSetType
	if isStatefulSet {
		if gatewayConfig.Storage == nil || !gatewayConfig.Storage.Enabled {
			return fmt.Errorf("gateway workload is StatefulSet but Storage configuration is missing or disabled for component '%s'", appComp.Name)
		}
		if gatewayConfig.Storage.Enabled && gatewayConfig.Storage.Size == nil {
			return fmt.Errorf("gateway Storage is enabled but 'size' is missing for component '%s'", appComp.Name)
		}
		if gatewayConfig.Storage.Enabled && gatewayConfig.Storage.MountPath == "" {
			return fmt.Errorf("gateway Storage is enabled but 'mountPath' is missing for component '%s'", appComp.Name)
		}
	}

	return nil
}

// BuildObjects implements the AppBuilderStrategy interface.
func (b *GatewayBuilderStrategy) BuildObjects(ctx context.Context, k8sClient client.Client,
	scheme *runtime.Scheme, owner client.Object, appDef *appv1.ApplicationDefinition,
	appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) ([]client.Object, error) {
	logger := log.FromContext(ctx).WithValues("component", appComp.Name, "type", appComp.Type, "builder", "Gateway")

	// --- Unmarshal and Validate Specific Configuration ---
	gatewayConfig, ok := appSpecificConfig.(*common.ResourceConfig)
	if !ok {
		return nil, fmt.Errorf("internal error: expected *common.ResourceConfig for component '%s' but received type %T", appComp.Name, appSpecificConfig)
	}

	// Perform validation of the specific ResourceConfig structure.
	if err := b.verifyParameters(gatewayConfig, appComp); err != nil {
		return nil, fmt.Errorf("failed to validate ResourceConfig for component '%s': %w", appComp.Name, err)
	}
	// TODO: Add more Gateway specific validation if needed

	logger.V(1).Info("Validated ResourceConfig successfully")

	// --- Build K8s Spec Parts & Objects ---
	builtObjects := []client.Object{}

	// Derive common K8s object metadata and labels
	instanceName := appComp.Name
	appName := appDef.Name
	namespace := appDef.Namespace
	resourceName := builders.DeriveResourceName(instanceName) // Base name for STS, Services etc.
	commonLabels := builders.BuildCommonLabels(appName, appComp.Type, instanceName)
	selectorLabels := builders.BuildSelectorLabels(instanceName, appComp.Type) // Pass type for app.kubernetes.io/name

	logger.V(1).Info("Derived names and labels", "resourceName", resourceName, "commonLabels", commonLabels, "selectorLabels", selectorLabels)

	// --- 1. Build Pod Template Spec ---
	replicas := commonutil.GetInt32ValueOrDefault(gatewayConfig.Replicas, 1)

	mainContainerSpec, err := buildGatewayMainContainerSpec(gatewayConfig, instanceName)
	if err != nil {
		return nil, fmt.Errorf("failed to build Gateway main container spec for %s: %w", instanceName, err)
	}
	mainContainer := *mainContainerSpec

	initContainers := buildGatewayInitContainers(gatewayConfig, instanceName)
	logger.V(1).Info("Built init containers", "count", len(initContainers))

	volumes := buildGatewayVolumes(gatewayConfig, instanceName)
	logger.V(1).Info("Built volumes", "count", len(volumes))

	mainContainerVolumeMounts := buildGatewayVolumeMounts(gatewayConfig, instanceName)
	logger.V(1).Info("Built main container volume mounts", "count", len(mainContainerVolumeMounts))
	mainContainer.VolumeMounts = mainContainerVolumeMounts // Attach mounts

	nodeSelector := gatewayConfig.NodeSelector
	tolerations := gatewayConfig.Tolerations
	affinity := builders.GetAffinityOrDefault(gatewayConfig.Affinity)
	podSecurityContext := builders.GetPodSecurityContextOrDefault(gatewayConfig.PodSecurityContext)
	serviceAccountConfig := gatewayConfig.ServiceAccount
	serviceAccountName := builders.DeriveServiceAccountName(instanceName, serviceAccountConfig)

	podLabels := builders.MergeMaps(commonLabels, selectorLabels)
	var podAnnotations map[string]string
	// if gatewayConfig.PodAnnotations != nil { podAnnotations = gatewayConfig.PodAnnotations }

	logger.V(1).Info("Building PodTemplateSpec")
	builtPodTemplateSpec, err := builders.BuildPodTemplateSpec(
		[]corev1.Container{mainContainer}, initContainers, volumes,
		podSecurityContext, serviceAccountName, nodeSelector, tolerations, affinity,
		podLabels, podAnnotations,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build final PodTemplateSpec for Gateway %s: %w", instanceName, err)
	}
	logger.V(1).Info("Successfully built PodTemplateSpec")

	// --- 2. Build primary workload resource (StatefulSet for Gateway) ---
	stsMetadata := builders.BuildObjectMeta(resourceName, namespace, commonLabels, nil)

	vctList, err := builders.BuildVolumeClaimTemplates(gatewayConfig.Storage, commonLabels)
	if err != nil {
		return nil, fmt.Errorf("failed to build VCTs for Gateway %s: %w", instanceName, err)
	}
	logger.V(1).Info("Built VolumeClaimTemplates", "count", len(vctList))

	stsUpdateStrategy := builders.GetStatefulSetUpdateStrategyOrDefault(gatewayConfig.StatefulSetUpdateStrategy)
	stsPodManagementPolicy := builders.GetStatefulSetPodManagementPolicyOrDefault(gatewayConfig.PodManagementPolicy)

	headlessServiceName := builders.DeriveResourceName(instanceName) + "-headless"
	// Optional override check...

	logger.V(1).Info("Building StatefulSet Spec")
	stsSpec := appsv1.StatefulSetSpec{
		Replicas:             &replicas,
		Selector:             &metav1.LabelSelector{MatchLabels: selectorLabels},
		Template:             *builtPodTemplateSpec,
		VolumeClaimTemplates: vctList,
		ServiceName:          headlessServiceName,
		UpdateStrategy:       stsUpdateStrategy,
		PodManagementPolicy:  stsPodManagementPolicy,
		// RevisionHistoryLimit, MinReadySeconds...
	}

	logger.V(1).Info("Building StatefulSet object")
	statefulSet := builders.BuildStatefulSet(stsMetadata, stsSpec)
	builtObjects = append(builtObjects, statefulSet)
	logger.V(1).Info("Successfully built StatefulSet object", "name", statefulSet.Name)

	// --- 3. Build Headless Service ---
	headlessServiceMetadata := builders.BuildObjectMeta(headlessServiceName, namespace, commonLabels, nil)
	headlessServicePorts := builders.BuildServicePorts(gatewayConfig.Ports)
	logger.V(1).Info("Building Headless Service object", "name", headlessServiceName)
	headlessService := builders.BuildHeadlessService(headlessServiceMetadata, selectorLabels, headlessServicePorts)
	builtObjects = append(builtObjects, headlessService)
	logger.V(1).Info("Successfully built Headless Service object")

	// --- 4. Build Client/Transport Service (Optional) ---
	if gatewayConfig.Service != nil && ShouldBuildClientService(gatewayConfig.Service) { // Use local helper
		regularServiceName := resourceName
		regularServiceMetadata := builders.BuildObjectMeta(regularServiceName, namespace, commonLabels, gatewayConfig.Service.Annotations)
		clientServicePorts := builders.BuildServicePorts(gatewayConfig.Service.Ports)

		var serviceType corev1.ServiceType = corev1.ServiceTypeClusterIP // Default
		if gatewayConfig.Service.Type != nil {
			serviceType = *gatewayConfig.Service.Type
		}

		clientServiceSpec := corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    clientServicePorts,
			Type:     serviceType,
			// Other service spec fields...
		}

		logger.V(1).Info("Building Client Service object", "name", regularServiceName, "type", serviceType)
		clientService := builders.BuildService(regularServiceMetadata, selectorLabels, clientServiceSpec)
		builtObjects = append(builtObjects, clientService)
		logger.V(1).Info("Successfully built Client Service object")
	} else {
		logger.V(1).Info("Skipping Client Service creation based on configuration")
	}

	// --- 5. Build ConfigMaps/Secrets from Config File Data ---
	if gatewayConfig.ConfigFiles != nil && len(gatewayConfig.ConfigFiles) > 0 {
		configMapResourceName := resourceName + "-config"
		logger.V(1).Info("Building ConfigMap object", "name", configMapResourceName)
		cmObjects, err := builders.BuildConfigMapsFromAppData(gatewayConfig.ConfigFiles, configMapResourceName, namespace, commonLabels)
		if err != nil {
			return nil, fmt.Errorf("failed to build ConfigMaps from ConfigFiles for %s: %w", instanceName, err)
		}
		builtObjects = append(builtObjects, cmObjects...)
		logger.V(1).Info("Successfully built ConfigMap object(s)")
		// TODO: Handle Secrets similarly if needed
	}

	// --- 6. Build Service Account object ---
	if serviceAccountConfig != nil && commonutil.GetBoolValueOrDefault(serviceAccountConfig.Create, true) {
		saName := serviceAccountName
		saMetadata := builders.BuildObjectMeta(saName, namespace, commonLabels, serviceAccountConfig.Annotations)
		logger.V(1).Info("Building ServiceAccount object", "name", saName)
		serviceAccount := builders.BuildServiceAccount(saMetadata /*, pullSecrets, secrets */)
		builtObjects = append(builtObjects, serviceAccount)
		logger.V(1).Info("Successfully built ServiceAccount object")
	} else {
		logger.V(1).Info("Skipping ServiceAccount creation based on configuration")
	}

	// --- 7. Build PersistentVolumeClaim (for Deployment shared PVC) ---
	isStatefulSet := (gatewayWorkloadGVK.Kind == StatefulSetType)
	if !isStatefulSet && gatewayConfig.Persistence != nil && gatewayConfig.Persistence.Enabled {
		pvcName := builders.DeriveResourceName(instanceName) + "-pvc"
		logger.V(1).Info("Building shared PersistentVolumeClaim object", "name", pvcName)
		pvcObject, err := builders.BuildSharedPVCPVC(gatewayConfig.Persistence, instanceName, namespace, commonLabels)
		if err != nil {
			return nil, fmt.Errorf("failed to build shared PVC for %s: %w", instanceName, err)
		}
		if pvcObject != nil {
			pvcObject.ObjectMeta.Name = pvcName // Ensure name consistency
			pvcObject.ObjectMeta.Namespace = namespace
			pvcObject.ObjectMeta.Labels = commonLabels
			builtObjects = append(builtObjects, pvcObject)
			logger.V(1).Info("Successfully built shared PersistentVolumeClaim object")
		}
	}

	// --- 8. Build PodDisruptionBudget (Optional) ---
	// Uncomment and adapt if needed
	/*
		if gatewayConfig.PodDisruptionBudget != nil {
			// ... build PDB logic ...
		}
	*/

	logger.Info("Finished building all Kubernetes objects for Gateway", "count", len(builtObjects))
	return builtObjects, nil // Success!
}

// --- Application Specific Builder Helpers (internal to this package) ---
// NOTE: These helpers remain within builder.go as they are specific to Gateway building logic.

// buildGatewayMainContainerSpec builds the corev1.Container spec for the main gateway app.
func buildGatewayMainContainerSpec(gatewayConfig *common.ResourceConfig, instanceName string) (*corev1.Container, error) {
	logger := log.Log.WithName("gateway-container-builder")

	imageName := builders.BuildImageName(gatewayConfig.Image.Repository, gatewayConfig.Image.Tag)
	imagePullPolicy := builders.GetImagePullPolicy(gatewayConfig.Image.PullPolicy, gatewayConfig.Image.Tag)
	k8sPorts := builders.BuildContainerPorts(gatewayConfig.Ports)
	k8sResources := builders.BuildK8sResourceRequirements(gatewayConfig.Resources)

	var livenessProbe, readinessProbe, startupProbe *corev1.Probe
	if gatewayConfig.Probes != nil {
		livenessProbe = builders.BuildProbe(gatewayConfig.Probes.Liveness)
		readinessProbe = builders.BuildProbe(gatewayConfig.Probes.Readiness)
		startupProbe = builders.BuildProbe(gatewayConfig.Probes.Startup)
	}
	containerSecurityContext := builders.GetContainerSecurityContextOrDefault(gatewayConfig.ContainerSecurityContext)

	container := corev1.Container{
		Name:            builders.DeriveContainerName(instanceName),
		Image:           imageName,
		ImagePullPolicy: imagePullPolicy,
		Command:         gatewayConfig.Command, // Add if defined in common.ResourceConfig
		Args:            gatewayConfig.Args,    // Add if defined in common.ResourceConfig
		Ports:           k8sPorts,
		Env:             gatewayConfig.Env,
		EnvFrom:         gatewayConfig.EnvFrom,
		Resources:       k8sResources,
		LivenessProbe:   livenessProbe,
		ReadinessProbe:  readinessProbe,
		StartupProbe:    startupProbe,
		SecurityContext: containerSecurityContext,
		// VolumeMounts added later by caller
	}
	logger.V(1).Info("Built main container spec", "name", container.Name, "image", container.Image)
	return &container, nil
}

// buildGatewayInitContainers builds necessary init containers for the gateway.
func buildGatewayInitContainers(gatewayConfig *common.ResourceConfig, instanceName string) []corev1.Container {
	// Add custom init containers if defined
	if gatewayConfig.InitContainer == nil {
		return nil
	}

	initContainers := []corev1.Container{}
	logger := log.Log.WithName("gateway-init-builder").WithValues("instance", instanceName)
	isStatefulSet := (gatewayWorkloadGVK.Kind == StatefulSetType)

	var persistentMountPath string
	var persistentVolumeName string
	ownerUID, ownerGID := int64(1000), int64(1000) // Defaults

	if isStatefulSet && gatewayConfig.Storage != nil && gatewayConfig.Storage.Enabled {
		persistentMountPath = gatewayConfig.Storage.MountPath
		persistentVolumeName = commonutil.GetStringValueOrDefault(&gatewayConfig.Storage.VolumeClaimTemplateName, "data")
		logger.V(1).Info("StatefulSet storage detected", "mountPath", persistentMountPath, "volumeName(VCT)", persistentVolumeName)
	} else if !isStatefulSet && gatewayConfig.Persistence != nil && gatewayConfig.Persistence.Enabled {
		persistentMountPath = gatewayConfig.Persistence.MountPath
		persistentVolumeName = commonutil.GetStringValueOrDefault(&gatewayConfig.Persistence.VolumeName, builders.DeriveResourceName(instanceName)+"-pvc-vol")
		logger.V(1).Info("Deployment persistence detected", "mountPath", persistentMountPath, "volumeName", persistentVolumeName)
	}

	if gatewayConfig.PodSecurityContext != nil && gatewayConfig.PodSecurityContext.FSGroup != nil {
		ownerGID = *gatewayConfig.PodSecurityContext.FSGroup
	}
	if gatewayConfig.ContainerSecurityContext != nil {
		if gatewayConfig.ContainerSecurityContext.RunAsUser != nil {
			ownerUID = *gatewayConfig.ContainerSecurityContext.RunAsUser
		}
		if gatewayConfig.ContainerSecurityContext.RunAsGroup != nil {
			ownerGID = *gatewayConfig.ContainerSecurityContext.RunAsGroup
		}
	}

	if persistentMountPath != "" && persistentVolumeName != "" {
		initContainerName := "init-ensure-data-dir"
		logger.V(1).Info("Adding init container", "name", initContainerName, "path", persistentMountPath, "vol", persistentVolumeName, "owner", fmt.Sprintf("%d:%d", ownerUID, ownerGID))
		ensureDirInit := builders.BuildEnsureDirectoryContainer(initContainerName, "", persistentMountPath, ownerUID, ownerGID)
		ensureDirInit.VolumeMounts = []corev1.VolumeMount{{Name: persistentVolumeName, MountPath: persistentMountPath}}
		initContainers = append(initContainers, ensureDirInit)
	} else {
		logger.V(1).Info("Skipping data directory init container")
	}

	return initContainers
}

// buildGatewayVolumes builds the PodSpec.Volumes list (excluding VCTs).
func buildGatewayVolumes(gatewayConfig *common.ResourceConfig, instanceName string) []corev1.Volume {
	volumes := []corev1.Volume{}
	logger := log.Log.WithName("gateway-volume-builder").WithValues("instance", instanceName)
	isStatefulSet := (gatewayWorkloadGVK.Kind == StatefulSetType)

	cmVolumes := builders.BuildVolumesFromConfigMaps(gatewayConfig.ConfigMounts)
	volumes = append(volumes, cmVolumes...)
	logger.V(1).Info("Built volumes from ConfigMounts", "count", len(cmVolumes))

	secretVolumes := builders.BuildVolumesFromSecrets(gatewayConfig.SecretMounts)
	volumes = append(volumes, secretVolumes...)
	logger.V(1).Info("Built volumes from SecretMounts", "count", len(secretVolumes))

	if !isStatefulSet && gatewayConfig.Persistence != nil && gatewayConfig.Persistence.Enabled {
		pvcVolumeName := commonutil.GetStringValueOrDefault(&gatewayConfig.Persistence.VolumeName, builders.DeriveResourceName(instanceName)+"-pvc-vol")
		pvcResourceName := builders.DeriveResourceName(instanceName) + "-pvc"
		logger.V(1).Info("Adding volume definition for shared PVC", "volumeName", pvcVolumeName, "pvcName", pvcResourceName)
		volumes = append(volumes, corev1.Volume{
			Name: pvcVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcResourceName,
					ReadOnly:  false,
				},
			},
		})
	}

	// Add AdditionalVolumes if defined
	// if gatewayConfig.AdditionalVolumes != nil {
	//     // ... build and append ...
	// }

	return volumes
}

// buildGatewayVolumeMounts builds the main container's VolumeMounts list.
func buildGatewayVolumeMounts(gatewayConfig *common.ResourceConfig, instanceName string) []corev1.VolumeMount {
	allVolumeMounts := []corev1.VolumeMount{}
	logger := log.Log.WithName("gateway-mount-builder").WithValues("instance", instanceName)
	isStatefulSet := (gatewayWorkloadGVK.Kind == StatefulSetType)

	cmMounts := builders.BuildVolumeMountsFromConfigMaps(gatewayConfig.ConfigMounts)
	allVolumeMounts = append(allVolumeMounts, cmMounts...)
	logger.V(1).Info("Built volume mounts from ConfigMounts", "count", len(cmMounts))

	secretMounts := builders.BuildVolumeMountsFromSecrets(gatewayConfig.SecretMounts)
	allVolumeMounts = append(allVolumeMounts, secretMounts...)
	logger.V(1).Info("Built volume mounts from SecretMounts", "count", len(secretMounts))

	if !isStatefulSet && gatewayConfig.Persistence != nil && gatewayConfig.Persistence.Enabled {
		persistenceVolumeName := commonutil.GetStringValueOrDefault(&gatewayConfig.Persistence.VolumeName, builders.DeriveResourceName(instanceName)+"-pvc-vol")
		persistentMounts := builders.BuildPersistentVolumeMounts(gatewayConfig.Persistence, persistenceVolumeName)
		allVolumeMounts = append(allVolumeMounts, persistentMounts...)
		logger.V(1).Info("Built volume mounts from Persistence", "count", len(persistentMounts))
	}

	if isStatefulSet && gatewayConfig.Storage != nil && gatewayConfig.Storage.Enabled {
		storageMounts := builders.BuildVolumeMountsFromStorage(gatewayConfig.Storage)
		allVolumeMounts = append(allVolumeMounts, storageMounts...)
		logger.V(1).Info("Built volume mounts from Storage", "count", len(storageMounts))
	}

	// Add AdditionalVolumeMounts if defined
	// if gatewayConfig.VolumeMounts != nil {
	//     // ... build and append ...
	// }

	return allVolumeMounts
}

// ShouldBuildClientService determines if a regular client service should be built.
func ShouldBuildClientService(svcConfig *common.ServiceSpecPart) bool {
	if svcConfig == nil {
		return false
	}
	if svcConfig.Ports == nil || len(svcConfig.Ports) == 0 {
		// If no ports specified in Service section, don't build client service.
		return false
	}
	// Check if type is explicitly Headless (ClusterIP + ClusterIPNone)
	if svcConfig.Type != nil && *svcConfig.Type == corev1.ServiceTypeClusterIP {
		// Check if ClusterIP field exists in ServiceSpecPart and is "None"
		// if svcConfig.ClusterIP != nil && *svcConfig.ClusterIP == corev1.ClusterIPNone {
		//     return false
		// }
		// Assuming ClusterIP field is NOT in ServiceSpecPart for simplicity now.
		// If Type is ClusterIP, it's not headless unless ClusterIP=None, so build it.
	}
	// If Type is NodePort, LoadBalancer, or nil (defaults to ClusterIP), build it.
	// Explicitly check against ClusterIPNone just in case type was set but ClusterIP wasn't
	if svcConfig.Type != nil && *svcConfig.Type == corev1.ServiceType(corev1.ClusterIPNone) { // Explicit check against "None" string constant
		return false
	}

	return true
}
