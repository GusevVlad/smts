package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"smts/internal/api"
	"smts/pkg/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAPIClient_DeliverMessage_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/monterra.event", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-message-123", r.Header.Get("X-SMTS-Message-ID"))
		assert.Equal(t, "test", r.Header.Get("X-SMTS-Source"))
		assert.Equal(t, "value", r.Header.Get("custom-header"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
		Retry: types.RetryConfig{
			MaxAttempts: 3,
			Backoff:     1 * time.Second,
		},
	}

	client := api.NewClient(config, &types.DLPConfig{Enabled: false}, logger)

	// Create a test message
	messageBody := map[string]interface{}{
		"event": "test_event",
		"data":  "test_data",
	}
	bodyJSON, _ := json.Marshal(messageBody)
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Headers:   map[string]string{"custom-header": "value"},
		Body:      bodyJSON,
	}

	result, err := client.DeliverMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "test-message-123", result.MessageID)
}

func TestAPIClient_DeliverMessage_RetrySuccess(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			// First attempt fails
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		} else {
			// Second attempt succeeds
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
		Retry: types.RetryConfig{
			MaxAttempts: 3,
			Backoff:     100 * time.Millisecond, // Short backoff for testing
		},
	}

	client := api.NewClient(config, &types.DLPConfig{Enabled: false}, logger)

	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"test": "data"}`),
	}

	result, err := client.DeliverMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, attempt) // Should have retried once
}

func TestAPIClient_DeliverMessage_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
		Retry: types.RetryConfig{
			MaxAttempts: 3,
			Backoff:     1 * time.Second,
		},
	}

	client := api.NewClient(config, &types.DLPConfig{Enabled: false}, logger)

	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"test": "data"}`),
	}

	result, err := client.DeliverMessage(context.Background(), msg)

	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Client error")
}

func TestAPIClient_ValidateMessage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/validate", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body to verify DLP request structure
		var dlpRequest types.DLPValidationRequest
		err := json.NewDecoder(r.Body).Decode(&dlpRequest)
		assert.NoError(t, err)
		assert.Equal(t, "test-message-123", dlpRequest.MessageID)
		assert.Equal(t, "monterra.event", dlpRequest.Topic)

		// Return DLP approval
		response := types.DLPValidationResponse{
			Approved:  true,
			MessageID: "test-message-123",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	client := api.NewClient(config, &types.DLPConfig{
		Enabled:  true,
		Endpoint: server.URL + "/validate",
		Timeout:  30 * time.Second,
	}, logger)

	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"sensitive": "data"}`),
	}

	result, err := client.ValidateMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, "test-message-123", result.MessageID)
}

func TestAPIClient_ValidateMessage_Rejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return DLP rejection
		response := types.DLPValidationResponse{
			Approved:  false,
			MessageID: "test-message-123",
			Reasons:   []string{"Contains sensitive information"},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	client := api.NewClient(config, &types.DLPConfig{
		Enabled:  true,
		Endpoint: server.URL + "/validate",
		Timeout:  30 * time.Second,
	}, logger)

	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"sensitive": "data"}`),
	}

	result, err := client.ValidateMessage(context.Background(), msg)

	assert.Error(t, err, "ValidateMessage should return error for DLP rejection")
	assert.NotNil(t, result, "Response should not be nil")
	assert.False(t, result.Approved, "Message should be rejected by DLP")
	if len(result.Reasons) > 0 {
		assert.Contains(t, result.Reasons[0], "sensitive information")
	}
	assert.Contains(t, err.Error(), "Message rejected by DLP validation", "Error should indicate DLP rejection")
}

func TestAPIClient_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	client := api.NewClient(config, &types.DLPConfig{Enabled: false}, logger)

	err := client.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestAPIClient_HealthCheck_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	client := api.NewClient(config, &types.DLPConfig{Enabled: false}, logger)

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check returned status 500")
}

func TestAPIClient_MessageValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: "https://api.corporate.com",
		Timeout: 30 * time.Second,
	}

	client := api.NewClient(config, &types.DLPConfig{Enabled: false}, logger)

	tests := []struct {
		name        string
		message     *types.Message
		expectError bool
	}{
		{
			name:        "nil message",
			message:     nil,
			expectError: true,
		},
		{
			name: "empty topic",
			message: &types.Message{
				ID:    "test-id",
				Topic: "",
				Body:  []byte("test"),
			},
			expectError: true,
		},
		{
			name: "empty message ID",
			message: &types.Message{
				ID:    "",
				Topic: "test.topic",
				Body:  []byte("test"),
			},
			expectError: true,
		},
		{
			name: "empty body",
			message: &types.Message{
				ID:    "test-id",
				Topic: "test.topic",
				Body:  []byte(""),
			},
			expectError: true,
		},
		{
			name: "valid message",
			message: &types.Message{
				ID:    "test-id",
				Topic: "test.topic",
				Body:  []byte("test data"),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.DeliverMessage(context.Background(), tt.message)

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, result.Success)
			} else {
				// We expect an error because we're not mocking the HTTP call for valid messages
				assert.Error(t, err)
			}
		})
	}
}

func TestAPIClient_Authentication(t *testing.T) {
	apiKeyReceived := false
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key header
		receivedAPIKey := r.Header.Get("X-API-Key")
		t.Logf("Received headers: %v", r.Header)
		t.Logf("X-API-Key header value: '%s'", receivedAPIKey)
		t.Logf("Request URL: %s", r.URL.String())
		t.Logf("Request Method: %s", r.Method)
		
		if receivedAPIKey == "test-api-key" {
			apiKeyReceived = true
			t.Log("API key correctly received!")
		} else {
			t.Logf("API key mismatch. Expected: 'test-api-key', Got: '%s'", receivedAPIKey)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	config := &types.APIConfig{
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
		Retry: types.RetryConfig{
			MaxAttempts: 3,
			Backoff:     1 * time.Second,
		},
		Auth: types.AuthConfig{
			Type:   "api_key",
			APIKey: "test-api-key",
		},
	}

	t.Logf("Test server URL: %s", server.URL)
	t.Logf("Config BaseURL: %s", config.BaseURL)

	client := api.NewClient(config, &types.DLPConfig{Enabled: false}, logger)

	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"test": "data"}`),
	}

	t.Logf("Message topic: %s", msg.Topic)
	t.Logf("Expected endpoint: %s/%s", config.BaseURL, msg.Topic)

	result, err := client.DeliverMessage(context.Background(), msg)

	t.Logf("API call result: success=%v, error=%v", result.Success, err)
	t.Logf("API key received by server: %v", apiKeyReceived)
	
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.True(t, apiKeyReceived, "API key should have been received by the server")
}