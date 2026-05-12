package interactive

import (
	"fmt"
	"strings"
	"time"

	"kubectl-flex-to-csi/pkg/discovery"
)

func (a *App) showWelcome() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                            ║")
	fmt.Println("║     IBM Cloud Object Storage FlexVolume to CSI            ║")
	fmt.Println("║              Migration Tool v2.0                           ║")
	fmt.Println("║                                                            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func (a *App) viewStatus() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           MIGRATION STATUS                                 ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	a.refreshStatusesFromCluster()

	if a.tracker.TotalResources == 0 {
		fmt.Println("\n⚠️  No resources tracked yet.")
		fmt.Println("   Please run 'Discover Resources' first.")
		a.pause()
		return
	}

	fmt.Println("\n📊 Overall Status:")
	fmt.Println("┌─────────────────────────────────────────┐")
	fmt.Printf("│ Total Resources:      %-17d │\n", a.tracker.TotalResources)
	fmt.Printf("│ ✅ Migrated:          %-17d │\n", a.tracker.MigratedCount)
	fmt.Printf("│ ⏳ Pending:           %-17d │\n", a.tracker.PendingCount)
	fmt.Printf("│ 🔄 In Progress:       %-17d │\n", a.tracker.InProgressCount)
	fmt.Printf("│ ❌ Failed:            %-17d │\n", a.tracker.FailedCount)
	fmt.Println("└─────────────────────────────────────────┘")

	if a.tracker.TotalResources > 0 {
		percentage := (a.tracker.MigratedCount * 100) / a.tracker.TotalResources
		fmt.Printf("\n📈 Progress: %d%% complete\n", percentage)
		a.showProgressBar(percentage)
	}

	fmt.Println("\n📋 Resource Details:")
	fmt.Println("┌────┬─────────────────────┬─────────────┬────────────┬──────────────┬──────────────────────────┐")
	fmt.Println("│ #  │ Resource Name       │ Type        │ Status     │ Namespace    │ Migration Time (US)      │")
	fmt.Println("├────┼─────────────────────┼─────────────┼────────────┼──────────────┼──────────────────────────┤")

	displayIndex := 1
	for _, res := range a.tracker.Resources {
		if res.Name == "" || res.Type == "" {
			continue
		}

		statusIcon := a.getStatusIcon(res.Status)
		fmt.Printf("│ %-2d │ %-19s │ %-11s │ %s %-8s │ %-12s │ %-24s │\n",
			displayIndex,
			truncate(res.Name, 19),
			res.Type,
			statusIcon,
			res.Status,
			truncate(res.Namespace, 12),
			truncate(formatUSTime(res.LastUpdated), 24))
		displayIndex++
	}
	fmt.Println("└────┴─────────────────────┴─────────────┴────────────┴──────────────┴──────────────────────────┘")

	a.pause()
}

