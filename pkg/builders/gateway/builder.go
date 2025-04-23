// pkg/builders/gateway/strategy.go
package gateway

import (
	"context" // Needed for methods
	"fmt"     // For errors

	// K8s Types needed for building objects
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	// Import App types and Common types
	appv1 "github.com/infinilabs/operator/api/app/v1" // App types
	"github.com/infinilabs/operator/pkg/apis/common"  // Common types
	commonutil "github.com/infinilabs/operator/pkg/apis/common/util" // Common utils

	// Import other builders needed to construct nested/related objects
	builders "github.com/infinilabs/operator/pkg/builders/k8s" // Import generic K8s builders

	"github.com/infinilabs/operator/pkg/strategy" // Strategy interface and registry

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure our builder implementation complies with the strategy interface
var _ strategy.AppBuilderStrategy = &GatewayBuilderStrategy{} // This line ensures implementation checks

// GatewayBuilderStrategy is the concrete implementation of AppBuilderStrategy for the "gateway" type.
type GatewayBuilderStrategy struct{}

// Define the expected GVK for Gateway's primary workload (assuming StatefulSet).
// This should match what's set in the ComponentDefinition for 'gateway' type.
var gatewayWorkloadGVK = schema.FromAPIVersionAndKind("apps/v1", "StatefulSet")

// Register the GatewayBuilderStrategy strategy with the strategy registry.
func init() {
	// Register this builder using the component type name as the key ("gateway").
	strategy.RegisterAppBuilderStrategy("gateway", &GatewayBuilderStrategy{})
}

// GetWorkloadGVK implements the AppBuilderStrategy interface.
// Returns the expected primary workload GVK for Gateway.
func (b *GatewayBuilderStrategy) GetWorkloadGVK() schema.GroupVersionKind {
	return gatewayWorkloadGVK
}

// BuildObjects implements the AppBuilderStrategy interface.
// It builds the K8s objects (StatefulSet, Services, CMs, Secrets, PVC templates etc.)
// for a Gateway component instance based on the unmarshalled GatewayConfig.
func (b *GatewayBuilderStrategy) BuildObjects(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, owner client.Object, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) ([]client.Object, error) {

	// --- Unmarshal and Validate Specific Configuration ---
	// The 'appSpecificConfig' interface{} MUST be type asserted to the specific GatewayConfig type here.
	// It is assumed this function is ONLY called when the component type is "gateway".
	gatewayConfig, ok := appSpecificConfig.(*common.GatewayConfig)
	if !ok {
		// This indicates a type mismatch or issue in config unmarshalling *before* strategy dispatch.
		return nil, fmt.Errorf("expected *common.GatewayConfig for component '%s' but received type %T", appComp.Name, appSpecificConfig)
	}
	// If appSpecificConfig was nil (properties was empty), gatewayConfig will be nil pointer. Handle it.
	if gatewayConfig == nil {
        // If Gateway config is mandatory for this type, return an error.
         return nil, fmt.Errorf("Gateway configuration is mandatory but missing or empty for component '%s'", appComp.Name)
    }

	// Perform validation of the specific GatewayConfig structure.
	// Additional validation beyond common types (e.g., complex relationships, specific required fields)
	// should happen here or in dedicated validation functions.
	// TODO: Add GatewayConfig specific validation logic here.
	if gatewayConfig.Image == nil || (gatewayConfig.Image.Repository == "" && gatewayConfig.Image.Tag == "") {
         return nil, fmt.Errorf("Gateway config is missing required Image for component '%s'", appComp.Name)
    }
    if gatewayConfig.Ports == nil || len(gatewayConfig.Ports) == 0 {
         return nil, fmt.Errorf("Gateway config is missing required Ports configuration for component '%s'", appComp.Name)
    }
    // Check Storage if Workload is StatefulSet and Enabled=true
    if b.GetWorkloadGVK().GroupKind() == appsv1.Kind("StatefulSet").GroupKind() && (gatewayConfig.Storage == nil || !gatewayConfig.Storage.Enabled) {
         return nil, fmt.Errorf("Gateway workload is StatefulSet but Storage configuration is missing or disabled for component '%s'", appComp.Name)
    }


	// --- Build K8s Spec Parts & Objects (Specific to Gateway based on GatewayConfig) ---
	// Map GatewayConfig fields to standard K8s spec parts and build objects using generic builders.
	builtObjects := []client.Object{}

	// Derive common K8s object metadata and labels
	instanceName := appComp.Name
	appName := appDef.Name
	namespace := appDef.Namespace
	resourceName := builders.DeriveResourceName(instanceName)
	commonLabels := builders.BuildCommonLabels(appName, appComp.Type, instanceName)
	selectorLabels := builders.BuildSelectorLabels(instanceName)

	// 1. Build the primary K8s workload resource (StatefulSet for Gateway)
	// Build necessary inputs for the generic StatefulSet builder.
	// This maps fields from gatewayConfig to inputs for the K8s StatefulSet builder.

	// Resolve replicas from gatewayConfig
	replicas := builders.GetInt32ValueOrDefault(gatewayConfig.Replicas, 1)

	// Build Volume Claim Templates for StatefulSet Storage (if enabled)
	vctList := []corev1.PersistentVolumeClaim{}
	if gatewayConfig.Storage != nil && gatewayConfig.Storage.Enabled {
        // Ensure required fields are present IF enabled
         if gatewayConfig.Storage.Size == nil {
              return nil, fmt.Errorf("Gateway Storage is enabled but 'size' is missing for component '%s'", instanceName)
         }
         // Build VCTs based on storage config
         var err error
         vctList, err = builders.BuildVolumeClaimTemplates(gatewayConfig.Storage, commonLabels) // Use generic builders helper
         if err != nil { return nil, fmt.Errorf("failed to build VCTs for Gateway: %w", err)}
    } else if b.GetWorkloadGVK().GroupKind() == appsv1.Kind("StatefulSet").GroupKind() {
        // Gateway Workload is StatefulSet, but Storage is not enabled/missing.
        // Depends on whether stateful Gateway *can* run without persistence (rare for data).
        // Could return error or build STS without VCTs if that's supported by the app.
        // For now, validation above checks if Storage is mandatory for STS gateway.
    }

	// Build Pod Template Spec content: containers, init containers, volumes, mounts, scheduling, security, SA.
	// This needs to be a complex internal step calling lower-level builders.

    // --- Build PodSpec content based on gatewayConfig ---
    // This is where you assemble lists of K8s structs for PodSpec.

    // Build Main Container spec from GatewayConfig
    mainContainerSpec, err := buildGatewayMainContainerSpec(gatewayConfig) // Needs implementation below or separate builder

    if err != nil { return nil, fmt.Errorf("failed to build Gateway main container spec: %w", err)}
    mainContainer := *mainContainerSpec


	// Build Init Containers list based on GatewayConfig
	initContainers := buildGatewayInitContainers(gatewayConfig) // Needs implementation

	// Build list of Volumes based on ConfigMounts, SecretMounts, AdditionalVolumes (NOT Persistent/Storage)
	// Needs aggregation logic.
	volumes := buildGatewayVolumes(gatewayConfig) // Needs implementation

	// Build list of Volume Mounts for the main container based on ConfigMounts, SecretMounts, Persistence/Storage, etc.
	// Needs aggregation logic and reference the Volumes created or VCT names.
	mainContainerVolumeMounts := buildGatewayVolumeMounts(gatewayConfig) // Needs implementation

    // Assign calculated VolumeMounts to the main container
    mainContainer.VolumeMounts = mainContainerVolumeMounts

	// Resolve scheduling (NodeSelector, Tolerations, Affinity) from GatewayConfig
	nodeSelector := gatewayConfig.NodeSelector // Use map directly if common.types has map[string]string
	tolerations := gatewayConfig.Tolerations   // Use slice directly if common.types has []corev1.Toleration
	affinity := gatewayConfig.Affinity         // Use pointer directly

	// Resolve Security Contexts (Pod and Container) from GatewayConfig
	podSecurityContext := gatewayConfig.PodSecurityContext       // Use pointer
	containerSecurityContext := gatewayConfig.ContainerSecurityContext // Use pointer

	// Resolve Service Account Config
	serviceAccountConfig := gatewayConfig.ServiceAccount // Use pointer
	serviceAccountName := builders.DeriveServiceAccountName(instanceName, serviceAccountConfig) // Use helper

	// Build final PodTemplateSpec using common builder with all aggregated parts
	podTemplateSpec, err := builders.BuildPodTemplateSpec(
		[]corev1.Container{mainContainer}, // Containers list (includes main + any others)
		initContainers,                   // Init Containers list
		volumes,                           // Volumes list (excl. VCT)
		// mainContainerVolumeMounts is now inside mainContainer, no need to pass separately
		// But common PodTemplateSpec builder needs inputs for PodSpec level fields.
		// Adjusting common.PodTemplateSpec builder inputs in mind. Let's re-evaluate.
        // Generic Pod Builder should receive: Container list, Init list, Volume list, and standard PodSpec fields.
        // MainContainer build should return corev1.Container *with VolumeMounts*.

        // Let's pass all necessary direct PodSpec fields explicitly to a *generic* PodTemplate builder
        // after deriving them from gatewayConfig.

        // Call generic BuildPodTemplateSpec from common builders
        builtPodTemplateSpec, err := builders.BuildPodTemplateSpec(
            // Standard PodSpec parts inputs:
             initContainers,       // []corev1.Container list
             []corev1.Container{mainContainer}, // []corev1.Container list (just the main one for now)
             volumes,               // []corev1.Volume list

             // PodSpec direct fields:
             nodeSelector, // map[string]string
             tolerations, // []corev1.Toleration
             affinity, // *corev1.Affinity
             podSecurityContext, // *corev1.PodSecurityContext
             serviceAccountName, // string (derived)

             // Pod Metadata parts (mostly labels from context/helpers)
             podLabels, // map[string]string
             // No Pod Annotations field in common.ComponentConfig currently, if needed add it and pass it.
        )

		if err != nil { return nil, fmtf.Errorf("failed to build PodTemplateSpec for Gateway: %w", err)}


	// 2. Build primary workload resource (StatefulSet for Gateway)
	// Call generic StatefulSet builder, passing resolved K8s specs.
	statefulSet := builders.BuildStatefulSet(
		appDef,       // Owner
		appComp,      // Component context
		resourceName, // Derived name
		commonLabels, // Common labels for STS object
		selectorLabels, // Selector labels for STS
		&replicas,      // Resolved replicas (pointer)
		*builtPodTemplateSpec, // Pre-built Pod template (dereferenced)
		vctList,        // Pre-built Volume Claim Templates list
		commonutil.GetStatefulSetUpdateStrategyOrDefault(gatewayConfig.StatefulSetOverrides), // Update Strategy
		commonutil.GetStatefulSetPodManagementPolicyOrDefault(gatewayConfig.StatefulSetOverrides), // Pod Management Policy
		// Pass other direct StatefulSet fields if supported and in common.types
	)

	builtObjects = append(builtObjects, statefulSet) // Add StatefulSet object


	// 3. Build Headless Service (REQUIRED by StatefulSet)
	// Name convention derived from instance name.
	headlessServiceName := builders.DeriveResourceName(instanceName) + "-headless"
	// Optionally override from GatewayConfig if a specific field is added.
	// if gatewayConfig.StatefulSetOverrides != nil && gatewayConfig.StatefulSetOverrides.ServiceName != nil && *gatewayConfig.StatefulSetOverrides.ServiceName != "" { headlessServiceName = *gatewayConfig.StatefulSetOverrides.ServiceName }

	headlessService := builders.BuildHeadlessService(
		gatewayConfig.Service, // Services config part (needed for ports)
		builders.BuildServiceMetadata(headlessServiceName, namespace, commonLabels), // Needs Builder for Metadata
		selectorLabels, // Selector
	)
	builtObjects = append(builtObjects, headlessService) // Add Headless Service


	// 4. Build Client/Transport Service (Optional, based on gatewayConfig.Service)
	// Build only if type is not ClusterIP=None and ports are defined.
	if gatewayConfig.Service != nil && (gatewayConfig.Service.Type != nil && *gatewayConfig.Service.Type != corev1.ServiceTypeClusterIPNone) && gatewayConfig.Service.Ports != nil && len(gatewayConfig.Service.Ports) > 0 {
         // Regular Service name is often the same as the primary workload name.
          regularServiceName := builders.DeriveResourceName(instanceName) // Standard convention
         clientService := builders.BuildService( // Use generic Service builder
              gatewayConfig.Service, // Services config part (needed for ports, type)
               builders.BuildServiceMetadata(regularServiceName, namespace, commonLabels), // Metadata
               selectorLabels, // Selector
          )
         builtObjects = append(builtObjects, clientService) // Add regular Service
    }


	// 5. Build ConfigMaps/Secrets from Config File Data (AppConfigData map)
	// Needs Builders specifically for CMs/Secrets from a map[string]string
	if gatewayConfig.AppConfigData != nil && len(gatewayConfig.AppConfigData) > 0 {
		// Builders create CMs/Secrets, setting names, namespaces, labels, and putting data inside.
		// The Pod Template Builder already assumes standard mount paths exist if ConfigMounts/SecretMounts/AppData are used.
		cmObjects, err := builders.BuildConfigMapsFromAppData(gatewayConfig.AppConfigData, builders.DeriveResourceName(instanceName), namespace, commonLabels) // Use DerivedResourceName + suffix if needed
		if err != nil { return nil, fmt.Errorf("failed to build ConfigMaps from AppConfigData: %w", err)}
		builtObjects = append(builtObjects, cmObjects...) // Append the built ConfigMaps
	}
	// TODO: Add SecretFromAppData if needed.

	// 6. Build Service Account object if needed (create=true)
	if gatewayConfig.ServiceAccount != nil && builders.GetBoolPtrValueOrDefault(gatewayConfig.ServiceAccount.Create, true) { // Check create flag with default
         serviceAccount := builders.BuildServiceAccount(gatewayConfig.ServiceAccount, builders.DeriveServiceAccountName(instanceName, gatewayConfig.ServiceAccount), namespace, commonLabels) // Needs builders.BuildServiceAccount
         builtObjects = append(builtObjects, serviceAccount)
    }

	// TODO: Build PDB object if PDB config exists (needs its own builder)
	// if gatewayConfig.Pdb != nil {
	//     pdbObject := builders.BuildPodDisruptionBudget(gatewayConfig.Pdb, builders.DeriveResourceName(instanceName) + "-pdb", namespace, commonLabels, selectorLabels)
	//     builtObjects = append(builtObjects, pdbObject)
	// }


	// --- Return the final list of K8s objects ---
	return builtObjects, nil // Success!
}

// --- Application Specific Builder Helper (internal to this package) ---

// buildGatewayMainContainerSpec builds the specification for the primary Gateway container.
// It maps fields from GatewayConfig to corev1.Container spec.
// This helper is specific to Gateway and used by the Gateway builder strategy.
func buildGatewayMainContainerSpec(gatewayConfig *common.GatewayConfig) (*corev1.Container, error) {

    // Ensure essential fields for the container exist (validated earlier)
    if gatewayConfig == nil || gatewayConfig.Image == nil || (gatewayConfig.Image.Repository == "" && gatewayConfig.Image.Tag == "") || gatewayConfig.Ports == nil || len(gatewayConfig.Ports) == 0 {
         return nil, fmt.Errorf("essential configuration (Image, Ports) is missing for Gateway main container spec")
    }

	// Use common helpers to build nested structures where possible.
	container := corev1.Container{
		Name:            builders.DeriveContainerName("gateway"), // Container name specific to gateway app type
		Image:           builders.BuildImageName(gatewayConfig.Image), // Use common helper
		ImagePullPolicy: builders.GetImagePullPolicyOrDefault(gatewayConfig.Image.PullPolicy), // Use common helper

		// Command and Args: Application specific! Use defaults or derive from config if fields exist.
		// If config allows specifying raw Command/Args: Command: gatewayConfig.Command, Args: gatewayConfig.Args
		// Or derive args based on common/app-specific config fields:
		// Example: Args based on service ports/binding from config.Service or fixed pattern
		// For now, assume gateway image has a standard entrypoint, just need ENV or VolumeMounts config.

		Ports: builders.BuildContainerPorts(gatewayConfig.Ports), // Use common Port builder

		// Environment variables from config (User defined Env + EnvFrom)
		Env:     gatewayConfig.Env,
		EnvFrom: gatewayConfig.EnvFrom,

		// Resources: Use common builder/helper
		Resources: builders.GetResourcesSpecOrDefault(gatewayConfig.Resources),

		// Probes: Use common builder/helper (assuming they take K8s Probe specs directly)
		LivenessProbe:  builders.BuildProbe(gatewayConfig.Probes.Liveness), // Needs builders.BuildProbe helper
		ReadinessProbe: builders.BuildProbe(gatewayConfig.Probes.Readiness),
		StartupProbe:   builders.BuildProbe(gatewayConfig.Probes.Startup),

		// Security Context (Container level):
		SecurityContext: gatewayConfig.ContainerSecurityContext, // This is a pointer, directly used.

		// VolumeMounts - These are *aggregated* from all sources (ConfigMounts, SecretMounts, Persistence/Storage mounts, etc.)
		// by the higher-level builder (BuildDeploymentResources/BuildStatefulSetResources).
		// They are then added to the main container *after* this function builds the core spec.
		// So this function does NOT set VolumeMounts.
	}

	return &container, nil // Return pointer to container spec
}


// buildGatewayInitContainers builds the list of init containers for Gateway based on its config.
// It should include standard operator-managed init containers for gateway (like ensure data dir).
// Returns a list of K8s corev1.Container specs.
func buildGatewayInitContainers(gatewayConfig *common.GatewayConfig) []corev1.Container {
	initContainers := []corev1.Container{}

	// Add a standard init container to ensure the data directory exists for persistent storage.
	// This is crucial for StatefulSets and Deployments with persistence.
	// Determine the path based on Storage or Persistence config.
	var persistentMountPath string
	var persistentVolumeName string // Volume name for the persistent volume
	var ownerUID int64 = 1000      // Default owner UID
	var ownerGID int64 = 1000      // Default owner GID (e.g., Elasticsearch/OpenSearch user ID)

	isStatefulSetWorkload := (gatewayWorkloadGVK.GroupKind() == appsv1.Kind("StatefulSet").GroupKind())

	if isStatefulSetWorkload && gatewayConfig.Storage != nil && gatewayConfig.Storage.Enabled {
		persistentMountPath = gatewayConfig.Storage.MountPath
		persistentVolumeName = gatewayConfig.Storage.VolumeClaimTemplateName // Use the VCT name
		// Optional: Get owner from config if present (e.g., podSecurityContext.fsGroup, containerSecurityContext.runAsUser/runAsGroup)
        if gatewayConfig.PodSecurityContext != nil && gatewayConfig.PodSecurityContext.FSGroup != nil { ownerGID = *gatewayConfig.PodSecurityContext.FSGroup }
        // Could also use ContainerSecurityContext.RunAsUser/RunAsGroup, but FSGroup is common for volume ownership
        if gatewayConfig.ContainerSecurityContext != nil && gatewayConfig.ContainerSecurityContext.RunAsUser != nil { ownerUID = *gatewayConfig.ContainerSecurityContext.RunAsUser }
        if gatewayConfig.ContainerSecurityContext != nil && gatewayConfig.ContainerSecurityContext.RunAsGroup != nil { ownerGID = *gatewayConfig.ContainerSecurityContext.RunAsGroup }


	} else if gatewayConfig.Persistence != nil && gatewayConfig.Persistence.Enabled {
		persistentMountPath = gatewayConfig.Persistence.MountPath
		persistentVolumeName = gatewayConfig.Persistence.VolumeName // Use the Volume Name for the shared PVC
        // Get owner like above if applicable
         if gatewayConfig.PodSecurityContext != nil && gatewayConfig.PodSecurityContext.FSGroup != nil { ownerGID = *gatewayConfig.PodSecurityContext.FSGroup }
         if gatewayConfig.ContainerSecurityContext != nil && gatewayConfig.ContainerSecurityContext.RunAsUser != nil { ownerUID = *gatewayConfig.ContainerSecurityContext.RunAsUser }
         if gatewayConfig.ContainerSecurityContext != nil && gatewayConfig.ContainerSecurityContext.RunAsGroup != nil { ownerGID = *gatewayConfig.ContainerSecurityContext.RunAsGroup }

	}
    // Only add the ensure-dir init container if persistent storage (Storage or Persistence) is enabled and the mount path is set.
    // Also, check if init container itself is explicitly disabled by user in config if such a flag exists.
    // e.g., if gatewayConfig.InitContainers != nil && gatewayConfig.InitContainers.Common != nil && gatewayConfig.InitContainers.Common.EnsureDataDirectoryExists != nil && !*gatewayConfig.InitContainers.Common.EnsureDataDirectoryExists { /* skip adding this init container */ }
    // For now, simple check: if mount path for persistence/storage is valid, add the ensure dir init.

    if persistentMountPath != "" && persistentVolumeName != "" {
         // Build a standard ensure directory init container
         ensureDirInit := builders.BuildEnsureDirectoryContainer("init-data-dir", persistentMountPath, ownerUID, ownerGID) // Use helper in builders.k8s/

         // The EnsureDirectoryContainer needs to mount the persistent volume itself.
         // Add the necessary volume mount to its spec.
         ensureDirInit.VolumeMounts = []corev1.VolumeMount{
              {
                   Name: persistentVolumeName, // Must match the persistent volume name (VCT name or Persistence VolumeName)
                   MountPath: persistentMountPath, // Mount the persistent volume here too
                   // Ensure the permissions logic in the command is correct (chown)
              },
         }
         initContainers = append(initContainers, ensureDirInit)
    }


	// Add user-provided custom init containers (raw K8s spec slice)
	// Assuming GatewayConfig has a field like CustomInitContainers []corev1.Container
	// if gatewayConfig.InitContainers != nil && gatewayConfig.InitContainers.Custom != nil && len(gatewayConfig.InitContainers.Custom) > 0 {
	//     initContainers = append(initContainers, gatewayConfig.InitContainers.Custom...) // Append raw containers from config
	// }

	// TODO: Add more Gateway-specific init containers based on config (e.g., Bootstrap Job if using DaemonSet/Pod or specific K8s op)

	return initContainers // Return the list of built init containers
}


// buildGatewayVolumes builds the list of corev1.Volume objects for the Gateway Pod spec.
// Aggregates volumes from ConfigMounts, SecretMounts, AdditionalVolumes.
// Persistence/Storage volumes are handled separately (either PVC object built, or implicit VCT).
func buildGatewayVolumes(gatewayConfig *common.GatewayConfig) []corev1.Volume {
	volumes := []corev1.Volume{}

	// Add Volumes for ConfigMaps based on config.ConfigMounts
	// Calls a builder helper in builders/k8s/
	cmVolumes := builders.BuildVolumesFromConfigMounts(gatewayConfig.ConfigMounts)
	volumes = append(volumes, cmVolumes...)

	// Add Volumes for Secrets based on config.SecretMounts
	secretVolumes := builders.BuildVolumesFromSecrets(gatewayConfig.SecretMounts)
	volumes = append(volumes, secretVolumes...)

	// Add Volumes from common.AdditionalVolumes (e.g. EmptyDir, HostPath provided raw)
	// If gatewayConfig has a field like AdditionalVolumes []corev1.Volume
	// if gatewayConfig.AdditionalVolumes != nil && len(gatewayConfig.AdditionalVolumes) > 0 {
	//     volumes = append(volumes, builders.BuildVolumesFromAdditionalVolumes(gatewayConfig.AdditionalVolumes)...) // Needs helper
	// }

	// TODO: Add volumes needed by operator/standard patterns (e.g., emptyDir for logs/temp files if common)

	return volumes
}


// buildGatewayVolumeMounts builds the list of corev1.VolumeMount objects for the Gateway main container.
// Aggregates mounts from common ConfigMounts, SecretMounts, Persistence/Storage, and explicit VolumeMounts.
// The calling Pod builder (BuildPodTemplateSpec) expects the list of ALL mounts for the main container.
func buildGatewayVolumeMounts(gatewayConfig *common.GatewayConfig) []corev1.VolumeMount {
	allVolumeMounts := []corev1.VolumeMount{}

	// Add Volume Mounts from common.ConfigMounts
	// Calls a builder helper in builders/k8s/
	cmMounts := builders.BuildVolumeMountsFromConfigMaps(gatewayConfig.ConfigMounts)
	allVolumeMounts = append(allVolumeMounts, cmMounts...)

	// Add Volume Mounts from common.SecretMounts
	secretMounts := builders.BuildVolumeMountsFromSecrets(gatewayConfig.SecretMounts)
	allVolumeMounts = append(allVolumeMounts, secretMounts...)

	// Add Volume Mounts from common.Persistence (shared PVC) if enabled
	// Needs resolved VolumeName for the shared PVC.
	if gatewayConfig.Persistence != nil && gatewayConfig.Persistence.Enabled {
        persistenceVolumeName := builders.DeriveResourceName("todo") + "-pvc-vol" // Default or derived Name
        if gatewayConfig.Persistence.VolumeName != "" { persistenceVolumeName = gatewayConfig.Persistence.VolumeName }

        persistentMounts := builders.BuildPersistentVolumeMounts(gatewayConfig.Persistence, persistenceVolumeName) // Needs helper
        allVolumeMounts = append(allVolumeMounts, persistentMounts...)
	}


	// Add Volume Mounts from common.Storage (per-replica PVC) if enabled
	if gatewayConfig.Storage != nil && gatewayConfig.Storage.Enabled {
         // Needs resolved VolumeClaimTemplate Name.
          volumeClaimTemplateName := storageConfig.VolumeClaimTemplateName // Use config value (defaulted earlier)
         if volumeClaimTemplateName == "" { // Default name
              volumeClaimTemplateName = builders.DeriveResourceName("todo") + "-data" // Example
          }
         // Need Builder for the Storage volume mount using the VCT name.
         // Needs builders.BuildVolumeMountsFromStorage helper
         storageMounts := builders.BuildVolumeMountsFromStorage(gatewayConfig.Storage)
         allVolumeMounts = append(allVolumeMounts, storageMounts...)
    }


	// Add user-provided explicit VolumeMounts from common.VolumeMounts (raw K8s spec slice)
	if gatewayConfig.VolumeMounts != nil && len(gatewayConfig.VolumeMounts) > 0 {
         // This assumes common.VolumeMounts contains *valid* raw K8s VolumeMount specs.
         // They should already have Volume Name, MountPath etc. defined.
         // The builder just appends them. No DeepCopy needed if common.VolumeMounts is slice of value types or builder guarantees new structs.
         allVolumeMounts = append(allVolumeMounts, gatewayConfig.VolumeMounts...) // Append user's explicit mounts
    }

	// TODO: Add Mounts for common/standard patterns (e.g., emptyDir for logs/temp) if corresponding volumes are built.

	return allVolumeMounts // Return the aggregated list
}