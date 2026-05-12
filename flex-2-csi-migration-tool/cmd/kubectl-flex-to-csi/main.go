package main

import (
	"fmt"
	"os"

	"kubectl-flex-to-csi/pkg/cli"
	"kubectl-flex-to-csi/pkg/interactive"
	"kubectl-flex-to-csi/pkg/verify"
)

func main() {

	// Production-safe default: require explicit command selection
	if len(os.Args) < 2 {
		showUsage()
		os.Exit(1)
	}

	switch os.Args[1] {

	case "interactive", "i":
		app, err := interactive.New()
		if err != nil {
			fmt.Printf("❌ Failed to initialize: %v\n", err)
			os.Exit(1)
		}
		app.Run()

	case "discover":
		cli.Run(os.Args[2:])

	case "migrate-one":
		// Interactive one-at-a-time migration
		cli.Run(os.Args[2:])

	case "migrate":
		fmt.Println("⚠️  The standalone 'migrate' command is not production-ready in this build.")
		fmt.Println("Use 'interactive' for guided migration or 'discover' for discovery-driven workflow.")
		os.Exit(1)

	case "cleanup":
		// Cleanup is now handled by the migrate command with --cleanup flag
		// Or use the migration engine directly
		fmt.Println("⚠️  Cleanup command has been integrated into the migration workflow.")
		fmt.Println()
		fmt.Println("To cleanup old resources after migration:")
		fmt.Println("  1. Use interactive mode: kubectl flex-to-csi interactive")
		fmt.Println("  2. Or use migrate command: kubectl flex-to-csi migrate --workload <name> --namespace <ns>")
		fmt.Println()
		fmt.Println("The migration engine automatically handles cleanup as step 16.")

	case "verify":
		verify.Run(os.Args[2:])

	case "version", "-v", "--version":
		showVersion()

	case "help", "-h", "--help":
		showUsage()

	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		showUsage()
	}
}

func showUsage() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║  IBM Cloud Object Storage FlexVolume to CSI Migration     ║")
	fmt.Println("║                    Tool v2.0                               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Usage: kubectl flex-to-csi <command>")
	fmt.Println()
	fmt.Println("Available Commands:")
	fmt.Println("  interactive, i    🎯 Interactive mode")
	fmt.Println("  discover          🔍 Discover Flex resources")
	fmt.Println("  migrate-one       🎯 Migrate one resource at a time (interactive)")
	fmt.Println("  verify            ✅ Verify migrated resources")
	fmt.Println("  cleanup           🗑️  Cleanup old resources")
	fmt.Println("  version           📌 Show version information")
	fmt.Println("  help              ❓ Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Interactive mode (recommended for most users)")
	fmt.Println("  kubectl flex-to-csi interactive")
	fmt.Println()
	fmt.Println("  # Discover Flex resources")
	fmt.Println("  kubectl flex-to-csi discover --namespace default")
	fmt.Println("  kubectl flex-to-csi discover --all-namespaces")
	fmt.Println()
	fmt.Println("  # Verify mount options on a node")
	fmt.Println("  kubectl flex-to-csi verify <node-ip> <pv-name>")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  ✅ CSI addon verification")
	fmt.Println("  ✅ OS compatibility check (Ubuntu/RHEL/CoreOS)")
	fmt.Println("  ✅ Automatic resource discovery")
	fmt.Println("  ✅ Secret and PVC migration")
	fmt.Println("  ✅ Workload migration (Deployments, StatefulSets, DaemonSets, Pods)")
	fmt.Println("  ✅ Service attachment")
	fmt.Println("  ✅ Mount options verification")
	fmt.Println("  ✅ Retention policy management")
	fmt.Println("  ✅ Traffic verification")
	fmt.Println("  ✅ Status tracking (migrated/pending/failed)")
	fmt.Println()
}

func showVersion() {
	fmt.Println("kubectl-flex-to-csi version 2.0")
	fmt.Println("IBM Cloud Object Storage FlexVolume to CSI Migration Tool")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  • User-friendly interactive CLI")
	fmt.Println("  • Comprehensive pre-flight checks")
	fmt.Println("  • Automated resource discovery")
	fmt.Println("  • Safe migration with verification")
	fmt.Println("  • Status tracking and reporting")
	fmt.Println()
}
