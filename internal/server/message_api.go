package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"smts/internal/nats"
	"smts/pkg/types"

	natsio "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// MessageAPIServer handles message operations
type MessageAPIServer struct {
	config         *types.Config
	logger         *zap.Logger
	server         *http.Server
	natsClient     *nats.Client
	ldapMiddleware *LDAPMiddleware
}

// NewMessageAPIServer creates a new message API server
func NewMessageAPIServer(config *types.Config, logger *zap.Logger) *MessageAPIServer {
	ldapMiddleware := NewLDAPMiddleware(&config.LDAP, logger)

	return &MessageAPIServer{
		config:         config,
		logger:         logger,
		ldapMiddleware: ldapMiddleware,
	}
}

// SetNATSClient sets the NATS client for the message API server
func (m *MessageAPIServer) SetNATSClient(client *nats.Client) {
	m.natsClient = client
}

// Start starts the message API server
func (m *MessageAPIServer) Start() error {
	mux := http.NewServeMux()

	// Apply LDAP authentication to message endpoints
	mux.HandleFunc("/messages", m.ldapMiddleware.Authenticate(m.messagesHandler))
	mux.HandleFunc("/send", m.ldapMiddleware.Authenticate(m.sendHandler))

	// Use a different port than health server
	port := m.config.Health.Port + 1000 // Use health port + 1000

	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		m.logger.Info("Starting message API server",
			zap.Int("port", port))

		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Error("Message API server failed", zap.Error(err))
		}
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Stop stops the message API server
func (m *MessageAPIServer) Stop() error {
	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown message API server: %w", err)
		}
	}

	// Close LDAP connection
	if m.ldapMiddleware != nil {
		m.ldapMiddleware.Close()
	}

	m.logger.Info("Message API server stopped")
	return nil
}

