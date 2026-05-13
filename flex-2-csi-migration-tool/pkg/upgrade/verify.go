package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifyChecksum verifies a file against its SHA256 checksum
func VerifyChecksum(filePath, checksumPath string) error {
	// Calculate actual hash of the file
	actualHash, err := calculateSHA256(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Read expected hash from checksum file
	expectedHash, err := readChecksumFile(checksumPath)
	if err != nil {
		return fmt.Errorf("failed to read checksum: %w", err)
	}

	// Compare hashes
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch!\nExpected: %s\nGot:      %s",
			expectedHash, actualHash)
	}

	fmt.Println("✅ Checksum verified successfully")
	return nil
}

// calculateSHA256 calculates the SHA256 hash of a file
func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// readChecksumFile reads a checksum from a .sha256 file
// Format can be: "hash  filename" or just "hash"
func readChecksumFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Parse the checksum file
	// Format: "hash  filename" or just "hash"
	content := strings.TrimSpace(string(data))
	parts := strings.Fields(content)

	if len(parts) == 0 {
		return "", fmt.Errorf("empty checksum file")
	}

	// Return the first field (the hash)
	return parts[0], nil
}

// Made with Bob
