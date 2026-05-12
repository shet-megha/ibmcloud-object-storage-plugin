package interactive

import (
	"fmt"
	"strings"
	"time"

	"kubectl-flex-to-csi/pkg/discovery"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a *App) discoverResources() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           DISCOVER RESOURCES                               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	fmt.Println("\n🔍 Scanning all namespaces for Flex resources...")
	namespace := ""

	flexResources, err := a.discovery.DiscoverAllFlexResourcesOptimized(namespace)
	if err != nil {
		fmt.Printf("\n❌ Error loading Flex resources: %v\n", err)
		a.pause()
		return
	}

	migratedResources, err := a.discovery.DiscoverMigratedResourcesOptimized(namespace)
	if err != nil {
		fmt.Printf("\n❌ Error loading migrated resources: %v\n", err)
		a.pause()
		return
	}

	if len(flexResources) == 0 && len(migratedResources) == 0 {
		fmt.Println("\n✅ No Flex or migrated resources found!")
		fmt.Println("   No actionable resources exist in the selected namespace scope.")
		a.pause()
		return
	}

	a.tracker.TotalResources = len(flexResources)
	a.tracker.PendingCount = len(flexResources)
	a.tracker.MigratedCount = 0
	a.tracker.InProgressCount = 0
	a.tracker.FailedCount = 0
	a.tracker.Resources = []ResourceStatus{}

	for _, res := range flexResources {
		oldPVCs := []string{}
		for _, pvc := range res.PVCs {
			oldPVCs = append(oldPVCs, pvc.Name)
		}

		a.tracker.Resources = append(a.tracker.Resources, ResourceStatus{
			Name:        res.Name,
			Namespace:   res.Namespace,
			Type:        res.Type,
			Status:      "pending",
			OldPVCs:     oldPVCs,
			LastUpdated: time.Now(),
		})
	}

	for _, res := range migratedResources {
		baseName := strings.TrimSuffix(res.Name, "-csi-migrated")

		newPVCs := []string{}
		for _, pvc := range res.PVCs {
			newPVCs = append(newPVCs, pvc.Name)
		}

		a.tracker.Resources = append(a.tracker.Resources, ResourceStatus{
			Name:        baseName,
			Namespace:   res.Namespace,
			Type:        res.Type,
			Status:      "migrated",
			NewPVCs:     newPVCs,
			LastUpdated: time.Now(),
		})
		a.tracker.TotalResources++
		a.tracker.MigratedCount++
	}

	fmt.Printf("\n📊 Discovery Summary:\n")

	if len(flexResources) > 0 {
		fmt.Println("\n📋 Flex Resources:")
		a.printResourceInfoTable(flexResources)
	} else {
		fmt.Println("\n📋 Flex Resources:")
		fmt.Println("   No Flex-only resources found.")
	}

	if len(migratedResources) > 0 {
		fmt.Println("\n✅ Migrated Resources:")
		a.printResourceInfoTable(migratedResources)
	} else {
		fmt.Println("\n✅ Migrated Resources:")
		fmt.Println("   No migrated resources found.")
	}

	a.pause()
}

