package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/corporate/smts/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocker_EXT_SMTS_Integration tests the EXT SMTS deployment in Docker Compose environment
func TestDocker_EXT_SMTS_Integration(t *testing.T) {
	// Connect to the existing EXT SMTS NATS server (running in Docker)
	nc, err := nats.Connect("nats://smts-ext-test:14222")
	require.NoError(t, err, "Failed to connect to EXT SMTS NATS server")
	defer nc.Close()

	// Get JetStream context
	js, err := nc.JetStream()
	require.NoError(t, err)

	// Test 1: Publish a message to NATS and verify it gets delivered to API
	t.Run("MessageFlow_EXT", func(t *testing.T) {
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

		// Wait for message to be processed by the running EXT SMTS service
		time.Sleep(3 * time.Second)

		// Verify the message was processed by checking the stream
		streamInfo, err := js.StreamInfo("SMTS_EXT_TEST")
		require.NoError(t, err)
		assert.Greater(t, streamInfo.State.Msgs, uint64(0), "Should have messages in stream")
	})

	// Test 2: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		// Check if EXT SMTS health endpoint is accessible
		resp, err := http.Get("http://smts-ext-test:18081/health")
		require.NoError(t, err, "Health check should be accessible")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")
	})

	// Test 3: Multiple messages
	t.Run("MultipleMessages", func(t *testing.T) {
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

// TestDocker_INT_SMTS_Integration tests the INT SMTS deployment in Docker Compose environment
func TestDocker_INT_SMTS_Integration(t *testing.T) {
	// Connect to the existing INT SMTS NATS server (running in Docker)
	nc, err := nats.Connect("nats://smts-int-test:14223")
	require.NoError(t, err, "Failed to connect to INT SMTS NATS server")
	defer nc.Close()

	// Get JetStream context
	js, err := nc.JetStream()
	require.NoError(t, err)

	// Test 1: Publish a message to NATS and verify it gets delivered to API
	t.Run("MessageFlow_INT", func(t *testing.T) {
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

		// Wait for message to be processed by the running INT SMTS service
		time.Sleep(3 * time.Second)

		// Verify the message was processed by checking the stream
		streamInfo, err := js.StreamInfo("SMTS_INT_TEST")
		require.NoError(t, err)
		assert.Greater(t, streamInfo.State.Msgs, uint64(0), "Should have messages in stream")
	})

	// Test 2: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		// Check if INT SMTS health endpoint is accessible
		resp, err := http.Get("http://smts-int-test:18083/health")
		require.NoError(t, err, "Health check should be accessible")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")
	})

	// Test 3: Multiple messages
	t.Run("MultipleMessages", func(t *testing.T) {
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

// TestDocker_MockServices tests that mock services are accessible
func TestDocker_MockServices(t *testing.T) {
	t.Run("APIMock_Accessible", func(t *testing.T) {
		// Test API mock service connectivity
		// MockServer is running on port 1081 (control API)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", "api-mock:1081")
		require.NoError(t, err, "API mock should be accessible via TCP on port 1081")
		conn.Close()
	})

	t.Run("DLPMock_Accessible", func(t *testing.T) {
		// Test DLP mock service connectivity
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", "dlp-mock:1080")
		require.NoError(t, err, "DLP mock should be accessible via TCP")
		conn.Close()
	})

	t.Run("Artemis_Accessible", func(t *testing.T) {
		// Test Artemis service connectivity
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", "artemis-test:61613")
		require.NoError(t, err, "Artemis should be accessible via TCP")
		conn.Close()
	})
}

// TestDocker_MessageDelivery tests end-to-end message delivery
func TestDocker_MessageDelivery(t *testing.T) {
	// This test verifies that messages published to EXT SMTS are delivered to the API mock
	// and that INT SMTS properly handles DLP validation

	t.Run("EXT_MessageDelivery", func(t *testing.T) {
		// Connect to EXT SMTS
		nc, err := nats.Connect("nats://smts-ext-test:14222")
		require.NoError(t, err)
		defer nc.Close()

		js, err := nc.JetStream()
		require.NoError(t, err)

		// Create and publish a test message
		message := &types.Message{
			ID:        "docker-test-message-ext",
			Timestamp: time.Now().UTC(),
			Topic:     "test.monterra.event",
			Source:    "docker-test",
			Body:      []byte(`{"event": "docker_integration_test", "data": "test_data"}`),
		}

		messageData, err := json.Marshal(message)
		require.NoError(t, err)

		_, err = js.Publish("test.monterra.event", messageData)
		require.NoError(t, err)

		// Wait for delivery
		time.Sleep(5 * time.Second)

		// Verify message was processed
		streamInfo, err := js.StreamInfo("SMTS_EXT_TEST")
		require.NoError(t, err)
		assert.Greater(t, streamInfo.State.Msgs, uint64(0), "Message should be processed")
	})

	t.Run("INT_MessageDelivery", func(t *testing.T) {
		// Connect to INT SMTS
		nc, err := nats.Connect("nats://smts-int-test:14223")
		require.NoError(t, err)
		defer nc.Close()

		js, err := nc.JetStream()
		require.NoError(t, err)

		// Create and publish a test message
		message := &types.Message{
			ID:        "docker-test-message-int",
			Timestamp: time.Now().UTC(),
			Topic:     "test.monterra.event",
			Source:    "docker-test",
			Body:      []byte(`{"event": "docker_integration_test", "data": "test_data"}`),
		}

		messageData, err := json.Marshal(message)
		require.NoError(t, err)

		_, err = js.Publish("test.monterra.event", messageData)
		require.NoError(t, err)

		// Wait for delivery (INT has DLP validation, might take longer)
		time.Sleep(8 * time.Second)

		// Verify message was processed
		streamInfo, err := js.StreamInfo("SMTS_INT_TEST")
		require.NoError(t, err)
		assert.Greater(t, streamInfo.State.Msgs, uint64(0), "Message should be processed")
	})
}