package kube

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// ✅ IMPORTANT FIX
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func NewClient() (*kubernetes.Clientset, error) {
	// Check for KUBECONFIG environment variable first
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		// Fall back to default location
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	// ⚡ ULTRA-FAST OPTIMIZATION: Maximum rate limits for sub-second discovery
	// These are extremely aggressive values for fastest possible discovery
	config.QPS = 1000   // Queries per second (was 5, then 100)
	config.Burst = 2000 // Burst capacity (was 10, then 200)

	// Reduce timeout for faster failures
	// config.Timeout = 10 * time.Second

	return kubernetes.NewForConfig(config)
}
