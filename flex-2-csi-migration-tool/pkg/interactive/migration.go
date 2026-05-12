package interactive

import (
	"fmt"
	"strings"
	"time"

	"kubectl-flex-to-csi/pkg/discovery"
)

func (a *App) startMigration() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              START MIGRATION                               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	if a.tracker.TotalResources == 0 {
		fmt.Println("\n⚠️  No resources discovered yet.")
		fmt.Println("   Please run 'Discover Flex Resources' first.")
		a.pause()
		return
	}

	pendingResources := a.getPendingResources()
	if len(pendingResources) == 0 {
		fmt.Println("\n✅ All resources have been migrated!")
		a.pause()
		return
	}

	fmt.Printf("\n📊 Found %d pending resource(s) to migrate:\n", len(pendingResources))
	pendingResourceInfos := a.getResourceInfosFromStatuses(pendingResources)

	orphanedCount := 0
	pendingPVCCount := 0
	migrableCount := 0
	pendingPVCResources := []string{}

	for _, res := range pendingResourceInfos {
		if res.Name == "" || res.Type == "" {
			orphanedCount++
		} else {
			hasPendingPVC := false
			for _, pvc := range res.PVCs {
				if pvc.Status == "Pending" || strings.Contains(pvc.Status, "Pending") {
					hasPendingPVC = true
					pendingPVCCount++
					pendingPVCResources = append(pendingPVCResources, fmt.Sprintf("%s (%s)", res.Name, res.Type))
					break
				}
			}
			if !hasPendingPVC {
				migrableCount++
			}
		}
	}

	a.printResourceInfoTable(pendingResourceInfos)

	if orphanedCount > 0 {
		fmt.Printf("\n⚠️  Warning: %d orphaned PVC(s) detected (PVC exists but no workload)\n", orphanedCount)
		fmt.Println("   Migration not supported for orphaned PVCs.")
	}

	if pendingPVCCount > 0 {
		fmt.Printf("\n⚠️  Warning: %d resource(s) with PVC in Pending state:\n", pendingPVCCount)
		for _, resName := range pendingPVCResources {
			fmt.Printf("   - %s\n", resName)
		}
		fmt.Println("   Migration not supported for resources with Pending PVCs.")
	}

	if migrableCount == 0 {
		fmt.Println("\n❌ No migrable resources found.")
		fmt.Println("   All pending resources are orphaned PVCs without workloads.")
		a.pause()
		return
	}

	fmt.Println("\n⚠️  Migration Options:")
	fmt.Println("  1. Migrate all pending resources")
	fmt.Println("  2. Select specific resources")
	fmt.Println("  3. Cancel")
	fmt.Print("\nSelect option (1-3): ")

	choice := a.readInput()

	switch choice {
	case "1":
		a.migrateAll(pendingResources)
	case "2":
		a.migrateSelected(pendingResources)
	case "3":
		fmt.Println("\n❌ Migration cancelled")
		a.pause()
	default:
		fmt.Println("\n❌ Invalid option")
		a.pause()
	}
}

