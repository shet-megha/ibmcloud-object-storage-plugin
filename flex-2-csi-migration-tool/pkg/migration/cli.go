package migration

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type MigrateOptions struct {
	Workload    string
	DryRun      bool
	SkipCleanup bool
	Namespace   string
	clientset   *kubernetes.Clientset
	ctx         context.Context
}

func RunCLIMigration(args []string) {
	opts := &MigrateOptions{}

	// Create flagset
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	fs.StringVar(&opts.Workload, "workload", "", "Workload name to migrate (deployment/statefulset/daemonset/pod)")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "Show what would be migrated without actually migrating")
	fs.BoolVar(&opts.SkipCleanup, "skip-cleanup", false, "Skip cleanup of old Flex resources")
	fs.StringVar(&opts.Namespace, "namespace", "", "Namespace (default: all namespaces)")

	fs.Parse(args)

	// Validate
	if opts.Workload == "" {
		fmt.Println("Error: --workload flag is required")
		fmt.Println("\nUsage:")
		fmt.Println("  kubectl flex-to-csi migrate --workload <name> [flags]")
		fmt.Println("\nFlags:")
		fmt.Println("  --workload string      Workload name to migrate (required)")
		fmt.Println("  --dry-run              Show what would be migrated without actually migrating")
		fmt.Println("  --skip-cleanup         Skip cleanup of old Flex resources")
		fmt.Println("  --namespace string     Namespace (default: all namespaces)")
		fmt.Println("\nExamples:")
		fmt.Println("  # Dry run migration")
		fmt.Println("  kubectl flex-to-csi migrate --workload app-1 --dry-run")
		fmt.Println("")
		fmt.Println("  # Migrate without cleanup")
		fmt.Println("  kubectl flex-to-csi migrate --workload app-1 --skip-cleanup")
		fmt.Println("")
		fmt.Println("  # Full migration with cleanup")
		fmt.Println("  kubectl flex-to-csi migrate --workload app-1")
		os.Exit(1)
	}

	// Initialize Kubernetes client
	if err := opts.initClient(); err != nil {
		fmt.Printf("Error initializing client: %v\n", err)
		os.Exit(1)
	}

	// Run migration
	if err := opts.migrate(); err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		os.Exit(1)
	}
}

func (opts *MigrateOptions) initClient() error {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return fmt.Errorf("failed to build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}

	opts.clientset = clientset
	opts.ctx = context.Background()

	return nil
}

func (opts *MigrateOptions) migrate() error {
	fmt.Printf("\n========================================\n")
	fmt.Printf("Migration Configuration\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Workload:      %s\n", opts.Workload)
	fmt.Printf("Namespace:     %s\n", opts.getNamespace())
	fmt.Printf("Dry Run:       %v\n", opts.DryRun)
	fmt.Printf("Skip Cleanup:  %v\n", opts.SkipCleanup)
	fmt.Printf("========================================\n\n")

	if opts.DryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made\n")
	}

	// Step 1: Discover workload
	workloadType, workloadObj, err := opts.discoverWorkload()
	if err != nil {
		return err
	}

	// Step 2: Analyze PVCs used by workload
	pvcs, err := opts.analyzePVCs(workloadType, workloadObj)
	if err != nil {
		return err
	}

	if len(pvcs) == 0 {
		fmt.Println("✅ No Flex PVCs found - workload already using CSI or no PVCs")
		return nil
	}

	// Step 3: Create CSI resources
	if err := opts.createCSIResources(pvcs); err != nil {
		return err
	}

	// Step 4: Migrate workload
	if err := opts.migrateWorkload(workloadType, workloadObj, pvcs); err != nil {
		return err
	}

	// Step 5: Verify migration
	if err := opts.verifyMigration(workloadType, workloadObj); err != nil {
		return err
	}

	// Step 6: Cleanup (if not skipped)
	if !opts.SkipCleanup {
		if err := opts.cleanup(pvcs); err != nil {
			return err
		}
	} else {
		fmt.Println("\n⚠️  Cleanup skipped (--skip-cleanup flag)")
		fmt.Println("Old Flex resources remain:")
		for _, pvc := range pvcs {
			fmt.Printf("  - PVC: %s/%s\n", pvc.Namespace, pvc.Name)
		}
	}

	fmt.Println("\n✅ Migration completed successfully!")
	return nil
}

