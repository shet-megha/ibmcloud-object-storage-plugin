package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// GitHubAPIURL is the base URL for GitHub API
	GitHubAPIURL = "https://api.github.com"
	// RepoOwner is the GitHub repository owner
	RepoOwner = "shet-megha"
	// RepoName is the GitHub repository name
	RepoName = "ibmcloud-object-storage-plugin"
)

// Release represents a GitHub release
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Body       string  `json:"body"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a release asset (downloadable file)
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// GetLatestRelease fetches the latest release from GitHub
func GetLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest",
		GitHubAPIURL, RepoOwner, RepoName)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var release Release
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// FindAsset finds an asset by name pattern
func (r *Release) FindAsset(pattern string) *Asset {
	for i := range r.Assets {
		if strings.Contains(r.Assets[i].Name, pattern) {
			return &r.Assets[i]
		}
	}
	return nil
}

// GetAssetByName finds an asset by exact name
func (r *Release) GetAssetByName(name string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

// Made with Bob
