#!/bin/bash
# setup-int-vault-secrets.sh
# Helper script to set up Internal SMTS Vault secret files

set -e

echo "=== Internal SMTS Vault Secret Setup ==="
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

echo "--- Internal SMTS Vault Credentials ---"
prompt_secret "INT_VAULT_ROLE_ID" "secrets/int-vault-role-id" "int-role-id-here"
prompt_secret "INT_VAULT_SECRET_ID" "secrets/int-vault-secret-id" "int-secret-id-here"

echo ""
echo "=== Summary ==="
echo "Secret files created:"
ls -la secrets/int-vault-*
echo ""
echo "To use these secrets:"
echo "1. Set environment variables:"
echo "   export INT_VAULT_ROLE_ID=\$(cat secrets/int-vault-role-id)"
echo "   export INT_VAULT_SECRET_ID=\$(cat secrets/int-vault-secret-id)"
echo ""
echo "2. Or use with Docker Compose (already configured):"
echo "   docker-compose -f docker-compose.int-prod.yml up -d"
echo ""
echo "Remember:"
echo "- These files are listed in .gitignore and should not be committed"
echo "- For production, use a proper secret management system"
echo "- Rotate secrets regularly"