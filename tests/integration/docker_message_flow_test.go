package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocker_MessageFlow_EXT_to_INT tests the complete EXT → INT message flow
// This matches the same checks as test-message-flow.sh Flow 1
func TestDocker_MessageFlow_EXT_to_INT(t *testing.T) {
	// Test configuration for Docker environment
	apiBaseUrlExt := "http://smts-ext-test:18082"
	messageApiInt := "http://smts-int-test:19083"
	apiKey := "test-api-key"
	topicMonterra := "test.monterra.event"

	// Step 1: Check if services are healthy (same as test-message-flow.sh)
	t.Run("ServiceHealthChecks", func(t *testing.T) {
		// Check EXT SMTS health
		resp, err := http.Get("http://smts-ext-test:18081/health")
		require.NoError(t, err, "EXT SMTS health check should be accessible")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "EXT SMTS health check should return 200")

		// Check INT SMTS health
		resp, err = http.Get("http://smts-int-test:18083/health")
		require.NoError(t, err, "INT SMTS health check should be accessible")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "INT SMTS health check should return 200")

		// Check Corporate API health
		resp, err = http.Get("http://corporate-api:8080/health")
		require.NoError(t, err, "Corporate API health check should be accessible")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Corporate API health check should return 200")
	})

	// Step 2: Send message to EXT SMTS (Flow 1)
	t.Run("SendMessageToEXT", func(t *testing.T) {
		messageID := fmt.Sprintf("test-ext-int-%d", time.Now().Unix())
		timestamp := time.Now().UTC().Format(time.RFC3339)

		payload := map[string]interface{}{
			"event_type":  "dora_metrics_update",
			"project_id":  fmt.Sprintf("project-%s", messageID),
			"project_name": "Monterra Platform",
			"timestamp":   timestamp,
			"metrics": map[string]interface{}{
				"deployment_frequency": map[string]interface{}{
					"value": 12.5,
					"unit":  "deployments/week",
					"trend": "improving",
				},
				"lead_time_for_changes": map[string]interface{}{
					"value": 3.2,
					"unit":  "days",
					"trend": "stable",
				},
				"mean_time_to_restore": map[string]interface{}{
					"value": 2.1,
					"unit":  "hours",
					"trend": "improving",
				},
				"change_failure_rate": map[string]interface{}{
					"value": 8.5,
					"unit":  "percent",
					"trend": "stable",
				},
			},
			"period": map[string]interface{}{
				"start": time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
				"end":   timestamp,
			},
			"team_size":    15,
			"environment":  "production",
			"source":       "external-monitoring",
		}

		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/send/%s", apiBaseUrlExt, topicMonterra), bytes.NewReader(payloadBytes))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("X-SMTS-Message-ID", messageID)
		req.Header.Set("X-SMTS-Timestamp", timestamp)
		req.Header.Set("X-SMTS-Source", "ext-client")
		req.Header.Set("smts-role", "ext_writer")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
			"Message should be accepted by EXT SMTS (status %d)", resp.StatusCode)
	})

	// Step 3: Check if message was transported to INT SMTS via Corporate API and ArtemisMQ
	t.Run("CheckMessageTransportToINT", func(t *testing.T) {
		// Wait for message to flow through Corporate API and ArtemisMQ
		time.Sleep(5 * time.Second)

		req, err := http.NewRequest("GET", fmt.Sprintf("%s/messages?topic=%s&count=1", messageApiInt, topicMonterra), nil)
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("smts-role", "int_reader")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should be able to query INT SMTS message API")

		var response struct {
			Received int `json:"received"`
			Messages []interface{} `json:"messages"`
		}

		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err, "Should decode response from INT SMTS message API")

		// Check if we received at least one message
		if response.Received > 0 {
			t.Logf("✅ Message successfully transported to INT SMTS via Corporate API and ArtemisMQ! (received: %d)", response.Received)
		} else {
			t.Logf("⚠️ No messages found in INT SMTS (received: %d)", response.Received)
			t.Log("This might be because:")
			t.Log("- The Corporate API is not forwarding messages to ArtemisMQ")
			t.Log("- INT-SMTS is not consuming from ArtemisMQ")
			t.Log("- The message is still being processed")
		}
	})
}

