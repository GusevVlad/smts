package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"smts/internal/nats"
	"smts/pkg/types"

	"go.uber.org/zap"
)

// MessageAPIServer handles message operations
type MessageAPIServer struct {
	config          *types.Config
	logger          *zap.Logger
	server          *http.Server
	natsClient      *nats.Client
	ldapMiddleware  *LDAPMiddleware
	messageRetriever *MessageRetriever
	httpUtilities   *HTTPUtilities
}

// NewMessageAPIServer creates a new message API server
func NewMessageAPIServer(config *types.Config, logger *zap.Logger) *MessageAPIServer {
	ldapMiddleware := NewLDAPMiddleware(&config.LDAP, logger)
	messageRetriever := NewMessageRetriever(logger)
	httpUtilities := NewHTTPUtilities(logger)

	return &MessageAPIServer{
		config:          config,
		logger:          logger,
		ldapMiddleware:  ldapMiddleware,
		messageRetriever: messageRetriever,
		httpUtilities:   httpUtilities,
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

	if !m.httpUtilities.RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Parse query parameters
	topic := r.URL.Query().Get("topic")
	countStr := r.URL.Query().Get("count")

	if topic == "" {
		m.logger.Warn("Missing topic parameter in message API request",
			zap.String("path", r.URL.Path))
		m.httpUtilities.JSONError(w, "topic parameter is required", http.StatusBadRequest)
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
			zap.String("deployment", m.config.Deployment.Type),
			zap.String("client_receiver", clientReceiver))
		m.httpUtilities.JSONError(w, "Access denied - insufficient permissions", http.StatusForbidden)
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
			m.httpUtilities.JSONError(w, "count must be a positive integer", http.StatusBadRequest)
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

	// Use shared message retriever
	messages, received, err := m.messageRetriever.RetrieveMessages(
		m.natsClient, m.config, topic, count, clientReceiver)
	if err != nil {
		m.logger.Error("Failed to retrieve messages for message API",
			zap.String("topic", topic),
			zap.Int("count", count),
			zap.Error(err))
		m.httpUtilities.JSONError(w, "Failed to retrieve messages", http.StatusInternalServerError)
		return
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

	m.httpUtilities.JSONSuccess(w, response, http.StatusOK)
}

// sendHandler handles message sending endpoint (for testing)
func (m *MessageAPIServer) sendHandler(w http.ResponseWriter, r *http.Request) {
	if !m.httpUtilities.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var request struct {
		Topic   string                 `json:"topic"`
		Message map[string]interface{} `json:"message"`
	}

	if !m.httpUtilities.RequireJSONBody(w, r, &request) {
		return
	}

	if request.Topic == "" {
		m.httpUtilities.JSONError(w, "topic is required", http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"status":     "sent",
		"message_id": fmt.Sprintf("sent-%s", time.Now().UTC().Format("20060102150405")),
		"topic":      request.Topic,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"deployment": m.config.Deployment.Type,
	}

	m.httpUtilities.JSONSuccess(w, response, http.StatusOK)
}

