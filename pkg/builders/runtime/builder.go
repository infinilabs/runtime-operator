// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Runtime Operator is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package runtime

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appv1 "github.com/infinilabs/runtime-operator/api/app/v1"
	"github.com/infinilabs/runtime-operator/pkg/apis/common"
	commonutil "github.com/infinilabs/runtime-operator/pkg/apis/common/util"

	builders "github.com/infinilabs/runtime-operator/pkg/builders/k8s"

	"github.com/infinilabs/runtime-operator/pkg/strategy"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	StatefulSetType = "StatefulSet"
)

// Ensure our builder implementation complies with the strategy interface
var _ strategy.AppBuilderStrategy = &RuntimeBuilderStrategy{}

type RuntimeBuilderStrategy struct{}

var workloadGVK = schema.FromAPIVersionAndKind("apps/v1", StatefulSetType)

// init registers this specific builder strategy with the central registry.
// This function runs automatically when this package is imported.
func init() {
	// 默认注册operator策略
	strategy.RegisterAppBuilderStrategy("operator", &RuntimeBuilderStrategy{})
}

// GetWorkloadGVK implements the AppBuilderStrategy interface.
func (b *RuntimeBuilderStrategy) GetWorkloadGVK() schema.GroupVersionKind {
	return workloadGVK
}

func (b *RuntimeBuilderStrategy) verifyParameters(runtimeConfig *common.RuntimeConfig, appComp *appv1.ApplicationComponent) error {
	if runtimeConfig == nil {
		return fmt.Errorf("runtime configuration (properties) is mandatory but missing or empty for component '%s'", appComp.Name)
	}

	// Perform validation of the specific RuntimeConfig structure.
	if runtimeConfig.Replicas == nil {
		return fmt.Errorf("runtime config missing required 'replicas' for component '%s'", appComp.Name)
	}
	if runtimeConfig.Image == nil || (runtimeConfig.Image.Repository == "" && runtimeConfig.Image.Tag == "") {
		return fmt.Errorf("runtime config is missing required 'image' for component '%s'", appComp.Name)
	}
	if runtimeConfig.Ports == nil || len(runtimeConfig.Ports) == 0 {
		return fmt.Errorf("runtime config is missing required 'ports' configuration for component '%s'", appComp.Name)
	}
	isStatefulSet := b.GetWorkloadGVK().Kind == StatefulSetType
	if isStatefulSet {
		if runtimeConfig.Storage == nil || !runtimeConfig.Storage.Enabled {
			return fmt.Errorf("runtime workload is StatefulSet but Storage configuration is missing or disabled for component '%s'", appComp.Name)
		}
		if runtimeConfig.Storage.Enabled && runtimeConfig.Storage.Size == nil {
			return fmt.Errorf("runtime Storage is enabled but 'size' is missing for component '%s'", appComp.Name)
		}
		if runtimeConfig.Storage.Enabled && runtimeConfig.Storage.MountPath == "" {
			return fmt.Errorf("runtime Storage is enabled but 'mountPath' is missing for component '%s'", appComp.Name)
		}
	}

	return nil
}