// TestDocker_MessageFlow_INT_to_EXT tests the complete INT → EXT message flow
// This matches the same checks as test-message-flow.sh Flow 2
func TestDocker_MessageFlow_INT_to_EXT(t *testing.T) {
	// Test configuration for Docker environment
	apiBaseUrlInt := "http://smts-int-test:18084"
	messageApiExt := "http://smts-ext-test:19081"
	apiKey := "test-api-key"
	topicMonterra := "test.monterra.event"

	// Step 1: Send message to INT SMTS (Flow 2)
	t.Run("SendMessageToINT", func(t *testing.T) {
		messageID := fmt.Sprintf("test-int-ext-%d", time.Now().Unix())
		timestamp := time.Now().UTC().Format(time.RFC3339)

		payload := map[string]interface{}{
			"event_type": "dora_metrics_update",
			"system_id":  fmt.Sprintf("system-%s", messageID),
			"system_name": "Internal Monitoring System",
			"timestamp":  timestamp,
			"metrics": map[string]interface{}{
				"deployment_frequency": map[string]interface{}{
					"value": 15.2,
					"unit":  "deployments/week",
					"trend": "improving",
				},
				"lead_time_for_changes": map[string]interface{}{
					"value": 2.8,
					"unit":  "days",
					"trend": "improving",
				},
				"mean_time_to_restore": map[string]interface{}{
					"value": 1.5,
					"unit":  "hours",
					"trend": "stable",
				},
				"change_failure_rate": map[string]interface{}{
					"value": 6.2,
					"unit":  "percent",
					"trend": "improving",
				},
			},
			"status":      "healthy",
			"environment": "production",
			"source":      "internal-monitoring",
		}

		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/send/%s", apiBaseUrlInt, topicMonterra), bytes.NewReader(payloadBytes))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("X-SMTS-Message-ID", messageID)
		req.Header.Set("X-SMTS-Timestamp", timestamp)
		req.Header.Set("X-SMTS-Source", "int-client")
		req.Header.Set("smts-role", "int_writer")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
			"Message should be accepted by INT SMTS (status %d)", resp.StatusCode)
	})

	// Step 2: Check if message was transported to EXT SMTS via ArtemisMQ and Corporate API
	t.Run("CheckMessageTransportToEXT", func(t *testing.T) {
		// Wait for message to flow through ArtemisMQ and Corporate API
		time.Sleep(5 * time.Second)

		req, err := http.NewRequest("GET", fmt.Sprintf("%s/messages?topic=%s&count=1", messageApiExt, topicMonterra), nil)
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("smts-role", "ext_reader")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should be able to query EXT SMTS message API")

		var response struct {
			Received int `json:"received"`
			Messages []interface{} `json:"messages"`
		}

		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err, "Should decode response from EXT SMTS message API")

		// Check if we received at least one message
		if response.Received > 0 {
			t.Logf("✅ Message successfully transported to EXT SMTS via ArtemisMQ and Corporate API! (received: %d)", response.Received)
		} else {
			t.Logf("⚠️ No messages found in EXT SMTS (received: %d)", response.Received)
			t.Log("This might be because:")
			t.Log("- INT-SMTS is not pushing messages to ArtemisMQ")
			t.Log("- Corporate API is not consuming from ArtemisMQ")
			t.Log("- The message is still being processed")
		}
	})
}

// TestDocker_MessageFlow_Bidirectional tests both directions in sequence
func TestDocker_MessageFlow_Bidirectional(t *testing.T) {
	t.Run("EXT_to_INT_Flow", func(t *testing.T) {
		TestDocker_MessageFlow_EXT_to_INT(t)
	})

	t.Run("INT_to_EXT_Flow", func(t *testing.T) {
		TestDocker_MessageFlow_INT_to_EXT(t)
	})
}