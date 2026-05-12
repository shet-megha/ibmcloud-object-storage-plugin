package verify

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// MountOptions represents the essential mount options extracted from s3fs process
type MountOptions struct {
	PVCName          string
	PVName           string
	Namespace        string
	MultipartSize    string
	ParallelCount    string
	MaxStatCacheSize string
	Retries          string
	MaxDirtyData     string
	MultireqMax      string
	KernelCache      bool
}

// Run executes the verify command
func Run(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: kubectl flex-to-csi verify <node-ip> <pv-name>")
		fmt.Println("Example: kubectl flex-to-csi verify 10.245.128.4 test-flex-pv-migrated-2")
		os.Exit(1)
	}

	nodeIP := args[0]
	pvName := args[1]

	fmt.Printf("Verifying mount options for PV '%s' on node '%s'\n\n", pvName, nodeIP)

	// Get mount options from node
	mountOpts, err := GetMountOptions(pvName, nodeIP)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Display results
	DisplayMountOptions(mountOpts)
}

// GetMountOptions fetches mount options from s3fs process on the node
// Input: pvName (string), nodeIP (string)
// Output: *MountOptions object containing all mount options
func GetMountOptions(pvName, nodeIP string) (*MountOptions, error) {
	// Create Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	ctx := context.Background()

	// Get PV details
	pv, err := clientset.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get PV: %v", err)
	}

	// Get PVC details
	var pvcName, pvcNamespace string
	if pv.Spec.ClaimRef != nil {
		pvcName = pv.Spec.ClaimRef.Name
		pvcNamespace = pv.Spec.ClaimRef.Namespace
	} else {
		return nil, fmt.Errorf("PV has no claimRef")
	}

	// Get additional PV details
	storageClass := pv.Spec.StorageClassName
	capacity := pv.Spec.Capacity.Storage().String()
	accessModes := ""
	if len(pv.Spec.AccessModes) > 0 {
		accessModes = string(pv.Spec.AccessModes[0])
	}
	reclaimPolicy := string(pv.Spec.PersistentVolumeReclaimPolicy)

	fmt.Printf("========================================\n")
	fmt.Printf("Verification Details\n")
	fmt.Printf("========================================\n")
	fmt.Printf("PV Name:         %s\n", pvName)
	fmt.Printf("PVC Name:        %s\n", pvcName)
	fmt.Printf("Namespace:       %s\n", pvcNamespace)
	fmt.Printf("Storage Class:   %s\n", storageClass)
	fmt.Printf("Capacity:        %s\n", capacity)
	fmt.Printf("Access Mode:     %s\n", accessModes)
	fmt.Printf("Reclaim Policy:  %s\n", reclaimPolicy)
	fmt.Printf("Node IP:         %s\n", nodeIP)
	fmt.Printf("========================================\n\n")

	// Execute kubectl debug command
	fmt.Printf("Executing kubectl debug on node %s...\n\n", nodeIP)

	cmdStr := fmt.Sprintf("ps aux | grep s3fs | grep %s | grep -v grep", pvName)

	cmd := exec.Command("kubectl", "debug", fmt.Sprintf("node/%s", nodeIP),
		"-it", "--image=ubuntu", "--",
		"bash", "-c", cmdStr)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout
	execCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd = exec.CommandContext(execCtx, "kubectl", "debug", fmt.Sprintf("node/%s", nodeIP),
		"-it", "--image=ubuntu", "--",
		"bash", "-c", cmdStr)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("failed to execute kubectl debug: %v\nStderr: %s", err, stderr.String())
	}

	processLine := strings.TrimSpace(stdout.String())
	if processLine == "" {
		return nil, fmt.Errorf("no s3fs process found for PV %s on node %s", pvName, nodeIP)
	}

	fmt.Printf("Found s3fs process!\n\n")

	// Parse and populate mount options
	opts, err := ParseS3FSProcess(processLine)
	if err != nil {
		return nil, fmt.Errorf("failed to parse s3fs process: %v", err)
	}

	opts.PVName = pvName
	opts.PVCName = pvcName
	opts.Namespace = pvcNamespace

	return opts, nil
}

// ParseS3FSProcess parses s3fs process output and extracts mount options
func ParseS3FSProcess(processLine string) (*MountOptions, error) {
	opts := &MountOptions{}

	// Extract essential mount options only
	opts.MultipartSize = extractOption(processLine, "multipart_size")
	opts.ParallelCount = extractOption(processLine, "parallel_count")
	opts.MaxStatCacheSize = extractOption(processLine, "max_stat_cache_size")
	opts.Retries = extractOption(processLine, "retries")
	opts.MaxDirtyData = extractOption(processLine, "max_dirty_data")
	opts.MultireqMax = extractOption(processLine, "multireq_max")

	// Extract kernel_cache flag
	opts.KernelCache = strings.Contains(processLine, "-o kernel_cache")

	return opts, nil
}

// extractOption extracts a specific option value from the process line
func extractOption(line, optionName string) string {
	regex := regexp.MustCompile(fmt.Sprintf(`-o %s=([^\s]+)`, optionName))
	if matches := regex.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	return "N/A"
}

// DisplayMountOptions displays the mount options in a formatted way
func DisplayMountOptions(opts *MountOptions) {
	fmt.Println("========================================")
	fmt.Printf("PV Name: %s\n", opts.PVName)
	fmt.Printf("PVC Name: %s\n", opts.PVCName)
	fmt.Printf("Namespace: %s\n", opts.Namespace)

	fmt.Println("\nMount Options:")
	fmt.Println("========================================")
	fmt.Printf("  multipart_size:        %s MB\n", opts.MultipartSize)
	fmt.Printf("  parallel_count:        %s threads\n", opts.ParallelCount)
	fmt.Printf("  max_stat_cache_size:   %s entries\n", opts.MaxStatCacheSize)
	fmt.Printf("  retries:               %s attempts\n", opts.Retries)
	fmt.Printf("  max_dirty_data:        %s MB\n", opts.MaxDirtyData)
	fmt.Printf("  multireq_max:          %s requests\n", opts.MultireqMax)
	fmt.Printf("  kernel_cache:          %t\n", opts.KernelCache)
	fmt.Println("========================================")
}

// Made with Bob