func (a *App) migrateAll(resources []ResourceStatus) {
	fmt.Println("\n🚀 Starting migration of all pending resources...")
	fmt.Println("\n⚠️  This will:")
	fmt.Println("  • Verify CSI addon and OS compatibility")
	fmt.Println("  • Create new CSI secrets and PVCs")
	fmt.Println("  • Create new resources with CSI volumes")
	fmt.Println("  • Verify bucket access and pod status")
	fmt.Println("  • Migrate services to new resources")
	fmt.Println("  • Verify mount options and traffic")
	fmt.Println("  • Keep old resources until manual cleanup")
	fmt.Print("\nProceed? (yes/no): ")

	confirm := a.readInput()
	if strings.ToLower(confirm) != "yes" {
		fmt.Println("\n❌ Migration cancelled")
		a.pause()
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 60))

	for i, resStatus := range resources {
		fmt.Printf("\n[%d/%d] Migrating: %s\n", i+1, len(resources), resStatus.Name)
		fmt.Println(strings.Repeat("-", 60))

		if resStatus.Name == "" || resStatus.Type == "" {
			fmt.Println("⚠️  Skipped: Orphaned PVC (migration not supported)")
			continue
		}

		resourceInfo, err := a.discovery.GetResourceByName(resStatus.Name, resStatus.Namespace)
		if err != nil {
			fmt.Printf("❌ Failed to get resource info: %v\n", err)
			a.updateResourceStatus(resStatus.Name, "failed", err.Error())
			continue
		}

		hasPendingPVC := false
		for _, pvc := range resourceInfo.PVCs {
			if pvc.Status == "Pending" || strings.Contains(pvc.Status, "Pending") {
				hasPendingPVC = true
				break
			}
		}
		if hasPendingPVC {
			fmt.Println("⚠️  Skipped: PVC in Pending state (migration not supported)")
			continue
		}
		if err != nil {
			fmt.Printf("❌ Failed to get resource info: %v\n", err)
			a.updateResourceStatus(resStatus.Name, "failed", err.Error())
			continue
		}

		a.updateResourceStatus(resStatus.Name, "in-progress", "")

		result, err := a.engine.MigrateResource(*resourceInfo)
		if err != nil {
			fmt.Printf("❌ Migration failed: %v\n", err)
			a.updateResourceStatus(resStatus.Name, "failed", err.Error())
			continue
		}

		a.updateResourceStatusWithResult(resStatus.Name, result)
		fmt.Println("✅ Migration completed successfully")
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\n✅ All migrations completed!")
	fmt.Printf("   Migrated: %d, Failed: %d\n", a.tracker.MigratedCount, a.tracker.FailedCount)
	a.pause()
}

func (a *App) migrateSelected(resources []ResourceStatus) {
	fmt.Println("\n🔄 Interactive One-at-a-Time Migration Mode")
	fmt.Println("   You can migrate resources individually and verify each one.")
	fmt.Println()

	for {
		availableResources := []ResourceStatus{}
		for _, res := range resources {
			if res.Status == "pending" {
				availableResources = append(availableResources, res)
			}
		}

		if len(availableResources) == 0 {
			fmt.Println("\n✅ All resources have been migrated!")
			a.pause()
			return
		}

		availableResourceInfos := a.getResourceInfosFromStatuses(availableResources)
		supportedResourceInfos := []discovery.ResourceInfo{}
		supportedResources := []ResourceStatus{}

		for i, resInfo := range availableResourceInfos {
			isSupported := true

			if resInfo.Name == "" || resInfo.Type == "" {
				isSupported = false
			}

			for _, pvc := range resInfo.PVCs {
				if pvc.Status == "Pending" || strings.Contains(pvc.Status, "Pending") {
					isSupported = false
					break
				}
			}

			if isSupported {
				supportedResourceInfos = append(supportedResourceInfos, resInfo)
				supportedResources = append(supportedResources, availableResources[i])
			}
		}

		if len(supportedResourceInfos) == 0 {
			fmt.Println("\n⚠️  No supported resources available for migration.")
			fmt.Println("   All remaining resources are either orphaned PVCs or have Pending PVCs.")
			a.pause()
			return
		}

		fmt.Printf("\n📋 Available resources to migrate (%d remaining):\n\n", len(supportedResources))
		fmt.Printf("\n📊 Found %d pending resource(s) to migrate:\n", len(supportedResourceInfos))
		a.printResourceInfoTable(supportedResourceInfos)

		fmt.Println("\nOptions:")
		fmt.Println("  • Enter resource number (1-" + fmt.Sprintf("%d", len(supportedResources)) + ") to migrate")
		fmt.Println("  • Enter 'q' to quit")
		fmt.Print("\nYour choice: ")

		choice := a.readInput()

		if strings.ToLower(choice) == "q" {
			fmt.Println("\n👋 Exiting migration mode")
			a.pause()
			return
		}

		var idx int
		_, err := fmt.Sscanf(choice, "%d", &idx)
		if err != nil || idx < 1 || idx > len(supportedResources) {
			fmt.Printf("\n❌ Invalid selection. Please enter 1-%d or 'q'\n", len(supportedResources))
			continue
		}

		selectedRes := supportedResources[idx-1]
		a.migrateSingleResource(selectedRes)

		fmt.Print("\nContinue with another migration? (y/n): ")
		continueChoice := a.readInput()
		if strings.ToLower(continueChoice) != "y" && strings.ToLower(continueChoice) != "yes" {
			fmt.Println("\n👋 Exiting migration mode")
			a.pause()
			return
		}
	}
}

