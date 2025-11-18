package server

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"smts/internal/api"
	"smts/internal/artemis"
	"smts/internal/config"
	"smts/internal/message"
	"smts/internal/nats"
	"smts/pkg/types"
	"smts/pkg/utils"

	natsio "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Server represents the main SMTS server
type Server struct {
	config           *types.Config
	logger           *zap.Logger
	natsClient       *nats.Client
	apiClient        *api.Client
	artemisClient    *artemis.Client
	processor        *message.Processor
	clientConsumer   *nats.Consumer
	externalConsumer *nats.Consumer
	healthServer     *HealthServer
	messageAPIServer *MessageAPIServer
	publisher        *nats.Publisher
	httpServer       *http.Server
	ldapMiddleware   *LDAPMiddleware
	running          bool
}

// NewServer creates a new SMTS server
func NewServer(configPath string) (*Server, error) {
	// Create temporary logger for configuration loading
	tempLogger, err := utils.NewLogger("info", "json", "stdout")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary logger: %w", err)
	}

	// Load configuration
	configLoader := config.NewLoader(tempLogger)
	cfg, err := configLoader.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create proper logger with configuration settings
	logger, err := utils.NewLogger(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Recreate config loader with proper logger and reload configuration
	configLoader = config.NewLoader(logger)
	cfg, err = configLoader.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to reload configuration: %w", err)
	}

	// Load topics configuration only if not already defined in main config
	if len(cfg.Topics.Topics) == 0 {
		topicsConfig, err := configLoader.LoadTopicsConfig("", cfg.Deployment.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to load topics configuration: %w", err)
		}
		cfg.Topics = *topicsConfig
	} else {
		logger.Info("Using topics configuration from main config file",
			zap.String("deployment", cfg.Deployment.Type))
	}

	// Create NATS client
	natsClient, err := nats.NewClient(&cfg.NATS, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS client: %w", err)
	}

	// Create API client
	apiClient := api.NewClient(&cfg.API, &cfg.DLP, logger)

	// Create Artemis client for INT deployment
	var artemisClient *artemis.Client
	if cfg.Deployment.Type == "int" && cfg.Artemis.Enabled {
		artemisClient, err = artemis.NewClient(&cfg.Artemis, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Artemis client: %w", err)
		}
	}

	// Create NATS publisher
	publisher := nats.NewPublisher(natsClient, logger)

	// Create message processor
	processor := message.NewProcessor(cfg, apiClient, natsClient, publisher, artemisClient, logger)

	// Create NATS consumer for client messages
	clientConsumer := nats.NewConsumer(natsClient, cfg.NATS.ClientStream.Name, &cfg.NATS.ClientConsumer, processor, logger, cfg.Deployment.Type)

	// Create NATS consumer for external messages (for message API only - no processing)
	externalConsumer := nats.NewConsumer(natsClient, cfg.NATS.ExternalStream.Name, &cfg.NATS.ExternalConsumer, nil, logger, cfg.Deployment.Type)

	// Create health server
	healthServer := NewHealthServer(cfg, logger)

	// Create message API server
	messageAPIServer := NewMessageAPIServer(cfg, logger)
	messageAPIServer.SetNATSClient(natsClient)

	// Create LDAP middleware for main server
	ldapMiddleware := NewLDAPMiddleware(&cfg.LDAP, logger)

	server := &Server{
		config:           cfg,
		logger:           logger,
		natsClient:       natsClient,
		apiClient:        apiClient,
		artemisClient:    artemisClient,
		processor:        processor,
		clientConsumer:   clientConsumer,
		externalConsumer: externalConsumer,
		publisher:        publisher,
		healthServer:     healthServer,
		messageAPIServer: messageAPIServer,
		ldapMiddleware:   ldapMiddleware,
	}

	return server, nil
}

