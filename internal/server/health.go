package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"smts/pkg/types"
	"go.uber.org/zap"
)

// HealthServer handles health checks and metrics
type HealthServer struct {
	config   *types.Config
	logger   *zap.Logger
	server   *http.Server
	healthy  bool
	mu       sync.RWMutex
	startTime time.Time
}

// NewHealthServer creates a new health server
func NewHealthServer(config *types.Config, logger *zap.Logger) *HealthServer {
	return &HealthServer{
		config:    config,
		logger:    logger,
		healthy:   false,
		startTime: time.Now(),
	}
}

// Start starts the health server
func (h *HealthServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(h.config.Health.Path, h.healthHandler)
	mux.HandleFunc("/metrics", h.metricsHandler)
	mux.HandleFunc("/ready", h.readyHandler)
	mux.HandleFunc("/live", h.liveHandler)

	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.config.Health.Port),
		Handler: mux,
	}

	go func() {
		h.logger.Info("Starting health server", 
			zap.Int("port", h.config.Health.Port),
			zap.String("path", h.config.Health.Path))

		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("Health server failed", zap.Error(err))
		}
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Stop stops the health server
func (h *HealthServer) Stop() error {
	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown health server: %w", err)
		}
	}

	h.logger.Info("Health server stopped")
	return nil
}

// SetHealthy sets the health status of the server
func (h *HealthServer) SetHealthy(healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthy = healthy
}

// IsHealthy returns the current health status
func (h *HealthServer) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.healthy
}

// healthHandler handles the main health check endpoint
func (h *HealthServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	healthy := h.IsHealthy()
	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    getStatusText(healthy),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
		"version":   "1.0.0",
		"deployment": map[string]string{
			"type":        h.config.Deployment.Type,
			"name":        h.config.Deployment.Name,
			"environment": h.config.Deployment.Environment,
		},
	}

	// Add health check results
	healthChecks := h.performHealthChecks(r.Context())
	response["checks"] = healthChecks

	JSONSuccess(w, response, status)
}

// metricsHandler handles metrics endpoint (placeholder for future metrics)
func (h *HealthServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# SMTS Metrics\n# This endpoint will provide metrics in the future\n"))
}

// readyHandler handles readiness probe
func (h *HealthServer) readyHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	healthy := h.IsHealthy()
	if healthy {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("NOT READY"))
	}
}

// liveHandler handles liveness probe
func (h *HealthServer) liveHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Liveness check is simpler - just check if the process is running
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ALIVE"))
}

// performHealthChecks performs various health checks
func (h *HealthServer) performHealthChecks(ctx context.Context) map[string]interface{} {
	checks := make(map[string]interface{})

	// Basic process health
	checks["process"] = map[string]interface{}{
		"status":  "healthy",
		"details": "Process is running",
	}

	// Uptime check
	uptime := time.Since(h.startTime)
	checks["uptime"] = map[string]interface{}{
		"status":  "healthy",
		"details": uptime.String(),
		"seconds": uptime.Seconds(),
	}

	// Overall health status
	healthy := h.IsHealthy()
	checks["overall"] = map[string]interface{}{
		"status":  getStatusText(healthy),
		"details": "Overall system health",
	}

	return checks
}

// getStatusText returns the status text for a boolean health value
func getStatusText(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}