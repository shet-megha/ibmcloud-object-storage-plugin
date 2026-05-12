package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kubectl-flex-to-csi/pkg/logger"

	"github.com/sirupsen/logrus"
)

// MigrationState tracks the state of an ongoing migration
type MigrationState struct {
	ResourceName     string            `json:"resourceName"`
	ResourceType     string            `json:"resourceType"`
	Namespace        string            `json:"namespace"`
	CurrentStep      int               `json:"currentStep"`
	CompletedSteps   []string          `json:"completedSteps"`
	CreatedResources []CreatedResource `json:"createdResources"`
	StartTime        time.Time         `json:"startTime"`
	LastUpdated      time.Time         `json:"lastUpdated"`
	Status           string            `json:"status"` // "in-progress", "completed", "failed"
	ErrorMessage     string            `json:"errorMessage,omitempty"`
}

// CreatedResource represents a resource created during migration
type CreatedResource struct {
	Type      string    `json:"type"` // Secret, PVC, Deployment, etc.
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	CreatedAt time.Time `json:"createdAt"`
}

// NewMigrationState creates a new migration state
func NewMigrationState(resourceName, resourceType, namespace string) *MigrationState {
	now := time.Now()
	return &MigrationState{
		ResourceName:     resourceName,
		ResourceType:     resourceType,
		Namespace:        namespace,
		CurrentStep:      0,
		CompletedSteps:   []string{},
		CreatedResources: []CreatedResource{},
		StartTime:        now,
		LastUpdated:      now,
		Status:           "in-progress",
	}
}

// AddCreatedResource adds a resource to the state
func (s *MigrationState) AddCreatedResource(resourceType, name, namespace string) {
	s.CreatedResources = append(s.CreatedResources, CreatedResource{
		Type:      resourceType,
		Name:      name,
		Namespace: namespace,
		CreatedAt: time.Now(),
	})
	s.LastUpdated = time.Now()

	logger.WithFields(logrus.Fields{
		"type":      resourceType,
		"name":      name,
		"namespace": namespace,
	}).Debug("Added created resource to state")
}

// CompleteStep marks a step as completed
func (s *MigrationState) CompleteStep(stepName string) {
	s.CompletedSteps = append(s.CompletedSteps, stepName)
	s.CurrentStep++
	s.LastUpdated = time.Now()

	logger.WithFields(logrus.Fields{
		"step":        stepName,
		"currentStep": s.CurrentStep,
	}).Debug("Completed migration step")
}

// MarkFailed marks the migration as failed
func (s *MigrationState) MarkFailed(err error) {
	s.Status = "failed"
	s.ErrorMessage = err.Error()
	s.LastUpdated = time.Now()

	logger.WithError(err).Error("Migration marked as failed")
}

// MarkCompleted marks the migration as completed
func (s *MigrationState) MarkCompleted() {
	s.Status = "completed"
	s.LastUpdated = time.Now()

	logger.Info("Migration marked as completed")
}

// Save persists the migration state to disk
func (s *MigrationState) Save() error {
	stateDir := filepath.Join(os.Getenv("HOME"), ".kube", "flex-to-csi-state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %v", err)
	}

	filename := filepath.Join(stateDir, fmt.Sprintf("%s-%s.json", s.Namespace, s.ResourceName))
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}

	logger.WithField("file", filename).Debug("Migration state saved")
	return nil
}

// LoadMigrationState loads a migration state from disk
func LoadMigrationState(namespace, resourceName string) (*MigrationState, error) {
	stateDir := filepath.Join(os.Getenv("HOME"), ".kube", "flex-to-csi-state")
	filename := filepath.Join(stateDir, fmt.Sprintf("%s-%s.json", namespace, resourceName))

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %v", err)
	}

	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %v", err)
	}

	logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"resource":  resourceName,
		"status":    state.Status,
	}).Debug("Migration state loaded")

	return &state, nil
}

// DeleteState removes the state file
func (s *MigrationState) DeleteState() error {
	stateDir := filepath.Join(os.Getenv("HOME"), ".kube", "flex-to-csi-state")
	filename := filepath.Join(stateDir, fmt.Sprintf("%s-%s.json", s.Namespace, s.ResourceName))

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %v", err)
	}

	logger.WithField("file", filename).Debug("Migration state deleted")
	return nil
}

// ListMigrationStates lists all migration states
func ListMigrationStates() ([]*MigrationState, error) {
	stateDir := filepath.Join(os.Getenv("HOME"), ".kube", "flex-to-csi-state")

	// Check if directory exists
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return []*MigrationState{}, nil
	}

	files, err := os.ReadDir(stateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %v", err)
	}

	var states []*MigrationState
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(stateDir, file.Name()))
		if err != nil {
			logger.WithError(err).Warnf("Failed to read state file: %s", file.Name())
			continue
		}

		var state MigrationState
		if err := json.Unmarshal(data, &state); err != nil {
			logger.WithError(err).Warnf("Failed to unmarshal state file: %s", file.Name())
			continue
		}

		states = append(states, &state)
	}

	return states, nil
}

// Made with Bob