// BuildObjects implements the AppBuilderStrategy interface.
func (b *RuntimeBuilderStrategy) BuildObjects(ctx context.Context, k8sClient client.Client,
	scheme *runtime.Scheme, owner client.Object, appDef *appv1.ApplicationDefinition,
	appComp *appv1.ApplicationComponent, appSpecificConfig interface{}) ([]client.Object, error) {
	logger := log.FromContext(ctx).WithValues("component", appComp.Name, "type", appComp.Type, "builder", "runtime")

	// --- Unmarshal and Validate Specific Configuration ---
	runtimeConfig, ok := appSpecificConfig.(*common.RuntimeConfig)
	if !ok {
		return nil, fmt.Errorf("internal error: expected *common.RuntimeConfig for component '%s' but received type %T", appComp.Name, appSpecificConfig)
	}

	// Perform validation of the specific RuntimeConfig structure.
	if err := b.verifyParameters(runtimeConfig, appComp); err != nil {
		return nil, fmt.Errorf("failed to validate RuntimeConfig for component '%s': %w", appComp.Name, err)
	}
	// TODO: Add more specific validation if needed

	logger.V(1).Info("Validated RuntimeConfig successfully")

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
	replicas := commonutil.GetInt32ValueOrDefault(runtimeConfig.Replicas, 1)

	mainContainerSpec, err := buildRuntimeMainContainerSpec(runtimeConfig, instanceName)
	if err != nil {
		return nil, fmt.Errorf("failed to build runtime main container spec for %s: %w", instanceName, err)
	}
	mainContainer := *mainContainerSpec

	initContainers := buildRuntimeInitContainers(runtimeConfig, instanceName)
	logger.V(1).Info("Built init containers", "count", len(initContainers))

	volumes := buildRuntimeVolumes(runtimeConfig, instanceName)
	logger.V(1).Info("Built volumes", "count", len(volumes))

	mainContainerVolumeMounts := buildRuntimeVolumeMounts(runtimeConfig, instanceName)
	logger.V(1).Info("Built main container volume mounts", "count", len(mainContainerVolumeMounts))
	mainContainer.VolumeMounts = mainContainerVolumeMounts // Attach mounts

	nodeSelector := runtimeConfig.NodeSelector
	tolerations := runtimeConfig.Tolerations
	affinity := builders.GetAffinityOrDefault(runtimeConfig.Affinity)
	podSecurityContext := builders.GetPodSecurityContextOrDefault(runtimeConfig.PodSecurityContext)
	serviceAccountConfig := runtimeConfig.ServiceAccount
	serviceAccountName := builders.DeriveServiceAccountName(instanceName, serviceAccountConfig)

	podLabels := builders.MergeMaps(commonLabels, selectorLabels)
	var podAnnotations = map[string]string{}
	// --- 5. Build ConfigMaps/Secrets from Config File Data ---
	if runtimeConfig.ConfigFiles != nil && len(runtimeConfig.ConfigFiles) > 0 {
		configMapResourceName := resourceName + "-config"
		logger.V(1).Info("Building ConfigMap object", "name", configMapResourceName)
		cmObjects, err := builders.BuildConfigMapsFromAppData(runtimeConfig.ConfigFiles, configMapResourceName, namespace, commonLabels)
		if err != nil {
			return nil, fmt.Errorf("failed to build ConfigMaps from ConfigFiles for %s: %w", instanceName, err)
		}
		
		// Check if we need to restart the pod based on ConfigMap changes
		var needRestart bool
		for _, cmObj := range cmObjects {
			cm, ok := cmObj.(*corev1.ConfigMap)
			if ok {
				hashV, err := builders.HashConfigMap(cm) // Hash the ConfigMap data for consistency
				if err != nil {
					return nil, fmt.Errorf("failed to hash ConfigMap data for %s: %w", instanceName, err)
				}
				if oldCv, ok := appDef.Status.Annotations[cm.GetName()]; !ok || oldCv != hashV {
					needRestart = true // If hash changed, we need to restart the pod
					if appDef.Status.Annotations == nil {
						appDef.Status.Annotations = make(map[string]string) // Initialize if nil
					}
					appDef.Status.Annotations[cm.GetName()] = hashV // Update the annotation with new hash
				}
			}
		}
		if needRestart {
			logger.V(1).Info("ConfigMap change detected, marking pod for restart")
			// Add annotation to trigger pod rolling restart
			podAnnotations["runtime-operator/restartedAt"] = time.Now().Format(time.RFC3339) // Add restart timestamp annotation
		}
		builtObjects = append(builtObjects, cmObjects...)
		logger.V(1).Info("Successfully built ConfigMap object(s)")
		// TODO: Handle Secrets similarly if needed
	}

	logger.V(1).Info("Building PodTemplateSpec")
	builtPodTemplateSpec, err := builders.BuildPodTemplateSpec(
		[]corev1.Container{mainContainer}, initContainers, volumes,
		podSecurityContext, serviceAccountName, nodeSelector, tolerations, affinity,
		podLabels, podAnnotations,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build final PodTemplateSpec for runtime %s: %w", instanceName, err)
	}
	logger.V(1).Info("Successfully built PodTemplateSpec")

	// --- 2. Build primary workload resource (StatefulSet) ---
	stsMetadata := builders.BuildObjectMeta(resourceName, namespace, commonLabels, nil)

	vctList, err := builders.BuildVolumeClaimTemplates(runtimeConfig.Storage, commonLabels)
	if err != nil {
		return nil, fmt.Errorf("failed to build VCTs for Runtime %s: %w", instanceName, err)
	}
	logger.V(1).Info("Built VolumeClaimTemplates", "count", len(vctList))

	stsUpdateStrategy := builders.GetStatefulSetUpdateStrategyOrDefault(runtimeConfig.StatefulSetUpdateStrategy)
	stsPodManagementPolicy := builders.GetStatefulSetPodManagementPolicyOrDefault(runtimeConfig.PodManagementPolicy)

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
	headlessServicePorts := builders.BuildServicePorts(runtimeConfig.Ports)
	logger.V(1).Info("Building Headless Service object", "name", headlessServiceName)
	headlessService := builders.BuildHeadlessService(headlessServiceMetadata, selectorLabels, headlessServicePorts)
	builtObjects = append(builtObjects, headlessService)
	logger.V(1).Info("Successfully built Headless Service object")

	// --- 4. Build Client/Transport Service (Optional) ---
	if runtimeConfig.Service != nil && ShouldBuildClientService(runtimeConfig.Service) { // Use local helper
		regularServiceName := resourceName
		regularServiceMetadata := builders.BuildObjectMeta(regularServiceName, namespace, commonLabels, runtimeConfig.Service.Annotations)
		clientServicePorts := builders.BuildServicePorts(runtimeConfig.Service.Ports)

		var serviceType corev1.ServiceType = corev1.ServiceTypeClusterIP // Default
		if runtimeConfig.Service.Type != nil {
			serviceType = *runtimeConfig.Service.Type
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
	isStatefulSet := workloadGVK.Kind == StatefulSetType
	if !isStatefulSet && runtimeConfig.Persistence != nil && runtimeConfig.Persistence.Enabled {
		pvcName := builders.DeriveResourceName(instanceName) + "-pvc"
		logger.V(1).Info("Building shared PersistentVolumeClaim object", "name", pvcName)
		pvcObject, err := builders.BuildSharedPVCPVC(runtimeConfig.Persistence, instanceName, namespace, commonLabels)
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

	logger.Info("Finished building all Kubernetes objects for Runtime", "count", len(builtObjects))
	return builtObjects, nil // Success!
}

// buildRuntimeMainContainerSpec builds the corev1.Container spec for the main Runtime app.
func buildRuntimeMainContainerSpec(runtimeConfig *common.RuntimeConfig, instanceName string) (*corev1.Container, error) {
	logger := log.Log.WithName("runtime-container-builder")

	imageName := builders.BuildImageName(runtimeConfig.Image.Repository, runtimeConfig.Image.Tag)
	imagePullPolicy := builders.GetImagePullPolicy(runtimeConfig.Image.PullPolicy, runtimeConfig.Image.Tag)
	k8sPorts := builders.BuildContainerPorts(runtimeConfig.Ports)
	k8sResources := builders.BuildK8sResourceRequirements(runtimeConfig.Resources)

	var livenessProbe, readinessProbe, startupProbe *corev1.Probe
	if runtimeConfig.Probes != nil {
		livenessProbe = builders.BuildProbe(runtimeConfig.Probes.Liveness)
		readinessProbe = builders.BuildProbe(runtimeConfig.Probes.Readiness)
		startupProbe = builders.BuildProbe(runtimeConfig.Probes.Startup)
	}
	containerSecurityContext := builders.GetContainerSecurityContextOrDefault(runtimeConfig.ContainerSecurityContext)

	container := corev1.Container{
		Name:            builders.DeriveContainerName(instanceName),
		Image:           imageName,
		ImagePullPolicy: imagePullPolicy,
		Command:         runtimeConfig.Command, // Add if defined in common.RuntimeConfig
		Args:            runtimeConfig.Args,    // Add if defined in common.RuntimeConfig
		Ports:           k8sPorts,
		Env:             runtimeConfig.Env,
		EnvFrom:         runtimeConfig.EnvFrom,
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

// buildRuntimeInitContainers builds necessary init containers for the Runtime.
func buildRuntimeInitContainers(runtimeConfig *common.RuntimeConfig, instanceName string) []corev1.Container {
	// Add custom init containers if defined
	if runtimeConfig.InitContainer == nil {
		return nil
	}

	initContainers := []corev1.Container{}
	logger := log.Log.WithName("runtime-init-builder").WithValues("instance", instanceName)
	isStatefulSet := (workloadGVK.Kind == StatefulSetType)

	var persistentMountPath string
	var persistentVolumeName string
	ownerUID, ownerGID := int64(1000), int64(1000) // Defaults

	if isStatefulSet && runtimeConfig.Storage != nil && runtimeConfig.Storage.Enabled {
		persistentMountPath = runtimeConfig.Storage.MountPath
		persistentVolumeName = commonutil.GetStringValueOrDefault(&runtimeConfig.Storage.VolumeClaimTemplateName, "data")
		logger.V(1).Info("StatefulSet storage detected", "mountPath", persistentMountPath, "volumeName(VCT)", persistentVolumeName)
	} else if !isStatefulSet && runtimeConfig.Persistence != nil && runtimeConfig.Persistence.Enabled {
		persistentMountPath = runtimeConfig.Persistence.MountPath
		persistentVolumeName = commonutil.GetStringValueOrDefault(&runtimeConfig.Persistence.VolumeName, builders.DeriveResourceName(instanceName)+"-pvc-vol")
		logger.V(1).Info("Deployment persistence detected", "mountPath", persistentMountPath, "volumeName", persistentVolumeName)
	}

	if runtimeConfig.PodSecurityContext != nil && runtimeConfig.PodSecurityContext.FSGroup != nil {
		ownerGID = *runtimeConfig.PodSecurityContext.FSGroup
	}
	if runtimeConfig.ContainerSecurityContext != nil {
		if runtimeConfig.ContainerSecurityContext.RunAsUser != nil {
			ownerUID = *runtimeConfig.ContainerSecurityContext.RunAsUser
		}
		if runtimeConfig.ContainerSecurityContext.RunAsGroup != nil {
			ownerGID = *runtimeConfig.ContainerSecurityContext.RunAsGroup
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

// buildRuntimeVolumes builds the PodSpec.Volumes list (excluding VCTs).
func buildRuntimeVolumes(runtimeConfig *common.RuntimeConfig, instanceName string) []corev1.Volume {
	volumes := []corev1.Volume{}
	logger := log.Log.WithName("runtime-volume-builder").WithValues("instance", instanceName)
	isStatefulSet := (workloadGVK.Kind == StatefulSetType)

	cmVolumes := builders.BuildVolumesFromConfigMaps(runtimeConfig.ConfigMounts)
	volumes = append(volumes, cmVolumes...)
	logger.V(1).Info("Built volumes from ConfigMounts", "count", len(cmVolumes))

	secretVolumes := builders.BuildVolumesFromSecrets(runtimeConfig.SecretMounts)
	volumes = append(volumes, secretVolumes...)
	logger.V(1).Info("Built volumes from SecretMounts", "count", len(secretVolumes))

	if !isStatefulSet && runtimeConfig.Persistence != nil && runtimeConfig.Persistence.Enabled {
		pvcVolumeName := commonutil.GetStringValueOrDefault(&runtimeConfig.Persistence.VolumeName, builders.DeriveResourceName(instanceName)+"-pvc-vol")
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

	return volumes
}

// buildRuntimeVolumeMounts builds the main container's VolumeMounts list.
func buildRuntimeVolumeMounts(runtimeConfig *common.RuntimeConfig, instanceName string) []corev1.VolumeMount {
	allVolumeMounts := []corev1.VolumeMount{}
	logger := log.Log.WithName("runtime-mount-builder").WithValues("instance", instanceName)
	isStatefulSet := (workloadGVK.Kind == StatefulSetType)

	cmMounts := builders.BuildVolumeMountsFromConfigMaps(runtimeConfig.ConfigMounts)
	allVolumeMounts = append(allVolumeMounts, cmMounts...)
	logger.V(1).Info("Built volume mounts from ConfigMounts", "count", len(cmMounts))

	secretMounts := builders.BuildVolumeMountsFromSecrets(runtimeConfig.SecretMounts)
	allVolumeMounts = append(allVolumeMounts, secretMounts...)
	logger.V(1).Info("Built volume mounts from SecretMounts", "count", len(secretMounts))

	if !isStatefulSet && runtimeConfig.Persistence != nil && runtimeConfig.Persistence.Enabled {
		persistenceVolumeName := commonutil.GetStringValueOrDefault(&runtimeConfig.Persistence.VolumeName, builders.DeriveResourceName(instanceName)+"-pvc-vol")
		persistentMounts := builders.BuildPersistentVolumeMounts(runtimeConfig.Persistence, persistenceVolumeName)
		allVolumeMounts = append(allVolumeMounts, persistentMounts...)
		logger.V(1).Info("Built volume mounts from Persistence", "count", len(persistentMounts))
	}

	if isStatefulSet && runtimeConfig.Storage != nil && runtimeConfig.Storage.Enabled {
		storageMounts := builders.BuildVolumeMountsFromStorage(runtimeConfig.Storage)
		allVolumeMounts = append(allVolumeMounts, storageMounts...)
		logger.V(1).Info("Built volume mounts from Storage", "count", len(storageMounts))
	}

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
		return true
	}
	// If Type is NodePort, LoadBalancer, or nil (defaults to ClusterIP), build it.
	// Explicitly check against ClusterIPNone just in case type was set but ClusterIP wasn't
	if svcConfig.Type != nil && *svcConfig.Type == corev1.ClusterIPNone { // Explicit check against "None" string constant
		return false
	}

	return true
}
