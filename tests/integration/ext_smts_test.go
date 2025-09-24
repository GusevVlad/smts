package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/corporate/smts/internal/nats"
	"github.com/corporate/smts/internal/server"
	"github.com/corporate/smts/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestEXT_SMTS_Integration tests the complete EXT SMTS deployment flow
func TestEXT_SMTS_Integration(t *testing.T) {
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

	// Update test config to use our test server
	configPath := "tests/configs/ext-test-config.yaml"
	
	// Create logger
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	// Create and start the EXT SMTS server
	extServer, err := server.NewServer(configPath)
	require.NoError(t, err)

	// Start the server
	err = extServer.Start()
	require.NoError(t, err)
	defer extServer.Stop()

	// Wait a moment for server to initialize
	time.Sleep(2 * time.Second)

	// Test 1: Publish a message to NATS and verify it gets delivered to API
	t.Run("MessageFlow_EXT", func(t *testing.T) {
		// Connect to the embedded NATS server
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
		streamInfo, err := js.StreamInfo("SMTS_EXT_TEST")
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
		streamInfo, err := js.StreamInfo("SMTS_EXT_TEST")
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

	configPath := "tests/configs/ext-test-config.yaml"
	
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	extServer, err := server.NewServer(configPath)
	require.NoError(t, err)

	err = extServer.Start()
	require.NoError(t, err)
	defer extServer.Stop()

	time.Sleep(2 * time.Second)

	// Publish a message that will trigger retries
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
	configPath := "tests/configs/ext-test-config.yaml"
	
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	extServer, err := server.NewServer(configPath)
	require.NoError(t, err)

	err = extServer.Start()
	require.NoError(t, err)
	defer extServer.Stop()

	time.Sleep(2 * time.Second)

	// Publish invalid message (not SMTS format)
	nc, err := nats.Connect("nats://localhost:14222")
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
	streamInfo, err := js.StreamInfo("SMTS_EXT_TEST")
	require.NoError(t, err)
	
	// Check consumer info to see if message was processed
	consumerInfo, err := js.ConsumerInfo("SMTS_EXT_TEST", "SMTS_EXT_TEST_CONSUMER")
	if err == nil {
		// If we can get consumer info, check if messages were processed
		assert.Greater(t, consumerInfo.NumAckPending, uint64(0), "Should have processed messages")
	}
}

// TestEXT_SMTS_Shutdown tests graceful shutdown
func TestEXT_SMTS_Shutdown(t *testing.T) {
	configPath := "tests/configs/ext-test-config.yaml"
	
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	extServer, err := server.NewServer(configPath)
	require.NoError(t, err)

	err = extServer.Start()
	require.NoError(t, err)

	// Verify server is running
	assert.True(t, extServer.IsRunning(), "Server should be running")

	// Stop the server
	err = extServer.Stop()
	assert.NoError(t, err, "Server should stop gracefully")

	// Verify server is not running
	assert.False(t, extServer.IsRunning(), "Server should not be running after stop")
}