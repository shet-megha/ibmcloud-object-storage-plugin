package interactive

import "fmt"

func (a *App) preflightChecks() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PRE-FLIGHT CHECKS                             ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	fmt.Println("\n1️⃣  Checking CSI Driver Installation...")
	if err := a.engine.VerifyCSIAddon(); err != nil {
		fmt.Printf("   ❌ %v\n", err)
		fmt.Println("\n⚠️  Please install the IBM Cloud Object Storage CSI driver before proceeding.")
		a.pause()
		return
	}
	fmt.Println("   ✅ CSI Driver is installed and ready")

	fmt.Println("\n2️⃣  Checking Node Operating Systems...")
	if err := a.engine.CheckSupportedOS(); err != nil {
		fmt.Printf("   ❌ %v\n", err)
		fmt.Println("\n⚠️  Some nodes may not support CSI driver.")
		a.pause()
		return
	}
	fmt.Println("   ✅ All nodes are running supported OS (Ubuntu/RHEL/CoreOS)")

	fmt.Println("\n✅ All pre-flight checks passed!")
	fmt.Println("   You can proceed with migration.")
	a.pause()
}

func (a *App) preflightChecksNoPause() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PRE-FLIGHT CHECKS                             ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	fmt.Println("\n1️⃣  Checking CSI Driver Installation...")
	if err := a.engine.VerifyCSIAddon(); err != nil {
		fmt.Printf("   ❌ %v\n", err)
		fmt.Println("\n⚠️  Please install the IBM Cloud Object Storage CSI driver before proceeding.")
		return
	}
	fmt.Println("   ✅ CSI Driver is installed and ready")

	fmt.Println("\n2️⃣  Checking Node Operating Systems...")
	if err := a.engine.CheckSupportedOS(); err != nil {
		fmt.Printf("   ❌ %v\n", err)
		fmt.Println("\n⚠️  Some nodes may not support CSI driver.")
		return
	}
	fmt.Println("   ✅ All nodes are running supported OS (Ubuntu/RHEL/CoreOS)")

	fmt.Println("\n✅ All pre-flight checks passed!")
	fmt.Println("   You can proceed with migration.")
}

// Made with Bob
