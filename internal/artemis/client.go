package artemis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/corporate/smts/internal/message"
	"github.com/corporate/smts/pkg/types"
	"github.com/corporate/smts/pkg/utils"
	"github.com/go-stomp/stomp"
	"go.uber.org/zap"
)

// Client represents an ArtemisMQ client
type Client struct {
	config    *types.ArtemisConfig
	logger    *zap.Logger
	conn      *stomp.Conn
	connected bool
}

// NewClient creates a new ArtemisMQ client
func NewClient(config *types.ArtemisConfig, logger *zap.Logger) (*Client, error) {
	client := &Client{
		config: config,
		logger: logger,
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

// connect establishes connection to ArtemisMQ
func (c *Client) connect() error {
	server := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	// Connection options
	options := []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(c.config.Username, c.config.Password),
		stomp.ConnOpt.HeartBeat(10*time.Second, 10*time.Second),
		stomp.ConnOpt.AcceptVersion(stomp.V12),
	}

	// Connect to Artemis
	conn, err := stomp.Dial("tcp", server, options...)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrArtemisConnection, "Failed to connect to ArtemisMQ")
	}

	c.conn = conn
	c.connected = true

	c.logger.Info("Connected to ArtemisMQ",
		zap.String("server", server),
		zap.String("queue", c.config.Queue))

	return nil
}

// Start begins consuming messages from ArtemisMQ
func (c *Client) Start(ctx context.Context, processor *message.Processor) error {
	if !c.connected || c.conn == nil {
		return types.NewSMTSError(types.ErrArtemisConnection, "Not connected to ArtemisMQ")
	}

	// Subscribe to the queue
	sub, err := c.conn.Subscribe(c.config.Queue, stomp.AckClientIndividual)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrArtemisConsume, "Failed to subscribe to Artemis queue")
	}

	c.logger.Info("Started consuming from ArtemisMQ", zap.String("queue", c.config.Queue))

	// Start message processing loop
	go c.processMessages(ctx, sub, processor)

	return nil
}

// processMessages processes messages from ArtemisMQ subscription
func (c *Client) processMessages(ctx context.Context, sub *stomp.Subscription, processor *message.Processor) {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping ArtemisMQ message processing")
			return

		case msg := <-sub.C:
			if msg == nil {
				c.logger.Warn("Received nil message from ArtemisMQ")
				continue
			}

			c.handleMessage(ctx, msg, processor)
		}
	}
}

// handleMessage processes a single ArtemisMQ message
func (c *Client) handleMessage(ctx context.Context, msg *stomp.Message, processor *message.Processor) {
	operation := "artemis_process_message"
	startTime := time.Now()

	// Parse the message
	smtsMsg, err := c.parseMessage(msg)
	if err != nil {
		c.logger.Error("Failed to parse ArtemisMQ message",
			utils.LoggerFields(operation, "int", "", "")...,
			utils.WithError(err))

		// Reject the message
		if rejErr := c.conn.Nack(msg); rejErr != nil {
			c.logger.Error("Failed to nack invalid message", zap.Error(rejErr))
		}
		return
	}

	// Process the message using the processor
	result, err := processor.HandleMessage(ctx, smtsMsg)
	if err != nil {
		c.logger.Error("Failed to process ArtemisMQ message",
			utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic)...,
			utils.WithError(err))

		// Check if we should retry
		if types.IsRetryableError(err) {
			// Nack for retry
			if nackErr := c.conn.Nack(msg); nackErr != nil {
				c.logger.Error("Failed to nack message for retry", zap.Error(nackErr))
			}
		} else {
			// Ack non-retryable errors
			if ackErr := c.conn.Ack(msg); ackErr != nil {
				c.logger.Error("Failed to ack failed message", zap.Error(ackErr))
			}
		}
		return
	}

	// Acknowledge successful processing
	if result.Success {
		if err := c.conn.Ack(msg); err != nil {
			c.logger.Error("Failed to ack successful message",
				utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic)...,
				utils.WithError(err))
		} else {
			c.logger.Debug("ArtemisMQ message processed successfully",
				utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic)...,
				utils.WithDuration(time.Since(startTime).Milliseconds()))
		}
	} else {
		// Message processing failed but we want to ack to avoid reprocessing
		if err := c.conn.Ack(msg); err != nil {
			c.logger.Error("Failed to ack failed message",
				utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic)...,
				utils.WithError(err))
		}
		c.logger.Warn("ArtemisMQ message processing failed",
			utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic)...,
			zap.String("error", result.Error),
			utils.WithDuration(time.Since(startTime).Milliseconds()))
	}
}

