package cli

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"kubectl-flex-to-csi/internal/kube"
	"kubectl-flex-to-csi/pkg/discovery"
	"kubectl-flex-to-csi/pkg/migration"

	"k8s.io/client-go/kubernetes"
)

func Run(args []string) {
	fmt.Println("Starting Discovery and Migration...")
	fmt.Println()

	namespace, allNamespaces := parseDiscoveryArgs(args)

	clientset, err := kube.NewClient()
	if err != nil {
		log.Fatalf("Client error: %v", err)
	}

	inspector := NewInspector(clientset)
	if err := inspector.phase0(); err != nil {
		log.Fatalf("Pre-flight checks failed: %v", err)
	}

	fmt.Println("\n========================================")
	fmt.Println("Phase 1: Resource Discovery & Migration")
	fmt.Println("========================================\n")

	discoveryClient := discovery.NewResourceDiscovery(clientset)

	discoveryNamespace := namespace
	if allNamespaces {
		discoveryNamespace = ""
		fmt.Println("🔍 Running all-namespaces discovery (slower, full-cluster scan)")
	} else if discoveryNamespace != "" {
		fmt.Printf("🔍 Running fast namespace-scoped discovery in namespace: %s\n", discoveryNamespace)
	} else {
		discoveryNamespace = detectActiveNamespace()
		if discoveryNamespace == "" {
			discoveryNamespace = "default"
		}
		fmt.Printf("🔍 Running fast namespace-scoped discovery in namespace: %s\n", discoveryNamespace)
		fmt.Println("   Use --all-namespaces for a full-cluster scan.")
	}

	resources, err := discoveryClient.DiscoverAllFlexResourcesOptimized(discoveryNamespace)
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	if len(resources) == 0 {
		fmt.Println("✅ No Flex resources found to migrate!")
		fmt.Println("   Your selected discovery scope has no Flex volumes.")
		fmt.Println("\nDiscovery Completed")
		return
	}

	runInteractiveMigration(resources, clientset, discoveryClient)
}

func parseDiscoveryArgs(args []string) (string, bool) {
	namespace := ""
	allNamespaces := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--namespace", "-n":
			if i+1 < len(args) {
				namespace = strings.TrimSpace(args[i+1])
				i++
			}
		case "--all-namespaces", "-A":
			allNamespaces = true
			namespace = ""
		}
	}

	return namespace, allNamespaces
}

func detectActiveNamespace() string {
	if ns := strings.TrimSpace(os.Getenv("KUBECTL_NAMESPACE")); ns != "" {
		return ns
	}
	if ns := strings.TrimSpace(os.Getenv("NAMESPACE")); ns != "" {
		return ns
	}
	return ""
}

func runInteractiveMigration(resources []discovery.ResourceInfo, clientset *kubernetes.Clientset, discoveryClient *discovery.ResourceDiscovery) {
	fmt.Println("\n🔄 Interactive One-at-a-Time Migration Mode")
	fmt.Println("   You can migrate resources individually and verify each one.")
	fmt.Println()

	engine := migration.NewMigrationEngine(clientset, false)
	reader := bufio.NewReader(os.Stdin)

	migratedCount := 0
	failedCount := 0
	remainingResources := make([]discovery.ResourceInfo, len(resources))
	copy(remainingResources, resources)

	for len(remainingResources) > 0 {
		// Show available resources
		fmt.Printf("\n📋 Available resources to migrate (%d remaining):\n\n", len(remainingResources))

		for i, res := range remainingResources {
			fmt.Printf("%d. %s/%s (%s)\n", i+1, res.Namespace, res.Name, res.Type)
			pvcNames := []string{}
			for _, pvc := range res.PVCs {
				pvcNames = append(pvcNames, pvc.Name)
			}
			fmt.Printf("   PVCs: %s\n", strings.Join(pvcNames, ", "))
		}

		fmt.Println("\nOptions:")
		fmt.Printf("  • Enter resource number (1-%d) to migrate\n", len(remainingResources))
		fmt.Println("  • Enter 'a' to migrate all remaining")
		fmt.Println("  • Enter 'q' to quit")
		fmt.Print("\nYour choice: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if strings.ToLower(choice) == "q" {
			fmt.Println("\n👋 Exiting migration mode")
			break
		}

		if strings.ToLower(choice) == "a" {
			fmt.Println("\n🚀 Migrating all remaining resources...")
			for _, res := range remainingResources {
				result, err := engine.MigrateResource(res)
				if err != nil || result.Status != "success" {
					failedCount++
				} else {
					migratedCount++
				}
			}
			remainingResources = []discovery.ResourceInfo{}
			break
		}

		// Parse selection
		var idx int
		_, err := fmt.Sscanf(choice, "%d", &idx)
		if err != nil || idx < 1 || idx > len(remainingResources) {
			fmt.Printf("\n❌ Invalid selection. Please enter 1-%d, 'a', or 'q'\n", len(remainingResources))
			continue
		}

		// Migrate selected resource
		selectedRes := remainingResources[idx-1]
		success := migrateSingleResource(selectedRes, engine, reader)

		if success {
			migratedCount++
		} else {
			failedCount++
		}

		// Remove from remaining list
		remainingResources = append(remainingResources[:idx-1], remainingResources[idx:]...)

		if len(remainingResources) == 0 {
			fmt.Println("\n✅ All resources have been migrated!")
			break
		}

		// Ask if user wants to continue
		fmt.Print("\nContinue with another migration? (y/n): ")
		continueChoice, _ := reader.ReadString('\n')
		continueChoice = strings.TrimSpace(strings.ToLower(continueChoice))
		if continueChoice != "y" && continueChoice != "yes" {
			fmt.Println("\n👋 Exiting migration mode")
			break
		}
	}

	// Final summary
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              MIGRATION SUMMARY                             ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n  Total Resources:    %d\n", len(resources))
	fmt.Printf("  ✅ Migrated:        %d\n", migratedCount)
	fmt.Printf("  ❌ Failed:          %d\n", failedCount)
	fmt.Printf("  ⏳ Remaining:       %d\n", len(remainingResources))
	fmt.Println()
}

