package discovery

import corev1 "k8s.io/api/core/v1"

// ResourceInfo contains information about a resource to migrate.
type ResourceInfo struct {
	Name      string
	Namespace string
	Type      string
	PVCs      []PVCInfo
	Services  []string
	Object    interface{}
}

// PVCInfo contains PVC details.
type PVCInfo struct {
	Name         string
	Namespace    string
	SecretName   string
	PVName       string
	StorageClass *string
	AccessModes  []corev1.PersistentVolumeAccessMode
	Storage      string
	Annotations  map[string]string
	Status       string
}

// Made with Bob
