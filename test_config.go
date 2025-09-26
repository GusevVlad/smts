package main

import (
	"fmt"
	"os"

	"github.com/corporate/smts/internal/config"
	"github.com/corporate/smts/pkg/utils"
)

func main() {
	// Create a simple logger
	logger, err := utils.NewLogger("debug", "console", "stdout")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	// Test different config paths
	paths := []string{
		"/app/tests/configs/ext-test-config.yaml",
		"/app/tests/configs/int-test-config.yaml",
		"../configs/ext-test-config.yaml",
		"../configs/int-test-config.yaml",
		"tests/configs/ext-test-config.yaml",
		"tests/configs/int-test-config.yaml",
	}

	for _, path := range paths {
		fmt.Printf("Testing path: %s\n", path)
		
		// Check if file exists
		if _, err := os.Stat(path); err != nil {
			fmt.Printf("  File not found: %v\n", err)
			continue
		}
		
		fmt.Printf("  File exists\n")
		
		// Try to load configuration
		loader := config.NewLoader(logger)
		cfg, err := loader.LoadConfig(path)
		if err != nil {
			fmt.Printf("  Failed to load config: %v\n", err)
		} else {
			fmt.Printf("  Config loaded successfully: %s\n", cfg.Deployment.Type)
			fmt.Printf("  Topics count: %d\n", len(cfg.Topics.Topics))
			fmt.Printf("  Roles count: %d\n", len(cfg.Topics.Roles))
		}
		fmt.Println()
	}
}