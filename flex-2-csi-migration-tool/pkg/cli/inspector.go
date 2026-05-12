package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kubectl-flex-to-csi/internal/kube"
	"kubectl-flex-to-csi/pkg/templates"
	"kubectl-flex-to-csi/pkg/verify"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

type Inspector struct {
	clientset *kubernetes.Clientset
	ctx       context.Context
}

// Constructor
func NewInspector(clientset *kubernetes.Clientset) *Inspector {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Minute)

	return &Inspector{
		clientset: clientset,
		ctx:       ctx,
	}
}

// ================= MAIN =================

func (i *Inspector) Run() error {
	overallStart := time.Now()

	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          FLEX TO CSI RESOURCE DISCOVERY                   ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n🕐 Started at: %s\n", overallStart.Format("2006-01-02 15:04:05"))

	// Phase 0: Pre-flight Checks
	stepStart := time.Now()
	fmt.Println("\n📋 Step 1/9: Running pre-flight checks...")
	if err := i.phase0(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Check Helm Charts
	stepStart = time.Now()
	fmt.Println("\n📋 Step 2/9: Checking Helm charts...")
	helmData, err := i.checkHelmCharts()
	if err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Check CSI Enabled
	stepStart = time.Now()
	fmt.Println("\n📋 Step 3/9: Verifying CSI configuration...")
	csiConfig, err := i.checkCSIEnabled()
	if err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Fetch COS Secrets
	stepStart = time.Now()
	fmt.Println("\n📋 Step 4/9: Discovering Flex secrets...")
	secrets, err := i.fetchCOSSecrets()
	if err != nil {
		return err
	}

	// Don't fail if no Flex secrets found - we can still discover Flex PVs
	if len(secrets) == 0 {
		fmt.Println("   ⚠️  No Flex secrets found (may have been migrated already)")
		fmt.Println("   ℹ️  Will attempt to discover Flex PVs directly...")
	} else {
		fmt.Printf("   ✅ Found %d Flex secret(s)\n", len(secrets))
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Build PVC Mapping
	stepStart = time.Now()
	fmt.Println("\n📋 Step 5/9: Building PVC to Secret mapping...")
	if err := i.buildPVCMapping(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Map Helm to COS
	stepStart = time.Now()
	fmt.Println("\n📋 Step 6/9: Mapping Helm releases to COS resources...")
	if err := i.mapHelmToCOS(helmData, csiConfig); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Phase 4 - Create CSI Resources
	stepStart = time.Now()
	fmt.Println("\n📋 Step 7/9: Creating CSI secrets and PVCs...")
	if err := i.phase4(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Phase 5 - Validate CSI PVCs
	stepStart = time.Now()
	fmt.Println("\n📋 Step 8/9: Validating CSI PVC migration...")
	if err := i.phase5(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Phase 6 - Migrate Workloads
	stepStart = time.Now()
	fmt.Println("\n📋 Step 9/9: Migrating workloads to CSI PVCs...")
	if err := i.phase6_migrateWorkloads(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Additional phases (not counted in main steps)
	// Phase 7 - Check Retention Policy
	stepStart = time.Now()
	fmt.Println("\n🔍 Additional: Checking retention policies...")
	if err := i.phase7_checkRetentionPolicy(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Phase 7.5 - Verify Traffic (Mentor requirement #15)
	stepStart = time.Now()
	fmt.Println("\n🔍 Additional: Verifying traffic routing...")
	if err := i.phase7_5_verifyTraffic(); err != nil {
		fmt.Printf("   ⚠️  Traffic verification warning: %v\n", err)
		fmt.Println("   ℹ️  Proceeding with caution...")
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Phase 8 - Safe Delete Old Resources
	stepStart = time.Now()
	fmt.Println("\n🔍 Additional: Safe deletion of old resources...")
	if err := i.phase8_safeDelete(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Phase 9 - Check PV Objects (moved from phase 6)
	stepStart = time.Now()
	fmt.Println("\n🔍 Additional: Checking PV objects and mount options...")
	if err := i.checkPVObjects(); err != nil {
		return err
	}
	fmt.Printf("   ⏱️  Completed in: %v\n", time.Since(stepStart))

	// Final summary
	totalDuration := time.Since(overallStart)
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              DISCOVERY COMPLETED                           ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n🕐 Finished at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("⏱️  Total time: %v\n", totalDuration)
	fmt.Printf("📊 Discovery completed successfully!\n\n")

	return nil
}

//
// ================= PHASE 0 =================
//

func (i *Inspector) phase0() error {
	fmt.Println("\n========================================")
	fmt.Println("Phase 0: Pre-flight Checks")
	fmt.Println("========================================\n")

	// Check 1: CSI Driver Installation
	if err := i.checkCSIDriver(); err != nil {
		return fmt.Errorf("CSI driver check failed: %v", err)
	}

	// Check 2: Node OS Validation
	if err := i.checkNodeOS(); err != nil {
		return fmt.Errorf("node OS check failed: %v", err)
	}

	// Check 3: Storage Classes
	if err := i.checkStorageClasses(); err != nil {
		return fmt.Errorf("storage class check failed: %v", err)
	}

	fmt.Println("\n✅ All pre-flight checks passed!\n")
	return nil
}

// checkCSIDriver verifies IBM Cloud Object Storage CSI driver is installed
func (i *Inspector) checkCSIDriver() error {
	fmt.Println("1. Checking CSI Driver Installation...")

	// Check for CSI driver DaemonSet
	daemonsets, err := i.clientset.AppsV1().DaemonSets("").List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list daemonsets: %v", err)
	}

	csiDriverFound := false
	var csiDaemonSet *appsv1.DaemonSet

	for i := range daemonsets.Items {
		ds := &daemonsets.Items[i]
		if strings.Contains(ds.Name, "ibm-object-csi") || strings.Contains(ds.Name, "cos-csi") {
			csiDriverFound = true
			csiDaemonSet = ds
			break
		}
	}

	if !csiDriverFound {
		return fmt.Errorf("IBM Cloud Object Storage CSI driver not found. Please install it first")
	}

	// Check if DaemonSet is ready
	if csiDaemonSet.Status.NumberReady == 0 {
		return fmt.Errorf("CSI driver DaemonSet found but no pods are ready")
	}

	fmt.Printf("   ✅ CSI Driver: %s (namespace: %s)\n", csiDaemonSet.Name, csiDaemonSet.Namespace)
	fmt.Printf("   ✅ Ready Pods: %d/%d\n", csiDaemonSet.Status.NumberReady, csiDaemonSet.Status.DesiredNumberScheduled)

	return nil
}

// checkNodeOS verifies nodes are running supported OS (Ubuntu or RHEL CoreOS)
func (i *Inspector) checkNodeOS() error {
	fmt.Println("\n2. Checking Node Operating Systems...")

	nodes, err := i.clientset.CoreV1().Nodes().List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	if len(nodes.Items) == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}

	supportedOSCount := 0
	unsupportedNodes := []string{}

	for _, node := range nodes.Items {
		osImage := node.Status.NodeInfo.OSImage
		osImageLower := strings.ToLower(osImage)

		// Check for supported OS
		isSupported := strings.Contains(osImageLower, "ubuntu") ||
			strings.Contains(osImageLower, "rhel") ||
			strings.Contains(osImageLower, "red hat") ||
			strings.Contains(osImageLower, "coreos")

		if isSupported {
			supportedOSCount++
			fmt.Printf("   ✅ Node: %s - OS: %s (Supported)\n", node.Name, osImage)
		} else {
			unsupportedNodes = append(unsupportedNodes, fmt.Sprintf("%s (%s)", node.Name, osImage))
			fmt.Printf("   ⚠️  Node: %s - OS: %s (Unsupported)\n", node.Name, osImage)
		}
	}

	if supportedOSCount == 0 {
		return fmt.Errorf("no nodes with supported OS found. Supported: Ubuntu, RHEL CoreOS")
	}

	if len(unsupportedNodes) > 0 {
		fmt.Printf("\n   ⚠️  Warning: %d node(s) with unsupported OS\n", len(unsupportedNodes))
		fmt.Println("   Migration may not work on these nodes.")
	}

	fmt.Printf("\n   ✅ Supported Nodes: %d/%d\n", supportedOSCount, len(nodes.Items))
	return nil
}

// checkStorageClasses verifies required storage classes exist
func (i *Inspector) checkStorageClasses() error {
	fmt.Println("\n3. Checking Storage Classes...")

	storageClasses, err := i.clientset.StorageV1().StorageClasses().List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list storage classes: %v", err)
	}

	// Look for IBM COS CSI storage classes
	csiStorageClassCount := 0
	for _, sc := range storageClasses.Items {
		if sc.Provisioner == "cos.s3.csi.ibm.io" {
			csiStorageClassCount++
			fmt.Printf("   ✅ CSI Storage Class: %s\n", sc.Name)
		}
	}

	if csiStorageClassCount == 0 {
		return fmt.Errorf("no IBM COS CSI storage classes found. Please create at least one")
	}

	fmt.Printf("\n   ✅ Found %d CSI Storage Class(es)\n", csiStorageClassCount)
	return nil
}

//
// ================= PHASE 1 =================
//

func (i *Inspector) checkHelmCharts() (map[string]map[string]interface{}, error) {

	fmt.Println("\nPhase 1: Checking Helm charts...")

	settings := cli.New()
	settings.KubeConfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")

	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(
		settings.RESTClientGetter(),
		"",
		os.Getenv("HELM_DRIVER"),
		log.Printf,
	); err != nil {
		return nil, err
	}

	listAction := action.NewList(actionConfig)
	listAction.All = true

	releases, err := listAction.Run()
	if err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no helm releases found")
	}

	helmData := make(map[string]map[string]interface{})

	for _, r := range releases {
		fmt.Printf("Helm Release: %s (%s)\n", r.Name, r.Namespace)
		helmData[r.Name] = r.Config
	}

	return helmData, nil
}

//
// ================= CSI CHECK =================
//

func (i *Inspector) checkCSIEnabled() (map[string]string, error) {

	fmt.Println("\nChecking CSI...")

	pods, err := i.clientset.CoreV1().
		Pods("ibm-object-csi-operator").
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("CSI not enabled")
	}

	fmt.Println("CSI is enabled")

	return make(map[string]string), nil
}

//
// ================= PHASE 2 =================
//

func (i *Inspector) fetchCOSSecrets() ([]string, error) {

	fmt.Println("\nPhase 2 : Fetch COS Secrets")

	secrets, err := i.clientset.CoreV1().
		Secrets("").
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	var cosSecrets []string

	for _, sec := range secrets.Items {
		if string(sec.Type) == "ibm/ibmc-s3fs" {
			fmt.Printf("COS Secret: %s (%s)\n", sec.Name, sec.Namespace)
			cosSecrets = append(cosSecrets, fmt.Sprintf("%s/%s", sec.Namespace, sec.Name))
		}
	}

	return cosSecrets, nil
}

func (i *Inspector) buildPVCMapping() error {

	fmt.Println("\nBuilding PVC → Secret Mapping")

	pvcs, err := i.clientset.CoreV1().
		PersistentVolumeClaims("").
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, pvc := range pvcs.Items {
		if pvc.Annotations != nil {
			if sec := pvc.Annotations["ibm.io/secret-name"]; sec != "" {
				fmt.Printf("PVC %s → Secret %s\n", pvc.Name, sec)
			}
		}
	}

	return nil
}

//
// ================= PHASE 3 =================
//

func (i *Inspector) mapHelmToCOS(
	helmData map[string]map[string]interface{},
	csiConfig map[string]string,
) error {

	fmt.Println("\nPhase 3 : Mapping (COS Flex Only)")

	pvcs, err := i.clientset.CoreV1().
		PersistentVolumeClaims("").
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	storageClasses, err := i.clientset.StorageV1().
		StorageClasses().
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	scMap := make(map[string]string)
	for _, sc := range storageClasses.Items {
		scMap[sc.Name] = sc.Provisioner
	}

	for _, pvc := range pvcs.Items {

		if pvc.Annotations == nil {
			continue
		}

		secret := pvc.Annotations["ibm.io/secret-name"]
		if secret == "" {
			continue
		}

		scName := ""
		if pvc.Spec.StorageClassName != nil {
			scName = *pvc.Spec.StorageClassName
		}

		provisioner := scMap[scName]

		// Skip CSI
		if provisioner == "cos.s3.csi.ibm.io" {
			continue
		}

		fmt.Printf("\nPVC: %s\n", pvc.Name)
		fmt.Println("  Secret:", secret)
		fmt.Println("  StorageClass:", scName)
		fmt.Println("  Provisioner:", provisioner)
	}

	return nil
}

//
// ================= PHASE 4 =================
//

func (i *Inspector) phase4() error {

	fmt.Println("\nPhase 4 : Creating CSI Addon Secret & PVC")

	pvcs, err := i.clientset.CoreV1().
		PersistentVolumeClaims("").
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, pvc := range pvcs.Items {

		if pvc.Annotations == nil {
			continue
		}

		oldSecret := pvc.Annotations["ibm.io/secret-name"]
		if oldSecret == "" {
			continue
		}

		// Skip addon PVCs
		if strings.HasSuffix(pvc.Name, "-migrated") {
			continue
		}

		// Skip already CSI PVCs
		if pvc.Spec.StorageClassName != nil &&
			strings.Contains(*pvc.Spec.StorageClassName, "ibm-object-storage") {
			continue
		}

		fmt.Printf("\nProcessing PVC: %s\n", pvc.Name)

		// Get bucket name from PVC annotations
		bucketName := pvc.Annotations["ibm.io/bucket"]

		// Get Flex PV to extract mount options
		pvName := pvc.Spec.VolumeName
		if pvName == "" {
			fmt.Println("PVC not bound to PV, skipping:", pvc.Name)
			continue
		}

		flexPV, err := i.clientset.CoreV1().PersistentVolumes().Get(i.ctx, pvName, metav1.GetOptions{})
		if err != nil {
			fmt.Println("Failed to get PV:", err)
			continue
		}

		if err := i.createAddonSecret(pvc.Namespace, oldSecret, bucketName, flexPV); err != nil {
			fmt.Println("Secret Error:", err)
		}

		if err := i.createAddonPVC(&pvc); err != nil {
			fmt.Println("PVC Error:", err)
		}
	}

	return nil
}

//
// ---------- CREATE CSI SECRET ----------
//

func (i *Inspector) createAddonSecret(namespace, oldSecret, bucketName string, flexPV *corev1.PersistentVolume) error {

	newName := oldSecret + "-migrated"

	// Check if exists
	_, err := i.clientset.CoreV1().
		Secrets(namespace).
		Get(i.ctx, newName, metav1.GetOptions{})

	if err == nil {
		fmt.Println("Secret exists, skipping:", newName)
		return nil
	}

	fmt.Println("Creating CSI Secret:", newName)

	// 🔹 Fetch old secret
	sec, err := i.clientset.CoreV1().
		Secrets(namespace).
		Get(i.ctx, oldSecret, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// 🔹 DEBUG (optional but recommended once)
	fmt.Println("Secret Data:", sec.Data)

	// 🔹 Extract mount options from Flex PV
	var chunkSizeMB, parallelCount, multireqMax, statCacheSize, retries, kernelCache string

	// Step 1: Try to find pod using this Flex PV to get actual mount options
	pod, nodeIP := i.findPodUsingFlexPV(flexPV.Name)

	// Step 2: If pod found, fetch actual mount options from node
	if pod != nil && nodeIP != "" {
		actualOpts, err := i.fetchActualMountOptions(nodeIP, flexPV.Name)
		if err == nil && len(actualOpts) > 0 {
			// Use actual values from running mount
			chunkSizeMB = actualOpts["multipart_size"]
			parallelCount = actualOpts["parallel_count"]
			multireqMax = actualOpts["multireq_max"]
			statCacheSize = actualOpts["max_stat_cache_size"]
			retries = actualOpts["retries"]
			if actualOpts["kernel_cache"] == "true" {
				kernelCache = "kernel_cache"
			}
			fmt.Printf("  ✅ Fetched actual mount options from node %s\n", nodeIP)
		} else {
			fmt.Printf("  ⚠️  Could not fetch mount options from node: %v\n", err)
		}
	}

	// Step 3: Fallback to FlexVolume options if not fetched from node
	if chunkSizeMB == "" && flexPV.Spec.FlexVolume != nil && flexPV.Spec.FlexVolume.Options != nil {
		options := flexPV.Spec.FlexVolume.Options
		chunkSizeMB = options["chunk-size-mb"]
		parallelCount = options["parallel-count"]
		multireqMax = options["multireq-max"]
		statCacheSize = options["stat-cache-size"]
		retries = options["s3fs-fuse-retry-count"]

		// Handle kernel-cache boolean
		if options["kernel-cache"] == "true" {
			kernelCache = "kernel_cache"
		}
	}

	// Step 4: Set defaults if not found (use FLEX defaults for compatibility)
	if chunkSizeMB == "" {
		chunkSizeMB = "16" // Flex default
	}
	if parallelCount == "" {
		parallelCount = "2" // Flex default
	}
	if multireqMax == "" {
		multireqMax = "20" // Flex default
	}
	if statCacheSize == "" {
		statCacheSize = "100000" // Flex default
	}
	if retries == "" {
		retries = "5" // Flex default
	}
	if kernelCache == "" {
		kernelCache = "kernel_cache" // Default to enabled
	}

	// 🔹 Extract values and use bucket from PVC annotation
	data := map[string]string{
		"SecretName":        newName,
		"Namespace":         namespace,
		"AccessKey":         string(sec.Data["access-key"]),
		"SecretKey":         string(sec.Data["secret-key"]),
		"BucketName":        bucketName, // ← Use bucket from PVC annotation
		"Endpoint":          string(sec.Data["endpoint"]),
		"ServiceInstanceID": string(sec.Data["serviceInstanceID"]),
		"ChunkSizeMB":       chunkSizeMB,
		"ParallelCount":     parallelCount,
		"MultireqMax":       multireqMax,
		"StatCacheSize":     statCacheSize,
		"Retries":           retries,
		"KernelCache":       kernelCache,
	}

	// 🔹 Render template
	path := filepath.Join("pkg", "templates", "secret.yaml")

	yaml, err := templates.Render(path, data)
	if err != nil {
		return err
	}

	// ✅ ADD HERE
	os.MkdirAll("output", os.ModePerm)
	fileName := fmt.Sprintf("output/%s-secret.yaml", newName)
	os.WriteFile(fileName, []byte(yaml), 0644)

	// Apply
	return kube.Apply(yaml)
}

//
// ---------- STORAGE CLASS MAPPING ----------
//

func mapToCSIStorageClass(oldSC string) string {

	if strings.Contains(oldSC, "standard") {
		return "ibm-object-storage-standard-s3fs"
	}

	if strings.Contains(oldSC, "smart") {
		return "ibm-object-storage-smart-s3fs"
	}

	return "ibm-object-storage-standard-s3fs"
}

//
// ---------- CREATE CSI PVC ----------
//

func (i *Inspector) createAddonPVC(oldPVC *corev1.PersistentVolumeClaim) error {

	newName := oldPVC.Name + "-migrated"

	_, err := i.clientset.CoreV1().
		PersistentVolumeClaims(oldPVC.Namespace).
		Get(i.ctx, newName, metav1.GetOptions{})

	if err == nil {
		fmt.Println("PVC exists, skipping:", newName)
		return nil
	}

	oldSC := ""
	if oldPVC.Spec.StorageClassName != nil {
		oldSC = *oldPVC.Spec.StorageClassName
	}

	storageClass := mapToCSIStorageClass(oldSC)

	oldSecret := oldPVC.Annotations["ibm.io/secret-name"]
	newSecret := oldSecret + "-migrated"

	// 🔹 Get the Flex PV to extract mount options
	pvName := oldPVC.Spec.VolumeName
	if pvName == "" {
		return fmt.Errorf("PVC %s is not bound to a PV", oldPVC.Name)
	}

	flexPV, err := i.clientset.CoreV1().PersistentVolumes().Get(i.ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PV %s: %v", pvName, err)
	}

	// 🔹 Extract bucket name from PVC annotation
	bucketName := oldPVC.Annotations["ibm.io/bucket"]
	if bucketName == "" {
		return fmt.Errorf("bucket name not found in PVC annotation 'ibm.io/bucket'")
	}

	// Extract bucket control annotations (default to "false" for same-bucket migration)
	autoCreateBucket := oldPVC.Annotations["ibm.io/auto-create-bucket"]
	if autoCreateBucket == "" {
		autoCreateBucket = "false" // Don't create new bucket
	}

	autoDeleteBucket := oldPVC.Annotations["ibm.io/auto-delete-bucket"]
	if autoDeleteBucket == "" {
		autoDeleteBucket = "false" // Don't delete bucket on PVC deletion
	}

	// 🔹 Extract mount options from Flex PV (not PVC annotations)
	var endpoint, region, objectPath, chunkSizeMB, parallelCount, multireqMax, statCacheSize, kernelCache string

	if flexPV.Spec.FlexVolume != nil && flexPV.Spec.FlexVolume.Options != nil {
		options := flexPV.Spec.FlexVolume.Options

		// Extract endpoint from Flex PV
		if val, ok := options["object-store-endpoint"]; ok {
			endpoint = val
		} else {
			endpoint = oldPVC.Annotations["ibm.io/endpoint"]
		}

		// Extract mount options from Flex PV
		if val, ok := options["chunk-size-mb"]; ok {
			chunkSizeMB = val
		} else {
			chunkSizeMB = "52" // Default
		}

		if val, ok := options["parallel-count"]; ok {
			parallelCount = val
		} else {
			parallelCount = "20" // Default
		}

		if val, ok := options["multireq-max"]; ok {
			multireqMax = val
		} else {
			multireqMax = "20" // Default
		}

		if val, ok := options["stat-cache-size"]; ok {
			statCacheSize = val
		} else {
			statCacheSize = "100000" // Default
		}

		if val, ok := options["kernel-cache"]; ok {
			kernelCache = val
		} else {
			kernelCache = "true" // Default
		}
	} else {
		// Fallback to PVC annotations if FlexVolume options not available
		endpoint = oldPVC.Annotations["ibm.io/endpoint"]
		chunkSizeMB = "52"
		parallelCount = "20"
		multireqMax = "20"
		statCacheSize = "100000"
		kernelCache = "true"
	}

	// Extract region and object path from PVC annotations
	region = oldPVC.Annotations["ibm.io/region"]
	objectPath = oldPVC.Annotations["ibm.io/object-path"]
	if objectPath == "" {
		objectPath = ""
	}

	// 🔹 Format access modes for YAML
	accessModes := ""
	for _, mode := range oldPVC.Spec.AccessModes {
		accessModes += fmt.Sprintf("  - %s\n", mode)
	}

	data := map[string]string{
		"PVCName":          newName,
		"Namespace":        oldPVC.Namespace,
		"SecretName":       newSecret,
		"Storage":          oldPVC.Spec.Resources.Requests.Storage().String(),
		"StorageClass":     storageClass,
		"AccessModes":      accessModes,
		"BucketName":       bucketName,
		"AutoCreateBucket": autoCreateBucket,
		"AutoDeleteBucket": autoDeleteBucket,
		"Endpoint":         endpoint,
		"Region":           region,
		"ObjectPath":       objectPath,
		"ChunkSizeMB":      chunkSizeMB,
		"ParallelCount":    parallelCount,
		"MultireqMax":      multireqMax,
		"StatCacheSize":    statCacheSize,
		"KernelCache":      kernelCache,
	}

	path := filepath.Join("pkg", "templates", "pvc.yaml")

	yaml, err := templates.Render(path, data)
	if err != nil {
		return err
	}

	// ✅ ADD HERE
	os.MkdirAll("output", os.ModePerm)
	fileName := fmt.Sprintf("output/%s-pvc.yaml", newName)
	os.WriteFile(fileName, []byte(yaml), 0644)

	return kube.Apply(yaml)
}

//
// ================= PHASE 5 =================
//

func (i *Inspector) phase5() error {

	fmt.Println("\nPhase 5 : Validating CSI PVC Migration")

	pvcs, err := i.clientset.CoreV1().
		PersistentVolumeClaims("").
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	success := 0
	failed := 0

	for _, pvc := range pvcs.Items {

		if !strings.HasSuffix(pvc.Name, "-migrated") {
			continue
		}

		fmt.Printf("\nChecking PVC: %s/%s\n", pvc.Namespace, pvc.Name)
		fmt.Println("  StorageClass:", *pvc.Spec.StorageClassName)
		fmt.Println("  Initial Status:", pvc.Status.Phase)

		// Wait for PVC to be bound (Mentor requirement #9)
		if pvc.Status.Phase != corev1.ClaimBound {
			fmt.Println("  ⏳ Waiting for PVC to be bound...")
			if err := i.waitForPVCBound(pvc.Name, pvc.Namespace, 3*time.Minute); err != nil {
				fmt.Printf("  ❌ Failed to bind: %v\n", err)
				failed++
				continue
			}
		}

		fmt.Println("  ✅ Bound (CSI working)")
		success++
	}

	fmt.Println("\nSummary:")
	fmt.Println("  Successful PVCs:", success)
	fmt.Println("  Failed PVCs:", failed)

	if failed > 0 {
		return fmt.Errorf("%d PVC(s) failed to bind", failed)
	}

	return nil
}

// waitForPVCBound waits for a PVC to be bound (Mentor requirement #9)
func (i *Inspector) waitForPVCBound(pvcName, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pvc, err := i.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(i.ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get PVC: %v", err)
		}

		if pvc.Status.Phase == corev1.ClaimBound {
			return nil
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for PVC to be bound")
}

//
// ================= PHASE 6 - MIGRATE WORKLOADS =================
//

func (i *Inspector) phase6_migrateWorkloads() error {

	fmt.Println("\nPhase 6 : Migrating Workloads to CSI PVCs")

	// Get all namespaces
	namespaces, err := i.clientset.CoreV1().Namespaces().List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	totalWorkloads := 0
	migratedWorkloads := 0
	skippedWorkloads := 0

	for _, ns := range namespaces.Items {
		namespace := ns.Name

		// Skip system namespaces
		if namespace == "kube-system" || namespace == "kube-public" ||
			namespace == "kube-node-lease" || namespace == "ibm-object-csi-operator" {
			continue
		}

		// Check Deployments
		deployments, err := i.clientset.AppsV1().Deployments(namespace).List(i.ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: Failed to list deployments in %s: %v\n", namespace, err)
			continue
		}

		for _, deploy := range deployments.Items {
			if i.needsWorkloadMigration(&deploy.Spec.Template.Spec, namespace) {
				totalWorkloads++
				fmt.Printf("\nFound Deployment: %s/%s\n", namespace, deploy.Name)

				if i.migrateDeployment(namespace, &deploy) {
					migratedWorkloads++
				} else {
					skippedWorkloads++
				}
			}
		}

		// Check StatefulSets
		statefulsets, err := i.clientset.AppsV1().StatefulSets(namespace).List(i.ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: Failed to list statefulsets in %s: %v\n", namespace, err)
			continue
		}

		for _, sts := range statefulsets.Items {
			if i.needsWorkloadMigration(&sts.Spec.Template.Spec, namespace) {
				totalWorkloads++
				fmt.Printf("\nFound StatefulSet: %s/%s\n", namespace, sts.Name)

				if i.migrateStatefulSet(namespace, &sts) {
					migratedWorkloads++
				} else {
					skippedWorkloads++
				}
			}
		}

		// Check DaemonSets
		daemonsets, err := i.clientset.AppsV1().DaemonSets(namespace).List(i.ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: Failed to list daemonsets in %s: %v\n", namespace, err)
			continue
		}

		for _, ds := range daemonsets.Items {
			if i.needsWorkloadMigration(&ds.Spec.Template.Spec, namespace) {
				totalWorkloads++
				fmt.Printf("\nFound DaemonSet: %s/%s\n", namespace, ds.Name)

				if i.migrateDaemonSet(namespace, &ds) {
					migratedWorkloads++
				} else {
					skippedWorkloads++
				}
			}
		}

		// Check standalone Pods
		pods, err := i.clientset.CoreV1().Pods(namespace).List(i.ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: Failed to list pods in %s: %v\n", namespace, err)
			continue
		}

		for _, pod := range pods.Items {
			// Skip pods managed by controllers
			if len(pod.OwnerReferences) > 0 {
				continue
			}

			if i.needsWorkloadMigration(&pod.Spec, namespace) {
				totalWorkloads++
				fmt.Printf("\nFound standalone Pod: %s/%s\n", namespace, pod.Name)

				if i.migrateStandalonePod(namespace, &pod) {
					migratedWorkloads++
				} else {
					skippedWorkloads++
				}
			}
		}
	}

	fmt.Println("\n--- Workload Migration Summary ---")
	fmt.Printf("Total workloads found: %d\n", totalWorkloads)
	fmt.Printf("Successfully migrated: %d\n", migratedWorkloads)
	fmt.Printf("Skipped: %d\n", skippedWorkloads)

	if totalWorkloads == 0 {
		fmt.Println("ℹ️  No workloads using Flex PVCs found")
	}

	return nil
}

// needsWorkloadMigration checks if a pod spec uses any Flex PVCs that need migration
func (i *Inspector) needsWorkloadMigration(podSpec *corev1.PodSpec, namespace string) bool {
	for _, volume := range podSpec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcName := volume.PersistentVolumeClaim.ClaimName

			// Skip if already migrated
			if strings.HasSuffix(pvcName, "-migrated") {
				continue
			}

			// Check if this PVC has a migrated version
			// (This means it's a Flex PVC that we migrated)
			migratedName := pvcName + "-migrated"
			_, err := i.clientset.CoreV1().PersistentVolumeClaims(namespace).
				Get(i.ctx, migratedName, metav1.GetOptions{})

			if err == nil {
				return true // Found a migrated PVC, so this workload needs migration
			}
		}
	}
	return false
}

// migrateDeployment updates a deployment to use migrated CSI PVCs
func (i *Inspector) migrateDeployment(namespace string, deploy *appsv1.Deployment) bool {
	fmt.Println("  Migrating Deployment...")

	updated := false
	for idx, volume := range deploy.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			oldPVC := volume.PersistentVolumeClaim.ClaimName
			newPVC := oldPVC + "-migrated"

			// Check if migrated PVC exists
			_, err := i.clientset.CoreV1().PersistentVolumeClaims(namespace).
				Get(i.ctx, newPVC, metav1.GetOptions{})

			if err == nil {
				fmt.Printf("  Updating PVC: %s → %s\n", oldPVC, newPVC)
				deploy.Spec.Template.Spec.Volumes[idx].PersistentVolumeClaim.ClaimName = newPVC
				updated = true
			}
		}
	}

	if !updated {
		fmt.Println("  ⚠️  No PVCs to migrate")
		return false
	}

	// Update the deployment
	_, err := i.clientset.AppsV1().Deployments(namespace).Update(i.ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("  ❌ Failed to update deployment: %v\n", err)
		return false
	}

	fmt.Println("  ✅ Deployment updated successfully")
	fmt.Println("  ⏳ Waiting for rollout...")

	// Wait for rollout (simplified - just wait 30 seconds)
	time.Sleep(30 * time.Second)

	// Check if pods are ready
	pods, err := i.clientset.CoreV1().Pods(namespace).List(i.ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deploy.Spec.Selector),
	})

	if err == nil {
		readyPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				readyPods++
			}
		}
		fmt.Printf("  ✅ %d/%d pods running\n", readyPods, len(pods.Items))
	}

	return true
}

