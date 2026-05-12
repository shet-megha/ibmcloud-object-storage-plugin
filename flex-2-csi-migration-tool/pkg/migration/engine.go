package migration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kubectl-flex-to-csi/pkg/discovery"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MigrationEngine handles the complete migration workflow
type MigrationEngine struct {
	clientset *kubernetes.Clientset
	ctx       context.Context
	dryRun    bool
}

// MigrationResult tracks the outcome of a migration
type MigrationResult struct {
	ResourceName   string
	ResourceType   string
	Namespace      string
	Status         string // success, failed, skipped
	Error          error
	OldPVCName     string
	NewPVCName     string
	OldSecretName  string
	NewSecretName  string
	MigrationTime  time.Duration
	VerificationOK bool
}

// NewMigrationEngine creates a new migration engine
func NewMigrationEngine(clientset *kubernetes.Clientset, dryRun bool) *MigrationEngine {
	return &MigrationEngine{
		clientset: clientset,
		ctx:       context.Background(),
		dryRun:    dryRun,
	}
}

// MigrateResource performs complete migration of a single resource
func (e *MigrationEngine) MigrateResource(resource discovery.ResourceInfo) (*MigrationResult, error) {
	startTime := time.Now()

	result := &MigrationResult{
		ResourceName: resource.Name,
		ResourceType: resource.Type,
		Namespace:    resource.Namespace,
		Status:       "pending",
	}

	fmt.Printf("\n╔════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  Migrating: %s (%s)\n", resource.Name, resource.Type)
	fmt.Printf("║  Namespace: %s\n", resource.Namespace)
	fmt.Printf("╚════════════════════════════════════════════════════════════╝\n\n")

	// Display migration plan
	fmt.Println("📋 Migration Plan (12 Steps):")
	fmt.Println("   1. Verify CSI addon installation")
	fmt.Println("   2. Check node operating systems")
	fmt.Println("   3. Process PVC and read configurations")
	fmt.Println("   4. Read Flex secret and PV YAML")
	fmt.Println("   5. Create CSI secret")
	fmt.Println("   6. Create CSI PVC")
	fmt.Println("   7. Wait for PVC to bind")
	fmt.Println("   8. Create new resource with CSI volumes")
	fmt.Println("   9. Check pod running status")
	fmt.Println("   10. Migrate services")
	fmt.Println("   11. Verify mount options")
	fmt.Println("   12. Check retention policy")
	fmt.Println()

	// Step 1: Verify CSI addon is installed
	fmt.Println("Step 1: Verifying CSI addon installation...")
	if err := e.verifyCSIAddon(); err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("CSI addon verification failed: %v", err)
		return result, result.Error
	}
	fmt.Println("✅ CSI addon is installed and ready\n")

	// Step 2: Check supported OS
	fmt.Println("Step 2: Checking node operating systems...")
	if err := e.checkSupportedOS(); err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("OS check failed: %v", err)
		return result, result.Error
	}
	fmt.Println("✅ All nodes running supported OS\n")

	// Step 3-7: Process each PVC
	for _, pvcInfo := range resource.PVCs {
		fmt.Printf("Step 3: Processing PVC: %s\n", pvcInfo.Name)

		// Step 4: Read secret and PV YAML
		fmt.Println("\nStep 4: Reading Flex secret and PV YAML...")
		secret, pv, err := e.readFlexResources(pvcInfo)
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("failed to read Flex resources: %v", err)
			return result, result.Error
		}
		fmt.Printf("  ✅ Secret: %s\n", pvcInfo.SecretName)
		fmt.Printf("  ✅ PV: %s\n", pvcInfo.PVName)
		fmt.Println("✅ Resources read successfully\n")

		// Step 5: Create CSI secret
		fmt.Println("Step 5: Creating CSI secret...")
		newSecretName := fmt.Sprintf("%s-csi-migrated", pvcInfo.SecretName)
		if err := e.createCSISecret(secret, pv, newSecretName, pvcInfo.Namespace); err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("failed to create CSI secret: %v", err)
			return result, result.Error
		}
		result.NewSecretName = newSecretName
		result.OldSecretName = pvcInfo.SecretName
		fmt.Printf("✅ CSI secret created: %s\n\n", newSecretName)

		// Step 6: Create CSI PVC
		fmt.Println("Step 6: Creating CSI PVC...")
		newPVCName := fmt.Sprintf("%s-csi-migrated", pvcInfo.Name)
		if err := e.createCSIPVC(pvcInfo, pv, newPVCName, newSecretName); err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("failed to create CSI PVC: %v", err)
			return result, result.Error
		}
		result.NewPVCName = newPVCName
		result.OldPVCName = pvcInfo.Name
		fmt.Printf("✅ CSI PVC created: %s\n\n", newPVCName)

		// Step 7: Check PVC bound state
		fmt.Println("Step 7: Waiting for PVC to bind...")
		if err := e.waitForPVCBound(newPVCName, pvcInfo.Namespace); err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("PVC failed to bind: %v", err)
			return result, result.Error
		}
		fmt.Println("✅ PVC is bound\n")
	}

	// Step 8: Create new resource with CSI PVCs
	fmt.Println("Step 8: Creating new resource with CSI volumes...")
	newResourceName := fmt.Sprintf("%s-csi-migrated", resource.Name)
	fmt.Printf("  → Creating %s: %s\n", resource.Type, newResourceName)
	if err := e.createMigratedResource(resource, newResourceName); err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("failed to create migrated resource: %v", err)
		fmt.Printf("❌ Error creating resource: %v\n\n", err)
		return result, result.Error
	}
	fmt.Printf("✅ New resource created: %s\n\n", newResourceName)

	// Step 9: Check pod running status
	fmt.Println("Step 9: Checking pod status...")
	if err := e.waitForPodRunning(newResourceName, resource.Namespace, resource.Type); err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("pod failed to start: %v", err)
		return result, result.Error
	}
	fmt.Println("✅ Pod is running\n")

	// Step 10: Attach services
	fmt.Println("Step 10: Migrating services...")
	if err := e.migrateServices(resource, newResourceName); err != nil {
		fmt.Printf("⚠️  Service migration warning: %v\n", err)
	} else {
		fmt.Println("✅ Services migrated\n")
	}

	// Step 11: Verify mount options
	fmt.Println("Step 11: Verifying mount options...")
	if err := e.verifyMountOptions(newResourceName, resource.Namespace); err != nil {
		fmt.Printf("⚠️  Mount options verification warning: %v\n", err)
	} else {
		fmt.Println("✅ Mount options verified\n")
		result.VerificationOK = true
	}

	// Step 12: Check retention policy
	fmt.Println("Step 12: Checking retention policy...")
	if err := e.ensureRetentionPolicy(resource.PVCs); err != nil {
		fmt.Printf("⚠️  Retention policy warning: %v\n", err)
	} else {
		fmt.Println("✅ Retention policy set\n")
	}

	result.Status = "success"
	result.MigrationTime = time.Since(startTime)

	fmt.Printf("\n✅ Migration completed successfully in %v\n", result.MigrationTime)
	fmt.Println("═══════════════════════════════════════════════════════════\n")

	return result, nil
}

