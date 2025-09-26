package types

import (
	"encoding/json"
	"time"
)

// Message represents the unified message format for SMTS
type Message struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Topic     string            `json:"topic"`
	Source    string            `json:"source"`
	Headers   map[string]string `json:"headers"`
	Body      json.RawMessage   `json:"body"` // JSON raw type for flexible message content
}

// NewMessage creates a new message with the given topic and body
func NewMessage(topic string, body json.RawMessage) *Message {
	return &Message{
		ID:        generateID(),
		Timestamp: time.Now().UTC(),
		Topic:     topic,
		Source:    "smts",
		Headers:   make(map[string]string),
		Body:      body,
	}
}

// DeliveryResult represents the outcome of message delivery
type DeliveryResult struct {
	Success    bool      `json:"success"`
	MessageID  string    `json:"message_id"`
	Timestamp  time.Time `json:"timestamp"`
	Error      string    `json:"error,omitempty"`
	RetryCount int       `json:"retry_count,omitempty"`
}

// DLPValidationRequest represents a request for DLP validation
type DLPValidationRequest struct {
	MessageID string          `json:"message_id"`
	Topic     string          `json:"topic"`
	Content   json.RawMessage `json:"content"`
	Metadata  map[string]any  `json:"metadata"`
}

// DLPValidationResponse represents the result of DLP validation
type DLPValidationResponse struct {
	Approved  bool     `json:"approved"`
	MessageID string   `json:"message_id"`
	Reasons   []string `json:"reasons,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// TopicPermission represents access control for topics
type TopicPermission struct {
	ReadRoles   []string `mapstructure:"read_roles" json:"read_roles"`
	WriteRoles  []string `mapstructure:"write_roles" json:"write_roles"`
	Description string   `mapstructure:"description" json:"description"`
}

// RoleDefinition represents a role with associated topic permissions
type RoleDefinition struct {
	Name        string   `mapstructure:"name" json:"name"`
	Description string   `mapstructure:"description" json:"description"`
	Topics      []string `mapstructure:"topics" json:"topics"`
}

// generateID generates a unique message ID
func generateID() string {
	return time.Now().UTC().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of the specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}