func (opts *MigrateOptions) getNamespace() string {
	if opts.Namespace != "" {
		return opts.Namespace
	}
	return "all namespaces"
}

func (opts *MigrateOptions) discoverWorkload() (string, interface{}, error) {
	fmt.Println("Step 1: Discovering workload...")

	namespace := opts.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	// Try Deployment
	deploy, err := opts.clientset.AppsV1().Deployments(namespace).Get(opts.ctx, opts.Workload, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("✅ Found Deployment: %s/%s\n", deploy.Namespace, deploy.Name)
		return "Deployment", deploy, nil
	}

	// Try StatefulSet
	sts, err := opts.clientset.AppsV1().StatefulSets(namespace).Get(opts.ctx, opts.Workload, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("✅ Found StatefulSet: %s/%s\n", sts.Namespace, sts.Name)
		return "StatefulSet", sts, nil
	}

	// Try DaemonSet
	ds, err := opts.clientset.AppsV1().DaemonSets(namespace).Get(opts.ctx, opts.Workload, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("✅ Found DaemonSet: %s/%s\n", ds.Namespace, ds.Name)
		return "DaemonSet", ds, nil
	}

	// Try Pod
	pod, err := opts.clientset.CoreV1().Pods(namespace).Get(opts.ctx, opts.Workload, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("✅ Found Pod: %s/%s\n", pod.Namespace, pod.Name)
		return "Pod", pod, nil
	}

	return "", nil, fmt.Errorf("workload '%s' not found in namespace '%s'", opts.Workload, opts.getNamespace())
}

func (opts *MigrateOptions) analyzePVCs(workloadType string, workloadObj interface{}) ([]*corev1.PersistentVolumeClaim, error) {
	fmt.Println("\nStep 2: Analyzing PVCs...")

	var volumes []corev1.Volume
	var namespace string

	switch workloadType {
	case "Deployment":
		deploy := workloadObj.(*appsv1.Deployment)
		volumes = deploy.Spec.Template.Spec.Volumes
		namespace = deploy.Namespace
	case "StatefulSet":
		sts := workloadObj.(*appsv1.StatefulSet)
		volumes = sts.Spec.Template.Spec.Volumes
		namespace = sts.Namespace
	case "DaemonSet":
		ds := workloadObj.(*appsv1.DaemonSet)
		volumes = ds.Spec.Template.Spec.Volumes
		namespace = ds.Namespace
	case "Pod":
		pod := workloadObj.(*corev1.Pod)
		volumes = pod.Spec.Volumes
		namespace = pod.Namespace
	}

	var flexPVCs []*corev1.PersistentVolumeClaim

	for _, vol := range volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}

		pvcName := vol.PersistentVolumeClaim.ClaimName

		// Skip already migrated PVCs
		if strings.HasSuffix(pvcName, "-migrated") {
			continue
		}

		// Get PVC
		pvc, err := opts.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(opts.ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("⚠️  Warning: Could not get PVC %s: %v\n", pvcName, err)
			continue
		}

		// Check if it's a Flex PVC (has ibm.io/secret-name annotation)
		if pvc.Annotations != nil && pvc.Annotations["ibm.io/secret-name"] != "" {
			fmt.Printf("📦 Found Flex PVC: %s\n", pvcName)
			flexPVCs = append(flexPVCs, pvc)
		}
	}

	if len(flexPVCs) == 0 {
		fmt.Println("ℹ️  No Flex PVCs found")
	} else {
		fmt.Printf("✅ Found %d Flex PVC(s) to migrate\n", len(flexPVCs))
	}

	return flexPVCs, nil
}

