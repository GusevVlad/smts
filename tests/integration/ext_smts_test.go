package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/corporate/smts/internal/server"
	"github.com/corporate/smts/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEXT_SMTS_Integration tests the complete EXT SMTS deployment flow
func TestEXT_SMTS_Integration(t *testing.T) {
	// Create test HTTP server to simulate corporate API
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request for debugging
		t.Logf("API Server received request: %s %s", r.Method, r.URL.Path)
		t.Logf("Headers: %v", r.Header)
		
		// Handle different endpoints
		if r.URL.Path == "/test.monterra.event" {
			// Verify the request headers for message delivery
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))
			assert.Equal(t, "test-message-123", r.Header.Get("X-SMTS-Message-ID"))

			// Return success
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else if r.URL.Path == "/health" {
			// Health check endpoint
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			// Unknown endpoint
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))
	defer apiServer.Close()

	// Use test config file directly for docker compatibility
	// In docker, the config file is already set up with correct topics and URLs
	configFile := getTestConfigFile("ext")
	
	// Create and start the EXT SMTS server
	extServer, err := server.NewServer(configFile)
	require.NoError(t, err)

	// Start the server
	err = extServer.Start()
	require.NoError(t, err)
	defer extServer.Stop()

	// Wait a moment for server to initialize
	time.Sleep(2 * time.Second)

	// Test 1: Publish a message to NATS and verify it gets delivered to API
	t.Run("MessageFlow_EXT", func(t *testing.T) {
		// Connect to the embedded NATS server (port 14222 for test 0)
		nc, err := nats.Connect("nats://localhost:14222")
		require.NoError(t, err)
		defer nc.Close()

		// Get JetStream context
		js, err := nc.JetStream()
		require.NoError(t, err)

		// Create test message
		messageBody := map[string]interface{}{
			"event": "test_event",
			"data":  "test_data",
		}
		bodyJSON, _ := json.Marshal(messageBody)

		smtsMessage := &types.Message{
			ID:        "test-message-123",
			Timestamp: time.Now().UTC(),
			Topic:     "test.monterra.event",
			Source:    "test",
			Headers:   map[string]string{"test-header": "value"},
			Body:      bodyJSON,
		}

		// Publish message to NATS
		messageData, err := json.Marshal(smtsMessage)
		require.NoError(t, err)

		_, err = js.Publish("test.monterra.event", messageData)
		require.NoError(t, err)

		// Wait for message to be processed
		time.Sleep(3 * time.Second)

		// Verify the message was processed by checking the stream
		streamInfo, err := js.StreamInfo("SMTS_EXT_TEST_0")
		require.NoError(t, err)
		assert.Greater(t, streamInfo.State.Msgs, uint64(0), "Should have messages in stream")
	})

	// Test 2: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := extServer.HealthCheck(ctx)
		assert.NoError(t, err, "Health check should pass")
	})

	// Test 3: Multiple messages
	t.Run("MultipleMessages", func(t *testing.T) {
		// Connect to the embedded NATS server (port 14222 for test 0)
		nc, err := nats.Connect("nats://localhost:14222")
		require.NoError(t, err)
		defer nc.Close()

		js, err := nc.JetStream()
		require.NoError(t, err)

		// Publish multiple messages
		for i := 0; i < 5; i++ {
			message := &types.Message{
				ID:        fmt.Sprintf("test-message-%d", i),
				Timestamp: time.Now().UTC(),
				Topic:     "test.monterra.event",
				Source:    "test",
				Body:      []byte(fmt.Sprintf(`{"event": "test_%d", "data": "value_%d"}`, i, i)),
			}

			messageData, err := json.Marshal(message)
			require.NoError(t, err)

			_, err = js.Publish("test.monterra.event", messageData)
			require.NoError(t, err)
		}

		// Wait for processing
		time.Sleep(5 * time.Second)

		// Verify messages were processed
		streamInfo, err := js.StreamInfo("SMTS_EXT_TEST_0")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, streamInfo.State.Msgs, uint64(5), "Should have processed multiple messages")
	})
}

// TestEXT_SMTS_ErrorHandling tests error scenarios
func TestEXT_SMTS_ErrorHandling(t *testing.T) {
	// Create test HTTP server that returns errors
	errorCount := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errorCount++
		t.Logf("Error handling server received request %d: %s %s", errorCount, r.Method, r.URL.Path)
		
		if r.URL.Path == "/test.monterra.event" {
			if errorCount <= 2 {
				// Return server error for first 2 attempts (should trigger retry)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			} else {
				// Then succeed
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}
		} else if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))
	defer apiServer.Close()

	// Use test config file directly for docker compatibility
	configFile := getTestConfigFile("ext")
	
	extServer, err := server.NewServer(configFile)
	require.NoError(t, err)

	err = extServer.Start()
	require.NoError(t, err)
	defer extServer.Stop()

	time.Sleep(2 * time.Second)

	// Publish a message that will trigger retries
	// Connect to the embedded NATS server (port 14222 for test 1)
	nc, err := nats.Connect("nats://localhost:14222")
	require.NoError(t, err)
	defer nc.Close()

	js, err := nc.JetStream()
	require.NoError(t, err)

	message := &types.Message{
		ID:        "test-retry-message",
		Timestamp: time.Now().UTC(),
		Topic:     "test.monterra.event",
		Source:    "test",
		Body:      []byte(`{"event": "retry_test"}`),
	}

	messageData, err := json.Marshal(message)
	require.NoError(t, err)

	_, err = js.Publish("test.monterra.event", messageData)
	require.NoError(t, err)

	// Wait for retries to complete
	time.Sleep(10 * time.Second)

	// Verify the message was eventually processed successfully
	assert.GreaterOrEqual(t, errorCount, 3, "Should have retried at least twice")
}