func (a *App) detailedReport() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║           DETAILED MIGRATION REPORT                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	if a.tracker.TotalResources == 0 {
		fmt.Println("\n⚠️  No migration data available.")
		a.pause()
		return
	}

	a.refreshStatusesFromCluster()

	fmt.Println("\n📊 Migration Summary:")
	fmt.Printf("  Total Resources:    %d\n", a.tracker.TotalResources)
	fmt.Printf("  ✅ Migrated:        %d\n", a.tracker.MigratedCount)
	fmt.Printf("  ⏳ Pending:         %d\n", a.tracker.PendingCount)
	fmt.Printf("  🔄 In Progress:     %d\n", a.tracker.InProgressCount)
	fmt.Printf("  ❌ Failed:          %d\n", a.tracker.FailedCount)

	if a.tracker.MigratedCount > 0 {
		fmt.Println("\n✅ Migrated Resources:")
		fmt.Println("┌────┬─────────────────────┬─────────────┬──────────────┬──────────────────────────┬──────────────┐")
		fmt.Println("│ #  │ Resource Name       │ Type        │ Namespace    │ Migration Time (US)      │ New PVCs     │")
		fmt.Println("├────┼─────────────────────┼─────────────┼──────────────┼──────────────────────────┼──────────────┤")

		displayIndex := 1
		for _, res := range a.tracker.Resources {
			if res.Status == "migrated" {
				fmt.Printf("│ %-2d │ %-19s │ %-11s │ %-12s │ %-24s │ %-12s │\n",
					displayIndex,
					truncate(res.Name, 19),
					truncate(res.Type, 11),
					truncate(res.Namespace, 12),
					truncate(formatUSTime(res.LastUpdated), 24),
					truncate(strings.Join(res.NewPVCs, ", "), 12))
				displayIndex++
			}
		}
		fmt.Println("└────┴─────────────────────┴─────────────┴──────────────┴──────────────────────────┴──────────────┘")
	}

	if a.tracker.FailedCount > 0 {
		fmt.Println("\n❌ Failed Resources:")
		for _, res := range a.tracker.Resources {
			if res.Status == "failed" {
				fmt.Printf("\n  Resource: %s/%s (%s)\n", res.Namespace, res.Name, res.Type)
				fmt.Printf("  Error: %s\n", res.Error)
			}
		}
	}

	if a.tracker.PendingCount > 0 {
		fmt.Println("\n⏳ Pending Resources:")
		for _, res := range a.tracker.Resources {
			if res.Status == "pending" {
				fmt.Printf("  • %s/%s (%s)\n", res.Namespace, res.Name, res.Type)
			}
		}
	}

	a.pause()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func uniqueJoinedPVCField(pvcs []discovery.PVCInfo, selector func(discovery.PVCInfo) string) string {
	seen := make(map[string]struct{})
	values := make([]string, 0, len(pvcs))

	for _, pvc := range pvcs {
		value := strings.TrimSpace(selector(pvc))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}

	if len(values) == 0 {
		return "-"
	}

	return strings.Join(values, ", ")
}

func (a *App) printResourceInfoTable(resources []discovery.ResourceInfo) {
	fmt.Println("┌────┬──────────────────────┬─────────────┬──────────────┬──────────────────────┬──────────────────────┬────────────┬──────────────┐")
	fmt.Println("│ #  │ Resource Name        │ Type        │ Namespace    │ Flex Secret          │ Flex PVC             │ PVC Status │ Migration    │")
	fmt.Println("│    │                      │             │              │                      │                      │            │ Support      │")
	fmt.Println("├────┼──────────────────────┼─────────────┼──────────────┼──────────────────────┼──────────────────────┼────────────┼──────────────┤")

	for i, res := range resources {
		flexSecrets := uniqueJoinedPVCField(res.PVCs, func(pvc discovery.PVCInfo) string {
			return pvc.SecretName
		})
		flexPVCs := uniqueJoinedPVCField(res.PVCs, func(pvc discovery.PVCInfo) string {
			return pvc.Name
		})
		pvcStatus := uniqueJoinedPVCField(res.PVCs, func(pvc discovery.PVCInfo) string {
			return pvc.Status
		})

		resourceName := res.Name
		if resourceName == "" {
			resourceName = "(No workload)"
		}

		resourceType := res.Type
		if resourceType == "" {
			resourceType = "Orphaned PVC"
		}

		migrationSupport := "Supported"
		if res.Name == "" || res.Type == "" {
			migrationSupport = "Unsupported"
		} else if pvcStatus == "Pending" || strings.Contains(pvcStatus, "Pending") {
			migrationSupport = "Unsupported"
		}

		fmt.Printf("│ %-2d │ %-20s │ %-11s │ %-12s │ %-20s │ %-20s │ %-10s │ %-12s │\n",
			i+1,
			truncate(resourceName, 20),
			truncate(resourceType, 11),
			truncate(res.Namespace, 12),
			truncate(flexSecrets, 20),
			truncate(flexPVCs, 20),
			truncate(pvcStatus, 10),
			migrationSupport)
	}

	fmt.Println("└────┴──────────────────────┴─────────────┴──────────────┴──────────────────────┴──────────────────────┴────────────┴──────────────┘")
	fmt.Println("\nℹ️  Each row shows the workload and the associated Flex secret/PVC discovered for migration.")
	fmt.Println("   Migration Support: 'Unsupported' for orphaned PVCs or PVCs in Pending state.")
}

func formatUSTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	usLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		return t.Format("02 Jan 2006 03:04:05 PM MST")
	}

	return t.In(usLocation).Format("02 Jan 2006 03:04:05 PM MST")
}

// Made with Bob