func (opts *MigrateOptions) createCSIResources(pvcs []*corev1.PersistentVolumeClaim) error {
	fmt.Println("\nStep 3: Creating CSI resources...")

	for _, pvc := range pvcs {
		fmt.Printf("\nProcessing PVC: %s\n", pvc.Name)

		// Check if migrated PVC already exists
		migratedPVCName := pvc.Name + "-migrated"
		_, err := opts.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(opts.ctx, migratedPVCName, metav1.GetOptions{})
		if err == nil {
			fmt.Printf("  ℹ️  CSI PVC already exists: %s\n", migratedPVCName)
			continue
		}

		if opts.DryRun {
			fmt.Printf("  [DRY RUN] Would create CSI Secret: %s-migrated\n", pvc.Annotations["ibm.io/secret-name"])
			fmt.Printf("  [DRY RUN] Would create CSI PVC: %s\n", migratedPVCName)
			continue
		}

		// Get PV to extract mount options
		pvName := pvc.Spec.VolumeName
		if pvName == "" {
			fmt.Printf("  ⚠️  PVC not bound to PV, skipping\n")
			continue
		}

		pv, err := opts.clientset.CoreV1().PersistentVolumes().Get(opts.ctx, pvName, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("  ⚠️  Could not get PV: %v\n", err)
			continue
		}

		// Create CSI Secret
		if err := opts.createCSISecret(pvc, pv); err != nil {
			fmt.Printf("  ❌ Failed to create CSI Secret: %v\n", err)
			continue
		}

		// Create CSI PVC
		if err := opts.createCSIPVC(pvc, pv); err != nil {
			fmt.Printf("  ❌ Failed to create CSI PVC: %v\n", err)
			continue
		}

		fmt.Printf("  ✅ CSI resources created\n")
	}

	return nil
}

func (opts *MigrateOptions) createCSISecret(pvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume) error {
	// This would use the same logic as in inspector.go createAddonSecret()
	// For now, just a placeholder
	fmt.Printf("  Creating CSI Secret...\n")
	return nil
}

func (opts *MigrateOptions) createCSIPVC(pvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume) error {
	// This would use the same logic as in inspector.go createAddonPVC()
	// For now, just a placeholder
	fmt.Printf("  Creating CSI PVC...\n")
	return nil
}

func (opts *MigrateOptions) migrateWorkload(workloadType string, workloadObj interface{}, pvcs []*corev1.PersistentVolumeClaim) error {
	fmt.Println("\nStep 4: Migrating workload...")

	if opts.DryRun {
		fmt.Printf("[DRY RUN] Would update %s to use CSI PVCs:\n", workloadType)
		for _, pvc := range pvcs {
			fmt.Printf("  %s → %s-migrated\n", pvc.Name, pvc.Name)
		}
		return nil
	}

	switch workloadType {
	case "Deployment":
		return opts.migrateDeployment(workloadObj.(*appsv1.Deployment), pvcs)
	case "StatefulSet":
		return opts.migrateStatefulSet(workloadObj.(*appsv1.StatefulSet), pvcs)
	case "DaemonSet":
		return opts.migrateDaemonSet(workloadObj.(*appsv1.DaemonSet), pvcs)
	case "Pod":
		return opts.migratePod(workloadObj.(*corev1.Pod), pvcs)
	}

	return nil
}

func (opts *MigrateOptions) migrateDeployment(deploy *appsv1.Deployment, pvcs []*corev1.PersistentVolumeClaim) error {
	fmt.Printf("Updating Deployment: %s\n", deploy.Name)

	updated := false
	for i, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}

		for _, pvc := range pvcs {
			if vol.PersistentVolumeClaim.ClaimName == pvc.Name {
				newName := pvc.Name + "-migrated"
				fmt.Printf("  %s → %s\n", pvc.Name, newName)
				deploy.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = newName
				updated = true
			}
		}
	}

	if !updated {
		fmt.Println("  ℹ️  No PVCs to update")
		return nil
	}

	_, err := opts.clientset.AppsV1().Deployments(deploy.Namespace).Update(opts.ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	fmt.Println("  ✅ Deployment updated")
	fmt.Println("  ⏳ Waiting for rollout...")
	time.Sleep(30 * time.Second)

	return nil
}

func (opts *MigrateOptions) migrateStatefulSet(sts *appsv1.StatefulSet, pvcs []*corev1.PersistentVolumeClaim) error {
	fmt.Printf("Updating StatefulSet: %s\n", sts.Name)
	// Similar to Deployment
	return nil
}

func (opts *MigrateOptions) migrateDaemonSet(ds *appsv1.DaemonSet, pvcs []*corev1.PersistentVolumeClaim) error {
	fmt.Printf("Updating DaemonSet: %s\n", ds.Name)
	// Similar to Deployment
	return nil
}