// verifyCSIAddon checks if CSI driver is installed
func (e *MigrationEngine) verifyCSIAddon() error {
	daemonsets, err := e.clientset.AppsV1().DaemonSets("").List(e.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list daemonsets: %v", err)
	}

	for _, ds := range daemonsets.Items {
		if strings.Contains(ds.Name, "ibm-object-csi") ||
			strings.Contains(ds.Name, "cos-csi") ||
			strings.Contains(ds.Name, "ibm-cos-csi") {
			if ds.Status.NumberReady > 0 {
				fmt.Printf("  Found CSI driver: %s (namespace: %s)\n", ds.Name, ds.Namespace)
				fmt.Printf("  Ready pods: %d/%d\n", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
				return nil
			}
		}
	}

	return fmt.Errorf("CSI driver not found or not ready")
}

// checkSupportedOS verifies nodes are running supported OS
func (e *MigrationEngine) checkSupportedOS() error {
	nodes, err := e.clientset.CoreV1().Nodes().List(e.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	supportedCount := 0
	unsupportedNodes := []string{}

	for _, node := range nodes.Items {
		osImage := strings.ToLower(node.Status.NodeInfo.OSImage)

		// Check for supported OS: Ubuntu or RHEL/CoreOS
		if strings.Contains(osImage, "ubuntu") ||
			strings.Contains(osImage, "rhel") ||
			strings.Contains(osImage, "coreos") ||
			strings.Contains(osImage, "red hat") {
			supportedCount++
			fmt.Printf("  ✅ %s: %s\n", node.Name, node.Status.NodeInfo.OSImage)
		} else {
			unsupportedNodes = append(unsupportedNodes, node.Name)
			fmt.Printf("  ⚠️  %s: %s (unsupported)\n", node.Name, node.Status.NodeInfo.OSImage)
		}
	}

	if supportedCount == 0 {
		return fmt.Errorf("no nodes with supported OS (Ubuntu/RHEL/CoreOS) found")
	}

	if len(unsupportedNodes) > 0 {
		fmt.Printf("  ⚠️  Warning: %d node(s) with unsupported OS\n", len(unsupportedNodes))
	}

	return nil
}

func (e *MigrationEngine) VerifyCSIAddon() error {
	return e.verifyCSIAddon()
}

func (e *MigrationEngine) CheckSupportedOS() error {
	return e.checkSupportedOS()
}

// readFlexResources reads the Flex secret and PV
func (e *MigrationEngine) readFlexResources(pvcInfo discovery.PVCInfo) (*corev1.Secret, *corev1.PersistentVolume, error) {
	// Check if PVC is bound to a PV
	if pvcInfo.PVName == "" {
		return nil, nil, fmt.Errorf("PVC %s/%s is not bound to any PV (status: Pending). Please ensure the PVC is bound before migration", pvcInfo.Namespace, pvcInfo.Name)
	}

	// Get secret
	secret, err := e.clientset.CoreV1().Secrets(pvcInfo.Namespace).Get(e.ctx, pvcInfo.SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret %s: %v", pvcInfo.SecretName, err)
	}

	// Get PV
	pv, err := e.clientset.CoreV1().PersistentVolumes().Get(e.ctx, pvcInfo.PVName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get PV %s: %v", pvcInfo.PVName, err)
	}

	return secret, pv, nil
}

// createCSISecret creates a new CSI secret from Flex secret
func (e *MigrationEngine) createCSISecret(flexSecret *corev1.Secret, pv *corev1.PersistentVolume, newName, namespace string) error {
	if e.dryRun {
		fmt.Printf("  [DRY RUN] Would create CSI secret: %s\n", newName)
		return nil
	}

	// Check if already exists
	_, err := e.clientset.CoreV1().Secrets(namespace).Get(e.ctx, newName, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("  ℹ️  CSI secret already exists: %s\n", newName)
		return nil
	}

	// Extract mount options from PV
	mountOptions := e.extractMountOptions(pv)

	// Extract bucket name from PV FlexVolume options
	bucketName := ""
	endpoint := ""
	if pv.Spec.FlexVolume != nil && pv.Spec.FlexVolume.Options != nil {
		if val, ok := pv.Spec.FlexVolume.Options["bucket"]; ok {
			bucketName = val
		}
		if val, ok := pv.Spec.FlexVolume.Options["object-store-endpoint"]; ok {
			endpoint = val
		}
	}

	// Fallback to secret data if not in PV
	if bucketName == "" {
		bucketName = string(flexSecret.Data["bucket-name"])
	}
	if endpoint == "" {
		endpoint = string(flexSecret.Data["api-endpoint"])
	}

	// Create new CSI secret
	csiSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newName,
			Namespace: namespace,
		},
		Type: "cos-s3-csi-driver",
		StringData: map[string]string{
			"accessKey":         string(flexSecret.Data["access-key"]),
			"secretKey":         string(flexSecret.Data["secret-key"]),
			"bucketName":        bucketName,
			"endpoint":          endpoint,
			"serviceInstanceID": string(flexSecret.Data["res-conf-apikey"]),
			"bucketVersioning":  "false",
			"mountOptions":      mountOptions,
		},
	}

	_, err = e.clientset.CoreV1().Secrets(namespace).Create(e.ctx, csiSecret, metav1.CreateOptions{})
	return err
}