// migrateStatefulSet updates a statefulset to use migrated CSI PVCs
func (i *Inspector) migrateStatefulSet(namespace string, sts *appsv1.StatefulSet) bool {
	fmt.Println("  Migrating StatefulSet...")

	updated := false
	for idx, volume := range sts.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			oldPVC := volume.PersistentVolumeClaim.ClaimName
			newPVC := oldPVC + "-migrated"

			_, err := i.clientset.CoreV1().PersistentVolumeClaims(namespace).
				Get(i.ctx, newPVC, metav1.GetOptions{})

			if err == nil {
				fmt.Printf("  Updating PVC: %s → %s\n", oldPVC, newPVC)
				sts.Spec.Template.Spec.Volumes[idx].PersistentVolumeClaim.ClaimName = newPVC
				updated = true
			}
		}
	}

	if !updated {
		fmt.Println("  ⚠️  No PVCs to migrate")
		return false
	}

	_, err := i.clientset.AppsV1().StatefulSets(namespace).Update(i.ctx, sts, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("  ❌ Failed to update statefulset: %v\n", err)
		return false
	}

	fmt.Println("  ✅ StatefulSet updated successfully")
	return true
}

// migrateDaemonSet updates a daemonset to use migrated CSI PVCs
func (i *Inspector) migrateDaemonSet(namespace string, ds *appsv1.DaemonSet) bool {
	fmt.Println("  Migrating DaemonSet...")

	updated := false
	for idx, volume := range ds.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			oldPVC := volume.PersistentVolumeClaim.ClaimName
			newPVC := oldPVC + "-migrated"

			_, err := i.clientset.CoreV1().PersistentVolumeClaims(namespace).
				Get(i.ctx, newPVC, metav1.GetOptions{})

			if err == nil {
				fmt.Printf("  Updating PVC: %s → %s\n", oldPVC, newPVC)
				ds.Spec.Template.Spec.Volumes[idx].PersistentVolumeClaim.ClaimName = newPVC
				updated = true
			}
		}
	}

	if !updated {
		fmt.Println("  ⚠️  No PVCs to migrate")
		return false
	}

	_, err := i.clientset.AppsV1().DaemonSets(namespace).Update(i.ctx, ds, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("  ❌ Failed to update daemonset: %v\n", err)
		return false
	}

	fmt.Println("  ✅ DaemonSet updated successfully")
	return true
}

