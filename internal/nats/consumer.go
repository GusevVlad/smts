package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/corporate/smts/pkg/types"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Consumer represents a NATS JetStream consumer
type Consumer struct {
	client    *Client
	config    *types.ConsumerConfig
	stream    string
	handler   MessageHandler
	logger    *zap.Logger
	subscription *nats.Subscription
}

// MessageHandler defines the interface for processing messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error)
}

// NewConsumer creates a new NATS consumer
func NewConsumer(client *Client, stream string, config *types.ConsumerConfig, handler MessageHandler, logger *zap.Logger) *Consumer {
	return &Consumer{
		client:  client,
		config:  config,
		stream:  stream,
		handler: handler,
		logger:  logger,
	}
}

// Start begins consuming messages from the stream
func (c *Consumer) Start(ctx context.Context) error {
	if c.client == nil || c.client.js == nil {
		return types.NewSMTSError(types.ErrNATSConsumer, "NATS client not initialized")
	}

	// Create or get the consumer
	js, err := c.getOrCreateConsumer()
	if err != nil {
		return err
	}

	// Subscribe to the consumer - use pull subscription for explicit ack policy
	sub, err := js.PullSubscribe("", c.config.DurableName,
		nats.BindStream(c.stream),
		nats.AckWait(30*time.Second),
		nats.MaxAckPending(100),
	)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrNATSConsumer, "Failed to subscribe to consumer")
	}

	c.subscription = sub
	c.logger.Info("Consumer started",
		zap.String("stream", c.stream),
		zap.String("consumer", c.config.DurableName))

	// Start the pull consumer
	go c.startPullConsumer(ctx)

	// Wait for context cancellation
	go c.waitForShutdown(ctx)

	return nil
}

// Stop stops the consumer
func (c *Consumer) Stop() error {
	if c.subscription != nil {
		if err := c.subscription.Drain(); err != nil {
			c.logger.Error("Failed to drain subscription", zap.Error(err))
		}
		c.subscription = nil
	}

	c.logger.Info("Consumer stopped", 
		zap.String("stream", c.stream),
		zap.String("consumer", c.config.DurableName))

	return nil
}

// getOrCreateConsumer gets an existing consumer or creates a new one
func (c *Consumer) getOrCreateConsumer() (nats.JetStreamContext, error) {
	js := c.client.GetJetStream()

	// Check if consumer already exists
	consumerInfo, err := js.ConsumerInfo(c.stream, c.config.DurableName)
	if err != nil && err != nats.ErrConsumerNotFound {
		return nil, types.WrapSMTSError(err, types.ErrNATSConsumer, "Failed to get consumer info")
	}

	if consumerInfo == nil {
		// Create new consumer
		consumerConfig := &nats.ConsumerConfig{
			Durable:       c.config.DurableName,
			AckPolicy:     nats.AckExplicitPolicy,
			DeliverPolicy: nats.DeliverAllPolicy,
			AckWait:       30 * time.Second,
			MaxAckPending: 100,
		}

		// Parse deliver policy
		switch c.config.DeliverPolicy {
		case "all":
			consumerConfig.DeliverPolicy = nats.DeliverAllPolicy
		case "last":
			consumerConfig.DeliverPolicy = nats.DeliverLastPolicy
		case "new":
			consumerConfig.DeliverPolicy = nats.DeliverNewPolicy
		default:
			consumerConfig.DeliverPolicy = nats.DeliverAllPolicy
		}

		// Parse ack policy
		switch c.config.AckPolicy {
		case "explicit":
			consumerConfig.AckPolicy = nats.AckExplicitPolicy
		case "none":
			consumerConfig.AckPolicy = nats.AckNonePolicy
		case "all":
			consumerConfig.AckPolicy = nats.AckAllPolicy
		default:
			consumerConfig.AckPolicy = nats.AckExplicitPolicy
		}

		_, err = js.AddConsumer(c.stream, consumerConfig)
		if err != nil {
			return nil, types.WrapSMTSError(err, types.ErrNATSConsumer, "Failed to create consumer")
		}

		c.logger.Info("Consumer created", 
			zap.String("stream", c.stream),
			zap.String("consumer", c.config.DurableName))
	} else {
		c.logger.Info("Using existing consumer", 
			zap.String("stream", c.stream),
			zap.String("consumer", c.config.DurableName))
	}

	return js, nil
}

