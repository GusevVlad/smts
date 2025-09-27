package types

import "fmt"

// SMTSError represents a custom error type for SMTS operations
type SMTSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Err     error  `json:"-"`
}

// Error implements the error interface
func (e *SMTSError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("SMTS Error [%s]: %s - %s (details: %s)", e.Code, e.Message, e.Err.Error(), e.Details)
	}
	return fmt.Sprintf("SMTS Error [%s]: %s (details: %s)", e.Code, e.Message, e.Details)
}

// Unwrap returns the underlying error
func (e *SMTSError) Unwrap() error {
	return e.Err
}

// Error codes for different types of failures
const (
	// Configuration errors
	ErrConfigLoad     = "CONFIG_LOAD_ERROR"
	ErrConfigValidate = "CONFIG_VALIDATION_ERROR"

	// NATS errors
	ErrNATSConnection = "NATS_CONNECTION_ERROR"
	ErrNATSStream     = "NATS_STREAM_ERROR"
	ErrNATSConsumer   = "NATS_CONSUMER_ERROR"
	ErrNATSPublish    = "NATS_PUBLISH_ERROR"
	ErrNATSConsume    = "NATS_CONSUME_ERROR"

	// API errors
	ErrAPIConnection  = "API_CONNECTION_ERROR"
	ErrAPIRequest     = "API_REQUEST_ERROR"
	ErrAPIResponse    = "API_RESPONSE_ERROR"
	ErrAPIAuth        = "API_AUTHENTICATION_ERROR"

	// DLP errors
	ErrDLPConnection = "DLP_CONNECTION_ERROR"
	ErrDLPRequest    = "DLP_REQUEST_ERROR"
	ErrDLPValidation = "DLP_VALIDATION_ERROR"

	// Artemis errors
	ErrArtemisConnection = "ARTEMIS_CONNECTION_ERROR"
	ErrArtemisConsume    = "ARTEMIS_CONSUME_ERROR"
	ErrArtemisPublish    = "ARTEMIS_PUBLISH_ERROR"

	// Message processing errors
	ErrMessageValidation = "MESSAGE_VALIDATION_ERROR"
	ErrMessageRouting    = "MESSAGE_ROUTING_ERROR"
	ErrPermissionDenied  = "PERMISSION_DENIED_ERROR"

	// System errors
	ErrSystemStartup   = "SYSTEM_STARTUP_ERROR"
	ErrSystemShutdown  = "SYSTEM_SHUTDOWN_ERROR"
	ErrHealthCheck     = "HEALTH_CHECK_ERROR"
	ErrResourceExhausted = "RESOURCE_EXHAUSTED_ERROR"
)

// NewSMTSError creates a new SMTS error
func NewSMTSError(code, message string) *SMTSError {
	return &SMTSError{
		Code:    code,
		Message: message,
	}
}

// NewSMTSErrorWithDetails creates a new SMTS error with details
func NewSMTSErrorWithDetails(code, message, details string) *SMTSError {
	return &SMTSError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// WrapSMTSError wraps an existing error with SMTS error context
func WrapSMTSError(err error, code, message string) *SMTSError {
	return &SMTSError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// WrapSMTSErrorWithDetails wraps an existing error with SMTS error context and details
func WrapSMTSErrorWithDetails(err error, code, message, details string) *SMTSError {
	return &SMTSError{
		Code:    code,
		Message: message,
		Details: details,
		Err:     err,
	}
}

// Common error messages
const (
	MsgConfigLoadFailed     = "Failed to load configuration"
	MsgNATSConnectionFailed = "Failed to connect to NATS"
	MsgAPIRequestFailed     = "API request failed"
	MsgDLPValidationFailed  = "DLP validation failed"
	MsgPermissionDenied     = "Permission denied for topic"
	MsgMessageInvalid       = "Message validation failed"
)

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if smtsErr, ok := err.(*SMTSError); ok {
		switch smtsErr.Code {
		case ErrAPIConnection, ErrAPIResponse, ErrDLPConnection,
			ErrArtemisConnection, ErrNATSConnection, ErrResourceExhausted:
			// Connection errors and server errors (5xx) are retryable
			return true
		case ErrAPIRequest, ErrDLPRequest:
			// Client errors (4xx) are NOT retryable - they indicate permanent issues
			return false
		default:
			return false
		}
	}
	return false
}

// IsPermissionError checks if an error is related to permissions
func IsPermissionError(err error) bool {
	if smtsErr, ok := err.(*SMTSError); ok {
		return smtsErr.Code == ErrPermissionDenied
	}
	return false
}

// IsValidationError checks if an error is related to validation
func IsValidationError(err error) bool {
	if smtsErr, ok := err.(*SMTSError); ok {
		return smtsErr.Code == ErrMessageValidation || smtsErr.Code == ErrDLPValidation
	}
	return false
}