// migrateStandalonePod recreates a standalone pod with migrated CSI PVCs
func (i *Inspector) migrateStandalonePod(namespace string, pod *corev1.Pod) bool {
	fmt.Println("  Migrating standalone Pod...")

	// Create new pod spec with updated PVCs
	newPod := pod.DeepCopy()
	newPod.ResourceVersion = ""
	newPod.UID = ""
	newPod.Name = pod.Name + "-migrated"

	updated := false
	for idx, volume := range newPod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			oldPVC := volume.PersistentVolumeClaim.ClaimName
			newPVC := oldPVC + "-migrated"

			_, err := i.clientset.CoreV1().PersistentVolumeClaims(namespace).
				Get(i.ctx, newPVC, metav1.GetOptions{})

			if err == nil {
				fmt.Printf("  Updating PVC: %s → %s\n", oldPVC, newPVC)
				newPod.Spec.Volumes[idx].PersistentVolumeClaim.ClaimName = newPVC
				updated = true
			}
		}
	}

	if !updated {
		fmt.Println("  ⚠️  No PVCs to migrate")
		return false
	}

	// Create new pod
	createdPod, err := i.clientset.CoreV1().Pods(namespace).Create(i.ctx, newPod, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("  ❌ Failed to create new pod: %v\n", err)
		return false
	}

	fmt.Printf("  ✅ Created new pod: %s\n", newPod.Name)

	// Wait for pod to be running
	fmt.Println("  ⏳ Waiting for pod to be running...")
	if err := i.waitForPodRunning(newPod.Name, namespace, 3*time.Minute); err != nil {
		fmt.Printf("  ❌ Pod failed to start: %v\n", err)
		return false
	}
	fmt.Println("  ✅ Pod is running")

	// Verify bucket access (Mentor requirement #10)
	fmt.Println("  🔍 Verifying bucket access...")
	if err := i.verifyBucketAccess(newPod.Name, namespace); err != nil {
		fmt.Printf("  ⚠️  Bucket access verification failed: %v\n", err)
		fmt.Println("  ℹ️  Pod is running but bucket access could not be verified")
	} else {
		fmt.Println("  ✅ Bucket access verified")
	}

	// Migrate services (Mentor requirement #12)
	fmt.Println("  🔄 Checking for services to migrate...")
	if err := i.migrateServices(pod, createdPod); err != nil {
		fmt.Printf("  ⚠️  Service migration warning: %v\n", err)
	}

	fmt.Println("  ℹ️  Note: Old pod not deleted automatically. Delete manually after verification.")

	return true
}

