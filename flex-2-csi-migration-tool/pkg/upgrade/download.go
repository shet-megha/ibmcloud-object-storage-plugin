package upgrade

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadFile downloads a file from URL to destination path
func DownloadFile(url, dest string) error {
	// Create destination directory if needed
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Download
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Write to file
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Downloaded %d bytes\n", written)
	return nil
}

// DownloadWithProgress downloads a file with progress indication
func DownloadWithProgress(url, dest string) error {
	fmt.Printf("📥 Downloading from %s...\n", url)

	if err := DownloadFile(url, dest); err != nil {
		return err
	}

	// Get file size
	info, err := os.Stat(dest)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Downloaded successfully (%d bytes)\n", info.Size())
	return nil
}

// Made with Bob
