package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// GetEnvWithDefault gets an environment variable with a default value
func GetEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// HealthHandler creates a standard health check handler
func HealthHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "healthy", "service": "%s", "timestamp": "%s"}`,
			serviceName, time.Now().UTC().Format(time.RFC3339))
	}
}

// StartServer starts an HTTP server with the given configuration
func StartServer(port string, serviceName string, routes map[string]http.HandlerFunc) error {
	// Set up routes
	mux := http.NewServeMux()
	for path, handler := range routes {
		mux.HandleFunc(path, handler)
	}

	// Log startup information
	log.Printf("%s server starting on port %s", serviceName, port)
	log.Printf("Available endpoints:")
	for path := range routes {
		log.Printf("  %s", path)
	}

	// Start server
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		return fmt.Errorf("failed to start %s server: %w", serviceName, err)
	}

	return nil
}