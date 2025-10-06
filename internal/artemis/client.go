package artemis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"smts/pkg/types"
	"smts/pkg/utils"
	"github.com/go-stomp/stomp"
	"go.uber.org/zap"
)

// MessageProcessor defines the interface for processing messages from ArtemisMQ
type MessageProcessor interface {
	HandleMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error)
}

// Client represents an ArtemisMQ client
type Client struct {
	config        *types.ArtemisConfig
	logger        *zap.Logger
	conn          *stomp.Conn
	connected     bool
	publishQueue  string // Queue for publishing messages (Flow 2)
}

// NewClient creates a new ArtemisMQ client
func NewClient(config *types.ArtemisConfig, logger *zap.Logger) (*Client, error) {
	client := &Client{
		config:       config,
		logger:       logger,
		publishQueue: config.Queue, // Default to same queue for backward compatibility
	}

	// For INT deployment, use different queue for publishing (Flow 2)
	if config.PublishQueue != "" {
		client.publishQueue = config.PublishQueue
	}

	logger.Debug("Artemis client configuration",
		zap.String("queue", config.Queue),
		zap.String("publish_queue", config.PublishQueue),
		zap.String("actual_publish_queue", client.publishQueue))

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
func (c *Client) Start(ctx context.Context, processor MessageProcessor) error {
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
func (c *Client) processMessages(ctx context.Context, sub *stomp.Subscription, processor MessageProcessor) {
	backoff := time.Millisecond * 100
	maxBackoff := time.Second * 30
	
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping ArtemisMQ message processing")
			return

		case msg := <-sub.C:
			if msg == nil {
				c.logger.Warn("Received nil message from ArtemisMQ, backing off",
					zap.Duration("backoff", backoff))
				
				// Exponential backoff with context-aware sleep
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return
				}
				
				// Increase backoff for next time, capped at max
				backoff = minDuration(backoff*2, maxBackoff)
				continue
			}
			
			// Reset backoff on successful message
			backoff = time.Millisecond * 100
			c.handleMessage(ctx, msg, processor)
		}
	}
}

// minDuration returns the minimum of two durations
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// handleMessage processes a single ArtemisMQ message
func (c *Client) handleMessage(ctx context.Context, msg *stomp.Message, processor MessageProcessor) {
	operation := "artemis_process_message"
	startTime := time.Now()

	// Parse the message
	smtsMsg, err := c.parseMessage(msg)
	if err != nil {
		c.logger.Error("Failed to parse ArtemisMQ message",
			append(utils.LoggerFields(operation, "int", "", ""),
				utils.WithError(err))...)

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
			append(utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic),
				utils.WithError(err))...)

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
				append(utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic),
					utils.WithError(err))...)
		} else {
			c.logger.Debug("ArtemisMQ message processed successfully",
				append(utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic),
					utils.WithDuration(time.Since(startTime).Milliseconds()))...)
		}
	} else {
		// Message processing failed but we want to ack to avoid reprocessing
		if err := c.conn.Ack(msg); err != nil {
			c.logger.Error("Failed to ack failed message",
				append(utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic),
					utils.WithError(err))...)
		}
		c.logger.Warn("ArtemisMQ message processing failed",
			append(utils.LoggerFields(operation, "int", smtsMsg.ID, smtsMsg.Topic),
				zap.String("error", result.Error),
				utils.WithDuration(time.Since(startTime).Milliseconds()))...)
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
	// Note: The stomp.Message.Header field structure may vary by library version
	// For now, we'll handle common headers explicitly
	if msgId := msg.Header.Get("smts-message-id"); msgId != "" {
		smtsMsg.Headers["smts-message-id"] = msgId
	}
	if timestamp := msg.Header.Get("smts-timestamp"); timestamp != "" {
		smtsMsg.Headers["smts-timestamp"] = timestamp
	}
	if source := msg.Header.Get("smts-source"); source != "" {
		smtsMsg.Headers["smts-source"] = source
	}
	if topic := msg.Header.Get("smts-topic"); topic != "" {
		smtsMsg.Headers["smts-topic"] = topic
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

	// Publish the message with headers using the same approach as corporate API
	err = c.conn.Send(
		c.publishQueue,
		"application/json",
		messageData,
		stomp.SendOpt.Header("smts-message-id", msg.ID),
		stomp.SendOpt.Header("smts-timestamp", msg.Timestamp.Format(time.RFC3339)),
		stomp.SendOpt.Header("smts-source", msg.Source),
		stomp.SendOpt.Header("smts-topic", msg.Topic),
		stomp.SendOpt.Header("persistent", "true"),
	)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrArtemisPublish, "Failed to publish message to ArtemisMQ")
	}

	c.logger.Debug("Message published to ArtemisMQ",
		zap.String("message_id", msg.ID),
		zap.String("topic", msg.Topic),
		zap.String("queue", c.publishQueue))

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

	if err := c.conn.Send(testMsg.Destination, "text/plain", testMsg.Body); err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "ArtemisMQ health check failed")
	}

	return nil
}

// IsConnected returns true if the client is connected to ArtemisMQ
func (c *Client) IsConnected() bool {
	return c.connected && c.conn != nil
}