// Start starts the SMTS server
func (s *Server) Start() error {
	s.logger.Info("Starting SMTS server",
		zap.String("deployment", s.config.Deployment.Type),
		zap.String("environment", s.config.Deployment.Environment))

	// Create streams if they don't exist
	if err := s.natsClient.CreateStream(&s.config.NATS.ClientStream); err != nil {
		return fmt.Errorf("failed to create NATS client stream: %w", err)
	}
	if err := s.natsClient.CreateStream(&s.config.NATS.ExternalStream); err != nil {
		return fmt.Errorf("failed to create NATS external stream: %w", err)
	}

	// Start HTTP server for message publishing
	if err := s.startHTTPServer(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start health server
	if err := s.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}

	// Start message API server
	if err := s.messageAPIServer.Start(); err != nil {
		return fmt.Errorf("failed to start message API server: %w", err)
	}

	// Start NATS consumer for client messages
	ctx := context.Background()
	if err := s.clientConsumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start NATS client consumer: %w", err)
	}

	// Create external consumer without starting processing loop
	// This ensures the consumer exists for message API to read from
	if err := s.createExternalConsumer(); err != nil {
		return fmt.Errorf("failed to create external consumer: %w", err)
	}

	// Start Artemis consumer for INT deployment
	if s.artemisClient != nil {
		if err := s.artemisClient.Start(ctx, s.processor); err != nil {
			return fmt.Errorf("failed to start Artemis consumer: %w", err)
		}
	}

	s.running = true
	s.logger.Info("SMTS server started successfully")

	// Set health status to healthy
	s.healthServer.SetHealthy(true)

	return nil
}

// Stop stops the SMTS server gracefully
func (s *Server) Stop() error {
	s.logger.Info("Stopping SMTS server")

	// Set health status to unhealthy
	s.healthServer.SetHealthy(false)

	// Stop Artemis client
	if s.artemisClient != nil {
		if err := s.artemisClient.Stop(); err != nil {
			s.logger.Error("Error stopping Artemis client", zap.Error(err))
		}
	}

	// Stop NATS consumers
	if s.clientConsumer != nil {
		if err := s.clientConsumer.Stop(); err != nil {
			s.logger.Error("Error stopping NATS client consumer", zap.Error(err))
		}
	}
	if s.externalConsumer != nil {
		if err := s.externalConsumer.Stop(); err != nil {
			s.logger.Error("Error stopping NATS external consumer", zap.Error(err))
		}
	}

	// Stop HTTP server
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Error("Error stopping HTTP server", zap.Error(err))
		}
	}

	// Stop message API server
	if s.messageAPIServer != nil {
		if err := s.messageAPIServer.Stop(); err != nil {
			s.logger.Error("Error stopping message API server", zap.Error(err))
		}
	}

	// Stop health server
	if s.healthServer != nil {
		if err := s.healthServer.Stop(); err != nil {
			s.logger.Error("Error stopping health server", zap.Error(err))
		}
	}

	// Close LDAP connection
	if s.ldapMiddleware != nil {
		s.ldapMiddleware.Close()
	}

	// Close NATS client
	if s.natsClient != nil {
		s.natsClient.Close()
	}

	s.running = false
	s.logger.Info("SMTS server stopped")

	return nil
}

// WaitForShutdown waits for shutdown signals and stops the server gracefully
func (s *Server) WaitForShutdown() {
	// Create channel to listen for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	s.logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Stop the server
	if err := s.Stop(); err != nil {
		s.logger.Error("Error stopping server", zap.Error(err))
		os.Exit(1)
	}

	os.Exit(0)
}

// IsRunning returns true if the server is running
func (s *Server) IsRunning() bool {
	return s.running
}

// HealthCheck performs a comprehensive health check
func (s *Server) HealthCheck(ctx context.Context) error {
	// Check NATS connection
	if err := s.natsClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("NATS health check failed: %w", err)
	}

	// Check API client
	if err := s.apiClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("API client health check failed: %w", err)
	}

	// Check message processor
	if err := s.processor.HealthCheck(ctx); err != nil {
		return fmt.Errorf("message processor health check failed: %w", err)
	}

	// Check Artemis client for INT deployment
	if s.artemisClient != nil {
		if err := s.artemisClient.HealthCheck(ctx); err != nil {
			return fmt.Errorf("Artemis client health check failed: %w", err)
		}
	}

	return nil
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() *types.Config {
	return s.config
}

// GetLogger returns the server logger
func (s *Server) GetLogger() *zap.Logger {
	return s.logger
}