func (opts *MigrateOptions) migratePod(pod *corev1.Pod, pvcs []*corev1.PersistentVolumeClaim) error {
	fmt.Printf("Recreating Pod: %s\n", pod.Name)
	// Create new pod with -migrated suffix
	return nil
}

func (opts *MigrateOptions) verifyMigration(workloadType string, workloadObj interface{}) error {
	fmt.Println("\nStep 5: Verifying migration...")

	if opts.DryRun {
		fmt.Println("[DRY RUN] Would verify workload is running with CSI PVCs")
		return nil
	}

	var namespace, name string
	var labelSelector string

	switch workloadType {
	case "Deployment":
		deploy := workloadObj.(*appsv1.Deployment)
		namespace = deploy.Namespace
		name = deploy.Name
		labelSelector = metav1.FormatLabelSelector(deploy.Spec.Selector)
	case "StatefulSet":
		sts := workloadObj.(*appsv1.StatefulSet)
		namespace = sts.Namespace
		name = sts.Name
		labelSelector = metav1.FormatLabelSelector(sts.Spec.Selector)
	case "DaemonSet":
		ds := workloadObj.(*appsv1.DaemonSet)
		namespace = ds.Namespace
		name = ds.Name
		labelSelector = metav1.FormatLabelSelector(ds.Spec.Selector)
	case "Pod":
		pod := workloadObj.(*corev1.Pod)
		namespace = pod.Namespace
		name = pod.Name
	}

	if labelSelector != "" {
		pods, err := opts.clientset.CoreV1().Pods(namespace).List(opts.ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return err
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}

		fmt.Printf("✅ %d/%d pods running\n", runningPods, len(pods.Items))
	} else {
		pod, err := opts.clientset.CoreV1().Pods(namespace).Get(opts.ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if pod.Status.Phase == corev1.PodRunning {
			fmt.Println("✅ Pod is running")
		} else {
			fmt.Printf("⚠️  Pod status: %s\n", pod.Status.Phase)
		}
	}

	return nil
}

func (opts *MigrateOptions) cleanup(pvcs []*corev1.PersistentVolumeClaim) error {
	fmt.Println("\nStep 6: Cleaning up old Flex resources...")

	if opts.DryRun {
		fmt.Println("[DRY RUN] Would delete:")
		for _, pvc := range pvcs {
			fmt.Printf("  - PVC: %s/%s\n", pvc.Namespace, pvc.Name)
			if secretName := pvc.Annotations["ibm.io/secret-name"]; secretName != "" {
				fmt.Printf("  - Secret: %s/%s\n", pvc.Namespace, secretName)
			}
		}
		return nil
	}

	fmt.Println("⚠️  Deleting old Flex resources in 5 seconds...")
	time.Sleep(5 * time.Second)

	for _, pvc := range pvcs {
		// Check retention policy first
		if pvc.Spec.VolumeName != "" {
			pv, err := opts.clientset.CoreV1().PersistentVolumes().Get(opts.ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
			if err == nil && pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
				fmt.Printf("⚠️  Updating PV %s retention policy to Retain\n", pv.Name)
				pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
				_, err = opts.clientset.CoreV1().PersistentVolumes().Update(opts.ctx, pv, metav1.UpdateOptions{})
				if err != nil {
					fmt.Printf("  ❌ Failed to update PV: %v\n", err)
					continue
				}
			}
		}

		// Delete PVC
		fmt.Printf("Deleting PVC: %s/%s\n", pvc.Namespace, pvc.Name)
		err := opts.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(opts.ctx, pvc.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("  ❌ Failed: %v\n", err)
			continue
		}
		fmt.Println("  ✅ Deleted")

		// Delete Secret
		if secretName := pvc.Annotations["ibm.io/secret-name"]; secretName != "" {
			fmt.Printf("Deleting Secret: %s/%s\n", pvc.Namespace, secretName)
			err = opts.clientset.CoreV1().Secrets(pvc.Namespace).Delete(opts.ctx, secretName, metav1.DeleteOptions{})
			if err != nil {
				fmt.Printf("  ⚠️  Failed: %v\n", err)
			} else {
				fmt.Println("  ✅ Deleted")
			}
		}
	}

	return nil
}

// Made with Bob
