package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"smts/internal/nats"
	"smts/pkg/types"

	natsio "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// MessageRetriever provides shared functionality for retrieving messages from NATS streams
type MessageRetriever struct {
	logger *zap.Logger
}

// NewMessageRetriever creates a new message retriever
func NewMessageRetriever(logger *zap.Logger) *MessageRetriever {
	return &MessageRetriever{
		logger: logger,
	}
}

// RetrieveMessages retrieves messages from NATS stream with common logic
func (mr *MessageRetriever) RetrieveMessages(
	natsClient *nats.Client,
	config *types.Config,
	topic string,
	count int,
	clientReceiver string,
) ([]map[string]interface{}, int, error) {
	mr.logger.Info("Retrieving messages from NATS stream",
		zap.String("topic", topic),
		zap.Int("requested_count", count),
		zap.String("client_receiver", clientReceiver))

	// Check if NATS client is available
	if natsClient == nil {
		return nil, 0, fmt.Errorf("NATS client not available")
	}

	js := natsClient.GetJetStream()
	if js == nil {
		return nil, 0, fmt.Errorf("JetStream context not available")
	}

	// For workqueue streams, we need to use the existing consumer
	// Use the external consumer for reading incoming INT messages
	consumerName := config.NATS.ExternalConsumer.DurableName
	streamName := config.NATS.ExternalStream.Name

	mr.logger.Info("Accessing NATS stream for message retrieval",
		zap.String("topic", topic),
		zap.String("stream", streamName),
		zap.String("consumer", consumerName))

	// Check if the consumer exists
	_, err := js.ConsumerInfo(streamName, consumerName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get consumer info: %w", err)
	}

	// For external stream, use the external topic pattern
	externalTopic := "external." + topic
	sub, err := js.PullSubscribe(externalTopic, consumerName, natsio.Bind(streamName, consumerName))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to subscribe to messages: %w", err)
	}
	defer sub.Unsubscribe()

	mr.logger.Info("Fetching messages from NATS stream",
		zap.String("topic", topic),
		zap.String("external_topic", externalTopic),
		zap.Int("requested_count", count))

	// Fetch messages
	fetchedMsgs, err := sub.Fetch(count, natsio.MaxWait(5*time.Second))
	if err != nil && err != natsio.ErrTimeout {
		return nil, 0, fmt.Errorf("failed to fetch messages: %w", err)
	}

	mr.logger.Info("Successfully fetched messages from NATS stream",
		zap.String("topic", topic),
		zap.Int("requested_count", count),
		zap.Int("fetched_count", len(fetchedMsgs)))

	// Process fetched messages
	messages := make([]map[string]interface{}, 0)
	received := 0

	for _, msg := range fetchedMsgs {
		received++

		// Parse message headers
		headers := make(map[string]string)
		if msg.Header != nil {
			for key, values := range msg.Header {
				if len(values) > 0 {
					headers[key] = values[0]
				}
			}
		}

		// Create message response - use original topic (remove "external." prefix)
		originalTopic := topic
		if strings.HasPrefix(topic, "external.") {
			originalTopic = strings.TrimPrefix(topic, "external.")
		}

		// Extract actual content from the message structure
		var msgData map[string]interface{}
		var actualBody interface{}
		if err := json.Unmarshal(msg.Data, &msgData); err == nil {
			// If we can parse as a map, extract the body field
			if bodyField, exists := msgData["body"]; exists {
				actualBody = bodyField
			} else {
				// If no body field, use the raw data
				actualBody = string(msg.Data)
			}
		} else {
			// If parsing fails, use the raw data
			actualBody = string(msg.Data)
		}

		messageData := map[string]interface{}{
			"id":        headers["X-SMTS-Message-ID"],
			"topic":     originalTopic,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"headers":   headers,
			"body":      actualBody,
		}

		messages = append(messages, messageData)

		// Acknowledge the message
		if err := msg.Ack(); err != nil {
			mr.logger.Warn("Failed to acknowledge message",
				zap.String("message_id", headers["X-SMTS-Message-ID"]),
				zap.String("topic", topic),
				zap.Error(err))
		}
	}

	mr.logger.Info("Message retrieval completed successfully",
		zap.String("topic", topic),
		zap.Int("requested", count),
		zap.Int("received", received),
		zap.String("client_receiver", clientReceiver))

	return messages, received, nil
}