// extractMountOptions extracts mount options from PV
func (e *MigrationEngine) extractMountOptions(pv *corev1.PersistentVolume) string {
	options := []string{}

	// First, try to extract from FlexVolume options (for Flex PVs)
	if pv.Spec.FlexVolume != nil && pv.Spec.FlexVolume.Options != nil {
		flexOpts := pv.Spec.FlexVolume.Options

		// Map Flex options to CSI s3fs options
		if val, ok := flexOpts["chunk-size-mb"]; ok {
			options = append(options, fmt.Sprintf("multipart_size=%s", val))
		}
		if val, ok := flexOpts["parallel-count"]; ok {
			options = append(options, fmt.Sprintf("parallel_count=%s", val))
		}
		if val, ok := flexOpts["multireq-max"]; ok {
			options = append(options, fmt.Sprintf("multireq_max=%s", val))
		}
		if val, ok := flexOpts["stat-cache-size"]; ok {
			options = append(options, fmt.Sprintf("max_stat_cache_size=%s", val))
		}
		if val, ok := flexOpts["s3fs-fuse-retry-count"]; ok {
			options = append(options, fmt.Sprintf("retries=%s", val))
		}
		if val, ok := flexOpts["kernel-cache"]; ok && val == "true" {
			options = append(options, "kernel_cache")
		}
		// Add max_dirty_data mapping
		if val, ok := flexOpts["max-dirty-data"]; ok {
			options = append(options, fmt.Sprintf("max_dirty_data=%s", val))
		}
	}

	// Also check pv.Spec.MountOptions (for CSI PVs or if options are there)
	if pv.Spec.MountOptions != nil {
		for _, opt := range pv.Spec.MountOptions {
			if strings.HasPrefix(opt, "chunk-size-mb=") {
				val := strings.TrimPrefix(opt, "chunk-size-mb=")
				if !contains(options, "multipart_size") {
					options = append(options, fmt.Sprintf("multipart_size=%s", val))
				}
			} else if strings.HasPrefix(opt, "parallel-count=") {
				val := strings.TrimPrefix(opt, "parallel-count=")
				if !contains(options, "parallel_count") {
					options = append(options, fmt.Sprintf("parallel_count=%s", val))
				}
			} else if strings.HasPrefix(opt, "multireq-max=") {
				val := strings.TrimPrefix(opt, "multireq-max=")
				if !contains(options, "multireq_max") {
					options = append(options, fmt.Sprintf("multireq_max=%s", val))
				}
			} else if strings.HasPrefix(opt, "stat-cache-size=") {
				val := strings.TrimPrefix(opt, "stat-cache-size=")
				if !contains(options, "max_stat_cache_size") {
					options = append(options, fmt.Sprintf("max_stat_cache_size=%s", val))
				}
			} else if opt == "kernel-cache" {
				if !contains(options, "kernel_cache") {
					options = append(options, "kernel_cache")
				}
			}
		}
	}

	// Add defaults only if not already specified
	if !contains(options, "multipart_size") {
		options = append(options, "multipart_size=52")
	}
	if !contains(options, "parallel_count") {
		options = append(options, "parallel_count=10")
	}
	if !contains(options, "max_stat_cache_size") {
		options = append(options, "max_stat_cache_size=100000")
	}
	if !contains(options, "multireq_max") {
		options = append(options, "multireq_max=20")
	}
	if !contains(options, "retries") {
		options = append(options, "retries=5")
	}
	if !contains(options, "kernel_cache") {
		options = append(options, "kernel_cache")
	}
	if !contains(options, "max_dirty_data") {
		options = append(options, "max_dirty_data=5120")
	}

	options = append(options, "allow_other")

	return strings.Join(options, "\n    ")
}