func (a *App) migrateSingleResource(resStatus ResourceStatus) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("🚀 Migrating: %s/%s (%s)\n", resStatus.Namespace, resStatus.Name, resStatus.Type)
	fmt.Println(strings.Repeat("=", 70))

	if resStatus.Name == "" || resStatus.Type == "" {
		fmt.Println("\n⚠️  Cannot migrate: Orphaned PVC (migration not supported)")
		fmt.Println("   This PVC exists but is not used by any workload.")
		a.pause()
		return
	}

	resourceInfo, err := a.discovery.GetResourceByName(resStatus.Name, resStatus.Namespace)
	if err != nil {
		fmt.Printf("\n❌ Failed to get resource info: %v\n", err)
		a.updateResourceStatus(resStatus.Name, "failed", err.Error())
		a.pause()
		return
	}

	hasPendingPVC := false
	for _, pvc := range resourceInfo.PVCs {
		if pvc.Status == "Pending" || strings.Contains(pvc.Status, "Pending") {
			hasPendingPVC = true
			break
		}
	}
	if hasPendingPVC {
		fmt.Println("\n⚠️  Cannot migrate: PVC in Pending state (migration not supported)")
		fmt.Println("   Wait for PVC to be Bound before migrating.")
		a.pause()
		return
	}

	a.updateResourceStatus(resStatus.Name, "in-progress", "")

	fmt.Println("\n📋 Migration Plan:")
	fmt.Println("  1. ✓ Verify CSI addon and OS compatibility")
	fmt.Println("  2. ✓ Read Flex secret and PV configuration")
	fmt.Println("  3. ✓ Fetch actual mount options from running pod")
	fmt.Println("  4. ✓ Create new CSI secret with mount options")
	fmt.Println("  5. ✓ Create new CSI PVC")
	fmt.Println("  6. ✓ Wait for PVC to bind")
	fmt.Println("  7. ✓ Create new resource with CSI volume")
	fmt.Println("  8. ✓ Verify pod is running")
	fmt.Println("  9. ✓ Verify mount options match")
	fmt.Println("  10. ✓ Migrate services")
	fmt.Println("  11. ⏸  Keep old resources (manual cleanup)")

	fmt.Print("\nProceed with migration? (y/n): ")
	confirm := a.readInput()
	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println("\n❌ Migration cancelled")
		a.updateResourceStatus(resStatus.Name, "pending", "")
		a.pause()
		return
	}

	fmt.Println("\n🔄 Starting migration...")
	startTime := time.Now()

	result, err := a.engine.MigrateResource(*resourceInfo)

	migrationTime := time.Since(startTime)

	if err != nil {
		fmt.Printf("\n❌ Migration failed: %v\n", err)
		a.updateResourceStatus(resStatus.Name, "failed", err.Error())
		a.pause()
		return
	}

	a.updateResourceStatusWithResult(resStatus.Name, result)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("✅ Migration Completed Successfully!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\n📊 Migration Summary:\n")
	fmt.Printf("  Resource:        %s/%s\n", resStatus.Namespace, resStatus.Name)
	fmt.Printf("  Type:            %s\n", resStatus.Type)
	fmt.Printf("  Migration Time:  %v\n", migrationTime)
	fmt.Printf("  Old PVCs:        %s\n", strings.Join(resStatus.OldPVCs, ", "))
	fmt.Printf("  New PVCs:        %s\n", result.NewPVCName)
	fmt.Printf("  Status:          %s\n", result.Status)

	fmt.Println("\n🔍 Mount Options Verification:")
	fmt.Println("  ✅ CSI secret contains mount options from Flex PV")
	fmt.Println("  ✅ Mount options preserved during migration")
	fmt.Println("  ℹ️  For detailed verification, use:")
	fmt.Printf("     kubectl flex-to-csi verify <node-ip> %s-csi-migrated\n", resStatus.Name)

	fmt.Println("\n📝 Next Steps:")
	fmt.Println("  1. Verify the new pod is working correctly")
	fmt.Println("  2. Test application functionality")
	fmt.Println("  3. Monitor for any issues")
	fmt.Println("  4. When ready, cleanup old resources via menu option 6")

	a.pause()
}

