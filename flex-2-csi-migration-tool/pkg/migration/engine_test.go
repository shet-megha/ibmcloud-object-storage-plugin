package migration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestVerifyCSIAddon(t *testing.T) {
	tests := []struct {
		name      string
		daemonset *appsv1.DaemonSet
		wantErr   bool
	}{
		{
			name: "CSI driver found and ready",
			daemonset: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ibm-object-csi-driver",
					Namespace: "kube-system",
				},
				Status: appsv1.DaemonSetStatus{
					NumberReady:            3,
					DesiredNumberScheduled: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "CSI driver not ready",
			daemonset: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ibm-object-csi-driver",
					Namespace: "kube-system",
				},
				Status: appsv1.DaemonSetStatus{
					NumberReady:            0,
					DesiredNumberScheduled: 3,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			if tt.daemonset != nil {
				_, err := clientset.AppsV1().DaemonSets(tt.daemonset.Namespace).Create(
					context.Background(),
					tt.daemonset,
					metav1.CreateOptions{},
				)
				require.NoError(t, err)
			}

			engine := NewMigrationEngine(clientset, false)
			err := engine.verifyCSIAddon()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckSupportedOS(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []*corev1.Node
		wantErr bool
	}{
		{
			name: "Ubuntu nodes",
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							OSImage: "Ubuntu 20.04.3 LTS",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "RHEL nodes",
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							OSImage: "Red Hat Enterprise Linux CoreOS",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Unsupported OS",
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							OSImage: "Windows Server 2019",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			for _, node := range tt.nodes {
				_, err := clientset.CoreV1().Nodes().Create(
					context.Background(),
					node,
					metav1.CreateOptions{},
				)
				require.NoError(t, err)
			}

			engine := NewMigrationEngine(clientset, false)
			err := engine.checkSupportedOS()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetCSIStorageClass(t *testing.T) {
	tests := []struct {
		name     string
		flexSC   *string
		expected string
	}{
		{
			name:     "flex-regional",
			flexSC:   stringPtr("ibmc-s3fs-flex-regional"),
			expected: "ibm-object-storage-standard-s3fs",
		},
		{
			name:     "flex-cross-region",
			flexSC:   stringPtr("ibmc-s3fs-flex-cross-region"),
			expected: "ibm-object-storage-standard-s3fs",
		},
		{
			name:     "smart-regional",
			flexSC:   stringPtr("ibmc-s3fs-smart-regional"),
			expected: "ibm-object-storage-smart-s3fs",
		},
		{
			name:     "nil storage class",
			flexSC:   nil,
			expected: "ibm-object-storage-standard-s3fs",
		},
		{
			name:     "unknown storage class",
			flexSC:   stringPtr("unknown-storage-class"),
			expected: "ibm-object-storage-standard-s3fs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCSIStorageClass(tt.flexSC)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected, *result)
		})
	}
}

func TestExtractMountOptions(t *testing.T) {
	tests := []struct {
		name     string
		pv       *corev1.PersistentVolume
		contains []string
	}{
		{
			name: "FlexVolume with options",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					FlexVolume: &corev1.FlexPersistentVolumeSource{
						Driver: "ibm/ibmc-s3fs",
						Options: map[string]string{
							"chunk-size-mb":         "16",
							"parallel-count":        "2",
							"multireq-max":          "20",
							"stat-cache-size":       "100000",
							"s3fs-fuse-retry-count": "5",
							"kernel-cache":          "true",
						},
					},
				},
			},
			contains: []string{
				"multipart_size=16",
				"parallel_count=2",
				"multireq_max=20",
				"max_stat_cache_size=100000",
				"retries=5",
				"kernel_cache",
			},
		},
		{
			name: "PV with no options",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{},
			},
			contains: []string{
				"multipart_size=52", // default
				"parallel_count=10", // default
				"allow_other",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &MigrationEngine{}
			result := engine.extractMountOptions(tt.pv)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		prefix   string
		expected bool
	}{
		{
			name:     "contains prefix",
			slice:    []string{"multipart_size=16", "parallel_count=2"},
			prefix:   "multipart_size",
			expected: true,
		},
		{
			name:     "does not contain prefix",
			slice:    []string{"multipart_size=16", "parallel_count=2"},
			prefix:   "kernel_cache",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			prefix:   "test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.prefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateCSISecret(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	engine := NewMigrationEngine(clientset, false)

	flexSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flex-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"access-key":      []byte("test-access-key"),
			"secret-key":      []byte("test-secret-key"),
			"res-conf-apikey": []byte("test-apikey"),
		},
	}

	pv := &corev1.PersistentVolume{
		Spec: corev1.PersistentVolumeSpec{
			FlexVolume: &corev1.FlexPersistentVolumeSource{
				Options: map[string]string{
					"bucket":                "test-bucket",
					"object-store-endpoint": "https://s3.test.com",
				},
			},
		},
	}

	err := engine.createCSISecret(flexSecret, pv, "flex-secret-csi-migrated", "default")
	assert.NoError(t, err)

	// Verify secret was created
	secret, err := clientset.CoreV1().Secrets("default").Get(
		context.Background(),
		"flex-secret-csi-migrated",
		metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "cos-s3-csi-driver", string(secret.Type))
}

func TestCreateCSIPVC(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	engine := NewMigrationEngine(clientset, false)

	// Create original PVC
	storageClass := "ibmc-s3fs-flex-regional"
	originalPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flex-pvc",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: &storageClass,
		},
	}

	_, err := clientset.CoreV1().PersistentVolumeClaims("default").Create(
		context.Background(),
		originalPVC,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	pvcInfo := PVCInfo{
		Name:         "flex-pvc",
		Namespace:    "default",
		StorageClass: &storageClass,
		Annotations: map[string]string{
			"ibm.io/auto-create-bucket": "false",
			"ibm.io/bucket":             "test-bucket",
		},
	}

	pv := &corev1.PersistentVolume{}

	err = engine.createCSIPVC(pvcInfo, pv, "flex-pvc-csi-migrated", "flex-secret-csi-migrated")
	assert.NoError(t, err)

	// Verify PVC was created
	pvc, err := clientset.CoreV1().PersistentVolumeClaims("default").Get(
		context.Background(),
		"flex-pvc-csi-migrated",
		metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.NotNil(t, pvc)
	assert.Equal(t, "ibm-object-storage-standard-s3fs", *pvc.Spec.StorageClassName)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

// Made with Bob
