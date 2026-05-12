# GitHub Configuration

This directory contains GitHub-specific configuration files for the kubectl-flex-to-csi project.

## Contents

### Workflows

Located in `.github/workflows/`:

#### 1. `release.yml` - Automated Release Workflow
**Trigger**: Push of version tags (e.g., `v1.0.0`)

**Purpose**: Automatically builds, packages, and publishes releases

**Features**:
- Runs tests before building
- Builds binaries for multiple platforms (Linux, macOS, Windows)
- Creates distribution packages (.tar.gz, .zip)
- Generates SHA256 checksums
- Creates GitHub Release with all artifacts
- Makes artifacts publicly downloadable

**Platforms**:
- Linux: AMD64, ARM64
- macOS: Intel (AMD64), Apple Silicon (ARM64)
- Windows: AMD64

#### 2. `ci.yml` - Continuous Integration
**Trigger**: Push to main/develop branches, Pull Requests

**Purpose**: Validates code quality and builds

**Jobs**:
- Run tests with race detection
- Run linters (go vet, go fmt, staticcheck)
- Build for all platforms
- Generate coverage reports

#### 3. `test.yml` - Comprehensive Testing
**Trigger**: All pushes and PRs

**Purpose**: Run tests across multiple platforms

**Features**:
- Cross-platform testing (Linux, macOS, Windows)
- Coverage reporting
- Integration tests (on main/develop)
- Benchmark tests (on main)

### Documentation

#### `RELEASE_QUICK_START.md`
Quick reference guide for creating releases. Contains:
- Step-by-step release process
- Versioning guidelines
- Troubleshooting tips
- Release checklist

For detailed information, see [RELEASE.md](../RELEASE.md) in the root directory.

## Workflow Status Badges

Add these to your main README.md:

```markdown
[![Release](https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions/workflows/release.yml/badge.svg)](https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions/workflows/release.yml)
[![CI](https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions/workflows/ci.yml/badge.svg)](https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions/workflows/ci.yml)
[![Tests](https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions/workflows/test.yml/badge.svg)](https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions/workflows/test.yml)
```

## How It Works

### Release Process Flow

```
Developer pushes tag (v1.0.0)
         ↓
GitHub detects tag push
         ↓
Triggers release.yml workflow
         ↓
┌─────────────────────────────┐
│  1. Test Job                │
│     - Run all tests         │
│     - Run go vet            │
└─────────────────────────────┘
         ↓
┌─────────────────────────────┐
│  2. Build Job (Matrix)      │
│     - Build for each OS     │
│     - Create packages       │
│     - Generate checksums    │
│     - Upload artifacts      │
└─────────────────────────────┘
         ↓
┌─────────────────────────────┐
│  3. Release Job             │
│     - Download artifacts    │
│     - Create release notes  │
│     - Publish GitHub Release│
│     - Upload all assets     │
└─────────────────────────────┘
         ↓
Release is live and downloadable!
```

### CI Process Flow

```
Developer pushes code or creates PR
         ↓
GitHub triggers ci.yml workflow
         ↓
┌─────────────────────────────┐
│  1. Test Job                │
│     - Run tests             │
│     - Generate coverage     │
└─────────────────────────────┘
         ↓
┌─────────────────────────────┐
│  2. Lint Job                │
│     - go vet                │
│     - go fmt check          │
│     - staticcheck           │
└─────────────────────────────┘
         ↓
┌─────────────────────────────┐
│  3. Build Job (Matrix)      │
│     - Build for all OS      │
│     - Verify builds work    │
└─────────────────────────────┘
         ↓
All checks pass ✅
```

## Configuration

### Required Secrets

No additional secrets are required. The workflows use the default `GITHUB_TOKEN` which is automatically provided by GitHub Actions.

### Permissions

The release workflow requires `contents: write` permission to create releases and upload assets. This is configured in the workflow file.

### Branch Protection

Recommended branch protection rules for `main`:

- Require pull request reviews
- Require status checks to pass (CI workflow)
- Require branches to be up to date
- Include administrators

## Customization

### Modify Build Targets

To add or remove build targets, edit the matrix in `release.yml`:

```yaml
strategy:
  matrix:
    include:
      - goos: linux
        goarch: amd64
        platform: linux-amd64
      # Add more platforms here
```

### Change Test Requirements

Modify test commands in `ci.yml` or `test.yml`:

```yaml
- name: Run tests
  run: go test -v -race -timeout 10m ./...
```

### Update Go Version

Change the Go version in all workflow files:

```yaml
- name: Set up Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.25.6'  # Update this
```

## Monitoring

### View Workflow Runs

1. Go to your repository on GitHub
2. Click the "Actions" tab
3. Select a workflow from the left sidebar
4. View individual runs and their logs

### Workflow Notifications

Configure notifications in your GitHub settings:
- Settings → Notifications → Actions
- Choose email or web notifications for workflow failures

## Troubleshooting

### Workflow Not Triggering

**Release workflow**:
- Ensure tag matches pattern `v*.*.*`
- Check that tag was pushed: `git push origin v1.0.0`

**CI workflow**:
- Check branch name matches trigger patterns
- Verify workflow file syntax is correct

### Build Failures

1. Check the workflow logs in GitHub Actions
2. Look for specific error messages
3. Test locally: `make build` or `go build ./...`
4. Ensure all dependencies are in go.mod

### Permission Errors

If you see permission errors:
1. Check repository settings → Actions → General
2. Ensure "Read and write permissions" is enabled
3. Verify workflow has `permissions: contents: write`

## Best Practices

1. **Test Locally First**: Always run `make test` before pushing
2. **Use Semantic Versioning**: Follow semver for version tags
3. **Update CHANGELOG**: Keep CHANGELOG.md up to date
4. **Review Workflow Logs**: Check logs even for successful runs
5. **Monitor Resource Usage**: GitHub Actions has usage limits

## Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Workflow Syntax](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions)
- [Release Documentation](../RELEASE.md)
- [Quick Start Guide](RELEASE_QUICK_START.md)

## Support

For issues with workflows:
1. Check workflow logs in GitHub Actions
2. Review this documentation
3. Open an issue with the `ci/cd` label

---

**Last Updated**: 2026-04-27