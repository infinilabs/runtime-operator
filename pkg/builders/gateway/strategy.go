// pkg/builders/gateway/strategy.go
package gateway

import (
	"context" // Needed for methods
	"fmt"     // For errors
	"strings" // For string manipulation

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1" // Needed for K8s types like ServiceAccountSpec in helpers
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appv1 "github.com/infinilabs/operator/api/app/v1" // App types
	"github.com/infinilabs/operator/pkg/apis/common" // Common types
	// commonutil "github.com/infinilabs/operator/pkg/apis/common/util" // Common utils

	builders "github.com/infinilabs/operator/pkg/builders/k8s" // Generic K8s builders
	// Specific builders/helpers if needed (e.g., buildGatewayContainerSpec if logic is complex)
	// gateway_helpers "github.com/infinilabs/operator/pkg/builders/gateway/helpers"

	"github.com/infinilabs/operator/pkg/strategy" // Strategy interface and registry

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure our builder implementation complies with the strategy interface
var _ strategy.AppBuilderStrategy = &GatewayBuilderStrategy{} // This line ensures implementation checks

// GatewayBuilderStrategy is the concrete implementation of AppBuilderStrategy for the "gateway" application type.
type GatewayBuilderStrategy struct{}

// Define the expected GVK for Gateway's primary workload.
// This should match what's set in the ComponentDefinition for 'gateway' type.
var gatewayWorkloadGVK = schema.FromAPIVersionAndKind("apps/v1", "StatefulSet") // Assuming Gateway is StatefulSet


// Register the GatewayBuilderStrategy strategy with the strategy registry.
// This registration happens automatically when this package is imported.
func init() {
	// Register this builder using the component type name as the key.
	// This name must match AppComp.Type field in ApplicationDefinition.
	strategy.RegisterAppBuilderStrategy("gateway", &GatewayBuilderStrategy{})
}


// GetWorkloadGVK implements the AppBuilderStrategy interface.
// Returns the expected primary workload GVK for Gateway.
func (b *GatewayBuilderStrategy) GetWorkloadGVK() schema.GroupVersionKind {
	return gatewayWorkloadGVK
}

// BuildObjects implements the AppBuilderStrategy interface.
// It builds the Kubernetes objects (StatefulSet, Services, CMs, Secrets, PVC templates etc.)
// for a Gateway component instance based on the unmarshalled GatewayConfig.
func (b *GatewayBuilderStrategy) BuildObjects(ctx context.Context, k8sClient client.Client, scheme *runtime.Scheme, owner client.Object, appDef *appv1.ApplicationDefinition, appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) ([]client.Object, error) {

	// --- Unmarshal and Validate Specific Configuration ---
	// The 'appSpecificConfig' interface{} MUST be type asserted to the specific GatewayConfig type here.
	// It is assumed this function is ONLY called when the component type is "gateway".
	// Based on common.types.go, the specific config for Gateway is *common.GatewayConfig
	gatewayConfig, ok := appSpecificConfig.(*common.GatewayConfig)
	if !ok {
		// This indicates a type mismatch in the input config or issue in unmarshalling layer.
		return nil, fmt.Errorf("expected *common.GatewayConfig for component '%s' but received type %T", appComp.Name, appSpecificConfig)
	}
	// If appSpecificConfig was nil (properties was empty), gatewayConfig will be nil pointer. Handle it.
	if gatewayConfig == nil {
		// If Gateway config is mandatory for this type, return an error.
		// Note: Specific application config struct itself (GatewayConfig) does not define Required validation at CRD level if Properties is RawExtension.
		// Validation is handled by unmarshalling + checks here, or by a validating webhook.
		// Let's perform basic check and error if core parts are missing.
		return nil, fmt.Errorf("Gateway configuration (properties) is mandatory but missing or empty for component '%s'", appComp.Name)
	}

	// Perform validation of the specific GatewayConfig structure (if not done by webhook).
	// Check required fields for Gateway based on its design.
	// This validation is specific to the Gateway application logic/structure.
	// Add required fields like Replicas, Image, Ports.
	if gatewayConfig.Replicas == nil { return nil, fmt.Errorf("Gateway config missing required 'replicas'") }
	if gatewayConfig.Image == nil || (gatewayConfig.Image.Repository == "" && gatewayConfig.Image.Tag == "") { return nil, fmt.Errorf("Gateway config missing required 'image'") }
	if gatewayConfig.Ports == nil || len(gatewayConfig.Ports) == 0 { return nil, fmt.Errorf("Gateway config missing required 'ports'") }

	// Check Workload Type defined in ComponentDefinition if consistent builder used.
	// Strategy has GetWorkloadGVK(), Compare this with expected GVK (StatefulSet for Gateway).
	if b.GetWorkloadGVK().GroupKind() != appsv1.Kind("StatefulSet").GroupKind() {
         // This strategy is built for StatefulSet Gateway. Log a warning if used with different workload.
    }


	// --- Build Standard K8s Spec Parts & Objects based on GatewayConfig ---
	// Map GatewayConfig fields to standard K8s spec parts using helpers and builder conventions.
	// Use common builders (from pkg/builders/k8s/) to construct the final K8s objects.

	builtObjects := []client.Object{} // Slice to hold built objects


	// 1. Determine basic metadata and labels for K8s objects
	instanceName := appComp.Name // Name of the component instance
	appName := appDef.Name       // Name of the application definition
	namespace := appDef.Namespace // Namespace
	resourceName := builders.DeriveResourceName(instanceName) // Base name for K8s resources
	commonLabels := builders.BuildCommonLabels(appName, appComp.Type, instanceName) // Standard K8s labels and app labels
	selectorLabels := builders.BuildSelectorLabels(instanceName) // Labels for Deployment/StatefulSet selectors

	// Resolve core Pod Spec common fields from GatewayConfig using helpers/defaulting
	// Pass gatewayConfig fields explicitly or in logical groups.

	// Prepare inputs for common Pod Template builder
	// Common Pod Spec fields
	podSecurityContext := builders.GetPodSecurityContextOrDefault(gatewayConfig.PodSecurityContext) // Uses helper
	serviceAccountName := builders.DeriveServiceAccountName(instanceName, gatewayConfig.ServiceAccount) // Uses helper
	nodeSelector := gatewayConfig.NodeSelector                                                       // Direct map/slice
	tolerations := gatewayConfig.Tolerations                                                         // Direct slice
	affinity := gatewayConfig.Affinity                                                               // Pointer

	// Prepare Container Spec for the main Gateway container
	// Build Main Container spec using a helper (potentially specific to Gateway nuances if needed, or standard).
	// Let's use a helper in builders/k8s/pod.go to map common container fields.
	// This assumes standard fields in common.types can build main container.
	mainContainer, err := builders.BuildMainContainerSpec( // Use standard helper
		builders.DeriveContainerName("gateway"), // Name for the container ("gateway" or derive from type)
		*gatewayConfig.Image, // Image spec (pass value, needs helper for ptr default/value handling)
		gatewayConfig.Resources, // Resources spec (pointer)
		gatewayConfig.Ports, // []PortSpec (slice value)
		gatewayConfig.Env,     // []corev1.EnvVar (slice value)
		gatewayConfig.EnvFrom, // []corev1.EnvFromSource (slice value)
		gatewayConfig.Probes, // Probes (pointer)
		gatewayConfig.ContainerSecurityContext, // Container SC (pointer)
		// Add command/args if they exist in common.GatewayConfig
		// gatewayConfig.Command, // needs definition and passing
		// gatewayConfig.Args, // needs definition and passing
		// Add working dir etc. if they exist and needed.
	)
	if err != nil { return nil, fmt.Errorf("failed to build main Gateway container spec: %w", err)}


	// Build Init Containers list
	// Include standard operator-managed init containers for Gateway + any raw user-defined ones.
	initContainers := buildGatewayInitContainers(gatewayConfig) // Needs helper in this package (gateway/builders.go)

	// Build Volumes and Volume Mounts lists
	// Aggregate mounts from CMs, Secrets, Persistence/Storage, and explicit mounts.
	// Create Volumes based on this.
	volumes, mainContainerVolumeMounts, err := buildGatewayVolumesAndMounts(gatewayConfig) // Needs helper in this package

	if err != nil { return nil, fmtf.Errorf("failed to build Gateway volumes/mounts: %w", err)}

    // Attach aggregated volume mounts to the main container spec
    mainContainer.VolumeMounts = mainContainerVolumeMounts


	// Build final PodTemplateSpec using generic builder with all assembled parts
	podTemplateSpec, err := builders.BuildPodTemplateSpec(
		// Standard PodSpec lists inputs
		[]corev1.Container{mainContainer}, // List containing the built main container
		initContainers,                   // List of init containers
		volumes,                           // List of Volumes

		// PodSpec direct fields inputs (parsed/defaulted from GatewayConfig)
		nodeSelector,         // Map
		tolerations,        // Slice
		affinity,             // Pointer
		podSecurityContext, // Pointer
		serviceAccountName,   // string

		// Pod Metadata labels (pre-built)
		podLabels, // Common/Selector labels combined
		// No Pod annotations handled in common.types currently
	)
	if err != nil { return nil, fmtf.Errorf("failed to build final PodTemplateSpec for Gateway: %w", err)}


	// 2. Build primary workload resource (StatefulSet for Gateway)
	// This calls the generic StatefulSet builder (builders.k8s.BuildStatefulSet)
	// Needs all resolved inputs for StatefulSet Spec.

	// Resolve StatefulSet Spec fields specific to Gateway workload pattern
	stsMetadata := builders.BuildObjectMeta(resourceName, namespace, commonLabels, nil) // STS ObjectMeta
	stsSelectorLabels := selectorLabels // STS Selector is usually same as Pod Selector

	// Build Volume Claim Templates for StatefulSet Storage (if enabled in config)
	vctList, err := builders.BuildVolumeClaimTemplates(gatewayConfig.Storage, commonLabels) // Needs common builder

	if err != nil { return nil, fmt.Errorf("failed to build VCTs for Gateway: %w", err)}

	// Determine Update Strategy and Pod Management Policy (default or config override)
	// Need helper functions similar to GetDeploymentStrategyOrDefault for StatefulSet
	// If overrides are in common.GatewayConfig, need to map them.
	// Assuming GatewayConfig has direct fields like StatefulSetUpdateStrategy etc. (referencing common.types.go definitions)

	stsUpdateStrategy := builders.GetStatefulSetUpdateStrategyOrDefault(gatewayConfig.StatefulSetOverrides) // Need to pass GatewayConfig fields
	stsPodManagementPolicy := builders.GetStatefulSetPodManagementPolicyOrDefault(gatewayConfig.StatefulSetOverrides) // Need to pass GatewayConfig fields

	// Call generic StatefulSet builder
	statefulSet := builders.BuildStatefulSet(
		stsMetadata, // STS ObjectMeta
		stsSelectorLabels, // STS Selector
		&replicas,         // Resolved replicas pointer
		*builtPodTemplateSpec, // Pre-built Pod template
		vctList,               // Pre-built VCT list
		commonutil.GetServiceAccountName(instanceName, gatewayConfig.ServiceAccount), // Get SA name from config helper

		// ServiceAccountName is applied in PodTemplateSpec, not StatefulSetSpec usually.

		// Pass Headless Service name to StatefulSet Spec
		// It's often instanceName + "-headless".
		// Add parameter for Headless Service name if it's in GatewayConfig overrides
		headlessServiceName := builders.DeriveResourceName(instanceName) + "-headless" // Convention
		// If GatewayConfig allows overriding headless service name:
		// if gatewayConfig.Services != nil && gatewayConfig.Services.HeadlessServiceName != nil && *gatewayConfig.Services.HeadlessServiceName != "" {
		//      headlessServiceName = *gatewayConfig.Services.HeadlessServiceName
		// }

		// Need to build the StatefulSet Spec content as a value struct before calling generic BuildStatefulSet
		stsSpec := appsv1.StatefulSetSpec{
            Replicas: &replicas, // pointer
            Selector: &metav1.LabelSelector{MatchLabels: selectorLabels},
            ServiceName: headlessServiceName,
            Template: *builtPodTemplateSpec, // Value copy of built template
            VolumeClaimTemplates: vctList, // Slice of VCTs
            UpdateStrategy: stsUpdateStrategy, // Value
            PodManagementPolicy: stsPodManagementPolicy, // Value
             // Service Account is set in Pod Spec Template, not STS Spec.
             // persistentVolumeClaimRetentionPolicy: // If needed and in common config
             // revisionHistoryLimit: // If needed and in common config

		}


		// Call generic StatefulSet builder with built spec and meta
		statefulSet = builders.BuildStatefulSet(
			stsMetadata,
			stsSpec,
		)


	builtObjects = append(builtObjects, statefulSet) // Add StatefulSet object to the list


	// 3. Build Services (Headless and regular Client Service)
	// Call generic Service builders, passing relevant parts from GatewayConfig.
	// Metadata for services: separate BuildServiceMetadata helper in builders/k8s/service.go?
	// Labels and selectors: from builders.BuildCommonLabels/BuildSelectorLabels.

	// Build Headless Service object metadata and spec inputs
	headlessServiceMetadata := builders.BuildServiceMetadata(headlessServiceName, namespace, commonLabels)
	headlessServiceSpec := corev1.ServiceSpec{
        Selector: selectorLabels,
        ClusterIP: corev1.ClusterIPNone, // Required
        Type: corev1.ServiceTypeClusterIP, // Required
        Ports: builders.BuildServicePorts(gatewayConfig.Ports), // Needs Ports from config
        // Optional PublishNotReadyAddresses
    }

	// Call generic Headless Service builder
	headlessService := builders.BuildHeadlessService(
		//gatewayConfig.Service, // Config relevant to service overall (e.g. SessionAffinity?)
		headlessServiceMetadata, // Metadata
		selectorLabels,          // Selector
		// Removed passing ServiceConfig *pointer* here, pass required spec values/structs instead.
		builders.BuildServicePorts(gatewayConfig.Ports), // Directly pass ports
	)
	builtObjects = append(builtObjects, headlessService)


	// Build regular Client Service (Optional based on GatewayConfig.Service.Type)
	// Need to check if Type != ClusterIPNone and Ports are defined.
	// Need a function like builders.ShouldBuildClientService(serviceConfig *common.ServiceSpecPart) bool
	if gatewayConfig.Service != nil && builders.ShouldBuildClientService(gatewayConfig.Service) { // Use common builder helper
		regularServiceName := builders.DeriveResourceName(instanceName)
		regularServiceMetadata := builders.BuildServiceMetadata(regularServiceName, namespace, commonLabels)

		// Build regular Service Spec inputs
		regularServiceSpec := corev1.ServiceSpec{
             Selector: selectorLabels,
             Type: *gatewayConfig.Service.Type, // Get type from config (dereference pointer)
             Ports: builders.BuildServicePorts(gatewayConfig.Service.Ports),
             // Add other standard ServiceSpec fields from common.ServiceSpecPart if they exist
             // ClusterIP: gatewayConfig.Service.ClusterIP, // if field exists
             // SessionAffinity: gatewayConfig.Service.SessionAffinity,
        }

		// Handle NodePort overrides in the Spec if Type is NodePort and NodePorts are defined in common config.
		// BuildService builder might need to handle applying NodePort values based on type.
		// Alternatively, pass the node port map/info as parameter to generic BuildService.

		// Call generic Service builder
		clientService := builders.BuildService(
			//gatewayConfig.Service, // Removed pointer
			regularServiceMetadata, // Metadata
			selectorLabels,        // Selector
            regularServiceSpec, // Build spec struct here
		)
		builtObjects = append(builtObjects, clientService)
	}


	// 4. Build ConfigMaps from AppConfigData
	// Call generic ConfigMap builder, pass resolved data.
	if gatewayConfig.AppConfigData != nil && len(gatewayConfig.AppConfigData) > 0 {
		configMapResourceName := builders.DeriveResourceName(instanceName) + "-config" // Convention
		cmObjects, err := builders.BuildConfigMapsFromAppData(gatewayConfig.AppConfigData, configMapResourceName, namespace, commonLabels) // Needs BuildConfigMapsFromAppData helper
		if err != nil { return nil, fmtf.Errorf("failed to build ConfigMaps from AppConfigData: %w", err)}
		builtObjects = append(builtObjects, cmObjects...) // Append the built ConfigMaps
	}
	// TODO: Build Secrets from AppConfigData if needed. builders.BuildSecretsFromAppData(...)


	// 5. Build Service Account object (if create=true)
	// Call generic ServiceAccount builder.
	if gatewayConfig.ServiceAccount != nil && builders.GetBoolPtrValueOrDefault(gatewayConfig.ServiceAccount.Create, true) { // Check create flag with default
		serviceAccount := builders.BuildServiceAccount(
			builders.DeriveServiceAccountName(instanceName, gatewayConfig.ServiceAccount), // Derived name
			namespace,
			builders.GetServiceAccountAnnotations(gatewayConfig.ServiceAccount), // Annotations from config
			commonLabels, // Common labels
		) // Needs builders.BuildServiceAccount
		builtObjects = append(builtObjects, serviceAccount)
	}

	// 6. Build PDB object (if config exists)
	// Call generic PDB builder.
	// if gatewayConfig.Pdb != nil {
	//     pdbObject := builders.BuildPodDisruptionBudget(gatewayConfig.Pdb, builders.DeriveResourceName(instanceName) + "-pdb", namespace, commonLabels, selectorLabels) // Needs Pdb builder
	//     builtObjects = append(builtObjects, pdbObject)
	// }


	// --- Return the final list of K8s objects ---
	return builtObjects, nil // Success!
}