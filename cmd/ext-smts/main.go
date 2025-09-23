package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/corporate/smts/internal/server"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "./configs/ext-config.yaml", "Path to configuration file")
	healthCheck := flag.Bool("health-check", false, "Perform quick health check and exit")
	flag.Parse()

	// Handle health check mode
	if *healthCheck {
		if err := server.QuickHealthCheck(*configPath); err != nil {
			fmt.Printf("Health check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Health check passed")
		os.Exit(0)
	}

	// Run the server
	if err := server.Run(*configPath); err != nil {
		fmt.Printf("Failed to start EXT SMTS server: %v\n", err)
		os.Exit(1)
	}
}