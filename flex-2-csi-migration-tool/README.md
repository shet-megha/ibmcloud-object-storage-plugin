# IBM Cloud Object Storage FlexVolume to CSI Migration Tool

A comprehensive, user-friendly CLI tool for migrating IBM Cloud Object Storage FlexVolume resources to CSI (Container Storage Interface) driver.

## 🌟 Features

- ✅ **User-Friendly Interactive Mode** - Step-by-step guided migration
- ✅ **Pre-flight Checks** - Verify CSI addon and OS compatibility
- ✅ **Automatic Discovery** - Find all Flex resources in your cluster
- ✅ **Safe Migration** - Preserves data and validates each step
- ✅ **Status Tracking** - Monitor migration progress (migrated/pending/failed)
- ✅ **Comprehensive Verification** - Validates mount options, traffic, and bucket access
- ✅ **Service Migration** - Automatically updates services to point to new resources
- ✅ **Retention Policy Management** - Protects buckets from accidental deletion
- ✅ **Detailed Reporting** - View migration status and history

## 📋 Prerequisites

- Kubernetes cluster with kubectl access
- IBM Cloud Object Storage CSI driver installed
- Supported OS: Ubuntu, RHEL, or CoreOS
- Go 1.25+ (for building from source)

## 🚀 Installation

### Download Pre-built Binaries

