# Self-Upgrade Implementation - Complete Step-by-Step Guide

## ✅ Implementation Complete!

All files have been created and the self-upgrade feature is ready to use.

## 📁 Files Created

### 1. Version Package
- **File**: `version/version.go`
- **Purpose**: Stores version information that gets embedded at build time

### 2. Upgrade Package (6 files)
- **File**: `pkg/upgrade/github.go` - GitHub API client
- **File**: `pkg/upgrade/download.go` - File downloader
- **File**: `pkg/upgrade/verify.go` - Checksum verification
- **File**: `pkg/upgrade/extract.go` - Archive extraction (tar.gz/zip)
- **File**: `pkg/upgrade/replace.go` - Binary replacement
- **File**: `pkg/upgrade/upgrade.go` - Main orchestrator

### 3. Updated Files
- **File**: `cmd/kubectl-flex-to-csi/main.go` - Added upgrade command
- **File**: `Makefile` - Added version embedding with ldflags

---

## 🚀 How to Build and Test

### Step 1: Build with Version Information

```bash
cd flex-2-csi-migration-tool

# Build with a specific version
make build VERSION=1.0.0

# Or build with default "dev" version
make build
```

**What happens:**
- Go compiles the binary
- Version info is embedded using `-ldflags`
- Binary is created: `kubectl-flex-to-csi`

### Step 2: Verify Version

```bash
./kubectl-flex-to-csi version
```

**Expected output:**
```
╔════════════════════════════════════════════════════════════╗
║  IBM Cloud Object Storage FlexVolume to CSI Migration     ║
╚════════════════════════════════════════════════════════════╝

Version: v1.0.0 (commit: abc123..., built: 2024-01-15)

Features:
  • User-friendly interactive CLI
  • Comprehensive pre-flight checks
  • Automated resource discovery
  • Safe migration with verification
  • Status tracking and reporting
  • Self-upgrade capability
```

### Step 3: Test Upgrade Command

```bash
./kubectl-flex-to-csi upgrade
```

**What happens:**
1. Checks current version (v1.0.0)
2. Queries GitHub API for latest release
3. Compares versions
4. If newer version exists:
   - Downloads appropriate archive (linux/macos/windows)
   - Downloads checksum file
   - Verifies checksum
   - Extracts archive
   - Backs up current binary
   - Replaces with new version
5. Shows success message

---

## 📦 How to Create a Release

### Step 1: Update Version and Commit

```bash
cd flex-2-csi-migration-tool

# Make any final changes
git add .
git commit -m "Release v1.0.0"
git push origin main
```

### Step 2: Create and Push Tag

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0

Features:
- Self-upgrade capability
- Interactive migration
- Comprehensive verification
"

# Push tag to GitHub
git push origin v1.0.0
```

### Step 3: GitHub Actions Automatically:
1. Builds binaries for all platforms
2. Creates distribution packages (tar.gz/zip)
3. Generates SHA256 checksums
4. Creates GitHub Release
5. Uploads all artifacts

### Step 4: Verify Release

Go to: `https://github.com/shet-megha/ibmcloud-object-storage-plugin/releases`

You should see:
- `kubectl-flex-to-csi-linux.tar.gz`
- `kubectl-flex-to-csi-linux.tar.gz.sha256`
- `kubectl-flex-to-csi-macos.zip`
- `kubectl-flex-to-csi-macos.zip.sha256`
- `kubectl-flex-to-csi-windows.zip`
- `kubectl-flex-to-csi-windows.zip.sha256`

---

## 🧪 Testing the Upgrade Flow

### Scenario 1: Test with Existing Release

```bash
# Build with older version
make build VERSION=1.0.0

# Check version
./kubectl-flex-to-csi version
# Output: v1.0.0

# Run upgrade (will upgrade to latest release on GitHub)
./kubectl-flex-to-csi upgrade
```

### Scenario 2: Test When Already Latest

```bash
# Build with latest version
make build VERSION=1.1.0

# Run upgrade
./kubectl-flex-to-csi upgrade
# Output: ✅ You're already on the latest version!
```

### Scenario 3: Test Rollback

```bash
# If upgrade fails or you want to rollback
mv kubectl-flex-to-csi.backup kubectl-flex-to-csi
chmod +x kubectl-flex-to-csi

# Verify
./kubectl-flex-to-csi version
```

---

## 🔧 Build Commands Reference

### Local Development Build
```bash
make build
# Creates: kubectl-flex-to-csi (version: dev)
```

### Build with Specific Version
```bash
make build VERSION=1.0.0
# Creates: kubectl-flex-to-csi (version: 1.0.0)
```

### Build Distribution Packages
```bash
# Build all platforms
make dist VERSION=1.0.0

# Or build individually
make dist-linux VERSION=1.0.0
make dist-mac VERSION=1.0.0
make dist-windows VERSION=1.0.0
```

### Clean Build Artifacts
```bash
make clean
make dist-clean
```

---

## 📋 Complete Workflow Example

### For Developers

