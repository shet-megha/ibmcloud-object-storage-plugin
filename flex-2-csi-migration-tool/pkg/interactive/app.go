package interactive

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"kubectl-flex-to-csi/pkg/discovery"
	"kubectl-flex-to-csi/pkg/migration"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// App provides the interactive CLI for migration.
type App struct {
	clientset *kubernetes.Clientset
	ctx       context.Context
	reader    *bufio.Reader
	discovery *discovery.ResourceDiscovery
	engine    *migration.MigrationEngine
	tracker   *StatusTracker
}

// StatusTracker tracks migration status.
type StatusTracker struct {
	Resources       []ResourceStatus
	TotalResources  int
	MigratedCount   int
	PendingCount    int
	FailedCount     int
	InProgressCount int
}

// ResourceStatus tracks individual resource status.
type ResourceStatus struct {
	Name          string
	Namespace     string
	Type          string
	Status        string
	Error         string
	MigrationTime time.Duration
	OldPVCs       []string
	NewPVCs       []string
	LastUpdated   time.Time
}

// New creates a new interactive app instance.
func New() (*App, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	config.QPS = 1000
	config.Burst = 2000

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	return &App{
		clientset: clientset,
		ctx:       context.Background(),
		reader:    bufio.NewReader(os.Stdin),
		discovery: discovery.NewResourceDiscovery(clientset),
		engine:    migration.NewMigrationEngine(clientset, false),
		tracker:   &StatusTracker{Resources: []ResourceStatus{}},
	}, nil
}

// Run starts the interactive migration CLI.
func (a *App) Run() {
	a.showWelcome()

	fmt.Println("🚀 Running automatic pre-flight checks and resource discovery...\n")
	a.preflightChecksNoPause()
	a.discoverResourcesNoPause()
	a.mainMenu()
}

func (a *App) mainMenu() {
	for {
		fmt.Println("\n┌─────────────────────────────────────────┐")
		fmt.Println("│           MAIN MENU                     │")
		fmt.Println("└─────────────────────────────────────────┘")
		fmt.Println()
		fmt.Println("  1. 🚀 Start Migration")
		fmt.Println("  2. 📊 View Migration Status")
		fmt.Println("  3. ✅ Verify Migrated Resources")
		fmt.Println("  4. 🗑️  Cleanup Old Resources")
		fmt.Println("  5. ❌ Exit")
		fmt.Println()
		fmt.Print("Select an option (1-5): ")

		choice := a.readInput()

		switch choice {
		case "1":
			a.startMigration()
		case "2":
			a.viewStatus()
		case "3":
			a.verifyMigrated()
		case "4":
			a.cleanupOld()
		case "5":
			fmt.Println("\n👋 Thank you for using the migration tool!")
			fmt.Println("✅ All migration data has been preserved.")
			return
		default:
			fmt.Println("\n❌ Invalid option. Please select 1-5.")
		}
	}
}

func (a *App) readInput() string {
	input, _ := a.reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// Made with Bob