Download the appropriate package for your platform from the [releases page](https://github.com/balraj111/ibmcloud-object-storage-plugin/releases):

- **Linux**: `kubectl-flex-to-csi-linux.tar.gz`
- **macOS**: `kubectl-flex-to-csi-macos.zip`
- **Windows**: `kubectl-flex-to-csi-windows.zip`

#### Linux Installation

```bash
# Download and extract
wget https://github.com/balraj111/ibmcloud-object-storage-plugin/releases/latest/download/kubectl-flex-to-csi-linux.tar.gz
tar -xzf kubectl-flex-to-csi-linux.tar.gz
cd kubectl-flex-to-csi-linux

# Make executable and run
chmod +x kubectl-flex-to-csi
./kubectl-flex-to-csi --help
```

#### macOS Installation

⚠️ **Important for macOS users**: macOS may block the binary due to security settings. See [MACOS_INSTALL.md](MACOS_INSTALL.md) for detailed instructions.

```bash
# Download and extract
curl -LO https://github.com/balraj111/ibmcloud-object-storage-plugin/releases/latest/download/kubectl-flex-to-csi-macos.zip
unzip kubectl-flex-to-csi-macos.zip
cd kubectl-flex-to-csi-macos

# Run the installation script to bypass macOS security
./INSTALL.sh

# Or manually remove quarantine
xattr -d com.apple.quarantine kubectl-flex-to-csi
chmod +x kubectl-flex-to-csi
./kubectl-flex-to-csi --help
```

**📖 For detailed macOS installation instructions, see [MACOS_INSTALL.md](MACOS_INSTALL.md)**

#### Windows Installation

```powershell
# Download and extract kubectl-flex-to-csi-windows.zip
# Then run:
.\kubectl-flex-to-csi.exe --help
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/your-org/kubectl-flex-to-csi.git
cd kubectl-flex-to-csi

# Build
go build -o kubectl-flex-to-csi cmd/kubectl-flex-to-csi/main.go

# Install
sudo mv kubectl-flex-to-csi /usr/local/bin/
```

## 📖 Usage

### Interactive Mode (Recommended)

The easiest way to use the tool is through interactive mode:

```bash
kubectl flex-to-csi interactive
```

This will guide you through:
1. Pre-flight checks (CSI driver, OS compatibility)
2. Resource discovery
3. Migration process
4. Verification
5. Cleanup

### Command-Line Mode

For automation or specific tasks:

#### Discover Flex Resources

```bash
# Discover in all namespaces
kubectl flex-to-csi discover

# Discover in specific namespace
kubectl flex-to-csi discover --namespace default
```

#### Migrate Resources

```bash
# Migrate a specific workload
kubectl flex-to-csi migrate --workload my-app --namespace default

# Dry run (preview changes)
kubectl flex-to-csi migrate --workload my-app --dry-run

# Migrate without cleanup
kubectl flex-to-csi migrate --workload my-app --skip-cleanup
```

#### Verify Migration

```bash
# Verify mount options on a node
kubectl flex-to-csi verify <node-ip> <pv-name>

# Example
kubectl flex-to-csi verify 10.245.128.4 my-pv-csi-migrated
```

#### Cleanup Old Resources

```bash
kubectl flex-to-csi cleanup --workload my-app --namespace default
```

## 🔄 Migration Process

The tool performs the following steps for each resource:

### Step 1: Pre-flight Checks
- ✅ Verify CSI addon is installed
- ✅ Check node OS compatibility (Ubuntu/RHEL/CoreOS)

### Step 2: Resource Discovery
- 📦 List all Flex PVs
- 📋 Map PVs to PVCs
- 🚀 Find workloads using Flex PVCs

### Step 3: Read Flex Resources
- 📄 Read Flex secret
- 📄 Read Flex PV configuration
- 📄 Extract mount options

### Step 4: Create CSI Secret
- 🔐 Create new secret: `<old-secret-name>-csi-migrated`
- 🔄 Map Flex credentials to CSI format
- ⚙️  Convert mount options

### Step 5: Create CSI PVC
- 📦 Create new PVC: `<old-pvc-name>-csi-migrated`
- 🔄 Use CSI storage class
- 📋 Preserve annotations and settings

### Step 6: Wait for PVC Binding
- ⏳ Wait for PVC to bind to PV
- ✅ Verify bound state

### Step 7: Create New Resource
- 🚀 Create new workload: `<old-resource-name>-csi-migrated`
- 🔄 Update PVC references
- 📋 Preserve all other settings

### Step 8: Verify Bucket Binding
- 🪣 Check bucket accessibility
- ✅ Verify volume mounts

### Step 9: Check Pod Status
- 🔍 Wait for pods to be running
- ✅ Verify all replicas are ready

### Step 10: Migrate Services
- 🔗 Update service selectors
- 🔄 Point to new resource

### Step 11: Verify Mount Options
- ⚙️  Check s3fs mount options
- ✅ Validate configuration

### Step 12: Check Retention Policy
- 🛡️  Check auto-delete setting
- 🔒 Set retention policy if needed

### Step 13: Verify Traffic
- 🌐 Check service endpoints
- ✅ Verify traffic routing

### Step 14: Cleanup (Optional)
- 🗑️  Delete old Flex resources
- 🧹 Remove old PVCs and secrets

## 📊 Status Tracking

The tool tracks migration status for each resource:

- **⏳ Pending** - Resource discovered, not yet migrated
- **🔄 In Progress** - Migration currently running
- **✅ Migrated** - Successfully migrated and verified
- **❌ Failed** - Migration failed (with error details)

View status anytime with:
```bash
kubectl flex-to-csi interactive
# Select option 4: View Migration Status
```

## 🔍 Verification

### Mount Options Verification

The tool verifies that mount options are correctly applied:

```bash
kubectl flex-to-csi verify <node-ip> <pv-name>
```

This checks:
- `multipart_size` - Chunk size for uploads
- `parallel_count` - Number of parallel threads
- `max_stat_cache_size` - Stat cache size
- `retries` - Number of retry attempts
- `max_dirty_data` - Maximum dirty data
- `multireq_max` - Maximum concurrent requests
- `kernel_cache` - Kernel cache setting

### Traffic Verification

The tool verifies that traffic is flowing to new pods:
- Checks service endpoints
- Validates pod readiness
- Confirms traffic routing

## 🛡️ Safety Features

### Retention Policy Protection

Before deleting old resources, the tool:
1. Checks if `auto-delete-bucket` is enabled
2. Sets PV retention policy to `Retain` if needed
3. Prevents accidental bucket deletion

### Dry Run Mode

Test migrations without making changes:
```bash
kubectl flex-to-csi migrate --workload my-app --dry-run
```

### Skip Cleanup

Keep old resources for manual verification:
```bash
kubectl flex-to-csi migrate --workload my-app --skip-cleanup
```

## 📝 Examples

### Example 1: Interactive Migration

```bash
$ kubectl flex-to-csi interactive

╔════════════════════════════════════════════════════════════╗
║     IBM Cloud Object Storage FlexVolume to CSI            ║
║              Migration Tool v2.0                           ║
╚════════════════════════════════════════════════════════════╝

┌─────────────────────────────────────────┐
│           MAIN MENU                     │
└─────────────────────────────────────────┘

  1. 🔍 Pre-flight Checks
  2. 📋 Discover Flex Resources
  3. 🚀 Start Migration
  4. 📊 View Migration Status
  5. ✅ Verify Migrated Resources
  6. 🗑️  Cleanup Old Resources
  7. 📈 View Detailed Report
  8. ❌ Exit

Select an option (1-8): 1

╔════════════════════════════════════════════════════════════╗
║              PRE-FLIGHT CHECKS                             ║
╚════════════════════════════════════════════════════════════╝

1️⃣  Checking CSI Driver Installation...
   📦 Driver: ibm-object-csi-driver (namespace: kube-system)
   📊 Ready Pods: 3/3
   ✅ CSI Driver is installed and ready

2️⃣  Checking Node Operating Systems...
   ✅ worker-1: Ubuntu 20.04.3 LTS
   ✅ worker-2: Ubuntu 20.04.3 LTS
   ✅ worker-3: Ubuntu 20.04.3 LTS
   ✅ All nodes are running supported OS

✅ All pre-flight checks passed!
```

### Example 2: Command-Line Migration

```bash
$ kubectl flex-to-csi migrate --workload my-app --namespace default

╔════════════════════════════════════════════════════════════╗
║  Migrating: my-app (Deployment)
║  Namespace: default
╚════════════════════════════════════════════════════════════╝

Step 1: Verifying CSI addon installation...
✅ CSI addon is installed and ready

Step 2: Checking node operating systems...
✅ All nodes running supported OS

Step 3: Processing PVC: my-pvc
  → Reading Flex secret and PV...
  ✅ Resources read successfully
  → Creating CSI secret...
  ✅ CSI secret created: my-secret-csi-migrated
  → Creating CSI PVC...
  ✅ CSI PVC created: my-pvc-csi-migrated
  → Waiting for PVC to bind...
  ✅ PVC is bound

Step 8: Creating new resource with CSI volumes...
✅ New resource created: my-app-csi-migrated

Step 10: Verifying bucket binding...
✅ Bucket is accessible

Step 11: Checking pod status...
✅ Pod is running

Step 12: Migrating services...
✅ Services migrated

Step 13: Verifying mount options...
✅ Mount options verified

Step 14: Checking retention policy...
✅ Retention policy set

Step 15: Verifying traffic routing...
✅ Traffic verified

✅ Migration completed successfully in 2m15s
```

## 🐛 Troubleshooting

### CSI Driver Not Found

**Error**: `CSI driver not found or not ready`

**Solution**:
1. Install IBM Cloud Object Storage CSI driver
2. Verify driver pods are running:
   ```bash
   kubectl get pods -n kube-system | grep cos-csi
   ```

### Unsupported OS

**Error**: `no nodes with supported OS (Ubuntu/RHEL/CoreOS) found`

**Solution**:
- Ensure your nodes are running Ubuntu, RHEL, or CoreOS
- Check node OS: `kubectl get nodes -o wide`

### PVC Not Binding

**Error**: `timeout waiting for PVC to bind`

**Solution**:
1. Check PVC status: `kubectl get pvc <pvc-name>`
2. Check PVC events: `kubectl describe pvc <pvc-name>`
3. Verify CSI driver is running
4. Check storage class exists

### Pod Not Starting

**Error**: `pod failed to start`

**Solution**:
1. Check pod status: `kubectl get pod <pod-name>`
2. Check pod logs: `kubectl logs <pod-name>`
3. Check pod events: `kubectl describe pod <pod-name>`
4. Verify secret and PVC are created

## 🤝 Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## 📄 License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## 🙏 Acknowledgments

- IBM Cloud Object Storage team
- Kubernetes CSI community
- All contributors

## 📞 Support

For issues and questions:
- GitHub Issues: https://github.com/your-org/kubectl-flex-to-csi/issues
- Documentation: https://github.com/your-org/kubectl-flex-to-csi/wiki

## 🔗 Related Links

- [IBM Cloud Object Storage CSI Driver](https://github.com/IBM/ibm-object-csi-driver)
- [Kubernetes CSI Documentation](https://kubernetes-csi.github.io/docs/)
- [IBM Cloud Documentation](https://cloud.ibm.com/docs)