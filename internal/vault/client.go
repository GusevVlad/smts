package vault

import (
	"context"
	"fmt"
	"os"
	"time"

	"smts/pkg/types"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"go.uber.org/zap"
)

// Client wraps the Vault client and configuration.
type Client struct {
	client *vault.Client
	cfg    *types.Config
	logger *zap.Logger
}

// NewClient creates a new Vault client with AppRole authentication.
// If RoleID and SecretID are empty, it will try to use VAULT_TOKEN environment variable.
func NewClient(cfg *types.Config, logger *zap.Logger) (*Client, error) {
	ctx := context.Background()

	client, err := vault.New(
		vault.WithAddress(cfg.Vault.Address),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %v", err)
	}

	// If RoleID and SecretID are empty, try to use VAULT_TOKEN
	if cfg.Vault.RoleID == "" && cfg.Vault.SecretID == "" {
		token := os.Getenv("VAULT_TOKEN")
		if token != "" {
			if err := client.SetToken(token); err != nil {
				return nil, fmt.Errorf("failed to set vault token: %v", err)
			}
			logger.Info("Using VAULT_TOKEN for authentication")
			return &Client{
				client: client,
				cfg:    cfg,
				logger: logger,
			}, nil
		}
		// If no token, fallback to AppRole with empty credentials (will fail)
	}

	// AppRole login
	resp, err := client.Auth.AppRoleLogin(
		ctx,
		schema.AppRoleLoginRequest{
			RoleId:   cfg.Vault.RoleID,
			SecretId: cfg.Vault.SecretID,
		},
		vault.WithMountPath("approle"), // default mount path
	)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with vault: %v", err)
	}

	if err := client.SetToken(resp.Auth.ClientToken); err != nil {
		return nil, fmt.Errorf("failed to set vault token: %v", err)
	}

	return &Client{
		client: client,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// FetchSecrets retrieves secrets from Vault KV v1 at the path constructed from config.
func (c *Client) FetchSecrets() (map[string]interface{}, error) {
	ctx := context.Background()

	// Build secret path: pathPrefix/deploymentType/environment
	secretPath := fmt.Sprintf("%s/%s/%s",
		c.cfg.Vault.PathPrefix,
		c.cfg.Deployment.Type,
		c.cfg.Deployment.Environment,
	)

	c.logger.Info("Fetching secrets from Vault", zap.String("path", secretPath))

	secret, err := c.client.Secrets.KvV1Read(
		ctx,
		secretPath,
		vault.WithMountPath("secret"), // default KV v1 mount
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets from vault: %v", err)
	}

	return secret.Data, nil
}

// ApplySecrets updates the configuration with secrets from Vault.
// Currently updates API key, Artemis credentials, and LDAP bind password.
func (c *Client) ApplySecrets(data map[string]interface{}) error {
	// API key
	if val, ok := data["API_KEY"].(string); ok && val != "" {
		c.cfg.API.Auth.APIKey = val
		c.logger.Debug("Set API_KEY from Vault")
	}

	// Client credentials (if using client_credentials)
	if val, ok := data["CLIENT_ID"].(string); ok && val != "" {
		c.cfg.API.Auth.ClientID = val
	}
	if val, ok := data["CLIENT_SECRET"].(string); ok && val != "" {
		c.cfg.API.Auth.ClientSecret = val
	}
	if val, ok := data["TOKEN_URL"].(string); ok && val != "" {
		c.cfg.API.Auth.TokenURL = val
	}

	// Artemis credentials
	if val, ok := data["ARTEMIS_USER"].(string); ok && val != "" {
		c.cfg.Artemis.Username = val
	}
	if val, ok := data["ARTEMIS_PASSWORD"].(string); ok && val != "" {
		c.cfg.Artemis.Password = val
	}

	// LDAP bind password
	if val, ok := data["LDAP_BIND_PASSWORD"].(string); ok && val != "" {
		c.cfg.LDAP.BindPassword = val
	}

	// Add more secret mappings as needed

	c.logger.Info("Secrets applied to configuration")
	return nil
}

// SetDbCredentials is a legacy method that fetches and applies secrets.
func (c *Client) SetDbCredentials() error {
	data, err := c.FetchSecrets()
	if err != nil {
		return fmt.Errorf("failed to fetch secrets: %v", err)
	}
	return c.ApplySecrets(data)
}