// startHTTPServer starts the HTTP server for message publishing
func (s *Server) startHTTPServer() error {
	mux := http.NewServeMux()

	// Apply LDAP authentication to message sending endpoints
	mux.HandleFunc("/send/", s.ldapMiddleware.Authenticate(s.messageHandler))

	// Apply LDAP authentication to corporate message endpoints
	mux.HandleFunc("/corp_message/", s.ldapMiddleware.Authenticate(s.corporateMessageHandler))

	// Apply LDAP authentication to new REST API endpoints according to OpenAPI spec
	mux.HandleFunc("/send", s.ldapMiddleware.Authenticate(s.sendHandler))
	mux.HandleFunc("/receive", s.ldapMiddleware.Authenticate(s.receiveHandler))
	mux.HandleFunc("/processed", s.ldapMiddleware.Authenticate(s.processedHandler))

	// Use a different port for HTTP server (health port + 1)
	port := s.config.Health.Port + 1

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		s.logger.Info("Starting HTTP server for message publishing",
			zap.Int("port", port))

		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server failed", zap.Error(err))
		}
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// messageHandler handles incoming HTTP messages and publishes them to NATS
func (s *Server) messageHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Received HTTP message request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("deployment", s.config.Deployment.Type))

	if r.Method != http.MethodPost {
		s.logger.Warn("Invalid HTTP method for message endpoint",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract topic from URL path (remove "/send/" prefix)
	if !strings.HasPrefix(r.URL.Path, "/send/") {
		s.logger.Warn("Invalid endpoint for message handler",
			zap.String("path", r.URL.Path))
		http.Error(w, `{"error": "Invalid endpoint"}`, http.StatusNotFound)
		return
	}

	topic := strings.TrimPrefix(r.URL.Path, "/send/")
	if topic == "" {
		s.logger.Warn("Missing topic in URL path",
			zap.String("path", r.URL.Path))
		http.Error(w, `{"error": "Topic is required in URL path"}`, http.StatusBadRequest)
		return
	}

	s.logger.Info("Processing HTTP message for topic",
		zap.String("topic", topic),
		zap.String("deployment", s.config.Deployment.Type))

	// Check LDAP authorization for sending messages
	if s.ldapMiddleware != nil && !s.ldapMiddleware.AuthorizeEndpoint(r, s.config.Deployment.Type, "send") {
		s.logger.Warn("LDAP authorization denied for send endpoint",
			zap.String("topic", topic),
			zap.String("deployment", s.config.Deployment.Type))
		http.Error(w, `{"error": "Access denied - insufficient permissions"}`, http.StatusForbidden)
		return
	}

	// Check authorization headers
	apiKey := r.Header.Get("X-API-Key")
	messageID := r.Header.Get("X-SMTS-Message-ID")
	timestamp := r.Header.Get("X-SMTS-Timestamp")
	source := r.Header.Get("X-SMTS-Source")

	// Validate required headers
	if apiKey == "" {
		s.logger.Warn("Missing X-API-Key header in HTTP request",
			zap.String("topic", topic))
		http.Error(w, `{"error": "X-API-Key header is required"}`, http.StatusUnauthorized)
		return
	}

	// Check if topic exists in configuration (basic validation)
	if _, exists := s.config.Topics.Topics[topic]; !exists {
		s.logger.Warn("Topic not found in configuration",
			zap.String("topic", topic))
		http.Error(w, `{"error": "Topic not found in configuration"}`, http.StatusBadRequest)
		return
	}

	// Read message body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("Failed to read request body",
			zap.String("topic", topic),
			zap.Error(err))
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	s.logger.Info("Read message body from HTTP request",
		zap.String("topic", topic),
		zap.Int("body_size", len(body)))

	// Parse timestamp
	var msgTimestamp time.Time
	if timestamp != "" {
		msgTimestamp, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			s.logger.Warn("Invalid timestamp format, using current time",
				zap.String("timestamp", timestamp),
				zap.String("topic", topic),
				zap.Error(err))
			msgTimestamp = time.Now().UTC()
		}
	} else {
		msgTimestamp = time.Now().UTC()
	}

	// Generate message ID if not provided
	if messageID == "" {
		messageID = nats.GenerateID()
		s.logger.Info("Generated message ID for HTTP request",
			zap.String("topic", topic),
			zap.String("message_id", messageID))
	}

	// Set default source if not provided
	if source == "" {
		source = "http-client"
	}

	// Create message headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 && strings.HasPrefix(strings.ToLower(key), "x-smts-") {
			headers[key] = values[0]
		}
	}

	// Add additional headers
	headers["X-SMTS-API-Key"] = apiKey
	headers["X-SMTS-Source"] = source

	// Get client_sender from LDAP context
	clientSender := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientSender = userInfo["uid"]
	}

	// Create message
	msg := &types.Message{
		ID:           messageID,
		Timestamp:    msgTimestamp,
		Topic:        topic,
		Source:       source,
		ClientSender: clientSender,
		Headers:      headers,
		Body:         body,
	}

	s.logger.Info("Publishing HTTP message to NATS",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("source", source),
		zap.String("client_sender", clientSender),
		zap.Time("message_timestamp", msgTimestamp))

	// Publish message to NATS
	if err := s.publisher.PublishMessage(msg); err != nil {
		s.logger.Error("Failed to publish message",
			zap.String("topic", topic),
			zap.String("message_id", messageID),
			zap.String("source", source),
			zap.Error(err))
		http.Error(w, `{"error": "Failed to publish message to stream"}`, http.StatusInternalServerError)
		return
	}

	s.logger.Info("HTTP message published successfully to NATS",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("source", source),
		zap.String("client_sender", clientSender),
		zap.String("deployment", s.config.Deployment.Type))

	// Return success response
	response := map[string]interface{}{
		"status":     "delivered",
		"message_id": messageID,
		"topic":      topic,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"deployment": s.config.Deployment.Type,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// corporateMessageHandler handles incoming corporate messages from corporate API (Flow 2: INT → EXT)
func (s *Server) corporateMessageHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Received corporate message request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("deployment", s.config.Deployment.Type))

	if r.Method != http.MethodPost {
		s.logger.Warn("Invalid HTTP method for corporate message endpoint",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract topic from URL path (remove "/corp_message/" prefix)
	if !strings.HasPrefix(r.URL.Path, "/corp_message/") {
		s.logger.Warn("Invalid endpoint for corporate message handler",
			zap.String("path", r.URL.Path))
		http.Error(w, `{"error": "Invalid endpoint"}`, http.StatusNotFound)
		return
	}

	topic := strings.TrimPrefix(r.URL.Path, "/corp_message/")
	if topic == "" {
		s.logger.Warn("Missing topic in URL path for corporate message",
			zap.String("path", r.URL.Path))
		http.Error(w, `{"error": "Topic is required in URL path"}`, http.StatusBadRequest)
		return
	}

	s.logger.Info("Processing corporate message for topic",
		zap.String("topic", topic),
		zap.String("deployment", s.config.Deployment.Type))

	// Check LDAP authorization for corporate message endpoint
	if s.ldapMiddleware != nil && !s.ldapMiddleware.AuthorizeEndpoint(r, s.config.Deployment.Type, "corp_message") {
		s.logger.Warn("LDAP authorization denied for corp_message endpoint",
			zap.String("topic", topic),
			zap.String("deployment", s.config.Deployment.Type))
		http.Error(w, `{"error": "Access denied - insufficient permissions"}`, http.StatusForbidden)
		return
	}

	// Check authorization headers
	apiKey := r.Header.Get("X-API-Key")
	messageID := r.Header.Get("X-SMTS-Message-ID")
	timestamp := r.Header.Get("X-SMTS-Timestamp")
	source := r.Header.Get("X-SMTS-Source")

	// Validate required headers
	if apiKey == "" {
		s.logger.Warn("Missing X-API-Key header in corporate message request",
			zap.String("topic", topic))
		http.Error(w, `{"error": "X-API-Key header is required"}`, http.StatusUnauthorized)
		return
	}

	// Check if topic exists in configuration (basic validation)
	if _, exists := s.config.Topics.Topics[topic]; !exists {
		s.logger.Warn("Topic not found in configuration for corporate message",
			zap.String("topic", topic))
		http.Error(w, `{"error": "Topic not found in configuration"}`, http.StatusBadRequest)
		return
	}

	// Read message body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("Failed to read request body for corporate message",
			zap.String("topic", topic),
			zap.Error(err))
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	s.logger.Info("Read corporate message body from HTTP request",
		zap.String("topic", topic),
		zap.Int("body_size", len(body)))

	// Parse timestamp
	var msgTimestamp time.Time
	if timestamp != "" {
		msgTimestamp, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			s.logger.Warn("Invalid timestamp format for corporate message, using current time",
				zap.String("timestamp", timestamp),
				zap.String("topic", topic),
				zap.Error(err))
			msgTimestamp = time.Now().UTC()
		}
	} else {
		msgTimestamp = time.Now().UTC()
	}

	// Generate message ID if not provided
	if messageID == "" {
		messageID = nats.GenerateID()
		s.logger.Info("Generated message ID for corporate message",
			zap.String("topic", topic),
			zap.String("message_id", messageID))
	}

	// Set default source if not provided
	if source == "" {
		source = "corporate-api"
	}

	// Create message headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 && strings.HasPrefix(strings.ToLower(key), "x-smts-") {
			headers[key] = values[0]
		}
	}

	// Add additional headers
	headers["X-SMTS-API-Key"] = apiKey
	headers["X-SMTS-Source"] = source

	// Get client_sender from LDAP context
	clientSender := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientSender = userInfo["uid"]
	}

	// For corporate messages (Flow 2), publish directly to external stream
	// This avoids the client consumer processing loop and makes messages available for external clients
	// Store only the actual content in the body to avoid duplication
	externalMsg := &types.Message{
		ID:           messageID,
		Timestamp:    msgTimestamp,
		Topic:        "external." + topic, // Use external subject pattern
		Source:       source,
		ClientSender: clientSender,
		Headers:      headers,
		Body:         body, // Store only the actual content, not the full message structure
	}

	s.logger.Info("Publishing corporate message to external NATS stream",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("source", source),
		zap.String("client_sender", clientSender),
		zap.String("external_topic", externalMsg.Topic),
		zap.String("stream", s.config.NATS.ExternalStream.Name),
		zap.Time("message_timestamp", msgTimestamp))

	// Publish message directly to external stream
	if err := s.publisher.PublishMessageToStream(externalMsg, s.config.NATS.ExternalStream.Name); err != nil {
		s.logger.Error("Failed to publish corporate message to external stream",
			zap.String("topic", topic),
			zap.String("message_id", messageID),
			zap.String("source", source),
			zap.String("external_topic", externalMsg.Topic),
			zap.Error(err))
		http.Error(w, `{"error": "Failed to publish message to external stream"}`, http.StatusInternalServerError)
		return
	}

	s.logger.Info("Corporate message published successfully to external stream",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("source", source),
		zap.String("client_sender", clientSender),
		zap.String("external_topic", externalMsg.Topic),
		zap.String("stream", s.config.NATS.ExternalStream.Name),
		zap.String("deployment", s.config.Deployment.Type))

	// Return success response
	response := map[string]interface{}{
		"status":     "delivered",
		"message_id": messageID,
		"topic":      topic,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"deployment": s.config.Deployment.Type,
		"flow":       "int_to_ext",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendHandler handles the /send endpoint according to OpenAPI specification
func (s *Server) sendHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Received send request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("deployment", s.config.Deployment.Type))

	if r.Method != http.MethodPost {
		s.logger.Warn("Invalid HTTP method for send endpoint",
			zap.String("method", r.Method))
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check LDAP authorization for sending messages
	if s.ldapMiddleware != nil && !s.ldapMiddleware.AuthorizeEndpoint(r, s.config.Deployment.Type, "send") {
		s.logger.Warn("LDAP authorization denied for send endpoint",
			zap.String("deployment", s.config.Deployment.Type))
		JSONError(w, "Access denied - insufficient permissions", http.StatusForbidden)
		return
	}

	// Get topic from query parameter
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		s.logger.Warn("Missing topic parameter in send request")
		JSONError(w, "topic parameter is required", http.StatusBadRequest)
		return
	}

	// Read message body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("Failed to read request body for send endpoint",
			zap.String("topic", topic),
			zap.Error(err))
		JSONError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Generate message ID
	messageID := nats.GenerateID()
	timestamp := time.Now().UTC()

	// Get client_sender from LDAP context
	clientSender := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientSender = userInfo["uid"]
	}

	// Create message headers
	headers := make(map[string]string)
	headers["X-SMTS-Message-ID"] = messageID
	headers["X-SMTS-Timestamp"] = timestamp.Format(time.RFC3339)
	headers["X-SMTS-Source"] = "rest-api"

	// Create message
	msg := &types.Message{
		ID:           messageID,
		Timestamp:    timestamp,
		Topic:        topic,
		Source:       "rest-api",
		ClientSender: clientSender,
		Headers:      headers,
		Body:         body,
	}

	// Publish message to NATS
	if err := s.publisher.PublishMessage(msg); err != nil {
		s.logger.Error("Failed to publish message via send endpoint",
			zap.String("topic", topic),
			zap.String("message_id", messageID),
			zap.Error(err))
		JSONError(w, "Failed to publish message to stream", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Message sent successfully via send endpoint",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("client_sender", clientSender))

	// Return success response according to OpenAPI spec
	response := map[string]interface{}{
		"result":           "ok",
		"X-SMTS-MessageId": messageID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-SMTS-Message-ID", messageID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// receiveHandler handles the /receive endpoint according to OpenAPI specification
func (s *Server) receiveHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Received receive request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("deployment", s.config.Deployment.Type))

	if r.Method != http.MethodGet {
		s.logger.Warn("Invalid HTTP method for receive endpoint",
			zap.String("method", r.Method))
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check LDAP authorization for reading messages
	if s.ldapMiddleware != nil && !s.ldapMiddleware.AuthorizeEndpoint(r, s.config.Deployment.Type, "read") {
		s.logger.Warn("LDAP authorization denied for receive endpoint",
			zap.String("deployment", s.config.Deployment.Type))
		JSONError(w, "Access denied - insufficient permissions", http.StatusForbidden)
		return
	}

	// Get topic from query parameter
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		s.logger.Warn("Missing topic parameter in receive request")
		JSONError(w, "topic parameter is required", http.StatusBadRequest)
		return
	}

	// Get count from query parameter (default to 1)
	countStr := r.URL.Query().Get("count")
	count := 1
	if countStr != "" {
		if parsedCount, err := strconv.Atoi(countStr); err == nil && parsedCount > 0 {
			count = parsedCount
			if count > 100 {
				count = 100
			}
		}
	}

	// Get client_receiver from LDAP context
	clientReceiver := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientReceiver = userInfo["uid"]
	}

	s.logger.Info("Processing receive request",
		zap.String("topic", topic),
		zap.Int("count", count),
		zap.String("client_receiver", clientReceiver))

	// Check if NATS client is available
	if s.natsClient == nil {
		s.logger.Error("NATS client not available for receive endpoint")
		JSONError(w, "Service not properly initialized", http.StatusInternalServerError)
		return
	}

	js := s.natsClient.GetJetStream()
	if js == nil {
		s.logger.Error("JetStream context not available")
		JSONError(w, "JetStream not available", http.StatusInternalServerError)
		return
	}

	// Read messages from NATS stream
	messages := make([]map[string]interface{}, 0)
	received := 0

	// Use the external consumer for reading messages
	consumerName := s.config.NATS.ExternalConsumer.DurableName
	streamName := s.config.NATS.ExternalStream.Name
	externalTopic := "external." + topic

	// Check if the consumer exists
	_, err := js.ConsumerInfo(streamName, consumerName)
	if err != nil {
		s.logger.Error("Failed to get consumer info for receive endpoint",
			zap.String("stream", streamName),
			zap.String("consumer", consumerName),
			zap.String("topic", topic),
			zap.Error(err))
		JSONError(w, "Failed to access message stream consumer", http.StatusInternalServerError)
		return
	}

	// Subscribe to messages
	sub, err := js.PullSubscribe(externalTopic, consumerName, natsio.Bind(streamName, consumerName))
	if err != nil {
		s.logger.Error("Failed to subscribe to messages for receive endpoint",
			zap.String("topic", topic),
			zap.String("external_topic", externalTopic),
			zap.String("consumer", consumerName),
			zap.Error(err))
		JSONError(w, "Failed to subscribe to messages", http.StatusInternalServerError)
		return
	}
	defer sub.Unsubscribe()

	// Fetch messages
	fetchedMsgs, err := sub.Fetch(count, natsio.MaxWait(5*time.Second))
	if err != nil && err != natsio.ErrTimeout {
		s.logger.Error("Failed to fetch messages for receive endpoint",
			zap.String("topic", topic),
			zap.Int("requested_count", count),
			zap.Error(err))
		JSONError(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	// Process fetched messages
	for _, msg := range fetchedMsgs {
		received++

		// Extract actual content from the message structure
		var msgData map[string]interface{}
		var actualBody interface{}
		var smtsHeaders map[string]interface{}
		
		if err := json.Unmarshal(msg.Data, &msgData); err == nil {
			// Debug: log the message structure to understand what we're dealing with
			s.logger.Debug("Processing message structure for receive endpoint",
				zap.Any("msg_data", msgData),
				zap.String("topic", topic))
			
			// Check if this is a nested message structure (from ArtemisMQ)
			if bodyField, exists := msgData["body"]; exists {
				// Check if the body field itself contains nested structure
				if nestedBody, ok := bodyField.(map[string]interface{}); ok {
					// This is a complex nested structure - check for inner body
					if innerBody, exists := nestedBody["body"]; exists {
						// Extract the actual content from the nested structure
						actualBody = innerBody
						// Extract headers from the nested structure if available
						if nestedHeaders, exists := nestedBody["headers"]; exists {
							if headersMap, ok := nestedHeaders.(map[string]interface{}); ok {
								smtsHeaders = headersMap
							}
						}
					} else {
						// Use the nested body as the actual content
						actualBody = bodyField
						// Extract headers from the nested structure if available
						if nestedHeaders, exists := nestedBody["headers"]; exists {
							if headersMap, ok := nestedHeaders.(map[string]interface{}); ok {
								smtsHeaders = headersMap
							}
						}
					}
				} else {
					// This is a simple nested message structure - extract the actual content
					actualBody = bodyField
				}
			} else if nestedBody, exists := msgData["Body"]; exists {
				// Check for capitalized Body field (from ArtemisMQ)
				actualBody = nestedBody
			} else {
				// This is a direct message - use the raw data as body
				actualBody = string(msg.Data)
			}
			
			// If we haven't found headers in the nested structure, check the outer message
			if smtsHeaders == nil {
				if headersField, exists := msgData["headers"]; exists {
					if headersMap, ok := headersField.(map[string]interface{}); ok {
						smtsHeaders = headersMap
					}
				}
			}
		} else {
			// If parsing fails, use the raw data
			actualBody = string(msg.Data)
		}

		// Create clean headers structure
		cleanHeaders := map[string]interface{}{
			"Content-Type": "application/json",
		}
		
		// Extract SMTS headers from the message data
		if smtsHeaders != nil {
			if messageID, exists := smtsHeaders["X-SMTS-Message-ID"]; exists {
				cleanHeaders["X-SMTS-Message-ID"] = messageID
			}
			if timestamp, exists := smtsHeaders["X-SMTS-Timestamp"]; exists {
				cleanHeaders["X-SMTS-Timestamp"] = timestamp
			}
			// Also check for lowercase variants
			if messageID, exists := smtsHeaders["x-smts-message-id"]; exists {
				cleanHeaders["X-SMTS-Message-ID"] = messageID
			}
			if timestamp, exists := smtsHeaders["x-smts-timestamp"]; exists {
				cleanHeaders["X-SMTS-Timestamp"] = timestamp
			}
		}
		
		// Fallback to NATS headers if SMTS headers are not found
		if cleanHeaders["X-SMTS-Message-ID"] == nil || cleanHeaders["X-SMTS-Message-ID"] == "" {
			if msg.Header != nil {
				if values := msg.Header.Values("X-SMTS-Message-ID"); len(values) > 0 {
					cleanHeaders["X-SMTS-Message-ID"] = values[0]
				}
			}
		}
		if cleanHeaders["X-SMTS-Timestamp"] == nil || cleanHeaders["X-SMTS-Timestamp"] == "" {
			if msg.Header != nil {
				if values := msg.Header.Values("X-SMTS-Timestamp"); len(values) > 0 {
					cleanHeaders["X-SMTS-Timestamp"] = values[0]
				}
			}
		}

		messageData := map[string]interface{}{
			"body":    actualBody,
			"headers": cleanHeaders,
			"topic":   topic,
		}

		messages = append(messages, messageData)

		// Acknowledge the message
		if err := msg.Ack(); err != nil {
			s.logger.Warn("Failed to acknowledge message in receive endpoint",
				zap.String("message_id", fmt.Sprintf("%v", cleanHeaders["X-SMTS-Message-ID"])),
				zap.String("topic", topic),
				zap.Error(err))
		}
	}

	s.logger.Info("Receive request completed successfully",
		zap.String("topic", topic),
		zap.Int("requested", count),
		zap.Int("received", received),
		zap.String("client_receiver", clientReceiver))

	// Return response according to OpenAPI spec
	response := map[string]interface{}{
		"api":       "receive-api",
		"count":     count,
		"messages":  messages,
		"received":  received,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"topic":     topic,
	}

	JSONSuccess(w, response, http.StatusOK)
}

// processedHandler handles the /processed endpoint according to OpenAPI specification
func (s *Server) processedHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Received processed request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("deployment", s.config.Deployment.Type))

	if r.Method != http.MethodPost {
		s.logger.Warn("Invalid HTTP method for processed endpoint",
			zap.String("method", r.Method))
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check LDAP authorization for processed endpoint
	if s.ldapMiddleware != nil && !s.ldapMiddleware.AuthorizeEndpoint(r, s.config.Deployment.Type, "send") {
		s.logger.Warn("LDAP authorization denied for processed endpoint",
			zap.String("deployment", s.config.Deployment.Type))
		JSONError(w, "Access denied - insufficient permissions", http.StatusForbidden)
		return
	}

	// Get topic from query parameter
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		s.logger.Warn("Missing topic parameter in processed request")
		JSONError(w, "topic parameter is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var request struct {
		Processed string `json:"processed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.logger.Error("Failed to parse processed request body",
			zap.String("topic", topic),
			zap.Error(err))
		JSONError(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	if request.Processed == "" {
		s.logger.Warn("Missing processed field in request body")
		JSONError(w, "processed field is required", http.StatusBadRequest)
		return
	}

	// Get client_sender from LDAP context
	clientSender := ""
	if userInfo, _ := userInfoFromContext(r.Context()); userInfo != nil {
		clientSender = userInfo["uid"]
	}

	s.logger.Info("Message processing confirmed",
		zap.String("topic", topic),
		zap.String("message_id", request.Processed),
		zap.String("client_sender", clientSender))

	// Return success response according to OpenAPI spec
	response := map[string]interface{}{
		"result": "ok",
	}

	JSONSuccess(w, response, http.StatusOK)
}

// Run starts the server and waits for shutdown
func Run(configPath string) error {
	server, err := NewServer(configPath)
	if err != nil {
		return err
	}

	// Start the server
	if err := server.Start(); err != nil {
		return err
	}

	// Wait for shutdown signals
	server.WaitForShutdown()

	return nil
}

// QuickHealthCheck performs a quick health check without starting the full server
func QuickHealthCheck(configPath string) error {
	logger, err := utils.NewLogger("info", "json", "stdout")
	if err != nil {
		return err
	}

	// Load configuration
	configLoader := config.NewLoader(logger)
	cfg, err := configLoader.LoadConfig(configPath)
	if err != nil {
		return err
	}

	// Create NATS client for health check
	natsClient, err := nats.NewClient(&cfg.NATS, logger)
	if err != nil {
		return err
	}
	defer natsClient.Close()

	// Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := natsClient.HealthCheck(ctx); err != nil {
		return err
	}

	logger.Info("Quick health check passed")
	return nil
}

// createExternalConsumer creates the external consumer without starting the processing loop
func (s *Server) createExternalConsumer() error {
	if s.externalConsumer == nil {
		return nil
	}

	// Start the external consumer briefly to create it, then stop it immediately
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.externalConsumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to create external consumer: %w", err)
	}

	// Stop the consumer immediately to avoid processing loop
	if err := s.externalConsumer.Stop(); err != nil {
		s.logger.Warn("Failed to stop external consumer after creation", zap.Error(err))
	}

	s.logger.Info("External consumer created for message API",
		zap.String("stream", s.config.NATS.ExternalStream.Name),
		zap.String("consumer", s.config.NATS.ExternalConsumer.DurableName))

	return nil
}

// RunServer is a shared utility function that handles the common main logic
// for both EXT and INT SMTS servers
func RunServer(configPath string, serverType string) error {
	// Parse command line flags
	healthCheck := flag.Bool("health-check", false, "Perform quick health check and exit")
	flag.Parse()

	// Handle health check mode
	if *healthCheck {
		if err := QuickHealthCheck(configPath); err != nil {
			fmt.Printf("Health check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Health check passed")
		os.Exit(0)
	}

	// Run the server
	if err := Run(configPath); err != nil {
		fmt.Printf("Failed to start %s SMTS server: %v\n", strings.ToUpper(serverType), err)
		os.Exit(1)
	}

	return nil
}