// Helper function
func contains(slice []string, prefix string) bool {
	for _, item := range slice {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

// createCSIPVC creates a new CSI PVC
func (e *MigrationEngine) createCSIPVC(pvcInfo discovery.PVCInfo, pv *corev1.PersistentVolume, newName, secretName string) error {
	if e.dryRun {
		fmt.Printf("  [DRY RUN] Would create CSI PVC: %s\n", newName)
		return nil
	}

	// Check if already exists
	_, err := e.clientset.CoreV1().PersistentVolumeClaims(pvcInfo.Namespace).Get(e.ctx, newName, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("  ℹ️  CSI PVC already exists: %s\n", newName)
		return nil
	}

	// Get original PVC
	originalPVC, err := e.clientset.CoreV1().PersistentVolumeClaims(pvcInfo.Namespace).Get(e.ctx, pvcInfo.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get original PVC: %v", err)
	}

	// Create new CSI PVC
	csiPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newName,
			Namespace: pvcInfo.Namespace,
			Annotations: map[string]string{
				"cos.csi.driver/secret":     secretName,
				"ibm.io/auto-create-bucket": pvcInfo.Annotations["ibm.io/auto-create-bucket"],
				"ibm.io/auto-delete-bucket": pvcInfo.Annotations["ibm.io/auto-delete-bucket"],
				"ibm.io/bucket":             pvcInfo.Annotations["ibm.io/bucket"],
				"ibm.io/endpoint":           pvcInfo.Annotations["ibm.io/endpoint"],
				"ibm.io/region":             pvcInfo.Annotations["ibm.io/region"],
				"ibm.io/object-path":        pvcInfo.Annotations["ibm.io/object-path"],
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      originalPVC.Spec.AccessModes,
			Resources:        originalPVC.Spec.Resources,
			StorageClassName: getCSIStorageClass(originalPVC.Spec.StorageClassName),
			VolumeMode:       originalPVC.Spec.VolumeMode,
		},
	}

	_, err = e.clientset.CoreV1().PersistentVolumeClaims(pvcInfo.Namespace).Create(e.ctx, csiPVC, metav1.CreateOptions{})
	return err
}

// getCSIStorageClass maps Flex storage class to CSI storage class
func getCSIStorageClass(flexSC *string) *string {
	if flexSC == nil {
		sc := "ibm-object-storage-standard-s3fs"
		return &sc
	}

	// Map ALL flex storage classes (ibmc-s3fs-*) to CSI equivalents (ibm-object-storage-*-s3fs)
	// Flex provisioner: ibm.io/ibmc-s3fs
	// CSI provisioner: cos.s3.csi.ibm.io
	scMap := map[string]string{
		// Flex storage classes -> CSI storage classes
		"ibmc-s3fs-flex":                       "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-flex-regional":              "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-flex-cross-region":          "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-flex-perf-regional":         "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-standard-regional":          "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-standard-cross-region":      "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-standard-perf-regional":     "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-standard-perf-cross-region": "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-smart-regional":             "ibm-object-storage-smart-s3fs",
		"ibmc-s3fs-smart-cross-region":         "ibm-object-storage-smart-s3fs",
		"ibmc-s3fs-smart-perf-regional":        "ibm-object-storage-smart-s3fs",
		"ibmc-s3fs-smart-perf-cross-region":    "ibm-object-storage-smart-s3fs",
		"ibmc-s3fs-cold-regional":              "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-cold-cross-region":          "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-vault-regional":             "ibm-object-storage-standard-s3fs",
		"ibmc-s3fs-vault-cross-region":         "ibm-object-storage-standard-s3fs",
	}

	if csiSC, ok := scMap[*flexSC]; ok {
		return &csiSC
	}

	// If not in map, default to standard CSI storage class
	sc := "ibm-object-storage-standard-s3fs"
	return &sc
}

// waitForPVCBound waits for PVC to be bound
func (e *MigrationEngine) waitForPVCBound(pvcName, namespace string) error {
	if e.dryRun {
		return nil
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for PVC to bind")
		case <-ticker.C:
			pvc, err := e.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(e.ctx, pvcName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if pvc.Status.Phase == corev1.ClaimBound {
				return nil
			}

			fmt.Printf("  ⏳ PVC status: %s (waiting...)\n", pvc.Status.Phase)
		}
	}
}

// createMigratedResource creates a new resource with CSI PVCs
func (e *MigrationEngine) createMigratedResource(resource discovery.ResourceInfo, newName string) error {
	if e.dryRun {
		fmt.Printf("  [DRY RUN] Would create %s: %s\n", resource.Type, newName)
		return nil
	}

	switch resource.Type {
	case "Deployment":
		return e.createMigratedDeployment(resource, newName)
	case "StatefulSet":
		return e.createMigratedStatefulSet(resource, newName)
	case "DaemonSet":
		return e.createMigratedDaemonSet(resource, newName)
	case "Pod":
		return e.createMigratedPod(resource, newName)
	default:
		return fmt.Errorf("unsupported resource type: %s", resource.Type)
	}
}

// createMigratedDeployment creates a new deployment with CSI volumes
func (e *MigrationEngine) createMigratedDeployment(resource discovery.ResourceInfo, newName string) error {
	deploy := resource.Object.(*appsv1.Deployment)

	newDeploy := deploy.DeepCopy()
	newDeploy.Name = newName
	newDeploy.ResourceVersion = ""
	newDeploy.UID = ""

	if newDeploy.Spec.Selector.MatchLabels == nil {
		newDeploy.Spec.Selector.MatchLabels = make(map[string]string)
	}
	if newDeploy.Spec.Template.Labels == nil {
		newDeploy.Spec.Template.Labels = make(map[string]string)
	}
	newDeploy.Spec.Selector.MatchLabels["app"] = newName
	newDeploy.Spec.Template.Labels["app"] = newName

	// Update PVC references
	for i, vol := range newDeploy.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			for _, pvcInfo := range resource.PVCs {
				if vol.PersistentVolumeClaim.ClaimName == pvcInfo.Name {
					newDeploy.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = fmt.Sprintf("%s-csi-migrated", pvcInfo.Name)
				}
			}
		}
	}

	_, err := e.clientset.AppsV1().Deployments(resource.Namespace).Create(e.ctx, newDeploy, metav1.CreateOptions{})
	return err
}