// messagesHandler handles message retrieval endpoint
func (m *MessageAPIServer) messagesHandler(w http.ResponseWriter, r *http.Request) {
	// Get client_receiver from LDAP context
	clientReceiver := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientReceiver = userInfo["uid"]
	}

	m.logger.Info("Received message API request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("deployment", m.config.Deployment.Type),
		zap.String("client_receiver", clientReceiver))

	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Parse query parameters
	topic := r.URL.Query().Get("topic")
	countStr := r.URL.Query().Get("count")

	if topic == "" {
		m.logger.Warn("Missing topic parameter in message API request",
			zap.String("path", r.URL.Path))
		JSONError(w, "topic parameter is required", http.StatusBadRequest)
		return
	}

	m.logger.Info("Processing message API request for topic",
		zap.String("topic", topic),
		zap.String("deployment", m.config.Deployment.Type),
		zap.String("client_receiver", clientReceiver))

	// Check LDAP authorization for reading messages
	if m.ldapMiddleware != nil && !m.ldapMiddleware.AuthorizeEndpoint(r, m.config.Deployment.Type, "read") {
		m.logger.Warn("LDAP authorization denied for read endpoint",
			zap.String("topic", topic),
			zap.String("deployment", m.config.Deployment.Type))
			zap.String("client_receiver", clientReceiver)
		JSONError(w, "Access denied - insufficient permissions", http.StatusForbidden)
		return
	}

	// Default to 1 message if count not specified
	count := 1
	if countStr != "" {
		if parsedCount, err := strconv.Atoi(countStr); err != nil || parsedCount < 1 {
			m.logger.Warn("Invalid count parameter in message API request",
				zap.String("topic", topic),
				zap.String("count", countStr),
				zap.Error(err))
			JSONError(w, "count must be a positive integer", http.StatusBadRequest)
			return
		} else {
			count = parsedCount
		}
	}

	// Limit maximum count to prevent abuse
	if count > 100 {
		count = 100
	}

	m.logger.Info("Message API request parameters",
		zap.String("topic", topic),
		zap.Int("requested_count", count),
		zap.String("deployment", m.config.Deployment.Type))


	// Check if NATS client is available
	if m.natsClient == nil {
		m.logger.Error("NATS client not available for message API")
		JSONError(w, "Message API not properly initialized", http.StatusInternalServerError)
		return
	}

	js := m.natsClient.GetJetStream()
	if js == nil {
		m.logger.Error("JetStream context not available")
		JSONError(w, "JetStream not available", http.StatusInternalServerError)
		return
	}

	// Read messages from NATS stream
	messages := make([]map[string]interface{}, 0)
	received := 0

	// For workqueue streams, we need to use the existing consumer
	// Use the external consumer for reading incoming INT messages
	consumerName := m.config.NATS.ExternalConsumer.DurableName
	streamName := m.config.NATS.ExternalStream.Name

	m.logger.Info("Accessing NATS stream for message retrieval",
		zap.String("topic", topic),
		zap.String("stream", streamName),
		zap.String("consumer", consumerName))

	// Check if the consumer exists
	_, err := js.ConsumerInfo(streamName, consumerName)
	if err != nil {
		m.logger.Error("Failed to get consumer info",
			zap.String("stream", streamName),
			zap.String("consumer", consumerName),
			zap.String("topic", topic),
			zap.String("client_receiver", clientReceiver),
			zap.Error(err))
		JSONError(w, "Failed to access message stream consumer", http.StatusInternalServerError)
		return
	}

	// For external stream, use the external topic pattern
	externalTopic := "external." + topic
	sub, err := js.PullSubscribe(externalTopic, consumerName, natsio.Bind(streamName, consumerName))
	if err != nil {
		m.logger.Error("Failed to subscribe to messages",
			zap.String("topic", topic),
			zap.String("external_topic", externalTopic),
			zap.String("consumer", consumerName),
			zap.Error(err))
		JSONError(w, "Failed to subscribe to messages", http.StatusInternalServerError)
		return
	}
	defer sub.Unsubscribe()

	m.logger.Info("Fetching messages from NATS stream",
		zap.String("topic", topic),
		zap.String("external_topic", externalTopic),
		zap.Int("requested_count", count))

	// Fetch messages
	fetchedMsgs, err := sub.Fetch(count, natsio.MaxWait(5*time.Second))
	if err != nil && err != natsio.ErrTimeout {
		m.logger.Error("Failed to fetch messages",
			zap.String("topic", topic),
			zap.Int("requested_count", count),
			zap.Error(err))
		JSONError(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	m.logger.Info("Successfully fetched messages from NATS stream",
		zap.String("topic", topic),
		zap.Int("requested_count", count),
		zap.Int("fetched_count", len(fetchedMsgs)))

	// Process fetched messages
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
			m.logger.Warn("Failed to acknowledge message",
				zap.String("message_id", headers["X-SMTS-Message-ID"]),
				zap.String("topic", topic),
				zap.Error(err))
		}
	}

	m.logger.Info("Message API retrieved messages successfully",
		zap.String("topic", topic),
		zap.Int("requested", count),
		zap.Int("received", received),
		zap.String("deployment", m.config.Deployment.Type),
		zap.String("client_receiver", clientReceiver))

	response := map[string]interface{}{
		"topic":      topic,
		"count":      count,
		"received":   received,
		"messages":   messages,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"deployment": m.config.Deployment.Type,
		"api":        "message-api",
	}

	m.logger.Info("Message API request completed successfully",
		zap.String("topic", topic),
		zap.Int("message_count", len(messages)),
		zap.String("deployment", m.config.Deployment.Type),
		zap.String("client_receiver", clientReceiver))

	JSONSuccess(w, response, http.StatusOK)
}

// sendHandler handles message sending endpoint (for testing)
func (m *MessageAPIServer) sendHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	var request struct {
		Topic   string                 `json:"topic"`
		Message map[string]interface{} `json:"message"`
	}

	if !RequireJSONBody(w, r, &request) {
		return
	}

	if request.Topic == "" {
		JSONError(w, "topic is required", http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"status":     "sent",
		"message_id": fmt.Sprintf("sent-%s", time.Now().UTC().Format("20060102150405")),
		"topic":      request.Topic,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"deployment": m.config.Deployment.Type,
	}

	JSONSuccess(w, response, http.StatusOK)
}

