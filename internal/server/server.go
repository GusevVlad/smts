package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/corporate/smts/internal/api"
	"github.com/corporate/smts/internal/artemis"
	"github.com/corporate/smts/internal/config"
	"github.com/corporate/smts/internal/message"
	"github.com/corporate/smts/internal/nats"
	"github.com/corporate/smts/pkg/types"
	"github.com/corporate/smts/pkg/utils"
	"go.uber.org/zap"
)

// Server represents the main SMTS server
type Server struct {
	config        *types.Config
	logger        *zap.Logger
	natsClient    *nats.Client
	apiClient     *api.Client
	artemisClient *artemis.Client
	processor     *message.Processor
	consumer      *nats.Consumer
	healthServer  *HealthServer
	running       bool
}

// NewServer creates a new SMTS server
func NewServer(configPath string) (*Server, error) {
	// Create logger
	logger, err := utils.NewLogger("info", "json", "stdout")
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Load configuration
	configLoader := config.NewLoader(logger)
	cfg, err := configLoader.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Load topics configuration
	topicsConfig, err := configLoader.LoadTopicsConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load topics configuration: %w", err)
	}
	cfg.Topics = *topicsConfig

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

	// Create health server
	healthServer := NewHealthServer(cfg, logger)

	server := &Server{
		config:        cfg,
		logger:        logger,
		natsClient:    natsClient,
		apiClient:     apiClient,
		artemisClient: artemisClient,
		processor:     processor,
		consumer:      consumer,
		healthServer:  healthServer,
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

	// Start health server
	if err := s.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
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