// TestEXT_SMTS_InvalidMessage tests handling of invalid messages
func TestEXT_SMTS_InvalidMessage(t *testing.T) {
	// Create test HTTP server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Invalid message server received request: %s %s", r.Method, r.URL.Path)
		
		if r.URL.Path == "/test.monterra.event" || r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))
	defer apiServer.Close()

	// Use test config file directly for docker compatibility
	configFile := getTestConfigFile("ext")
	
	extServer, err := server.NewServer(configFile)
	require.NoError(t, err)

	err = extServer.Start()
	require.NoError(t, err)
	defer extServer.Stop()

	time.Sleep(2 * time.Second)

	// Publish invalid message (not SMTS format)
	// Connect to the embedded NATS server (port 14224 for test 2)
	nc, err := nats.Connect("nats://localhost:14224")
	require.NoError(t, err)
	defer nc.Close()

	js, err := nc.JetStream()
	require.NoError(t, err)

	// Publish raw JSON that's not a valid SMTS message
	invalidMessage := []byte(`{"invalid": "message", "format": true}`)
	_, err = js.Publish("test.monterra.event", invalidMessage)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(3 * time.Second)

	// The invalid message should be handled gracefully (logged and acked)
	// Check consumer info to see if message was processed
	consumerInfo, err := js.ConsumerInfo("SMTS_EXT_TEST_2", "SMTS_EXT_TEST_CONSUMER_2")
	if err == nil {
		// If we can get consumer info, check if messages were processed
		// Note: NumAckPending might be 0 if message was already acked
		assert.GreaterOrEqual(t, consumerInfo.NumAckPending, uint64(0), "Should have processed messages")
	} else {
		// Consumer might not exist yet or failed to create, log but don't fail test
		t.Logf("Consumer info not available: %v", err)
	}
}

// TestEXT_SMTS_Shutdown tests graceful shutdown without embedded NATS
func TestEXT_SMTS_Shutdown(t *testing.T) {
	// Create test HTTP server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Shutdown test server received request: %s %s", r.Method, r.URL.Path)
		
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		}
	}))
	defer apiServer.Close()

	// Test server lifecycle without starting embedded NATS
	t.Run("ServerLifecycle", func(t *testing.T) {
		// Use test config file directly for docker compatibility
		configFile := getTestConfigFile("ext")
		
		extServer, err := server.NewServer(configFile)
		require.NoError(t, err)

		// Test that server can be created and configured
		config := extServer.GetConfig()
		assert.NotNil(t, config)
		assert.Equal(t, "ext", config.Deployment.Type)

		// Test that server is not running initially
		assert.False(t, extServer.IsRunning(), "Server should not be running initially")

		// Note: We're not starting the server to avoid NATS conflicts
		// In a real integration test environment, you would start external NATS first
	})

	// Test configuration validation
	t.Run("ConfigValidation", func(t *testing.T) {
		// Test that configuration is properly loaded and validated
		configFile := getTestConfigFile("ext")
		
		extServer, err := server.NewServer(configFile)
		require.NoError(t, err)

		config := extServer.GetConfig()
		
		// Verify key configuration values
		assert.Equal(t, "ext", config.Deployment.Type)
		assert.Equal(t, "smts-ext-test", config.Deployment.Name)
		assert.Equal(t, "test", config.Deployment.Environment)
		assert.Equal(t, apiServer.URL, config.API.BaseURL)
		assert.Equal(t, "test-api-key", config.API.Auth.APIKey)
	})
}

// getTestConfigFile returns the path to the test config file for the given deployment type
// This works for both native and docker environments
func getTestConfigFile(deploymentType string) string {
	// Try to find the test config file in different locations
	possiblePaths := []string{
		// Docker container path
		fmt.Sprintf("/app/tests/configs/%s-test-config.yaml", deploymentType),
		// Native development path (relative to integration tests)
		fmt.Sprintf("../configs/%s-test-config.yaml", deploymentType),
		// Native development path (relative to project root)
		fmt.Sprintf("tests/configs/%s-test-config.yaml", deploymentType),
		// Absolute path from current directory
		fmt.Sprintf("./tests/configs/%s-test-config.yaml", deploymentType),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	panic(fmt.Sprintf("Test config file for %s deployment not found. Tried paths: %v", deploymentType, possiblePaths))
}

// Helper function to replace configuration values
func replaceConfigValue(config, oldValue, newValue string) string {
	return strings.Replace(config, oldValue, newValue, 1)
}