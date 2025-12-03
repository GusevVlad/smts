package vault

import (
	"smts/pkg/types"

	"go.uber.org/zap"
)

// SetupVault initializes Vault client and fetches secrets if Vault is enabled.
// It updates the configuration in place.
func SetupVault(cfg *types.Config, logger *zap.Logger) {
	if !cfg.Vault.Enabled {
		logger.Info("Vault is disabled, skipping secret retrieval")
		return
	}

	logger.Info("Vault is enabled, initializing client")

	client, err := NewClient(cfg, logger)
	if err != nil {
		logger.Error("Failed to create Vault client", zap.Error(err))
		return
	}

	if err := client.SetDbCredentials(); err != nil {
		logger.Error("Failed to fetch secrets from Vault", zap.Error(err))
		return
	}

	logger.Info("Vault secrets successfully loaded")
}