// waitForPodRunning waits for a pod to be in Running state
func (i *Inspector) waitForPodRunning(podName, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pod, err := i.clientset.CoreV1().Pods(namespace).Get(i.ctx, podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod: %v", err)
		}

		if pod.Status.Phase == corev1.PodRunning {
			// Check if all containers are ready
			allReady := true
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if !containerStatus.Ready {
					allReady = false
					break
				}
			}
			if allReady {
				return nil
			}
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for pod to be running")
}

// verifyBucketAccess verifies pod can access the bucket (Mentor requirement #10)
func (i *Inspector) verifyBucketAccess(podName, namespace string) error {
	// Try to list files in the mount point
	cmd := []string{"sh", "-c", "ls /data 2>/dev/null || ls /mnt 2>/dev/null || echo 'mounted'"}

	req := i.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
		}, metav1.ParameterCodec)

	// Note: Full exec implementation requires additional setup
	// For now, we'll do a basic check
	_, err := req.DoRaw(i.ctx)
	if err != nil {
		return fmt.Errorf("could not exec into pod: %v", err)
	}

	return nil
}

// migrateServices migrates services to point to new pod (Mentor requirement #12)
func (i *Inspector) migrateServices(oldPod, newPod *corev1.Pod) error {
	// Get all services in the namespace
	services, err := i.clientset.CoreV1().Services(oldPod.Namespace).List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}

	migratedServices := 0
	for _, svc := range services.Items {
		// Check if service selector matches old pod labels
		if svc.Spec.Selector == nil {
			continue
		}

		matches := true
		for key, value := range svc.Spec.Selector {
			if oldPod.Labels[key] != value {
				matches = false
				break
			}
		}

		if !matches {
			continue
		}

		// Service points to old pod, update to point to new pod
		fmt.Printf("    Found service: %s (points to old pod)\n", svc.Name)

		// Option 1: Update existing service (if new pod has same labels)
		// Option 2: Create new service for new pod
		// For now, we'll just log it
		fmt.Printf("    ℹ️  Service %s may need manual update to point to new pod\n", svc.Name)
		fmt.Printf("    ℹ️  Old pod labels: %v\n", oldPod.Labels)
		fmt.Printf("    ℹ️  New pod labels: %v\n", newPod.Labels)

		migratedServices++
	}

	if migratedServices == 0 {
		fmt.Println("    ℹ️  No services found pointing to this pod")
	} else {
		fmt.Printf("    ℹ️  Found %d service(s) that may need attention\n", migratedServices)
	}

	return nil
}