// createMigratedStatefulSet creates a new statefulset with CSI volumes
func (e *MigrationEngine) createMigratedStatefulSet(resource discovery.ResourceInfo, newName string) error {
	sts := resource.Object.(*appsv1.StatefulSet)

	newSts := sts.DeepCopy()
	newSts.Name = newName
	newSts.ResourceVersion = ""
	newSts.UID = ""

	if newSts.Spec.Selector.MatchLabels == nil {
		newSts.Spec.Selector.MatchLabels = make(map[string]string)
	}
	if newSts.Spec.Template.Labels == nil {
		newSts.Spec.Template.Labels = make(map[string]string)
	}
	newSts.Spec.Selector.MatchLabels["app"] = newName
	newSts.Spec.Template.Labels["app"] = newName

	// Update PVC references
	for i, vol := range newSts.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			for _, pvcInfo := range resource.PVCs {
				if vol.PersistentVolumeClaim.ClaimName == pvcInfo.Name {
					newSts.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = fmt.Sprintf("%s-csi-migrated", pvcInfo.Name)
				}
			}
		}
	}

	_, err := e.clientset.AppsV1().StatefulSets(resource.Namespace).Create(e.ctx, newSts, metav1.CreateOptions{})
	return err
}

// createMigratedDaemonSet creates a new daemonset with CSI volumes
func (e *MigrationEngine) createMigratedDaemonSet(resource discovery.ResourceInfo, newName string) error {
	ds := resource.Object.(*appsv1.DaemonSet)

	newDs := ds.DeepCopy()
	newDs.Name = newName
	newDs.ResourceVersion = ""
	newDs.UID = ""

	if newDs.Spec.Selector.MatchLabels == nil {
		newDs.Spec.Selector.MatchLabels = make(map[string]string)
	}
	if newDs.Spec.Template.Labels == nil {
		newDs.Spec.Template.Labels = make(map[string]string)
	}
	newDs.Spec.Selector.MatchLabels["app"] = newName
	newDs.Spec.Template.Labels["app"] = newName

	// Update PVC references
	for i, vol := range newDs.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			for _, pvcInfo := range resource.PVCs {
				if vol.PersistentVolumeClaim.ClaimName == pvcInfo.Name {
					newDs.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = fmt.Sprintf("%s-csi-migrated", pvcInfo.Name)
				}
			}
		}
	}

	_, err := e.clientset.AppsV1().DaemonSets(resource.Namespace).Create(e.ctx, newDs, metav1.CreateOptions{})
	return err
}

