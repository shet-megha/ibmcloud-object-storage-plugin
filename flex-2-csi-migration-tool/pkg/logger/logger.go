package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10MB
	maxFiles    = 10               // keep last 10 files
)

var (
	log         *logrus.Logger
	logFile     *os.File
	logDir      = "logs"
	currentDate string
	mu          sync.Mutex
	initialized bool
)

// init initializes the logger on package load
func init() {
	InitLogger()
}

// InitLogger initializes the logger with file rotation
func InitLogger() {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return
	}

	// Create logs directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
		return
	}

	// Initialize logrus
	log = logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	log.SetLevel(logrus.InfoLevel)

	// Open log file
	if err := rotateLogFile(); err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		log.SetOutput(os.Stdout)
	}

	initialized = true

	// Start background rotation checker
	go periodicRotationCheck()
}

// rotateLogFile creates a new log file
func rotateLogFile() error {
	// Close existing file
	if logFile != nil {
		logFile.Close()
	}

	// Create new log file
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(logDir, fmt.Sprintf("kubectl-flex-to-csi_%s.log", timestamp))

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	logFile = file
	currentDate = time.Now().Format("2006-01-02")

	// Set output to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)
	log.SetOutput(multiWriter)

	// Clean up old files
	go cleanupOldLogs()

	return nil
}

// periodicRotationCheck checks if rotation is needed every minute
func periodicRotationCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		if shouldRotate() {
			rotateLogFile()
		}
		mu.Unlock()
	}
}

// shouldRotate checks if log rotation is needed
func shouldRotate() bool {
	if logFile == nil {
		return true
	}

	// Check date change
	today := time.Now().Format("2006-01-02")
	if currentDate != today {
		return true
	}

	// Check file size
	info, err := logFile.Stat()
	if err != nil {
		return false
	}

	return info.Size() >= maxFileSize
}

// cleanupOldLogs removes old log files
func cleanupOldLogs() {
	files, err := filepath.Glob(filepath.Join(logDir, "kubectl-flex-to-csi_*.log"))
	if err != nil || len(files) <= maxFiles {
		return
	}

	// Sort by modification time
	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, fileInfo{path: file, modTime: info.ModTime()})
	}

	// Simple bubble sort (oldest first)
	for i := 0; i < len(fileInfos)-1; i++ {
		for j := i + 1; j < len(fileInfos); j++ {
			if fileInfos[i].modTime.After(fileInfos[j].modTime) {
				fileInfos[i], fileInfos[j] = fileInfos[j], fileInfos[i]
			}
		}
	}

	// Remove oldest files
	filesToRemove := len(fileInfos) - maxFiles
	for i := 0; i < filesToRemove; i++ {
		os.Remove(fileInfos[i].path)
	}
}

// GetLogger returns the logger instance
func GetLogger() *logrus.Logger {
	if !initialized {
		InitLogger()
	}
	return log
}

// WithField creates an entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// WithFields creates an entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

// WithError creates an entry with an error
func WithError(err error) *logrus.Entry {
	return GetLogger().WithError(err)
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// LogMigrationStart logs the start of a migration
func LogMigrationStart(resourceName, namespace, resourceType string) {
	WithFields(logrus.Fields{
		"resource":  resourceName,
		"namespace": namespace,
		"type":      resourceType,
	}).Info("=== Migration Started ===")
}

// LogMigrationStep logs a migration step
func LogMigrationStep(step int, description string, success bool) {
	fields := logrus.Fields{
		"step":        step,
		"description": description,
	}
	if success {
		WithFields(fields).Info("Migration step completed")
	} else {
		WithFields(fields).Error("Migration step failed")
	}
}

// LogMigrationComplete logs the completion of a migration
func LogMigrationComplete(resourceName, namespace string, success bool, duration time.Duration) {
	fields := logrus.Fields{
		"resource":  resourceName,
		"namespace": namespace,
		"duration":  duration.String(),
	}
	if success {
		WithFields(fields).Info("=== Migration Completed Successfully ===")
	} else {
		WithFields(fields).Error("=== Migration Failed ===")
	}
}

// LogFault logs a fault/error with details
func LogFault(component, operation string, err error, details map[string]string) {
	fields := logrus.Fields{
		"component": component,
		"operation": operation,
	}
	for k, v := range details {
		fields[k] = v
	}
	WithFields(fields).WithError(err).Error("FAULT DETECTED")
}

// Close closes the log file
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		return logFile.Close()
	}
	return nil
}

// Made with Bob
