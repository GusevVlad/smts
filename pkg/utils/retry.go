package utils

import (
	"context"
	"math/rand"
	"time"

	"smts/pkg/types"
	"go.uber.org/zap"
)

// RetryConfig holds configuration for retry operations
type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
	MaxBackoff  time.Duration
	Jitter      bool
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		Backoff:     2 * time.Second,
		MaxBackoff:  30 * time.Second,
		Jitter:      true,
	}
}

// RetryableFunc defines a function that can be retried
type RetryableFunc func(attempt int) error

// Retry executes a function with retry logic
func Retry(ctx context.Context, config RetryConfig, fn RetryableFunc, logger *zap.Logger, operation string) error {
	var lastErr error

	logger.Debug("Retry function called",
		zap.String("operation", operation),
		zap.Int("max_attempts", config.MaxAttempts),
		zap.Duration("backoff", config.Backoff))

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		logger.Debug("Retry attempt starting",
			zap.String("operation", operation),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", config.MaxAttempts))
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the function
		err := fn(attempt)
		if err == nil {
			// Success
			if attempt > 1 {
				logger.Info("Operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt))
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !types.IsRetryableError(err) {
			logger.Warn("Non-retryable error, not retrying",
				zap.String("operation", operation),
				zap.Int("attempt", attempt),
				zap.Error(err))
			return err
		}

		// Log retry attempt
		logger.Warn("Operation failed, retrying",
			zap.String("operation", operation),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", config.MaxAttempts),
			zap.Error(err))

		// If this is the last attempt, break without sleeping
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate backoff duration
		backoff := calculateBackoff(config, attempt)

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	logger.Error("Operation failed after all retry attempts",
		zap.String("operation", operation),
		zap.Int("attempts", config.MaxAttempts),
		zap.Error(lastErr))

	return lastErr
}

// calculateBackoff calculates the backoff duration for a retry attempt
func calculateBackoff(config RetryConfig, attempt int) time.Duration {
	// Exponential backoff: base * 2^(attempt-1)
	backoff := config.Backoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
			break
		}
	}

	// Add jitter if enabled
	if config.Jitter && backoff > 0 {
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		backoff = backoff/2 + jitter
	}

	return backoff
}

// RetryWithResult executes a function with retry logic that returns a result
func RetryWithResult[T any](ctx context.Context, config RetryConfig, fn func(attempt int) (T, error), logger *zap.Logger, operation string) (T, error) {
	var zero T
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Execute the function
		result, err := fn(attempt)
		if err == nil {
			// Success
			if attempt > 1 {
				logger.Info("Operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt))
			}
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !types.IsRetryableError(err) {
			logger.Warn("Non-retryable error, not retrying",
				zap.String("operation", operation),
				zap.Int("attempt", attempt),
				zap.Error(err))
			return zero, err
		}

		// Log retry attempt
		logger.Warn("Operation failed, retrying",
			zap.String("operation", operation),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", config.MaxAttempts),
			zap.Error(err))

		// If this is the last attempt, break without sleeping
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate backoff duration
		backoff := calculateBackoff(config, attempt)

		// Wait before retry
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	logger.Error("Operation failed after all retry attempts",
		zap.String("operation", operation),
		zap.Int("attempts", config.MaxAttempts),
		zap.Error(lastErr))

	return zero, lastErr
}

// SimpleRetry is a simplified retry function for common use cases
func SimpleRetry(fn RetryableFunc, maxAttempts int, backoff time.Duration) error {
	config := RetryConfig{
		MaxAttempts: maxAttempts,
		Backoff:     backoff,
		Jitter:      true,
	}

	return Retry(context.Background(), config, fn, zap.NewNop(), "simple-retry")
}