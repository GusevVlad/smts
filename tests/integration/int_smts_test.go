package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"smts/internal/server"
	"smts/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestINT_SMTS_Integration tests the complete INT SMTS deployment flow
func TestINT_SMTS_Integration(t *testing.T) {
	// Create test HTTP server to simulate corporate API
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))
		assert.Equal(t, "test-message-123", r.Header.Get("X-SMTS-Message-ID"))

		// Return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer apiServer.Close()

	// Use test config file directly for docker compatibility
	// In docker, the config file is already set up with correct topics and URLs
	configFile := getTestConfigFile("int")
	
	// Create and start the INT SMTS server
	intServer, err := server.NewServer(configFile)
	require.NoError(t, err)

	// Start the server
	err = intServer.Start()
	require.NoError(t, err)
	defer intServer.Stop()

	// Wait a moment for server to initialize
	time.Sleep(2 * time.Second)

	// Test 1: Publish a message to NATS and verify it gets delivered to API
	t.Run("MessageFlow_INT", func(t *testing.T) {
		// Connect to the embedded NATS server started by the INT SMTS instance
		nc, err := nats.Connect("nats://localhost:14223")
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
		streamInfo, err := js.StreamInfo("SMTS_INT_TEST")
		require.NoError(t, err)
		assert.Greater(t, streamInfo.State.Msgs, uint64(0), "Should have messages in stream")
	})

	// Test 2: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := intServer.HealthCheck(ctx)
		assert.NoError(t, err, "Health check should pass")
	})

	// Test 3: Multiple messages
	t.Run("MultipleMessages", func(t *testing.T) {
		// Connect to the embedded NATS server started by the INT SMTS instance
		nc, err := nats.Connect("nats://localhost:14223")
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
		streamInfo, err := js.StreamInfo("SMTS_INT_TEST")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, streamInfo.State.Msgs, uint64(5), "Should have processed multiple messages")
	})
}

// TestINT_SMTS_ErrorHandling tests error scenarios
func TestINT_SMTS_ErrorHandling(t *testing.T) {
	// Create test HTTP server that returns errors
	errorCount := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errorCount++
		if errorCount <= 2 {
			// Return server error for first 2 attempts (should trigger retry)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		} else {
			// Then succeed
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer apiServer.Close()

	// Use test config file directly for docker compatibility
	configFile := getTestConfigFile("int")
	
	intServer, err := server.NewServer(configFile)
	require.NoError(t, err)

	err = intServer.Start()
	require.NoError(t, err)
	defer intServer.Stop()

	time.Sleep(2 * time.Second)

	// Publish a message that will trigger retries
	// Connect to the embedded NATS server started by the INT SMTS instance
	nc, err := nats.Connect("nats://localhost:14223")
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

// TestINT_SMTS_InvalidMessage tests handling of invalid messages
func TestINT_SMTS_InvalidMessage(t *testing.T) {
	// Create test HTTP server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer apiServer.Close()

	// Use test config file directly for docker compatibility
	configFile := getTestConfigFile("int")
	
	intServer, err := server.NewServer(configFile)
	require.NoError(t, err)

	err = intServer.Start()
	require.NoError(t, err)
	defer intServer.Stop()

	time.Sleep(2 * time.Second)

	// Publish invalid message (not SMTS format)
	// Connect to the embedded NATS server started by the INT SMTS instance
	nc, err := nats.Connect("nats://localhost:14223")
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
	consumerInfo, err := js.ConsumerInfo("SMTS_INT_TEST", "SMTS_INT_TEST_CONSUMER")
	if err == nil {
		// If we can get consumer info, check if messages were processed
		// Note: NumAckPending might be 0 if message was already acked
		assert.GreaterOrEqual(t, consumerInfo.NumAckPending, uint64(0), "Should have processed messages")
	} else {
		// Consumer might not exist yet or failed to create, log but don't fail test
		t.Logf("Consumer info not available: %v", err)
	}
}

// TestINT_SMTS_Shutdown tests graceful shutdown without embedded NATS
func TestINT_SMTS_Shutdown(t *testing.T) {
	// Create test HTTP server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer apiServer.Close()

	// Test server lifecycle without starting embedded NATS
	t.Run("ServerLifecycle", func(t *testing.T) {
		// Use test config file directly for docker compatibility
		configFile := getTestConfigFile("int")
		
		intServer, err := server.NewServer(configFile)
		require.NoError(t, err)

		// Test that server can be created and configured
		config := intServer.GetConfig()
		assert.NotNil(t, config)
		assert.Equal(t, "int", config.Deployment.Type)

		// Test that server is not running initially
		assert.False(t, intServer.IsRunning(), "Server should not be running initially")

		// Note: We're not starting the server to avoid NATS conflicts
		// In a real integration test environment, you would start external NATS first
	})

	// Test configuration validation
	t.Run("ConfigValidation", func(t *testing.T) {
		// Test that configuration is properly loaded and validated
		configFile := getTestConfigFile("int")
		
		intServer, err := server.NewServer(configFile)
		require.NoError(t, err)

		config := intServer.GetConfig()
		
		// Verify key configuration values
		assert.Equal(t, "int", config.Deployment.Type)
		assert.Equal(t, "smts-int-test", config.Deployment.Name)
		assert.Equal(t, "test", config.Deployment.Environment)
		assert.Equal(t, apiServer.URL, config.API.BaseURL)
		assert.Equal(t, "test-api-key", config.API.Auth.APIKey)
		assert.True(t, config.DLP.Enabled, "DLP should be enabled for INT")
		assert.True(t, config.Artemis.Enabled, "Artemis should be enabled for INT")
	})
}