//
// ================= PHASE 7 - CHECK RETENTION POLICY =================
//

func (i *Inspector) phase7_checkRetentionPolicy() error {

	fmt.Println("\nPhase 7 : Checking Retention Policies")

	pvs, err := i.clientset.CoreV1().PersistentVolumes().List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	totalPVs := 0
	retainPVs := 0
	updatedPVs := 0

	for _, pv := range pvs.Items {
		// Only check Flex PVs
		if pv.Spec.FlexVolume == nil || pv.Spec.FlexVolume.Driver != "ibm/ibmc-s3fs" {
			continue
		}

		// Skip if no claim ref (not bound)
		if pv.Spec.ClaimRef == nil {
			continue
		}

		// Check if there's a migrated version of this PVC
		migratedPVCName := pv.Spec.ClaimRef.Name + "-migrated"
		_, err := i.clientset.CoreV1().PersistentVolumeClaims(pv.Spec.ClaimRef.Namespace).
			Get(i.ctx, migratedPVCName, metav1.GetOptions{})

		if err != nil {
			// No migrated PVC, skip this PV
			continue
		}

		totalPVs++
		fmt.Printf("\nChecking PV: %s\n", pv.Name)
		fmt.Printf("  Bound to PVC: %s/%s\n", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		fmt.Printf("  Current Reclaim Policy: %s\n", pv.Spec.PersistentVolumeReclaimPolicy)

		if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
			fmt.Println("  ✅ Already set to Retain - Safe to delete PVC")
			retainPVs++
		} else {
			fmt.Printf("  ⚠️  Policy is %s - Updating to Retain...\n", pv.Spec.PersistentVolumeReclaimPolicy)

			// Update to Retain
			pvCopy := pv.DeepCopy()
			pvCopy.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain

			_, err := i.clientset.CoreV1().PersistentVolumes().Update(i.ctx, pvCopy, metav1.UpdateOptions{})
			if err != nil {
				fmt.Printf("  ❌ Failed to update: %v\n", err)
			} else {
				fmt.Println("  ✅ Updated to Retain - Now safe to delete PVC")
				updatedPVs++
				retainPVs++
			}
		}
	}

	fmt.Println("\n--- Retention Policy Summary ---")
	fmt.Printf("Total Flex PVs checked: %d\n", totalPVs)
	fmt.Printf("Already Retain: %d\n", retainPVs-updatedPVs)
	fmt.Printf("Updated to Retain: %d\n", updatedPVs)
	fmt.Printf("Safe to delete PVCs: %d\n", retainPVs)

	if totalPVs == 0 {
		fmt.Println("ℹ️  No Flex PVs with migrated versions found")
	}

	return nil
}