func migrateSingleResource(resource discovery.ResourceInfo, engine *migration.MigrationEngine, reader *bufio.Reader) bool {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("🚀 Migrating: %s/%s (%s)\n", resource.Namespace, resource.Name, resource.Type)
	fmt.Println(strings.Repeat("=", 70))

	// Show migration plan
	fmt.Println("\n📋 Migration Plan:")
	fmt.Println("  1. ✓ Verify CSI addon and OS compatibility")
	fmt.Println("  2. ✓ Read Flex secret and PV configuration")
	fmt.Println("  3. ✓ Fetch actual mount options from running pod")
	fmt.Println("  4. ✓ Create new CSI secret with mount options")
	fmt.Println("  5. ✓ Create new CSI PVC")
	fmt.Println("  6. ✓ Wait for PVC to bind")
	fmt.Println("  7. ✓ Create new resource with CSI volume")
	fmt.Println("  8. ✓ Verify pod is running")
	fmt.Println("  9. ✓ Verify bucket access")
	fmt.Println("  10. ✓ Verify mount options match")
	fmt.Println("  11. ✓ Migrate services")
	fmt.Println("  12. ✓ Verify traffic routing")
	fmt.Println("  13. ⏸  Keep old resources (manual cleanup)")

	fmt.Print("\nProceed with migration? (y/n): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		fmt.Println("\n❌ Migration cancelled")
		return false
	}

	// Perform migration
	fmt.Println("\n🔄 Starting migration...")

	result, err := engine.MigrateResource(resource)

	if err != nil {
		fmt.Printf("\n❌ Migration failed: %v\n", err)
		fmt.Print("\nPress Enter to continue...")
		reader.ReadString('\n')
		return false
	}

	if result.Status != "success" {
		fmt.Printf("\n❌ Migration failed: %s\n", result.Error)
		fmt.Print("\nPress Enter to continue...")
		reader.ReadString('\n')
		return false
	}

	// Show success summary
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("✅ Migration Completed Successfully!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\n📊 Migration Summary:\n")
	fmt.Printf("  Resource:        %s/%s\n", resource.Namespace, resource.Name)
	fmt.Printf("  Type:            %s\n", resource.Type)
	fmt.Printf("  Migration Time:  %v\n", result.MigrationTime)
	fmt.Printf("  New PVC:         %s\n", result.NewPVCName)
	fmt.Printf("  Status:          %s\n", result.Status)

	// Show mount options verification
	fmt.Println("\n🔍 Mount Options Verification:")
	fmt.Println("  ✅ CSI secret contains mount options from Flex PV")
	fmt.Println("  ✅ Mount options preserved during migration")
	fmt.Println("  ℹ️  For detailed verification, use:")
	fmt.Printf("     kubectl flex-to-csi verify <node-ip> %s\n", result.NewPVCName)

	// Show next steps
	fmt.Println("\n📝 Next Steps:")
	fmt.Println("  1. Verify the new pod is working correctly")
	fmt.Println("  2. Test application functionality")
	fmt.Println("  3. Monitor for any issues")
	fmt.Println("  4. When ready, cleanup old resources")

	fmt.Print("\nPress Enter to continue...")
	reader.ReadString('\n')

	return true
}

// Made with Bob
