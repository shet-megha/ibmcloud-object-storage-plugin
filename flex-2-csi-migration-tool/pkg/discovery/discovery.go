package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DiscoverAllFlexResources discovers all resources using Flex PVCs
func (d *ResourceDiscovery) DiscoverAllFlexResources(namespace string) ([]ResourceInfo, error) {
	// Step 1: Find all Flex PVs
	flexPVs, err := d.findFlexPVs()
	if err != nil {
		return nil, fmt.Errorf("failed to find Flex PVs: %v", err)
	}

	if len(flexPVs) == 0 {
		fmt.Println("✅ No Flex PVs found - all resources are using CSI!")
		return []ResourceInfo{}, nil
	}

	// Step 2: Map PVs to PVCs
	pvcMap, err := d.mapPVsToPVCs(flexPVs)
	if err != nil {
		return nil, fmt.Errorf("failed to map PVs to PVCs: %v", err)
	}

	// Step 3: Find resources using these PVCs
	resources, err := d.findResourcesUsingPVCs(pvcMap, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find resources: %v", err)
	}

	// Step 4: Also discover workloads by naming convention (not ending with -csi-migrated)
	// This handles cases where PVCs exist but workloads don't use them yet
	namingResources, err := d.findResourcesByNamingConvention(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find resources by naming: %v", err)
	}

	// Step 5: Find orphaned Flex PVCs (PVCs not used by any workload)
	orphanedPVCs, err := d.findOrphanedFlexPVCs(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find orphaned PVCs: %v", err)
	}

	// Merge results, avoiding duplicates
	resourceMap := make(map[string]ResourceInfo)
	for _, r := range resources {
		key := fmt.Sprintf("%s/%s/%s", r.Namespace, r.Type, r.Name)
		resourceMap[key] = r
	}
	for _, r := range namingResources {
		key := fmt.Sprintf("%s/%s/%s", r.Namespace, r.Type, r.Name)
		if _, exists := resourceMap[key]; !exists {
			resourceMap[key] = r
		}
	}

	// Add orphaned PVCs with unique keys based on PVC name
	for _, r := range orphanedPVCs {
		if len(r.PVCs) > 0 {
			key := fmt.Sprintf("%s/OrphanedPVC/%s", r.Namespace, r.PVCs[0].Name)
			resourceMap[key] = r
		}
	}

	finalResources := make([]ResourceInfo, 0, len(resourceMap))
	for _, r := range resourceMap {
		finalResources = append(finalResources, r)
	}

	return finalResources, nil
}