func (a *App) discoverResourcesNoPause() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           DISCOVER RESOURCES                               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	discoveryStart := time.Now()

	fmt.Println("\n🔍 Scanning all namespaces for Flex resources...")
	fmt.Println("   ⏱️  This may take a few minutes depending on cluster size...")

	// Empty namespace string ("") triggers all-namespace discovery
	// When namespace is "", the discovery function converts it to metav1.NamespaceAll
	// which tells Kubernetes API to search across ALL namespaces in the cluster
	namespace := ""

	done := make(chan bool)
	go a.showLoadingAnimation(done)

	flexScanStart := time.Now()
	// DiscoverAllFlexResourcesOptimized with empty namespace searches ALL namespaces
	// How it works:
	// 1. buildDiscoveryCache("") converts "" to metav1.NamespaceAll
	// 2. Kubernetes API calls use metav1.NamespaceAll to fetch resources from all namespaces
	// 3. Cache is built with resources from every namespace in the cluster
	// 4. Worker pool processes all namespaces in parallel (up to 10 concurrent)
	// 5. Results are merged and returned as a single list
	flexResources, err := a.discovery.DiscoverAllFlexResourcesOptimized(namespace)
	flexScanDuration := time.Since(flexScanStart)

	done <- true
	fmt.Printf("\r   ✅ Flex resources scan complete! (took %v) ⚡⚡                    \n", flexScanDuration)

	if err != nil {
		fmt.Printf("\n❌ Error loading Flex resources: %v\n", err)
		return
	}

	fmt.Println("   🔍 Scanning for migrated resources...")
	done2 := make(chan bool)
	go a.showLoadingAnimation(done2)

	migratedScanStart := time.Now()
	migratedResources, err := a.discovery.DiscoverMigratedResourcesOptimized(namespace)
	migratedScanDuration := time.Since(migratedScanStart)

	done2 <- true
	fmt.Printf("\r   ✅ Migrated resources scan complete! (took %v) ⚡⚡               \n", migratedScanDuration)
	if err != nil {
		fmt.Printf("\n❌ Error loading migrated resources: %v\n", err)
		return
	}

	totalDiscoveryTime := time.Since(discoveryStart)

	if len(flexResources) == 0 && len(migratedResources) == 0 {
		fmt.Println("\n✅ No Flex or migrated resources found!")
		fmt.Println("   No actionable resources exist in the selected namespace scope.")
		fmt.Printf("\n⏱️  Total discovery time: %v\n", totalDiscoveryTime)
		return
	}

	a.tracker.TotalResources = len(flexResources)
	a.tracker.PendingCount = len(flexResources)
	a.tracker.MigratedCount = 0
	a.tracker.InProgressCount = 0
	a.tracker.FailedCount = 0
	a.tracker.Resources = []ResourceStatus{}

	for _, res := range flexResources {
		oldPVCs := []string{}
		for _, pvc := range res.PVCs {
			oldPVCs = append(oldPVCs, pvc.Name)
		}

		a.tracker.Resources = append(a.tracker.Resources, ResourceStatus{
			Name:        res.Name,
			Namespace:   res.Namespace,
			Type:        res.Type,
			Status:      "pending",
			OldPVCs:     oldPVCs,
			LastUpdated: time.Now(),
		})
	}

	for _, res := range migratedResources {
		baseName := strings.TrimSuffix(res.Name, "-csi-migrated")

		newPVCs := []string{}
		for _, pvc := range res.PVCs {
			newPVCs = append(newPVCs, pvc.Name)
		}

		a.tracker.Resources = append(a.tracker.Resources, ResourceStatus{
			Name:        baseName,
			Namespace:   res.Namespace,
			Type:        res.Type,
			Status:      "migrated",
			NewPVCs:     newPVCs,
			LastUpdated: time.Now(),
		})
		a.tracker.TotalResources++
		a.tracker.MigratedCount++
	}

	fmt.Printf("\n📊 Discovery Summary (completed in %v):\n", totalDiscoveryTime)
	fmt.Printf("   • Flex resources found: %d\n", len(flexResources))
	fmt.Printf("   • Already migrated resources: %d\n", len(migratedResources))

	if len(flexResources) > 0 {
		fmt.Println("\n📋 Flex Resources:")
		a.printResourceInfoTable(flexResources)
	} else {
		fmt.Println("\n📋 Flex Resources:")
		fmt.Println("   No Flex-only resources found.")
	}

	if len(migratedResources) > 0 {
		fmt.Println("\n✅ Migrated Resources:")
		a.printResourceInfoTable(migratedResources)
	} else {
		fmt.Println("\n✅ Migrated Resources:")
		fmt.Println("   No migrated resources found.")
	}
}

func (a *App) getResourceInfosFromStatuses(statuses []ResourceStatus) []discovery.ResourceInfo {
	resourceInfos := make([]discovery.ResourceInfo, 0, len(statuses))

	for _, status := range statuses {
		resourceInfo, err := a.discovery.GetResourceByName(status.Name, status.Namespace)
		if err != nil {
			if len(status.OldPVCs) > 0 {
				pvcInfos := []discovery.PVCInfo{}
				for _, pvcName := range status.OldPVCs {
					pvc, pvcErr := a.clientset.CoreV1().PersistentVolumeClaims(status.Namespace).Get(a.ctx, pvcName, metav1.GetOptions{})
					if pvcErr == nil && pvc.Annotations != nil {
						secretName := ""
						if flexSecret := pvc.Annotations["ibm.io/secret-name"]; flexSecret != "" {
							secretName = flexSecret
						}
						if csiSecret := pvc.Annotations["cos.csi.driver/secret"]; csiSecret != "" {
							secretName = csiSecret
						}

						storageClass := ""
						if pvc.Spec.StorageClassName != nil {
							storageClass = *pvc.Spec.StorageClassName
						}

						pvcInfos = append(pvcInfos, discovery.PVCInfo{
							Name:         pvcName,
							Namespace:    status.Namespace,
							SecretName:   secretName,
							PVName:       pvc.Spec.VolumeName,
							StorageClass: &storageClass,
							AccessModes:  pvc.Spec.AccessModes,
							Storage:      pvc.Spec.Resources.Requests.Storage().String(),
							Annotations:  pvc.Annotations,
							Status:       string(pvc.Status.Phase),
						})
					}
				}

				resourceInfos = append(resourceInfos, discovery.ResourceInfo{
					Name:      status.Name,
					Namespace: status.Namespace,
					Type:      status.Type,
					PVCs:      pvcInfos,
				})
				continue
			}

			resourceInfos = append(resourceInfos, discovery.ResourceInfo{
				Name:      status.Name,
				Namespace: status.Namespace,
				Type:      status.Type,
			})
			continue
		}
		resourceInfos = append(resourceInfos, *resourceInfo)
	}

	return resourceInfos
}

func (a *App) getNamespacesWithFlexResources() ([]string, error) {
	pvs, err := a.clientset.CoreV1().PersistentVolumes().List(a.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	namespacesMap := make(map[string]bool)

	for i := range pvs.Items {
		pv := &pvs.Items[i]

		if pv.Spec.FlexVolume != nil &&
			(strings.Contains(pv.Spec.FlexVolume.Driver, "ibm/ibmc-s3fs") ||
				strings.Contains(pv.Spec.FlexVolume.Driver, "ibm-s3fs")) {

			if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Namespace != "" {
				namespacesMap[pv.Spec.ClaimRef.Namespace] = true
			}
		}
	}

	namespaces := make([]string, 0, len(namespacesMap))
	for ns := range namespacesMap {
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// Made with Bob