```bash
# 1. Navigate to project
cd flex-2-csi-migration-tool

# 2. Make changes to code
# ... edit files ...

# 3. Test locally
make build VERSION=1.0.0
./kubectl-flex-to-csi version

# 4. Commit changes
git add .
git commit -m "Add new feature"
git push origin main

# 5. Create release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 6. Wait for GitHub Actions to complete
# Check: https://github.com/balraj111/ibmcloud-object-storage-plugin/actions

# 7. Verify release created
# Check: https://github.com/balraj111/ibmcloud-object-storage-plugin/releases
```

### For Users

```bash
# 1. Download initial version
wget https://github.com/shet-megha/ibmcloud-object-storage-plugin/releases/download/v1.0.0/kubectl-flex-to-csi-linux.tar.gz

# 2. Extract
tar -xzf kubectl-flex-to-csi-linux.tar.gz
cd kubectl-flex-to-csi-linux

# 3. Use the tool
./kubectl-flex-to-csi interactive

# 4. Later, upgrade to latest version
./kubectl-flex-to-csi upgrade

# That's it! Tool automatically updates itself
```

---

## 🔍 How It Works Internally

### Version Embedding Process

```
Build Time:
┌─────────────────────────────────────────┐
│ make build VERSION=1.0.0                │
│                                         │
│ go build -ldflags                       │
│   "-X kubectl-flex-to-csi/version.      │
│    Version=1.0.0                        │
│    -X ...GitCommit=abc123               │
│    -X ...BuildDate=2024-01-15"          │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Binary: kubectl-flex-to-csi             │
│ Contains: Version="1.0.0" (embedded)    │
└─────────────────────────────────────────┘
```

### Upgrade Process Flow

```
User runs: kubectl flex-to-csi upgrade
           ↓
┌──────────────────────────────────────────┐
│ 1. Read embedded version: v1.0.0         │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 2. API Call to GitHub                    │
│    GET /repos/.../releases/latest        │
│    Response: {"tag_name": "v1.1.0"}      │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 3. Compare: v1.0.0 < v1.1.0 ✓            │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 4. Download:                             │
│    - kubectl-flex-to-csi-linux.tar.gz    │
│    - kubectl-flex-to-csi-linux.tar.gz    │
│      .sha256                             │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 5. Verify SHA256 checksum ✓              │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 6. Extract tar.gz → new binary           │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 7. Backup current:                       │
│    mv kubectl-flex-to-csi                │
│       kubectl-flex-to-csi.backup         │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 8. Replace:                              │
│    cp new-binary kubectl-flex-to-csi     │
│    chmod +x kubectl-flex-to-csi          │
└──────────────────┬───────────────────────┘
                   ↓
┌──────────────────────────────────────────┐
│ 9. Success! Now running v1.1.0           │
└──────────────────────────────────────────┘
```

---

## 🔒 Security Features

1. **Checksum Verification**: Every download is verified with SHA256
2. **HTTPS Only**: All downloads use secure connections
3. **Backup Creation**: Original binary is backed up before replacement
4. **Atomic Replacement**: Uses file rename for atomic updates
5. **Permission Validation**: Checks write access before attempting upgrade

---

## 🐛 Troubleshooting

### Issue: "failed to check for updates"
**Cause**: Network issue or GitHub API rate limit
**Solution**: 
```bash
# Check internet connection
ping github.com

# Try again after a few minutes
./kubectl-flex-to-csi upgrade
```

### Issue: "checksum verification failed"
**Cause**: Corrupted download
**Solution**:
```bash
# Clear cache and try again
rm -rf /tmp/kubectl-flex-to-csi-upgrade
./kubectl-flex-to-csi upgrade
```

### Issue: "failed to replace binary: permission denied"
**Cause**: No write permission to binary location
**Solution**:
```bash
# If installed in system directory, use sudo
sudo ./kubectl-flex-to-csi upgrade

# Or move to user directory
cp kubectl-flex-to-csi ~/bin/
~/bin/kubectl-flex-to-csi upgrade
```

### Issue: Upgrade succeeded but version still shows old
**Cause**: Running from different location
**Solution**:
```bash
# Find all instances
which -a kubectl-flex-to-csi

# Check version of each
/usr/local/bin/kubectl-flex-to-csi version
~/bin/kubectl-flex-to-csi version
```

---

## 📚 Additional Resources

### GitHub API Documentation
- [Releases API](https://docs.github.com/en/rest/releases)
- [Rate Limiting](https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting)

### Go Build Documentation
- [Build Flags](https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies)
- [ldflags](https://pkg.go.dev/cmd/link)

### Related Tools
- [goreleaser](https://goreleaser.com/) - Alternative release automation
- [equinox.io](https://equinox.io/) - Commercial update service

---

## ✅ Summary

You now have a fully functional self-upgrade system that:

1. ✅ Embeds version info at build time
2. ✅ Checks GitHub for latest releases
3. ✅ Downloads appropriate platform artifacts
4. ✅ Verifies checksums for security
5. ✅ Extracts and replaces binary safely
6. ✅ Creates backups for rollback
7. ✅ Works across Linux, macOS, and Windows

**Users can now upgrade with a single command:**
```bash
kubectl flex-to-csi upgrade
```

**No manual downloads, no complex steps - just automatic updates!** 🎉