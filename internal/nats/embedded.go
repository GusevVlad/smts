package nats

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/corporate/smts/pkg/types"
	"github.com/nats-io/nats-server/v2/server"
	"go.uber.org/zap"
)

// EmbeddedServer represents an embedded NATS server
type EmbeddedServer struct {
	server *server.Server
	config *types.NATSConfig
	logger *zap.Logger
}

// NewEmbeddedServer creates and starts a new embedded NATS server
func NewEmbeddedServer(config *types.NATSConfig, logger *zap.Logger) (*EmbeddedServer, error) {
	// Create server options
	opts := &server.Options{
		Host: config.Host,
		Port: config.Port,
		JetStream: true,
		StoreDir:  getStoreDir(config),
	}

	// Configure JetStream
	opts.JetStreamMaxMemory = 1024 * 1024 * 1024 // 1GB
	opts.JetStreamMaxStore = 1024 * 1024 * 1024 * 10 // 10GB

	// Create and configure the server
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, types.WrapSMTSError(err, types.ErrSystemStartup, "Failed to create embedded NATS server")
	}

	embeddedServer := &EmbeddedServer{
		server: s,
		config: config,
		logger: logger,
	}

	// Configure server logging
	embeddedServer.configureLogging()

	// Start the server
	if err := embeddedServer.Start(); err != nil {
		return nil, err
	}

	return embeddedServer, nil
}

// getStoreDir returns the directory for JetStream storage
func getStoreDir(config *types.NATSConfig) string {
	// Use current directory if no specific store directory is configured
	storeDir := "./data/nats"
	
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		// Fallback to temp directory
		storeDir = filepath.Join(os.TempDir(), "smts-nats")
		os.MkdirAll(storeDir, 0755)
	}
	
	return storeDir
}

// configureLogging sets up server logging
func (s *EmbeddedServer) configureLogging() {
	// Configure the server's logger to use our structured logger
	s.server.SetLogger(&embeddedLogger{logger: s.logger}, false, false)
}

// Start starts the embedded NATS server
func (s *EmbeddedServer) Start() error {
	// Start the server
	go s.server.Start()

	// Wait for server to be ready
	if !s.server.ReadyForConnections(10) {
		return types.NewSMTSError(types.ErrSystemStartup, "Embedded NATS server failed to start within timeout")
	}

	s.logger.Info("Embedded NATS server started",
		zap.String("host", s.config.Host),
		zap.Int("port", s.config.Port),
		zap.String("store_dir", getStoreDir(s.config)))

	return nil
}

// Stop stops the embedded NATS server
func (s *EmbeddedServer) Stop() {
	if s.server != nil {
		s.server.Shutdown()
		s.logger.Info("Embedded NATS server stopped")
	}
}

// GetServer returns the underlying NATS server instance
func (s *EmbeddedServer) GetServer() *server.Server {
	return s.server
}

// IsRunning returns true if the server is running
func (s *EmbeddedServer) IsRunning() bool {
	return s.server != nil && s.server.Running()
}

// GetConnectionURL returns the connection URL for the embedded server
func (s *EmbeddedServer) GetConnectionURL() string {
	return fmt.Sprintf("nats://%s:%d", s.config.Host, s.config.Port)
}

// HealthCheck performs a health check on the embedded server
func (s *EmbeddedServer) HealthCheck() error {
	if !s.IsRunning() {
		return types.NewSMTSError(types.ErrHealthCheck, "Embedded NATS server is not running")
	}

	// Check if server is still accepting connections
	if !s.server.Running() {
		return types.NewSMTSError(types.ErrHealthCheck, "Embedded NATS server is not running")
	}

	return nil
}

// embeddedLogger adapts our structured logger to NATS server's logger interface
type embeddedLogger struct {
	logger *zap.Logger
}

// Noticef logs notice level messages
func (l *embeddedLogger) Noticef(format string, v ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, v...))
}

// Warnf logs warning level messages
func (l *embeddedLogger) Warnf(format string, v ...interface{}) {
	l.logger.Warn(fmt.Sprintf(format, v...))
}

// Fatalf logs fatal level messages
func (l *embeddedLogger) Fatalf(format string, v ...interface{}) {
	l.logger.Fatal(fmt.Sprintf(format, v...))
}

// Errorf logs error level messages
func (l *embeddedLogger) Errorf(format string, v ...interface{}) {
	l.logger.Error(fmt.Sprintf(format, v...))
}

// Debugf logs debug level messages
func (l *embeddedLogger) Debugf(format string, v ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, v...))
}

// Tracef logs trace level messages
func (l *embeddedLogger) Tracef(format string, v ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, v...)) // Map trace to debug
}