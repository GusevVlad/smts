package main

import (
	"flag"
	"os"

	"smts/internal/server"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "./configs/ext-config.yaml", "Path to configuration file")
	flag.Parse()

	// Use shared server runner
	if err := server.RunServer(*configPath, "ext"); err != nil {
		os.Exit(1)
	}
}