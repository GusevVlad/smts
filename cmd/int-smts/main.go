package main

import (
	"flag"
	"os"

	"smts/internal/server"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "./configs/int-config.yaml", "Path to configuration file")
	flag.Parse()

	// Use shared server runner
	if err := server.RunServer(*configPath, "int"); err != nil {
		os.Exit(1)
	}
}