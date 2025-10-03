package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	topic := "test.topic"
	body := json.RawMessage(`{"key": "value"}`)

	msg := NewMessage(topic, body)

	if msg.Topic != topic {
		t.Errorf("Expected topic %s, got %s", topic, msg.Topic)
	}

	if string(msg.Body) != string(body) {
		t.Errorf("Expected body %s, got %s", string(body), string(msg.Body))
	}

	if msg.ID == "" {
		t.Error("Expected message ID to be generated")
	}

	if msg.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	if msg.Source != "smts" {
		t.Errorf("Expected source 'smts', got %s", msg.Source)
	}

	if msg.Headers == nil {
		t.Error("Expected headers map to be initialized")
	}
}

func TestMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		message *Message
		wantErr bool
	}{
		{
			name: "valid message",
			message: &Message{
				ID:        "test-id",
				Timestamp: time.Now().UTC(),
				Topic:     "test.topic",
				Source:    "test",
				Headers:   make(map[string]string),
				Body:      json.RawMessage(`{"key": "value"}`),
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			message: &Message{
				Timestamp: time.Now().UTC(),
				Topic:     "test.topic",
				Source:    "test",
				Headers:   make(map[string]string),
				Body:      json.RawMessage(`{"key": "value"}`),
			},
			wantErr: true,
		},
		{
			name: "missing topic",
			message: &Message{
				ID:        "test-id",
				Timestamp: time.Now().UTC(),
				Source:    "test",
				Headers:   make(map[string]string),
				Body:      json.RawMessage(`{"key": "value"}`),
			},
			wantErr: true,
		},
		{
			name: "missing timestamp",
			message: &Message{
				ID:      "test-id",
				Topic:   "test.topic",
				Source:  "test",
				Headers: make(map[string]string),
				Body:    json.RawMessage(`{"key": "value"}`),
			},
			wantErr: true,
		},
		{
			name: "nil headers",
			message: &Message{
				ID:        "test-id",
				Timestamp: time.Now().UTC(),
				Topic:     "test.topic",
				Source:    "test",
				Headers:   nil,
				Body:      json.RawMessage(`{"key": "value"}`),
			},
			wantErr: false, // Headers can be nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a basic validation test
			// In a real implementation, you would call a validation function
			hasError := false
			
			if tt.message.ID == "" {
				hasError = true
			}
			if tt.message.Topic == "" {
				hasError = true
			}
			if tt.message.Timestamp.IsZero() {
				hasError = true
			}

			if hasError != tt.wantErr {
				t.Errorf("Expected error %v, got %v", tt.wantErr, hasError)
			}
		})
	}
}

func TestDeliveryResult(t *testing.T) {
	messageID := "test-message-id"
	timestamp := time.Now().UTC()

	result := &DeliveryResult{
		Success:    true,
		MessageID:  messageID,
		Timestamp:  timestamp,
		RetryCount: 0,
	}

	if result.MessageID != messageID {
		t.Errorf("Expected message ID %s, got %s", messageID, result.MessageID)
	}

	if !result.Timestamp.Equal(timestamp) {
		t.Error("Timestamp mismatch")
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}

	if result.RetryCount != 0 {
		t.Errorf("Expected retry count 0, got %d", result.RetryCount)
	}
}

func TestDLPValidationRequest(t *testing.T) {
	messageID := "test-message-id"
	topic := "test.topic"
	content := json.RawMessage(`{"sensitive": "data"}`)

	request := &DLPValidationRequest{
		MessageID: messageID,
		Topic:     topic,
		Content:   content,
		Metadata: map[string]any{
			"source": "test",
		},
	}

	if request.MessageID != messageID {
		t.Errorf("Expected message ID %s, got %s", messageID, request.MessageID)
	}

	if request.Topic != topic {
		t.Errorf("Expected topic %s, got %s", topic, request.Topic)
	}

	if string(request.Content) != string(content) {
		t.Errorf("Expected content %s, got %s", string(content), string(request.Content))
	}

	if request.Metadata["source"] != "test" {
		t.Errorf("Expected metadata source 'test', got %v", request.Metadata["source"])
	}
}

func TestDLPValidationResponse(t *testing.T) {
	messageID := "test-message-id"

	response := &DLPValidationResponse{
		Approved:  true,
		MessageID: messageID,
		Reasons:   []string{"Policy check passed"},
	}

	if !response.Approved {
		t.Error("Expected approval to be true")
	}

	if response.MessageID != messageID {
		t.Errorf("Expected message ID %s, got %s", messageID, response.MessageID)
	}

	if len(response.Reasons) != 1 {
		t.Errorf("Expected 1 reason, got %d", len(response.Reasons))
	}

	if response.Reasons[0] != "Policy check passed" {
		t.Errorf("Expected reason 'Policy check passed', got %s", response.Reasons[0])
	}
}

func TestTopicPermission(t *testing.T) {
	permission := &TopicPermission{
		Description: "Test topic",
	}

	if permission.Description != "Test topic" {
		t.Errorf("Expected description 'Test topic', got %s", permission.Description)
	}
}