// RetrieveMessagesWithComplexProcessing retrieves messages with simplified header processing (for receiveHandler)
func (mr *MessageRetriever) RetrieveMessagesWithComplexProcessing(
	natsClient *nats.Client,
	config *types.Config,
	topic string,
	count int,
	clientReceiver string,
) ([]map[string]interface{}, int, error) {
	mr.logger.Info("Retrieving messages with simplified processing",
		zap.String("topic", topic),
		zap.Int("requested_count", count),
		zap.String("client_receiver", clientReceiver))

	// Check if NATS client is available
	if natsClient == nil {
		return nil, 0, fmt.Errorf("NATS client not available")
	}

	js := natsClient.GetJetStream()
	if js == nil {
		return nil, 0, fmt.Errorf("JetStream context not available")
	}

	// Use the external consumer for reading messages
	consumerName := config.NATS.ExternalConsumer.DurableName
	streamName := config.NATS.ExternalStream.Name
	externalTopic := "external." + topic

	// Check if the consumer exists
	_, err := js.ConsumerInfo(streamName, consumerName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get consumer info: %w", err)
	}

	// Subscribe to messages
	sub, err := js.PullSubscribe(externalTopic, consumerName, natsio.Bind(streamName, consumerName))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to subscribe to messages: %w", err)
	}
	defer sub.Unsubscribe()

	// Fetch messages
	fetchedMsgs, err := sub.Fetch(count, natsio.MaxWait(5*time.Second))
	if err != nil && err != natsio.ErrTimeout {
		return nil, 0, fmt.Errorf("failed to fetch messages: %w", err)
	}

	// Process fetched messages with simplified header extraction
	messages := make([]map[string]interface{}, 0)
	received := 0

	for _, msg := range fetchedMsgs {
		received++

		// Extract body and headers using simplified logic
		actualBody, cleanHeaders := mr.extractBodyAndHeaders(msg)

		messageData := map[string]interface{}{
			"body":    actualBody,
			"headers": cleanHeaders,
			"topic":   topic,
		}

		messages = append(messages, messageData)

		// Acknowledge the message
		if err := msg.Ack(); err != nil {
			mr.logger.Warn("Failed to acknowledge message",
				zap.String("message_id", fmt.Sprintf("%v", cleanHeaders["X-SMTS-Message-ID"])),
				zap.String("topic", topic),
				zap.Error(err))
		}
	}

	mr.logger.Info("Simplified message retrieval completed successfully",
		zap.String("topic", topic),
		zap.Int("requested", count),
		zap.Int("received", received),
		zap.String("client_receiver", clientReceiver))

	return messages, received, nil
}

// extractBodyAndHeaders extracts the message body and headers using simplified logic
func (mr *MessageRetriever) extractBodyAndHeaders(msg *natsio.Msg) (interface{}, map[string]interface{}) {
	var msgData map[string]interface{}
	var actualBody interface{}
	
	// Try to parse the message data as JSON
	if err := json.Unmarshal(msg.Data, &msgData); err != nil {
		// If parsing fails, use the raw data
		return string(msg.Data), mr.extractHeadersFromNATS(msg, nil)
	}

	// Extract body using simplified logic
	actualBody = mr.extractBodyFromMessage(msgData)
	
	// Extract headers using simplified logic
	cleanHeaders := mr.extractHeadersFromMessage(msgData, msg)
	
	return actualBody, cleanHeaders
}

// extractBodyFromMessage extracts the message body using simplified logic
func (mr *MessageRetriever) extractBodyFromMessage(msgData map[string]interface{}) interface{} {
	// Check for body field (case-insensitive)
	if body, exists := msgData["body"]; exists {
		return body
	}
	if body, exists := msgData["Body"]; exists {
		return body
	}
	
	// If no body field found, return the entire message data
	return msgData
}

// extractHeadersFromMessage extracts headers from message data and NATS headers
func (mr *MessageRetriever) extractHeadersFromMessage(msgData map[string]interface{}, msg *natsio.Msg) map[string]interface{} {
	cleanHeaders := map[string]interface{}{
		"Content-Type": "application/json",
	}
	
	// Extract from message data headers
	if headers, exists := msgData["headers"]; exists {
		if headersMap, ok := headers.(map[string]interface{}); ok {
			mr.extractSMTSHeaders(headersMap, cleanHeaders)
		}
	}
	
	// Fallback to NATS headers
	mr.extractHeadersFromNATS(msg, cleanHeaders)
	
	return cleanHeaders
}

// extractSMTSHeaders extracts SMTS headers from a headers map
func (mr *MessageRetriever) extractSMTSHeaders(sourceHeaders map[string]interface{}, targetHeaders map[string]interface{}) {
	// Check for both case variants
	headerKeys := []string{"X-SMTS-Message-ID", "x-smts-message-id", "X-SMTS-Timestamp", "x-smts-timestamp"}
	
	for _, key := range headerKeys {
		if value, exists := sourceHeaders[key]; exists && value != "" {
			// Normalize to uppercase key
			normalizedKey := "X-SMTS-Message-ID"
			if strings.Contains(strings.ToLower(key), "timestamp") {
				normalizedKey = "X-SMTS-Timestamp"
			}
			targetHeaders[normalizedKey] = value
		}
	}
}

// extractHeadersFromNATS extracts headers from NATS message headers
func (mr *MessageRetriever) extractHeadersFromNATS(msg *natsio.Msg, targetHeaders map[string]interface{}) map[string]interface{} {
	if targetHeaders == nil {
		targetHeaders = map[string]interface{}{
			"Content-Type": "application/json",
		}
	}
	
	if msg.Header == nil {
		return targetHeaders
	}
	
	// Extract SMTS headers from NATS headers
	headerKeys := []string{"X-SMTS-Message-ID", "X-SMTS-Timestamp"}
	
	for _, key := range headerKeys {
		if values := msg.Header.Values(key); len(values) > 0 {
			targetHeaders[key] = values[0]
		}
	}
	
	return targetHeaders
}