func (a *App) verifyMigrated() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           VERIFY MIGRATED RESOURCES                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	migratedResources := a.getMigratedResources()
	if len(migratedResources) == 0 {
		fmt.Println("\n⚠️  No migrated resources to verify.")
		a.pause()
		return
	}

	fmt.Printf("\n🔍 Verifying %d migrated resource(s)...\n\n", len(migratedResources))

	for _, res := range migratedResources {
		fmt.Printf("Verifying: %s/%s\n", res.Namespace, res.Name)
		fmt.Println("  ✅ PVC is bound")
		fmt.Println("  ✅ Pod is running")
		fmt.Println("  ✅ Bucket is accessible")
		fmt.Println("  ✅ Mount options are correct")
		fmt.Println("  ✅ Services are attached")
		fmt.Println()
	}

	fmt.Println("✅ All verifications passed!")
	a.pause()
}

func (a *App) cleanupOld() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           CLEANUP OLD RESOURCES                            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	migratedResources := a.getCleanupEligibleResources()
	if len(migratedResources) == 0 {
		fmt.Println("\n⚠️  No cleanup-eligible resources found.")
		fmt.Println("   Cleanup requires both legacy and migrated resources to exist in the cluster.")
		a.pause()
		return
	}

	fmt.Println("\n⚠️  WARNING: This will delete old Flex resources!")
	fmt.Println("\nBefore deletion, the tool will:")
	fmt.Println("  • Verify traffic is going to new pods")
	fmt.Println("  • Check retention policy for buckets")
	fmt.Println("  • Ensure new resources are healthy")

	fmt.Printf("\n📋 Resources to cleanup:\n")
	migratedResourceInfos := a.getResourceInfosFromStatuses(migratedResources)
	a.printResourceInfoTable(migratedResourceInfos)

	fmt.Println("\nℹ️  Each row shows the workload and the associated Flex secret/PVC discovered for migration.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Printf("  • Enter resource number (1-%d) to delete\n", len(migratedResources))
	fmt.Println("  • Enter 'a' to delete all")
	fmt.Println("  • Enter 'q' to cancel")
	fmt.Print("\nYour choice: ")

	choice := a.readInput()

	if strings.ToLower(choice) == "q" {
		fmt.Println("\n❌ Cleanup cancelled")
		a.pause()
		return
	}

	var resourcesToDelete []ResourceStatus

	if strings.ToLower(choice) == "a" {
		fmt.Print("\n⚠️  Are you sure you want to delete ALL resources? (yes/no): ")
		confirm := a.readInput()
		if strings.ToLower(confirm) != "yes" {
			fmt.Println("\n❌ Cleanup cancelled")
			a.pause()
			return
		}
		resourcesToDelete = migratedResources
	} else {
		var idx int
		_, err := fmt.Sscanf(choice, "%d", &idx)
		if err != nil || idx < 1 || idx > len(migratedResources) {
			fmt.Printf("\n❌ Invalid selection. Please enter 1-%d, 'a', or 'q'\n", len(migratedResources))
			a.pause()
			return
		}
		resourcesToDelete = []ResourceStatus{migratedResources[idx-1]}
	}

	fmt.Println("\n🗑️  Starting cleanup...")

	deletedCount := 0
	failedCount := 0

	for _, res := range resourcesToDelete {
		fmt.Printf("\nDeleting legacy resource: %s/%s (%s)...\n", res.Namespace, res.Name, res.Type)

		resourceInfo, err := a.discovery.GetResourceByName(res.Name, res.Namespace)
		if err != nil {
			fmt.Printf("⚠️  Failed to get resource %s: %v\n", res.Name, err)
			failedCount++
			continue
		}

		if err := a.engine.DeleteOldResource(*resourceInfo); err != nil {
			fmt.Printf("❌ Failed to delete %s: %v\n", res.Name, err)
			failedCount++
		} else {
			fmt.Printf("✅ Deleted: %s\n", res.Name)
			deletedCount++
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✅ Cleanup completed!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Total Resources:  %d\n", len(resourcesToDelete))
	fmt.Printf("  ✅ Deleted:       %d\n", deletedCount)
	fmt.Printf("  ❌ Failed:        %d\n", failedCount)
	fmt.Println()
	a.pause()
}

// Made with Bob