// handleMessage processes individual messages from NATS
func (c *Consumer) handleMessage(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Parse the message
	smtsMsg, err := c.parseMessage(msg)
	if err != nil {
		c.logger.Error("Failed to parse message",
			zap.Error(err),
			zap.String("subject", msg.Subject))
		
		// Acknowledge the message to avoid reprocessing
		if ackErr := msg.Ack(); ackErr != nil {
			c.logger.Error("Failed to ack invalid message", zap.Error(ackErr))
		}
		return
	}

	// Process the message
	result, err := c.handler.HandleMessage(ctx, smtsMsg)
	if err != nil {
		c.logger.Error("Failed to process message",
			zap.Error(err),
			zap.String("message_id", smtsMsg.ID),
			zap.String("topic", smtsMsg.Topic))

		// Check if we should retry or nack the message
		if types.IsRetryableError(err) {
			// Nack with delay for retryable errors
			if nackErr := msg.NakWithDelay(10 * time.Second); nackErr != nil {
				c.logger.Error("Failed to nack message", zap.Error(nackErr))
			}
		} else {
			// Ack non-retryable errors to avoid reprocessing
			if ackErr := msg.Ack(); ackErr != nil {
				c.logger.Error("Failed to ack failed message", zap.Error(ackErr))
			}
		}
		return
	}

	// Acknowledge successful processing
	if result.Success {
		if err := msg.Ack(); err != nil {
			c.logger.Error("Failed to ack successful message",
				zap.Error(err),
				zap.String("message_id", smtsMsg.ID))
		} else {
			c.logger.Debug("Message processed successfully",
				zap.String("message_id", smtsMsg.ID),
				zap.String("topic", smtsMsg.Topic))
		}
	} else {
		// Message processing failed but we want to ack to avoid reprocessing
		if err := msg.Ack(); err != nil {
			c.logger.Error("Failed to ack failed message",
				zap.Error(err),
				zap.String("message_id", smtsMsg.ID))
		}
		c.logger.Warn("Message processing failed",
			zap.String("message_id", smtsMsg.ID),
			zap.String("topic", smtsMsg.Topic),
			zap.String("error", result.Error))
	}
}

// startPullConsumer starts a pull consumer that fetches messages manually
func (c *Consumer) startPullConsumer(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Fetch messages from the pull subscription
			msgs, err := c.subscription.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil && err != nats.ErrTimeout {
				c.logger.Error("Failed to fetch messages", zap.Error(err))
				continue
			}

			// Process each message
			for _, msg := range msgs {
				c.handleMessage(msg)
			}
		}
	}
}

// parseMessage converts a NATS message to our SMTS message format
func (c *Consumer) parseMessage(msg *nats.Msg) (*types.Message, error) {
	var smtsMsg types.Message
	
	// Try to parse as JSON first
	if err := json.Unmarshal(msg.Data, &smtsMsg); err != nil {
		// If JSON parsing fails, create a message with raw body
		smtsMsg = types.Message{
			ID:        fmt.Sprintf("nats-%d", time.Now().UnixNano()),
			Timestamp: time.Now().UTC(),
			Topic:     msg.Subject,
			Source:    "nats",
			Headers:   make(map[string]string),
			Body:      msg.Data,
		}
	}

	// Extract headers from NATS message
	if msg.Header != nil {
		for key, values := range msg.Header {
			if len(values) > 0 {
				smtsMsg.Headers[key] = values[0]
			}
		}
	}

	// Ensure required fields
	if smtsMsg.ID == "" {
		smtsMsg.ID = fmt.Sprintf("nats-%d", time.Now().UnixNano())
	}
	if smtsMsg.Topic == "" {
		smtsMsg.Topic = msg.Subject
	}
	if smtsMsg.Timestamp.IsZero() {
		smtsMsg.Timestamp = time.Now().UTC()
	}
	if smtsMsg.Headers == nil {
		smtsMsg.Headers = make(map[string]string)
	}

	return &smtsMsg, nil
}

// waitForShutdown waits for context cancellation and stops the consumer
func (c *Consumer) waitForShutdown(ctx context.Context) {
	<-ctx.Done()
	
	if err := c.Stop(); err != nil {
		c.logger.Error("Error stopping consumer", zap.Error(err))
	}
}

// GetConsumerInfo returns information about the consumer
func (c *Consumer) GetConsumerInfo() (*nats.ConsumerInfo, error) {
	if c.client.js == nil {
		return nil, types.NewSMTSError(types.ErrNATSConsumer, "JetStream context not initialized")
	}

	info, err := c.client.js.ConsumerInfo(c.stream, c.config.DurableName)
	if err != nil {
		return nil, types.WrapSMTSError(err, types.ErrNATSConsumer, "Failed to get consumer info")
	}

	return info, nil
}

// GetPendingMessages returns the number of pending messages
func (c *Consumer) GetPendingMessages() (int, error) {
	info, err := c.GetConsumerInfo()
	if err != nil {
		return 0, err
	}

	return int(info.NumPending), nil
}