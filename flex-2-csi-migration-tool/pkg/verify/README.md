# Verify Package

The `verify` package provides functionality to verify that CSI mount options are correctly applied on Kubernetes nodes.

## Purpose

After migrating from FlexVolume to CSI, this package helps you verify that:
1. The s3fs process is running on the node
2. The mount options from the CSI Secret are being used (not the PV's mount options)
3. All expected mount options are present and correct

## Usage

```bash
# Basic usage
kubectl flex-to-csi verify <node-ip> <pv-name>

# Example

```
kubectl flex-to-csi verify 10.245.128.4 test-flex-pv-migrated-2
## What It Does

1. **Connects to Kubernetes** - Uses your kubeconfig to access the cluster
2. **Gets PV/PVC Details** - Retrieves PV and PVC information
3. **Executes kubectl debug** - Creates a debug pod on the specified node
4. **Finds s3fs Process** - Searches for the s3fs process serving your PVC
5. **Parses Mount Options** - Extracts all mount options from the process
6. **Displays Results** - Shows formatted output with all mount options

## Output Format

```
========================================
Verification Details
========================================
PV Name:      test-flex-pv-migrated-2
PVC Name:     test-flex-pvc-migrated-2
Namespace:    default
PVC UID:      88b7acb1-a2a9-4b42-af5e-a75a77f2601e
Node IP:      10.245.128.4
========================================

Executing kubectl debug on node 10.245.128.4...

Found s3fs process!

========================================
PV Name: test-flex-pv-migrated-2
PVC Name: test-flex-pvc-migrated-2
Namespace: default
PID: 109125
Started: Mar31
Type: CSI

Mount Options:
========================================

Performance Options:
  multipart_size:        64 MB
  parallel_count:        30 threads
  max_stat_cache_size:   200000 entries
  retries:               10 attempts
  max_dirty_data:        10240 MB
  multireq_max:          25 requests

Connection Options:
  endpoint:              au-syd-standard
  url:                   https://s3.direct.au-syd.cloud-object-storage.appdomain.cloud
  cipher_suites:         AESGCM

Security Options:
  default_acl:           private
  mp_umask:              002
  sigv2:                 true

Feature Flags:
  allow_other:           true
  kernel_cache:          true
  use_path_request_style: true
========================================
```

## How to Find Node IP

```bash
# List all nodes with IPs
kubectl get nodes -o wide

# Get node IP for a specific pod
kubectl get pod <pod-name> -o jsonpath='{.status.hostIP}'

# Example
kubectl get pod mount-test-pod -o jsonpath='{.status.hostIP}'
```

## Requirements

- `kubectl` must be installed and configured
- Access to the Kubernetes cluster
- Permissions to create debug pods on nodes
- The PV must be mounted on the specified node

## Troubleshooting

### Error: "no s3fs process found"

**Cause:** The PVC is not mounted on the specified node, or the node IP is incorrect.

**Solution:**
1. Check which node your pod is running on:
   ```bash
   kubectl get pod <pod-name> -o wide
   ```
2. Use that node's IP for verification

### Error: "failed to get PV"

**Cause:** The PV name is incorrect or doesn't exist.

**Solution:**
1. List all PVs:
   ```bash
   kubectl get pv
   ```
2. Use the exact PV name

### Error: "PV has no claimRef"

**Cause:** The PV is not bound to any PVC.

**Solution:**
1. Check PV status:
   ```bash
   kubectl get pv <pv-name> -o yaml
   ```
2. Ensure the PV is bound to a PVC

## Implementation Details

### Key Functions

- **`Run(args []string)`** - Main entry point for the verify command
- **`GetMountOptionsFromNode(nodeIP, pvName string)`** - Executes kubectl debug and retrieves mount options
- **`ParseS3FSProcess(processLine string)`** - Parses s3fs process output
- **`DisplayMountOptions(opts *MountOptions)`** - Formats and displays results

### How It Works

1. Uses Kubernetes client-go to access cluster
2. Retrieves PV and PVC details via API
3. Executes `kubectl debug node/<node-ip>` with Ubuntu image
4. Runs `ps aux | grep s3fs | grep <pvc-uid>` inside debug pod
5. Parses the s3fs command line to extract mount options
6. Displays formatted results

### Why kubectl debug?

The s3fs process runs on the **node**, not inside your application pod. To see it, we need node-level access, which `kubectl debug node` provides by creating a privileged pod with access to the node's processes.

## Example Workflow

```bash
# 1. Deploy your application with CSI PVC
kubectl apply -f my-app.yaml

# 2. Find the node where pod is running
NODE_IP=$(kubectl get pod my-app -o jsonpath='{.status.hostIP}')

# 3. Get the PV name
PV_NAME=$(kubectl get pvc my-pvc -o jsonpath='{.spec.volumeName}')

# 4. Verify mount options
kubectl flex-to-csi verify $NODE_IP $PV_NAME
```

## Comparison with Manual Verification

### Manual Method (Complex)
```bash
# Step 1: Find node
kubectl get pod my-app -o wide

# Step 2: Create debug pod
kubectl debug node/10.245.128.4 -it --image=ubuntu

# Step 3: Inside debug pod
ps aux | grep s3fs

# Step 4: Manually parse output
# Step 5: Compare with expected values
```

### Using verify Command (Simple)
```bash
kubectl flex-to-csi verify 10.245.128.4 my-pv
```

The verify command automates all these steps and provides formatted output!