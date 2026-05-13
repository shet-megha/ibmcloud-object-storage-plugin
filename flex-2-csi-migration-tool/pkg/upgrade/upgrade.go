package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"kubectl-flex-to-csi/version"
)

// Run executes the self-upgrade process
func Run() error {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Self-Upgrade Tool                                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: Check current version
	currentVersion := version.GetVersion()
	fmt.Printf("📌 Current version: %s\n", currentVersion)
	fmt.Println()

	// Step 2: Fetch latest release from GitHub
	fmt.Println("🔍 Checking for updates...")
	release, err := GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := release.TagName
	fmt.Printf("📦 Latest version: %s\n", latestVersion)
	fmt.Println()

	// Step 3: Compare versions
	if currentVersion == latestVersion {
		fmt.Println("✅ You're already on the latest version!")
		return nil
	}

	fmt.Printf("🚀 New version available: %s → %s\n", currentVersion, latestVersion)
	fmt.Println()

	// Step 4: Determine platform
	platform := getPlatform()
	fmt.Printf("🖥️  Platform: %s\n", platform)
	fmt.Println()

	// Step 5: Find appropriate assets
	archiveName := getArchiveName(platform)
	checksumName := archiveName + ".sha256"

	asset := release.GetAssetByName(archiveName)
	if asset == nil {
		return fmt.Errorf("no release found for platform: %s (looking for %s)", platform, archiveName)
	}

	checksumAsset := release.GetAssetByName(checksumName)
	if checksumAsset == nil {
		return fmt.Errorf("no checksum found for platform: %s (looking for %s)", platform, checksumName)
	}

	fmt.Printf("📦 Found asset: %s (%d bytes)\n", asset.Name, asset.Size)
	fmt.Println()

	// Step 6: Create temporary directory for download
	tmpDir := filepath.Join(os.TempDir(), "kubectl-flex-to-csi-upgrade")
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	checksumPath := filepath.Join(tmpDir, checksumAsset.Name)

	// Step 7: Download archive
	fmt.Println("📥 Downloading update...")
	if err := DownloadWithProgress(asset.BrowserDownloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download archive: %w", err)
	}
	fmt.Println()

	// Step 8: Download checksum
	fmt.Println("📥 Downloading checksum...")
	if err := DownloadWithProgress(checksumAsset.BrowserDownloadURL, checksumPath); err != nil {
		return fmt.Errorf("failed to download checksum: %w", err)
	}
	fmt.Println()

	// Step 9: Verify checksum
	fmt.Println("🔐 Verifying checksum...")
	if err := VerifyChecksum(archivePath, checksumPath); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	fmt.Println()

	// Step 10: Extract archive
	fmt.Println("📦 Extracting archive...")
	extractDir := filepath.Join(tmpDir, "extracted")
	binaryPath, err := ExtractArchive(archivePath, extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}
	fmt.Printf("✅ Extracted binary: %s\n", binaryPath)
	fmt.Println()

	// Step 11: Replace binary
	fmt.Println("🔄 Replacing binary...")
	if err := ReplaceBinary(binaryPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	fmt.Println()

	// Success!
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║  ✅ Upgrade completed successfully!                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n🎉 Upgraded from %s to %s\n", currentVersion, latestVersion)
	fmt.Println("\n💡 Run 'kubectl flex-to-csi version' to verify the new version")
	fmt.Println("💡 If you encounter issues, restore from backup: kubectl-flex-to-csi.backup")

	return nil
}

// getPlatform returns the platform identifier for the current OS
func getPlatform() string {
	switch runtime.GOOS {
	case "linux":
		return "linux"
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

// getArchiveName returns the expected archive name for the platform
func getArchiveName(platform string) string {
	switch platform {
	case "linux":
		return "kubectl-flex-to-csi-linux.tar.gz"
	case "macos":
		return "kubectl-flex-to-csi-macos.zip"
	case "windows":
		return "kubectl-flex-to-csi-windows.zip"
	default:
		return fmt.Sprintf("kubectl-flex-to-csi-%s.tar.gz", platform)
	}
}

// CompareVersions compares two version strings (simple string comparison)
// Returns true if v1 < v2
func CompareVersions(v1, v2 string) bool {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Simple string comparison (works for semantic versioning)
	return v1 < v2
}

// Made with Bob