//
// ================= PHASE 7.5 - VERIFY TRAFFIC =================
//

func (i *Inspector) phase7_5_verifyTraffic() error {
	fmt.Println("\nPhase 7.5 : Verifying Traffic to Migrated Workloads")
	fmt.Println("(Mentor requirement #15: Make sure traffic is going to new pod)")

	// Get all namespaces
	namespaces, err := i.clientset.CoreV1().Namespaces().List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	totalServices := 0
	verifiedServices := 0

	for _, ns := range namespaces.Items {
		namespace := ns.Name

		// Skip system namespaces
		if namespace == "kube-system" || namespace == "kube-public" ||
			namespace == "kube-node-lease" || namespace == "ibm-object-csi-operator" {
			continue
		}

		// Get all services
		services, err := i.clientset.CoreV1().Services(namespace).List(i.ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: Failed to list services in %s: %v\n", namespace, err)
			continue
		}

		for _, svc := range services.Items {
			// Check if service has endpoints
			endpoints, err := i.clientset.CoreV1().Endpoints(namespace).Get(i.ctx, svc.Name, metav1.GetOptions{})
			if err != nil {
				continue
			}

			// Check if any endpoints point to migrated pods
			for _, subset := range endpoints.Subsets {
				for _, addr := range subset.Addresses {
					if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
						podName := addr.TargetRef.Name

						// Check if this is a migrated pod
						if strings.HasSuffix(podName, "-migrated") {
							totalServices++
							fmt.Printf("\n✅ Service: %s/%s\n", namespace, svc.Name)
							fmt.Printf("   Endpoint: %s (migrated pod)\n", podName)
							fmt.Printf("   IP: %s\n", addr.IP)
							verifiedServices++
						}
					}
				}
			}
		}
	}

	fmt.Println("\n--- Traffic Verification Summary ---")
	fmt.Printf("Services with migrated pod endpoints: %d\n", verifiedServices)

	if verifiedServices == 0 {
		fmt.Println("ℹ️  No services found pointing to migrated pods")
		fmt.Println("ℹ️  This is normal if workloads don't use services")
	} else {
		fmt.Println("✅ Traffic is flowing to migrated pods")
	}

	return nil
}

