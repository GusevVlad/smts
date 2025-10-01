package nats

import (
	"context"
	"fmt"
	"time"

	"smts/pkg/types"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// NATSClient defines the interface for NATS operations
type NATSClient interface {
	CreateStream(streamConfig *types.StreamConfig) error
	GetStreamInfo(streamName string) (*nats.StreamInfo, error)
	DeleteStream(streamName string) error
	HealthCheck(ctx context.Context) error
	Close()
	GetJetStream() nats.JetStreamContext
	GetConnection() *nats.Conn
	IsConnected() bool
}

// Client represents a NATS JetStream client
type Client struct {
	conn      *nats.Conn
	js        nats.JetStreamContext
	config    *types.NATSConfig
	logger    *zap.Logger
	embedded  bool
	server    *EmbeddedServer
}

// NewClient creates a new NATS client
func NewClient(config *types.NATSConfig, logger *zap.Logger) (*Client, error) {
	client := &Client{
		config: config,
		logger: logger,
	}

	// Start embedded server if configured
	if config.Embedded {
		server, err := NewEmbeddedServer(config, logger)
		if err != nil {
			return nil, types.WrapSMTSError(err, types.ErrNATSConnection, "Failed to start embedded NATS server")
		}
		client.server = server
		client.embedded = true
		client.logger.Info("Embedded NATS server started")
	}

	// Connect to NATS
	if err := client.connect(); err != nil {
		return nil, err
	}

	// Initialize JetStream context
	js, err := client.conn.JetStream()
	if err != nil {
		client.conn.Close()
		return nil, types.WrapSMTSError(err, types.ErrNATSConnection, "Failed to create JetStream context")
	}
	client.js = js

	client.logger.Info("NATS JetStream client initialized",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.Bool("embedded", config.Embedded))

	return client, nil
}

// connect establishes connection to NATS server
func (c *Client) connect() error {
	url := fmt.Sprintf("nats://%s:%d", c.config.Host, c.config.Port)
	
	// For embedded servers, add retry logic to handle startup time
	var conn *nats.Conn
	var err error
	
	maxAttempts := 5
	attempt := 0
	
	for attempt < maxAttempts {
		conn, err = nats.Connect(url,
			nats.MaxReconnects(-1), // Infinite reconnects
			nats.ReconnectWait(2*time.Second),
			nats.Timeout(10*time.Second),
			nats.PingInterval(30*time.Second),
			nats.MaxPingsOutstanding(3),
		)
		
		if err == nil {
			break
		}
		
		attempt++
		if attempt < maxAttempts {
			c.logger.Warn("Failed to connect to NATS server, retrying",
				zap.String("url", url),
				zap.Int("attempt", attempt),
				zap.Int("max_attempts", maxAttempts),
				zap.Error(err))
			time.Sleep(2 * time.Second)
		}
	}
	
	if err != nil {
		return types.WrapSMTSError(err, types.ErrNATSConnection, types.MsgNATSConnectionFailed)
	}

	c.conn = conn

	// Set up connection event handlers
	c.conn.SetDisconnectErrHandler(func(nc *nats.Conn, err error) {
		c.logger.Warn("NATS connection disconnected", zap.Error(err))
	})

	c.conn.SetReconnectHandler(func(nc *nats.Conn) {
		c.logger.Info("NATS connection reestablished")
	})

	c.conn.SetClosedHandler(func(nc *nats.Conn) {
		c.logger.Error("NATS connection closed")
	})

	c.logger.Info("Connected to NATS server", zap.String("url", url))
	return nil
}

// Close closes the NATS connection and stops embedded server if running
func (c *Client) Close() {
	if c.conn != nil && !c.conn.IsClosed() {
		c.conn.Close()
		c.logger.Info("NATS connection closed")
	}

	if c.embedded && c.server != nil {
		c.server.Stop()
		c.logger.Info("Embedded NATS server stopped")
	}
}

// GetJetStream returns the JetStream context
func (c *Client) GetJetStream() nats.JetStreamContext {
	return c.js
}

// GetConnection returns the underlying NATS connection
func (c *Client) GetConnection() *nats.Conn {
	return c.conn
}

// IsConnected returns true if the client is connected to NATS
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}

