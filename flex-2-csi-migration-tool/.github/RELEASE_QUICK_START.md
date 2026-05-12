# Quick Start: Creating a Release

This is a quick reference guide for creating releases. For detailed information, see [RELEASE.md](../../RELEASE.md).

## TL;DR - Create a Release

```bash
# 1. Ensure you're on main branch with latest changes
git checkout main
git pull origin main

# 2. Update CHANGELOG.md with release notes
# Edit CHANGELOG.md and move items from [Unreleased] to new version section

# 3. Commit changelog
git add CHANGELOG.md
git commit -m "docs: prepare for v1.0.0 release"
git push origin main

# 4. Create and push tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 5. Watch the magic happen!
# Go to: https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions
```

## What Happens Automatically

When you push a tag (e.g., `v1.0.0`), GitHub Actions will:

1. ✅ Run all tests
2. 🔨 Build binaries for:
   - Linux (AMD64, ARM64)
   - macOS (Intel, Apple Silicon)
   - Windows (AMD64)
3. 📦 Create distribution packages (.tar.gz, .zip)
4. 🔐 Generate SHA256 checksums
5. 🚀 Create GitHub Release with all artifacts
6. 📝 Add release notes automatically

## Release Artifacts

Each release includes:

```
kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz
kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz.sha256
kubectl-flex-to-csi-v1.0.0-linux-arm64.tar.gz
kubectl-flex-to-csi-v1.0.0-linux-arm64.tar.gz.sha256
kubectl-flex-to-csi-v1.0.0-darwin-amd64.tar.gz
kubectl-flex-to-csi-v1.0.0-darwin-amd64.tar.gz.sha256
kubectl-flex-to-csi-v1.0.0-darwin-arm64.tar.gz
kubectl-flex-to-csi-v1.0.0-darwin-arm64.tar.gz.sha256
kubectl-flex-to-csi-v1.0.0-windows-amd64.zip
kubectl-flex-to-csi-v1.0.0-windows-amd64.zip.sha256
checksums.txt (combined checksums)
```

## Versioning Rules

Follow [Semantic Versioning](https://semver.org/):

- **v1.0.0 → v2.0.0**: Breaking changes
- **v1.0.0 → v1.1.0**: New features (backward compatible)
- **v1.0.0 → v1.0.1**: Bug fixes only

## Pre-release Versions

For testing:

```bash
# Alpha release
git tag -a v1.0.0-alpha.1 -m "Alpha release for testing"
git push origin v1.0.0-alpha.1

# Beta release
git tag -a v1.0.0-beta.1 -m "Beta release for testing"
git push origin v1.0.0-beta.1

# Release candidate
git tag -a v1.0.0-rc.1 -m "Release candidate"
git push origin v1.0.0-rc.1
```

## Hotfix Release

For urgent bug fixes:

```bash
# 1. Create hotfix branch from release tag
git checkout -b hotfix/v1.0.1 v1.0.0

# 2. Fix the bug
# ... make changes ...
git add .
git commit -m "fix: critical bug in migration engine"

# 3. Merge to main
git checkout main
git merge hotfix/v1.0.1
git push origin main

# 4. Create patch release tag
git tag -a v1.0.1 -m "Hotfix release v1.0.1"
git push origin v1.0.1

# 5. Delete hotfix branch
git branch -d hotfix/v1.0.1
```

## Troubleshooting

### Release Failed?

1. Check GitHub Actions: https://github.com/YOUR_ORG/kubectl-flex-to-csi/actions
2. Look for error messages in the workflow logs
3. Common issues:
   - Tests failing → Fix tests, delete tag, recreate
   - Build errors → Check Go version, dependencies
   - Permission errors → Check repository settings

### Delete a Tag

If you need to recreate a release:

```bash
# Delete local tag
git tag -d v1.0.0

# Delete remote tag
git push origin :refs/tags/v1.0.0

# Delete GitHub release (via web UI)
# Then create new tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

## Testing Before Release

Always test before creating a release:

```bash
# Run tests
make test

# Build locally
make build

# Test the binary
./kubectl-flex-to-csi --help

# Run all checks
make check
```

## Release Checklist

Before pushing a tag:

- [ ] All tests pass locally (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] CHANGELOG.md is updated
- [ ] Version numbers are correct
- [ ] Documentation is up to date
- [ ] No uncommitted changes

## Download URLs

After release, users can download from:

```
# Specific version
https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/download/v1.0.0/kubectl-flex-to-csi-v1.0.0-linux-amd64.tar.gz

# Latest release
https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases/latest

# All releases
https://github.com/YOUR_ORG/kubectl-flex-to-csi/releases
```

## Need Help?

- 📖 Full documentation: [RELEASE.md](../../RELEASE.md)
- 🐛 Report issues: https://github.com/YOUR_ORG/kubectl-flex-to-csi/issues
- 💬 Discussions: https://github.com/YOUR_ORG/kubectl-flex-to-csi/discussions

---

**Remember**: Once a tag is pushed, the release process is automatic! 🚀