package migration

import (
	"context"
	"fmt"

	"kubectl-flex-to-csi/pkg/logger"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RollbackEngine handles rollback of failed migrations
type RollbackEngine struct {
	clientset *kubernetes.Clientset
	ctx       context.Context
	logger    *logrus.Entry
}

// NewRollbackEngine creates a new rollback engine
func NewRollbackEngine(clientset *kubernetes.Clientset) *RollbackEngine {
	return &RollbackEngine{
		clientset: clientset,
		ctx:       context.Background(),
		logger:    logger.WithField("component", "rollback-engine"),
	}
}

// Rollback performs a complete rollback of a migration
func (r *RollbackEngine) Rollback(state *MigrationState) error {
	r.logger.WithFields(logrus.Fields{
		"resource":  state.ResourceName,
		"namespace": state.Namespace,
		"step":      state.CurrentStep,
		"created":   len(state.CreatedResources),
	}).Info("Starting rollback")

	if len(state.CreatedResources) == 0 {
		r.logger.Info("No resources to rollback")
		return nil
	}

	// Delete resources in reverse order (LIFO)
	successCount := 0
	failCount := 0

	for i := len(state.CreatedResources) - 1; i >= 0; i-- {
		resource := state.CreatedResources[i]

		r.logger.WithFields(logrus.Fields{
			"type":      resource.Type,
			"name":      resource.Name,
			"namespace": resource.Namespace,
		}).Info("Deleting resource")

		if err := r.deleteResource(resource); err != nil {
			r.logger.WithError(err).Errorf("Failed to delete %s: %s/%s", resource.Type, resource.Namespace, resource.Name)
			failCount++
			continue
		}

		r.logger.Infof("✅ Deleted %s: %s/%s", resource.Type, resource.Namespace, resource.Name)
		successCount++
	}

	// Remove state file
	if err := state.DeleteState(); err != nil {
		r.logger.WithError(err).Warn("Failed to delete state file")
	}

	r.logger.WithFields(logrus.Fields{
		"success": successCount,
		"failed":  failCount,
		"total":   len(state.CreatedResources),
	}).Info("Rollback completed")

	if failCount > 0 {
		return fmt.Errorf("rollback completed with %d failures out of %d resources", failCount, len(state.CreatedResources))
	}

	return nil
}

// deleteResource deletes a single resource
func (r *RollbackEngine) deleteResource(resource CreatedResource) error {
	deleteOptions := metav1.DeleteOptions{}

	switch resource.Type {
	case "Secret":
		return r.clientset.CoreV1().Secrets(resource.Namespace).Delete(r.ctx, resource.Name, deleteOptions)

	case "PVC":
		return r.clientset.CoreV1().PersistentVolumeClaims(resource.Namespace).Delete(r.ctx, resource.Name, deleteOptions)

	case "Deployment":
		return r.clientset.AppsV1().Deployments(resource.Namespace).Delete(r.ctx, resource.Name, deleteOptions)

	case "StatefulSet":
		return r.clientset.AppsV1().StatefulSets(resource.Namespace).Delete(r.ctx, resource.Name, deleteOptions)

	case "DaemonSet":
		return r.clientset.AppsV1().DaemonSets(resource.Namespace).Delete(r.ctx, resource.Name, deleteOptions)

	case "Pod":
		return r.clientset.CoreV1().Pods(resource.Namespace).Delete(r.ctx, resource.Name, deleteOptions)

	default:
		return fmt.Errorf("unknown resource type: %s", resource.Type)
	}
}

// RollbackByName rolls back a migration by resource name
func (r *RollbackEngine) RollbackByName(namespace, resourceName string) error {
	state, err := LoadMigrationState(namespace, resourceName)
	if err != nil {
		return fmt.Errorf("failed to load migration state: %v", err)
	}

	return r.Rollback(state)
}

// ListRollbackCandidates lists migrations that can be rolled back
func (r *RollbackEngine) ListRollbackCandidates() ([]*MigrationState, error) {
	states, err := ListMigrationStates()
	if err != nil {
		return nil, err
	}

	var candidates []*MigrationState
	for _, state := range states {
		// Only include failed or in-progress migrations
		if state.Status == "failed" || state.Status == "in-progress" {
			candidates = append(candidates, state)
		}
	}

	return candidates, nil
}

// AutoRollback automatically rolls back failed migrations
func (r *RollbackEngine) AutoRollback() error {
	candidates, err := r.ListRollbackCandidates()
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		r.logger.Info("No failed migrations to rollback")
		return nil
	}

	r.logger.Infof("Found %d failed migration(s) to rollback", len(candidates))

	for _, state := range candidates {
		r.logger.WithFields(logrus.Fields{
			"resource":  state.ResourceName,
			"namespace": state.Namespace,
			"status":    state.Status,
		}).Info("Rolling back failed migration")

		if err := r.Rollback(state); err != nil {
			r.logger.WithError(err).Errorf("Failed to rollback %s/%s", state.Namespace, state.ResourceName)
			continue
		}
	}

	return nil
}

// Made with Bob