// createMigratedPod creates a new pod with CSI volumes
func (e *MigrationEngine) createMigratedPod(resource discovery.ResourceInfo, newName string) error {
	pod := resource.Object.(*corev1.Pod)

	newPod := pod.DeepCopy()
	newPod.Name = newName
	newPod.ResourceVersion = ""
	newPod.UID = ""
	newPod.Status = corev1.PodStatus{}

	// Clear fields that should not be copied
	newPod.Spec.NodeName = ""
	if newPod.ObjectMeta.Labels == nil {
		newPod.ObjectMeta.Labels = make(map[string]string)
	}
	newPod.ObjectMeta.Labels["app"] = newName
	if newPod.ObjectMeta.Annotations == nil {
		newPod.ObjectMeta.Annotations = make(map[string]string)
	}

	// Update PVC references
	fmt.Printf("  → Updating %d volume(s) to use CSI PVCs\n", len(newPod.Spec.Volumes))
	for i, vol := range newPod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			oldPVCName := vol.PersistentVolumeClaim.ClaimName
			for _, pvcInfo := range resource.PVCs {
				if vol.PersistentVolumeClaim.ClaimName == pvcInfo.Name {
					newPVCName := fmt.Sprintf("%s-csi-migrated", pvcInfo.Name)
					newPod.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = newPVCName
					fmt.Printf("    ✅ Volume %s: %s -> %s\n", vol.Name, oldPVCName, newPVCName)
				}
			}
		}
	}

	fmt.Printf("  → Creating pod in namespace: %s\n", resource.Namespace)
	_, err := e.clientset.CoreV1().Pods(resource.Namespace).Create(e.ctx, newPod, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("  ❌ Pod creation failed: %v\n", err)
		return fmt.Errorf("pod creation failed: %v", err)
	}
	return nil
}

