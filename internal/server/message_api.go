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
	config     *types.Config
	logger     *zap.Logger
	server     *http.Server
	natsClient *nats.Client
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
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	topic := r.URL.Query().Get("topic")
	countStr := r.URL.Query().Get("count")
	
	if topic == "" {
		http.Error(w, `{"error": "topic parameter is required"}`, http.StatusBadRequest)
		return
	}

	// Check topic authorization if LDAP is enabled
	if m.ldapMiddleware != nil && !m.ldapMiddleware.AuthorizeTopic(r, topic) {
		m.logger.Warn("Topic authorization denied",
			zap.String("topic", topic))
		http.Error(w, `{"error": "Access denied to requested topic"}`, http.StatusForbidden)
		return
	}

	// Default to 1 message if count not specified
	count := 1
	if countStr != "" {
		if parsedCount, err := strconv.Atoi(countStr); err != nil || parsedCount < 1 {
			http.Error(w, `{"error": "count must be a positive integer"}`, http.StatusBadRequest)
			return
		} else {
			count = parsedCount
		}
	}

	// Limit maximum count to prevent abuse
	if count > 100 {
		count = 100
	}

	w.Header().Set("Content-Type", "application/json")
	
	// Check if NATS client is available
	if m.natsClient == nil {
		m.logger.Error("NATS client not available for message API")
		http.Error(w, `{"error": "Message API not properly initialized"}`, http.StatusInternalServerError)
		return
	}

	js := m.natsClient.GetJetStream()
	if js == nil {
		m.logger.Error("JetStream context not available")
		http.Error(w, `{"error": "JetStream not available"}`, http.StatusInternalServerError)
		return
	}

	// Read messages from NATS stream
	messages := make([]map[string]interface{}, 0)
	received := 0

	// For workqueue streams, we need to use the existing consumer
	// Use the external consumer for reading incoming INT messages
	consumerName := m.config.NATS.ExternalConsumer.DurableName
	streamName := m.config.NATS.ExternalStream.Name
	
	// Check if the consumer exists
	_, err := js.ConsumerInfo(streamName, consumerName)
	if err != nil {
		m.logger.Error("Failed to get consumer info",
			zap.String("stream", streamName),
			zap.String("consumer", consumerName),
			zap.Error(err))
		http.Error(w, `{"error": "Failed to access message stream consumer"}`, http.StatusInternalServerError)
		return
	}

	// For external stream, use the external topic pattern
	externalTopic := "external." + topic
	sub, err := js.PullSubscribe(externalTopic, consumerName, natsio.Bind(streamName, consumerName))
	if err != nil {
		m.logger.Error("Failed to subscribe to messages",
			zap.String("topic", topic),
			zap.String("consumer", consumerName),
			zap.Error(err))
		http.Error(w, `{"error": "Failed to subscribe to messages"}`, http.StatusInternalServerError)
		return
	}
	defer sub.Unsubscribe()

	// Fetch messages
	fetchedMsgs, err := sub.Fetch(count, natsio.MaxWait(5*time.Second))
	if err != nil && err != natsio.ErrTimeout {
		m.logger.Error("Failed to fetch messages", zap.Error(err))
		http.Error(w, `{"error": "Failed to fetch messages"}`, http.StatusInternalServerError)
		return
	}

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
		
		messageData := map[string]interface{}{
			"id":        headers["X-SMTS-Message-ID"],
			"topic":     originalTopic,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"source":    headers["X-SMTS-Source"],
			"headers":   headers,
			"body":      json.RawMessage(msg.Data),
		}

		messages = append(messages, messageData)
		
		// Acknowledge the message
		if err := msg.Ack(); err != nil {
			m.logger.Warn("Failed to acknowledge message", zap.Error(err))
		}
	}

	m.logger.Info("Message API retrieved messages",
		zap.String("topic", topic),
		zap.Int("requested", count),
		zap.Int("received", received),
		zap.String("deployment", m.config.Deployment.Type))

	response := map[string]interface{}{
		"topic":      topic,
		"count":      count,
		"received":   received,
		"messages":   messages,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"deployment": m.config.Deployment.Type,
		"api":        "message-api",
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		m.logger.Error("Failed to marshal messages response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

// sendHandler handles message sending endpoint (for testing)
func (m *MessageAPIServer) sendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Topic   string                 `json:"topic"`
		Message map[string]interface{} `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if request.Topic == "" {
		http.Error(w, `{"error": "topic is required"}`, http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"status":    "sent",
		"message_id": fmt.Sprintf("sent-%s", time.Now().UTC().Format("20060102150405")),
		"topic":     request.Topic,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"deployment": m.config.Deployment.Type,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}