// HealthCheck performs a health check on the NATS connection
func (c *Client) HealthCheck(ctx context.Context) error {
	if !c.IsConnected() {
		return types.NewSMTSError(types.ErrHealthCheck, "NATS connection is not connected")
	}

	// Create a unique subject for this health check to avoid conflicts
	subject := fmt.Sprintf("smts.health.check.%d", time.Now().UnixNano())
	message := []byte("health_check")

	// Create subscription first to ensure we don't miss the message
	sub, err := c.conn.SubscribeSync(subject)
	if err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "NATS health check subscribe failed")
	}
	defer sub.Unsubscribe()

	// Give the subscription a moment to be established
	time.Sleep(100 * time.Millisecond)

	// Publish the health check message
	if err := c.conn.Publish(subject, message); err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "NATS health check publish failed")
	}

	// Wait for the message with timeout
	if _, err := sub.NextMsgWithContext(ctx); err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "NATS health check message receive failed")
	}

	return nil
}

// CreateStream creates or updates a JetStream stream
func (c *Client) CreateStream(streamConfig *types.StreamConfig) error {
	if c.js == nil {
		return types.NewSMTSError(types.ErrNATSStream, "JetStream context not initialized")
	}

	// Convert our stream config to NATS stream configuration
	natsConfig := &nats.StreamConfig{
		Name:      streamConfig.Name,
		Subjects:  streamConfig.Subjects,
		Retention: nats.RetentionPolicy(nats.WorkQueuePolicy), // Default to workqueue
		Storage:   nats.FileStorage,                           // Default to file storage
		Replicas:  streamConfig.Replicas,
	}

	// Parse retention policy
	switch streamConfig.Retention {
	case "workqueue":
		natsConfig.Retention = nats.WorkQueuePolicy
	case "limits":
		natsConfig.Retention = nats.LimitsPolicy
	case "interest":
		natsConfig.Retention = nats.InterestPolicy
	default:
		natsConfig.Retention = nats.WorkQueuePolicy
	}

	// Parse storage type
	switch streamConfig.Storage {
	case "memory":
		natsConfig.Storage = nats.MemoryStorage
	case "file":
		natsConfig.Storage = nats.FileStorage
	default:
		natsConfig.Storage = nats.FileStorage
	}

	// Parse max age
	if streamConfig.MaxAge != "" {
		if maxAge, err := time.ParseDuration(streamConfig.MaxAge); err == nil {
			natsConfig.MaxAge = maxAge
		} else {
			c.logger.Warn("Invalid max age duration, using default", 
				zap.String("max_age", streamConfig.MaxAge),
				zap.Error(err))
		}
	}

	// Create or update the stream
	stream, err := c.js.StreamInfo(streamConfig.Name)
	if err != nil && err != nats.ErrStreamNotFound {
		return types.WrapSMTSError(err, types.ErrNATSStream, "Failed to get stream info")
	}

	if stream != nil {
		// Stream exists, update it
		if _, err := c.js.UpdateStream(natsConfig); err != nil {
			return types.WrapSMTSError(err, types.ErrNATSStream, "Failed to update stream")
		}
		c.logger.Info("Stream updated", zap.String("stream", streamConfig.Name))
	} else {
		// Create new stream
		if _, err := c.js.AddStream(natsConfig); err != nil {
			return types.WrapSMTSError(err, types.ErrNATSStream, "Failed to create stream")
		}
		c.logger.Info("Stream created", zap.String("stream", streamConfig.Name))
	}

	return nil
}

// GetStreamInfo returns information about a stream
func (c *Client) GetStreamInfo(streamName string) (*nats.StreamInfo, error) {
	if c.js == nil {
		return nil, types.NewSMTSError(types.ErrNATSStream, "JetStream context not initialized")
	}

	info, err := c.js.StreamInfo(streamName)
	if err != nil {
		return nil, types.WrapSMTSError(err, types.ErrNATSStream, "Failed to get stream info")
	}

	return info, nil
}

// DeleteStream deletes a JetStream stream
func (c *Client) DeleteStream(streamName string) error {
	if c.js == nil {
		return types.NewSMTSError(types.ErrNATSStream, "JetStream context not initialized")
	}

	if err := c.js.DeleteStream(streamName); err != nil {
		return types.WrapSMTSError(err, types.ErrNATSStream, "Failed to delete stream")
	}

	c.logger.Info("Stream deleted", zap.String("stream", streamName))
	return nil
}