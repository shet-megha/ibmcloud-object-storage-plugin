package interactive

import (
	"strings"
	"time"

	"kubectl-flex-to-csi/pkg/migration"
)

func (a *App) updateResourceStatus(name, status, errorMsg string) {
	for i := range a.tracker.Resources {
		if a.tracker.Resources[i].Name == name {
			oldStatus := a.tracker.Resources[i].Status
			a.tracker.Resources[i].Status = status
			a.tracker.Resources[i].Error = errorMsg
			a.tracker.Resources[i].LastUpdated = time.Now()

			a.updateCounters(oldStatus, status)
			break
		}
	}
}

func (a *App) updateResourceStatusWithResult(name string, result *migration.MigrationResult) {
	finalStatus := result.Status
	if result.Status == "success" {
		finalStatus = "migrated"
	}

	for i := range a.tracker.Resources {
		if a.tracker.Resources[i].Name == name {
			oldStatus := a.tracker.Resources[i].Status
			a.tracker.Resources[i].Status = finalStatus
			a.tracker.Resources[i].MigrationTime = result.MigrationTime
			a.tracker.Resources[i].NewPVCs = []string{result.NewPVCName}
			a.tracker.Resources[i].LastUpdated = time.Now()

			if result.Error != nil {
				a.tracker.Resources[i].Error = result.Error.Error()
			}

			a.updateCounters(oldStatus, finalStatus)
			break
		}
	}
}

func (a *App) updateCounters(oldStatus, newStatus string) {
	switch oldStatus {
	case "pending":
		a.tracker.PendingCount--
	case "in-progress":
		a.tracker.InProgressCount--
	case "migrated":
		a.tracker.MigratedCount--
	case "failed":
		a.tracker.FailedCount--
	}

	switch newStatus {
	case "pending":
		a.tracker.PendingCount++
	case "in-progress":
		a.tracker.InProgressCount++
	case "migrated":
		a.tracker.MigratedCount++
	case "failed":
		a.tracker.FailedCount++
	}
}

func (a *App) getPendingResources() []ResourceStatus {
	var pending []ResourceStatus
	for _, res := range a.tracker.Resources {
		if res.Status == "pending" {
			pending = append(pending, res)
		}
	}
	return pending
}

func (a *App) getMigratedResources() []ResourceStatus {
	a.refreshStatusesFromCluster()

	var migrated []ResourceStatus
	for _, res := range a.tracker.Resources {
		if res.Status == "migrated" {
			migrated = append(migrated, res)
		}
	}

	if len(migrated) > 0 {
		return migrated
	}

	clusterMigratedResources, err := a.discovery.DiscoverMigratedResourcesOptimized("")
	if err != nil {
		return migrated
	}

	for _, res := range clusterMigratedResources {
		baseName := strings.TrimSuffix(res.Name, "-csi-migrated")

		newPVCs := make([]string, 0, len(res.PVCs))
		for _, pvc := range res.PVCs {
			newPVCs = append(newPVCs, pvc.Name)
		}

		migrated = append(migrated, ResourceStatus{
			Name:        baseName,
			Namespace:   res.Namespace,
			Type:        res.Type,
			Status:      "migrated",
			NewPVCs:     newPVCs,
			LastUpdated: time.Now(),
		})
	}

	return migrated
}

func (a *App) getCleanupEligibleResources() []ResourceStatus {
	a.refreshStatusesFromCluster()

	var cleanupEligible []ResourceStatus
	for _, res := range a.tracker.Resources {
		if a.discovery.LegacyAndMigratedResourcesExist(res.Name, res.Namespace) {
			cleanupEligible = append(cleanupEligible, res)
		}
	}

	return cleanupEligible
}

func (a *App) refreshStatusesFromCluster() {
	migratedCount := 0
	pendingCount := 0
	inProgressCount := 0
	failedCount := 0

	for i := range a.tracker.Resources {
		res := &a.tracker.Resources[i]
		migratedName := res.Name + "-csi-migrated"

		if migratedInfo, err := a.discovery.GetResourceByName(migratedName, res.Namespace); err == nil && migratedInfo != nil {
			res.Status = "migrated"
			res.Type = migratedInfo.Type
			res.LastUpdated = time.Now()

			newPVCs := make([]string, 0, len(migratedInfo.PVCs))
			for _, pvc := range migratedInfo.PVCs {
				newPVCs = append(newPVCs, pvc.Name)
			}
			if len(newPVCs) > 0 {
				res.NewPVCs = newPVCs
			}
		}

		switch res.Status {
		case "migrated":
			migratedCount++
		case "pending":
			pendingCount++
		case "in-progress":
			inProgressCount++
		case "failed":
			failedCount++
		}
	}

	a.tracker.MigratedCount = migratedCount
	a.tracker.PendingCount = pendingCount
	a.tracker.InProgressCount = inProgressCount
	a.tracker.FailedCount = failedCount
}

func (a *App) getStatusIcon(status string) string {
	switch status {
	case "migrated":
		return "✅"
	case "pending":
		return "⏳"
	case "in-progress":
		return "🔄"
	case "failed":
		return "❌"
	default:
		return "❓"
	}
}

// Made with Bob