// parseMessage converts an ArtemisMQ message to our SMTS message format
func (c *Client) parseMessage(msg *stomp.Message) (*types.Message, error) {
	var smtsMsg types.Message

	// Try to parse as JSON first
	if err := json.Unmarshal(msg.Body, &smtsMsg); err != nil {
		// If JSON parsing fails, create a message with raw body
		smtsMsg = types.Message{
			ID:        fmt.Sprintf("artemis-%d", time.Now().UnixNano()),
			Timestamp: time.Now().UTC(),
			Topic:     "artemis.inbound",
			Source:    "artemis",
			Headers:   make(map[string]string),
			Body:      msg.Body,
		}
	}

	// Extract headers from Artemis message
	for key, values := range msg.Header {
		if len(values) > 0 {
			smtsMsg.Headers[key] = values[0]
		}
	}

	// Ensure required fields
	if smtsMsg.ID == "" {
		smtsMsg.ID = fmt.Sprintf("artemis-%d", time.Now().UnixNano())
	}
	if smtsMsg.Topic == "" {
		smtsMsg.Topic = "artemis.inbound"
	}
	if smtsMsg.Timestamp.IsZero() {
		smtsMsg.Timestamp = time.Now().UTC()
	}
	if smtsMsg.Headers == nil {
		smtsMsg.Headers = make(map[string]string)
	}

	return &smtsMsg, nil
}

// PublishMessage publishes a message to ArtemisMQ
func (c *Client) PublishMessage(msg *types.Message) error {
	if !c.connected || c.conn == nil {
		return types.NewSMTSError(types.ErrArtemisConnection, "Not connected to ArtemisMQ")
	}

	// Convert message to JSON
	messageData, err := json.Marshal(msg)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrArtemisPublish, "Failed to marshal message to JSON")
	}

	// Create Artemis message with headers
	artemisMsg := &stomp.Message{
		Destination: c.config.Queue,
		Body:        messageData,
	}

	// Add SMTS headers
	artemisMsg.Header.Set("smts-message-id", msg.ID)
	artemisMsg.Header.Set("smts-timestamp", msg.Timestamp.Format(time.RFC3339))
	artemisMsg.Header.Set("smts-source", msg.Source)
	artemisMsg.Header.Set("smts-topic", msg.Topic)

	// Add custom headers
	for key, value := range msg.Headers {
		artemisMsg.Header.Set(key, value)
	}

	// Publish the message
	if err := c.conn.Send(artemisMsg); err != nil {
		return types.WrapSMTSError(err, types.ErrArtemisPublish, "Failed to publish message to ArtemisMQ")
	}

	c.logger.Debug("Message published to ArtemisMQ",
		zap.String("message_id", msg.ID),
		zap.String("topic", msg.Topic),
		zap.String("queue", c.config.Queue))

	return nil
}

// Stop stops the ArtemisMQ client
func (c *Client) Stop() error {
	if c.conn != nil {
		if err := c.conn.Disconnect(); err != nil {
			c.logger.Error("Error disconnecting from ArtemisMQ", zap.Error(err))
		}
		c.conn = nil
		c.connected = false
	}

	c.logger.Info("ArtemisMQ client stopped")
	return nil
}

// HealthCheck performs a health check on the ArtemisMQ connection
func (c *Client) HealthCheck(ctx context.Context) error {
	if !c.connected || c.conn == nil {
		return types.NewSMTSError(types.ErrHealthCheck, "Not connected to ArtemisMQ")
	}

	// Try to send a small test message
	testMsg := &stomp.Message{
		Destination: c.config.Queue + ".healthcheck",
		Body:        []byte("healthcheck"),
	}

	if err := c.conn.Send(testMsg); err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "ArtemisMQ health check failed")
	}

	return nil
}

// IsConnected returns true if the client is connected to ArtemisMQ
func (c *Client) IsConnected() bool {
	return c.connected && c.conn != nil
}