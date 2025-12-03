#!/bin/bash
# setup-ext-vault-secrets.sh
# Helper script to set up External SMTS Vault secret files

set -e

echo "=== External SMTS Vault Secret Setup ==="
echo "This script helps create secret files for Vault AppRole authentication."
echo "These files should NEVER be committed to git."
echo ""

# Create secrets directory if it doesn't exist
mkdir -p secrets

# Function to prompt for secret value
prompt_secret() {
    local name=$1
    local file=$2
    local default=$3
    
    read -sp "Enter $name (default: $default): " value
    echo
    if [ -z "$value" ]; then
        value="$default"
    fi
    echo "$value" > "$file"
    chmod 600 "$file"
    echo "  Saved to $file"
}

echo "--- External SMTS Vault Credentials ---"
prompt_secret "EXT_VAULT_ROLE_ID" "secrets/ext-vault-role-id" "ext-role-id-here"
prompt_secret "EXT_VAULT_SECRET_ID" "secrets/ext-vault-secret-id" "ext-secret-id-here"

echo ""
echo "=== Summary ==="
echo "Secret files created:"
ls -la secrets/ext-vault-*
echo ""
echo "To use these secrets:"
echo "1. Set environment variables:"
echo "   export EXT_VAULT_ROLE_ID=\$(cat secrets/ext-vault-role-id)"
echo "   export EXT_VAULT_SECRET_ID=\$(cat secrets/ext-vault-secret-id)"
echo ""
echo "2. Or use with Docker Compose (already configured):"
echo "   docker-compose -f docker-compose.ext-prod.yml up -d"
echo ""
echo "Remember:"
echo "- These files are listed in .gitignore and should not be committed"
echo "- For production, use a proper secret management system"
echo "- Rotate secrets regularly"