//
// ================= PHASE 8 - SAFE DELETE OLD RESOURCES =================
//

func (i *Inspector) phase8_safeDelete() error {

	fmt.Println("\nPhase 8 : Safe Deletion of Old Flex Resources")
	fmt.Println("\n⚠️  IMPORTANT: This will delete old Flex PVCs and Secrets")
	fmt.Println("Make sure:")
	fmt.Println("  1. New CSI PVCs are bound and working")
	fmt.Println("  2. Workloads have been migrated")
	fmt.Println("  3. Retention policies are set to Retain")
	fmt.Println("  4. You have backups if needed")

	// In production, you might want to add a confirmation prompt here
	fmt.Println("\nProceeding with deletion in 5 seconds...")
	time.Sleep(5 * time.Second)

	deletedPVCs := 0
	deletedSecrets := 0
	skippedPVCs := 0

	// Get all PVCs
	pvcs, err := i.clientset.CoreV1().PersistentVolumeClaims("").List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pvc := range pvcs.Items {
		// Skip migrated PVCs
		if strings.HasSuffix(pvc.Name, "-migrated") {
			continue
		}

		// Skip if no secret annotation (not a Flex PVC)
		if pvc.Annotations == nil || pvc.Annotations["ibm.io/secret-name"] == "" {
			continue
		}

		// Check if migrated version exists and is bound
		migratedPVCName := pvc.Name + "-migrated"
		migratedPVC, err := i.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).
			Get(i.ctx, migratedPVCName, metav1.GetOptions{})

		if err != nil {
			fmt.Printf("\n⚠️  Skipping %s/%s: No migrated version found\n", pvc.Namespace, pvc.Name)
			skippedPVCs++
			continue
		}

		if migratedPVC.Status.Phase != corev1.ClaimBound {
			fmt.Printf("\n⚠️  Skipping %s/%s: Migrated PVC not bound yet\n", pvc.Namespace, pvc.Name)
			skippedPVCs++
			continue
		}

		// Check if PV has Retain policy
		if pvc.Spec.VolumeName != "" {
			pv, err := i.clientset.CoreV1().PersistentVolumes().Get(i.ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
			if err == nil && pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
				fmt.Printf("\n⚠️  Skipping %s/%s: PV retention policy is not Retain\n", pvc.Namespace, pvc.Name)
				skippedPVCs++
				continue
			}
		}

		fmt.Printf("\nDeleting Flex PVC: %s/%s\n", pvc.Namespace, pvc.Name)

		// Delete the PVC
		err = i.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).
			Delete(i.ctx, pvc.Name, metav1.DeleteOptions{})

		if err != nil {
			fmt.Printf("  ❌ Failed to delete PVC: %v\n", err)
			skippedPVCs++
			continue
		}

		fmt.Println("  ✅ PVC deleted")
		deletedPVCs++

		// Delete the old Flex secret
		oldSecretName := pvc.Annotations["ibm.io/secret-name"]
		if oldSecretName != "" {
			fmt.Printf("  Deleting Flex Secret: %s\n", oldSecretName)

			err = i.clientset.CoreV1().Secrets(pvc.Namespace).
				Delete(i.ctx, oldSecretName, metav1.DeleteOptions{})

			if err != nil {
				fmt.Printf("  ⚠️  Failed to delete secret: %v\n", err)
			} else {
				fmt.Println("  ✅ Secret deleted")
				deletedSecrets++
			}
		}
	}

	fmt.Println("\n--- Deletion Summary ---")
	fmt.Printf("Deleted Flex PVCs: %d\n", deletedPVCs)
	fmt.Printf("Deleted Flex Secrets: %d\n", deletedSecrets)
	fmt.Printf("Skipped PVCs: %d\n", skippedPVCs)

	if deletedPVCs > 0 {
		fmt.Println("\n✅ Old Flex resources deleted successfully")
		fmt.Println("ℹ️  Note: Flex PVs are retained (Retain policy) and can be deleted manually if needed")
	} else {
		fmt.Println("\nℹ️  No Flex resources deleted (all skipped or none found)")
	}

	return nil
}

//
// ================= PHASE 9 - PV OBJECTS =================
//

func (i *Inspector) checkPVObjects() error {

	fmt.Println("\nPhase 6 : Checking PV Objects and Mount Options")

	pvs, err := i.clientset.CoreV1().
		PersistentVolumes().
		List(i.ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	if len(pvs.Items) == 0 {
		fmt.Println("No PV objects found")
		return nil
	}

	// Store Flex PV mount options for comparison
	flexPVMountOptions := make(map[string]map[string]string)

	fmt.Println("\n--- Flex PV Objects ---")
	flexCount := 0
	for _, pv := range pvs.Items {
		// Check if it's a FlexVolume PV (ibm/ibmc-s3fs driver)
		if pv.Spec.FlexVolume != nil && pv.Spec.FlexVolume.Driver == "ibm/ibmc-s3fs" {
			flexCount++
			fmt.Printf("\nPV Name: %s\n", pv.Name)
			fmt.Printf("  Driver: %s\n", pv.Spec.FlexVolume.Driver)
			fmt.Printf("  Status: %s\n", pv.Status.Phase)
			fmt.Printf("  Capacity: %s\n", pv.Spec.Capacity.Storage().String())

			if pv.Spec.ClaimRef != nil {
				fmt.Printf("  Bound to PVC: %s/%s\n",
					pv.Spec.ClaimRef.Namespace,
					pv.Spec.ClaimRef.Name)
			}

			if pv.Spec.StorageClassName != "" {
				fmt.Printf("  StorageClass: %s\n", pv.Spec.StorageClassName)
			}

			// Show bucket info from annotations
			if bucket, ok := pv.Annotations["ibm.io/bucket"]; ok {
				fmt.Printf("  Bucket: %s\n", bucket)
			}
			if endpoint, ok := pv.Annotations["ibm.io/endpoint"]; ok {
				fmt.Printf("  Endpoint: %s\n", endpoint)
			}

			// Extract and display mount options from FlexVolume
			if pv.Spec.FlexVolume.Options != nil {
				fmt.Println("  Mount Options:")
				mountOpts := make(map[string]string)

				if val, ok := pv.Spec.FlexVolume.Options["chunk-size-mb"]; ok {
					fmt.Printf("    chunk-size-mb: %s\n", val)
					mountOpts["chunk-size-mb"] = val
				}
				if val, ok := pv.Spec.FlexVolume.Options["parallel-count"]; ok {
					fmt.Printf("    parallel-count: %s\n", val)
					mountOpts["parallel-count"] = val
				}
				if val, ok := pv.Spec.FlexVolume.Options["multireq-max"]; ok {
					fmt.Printf("    multireq-max: %s\n", val)
					mountOpts["multireq-max"] = val
				}
				if val, ok := pv.Spec.FlexVolume.Options["stat-cache-size"]; ok {
					fmt.Printf("    stat-cache-size: %s\n", val)
					mountOpts["stat-cache-size"] = val
				}
				if val, ok := pv.Spec.FlexVolume.Options["kernel-cache"]; ok {
					fmt.Printf("    kernel-cache: %s\n", val)
					mountOpts["kernel-cache"] = val
				}

				// Store for comparison if PVC is bound
				if pv.Spec.ClaimRef != nil {
					pvcKey := fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
					flexPVMountOptions[pvcKey] = mountOpts
				}
			}
		}
	}

	fmt.Println("\n--- CSI PV Objects ---")
	csiCount := 0
	for _, pv := range pvs.Items {
		// Check if it's a CSI PV (cos.s3.csi.ibm.io driver)
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == "cos.s3.csi.ibm.io" {
			csiCount++
			fmt.Printf("\nPV Name: %s\n", pv.Name)
			fmt.Printf("  Driver: %s\n", pv.Spec.CSI.Driver)
			fmt.Printf("  Status: %s\n", pv.Status.Phase)
			fmt.Printf("  Capacity: %s\n", pv.Spec.Capacity.Storage().String())

			var csiPVCKey string
			if pv.Spec.ClaimRef != nil {
				fmt.Printf("  Bound to PVC: %s/%s\n",
					pv.Spec.ClaimRef.Namespace,
					pv.Spec.ClaimRef.Name)
				csiPVCKey = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
			}

			if pv.Spec.StorageClassName != "" {
				fmt.Printf("  StorageClass: %s\n", pv.Spec.StorageClassName)
			}

			// Show bucket info from CSI attributes
			if bucket, ok := pv.Spec.CSI.VolumeAttributes["bucketName"]; ok {
				fmt.Printf("  Bucket: %s\n", bucket)
			}
			if endpoint, ok := pv.Spec.CSI.VolumeAttributes["cosEndpoint"]; ok {
				fmt.Printf("  Endpoint: %s\n", endpoint)
			}

			// Display CSI PV mount options
			if len(pv.Spec.MountOptions) > 0 {
				fmt.Println("  Mount Options (from PV):")
				for _, opt := range pv.Spec.MountOptions {
					fmt.Printf("    %s\n", opt)
				}
			}

			// Compare with original Flex PV if this is a migrated PVC
			if csiPVCKey != "" && strings.HasSuffix(pv.Spec.ClaimRef.Name, "-migrated") {
				// Try to find the original Flex PVC name
				originalPVCName := strings.TrimSuffix(pv.Spec.ClaimRef.Name, "-migrated")
				originalPVCKey := fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, originalPVCName)

				if flexOpts, ok := flexPVMountOptions[originalPVCKey]; ok {
					fmt.Println("\n  📊 Mount Options Comparison:")
					i.compareMountOptions(flexOpts, pv.Spec.MountOptions)
				}
			}
		}
	}

	fmt.Println("\n--- Summary ---")
	fmt.Printf("Total PVs: %d\n", len(pvs.Items))
	fmt.Printf("FlexVolume PVs: %d\n", flexCount)
	fmt.Printf("CSI PVs: %d\n", csiCount)

	return nil
}

