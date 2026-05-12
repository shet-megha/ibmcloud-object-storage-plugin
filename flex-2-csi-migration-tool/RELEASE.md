# Release Guide for kubectl-flex-to-csi

This document describes how to create and manage releases for the kubectl-flex-to-csi project.

## Table of Contents

- [Overview](#overview)
- [Release Process](#release-process)
- [Versioning Strategy](#versioning-strategy)
- [Creating a Release](#creating-a-release)
- [Download and Installation](#download-and-installation)
- [kubectl Plugin Installation](#kubectl-plugin-installation)
- [Verifying Downloads](#verifying-downloads)
- [Automated Release Workflow](#automated-release-workflow)
- [Troubleshooting](#troubleshooting)

## Overview

The kubectl-flex-to-csi project uses **semantic versioning** (SemVer) and automated GitHub Actions workflows to build, package, and distribute releases across multiple platforms.

**This tool is designed to work as a kubectl plugin**, allowing you to run it as `kubectl flex-to-csi` after installation.

### Supported Platforms

- **Linux**: AMD64 (x86_64), ARM64
- **macOS**: Intel (AMD64), Apple Silicon (ARM64)
- **Windows**: AMD64 (x86_64)

### Release Artifacts

Each release includes:
- Pre-built binaries for all supported platforms
- Distribution packages (`.tar.gz` for Linux/macOS, `.zip` for Windows)
- SHA256 checksums for verification
- Launcher scripts for easy execution
- Documentation (README, usage guides)

## Release Process

### Workflow Overview

```
1. Developer creates and pushes a version tag (e.g., v1.0.0)
   ↓
2. GitHub Actions automatically triggers
   ↓
3. Runs tests and validation
   ↓
4. Builds binaries for all platforms
   ↓
5. Creates distribution packages
   ↓
6. Generates checksums
   ↓
7. Creates GitHub Release with all artifacts
   ↓
8. Artifacts are publicly downloadable
```

## Versioning Strategy

We follow **Semantic Versioning 2.0.0** (https://semver.org/):

```
v<MAJOR>.<MINOR>.<PATCH>

Examples:
- v1.0.0 - Initial release
- v1.0.1 - Patch release (bug fixes)
- v1.1.0 - Minor release (new features, backward compatible)
- v2.0.0 - Major release (breaking changes)
```

### Version Guidelines

- **MAJOR**: Increment for incompatible API changes or breaking changes
- **MINOR**: Increment for new features that are backward compatible
- **PATCH**: Increment for backward compatible bug fixes

### Pre-release Versions

For testing and development:
```
v1.0.0-alpha.1  - Alpha release
v1.0.0-beta.1   - Beta release
v1.0.0-rc.1     - Release candidate
```

## Creating a Release

### Prerequisites

1. Ensure all changes are committed and pushed to the main branch
2. All tests pass locally: `make test`
3. Code is properly formatted: `make fmt`
4. Update CHANGELOG.md with release notes

### Step-by-Step Release Process

#### 1. Update Version Information

Update any version references in documentation:
```bash
# Update README.md, CHANGELOG.md, etc.
git add .
git commit -m "docs: prepare for v1.0.0 release"
git push origin main
```

#### 2. Create and Push the Tag

```bash
# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0

- Feature: Add new migration capabilities
- Fix: Resolve mount options handling
- Docs: Update usage guide
"

# Push the tag to GitHub
git push origin v1.0.0
```

#### 3. Monitor the Release Workflow

1. Go to your repository on GitHub
2. Navigate to **Actions** tab
3. Watch the "Release" workflow execute
## kubectl Plugin Installation

kubectl-flex-to-csi is designed to work as a kubectl plugin. After installation, you can use it with kubectl commands.

### Quick Install (One-liner)

```bash
# Linux/macOS - Install latest version
curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/kubectl-flex-to-csi/main/install.sh | bash
```

### Manual kubectl Plugin Installation

#### Linux / macOS

```bash
# Download for your platform
curl -LO https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz

# Verify checksum
curl -LO https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256
sha256sum -c kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256

# Extract
tar -xzf kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz

# Install to PATH (requires sudo)
sudo mv kubectl-flex-to-csi-v1.0.0-linux-amd64/kubectl-flex-to-csi /usr/local/bin/
sudo chmod +x /usr/local/bin/kubectl-flex-to-csi

# Verify kubectl recognizes the plugin
kubectl plugin list | grep flex-to-csi

# Use as kubectl plugin
kubectl flex-to-csi --help
```

#### Windows

1. Download: `kubectl-flex-to-csi-v1.0.0-windows-amd64.zip`
2. Extract the ZIP file
3. Copy `kubectl-flex-to-csi.exe` to a directory in your PATH (e.g., `C:\Windows\System32`)
4. Verify: `kubectl flex-to-csi --help`

### Usage as kubectl Plugin

Once installed, use it as a kubectl plugin:

```bash
# Discovery in current namespace
kubectl flex-to-csi

# Discovery in specific namespace
kubectl flex-to-csi --namespace my-namespace

# Discovery across all namespaces
kubectl flex-to-csi --all-namespaces

# Interactive mode
kubectl flex-to-csi interactive

# Verify migration
kubectl flex-to-csi verify <node-ip> <pvc-name>
```

### Detailed Plugin Installation Guide

For comprehensive kubectl plugin installation instructions, see [KUBECTL_PLUGIN_INSTALL.md](KUBECTL_PLUGIN_INSTALL.md).

4. The workflow will:
   - Run all tests
   - Build binaries for all platforms
   - Create distribution packages
   - Generate checksums
   - Create a GitHub Release

#### 4. Verify the Release

Once the workflow completes:

1. Go to **Releases** page: `https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases`
2. Verify the new release is published
3. Check that all artifacts are present:
   - `kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz`
   - `kubectl-flex-to-csi-v1.0.0-linux-arm64.tar.gz`
   - `kubectl-flex-to-csi-v1.0.0-darwin-amd64.tar.gz`
   - `kubectl-flex-to-csi-v1.0.0-darwin-arm64.tar.gz`
   - `kubectl-flex-to-csi-v1.0.0-windows-amd64.zip`
   - Individual `.sha256` files
   - `checksums.txt` (combined checksums)

#### 5. Test the Release

Download and test the release on at least one platform:

```bash
# Download the release
wget https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz

# Verify checksum
sha256sum -c kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256

# Extract and test
tar -xzf kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz
cd kubectl-flex-to-csi-v1.0.0-linux-amd64
./kubectl-flex-to-csi --help
```

## Download and Installation

### For End Users

#### Linux (AMD64)

```bash
# Download
wget https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz

# Verify checksum
wget https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256
sha256sum -c kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256

# Extract
tar -xzf kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz
cd kubectl-flex-to-csi-v1.0.0-linux-amd64

# Run
./run.sh
```

#### macOS (Apple Silicon)

```bash
# Download
curl -LO https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-darwin-arm64.tar.gz

# Verify checksum
curl -LO https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-darwin-arm64.tar.gz.sha256
shasum -a 256 -c kubectl-flex-to-csi-v1.0.0-darwin-arm64.tar.gz.sha256

# Extract
tar -xzf kubectl-flex-to-csi-v1.0.0-darwin-arm64.tar.gz
cd kubectl-flex-to-csi-v1.0.0-darwin-arm64

# Run
./run.sh
```

#### Windows (AMD64)

1. Download: https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-windows-amd64.zip
2. Extract the ZIP file
3. Open the extracted folder
4. Double-click `run.bat` or run in PowerShell

### Using curl/wget with Latest Release

```bash
# Get the latest release URL
LATEST_URL=$(curl -s https://api.github.com/repos/YOUR_ORG/kubectl-flex-to-csi/releases/latest | grep "browser_download_url.*linux-amd64.tar.gz" | cut -d '"' -f 4)

# Download latest
curl -LO $LATEST_URL
```

## Verifying Downloads

### Using Individual Checksum Files

```bash
# Download the archive and its checksum
wget https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz
wget https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256

# Verify
sha256sum -c kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256
```

### Using Combined Checksums File

```bash
# Download the combined checksums file
wget https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/checksums.txt

# Download your platform's archive
wget https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz

# Verify against checksums.txt
sha256sum kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz
grep linux-amd64 checksums.txt
```

### Manual Verification

```bash
# Calculate checksum
sha256sum kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz

# Compare with the checksum in the release notes or checksums.txt
```

## Automated Release Workflow

### Workflow Files

The release process is automated using GitHub Actions:

- **`.github/workflows/release.yml`**: Main release workflow (triggered by tags)
- **`.github/workflows/ci.yml`**: Continuous integration (triggered by pushes/PRs)

### Release Workflow Steps

1. **Test Job**
   - Checkout code
   - Set up Go environment
   - Run tests with race detection
   - Run go vet

2. **Build Job** (Matrix strategy for all platforms)
   - Checkout code
   - Set up Go environment
   - Extract version from tag
   - Build binary for specific platform
   - Create distribution package
   - Generate SHA256 checksum
   - Upload artifacts

3. **Release Job**
   - Download all build artifacts
   - Prepare release assets
   - Generate combined checksums
   - Create release notes
   - Create GitHub Release
   - Upload all assets

### Workflow Triggers

The release workflow triggers on:
```yaml
on:
  push:
    tags:
      - 'v*.*.*'
```

This means any tag matching the pattern `v*.*.*` will trigger a release.

## Troubleshooting

### Release Workflow Failed

1. **Check the Actions tab** for error details
2. **Common issues**:
   - Tests failing: Fix tests and create a new tag
   - Build errors: Check Go version compatibility
   - Permission errors: Ensure `GITHUB_TOKEN` has write permissions

### Tag Already Exists

If you need to recreate a release:

```bash
# Delete local tag
git tag -d v1.0.0

# Delete remote tag
git push origin :refs/tags/v1.0.0

# Delete the GitHub release (via web UI or API)

# Create new tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### Checksum Verification Failed

1. Re-download the file (may be corrupted)
2. Ensure you're using the correct checksum file
3. Check for network issues during download

### Binary Won't Execute

**Linux/macOS**:
```bash
# Make executable
chmod +x kubectl-flex-to-csi

# Check for missing dependencies
ldd kubectl-flex-to-csi  # Linux
otool -L kubectl-flex-to-csi  # macOS
```

**macOS Security**:
```bash
# If blocked by Gatekeeper
xattr -d com.apple.quarantine kubectl-flex-to-csi
```

**Windows**:
- Check Windows Defender/antivirus
- Run as Administrator if needed

## Best Practices

### Before Creating a Release

- [ ] All tests pass locally
- [ ] Code is properly formatted
- [ ] CHANGELOG.md is updated
- [ ] Documentation is up to date
- [ ] Version numbers are consistent
- [ ] Breaking changes are documented

### Release Checklist

- [ ] Create and push version tag
- [ ] Monitor GitHub Actions workflow
- [ ] Verify all artifacts are present
- [ ] Test download and installation
- [ ] Verify checksums
- [ ] Update documentation if needed
- [ ] Announce release (if applicable)

### Hotfix Releases

For urgent bug fixes:

1. Create a hotfix branch from the release tag
2. Fix the issue
3. Create a new patch version tag (e.g., v1.0.1)
4. Follow normal release process

## Additional Resources

- [Semantic Versioning](https://semver.org/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Go Release Best Practices](https://go.dev/doc/modules/release-workflow)

## Support

For issues or questions about releases:
- Open an issue: https://github.com/YOUR_ORG/kubectl-flex-to-csi/issues
- Check existing releases: https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases

---

**Last Updated**: 2026-04-27
**Maintained By**: kubectl-flex-to-csi Team