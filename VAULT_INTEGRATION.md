# Vault Integration for SMTS

This document describes how to migrate from `.env` files to HashiCorp Vault for secret management in SMTS.

## Overview

The SMTS application now supports HashiCorp Vault for secret management using AppRole authentication. This provides a more secure way to manage secrets compared to storing them in `.env` files.

## Changes Made

### 1. Configuration Updates

- **`pkg/types/config.go`**: Added `VaultConfig` struct to hold Vault configuration
- **`configs/ext-config.yaml`** and **`configs/int-config.yaml`**: Added Vault configuration section with environment variable placeholders
- **`.env.ext-prod`** and **`.env.int-prod`**: Removed secrets, now only contain non-sensitive configuration
- **`internal/config/config.go`**: Updated to load Vault credentials from environment variables and secret files

### 2. Docker Compose Updates

- **`docker-compose.ext-prod.yml`** and **`docker-compose.int-prod.yml`**: Added Vault environment variables and secret file mounts
- Added support for Docker Swarm secrets and Kubernetes secrets

### 3. Security Improvements

- **`.gitignore`**: Added patterns to prevent committing secret files
- **Example files**: Created `configs/.env.*.example` files to show configuration structure
- **Setup scripts**: Created helper scripts to generate secret files

## How to Use

### Prerequisites

1. HashiCorp Vault instance running (local or remote)
2. AppRole authentication enabled in Vault
3. Appropriate policies configured for SMTS

### Setting Up Secrets

#### Option 1: Using Environment Variables

```bash
# Set environment variables
export EXT_VAULT_ROLE_ID="your-ext-role-id"
export EXT_VAULT_SECRET_ID="your-ext-secret-id"
export INT_VAULT_ROLE_ID="your-int-role-id"
export INT_VAULT_SECRET_ID="your-int-secret-id"
export VAULT_ADDR="http://vault:8200"

# Start services
docker-compose -f docker-compose.ext-prod.yml up -d
docker-compose -f docker-compose.int-prod.yml up -d
```

#### Option 2: Using Secret Files

1. Run the setup scripts:

```bash
# For External SMTS
./scripts/setup-ext-vault-secrets.sh

# For Internal SMTS
./scripts/setup-int-vault-secrets.sh
```

2. The scripts will create secret files in the `secrets/` directory
3. Docker Compose is already configured to mount these files

#### Option 3: Docker Swarm/Kubernetes Secrets

- For Docker Swarm: Use `docker secret create`
- For Kubernetes: Use Kubernetes Secrets mounted as files

### Local Development with Vault

For local development, you can run a Vault dev server:

```bash
# Add to docker-compose.override.yml
version: '3.8'
services:
  vault:
    image: hashicorp/vault:1.15.0
    ports:
      - "8200:8200"
    environment:
      - VAULT_DEV_ROOT_TOKEN_ID=dev-token-123
      - VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200
    cap_add:
      - IPC_LOCK
```

### Migrating Existing Secrets

1. Move secrets from `.env` files to Vault:

```bash
# Example: Migrate API key
vault kv put smts/ext/production API_KEY="your-api-key"

# Example: Migrate Artemis credentials
vault kv put smts/int/production \
  ARTEMIS_USER="admin" \
  ARTEMIS_PASSWORD="secure-password" \
  API_KEY="your-api-key"
```

2. Update the application to read secrets from Vault (future implementation)

## Configuration Reference

### Vault Configuration in YAML

```yaml
vault:
  enabled: true
  address: "${VAULT_ADDR:-http://vault:8200}"
  role_id: "${VAULT_ROLE_ID}"
  secret_id: "${VAULT_SECRET_ID}"
  path_prefix: "smts"
  auto_renew: true
  renewal_threshold: "5m"
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `VAULT_ADDR` | Vault server address | No (default: `http://vault:8200`) |
| `VAULT_ROLE_ID` | AppRole Role ID | Yes (if Vault enabled) |
| `VAULT_SECRET_ID` | AppRole Secret ID | Yes (if Vault enabled) |
| `EXT_VAULT_ROLE_ID` | External SMTS Role ID | For Docker Compose |
| `EXT_VAULT_SECRET_ID` | External SMTS Secret ID | For Docker Compose |
| `INT_VAULT_ROLE_ID` | Internal SMTS Role ID | For Docker Compose |
| `INT_VAULT_SECRET_ID` | Internal SMTS Secret ID | For Docker Compose |

### Secret File Paths

- `/run/secrets/vault-role-id`: Role ID file (Docker Swarm/Kubernetes compatible)
- `/run/secrets/vault-secret-id`: Secret ID file (Docker Swarm/Kubernetes compatible)

## Security Best Practices

1. **Never commit secret files to git**: All secret files are listed in `.gitignore`
2. **Use different credentials per environment**: Development, staging, and production should have separate Role IDs
3. **Rotate secrets regularly**: AppRole Secret IDs should be rotated periodically
4. **Use least privilege**: Configure Vault policies to grant only necessary permissions
5. **Audit logging**: Enable audit logging in Vault to track secret access

## Next Steps

The current implementation provides the configuration framework for Vault integration. Future work includes:

1. Implementing the actual Vault client to fetch secrets
2. Adding secret rotation support
3. Implementing fallback mechanisms for development without Vault
4. Adding integration tests with Vault mock

## Troubleshooting

### Vault Connection Issues

1. Check `VAULT_ADDR` is correct and accessible
2. Verify Role ID and Secret ID are valid
3. Check Vault logs for authentication errors

### Configuration Issues

1. Ensure YAML configuration files are valid
2. Check environment variables are set correctly
3. Verify secret files have correct permissions (600)

### Docker Issues

1. Ensure secret files exist if using file mounts
2. Check Docker Compose version supports the syntax used
3. Verify volumes are mounted correctly

## Support

For issues with Vault integration, check:
- HashiCorp Vault documentation
- SMTS issue tracker
- Configuration examples in `configs/.env.*.example` files