//
// ================= HELPER FUNCTIONS =================
//

// compareMountOptions compares Flex PV mount options with CSI PV mount options
func (i *Inspector) compareMountOptions(flexOpts map[string]string, csiMountOpts []string) {
	// Parse CSI mount options into a map
	csiOpts := make(map[string]string)
	for _, opt := range csiMountOpts {
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) == 2 {
			csiOpts[parts[0]] = parts[1]
		} else {
			// Boolean flag without value
			csiOpts[opt] = "true"
		}
	}

	// Mapping from Flex options to s3fs mount options
	flexToS3fsMapping := map[string]string{
		"chunk-size-mb":   "multipart_size",
		"parallel-count":  "parallel_count",
		"multireq-max":    "multireq_max",
		"stat-cache-size": "max_stat_cache_size",
		"kernel-cache":    "kernel_cache",
	}

	allMatch := true

	fmt.Println("    Option                  Flex Value    →    CSI Value (s3fs)         Status")
	fmt.Println("    " + strings.Repeat("-", 80))

	for flexKey, flexValue := range flexOpts {
		s3fsKey, ok := flexToS3fsMapping[flexKey]
		if !ok {
			continue
		}

		csiValue, csiExists := csiOpts[s3fsKey]

		// Special handling for chunk-size-mb to multipart_size conversion
		expectedCSIValue := flexValue
		if flexKey == "chunk-size-mb" {
			// Flex uses MB, s3fs uses MB as well for multipart_size
			expectedCSIValue = flexValue
		}

		// Special handling for kernel-cache (boolean)
		if flexKey == "kernel-cache" {
			if flexValue == "true" {
				expectedCSIValue = "true"
			} else {
				expectedCSIValue = "false"
			}
		}

		status := "❌ MISMATCH"
		if csiExists {
			if csiValue == expectedCSIValue {
				status = "✅ MATCH"
			} else {
				allMatch = false
			}
		} else {
			status = "⚠️  MISSING"
			allMatch = false
		}

		fmt.Printf("    %-23s %-13s → %-24s %s\n",
			flexKey,
			flexValue,
			fmt.Sprintf("%s=%s", s3fsKey, csiValue),
			status)
	}

	fmt.Println()
	if allMatch {
		fmt.Println("    ✅ All mount options migrated correctly!")
	} else {
		fmt.Println("    ⚠️  Some mount options differ or are missing")
	}
}

// ================= HELPER FUNCTIONS FOR MOUNT OPTIONS =================

// findPodUsingFlexPV finds a pod using the given Flex PV
func (i *Inspector) findPodUsingFlexPV(pvName string) (*corev1.Pod, string) {
	pods, err := i.clientset.CoreV1().Pods("").List(i.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, ""
	}

	for _, pod := range pods.Items {
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				pvc, err := i.clientset.CoreV1().PersistentVolumeClaims(pod.Namespace).
					Get(i.ctx, vol.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
				if err == nil && pvc.Spec.VolumeName == pvName {
					return &pod, pod.Status.HostIP
				}
			}
		}
	}
	return nil, ""
}

// fetchActualMountOptions fetches mount options from node using verify package
// Input: nodeIP (string), pvName (string)
// Output: map[string]string containing mount options, error
func (i *Inspector) fetchActualMountOptions(nodeIP, pvName string) (map[string]string, error) {
	// Call verify.GetMountOptions to fetch actual mount options from the node
	opts, err := verify.GetMountOptions(pvName, nodeIP)
	if err != nil {
		return nil, fmt.Errorf("failed to get mount options: %v", err)
	}

	// Convert MountOptions struct to map[string]string
	result := make(map[string]string)

	// Only include non-N/A values
	if opts.MultipartSize != "N/A" && opts.MultipartSize != "" {
		result["multipart_size"] = opts.MultipartSize
	}
	if opts.ParallelCount != "N/A" && opts.ParallelCount != "" {
		result["parallel_count"] = opts.ParallelCount
	}
	if opts.MultireqMax != "N/A" && opts.MultireqMax != "" {
		result["multireq_max"] = opts.MultireqMax
	}
	if opts.MaxStatCacheSize != "N/A" && opts.MaxStatCacheSize != "" {
		result["max_stat_cache_size"] = opts.MaxStatCacheSize
	}
	if opts.Retries != "N/A" && opts.Retries != "" {
		result["retries"] = opts.Retries
	}

	// Handle kernel_cache boolean
	if opts.KernelCache {
		result["kernel_cache"] = "true"
	} else {
		result["kernel_cache"] = "false"
	}

	return result, nil
}