func (d *ResourceDiscovery) DiscoverMigratedResources(namespace string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	searchNamespace := namespace
	if searchNamespace == "" {
		searchNamespace = metav1.NamespaceAll
	}

	deployments, err := d.clientset.AppsV1().Deployments(searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range deployments.Items {
		deploy := &deployments.Items[i]
		if !strings.HasSuffix(deploy.Name, "-csi-migrated") {
			continue
		}
		resourceInfo, err := d.buildResourceInfo(deploy, "Deployment", deploy.Namespace)
		if err == nil {
			resources = append(resources, *resourceInfo)
		}
	}

	statefulsets, err := d.clientset.AppsV1().StatefulSets(searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range statefulsets.Items {
		sts := &statefulsets.Items[i]
		if !strings.HasSuffix(sts.Name, "-csi-migrated") {
			continue
		}
		resourceInfo, err := d.buildResourceInfo(sts, "StatefulSet", sts.Namespace)
		if err == nil {
			resources = append(resources, *resourceInfo)
		}
	}

	daemonsets, err := d.clientset.AppsV1().DaemonSets(searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range daemonsets.Items {
		ds := &daemonsets.Items[i]
		if !strings.HasSuffix(ds.Name, "-csi-migrated") {
			continue
		}
		resourceInfo, err := d.buildResourceInfo(ds, "DaemonSet", ds.Namespace)
		if err == nil {
			resources = append(resources, *resourceInfo)
		}
	}

	pods, err := d.clientset.CoreV1().Pods(searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if len(pod.OwnerReferences) > 0 {
			continue
		}
		if !strings.HasSuffix(pod.Name, "-csi-migrated") {
			continue
		}
		resourceInfo, err := d.buildResourceInfo(pod, "Pod", pod.Namespace)
		if err == nil {
			resources = append(resources, *resourceInfo)
		}
	}

	return resources, nil
}

// findFlexPVs finds all PVs using FlexVolume driver
func (d *ResourceDiscovery) findFlexPVs() (map[string]*corev1.PersistentVolume, error) {
	pvs, err := d.clientset.CoreV1().PersistentVolumes().List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	flexPVs := make(map[string]*corev1.PersistentVolume)

	for i := range pvs.Items {
		pv := &pvs.Items[i]

		// Check if it's a Flex volume
		if pv.Spec.FlexVolume != nil &&
			(strings.Contains(pv.Spec.FlexVolume.Driver, "ibm/ibmc-s3fs") ||
				strings.Contains(pv.Spec.FlexVolume.Driver, "ibm-s3fs")) {
			flexPVs[pv.Name] = pv
		}
	}

	return flexPVs, nil
}

// mapPVsToPVCs maps PVs to their PVCs
func (d *ResourceDiscovery) mapPVsToPVCs(flexPVs map[string]*corev1.PersistentVolume) (map[string]*PVCInfo, error) {
	pvcMap := make(map[string]*PVCInfo)

	for pvName, pv := range flexPVs {
		if pv.Spec.ClaimRef == nil {
			continue
		}

		pvcNamespace := pv.Spec.ClaimRef.Namespace
		pvcName := pv.Spec.ClaimRef.Name

		// Get the PVC
		pvc, err := d.clientset.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(d.ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		// Skip if PVC is not bound
		if pvc.Status.Phase != corev1.ClaimBound {
			continue
		}

		// Extract secret name from annotations
		secretName := ""
		if pvc.Annotations != nil {
			secretName = pvc.Annotations["ibm.io/secret-name"]
		}

		if secretName == "" {
			continue
		}

		storageClass := ""
		if pvc.Spec.StorageClassName != nil {
			storageClass = *pvc.Spec.StorageClassName
		}

		pvcKey := fmt.Sprintf("%s/%s", pvcNamespace, pvcName)
		pvcMap[pvcKey] = &PVCInfo{
			Name:         pvcName,
			Namespace:    pvcNamespace,
			SecretName:   secretName,
			PVName:       pvName,
			StorageClass: &storageClass,
			AccessModes:  pvc.Spec.AccessModes,
			Storage:      pvc.Spec.Resources.Requests.Storage().String(),
			Annotations:  pvc.Annotations,
			Status:       string(pvc.Status.Phase),
		}

	}

	return pvcMap, nil
}

// findResourcesUsingPVCs finds all resources using the given PVCs
func (d *ResourceDiscovery) findResourcesUsingPVCs(pvcMap map[string]*PVCInfo, namespace string) ([]ResourceInfo, error) {
	searchNamespace := namespace
	if searchNamespace == "" {
		searchNamespace = metav1.NamespaceAll
	}

	cache, err := d.buildDiscoveryCache(namespace)
	if err != nil {
		return nil, err
	}

	return d.findResourcesUsingPVCsCached(pvcMap, cache.withSearchNamespace(searchNamespace))
}

func (d *ResourceDiscovery) findResourcesUsingPVCsCached(pvcMap map[string]*PVCInfo, cache *discoveryCache) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	deployments, err := d.findDeploymentsUsingPVCsCached(pvcMap, cache)
	if err != nil {
		return nil, err
	}
	resources = append(resources, deployments...)

	statefulsets, err := d.findStatefulSetsUsingPVCsCached(pvcMap, cache)
	if err != nil {
		return nil, err
	}
	resources = append(resources, statefulsets...)

	daemonsets, err := d.findDaemonSetsUsingPVCsCached(pvcMap, cache)
	if err != nil {
		return nil, err
	}
	resources = append(resources, daemonsets...)

	pods, err := d.findPodsUsingPVCsCached(pvcMap, cache)
	if err != nil {
		return nil, err
	}
	resources = append(resources, pods...)

	filteredResources := make([]ResourceInfo, 0, len(resources))
	for _, resource := range resources {
		if d.migratedResourceExists(resource.Name+"-csi-migrated", resource.Namespace, cache) {
			continue
		}
		filteredResources = append(filteredResources, resource)
	}

	return filteredResources, nil
}

// findDeploymentsUsingPVCs finds deployments using Flex PVCs
func (d *ResourceDiscovery) findDeploymentsUsingPVCs(pvcMap map[string]*PVCInfo, namespace string) ([]ResourceInfo, error) {
	cache := (&discoveryCache{searchNamespace: namespace}).withSearchNamespace(namespace)
	return d.findDeploymentsUsingPVCsCached(pvcMap, cache)
}

func (d *ResourceDiscovery) findDeploymentsUsingPVCsCached(pvcMap map[string]*PVCInfo, cache *discoveryCache) ([]ResourceInfo, error) {
	deployments, err := d.clientset.AppsV1().Deployments(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var resources []ResourceInfo

	for i := range deployments.Items {
		deploy := &deployments.Items[i]

		if strings.HasSuffix(deploy.Name, "-csi-migrated") {
			continue
		}

		pvcs := d.extractPVCsFromVolumes(deploy.Spec.Template.Spec.Volumes, deploy.Namespace, pvcMap)
		if len(pvcs) > 0 {
			services := d.findServicesForResourceCached(deploy.Namespace, deploy.Spec.Selector, cache)

			resources = append(resources, ResourceInfo{
				Name:      deploy.Name,
				Namespace: deploy.Namespace,
				Type:      "Deployment",
				PVCs:      pvcs,
				Services:  services,
				Object:    deploy,
			})

		}
	}

	return resources, nil
}

// findStatefulSetsUsingPVCs finds statefulsets using Flex PVCs
func (d *ResourceDiscovery) findStatefulSetsUsingPVCs(pvcMap map[string]*PVCInfo, namespace string) ([]ResourceInfo, error) {
	cache := (&discoveryCache{searchNamespace: namespace}).withSearchNamespace(namespace)
	return d.findStatefulSetsUsingPVCsCached(pvcMap, cache)
}

func (d *ResourceDiscovery) findStatefulSetsUsingPVCsCached(pvcMap map[string]*PVCInfo, cache *discoveryCache) ([]ResourceInfo, error) {
	statefulsets, err := d.clientset.AppsV1().StatefulSets(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var resources []ResourceInfo

	for i := range statefulsets.Items {
		sts := &statefulsets.Items[i]

		if strings.HasSuffix(sts.Name, "-csi-migrated") {
			continue
		}

		pvcs := d.extractPVCsFromVolumes(sts.Spec.Template.Spec.Volumes, sts.Namespace, pvcMap)
		if len(pvcs) > 0 {
			services := d.findServicesForResourceCached(sts.Namespace, sts.Spec.Selector, cache)

			resources = append(resources, ResourceInfo{
				Name:      sts.Name,
				Namespace: sts.Namespace,
				Type:      "StatefulSet",
				PVCs:      pvcs,
				Services:  services,
				Object:    sts,
			})

		}
	}

	return resources, nil
}

// findDaemonSetsUsingPVCs finds daemonsets using Flex PVCs
func (d *ResourceDiscovery) findDaemonSetsUsingPVCs(pvcMap map[string]*PVCInfo, namespace string) ([]ResourceInfo, error) {
	cache := (&discoveryCache{searchNamespace: namespace}).withSearchNamespace(namespace)
	return d.findDaemonSetsUsingPVCsCached(pvcMap, cache)
}

func (d *ResourceDiscovery) findDaemonSetsUsingPVCsCached(pvcMap map[string]*PVCInfo, cache *discoveryCache) ([]ResourceInfo, error) {
	daemonsets, err := d.clientset.AppsV1().DaemonSets(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var resources []ResourceInfo

	for i := range daemonsets.Items {
		ds := &daemonsets.Items[i]

		if strings.HasSuffix(ds.Name, "-csi-migrated") {
			continue
		}

		pvcs := d.extractPVCsFromVolumes(ds.Spec.Template.Spec.Volumes, ds.Namespace, pvcMap)
		if len(pvcs) > 0 {
			services := d.findServicesForResourceCached(ds.Namespace, ds.Spec.Selector, cache)

			resources = append(resources, ResourceInfo{
				Name:      ds.Name,
				Namespace: ds.Namespace,
				Type:      "DaemonSet",
				PVCs:      pvcs,
				Services:  services,
				Object:    ds,
			})

		}
	}

	return resources, nil
}

// findPodsUsingPVCs finds standalone pods using Flex PVCs
func (d *ResourceDiscovery) findPodsUsingPVCs(pvcMap map[string]*PVCInfo, namespace string) ([]ResourceInfo, error) {
	cache := (&discoveryCache{searchNamespace: namespace}).withSearchNamespace(namespace)
	return d.findPodsUsingPVCsCached(pvcMap, cache)
}

func (d *ResourceDiscovery) findPodsUsingPVCsCached(pvcMap map[string]*PVCInfo, cache *discoveryCache) ([]ResourceInfo, error) {
	pods, err := d.clientset.CoreV1().Pods(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var resources []ResourceInfo

	for i := range pods.Items {
		pod := &pods.Items[i]

		if len(pod.OwnerReferences) > 0 {
			continue
		}

		if strings.HasSuffix(pod.Name, "-csi-migrated") {
			continue
		}

		pvcs := d.extractPVCsFromVolumes(pod.Spec.Volumes, pod.Namespace, pvcMap)
		if len(pvcs) > 0 {
			var selector *metav1.LabelSelector
			if len(pod.Labels) > 0 {
				selector = &metav1.LabelSelector{
					MatchLabels: pod.Labels,
				}
			}

			services := d.findServicesForResourceCached(pod.Namespace, selector, cache)

			resources = append(resources, ResourceInfo{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Type:      "Pod",
				PVCs:      pvcs,
				Services:  services,
				Object:    pod,
			})

		}
	}

	return resources, nil
}

// extractPVCsFromVolumes extracts PVC information from volumes
func (d *ResourceDiscovery) extractPVCsFromVolumes(volumes []corev1.Volume, namespace string, pvcMap map[string]*PVCInfo) []PVCInfo {
	var pvcs []PVCInfo

	for _, vol := range volumes {
		if vol.PersistentVolumeClaim != nil {
			pvcKey := fmt.Sprintf("%s/%s", namespace, vol.PersistentVolumeClaim.ClaimName)
			if pvcInfo, exists := pvcMap[pvcKey]; exists {
				pvcs = append(pvcs, *pvcInfo)
			}
		}
	}

	return pvcs
}

// findServicesForResource finds services pointing to a resource
func (d *ResourceDiscovery) findServicesForResource(resourceName, namespace string, selector *metav1.LabelSelector) []string {
	return d.findServicesForResourceCached(namespace, selector, nil)
}

func (d *ResourceDiscovery) findServicesForResourceCached(namespace string, selector *metav1.LabelSelector, cache *discoveryCache) []string {
	var services []corev1.Service
	if cache != nil {
		services = cache.servicesByNamespace[namespace]
	} else {
		serviceList, err := d.clientset.CoreV1().Services(namespace).List(d.ctx, metav1.ListOptions{})
		if err != nil {
			return []string{}
		}
		services = serviceList.Items
	}

	var serviceNames []string

	for _, svc := range services {
		if svc.Spec.Selector == nil {
			continue
		}

		if selector != nil && selector.MatchLabels != nil {
			matches := true
			for key, value := range svc.Spec.Selector {
				if labelValue, exists := selector.MatchLabels[key]; !exists || labelValue != value {
					matches = false
					break
				}
			}
			if matches {
				serviceNames = append(serviceNames, svc.Name)
			}
		}
	}

	return serviceNames
}

// buildResourceInfo builds ResourceInfo from a Kubernetes object
func (d *ResourceDiscovery) buildResourceInfo(obj interface{}, resourceType, namespace string) (*ResourceInfo, error) {
	var volumes []corev1.Volume
	var name string
	var selector *metav1.LabelSelector

	switch resourceType {
	case "Deployment":
		deploy := obj.(*appsv1.Deployment)
		volumes = deploy.Spec.Template.Spec.Volumes
		name = deploy.Name
		selector = deploy.Spec.Selector
	case "StatefulSet":
		sts := obj.(*appsv1.StatefulSet)
		volumes = sts.Spec.Template.Spec.Volumes
		name = sts.Name
		selector = sts.Spec.Selector
	case "DaemonSet":
		ds := obj.(*appsv1.DaemonSet)
		volumes = ds.Spec.Template.Spec.Volumes
		name = ds.Name
		selector = ds.Spec.Selector
	case "Pod":
		pod := obj.(*corev1.Pod)
		volumes = pod.Spec.Volumes
		name = pod.Name
		if len(pod.Labels) > 0 {
			selector = &metav1.LabelSelector{MatchLabels: pod.Labels}
		}
	}

	// Get PVC information
	var pvcs []PVCInfo
	for _, vol := range volumes {
		if vol.PersistentVolumeClaim != nil {
			pvcName := vol.PersistentVolumeClaim.ClaimName

			pvc, err := d.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(d.ctx, pvcName, metav1.GetOptions{})
			if err != nil {
				continue
			}

			// For migrated resources, check for CSI annotations
			// For non-migrated resources, check for Flex annotations
			secretName := ""

			if pvc.Annotations != nil {
				// Check for CSI annotation (migrated resources)
				if csiSecret := pvc.Annotations["cos.csi.driver/secret"]; csiSecret != "" {
					secretName = csiSecret
				}
				// Check for Flex annotation (non-migrated resources)
				if flexSecret := pvc.Annotations["ibm.io/secret-name"]; flexSecret != "" {
					secretName = flexSecret
				}
			}

			// Skip if neither CSI nor Flex PVC
			if secretName == "" {
				continue
			}

			storageClass := ""
			if pvc.Spec.StorageClassName != nil {
				storageClass = *pvc.Spec.StorageClassName
			}

			pvcInfo := PVCInfo{
				Name:         pvcName,
				Namespace:    namespace,
				SecretName:   secretName,
				PVName:       pvc.Spec.VolumeName,
				StorageClass: &storageClass,
				AccessModes:  pvc.Spec.AccessModes,
				Storage:      pvc.Spec.Resources.Requests.Storage().String(),
				Annotations:  pvc.Annotations,
				Status:       string(pvc.Status.Phase),
			}

			pvcs = append(pvcs, pvcInfo)
		}
	}

	if len(pvcs) == 0 {
		return nil, fmt.Errorf("no COS PVCs found for resource %s", name)
	}

	services := d.findServicesForResource(name, namespace, selector)

	return &ResourceInfo{
		Name:      name,
		Namespace: namespace,
		Type:      resourceType,
		PVCs:      pvcs,
		Services:  services,
		Object:    obj,
	}, nil
}

// findResourcesByNamingConvention finds resources that don't end with -csi-migrated
// This helps discover Flex resources even when they don't use PVCs yet
func (d *ResourceDiscovery) findResourcesByNamingConvention(namespace string) ([]ResourceInfo, error) {
	cache, err := d.buildDiscoveryCache(namespace)
	if err != nil {
		return nil, err
	}
	return d.findResourcesByNamingConventionCached(cache)
}

func (d *ResourceDiscovery) findResourcesByNamingConventionCached(cache *discoveryCache) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	deployments, err := d.clientset.AppsV1().Deployments(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range deployments.Items {
		deploy := &deployments.Items[i]
		if !strings.HasSuffix(deploy.Name, "-csi-migrated") {
			if d.migratedResourceExists(deploy.Name+"-csi-migrated", deploy.Namespace, cache) {
				continue
			}

			resourceInfo := d.buildResourceInfoFastWithCache(deploy, "Deployment", deploy.Namespace, cache)
			if resourceInfo != nil {
				resources = append(resources, *resourceInfo)
			}
		}
	}

	statefulsets, err := d.clientset.AppsV1().StatefulSets(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range statefulsets.Items {
		sts := &statefulsets.Items[i]
		if !strings.HasSuffix(sts.Name, "-csi-migrated") {
			if d.migratedResourceExists(sts.Name+"-csi-migrated", sts.Namespace, cache) {
				continue
			}

			resourceInfo := d.buildResourceInfoFastWithCache(sts, "StatefulSet", sts.Namespace, cache)
			if resourceInfo != nil {
				resources = append(resources, *resourceInfo)
			}
		}
	}

	daemonsets, err := d.clientset.AppsV1().DaemonSets(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range daemonsets.Items {
		ds := &daemonsets.Items[i]
		if !strings.HasSuffix(ds.Name, "-csi-migrated") {
			if d.migratedResourceExists(ds.Name+"-csi-migrated", ds.Namespace, cache) {
				continue
			}

			resourceInfo := d.buildResourceInfoFastWithCache(ds, "DaemonSet", ds.Namespace, cache)
			if resourceInfo != nil {
				resources = append(resources, *resourceInfo)
			}
		}
	}

	pods, err := d.clientset.CoreV1().Pods(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range pods.Items {
		pod := &pods.Items[i]

		if len(pod.OwnerReferences) > 0 {
			continue
		}

		if !strings.HasSuffix(pod.Name, "-csi-migrated") {
			if d.migratedResourceExists(pod.Name+"-csi-migrated", pod.Namespace, cache) {
				continue
			}

			resourceInfo := d.buildResourceInfoFastWithCache(pod, "Pod", pod.Namespace, cache)
			if resourceInfo != nil {
				resources = append(resources, *resourceInfo)
			}
		}
	}

	return resources, nil
}

// findOrphanedFlexPVCs finds Flex PVCs that are not used by any workload
func (d *ResourceDiscovery) findOrphanedFlexPVCs(namespace string) ([]ResourceInfo, error) {
	searchNamespace := namespace
	if searchNamespace == "" {
		searchNamespace = metav1.NamespaceAll
	}

	flexPVs, err := d.findFlexPVs()
	if err != nil {
		return nil, err
	}

	pvcMap, err := d.mapPVsToPVCs(flexPVs)
	if err != nil {
		return nil, err
	}

	cache, err := d.buildDiscoveryCache(namespace)
	if err != nil {
		return nil, err
	}

	return d.findOrphanedFlexPVCsCached(pvcMap, cache.withSearchNamespace(searchNamespace))
}

func (d *ResourceDiscovery) findOrphanedFlexPVCsCached(pvcMap map[string]*PVCInfo, cache *discoveryCache) ([]ResourceInfo, error) {
	pods, err := d.clientset.CoreV1().Pods(cache.searchNamespace).List(d.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pvcsInUse := make(map[string]bool)
	for _, pod := range pods.Items {
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				pvcKey := fmt.Sprintf("%s/%s", pod.Namespace, vol.PersistentVolumeClaim.ClaimName)
				pvcsInUse[pvcKey] = true
			}
		}
	}

	var orphanedResources []ResourceInfo
	for pvcKey, pvcInfo := range pvcMap {
		if !pvcsInUse[pvcKey] {
			orphanedResources = append(orphanedResources, ResourceInfo{
				Name:      "",
				Namespace: pvcInfo.Namespace,
				Type:      "",
				PVCs:      []PVCInfo{*pvcInfo},
				Services:  []string{},
				Object:    nil,
			})
		}
	}

	return orphanedResources, nil
}

func (c *discoveryCache) withSearchNamespace(searchNamespace string) *discoveryCache {
	if searchNamespace == "" {
		searchNamespace = metav1.NamespaceAll
	}
	if c == nil {
		return &discoveryCache{searchNamespace: searchNamespace}
	}
	c.searchNamespace = searchNamespace
	return c
}

// Made with Bob

type discoveryCache struct {
	pvcsByNamespace     map[string]map[string]*corev1.PersistentVolumeClaim
	servicesByNamespace map[string][]corev1.Service
	migratedNamesByNS   map[string]map[string]struct{}
	deploymentsByNS     map[string][]*appsv1.Deployment
	statefulSetsByNS    map[string][]*appsv1.StatefulSet
	daemonSetsByNS      map[string][]*appsv1.DaemonSet
	podsByNS            map[string][]*corev1.Pod
	standalonePodsByNS  map[string][]*corev1.Pod
	namespaceOrder      []string
	searchNamespace     string
}

const (
	namespaceWorkerPoolSize  = 10
	resourceTypeWorkerPool   = 3
	resourceDetailWorkerPool = 3
)

// ResourceDiscovery provides shared dependencies for discovery flows.
type ResourceDiscovery struct {
	clientset *kubernetes.Clientset
	ctx       context.Context
}

// NewResourceDiscovery creates a discovery helper.
func NewResourceDiscovery(clientset *kubernetes.Clientset) *ResourceDiscovery {
	return &ResourceDiscovery{
		clientset: clientset,
		ctx:       context.Background(),
	}
}

// GetResourceByName returns a normalized ResourceInfo for a workload if it has COS-related PVCs.
func (d *ResourceDiscovery) GetResourceByName(name, namespace string) (*ResourceInfo, error) {
	deploy, err := d.clientset.AppsV1().Deployments(namespace).Get(d.ctx, name, metav1.GetOptions{})
	if err == nil {
		return d.buildResourceInfoFast(deploy, "Deployment", namespace), nil
	}

	sts, err := d.clientset.AppsV1().StatefulSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
	if err == nil {
		return d.buildResourceInfoFast(sts, "StatefulSet", namespace), nil
	}

	ds, err := d.clientset.AppsV1().DaemonSets(namespace).Get(d.ctx, name, metav1.GetOptions{})
	if err == nil {
		return d.buildResourceInfoFast(ds, "DaemonSet", namespace), nil
	}

	pod, err := d.clientset.CoreV1().Pods(namespace).Get(d.ctx, name, metav1.GetOptions{})
	if err == nil {
		return d.buildResourceInfoFast(pod, "Pod", namespace), nil
	}

	return nil, fmt.Errorf("resource %s not found in namespace %s", name, namespace)
}

// ResourceExists checks whether a workload exists and is discoverable.
func (d *ResourceDiscovery) ResourceExists(name, namespace string) bool {
	info, err := d.GetResourceByName(name, namespace)
	return err == nil && info != nil
}

// LegacyAndMigratedResourcesExist checks whether both original and migrated workloads exist.
func (d *ResourceDiscovery) LegacyAndMigratedResourcesExist(name, namespace string) bool {
	return d.ResourceExists(name, namespace) && d.ResourceExists(name+"-csi-migrated", namespace)
}

// buildResourceInfoFast builds normalized resource info from supported workload objects.
func (d *ResourceDiscovery) buildResourceInfoFast(obj interface{}, resourceType, namespace string) *ResourceInfo {
	return d.buildResourceInfoFastWithCache(obj, resourceType, namespace, nil)
}

func (d *ResourceDiscovery) buildResourceInfoFastWithCache(obj interface{}, resourceType, namespace string, cache *discoveryCache) *ResourceInfo {
	var (
		name     string
		pvcs     []PVCInfo
		services []string
		selector map[string]string
	)

	switch v := obj.(type) {
	case *appsv1.Deployment:
		name = v.Name
		pvcs = d.extractPVCsWithDetailsCached(v.Spec.Template.Spec.Volumes, namespace, cache)
		if v.Spec.Selector != nil {
			selector = v.Spec.Selector.MatchLabels
		}
	case *appsv1.StatefulSet:
		name = v.Name
		pvcs = d.extractPVCsWithDetailsCached(v.Spec.Template.Spec.Volumes, namespace, cache)
		if v.Spec.Selector != nil {
			selector = v.Spec.Selector.MatchLabels
		}
	case *appsv1.DaemonSet:
		name = v.Name
		pvcs = d.extractPVCsWithDetailsCached(v.Spec.Template.Spec.Volumes, namespace, cache)
		if v.Spec.Selector != nil {
			selector = v.Spec.Selector.MatchLabels
		}
	case *corev1.Pod:
		name = v.Name
		pvcs = d.extractPVCsWithDetailsCached(v.Spec.Volumes, namespace, cache)
		selector = v.Labels
	default:
		return nil
	}

	if len(pvcs) == 0 {
		return nil
	}

	services = d.findServicesForLabelsCached(namespace, selector, cache)

	return &ResourceInfo{
		Name:      name,
		Namespace: namespace,
		Type:      resourceType,
		PVCs:      pvcs,
		Services:  services,
		Object:    obj,
	}
}

// extractPVCsWithDetails extracts COS-related PVC information from volumes.
func (d *ResourceDiscovery) extractPVCsWithDetails(volumes []corev1.Volume, namespace string) []PVCInfo {
	return d.extractPVCsWithDetailsCached(volumes, namespace, nil)
}

func (d *ResourceDiscovery) extractPVCsWithDetailsCached(volumes []corev1.Volume, namespace string, cache *discoveryCache) []PVCInfo {
	type pvcJob struct {
		index   int
		pvcName string
	}
	type pvcResult struct {
		index int
		info  *PVCInfo
	}

	jobs := make(chan pvcJob, len(volumes))
	results := make(chan pvcResult, len(volumes))

	workerCount := resourceDetailWorkerPool
	if len(volumes) < workerCount {
		workerCount = len(volumes)
	}
	if workerCount == 0 {
		return nil
	}

	var workerWG sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for job := range jobs {
				info := d.buildPVCInfo(namespace, job.pvcName, cache)
				if info != nil {
					results <- pvcResult{index: job.index, info: info}
				}
			}
		}()
	}

	jobIndex := 0
	for _, vol := range volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		jobs <- pvcJob{index: jobIndex, pvcName: vol.PersistentVolumeClaim.ClaimName}
		jobIndex++
	}
	close(jobs)

	go func() {
		workerWG.Wait()
		close(results)
	}()

	ordered := make(map[int]PVCInfo, jobIndex)
	for result := range results {
		ordered[result.index] = *result.info
	}

	pvcs := make([]PVCInfo, 0, len(ordered))
	for i := 0; i < jobIndex; i++ {
		if pvc, ok := ordered[i]; ok {
			pvcs = append(pvcs, pvc)
		}
	}
	return pvcs
}

func (d *ResourceDiscovery) getPVCFromCache(namespace, name string, cache *discoveryCache) (*corev1.PersistentVolumeClaim, bool) {
	if cache == nil {
		return nil, false
	}
	nsPVCs, ok := cache.pvcsByNamespace[namespace]
	if !ok {
		return nil, false
	}
	pvc, ok := nsPVCs[name]
	return pvc, ok
}

func (d *ResourceDiscovery) buildPVCInfo(namespace, pvcName string, cache *discoveryCache) *PVCInfo {
	pvc, found := d.getPVCFromCache(namespace, pvcName, cache)
	if !found {
		var err error
		pvc, err = d.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(d.ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return &PVCInfo{
				Name:      pvcName,
				Namespace: namespace,
				Status:    "Unknown",
			}
		}
	}

	secretName := ""
	if pvc.Annotations != nil {
		if csiSecret := pvc.Annotations["cos.csi.driver/secret"]; csiSecret != "" {
			secretName = csiSecret
		}
		if flexSecret := pvc.Annotations["ibm.io/secret-name"]; flexSecret != "" {
			secretName = flexSecret
		}
		if secretName == "" {
			if origSecret := pvc.Annotations["flex-to-csi/original-secret"]; origSecret != "" {
				secretName = origSecret
			} else if origSecret := pvc.Annotations["ibm.io/original-secret-name"]; origSecret != "" {
				secretName = origSecret
			}
		}
	}

	if secretName == "" {
		return nil
	}

	storageClass := ""
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}

	return &PVCInfo{
		Name:         pvcName,
		Namespace:    namespace,
		SecretName:   normalizeOriginalSecretName(secretName),
		PVName:       pvc.Spec.VolumeName,
		StorageClass: &storageClass,
		AccessModes:  pvc.Spec.AccessModes,
		Storage:      pvc.Spec.Resources.Requests.Storage().String(),
		Annotations:  pvc.Annotations,
		Status:       string(pvc.Status.Phase),
	}
}

func (d *ResourceDiscovery) findServicesForLabels(namespace string, labels map[string]string) []string {
	return d.findServicesForLabelsCached(namespace, labels, nil)
}

func (d *ResourceDiscovery) findServicesForLabelsCached(namespace string, labels map[string]string, cache *discoveryCache) []string {
	if len(labels) == 0 {
		return nil
	}

	var services []corev1.Service
	if cache != nil {
		services = cache.servicesByNamespace[namespace]
	} else {
		serviceList, err := d.clientset.CoreV1().Services(namespace).List(d.ctx, metav1.ListOptions{})
		if err != nil {
			return nil
		}
		services = serviceList.Items
	}

	var matched []string
	for _, svc := range services {
		if len(svc.Spec.Selector) == 0 {
			continue
		}

		match := true
		for key, value := range svc.Spec.Selector {
			if labels[key] != value {
				match = false
				break
			}
		}
		if match {
			matched = append(matched, svc.Name)
		}
	}

	return matched
}

func normalizeOriginalSecretName(secretName string) string {
	if strings.HasSuffix(secretName, "-csi-migrated") {
		return strings.TrimSuffix(secretName, "-csi-migrated")
	}
	if strings.HasSuffix(secretName, "-migrated") {
		return strings.TrimSuffix(secretName, "-migrated")
	}
	return secretName
}

// DiscoverAllFlexResourcesOptimized discovers all FlexVolume resources using optimized parallel algorithm.
// Achieves ~800ms discovery time through 3-level parallelization.
//
// How all-namespace search works (when namespace = ""):
// Phase 1: Build Cache (~150-200ms)
//   - Empty namespace "" is converted to metav1.NamespaceAll
//   - 6 parallel API calls fetch ALL resources from ALL namespaces:
//   - PVCs from all namespaces
//   - Services from all namespaces
//   - Deployments from all namespaces
//   - StatefulSets from all namespaces
//   - DaemonSets from all namespaces
//   - Pods from all namespaces
//   - Resources organized by namespace in cache
//
// Phase 2: Discover Resources (~400-500ms)
//   - Worker pool (10 workers) processes namespaces in parallel
//   - Each namespace worker spawns 4 type workers
//   - All resources classified using cache (no API calls)
//
// Phase 3: Finalize (~50-100ms)
//   - Merge results from all namespaces
//   - Deduplicate resources
//   - Sort by namespace/type/name
//
// Parameters:
//
//	namespace: "" for all namespaces, or specific namespace name
//
// Returns:
//
//	[]ResourceInfo: List of discovered Flex resources across all namespaces
//	error: Error if discovery fails
func (d *ResourceDiscovery) DiscoverAllFlexResourcesOptimized(namespace string) ([]ResourceInfo, error) {
	// Phase 1: Build cache with resources from all namespaces (if namespace = "")
	cache, err := d.buildDiscoveryCache(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to build discovery cache: %v", err)
	}

	// Phase 2: Discover resources using parallel worker pools
	// Returns: flexResources (not migrated), migratedResources (CSI), orphanedPVCs (no workload)
	flexResources, _, orphanedFlexPVCs, err := d.discoverResourcesSinglePass(cache)
	if err != nil {
		return nil, err
	}

	resourceMap := make(map[string]ResourceInfo, len(flexResources)+len(orphanedFlexPVCs))
	for _, r := range flexResources {
		key := fmt.Sprintf("%s/%s/%s", r.Namespace, r.Type, r.Name)
		resourceMap[key] = r
	}
	for _, r := range orphanedFlexPVCs {
		if len(r.PVCs) == 0 {
			continue
		}
		key := fmt.Sprintf("%s/OrphanedPVC/%s", r.Namespace, r.PVCs[0].Name)
		resourceMap[key] = r
	}

	finalResources := make([]ResourceInfo, 0, len(resourceMap))
	for _, r := range resourceMap {
		finalResources = append(finalResources, r)
	}

	sort.Slice(finalResources, func(i, j int) bool {
		if finalResources[i].Namespace != finalResources[j].Namespace {
			return finalResources[i].Namespace < finalResources[j].Namespace
		}
		if finalResources[i].Type != finalResources[j].Type {
			return finalResources[i].Type < finalResources[j].Type
		}
		return finalResources[i].Name < finalResources[j].Name
	})

	return finalResources, nil
}

func (d *ResourceDiscovery) DiscoverMigratedResourcesOptimized(namespace string) ([]ResourceInfo, error) {
	cache, err := d.buildDiscoveryCache(namespace)
	if err != nil {
		return nil, err
	}

	_, migratedResources, _, err := d.discoverResourcesSinglePass(cache)
	if err != nil {
		return nil, err
	}

	sort.Slice(migratedResources, func(i, j int) bool {
		if migratedResources[i].Namespace != migratedResources[j].Namespace {
			return migratedResources[i].Namespace < migratedResources[j].Namespace
		}
		if migratedResources[i].Type != migratedResources[j].Type {
			return migratedResources[i].Type < migratedResources[j].Type
		}
		return migratedResources[i].Name < migratedResources[j].Name
	})

	return migratedResources, nil
}

// buildDiscoveryCache builds an in-memory cache by fetching all Kubernetes resources in parallel.
// This is the foundation for fast discovery (~150-200ms).
//
// How all-namespace search works:
// 1. If namespace parameter is "" (empty string), it's converted to metav1.NamespaceAll
// 2. metav1.NamespaceAll is a special constant that tells Kubernetes API to search ALL namespaces
// 3. All 6 API calls (PVCs, Services, Deployments, etc.) use this namespace parameter
// 4. Kubernetes returns resources from every namespace in the cluster
// 5. Resources are then organized by namespace in the cache for efficient processing
//
// Example:
//
//	buildDiscoveryCache("")          -> Searches ALL namespaces
//	buildDiscoveryCache("default")   -> Searches only "default" namespace
//	buildDiscoveryCache("production") -> Searches only "production" namespace
func (d *ResourceDiscovery) buildDiscoveryCache(namespace string) (*discoveryCache, error) {
	// Convert empty namespace to metav1.NamespaceAll for all-namespace search
	// metav1.NamespaceAll = "" (empty string is the Kubernetes convention for "all namespaces")
	searchNamespace := namespace
	if searchNamespace == "" {
		searchNamespace = metav1.NamespaceAll // This enables cluster-wide search
	}

	pvcMap := make(map[string]map[string]*corev1.PersistentVolumeClaim)
	serviceMap := make(map[string][]corev1.Service)
	migratedNames := make(map[string]map[string]struct{})

	var (
		pvcs         *corev1.PersistentVolumeClaimList
		services     *corev1.ServiceList
		deployments  *appsv1.DeploymentList
		statefulsets *appsv1.StatefulSetList
		daemonsets   *appsv1.DaemonSetList
		pods         *corev1.PodList
		pvcErr       error
		serviceErr   error
		deployErr    error
		statefulErr  error
		daemonErr    error
		podErr       error
		wg           sync.WaitGroup
	)

	wg.Add(6)

	go func() {
		defer wg.Done()
		pvcs, pvcErr = d.clientset.CoreV1().PersistentVolumeClaims(searchNamespace).List(d.ctx, metav1.ListOptions{})
	}()

	go func() {
		defer wg.Done()
		services, serviceErr = d.clientset.CoreV1().Services(searchNamespace).List(d.ctx, metav1.ListOptions{})
	}()

	go func() {
		defer wg.Done()
		deployments, deployErr = d.clientset.AppsV1().Deployments(searchNamespace).List(d.ctx, metav1.ListOptions{})
	}()

	go func() {
		defer wg.Done()
		statefulsets, statefulErr = d.clientset.AppsV1().StatefulSets(searchNamespace).List(d.ctx, metav1.ListOptions{})
	}()

	go func() {
		defer wg.Done()
		daemonsets, daemonErr = d.clientset.AppsV1().DaemonSets(searchNamespace).List(d.ctx, metav1.ListOptions{})
	}()

	go func() {
		defer wg.Done()
		pods, podErr = d.clientset.CoreV1().Pods(searchNamespace).List(d.ctx, metav1.ListOptions{})
	}()

	wg.Wait()

	if pvcErr != nil {
		return nil, pvcErr
	}
	if serviceErr != nil {
		return nil, serviceErr
	}
	if deployErr != nil {
		return nil, deployErr
	}
	if statefulErr != nil {
		return nil, statefulErr
	}
	if daemonErr != nil {
		return nil, daemonErr
	}
	if podErr != nil {
		return nil, podErr
	}

	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if _, ok := pvcMap[pvc.Namespace]; !ok {
			pvcMap[pvc.Namespace] = make(map[string]*corev1.PersistentVolumeClaim)
		}
		pvcMap[pvc.Namespace][pvc.Name] = pvc
	}

	for i := range services.Items {
		svc := services.Items[i]
		serviceMap[svc.Namespace] = append(serviceMap[svc.Namespace], svc)
	}

	deploymentsByNS := make(map[string][]*appsv1.Deployment)
	statefulSetsByNS := make(map[string][]*appsv1.StatefulSet)
	daemonSetsByNS := make(map[string][]*appsv1.DaemonSet)
	podsByNS := make(map[string][]*corev1.Pod)
	standalonePodsByNS := make(map[string][]*corev1.Pod)
	namespaceSet := make(map[string]struct{})

	for i := range deployments.Items {
		deploy := &deployments.Items[i]
		namespaceSet[deploy.Namespace] = struct{}{}
		deploymentsByNS[deploy.Namespace] = append(deploymentsByNS[deploy.Namespace], deploy)
		if strings.HasSuffix(deploy.Name, "-csi-migrated") {
			if _, ok := migratedNames[deploy.Namespace]; !ok {
				migratedNames[deploy.Namespace] = make(map[string]struct{})
			}
			migratedNames[deploy.Namespace][deploy.Name] = struct{}{}
		}
	}

	for i := range statefulsets.Items {
		sts := &statefulsets.Items[i]
		namespaceSet[sts.Namespace] = struct{}{}
		statefulSetsByNS[sts.Namespace] = append(statefulSetsByNS[sts.Namespace], sts)
		if strings.HasSuffix(sts.Name, "-csi-migrated") {
			if _, ok := migratedNames[sts.Namespace]; !ok {
				migratedNames[sts.Namespace] = make(map[string]struct{})
			}
			migratedNames[sts.Namespace][sts.Name] = struct{}{}
		}
	}

	for i := range daemonsets.Items {
		ds := &daemonsets.Items[i]
		namespaceSet[ds.Namespace] = struct{}{}
		daemonSetsByNS[ds.Namespace] = append(daemonSetsByNS[ds.Namespace], ds)
		if strings.HasSuffix(ds.Name, "-csi-migrated") {
			if _, ok := migratedNames[ds.Namespace]; !ok {
				migratedNames[ds.Namespace] = make(map[string]struct{})
			}
			migratedNames[ds.Namespace][ds.Name] = struct{}{}
		}
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		namespaceSet[pod.Namespace] = struct{}{}
		podsByNS[pod.Namespace] = append(podsByNS[pod.Namespace], pod)
		if len(pod.OwnerReferences) == 0 {
			standalonePodsByNS[pod.Namespace] = append(standalonePodsByNS[pod.Namespace], pod)
		}
		if strings.HasSuffix(pod.Name, "-csi-migrated") {
			if _, ok := migratedNames[pod.Namespace]; !ok {
				migratedNames[pod.Namespace] = make(map[string]struct{})
			}
			migratedNames[pod.Namespace][pod.Name] = struct{}{}
		}
	}

	for ns := range pvcMap {
		namespaceSet[ns] = struct{}{}
	}
	for ns := range serviceMap {
		namespaceSet[ns] = struct{}{}
	}

	namespaceOrder := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaceOrder = append(namespaceOrder, ns)
	}
	sort.Strings(namespaceOrder)

	return &discoveryCache{
		pvcsByNamespace:     pvcMap,
		servicesByNamespace: serviceMap,
		migratedNamesByNS:   migratedNames,
		deploymentsByNS:     deploymentsByNS,
		statefulSetsByNS:    statefulSetsByNS,
		daemonSetsByNS:      daemonSetsByNS,
		podsByNS:            podsByNS,
		standalonePodsByNS:  standalonePodsByNS,
		namespaceOrder:      namespaceOrder,
		searchNamespace:     searchNamespace,
	}, nil
}

func (d *ResourceDiscovery) migratedResourceExists(name, namespace string, cache *discoveryCache) bool {
	if cache == nil {
		return d.ResourceExists(name, namespace)
	}
	nsResources, ok := cache.migratedNamesByNS[namespace]
	if !ok {
		return false
	}
	_, exists := nsResources[name]
	return exists
}

func (d *ResourceDiscovery) discoverResourcesSinglePass(cache *discoveryCache) ([]ResourceInfo, []ResourceInfo, []ResourceInfo, error) {
	type namespaceResult struct {
		flex     []ResourceInfo
		migrated []ResourceInfo
		orphaned []ResourceInfo
		err      error
	}

	namespaceJobs := make(chan string, len(cache.namespaceOrder))
	namespaceResults := make(chan namespaceResult, len(cache.namespaceOrder))

	workerCount := namespaceWorkerPoolSize
	if len(cache.namespaceOrder) < workerCount {
		workerCount = len(cache.namespaceOrder)
	}
	if workerCount == 0 {
		return []ResourceInfo{}, []ResourceInfo{}, []ResourceInfo{}, nil
	}

	var workerWG sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for namespace := range namespaceJobs {
				flex, migrated, orphaned, err := d.discoverNamespaceResources(namespace, cache)
				namespaceResults <- namespaceResult{
					flex:     flex,
					migrated: migrated,
					orphaned: orphaned,
					err:      err,
				}
			}
		}()
	}

	for _, namespace := range cache.namespaceOrder {
		namespaceJobs <- namespace
	}
	close(namespaceJobs)

	go func() {
		workerWG.Wait()
		close(namespaceResults)
	}()

	var (
		flexResources     []ResourceInfo
		migratedResources []ResourceInfo
		orphanedResources []ResourceInfo
	)
	for result := range namespaceResults {
		if result.err != nil {
			return nil, nil, nil, result.err
		}
		flexResources = append(flexResources, result.flex...)
		migratedResources = append(migratedResources, result.migrated...)
		orphanedResources = append(orphanedResources, result.orphaned...)
	}

	return flexResources, migratedResources, orphanedResources, nil
}

func (d *ResourceDiscovery) discoverNamespaceResources(namespace string, cache *discoveryCache) ([]ResourceInfo, []ResourceInfo, []ResourceInfo, error) {
	type workloadResult struct {
		flex     []ResourceInfo
		migrated []ResourceInfo
		err      error
	}

	typeJobs := make(chan func() workloadResult, 4)
	typeResults := make(chan workloadResult, 4)

	workerCount := resourceTypeWorkerPool
	if 4 < workerCount {
		workerCount = 4
	}

	var workerWG sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for job := range typeJobs {
				typeResults <- job()
			}
		}()
	}

	typeJobs <- func() workloadResult {
		return workloadResult{flex: d.classifyDeployments(namespace, cache), migrated: d.classifyMigratedDeployments(namespace, cache)}
	}
	typeJobs <- func() workloadResult {
		return workloadResult{flex: d.classifyStatefulSets(namespace, cache), migrated: d.classifyMigratedStatefulSets(namespace, cache)}
	}
	typeJobs <- func() workloadResult {
		return workloadResult{flex: d.classifyDaemonSets(namespace, cache), migrated: d.classifyMigratedDaemonSets(namespace, cache)}
	}
	typeJobs <- func() workloadResult {
		return workloadResult{flex: d.classifyStandalonePods(namespace, cache), migrated: d.classifyMigratedStandalonePods(namespace, cache)}
	}
	close(typeJobs)

	go func() {
		workerWG.Wait()
		close(typeResults)
	}()

	var (
		flexResources     []ResourceInfo
		migratedResources []ResourceInfo
	)
	for result := range typeResults {
		if result.err != nil {
			return nil, nil, nil, result.err
		}
		flexResources = append(flexResources, result.flex...)
		migratedResources = append(migratedResources, result.migrated...)
	}

	orphaned := d.findOrphanedFlexPVCsFromCache(namespace, cache)
	return flexResources, migratedResources, orphaned, nil
}

