package migration

import (
	"fmt"
	"math"
	"strings"
	"time"

	"kubectl-flex-to-csi/pkg/logger"

	"github.com/sirupsen/logrus"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	RetryableErrors []string
}

// DefaultRetryConfig provides sensible defaults for retrying operations
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 1 * time.Second,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
	RetryableErrors: []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"resource temporarily unavailable",
		"too many requests",
		"service unavailable",
		"internal server error",
	},
}

// PVCBindRetryConfig is optimized for waiting for PVC binding
var PVCBindRetryConfig = RetryConfig{
	MaxAttempts:  60, // 5 minutes with 5-second intervals
	InitialDelay: 5 * time.Second,
	MaxDelay:     5 * time.Second,
	Multiplier:   1.0,
	RetryableErrors: []string{
		"not bound",
		"pending",
	},
}

// PodStartRetryConfig is optimized for waiting for pods to start
var PodStartRetryConfig = RetryConfig{
	MaxAttempts:  120, // 10 minutes with 5-second intervals
	InitialDelay: 5 * time.Second,
	MaxDelay:     5 * time.Second,
	Multiplier:   1.0,
	RetryableErrors: []string{
		"not running",
		"pending",
		"container creating",
	},
}

// WithRetry executes an operation with exponential backoff retry logic
func WithRetry(config RetryConfig, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt-1)))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			logger.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"delay":   delay,
			}).Debug("Retrying operation after delay")

			time.Sleep(delay)
		}

		// Execute the operation
		err := operation()
		if err == nil {
			if attempt > 0 {
				logger.WithField("attempts", attempt+1).Info("Operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryable(err, config.RetryableErrors) {
			logger.WithError(err).Debug("Error is not retryable, failing immediately")
			return err
		}

		logger.WithFields(logrus.Fields{
			"attempt":     attempt + 1,
			"maxAttempts": config.MaxAttempts,
			"error":       err.Error(),
		}).Warn("Operation failed, will retry")
	}

	return fmt.Errorf("operation failed after %d attempts: %v", config.MaxAttempts, lastErr)
}

// isRetryable checks if an error should trigger a retry
func isRetryable(err error, retryableErrors []string) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, strings.ToLower(retryable)) {
			return true
		}
	}
	return false
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures int
	timeout     time.Duration
	failures    int
	lastFailure time.Time
	state       string // "closed", "open", "half-open"
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       "closed",
	}
}

// Call executes an operation through the circuit breaker
func (cb *CircuitBreaker) Call(operation func() error) error {
	// Check if circuit is open
	if cb.state == "open" {
		if time.Since(cb.lastFailure) > cb.timeout {
			logger.Debug("Circuit breaker transitioning to half-open state")
			cb.state = "half-open"
		} else {
			return fmt.Errorf("circuit breaker is open, rejecting call")
		}
	}

	// Execute operation
	err := operation()
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.failures >= cb.maxFailures {
			logger.WithFields(logrus.Fields{
				"failures":    cb.failures,
				"maxFailures": cb.maxFailures,
			}).Warn("Circuit breaker opening due to failures")
			cb.state = "open"
		}
		return err
	}

	// Success - reset circuit breaker
	if cb.state == "half-open" {
		logger.Info("Circuit breaker closing after successful call")
	}
	cb.failures = 0
	cb.state = "closed"
	return nil
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() string {
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.failures = 0
	cb.state = "closed"
	logger.Info("Circuit breaker manually reset")
}

// Made with Bob
