package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
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
	"go.uber.org/zap"
)

// Server represents the main SMTS server
type Server struct {
	config          *types.Config
	logger          *zap.Logger
	natsClient      *nats.Client
	apiClient       *api.Client
	artemisClient   *artemis.Client
	processor       *message.Processor
	consumer        *nats.Consumer
	healthServer    *HealthServer
	messageAPIServer *MessageAPIServer
	publisher       *nats.Publisher
	httpServer      *http.Server
	running         bool
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
	if len(cfg.Topics.Topics) == 0 && len(cfg.Topics.Roles) == 0 {
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
	apiClient := api.NewClient(&cfg.API, logger)

	// Create message processor
	processor := message.NewProcessor(cfg, apiClient, natsClient, logger)

	// Create Artemis client for INT deployment
	var artemisClient *artemis.Client
	if cfg.Deployment.Type == "int" && cfg.Artemis.Enabled {
		artemisClient, err = artemis.NewClient(&cfg.Artemis, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Artemis client: %w", err)
		}
	}

	// Create NATS consumer
	consumer := nats.NewConsumer(natsClient, cfg.NATS.Stream.Name, &cfg.NATS.Consumer, processor, logger)

	// Create NATS publisher
	publisher := nats.NewPublisher(natsClient, logger)

	// Create health server
	healthServer := NewHealthServer(cfg, logger)

	// Create message API server
	messageAPIServer := NewMessageAPIServer(cfg, logger)
	messageAPIServer.SetNATSClient(natsClient)

	server := &Server{
		config:          cfg,
		logger:          logger,
		natsClient:      natsClient,
		apiClient:       apiClient,
		artemisClient:   artemisClient,
		processor:       processor,
		consumer:        consumer,
		publisher:       publisher,
		healthServer:    healthServer,
		messageAPIServer: messageAPIServer,
	}

	return server, nil
}

// Start starts the SMTS server
func (s *Server) Start() error {
	s.logger.Info("Starting SMTS server",
		zap.String("deployment", s.config.Deployment.Type),
		zap.String("environment", s.config.Deployment.Environment))

	// Create stream if it doesn't exist
	if err := s.natsClient.CreateStream(&s.config.NATS.Stream); err != nil {
		return fmt.Errorf("failed to create NATS stream: %w", err)
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

	// Start NATS consumer
	ctx := context.Background()
	if err := s.consumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start NATS consumer: %w", err)
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

	// Stop NATS consumer
	if s.consumer != nil {
		if err := s.consumer.Stop(); err != nil {
			s.logger.Error("Error stopping NATS consumer", zap.Error(err))
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
	
	// Add message handler for all topics (topic is extracted from URL path)
	mux.HandleFunc("/", s.messageHandler)
	
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
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract topic from URL path
	topic := r.URL.Path[1:] // Remove leading slash
	if topic == "" {
		http.Error(w, `{"error": "Topic is required in URL path"}`, http.StatusBadRequest)
		return
	}

	// Check authorization headers
	apiKey := r.Header.Get("X-API-Key")
	role := r.Header.Get("smts-role")
	messageID := r.Header.Get("X-SMTS-Message-ID")
	timestamp := r.Header.Get("X-SMTS-Timestamp")
	source := r.Header.Get("X-SMTS-Source")

	// Validate required headers
	if apiKey == "" {
		http.Error(w, `{"error": "X-API-Key header is required"}`, http.StatusUnauthorized)
		return
	}

	if role == "" {
		http.Error(w, `{"error": "smts-role header is required"}`, http.StatusUnauthorized)
		return
	}

	// Check if topic exists in configuration (basic validation)
	if _, exists := s.config.Topics.Topics[topic]; !exists {
		s.logger.Warn("Topic not found in configuration",
			zap.String("topic", topic),
			zap.String("role", role))
		http.Error(w, `{"error": "Topic not found in configuration"}`, http.StatusBadRequest)
		return
	}

	// Read message body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", zap.Error(err))
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse timestamp
	var msgTimestamp time.Time
	if timestamp != "" {
		msgTimestamp, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			s.logger.Warn("Invalid timestamp format, using current time",
				zap.String("timestamp", timestamp),
				zap.Error(err))
			msgTimestamp = time.Now().UTC()
		}
	} else {
		msgTimestamp = time.Now().UTC()
	}

	// Generate message ID if not provided
	if messageID == "" {
		messageID = nats.GenerateID()
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
	headers["X-SMTS-Role"] = role
	headers["X-SMTS-Source"] = source

	// Create message
	msg := &types.Message{
		ID:        messageID,
		Timestamp: msgTimestamp,
		Topic:     topic,
		Source:    source,
		Headers:   headers,
		Body:      body,
	}

	// Publish message to NATS
	if err := s.publisher.PublishMessage(msg); err != nil {
		s.logger.Error("Failed to publish message",
			zap.String("topic", topic),
			zap.String("message_id", messageID),
			zap.Error(err))
		http.Error(w, `{"error": "Failed to publish message to stream"}`, http.StatusInternalServerError)
		return
	}

	s.logger.Info("Message published successfully",
		zap.String("topic", topic),
		zap.String("message_id", messageID),
		zap.String("source", source),
		zap.String("role", role))

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