func (d *ResourceDiscovery) classifyDeployments(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyDeploymentSlice(cache.deploymentsByNS[namespace], namespace, cache, false)
}

func (d *ResourceDiscovery) classifyMigratedDeployments(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyDeploymentSlice(cache.deploymentsByNS[namespace], namespace, cache, true)
}

func (d *ResourceDiscovery) classifyDeploymentSlice(items []*appsv1.Deployment, namespace string, cache *discoveryCache, migrated bool) []ResourceInfo {
	resources := make([]ResourceInfo, 0)
	for _, item := range items {
		isMigrated := strings.HasSuffix(item.Name, "-csi-migrated")
		if migrated != isMigrated {
			continue
		}
		info := d.buildResourceInfoFastWithCache(item, "Deployment", namespace, cache)
		if info == nil {
			continue
		}
		if !migrated && d.migratedResourceExists(item.Name+"-csi-migrated", namespace, cache) {
			continue
		}
		resources = append(resources, *info)
	}
	return resources
}

func (d *ResourceDiscovery) classifyStatefulSets(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyStatefulSetSlice(cache.statefulSetsByNS[namespace], namespace, cache, false)
}

func (d *ResourceDiscovery) classifyMigratedStatefulSets(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyStatefulSetSlice(cache.statefulSetsByNS[namespace], namespace, cache, true)
}

