package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"smts/internal/nats"
	"smts/pkg/types"
	"go.uber.org/zap"
)

// HTTPUtilities provides shared functionality for HTTP handlers
type HTTPUtilities struct {
	logger *zap.Logger
}

// NewHTTPUtilities creates a new HTTP utilities instance
func NewHTTPUtilities(logger *zap.Logger) *HTTPUtilities {
	return &HTTPUtilities{
		logger: logger,
	}
}

// MessageRequest represents a common message request structure
type MessageRequest struct {
	Topic     string
	MessageID string
	Timestamp time.Time
	Source    string
	Body      []byte
	Headers   map[string]string
	ClientSender string
}

// ParseMessageHeaders extracts and validates common message headers from HTTP request
func (u *HTTPUtilities) ParseMessageHeaders(r *http.Request) (string, string, string, string) {
	apiKey := r.Header.Get("X-API-Key")
	messageID := r.Header.Get("X-SMTS-Message-ID")
	timestamp := r.Header.Get("X-SMTS-Timestamp")
	source := r.Header.Get("X-SMTS-Source")

	// Set default source if not provided
	if source == "" {
		source = "http-client"
	}

	return apiKey, messageID, timestamp, source
}

// ValidateTopicAndAuth validates topic and LDAP authorization
func (u *HTTPUtilities) ValidateTopicAndAuth(r *http.Request, config *types.Config, ldapMiddleware *LDAPMiddleware, topic string, endpoint string) bool {
	// Check LDAP authorization
	if ldapMiddleware != nil && !ldapMiddleware.AuthorizeEndpoint(r, config.Deployment.Type, endpoint) {
		u.logger.Warn("LDAP authorization denied",
			zap.String("topic", topic),
			zap.String("deployment", config.Deployment.Type),
			zap.String("endpoint", endpoint))
		return false
	}

	// Check if topic exists in configuration
	if _, exists := config.Topics.Topics[topic]; !exists {
		u.logger.Warn("Topic not found in configuration",
			zap.String("topic", topic))
		return false
	}

	return true
}

// CreateMessageFromRequest creates a message from HTTP request data
func (u *HTTPUtilities) CreateMessageFromRequest(r *http.Request, topic string, apiKey string, messageID string, timestamp string, source string, body []byte) (*types.Message, error) {
	// Parse timestamp
	var msgTimestamp time.Time
	var err error
	if timestamp != "" {
		msgTimestamp, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			u.logger.Warn("Invalid timestamp format, using current time",
				zap.String("timestamp", timestamp),
				zap.String("topic", topic),
				zap.Error(err))
			msgTimestamp = time.Now().UTC()
		}
	} else {
		msgTimestamp = time.Now().UTC()
	}

	// Generate message ID if not provided
	if messageID == "" {
		messageID = nats.GenerateID()
		u.logger.Info("Generated message ID",
			zap.String("topic", topic),
			zap.String("message_id", messageID))
	}

	// Create message headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 && strings.HasPrefix(strings.ToLower(key), "x-smts-") {
			headers[key] = values[0]
		}
	}

	// Add additional headers
	headers["X-SMTS-API-Key"] = apiKey
	headers["X-SMTS-Source"] = source

	// Get client_sender from LDAP context
	clientSender := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientSender = userInfo["uid"]
	}

	// Create message
	msg := &types.Message{
		ID:           messageID,
		Timestamp:    msgTimestamp,
		Topic:        topic,
		Source:       source,
		ClientSender: clientSender,
		Headers:      headers,
		Body:         body,
	}

	return msg, nil
}

// ExtractTopicFromPath extracts topic from URL path with given prefix
func (u *HTTPUtilities) ExtractTopicFromPath(r *http.Request, prefix string) (string, bool) {
	if !strings.HasPrefix(r.URL.Path, prefix) {
		u.logger.Warn("Invalid endpoint",
			zap.String("path", r.URL.Path))
		return "", false
	}

	topic := strings.TrimPrefix(r.URL.Path, prefix)
	if topic == "" {
		u.logger.Warn("Missing topic in URL path",
			zap.String("path", r.URL.Path))
		return "", false
	}

	return topic, true
}

// ReadRequestBody reads and validates request body
func (u *HTTPUtilities) ReadRequestBody(r *http.Request, topic string) ([]byte, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		u.logger.Error("Failed to read request body",
			zap.String("topic", topic),
			zap.Error(err))
		return nil, false
	}
	defer r.Body.Close()

	u.logger.Info("Read message body from HTTP request",
		zap.String("topic", topic),
		zap.Int("body_size", len(body)))

	return body, true
}

// SendSuccessResponse sends a standardized success response
func (u *HTTPUtilities) SendSuccessResponse(w http.ResponseWriter, messageID string, topic string, deployment string, additionalFields map[string]interface{}) {
	response := map[string]interface{}{
		"status":     "delivered",
		"message_id": messageID,
		"topic":      topic,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"deployment": deployment,
	}

	// Add additional fields if provided
	for key, value := range additionalFields {
		response[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// SendErrorResponse sends a standardized error response
func (u *HTTPUtilities) SendErrorResponse(w http.ResponseWriter, statusCode int, errorMessage string) {
	u.JSONError(w, errorMessage, statusCode)
}

// JSONError sends a JSON error response with the specified status code
func (u *HTTPUtilities) JSONError(w http.ResponseWriter, error string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": error,
	})
}

// JSONErrorWithMessage sends a JSON error response with additional message
func (u *HTTPUtilities) JSONErrorWithMessage(w http.ResponseWriter, error string, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   error,
		"message": message,
	})
}

// JSONSuccess sends a JSON success response
func (u *HTTPUtilities) JSONSuccess(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// ParseCountParameter parses and validates count parameter from query string
func (u *HTTPUtilities) ParseCountParameter(countStr string) int {
	count := 1
	if countStr != "" {
		if parsedCount, err := strconv.Atoi(countStr); err == nil && parsedCount > 0 {
			count = parsedCount
			if count > 100 {
				count = 100
			}
		}
	}
	return count
}

// GetClientReceiver extracts client receiver from LDAP context
func (u *HTTPUtilities) GetClientReceiver(r *http.Request) string {
	clientReceiver := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientReceiver = userInfo["uid"]
	}
	return clientReceiver
}

// RequireMethod validates that the request uses the specified HTTP method
func (u *HTTPUtilities) RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		u.JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// RequireJSONBody validates that the request has a valid JSON body
func (u *HTTPUtilities) RequireJSONBody(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		u.JSONError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return false
	}
	return true
}

// RequireHeader validates that the request has the specified header
func (u *HTTPUtilities) RequireHeader(w http.ResponseWriter, r *http.Request, headerName string) string {
	value := r.Header.Get(headerName)
	if value == "" {
		u.JSONError(w, headerName+" header is required", http.StatusBadRequest)
		return ""
	}
	return value
}