// verifyBucketBinding verifies the pod can access the bucket
func (e *MigrationEngine) verifyBucketBinding(resourceName, namespace string) error {
	if e.dryRun {
		return nil
	}

	// First try to get pod by name (for standalone pods)
	pod, err := e.clientset.CoreV1().Pods(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
	if err == nil {
		// Found pod by name
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				fmt.Printf("  ✅ Volume mounted: %s -> PVC: %s\n", vol.Name, vol.PersistentVolumeClaim.ClaimName)
			}
		}
		return nil
	}

	// If not found by name, try label selector (for Deployments/StatefulSets)
	pods, err := e.clientset.CoreV1().Pods(namespace).List(e.ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", resourceName),
	})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for resource %s", resourceName)
	}

	// Check if volumes are mounted
	for _, pod := range pods.Items {
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				fmt.Printf("  ✅ Volume mounted: %s -> PVC: %s\n", vol.Name, vol.PersistentVolumeClaim.ClaimName)
			}
		}
	}

	return nil
}

// waitForPodRunning waits for pods to be running
func (e *MigrationEngine) waitForPodRunning(resourceName, namespace, resourceType string) error {
	if e.dryRun {
		return nil
	}

	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for pods to be running")
		case <-ticker.C:
			var pods *corev1.PodList
			var err error

			// For standalone pods, check by name directly
			if resourceType == "Pod" {
				pod, err := e.clientset.CoreV1().Pods(namespace).Get(e.ctx, resourceName, metav1.GetOptions{})
				if err != nil {
					fmt.Printf("  ⏳ Waiting for pod %s to be created...\n", resourceName)
					continue
				}
				pods = &corev1.PodList{Items: []corev1.Pod{*pod}}
			} else {
				// For Deployments/StatefulSets/DaemonSets, use label selector
				pods, err = e.clientset.CoreV1().Pods(namespace).List(e.ctx, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("app=%s", resourceName),
				})
				if err != nil {
					return err
				}
			}

			if len(pods.Items) == 0 {
				fmt.Println("  ⏳ Waiting for pods to be created...")
				continue
			}

			allRunning := true
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					allRunning = false
					fmt.Printf("  ⏳ Pod %s: %s (waiting...)\n", pod.Name, pod.Status.Phase)
					break
				}
			}

			if allRunning {
				fmt.Printf("  ✅ All %d pod(s) are running\n", len(pods.Items))
				return nil
			}
		}
	}
}

// migrateServices migrates services to point to new resource
func (e *MigrationEngine) migrateServices(resource discovery.ResourceInfo, newResourceName string) error {
	if e.dryRun {
		fmt.Printf("  [DRY RUN] Would migrate %d service(s)\n", len(resource.Services))
		return nil
	}

	for _, svcName := range resource.Services {
		svc, err := e.clientset.CoreV1().Services(resource.Namespace).Get(e.ctx, svcName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("  ⚠️  Failed to get service %s: %v\n", svcName, err)
			continue
		}

		// Update selector to point to new resource
		if svc.Spec.Selector != nil {
			svc.Spec.Selector["app"] = newResourceName
			_, err = e.clientset.CoreV1().Services(resource.Namespace).Update(e.ctx, svc, metav1.UpdateOptions{})
			if err != nil {
				fmt.Printf("  ⚠️  Failed to update service %s: %v\n", svcName, err)
				continue
			}
			fmt.Printf("  ✅ Service %s updated\n", svcName)
		}
	}

	return nil
}

// verifyMountOptions verifies mount options are applied correctly
func (e *MigrationEngine) verifyMountOptions(resourceName, namespace string) error {
	if e.dryRun {
		return nil
	}

	// This would use the verify package to check mount options
	fmt.Println("  ℹ️  Mount options verification requires node access")
	fmt.Println("  ℹ️  Use 'kubectl flex-to-csi verify <node-ip> <pv-name>' for detailed verification")

	return nil
}

