package nats

import (
	"encoding/json"
	"context"
	"time"

	"smts/pkg/types"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Publisher represents a NATS JetStream publisher
type Publisher struct {
	client *Client
	logger *zap.Logger
}

// NewPublisher creates a new NATS publisher
func NewPublisher(client *Client, logger *zap.Logger) *Publisher {
	return &Publisher{
		client: client,
		logger: logger,
	}
}

// PublishMessage publishes a message to NATS JetStream
func (p *Publisher) PublishMessage(msg *types.Message) error {
	if p.client == nil || p.client.js == nil {
		return types.NewSMTSError(types.ErrNATSPublish, "NATS client not initialized")
	}

	// Validate the message
	if err := p.validateMessage(msg); err != nil {
		return err
	}

	// Convert message to JSON
	messageData, err := json.Marshal(msg)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrNATSPublish, "Failed to marshal message to JSON")
	}

	// Create NATS message with headers
	natsMsg := &nats.Msg{
		Subject: msg.Topic,
		Data:    messageData,
		Header:  make(nats.Header),
	}

	// Add SMTS headers
	natsMsg.Header.Set("smts-message-id", msg.ID)
	natsMsg.Header.Set("smts-timestamp", msg.Timestamp.Format(time.RFC3339))
	natsMsg.Header.Set("smts-source", msg.Source)

	// Add custom headers
	for key, value := range msg.Headers {
		natsMsg.Header.Set(key, value)
	}

	// Publish the message
	ack, err := p.client.js.PublishMsg(natsMsg)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrNATSPublish, "Failed to publish message")
	}

	p.logger.Debug("Message published successfully",
		zap.String("message_id", msg.ID),
		zap.String("topic", msg.Topic),
		zap.String("stream", ack.Stream),
		zap.Uint64("sequence", ack.Sequence))

	return nil
}

// PublishRaw publishes raw data to a topic
func (p *Publisher) PublishRaw(topic string, data []byte, headers map[string]string) error {
	if p.client == nil || p.client.js == nil {
		return types.NewSMTSError(types.ErrNATSPublish, "NATS client not initialized")
	}

	// Create a message with the raw data
	msg := &types.Message{
		ID:        GenerateID(),
		Timestamp: time.Now().UTC(),
		Topic:     topic,
		Source:    "smts-publisher",
		Headers:   headers,
		Body:      data,
	}

	return p.PublishMessage(msg)
}

// PublishJSON publishes a JSON object to a topic
func (p *Publisher) PublishJSON(topic string, data interface{}, headers map[string]string) error {
	// Marshal the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrNATSPublish, "Failed to marshal JSON data")
	}

	return p.PublishRaw(topic, jsonData, headers)
}

// validateMessage validates a message before publishing
func (p *Publisher) validateMessage(msg *types.Message) error {
	if msg == nil {
		return types.NewSMTSError(types.ErrMessageValidation, "Message cannot be nil")
	}

	if msg.Topic == "" {
		return types.NewSMTSError(types.ErrMessageValidation, "Message topic is required")
	}

	if msg.ID == "" {
		return types.NewSMTSError(types.ErrMessageValidation, "Message ID is required")
	}

	if msg.Timestamp.IsZero() {
		return types.NewSMTSError(types.ErrMessageValidation, "Message timestamp is required")
	}

	return nil
}

// GetStreamStats returns statistics for a stream
func (p *Publisher) GetStreamStats(streamName string) (*nats.StreamState, error) {
	if p.client == nil || p.client.js == nil {
		return nil, types.NewSMTSError(types.ErrNATSStream, "NATS client not initialized")
	}

	info, err := p.client.js.StreamInfo(streamName)
	if err != nil {
		return nil, types.WrapSMTSError(err, types.ErrNATSStream, "Failed to get stream info")
	}

	return &info.State, nil
}

// CreateStream creates or updates a stream
func (p *Publisher) CreateStream(streamConfig *types.StreamConfig) error {
	return p.client.CreateStream(streamConfig)
}

// DeleteStream deletes a stream
func (p *Publisher) DeleteStream(streamName string) error {
	return p.client.DeleteStream(streamName)
}

// HealthCheck performs a health check on the publisher
func (p *Publisher) HealthCheck() error {
	if p.client == nil {
		return types.NewSMTSError(types.ErrHealthCheck, "NATS client not initialized")
	}

	return p.client.HealthCheck(context.Background())
}

// GenerateID is a helper function to generate message IDs
func GenerateID() string {
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