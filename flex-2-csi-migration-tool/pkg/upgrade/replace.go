package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReplaceBinary replaces the current binary with a new one
func ReplaceBinary(newBinaryPath string) error {
	// Get current executable path
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	fmt.Printf("📍 Current binary: %s\n", currentPath)

	// Create backup of current binary
	backupPath := currentPath + ".backup"
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("📦 Backup created: %s\n", backupPath)

	// Copy new binary to current location
	if err := copyFile(newBinaryPath, currentPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("failed to copy new binary: %w", err)
	}

	// Set executable permissions
	if err := os.Chmod(currentPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	fmt.Println("✅ Binary replaced successfully")
	fmt.Printf("💡 Backup available at: %s\n", backupPath)
	fmt.Println("💡 You can delete the backup once you verify the upgrade works")

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, 0755)
}

// Made with Bob