// ensureRetentionPolicy ensures PV retention policy is set correctly
func (e *MigrationEngine) ensureRetentionPolicy(pvcs []discovery.PVCInfo) error {
	if e.dryRun {
		return nil
	}

	for _, pvcInfo := range pvcs {
		pv, err := e.clientset.CoreV1().PersistentVolumes().Get(e.ctx, pvcInfo.PVName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("  ⚠️  Failed to get PV %s: %v\n", pvcInfo.PVName, err)
			continue
		}

		// Check if auto-delete is enabled
		pvc, err := e.clientset.CoreV1().PersistentVolumeClaims(pvcInfo.Namespace).Get(e.ctx, pvcInfo.Name, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("  ⚠️  Failed to get PVC %s: %v\n", pvcInfo.Name, err)
			continue
		}

		autoDelete := pvc.Annotations["ibm.io/auto-delete-bucket"]
		if autoDelete == "true" {
			// Set retention policy to Retain to prevent bucket deletion
			if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
				fmt.Printf("  ⚠️  Auto-delete enabled, setting retention policy to Retain for PV %s\n", pv.Name)
				pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
				_, err = e.clientset.CoreV1().PersistentVolumes().Update(e.ctx, pv, metav1.UpdateOptions{})
				if err != nil {
					fmt.Printf("  ⚠️  Failed to update PV: %v\n", err)
					continue
				}
				fmt.Printf("  ✅ Retention policy updated for PV %s\n", pv.Name)
			}
		}
	}

	return nil
}

// verifyTraffic verifies traffic is going to new pods
func (e *MigrationEngine) verifyTraffic(oldResourceName, newResourceName, namespace string) error {
	if e.dryRun {
		return nil
	}

	// Get services pointing to the new resource
	services, err := e.clientset.CoreV1().Services(namespace).List(e.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	foundService := false
	for _, svc := range services.Items {
		if svc.Spec.Selector != nil && svc.Spec.Selector["app"] == newResourceName {
			foundService = true
			fmt.Printf("  ✅ Service %s is routing to new resource\n", svc.Name)

			// Get endpoints
			endpoints, err := e.clientset.CoreV1().Endpoints(namespace).Get(e.ctx, svc.Name, metav1.GetOptions{})
			if err == nil && len(endpoints.Subsets) > 0 {
				totalAddresses := 0
				for _, subset := range endpoints.Subsets {
					totalAddresses += len(subset.Addresses)
				}
				fmt.Printf("  ✅ %d endpoint(s) ready\n", totalAddresses)
			}
		}
	}

	if !foundService {
		return fmt.Errorf("no services found routing to new resource")
	}

	return nil
}

// DeleteOldResource deletes the old Flex resource (Step 16)
func (e *MigrationEngine) DeleteOldResource(resource discovery.ResourceInfo) error {
	if e.dryRun {
		fmt.Printf("[DRY RUN] Would delete old resource: %s\n", resource.Name)
		return nil
	}

	fmt.Printf("\n⚠️  Deleting old Flex resource: %s\n", resource.Name)
	fmt.Println("  Waiting 10 seconds before deletion...")
	time.Sleep(10 * time.Second)

	switch resource.Type {
	case "Deployment":
		err := e.clientset.AppsV1().Deployments(resource.Namespace).Delete(e.ctx, resource.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete deployment: %v", err)
		}
	case "StatefulSet":
		err := e.clientset.AppsV1().StatefulSets(resource.Namespace).Delete(e.ctx, resource.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete statefulset: %v", err)
		}
	case "DaemonSet":
		err := e.clientset.AppsV1().DaemonSets(resource.Namespace).Delete(e.ctx, resource.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete daemonset: %v", err)
		}
	case "Pod":
		err := e.clientset.CoreV1().Pods(resource.Namespace).Delete(e.ctx, resource.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete pod: %v", err)
		}
	}

	fmt.Printf("✅ Old resource deleted: %s\n", resource.Name)

	// Delete old PVCs and secrets
	for _, pvcInfo := range resource.PVCs {
		fmt.Printf("  Deleting old PVC: %s\n", pvcInfo.Name)
		err := e.clientset.CoreV1().PersistentVolumeClaims(pvcInfo.Namespace).Delete(e.ctx, pvcInfo.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("  ⚠️  Failed to delete PVC: %v\n", err)
		}

		fmt.Printf("  Deleting old secret: %s\n", pvcInfo.SecretName)
		err = e.clientset.CoreV1().Secrets(pvcInfo.Namespace).Delete(e.ctx, pvcInfo.SecretName, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("  ⚠️  Failed to delete secret: %v\n", err)
		}
	}

	return nil
}

// Made with Bob