func (d *ResourceDiscovery) classifyStatefulSetSlice(items []*appsv1.StatefulSet, namespace string, cache *discoveryCache, migrated bool) []ResourceInfo {
	resources := make([]ResourceInfo, 0)
	for _, item := range items {
		isMigrated := strings.HasSuffix(item.Name, "-csi-migrated")
		if migrated != isMigrated {
			continue
		}
		info := d.buildResourceInfoFastWithCache(item, "StatefulSet", namespace, cache)
		if info == nil {
			continue
		}
		if !migrated && d.migratedResourceExists(item.Name+"-csi-migrated", namespace, cache) {
			continue
		}
		resources = append(resources, *info)
	}
	return resources
}

func (d *ResourceDiscovery) classifyDaemonSets(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyDaemonSetSlice(cache.daemonSetsByNS[namespace], namespace, cache, false)
}

func (d *ResourceDiscovery) classifyMigratedDaemonSets(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyDaemonSetSlice(cache.daemonSetsByNS[namespace], namespace, cache, true)
}

func (d *ResourceDiscovery) classifyDaemonSetSlice(items []*appsv1.DaemonSet, namespace string, cache *discoveryCache, migrated bool) []ResourceInfo {
	resources := make([]ResourceInfo, 0)
	for _, item := range items {
		isMigrated := strings.HasSuffix(item.Name, "-csi-migrated")
		if migrated != isMigrated {
			continue
		}
		info := d.buildResourceInfoFastWithCache(item, "DaemonSet", namespace, cache)
		if info == nil {
			continue
		}
		if !migrated && d.migratedResourceExists(item.Name+"-csi-migrated", namespace, cache) {
			continue
		}
		resources = append(resources, *info)
	}
	return resources
}

func (d *ResourceDiscovery) classifyStandalonePods(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyPodSlice(cache.standalonePodsByNS[namespace], namespace, cache, false)
}

func (d *ResourceDiscovery) classifyMigratedStandalonePods(namespace string, cache *discoveryCache) []ResourceInfo {
	return d.classifyPodSlice(cache.standalonePodsByNS[namespace], namespace, cache, true)
}

func (d *ResourceDiscovery) classifyPodSlice(items []*corev1.Pod, namespace string, cache *discoveryCache, migrated bool) []ResourceInfo {
	resources := make([]ResourceInfo, 0)
	for _, item := range items {
		isMigrated := strings.HasSuffix(item.Name, "-csi-migrated")
		if migrated != isMigrated {
			continue
		}
		info := d.buildResourceInfoFastWithCache(item, "Pod", namespace, cache)
		if info == nil {
			continue
		}
		if !migrated && d.migratedResourceExists(item.Name+"-csi-migrated", namespace, cache) {
			continue
		}
		resources = append(resources, *info)
	}
	return resources
}

func (d *ResourceDiscovery) findOrphanedFlexPVCsFromCache(namespace string, cache *discoveryCache) []ResourceInfo {
	nsPVCs := cache.pvcsByNamespace[namespace]
	if len(nsPVCs) == 0 {
		return nil
	}

	pvcsInUse := make(map[string]struct{})
	for _, pod := range cache.podsByNS[namespace] {
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim == nil {
				continue
			}
			pvcsInUse[vol.PersistentVolumeClaim.ClaimName] = struct{}{}
		}
	}

	orphaned := make([]ResourceInfo, 0)
	for pvcName := range nsPVCs {
		info := d.buildPVCInfo(namespace, pvcName, cache)
		if info == nil {
			continue
		}
		if !d.isFlexPVC(info) {
			continue
		}
		if _, used := pvcsInUse[pvcName]; used {
			continue
		}
		orphaned = append(orphaned, ResourceInfo{
			Name:      "",
			Namespace: namespace,
			Type:      "",
			PVCs:      []PVCInfo{*info},
			Services:  []string{},
			Object:    nil,
		})
	}
	return orphaned
}

func (d *ResourceDiscovery) isFlexPVC(pvc *PVCInfo) bool {
	if pvc == nil {
		return false
	}
	if pvc.StorageClass != nil && strings.Contains(strings.ToLower(*pvc.StorageClass), "ibmc-s3fs") {
		return true
	}
	if pvc.Annotations != nil && pvc.Annotations["ibm.io/secret-name"] != "" {
		return true
	}
	return false
}
