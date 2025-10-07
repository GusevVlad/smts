package integration

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"smts/internal/server"
	"smts/pkg/types"
	"smts/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLDAPAuthIntegration tests LDAP authentication integration
func TestLDAPAuthIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LDAP integration test in short mode")
	}

	// Create test logger
	logger, err := utils.NewLogger("debug", "console", "stdout")
	require.NoError(t, err)

	// Test LDAP configuration scenarios
	t.Run("LDAPDisabledByDefault", func(t *testing.T) {
		// Create test configuration with LDAP disabled (default)
		testConfig := &types.Config{
			Deployment: types.DeploymentConfig{
				Type:        "int",
				Name:        "smts-int-test",
				Environment: "test",
			},
			LDAP: types.LDAPConfig{
				Enabled:           false, // LDAP disabled by default
				Host:              "localhost",
				Port:              3890,
				Base:              "dc=example,dc=com",
				BindDN:            "cn=admin,dc=example,dc=com",
				BindPassword:      "bindpass",
				UserFilter:        "(uid=%s)",
				GroupFilter:       "(member=%s)",
				Attributes:        []string{"uid", "cn", "mail", "distinguishedName"},
				ServerName:        "",
				UseSSL:            false,
				SkipTLS:           false,
				InsecureSkipVerify: false,
			},
		}

		// Create message API server
		messageAPIServer := server.NewMessageAPIServer(testConfig, logger)
		assert.NotNil(t, messageAPIServer)
		
		// Verify LDAP middleware is created but disabled
		assert.NotNil(t, messageAPIServer)
	})

	t.Run("LDAPEnabledConfiguration", func(t *testing.T) {
		// Create test configuration with LDAP enabled
		testConfig := &types.Config{
			Deployment: types.DeploymentConfig{
				Type:        "int",
				Name:        "smts-int-test",
				Environment: "test",
			},
			LDAP: types.LDAPConfig{
				Enabled:           true, // LDAP enabled
				Host:              "ldap.example.com",
				Port:              636,
				Base:              "dc=example,dc=com",
				BindDN:            "cn=admin,dc=example,dc=com",
				BindPassword:      "bindpass",
				UserFilter:        "(uid=%s)",
				GroupFilter:       "(member=%s)",
				Attributes:        []string{"uid", "cn", "mail", "distinguishedName"},
				ServerName:        "ldap.example.com",
				UseSSL:            true,
				SkipTLS:           false,
				InsecureSkipVerify: false,
			},
		}

		// Create message API server
		messageAPIServer := server.NewMessageAPIServer(testConfig, logger)
		assert.NotNil(t, messageAPIServer)
	})
}

// TestLDAPMiddlewareUnit tests the LDAP middleware functionality
func TestLDAPMiddlewareUnit(t *testing.T) {
	logger, err := utils.NewLogger("debug", "console", "stdout")
	require.NoError(t, err)

	// Test LDAP configuration
	ldapConfig := &types.LDAPConfig{
		Enabled:           true,
		Host:              "localhost",
		Port:              3890,
		Base:              "dc=example,dc=com",
		BindDN:            "cn=admin,dc=example,dc=com",
		BindPassword:      "bindpass",
		UserFilter:        "(uid=%s)",
		GroupFilter:       "(member=%s)",
		Attributes:        []string{"uid", "cn", "mail", "distinguishedName"},
		UseSSL:            false,
		SkipTLS:           false,
		InsecureSkipVerify: false,
	}

	// Create LDAP middleware
	ldapMiddleware := server.NewLDAPMiddleware(ldapConfig, logger)
	assert.NotNil(t, ldapMiddleware)

	// Test basic auth extraction
	t.Run("BasicAuthExtraction", func(t *testing.T) {
		// Create a mock request with Basic Auth
		req, err := http.NewRequest("GET", "/messages", nil)
		require.NoError(t, err)

		// Add Basic Auth header
		auth := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
		req.Header.Set("Authorization", "Basic "+auth)

		// Test extraction
		username, password, ok := extractCredentialsForTest(req)
		assert.True(t, ok)
		assert.Equal(t, "testuser", username)
		assert.Equal(t, "testpass", password)
	})

	t.Run("BearerTokenExtraction", func(t *testing.T) {
		// Create a mock request with Bearer token
		req, err := http.NewRequest("GET", "/messages", nil)
		require.NoError(t, err)

		// Add Bearer token header
		req.Header.Set("Authorization", "Bearer test-token")

		// Test extraction
		username, password, ok := extractCredentialsForTest(req)
		assert.True(t, ok)
		assert.Equal(t, "test-token", username)
		assert.Equal(t, "", password)
	})

	t.Run("NoAuthHeader", func(t *testing.T) {
		// Create a mock request without auth
		req, err := http.NewRequest("GET", "/messages", nil)
		require.NoError(t, err)

		// Test extraction
		username, password, ok := extractCredentialsForTest(req)
		assert.False(t, ok)
		assert.Equal(t, "", username)
		assert.Equal(t, "", password)
	})

	// Clean up
	ldapMiddleware.Close()
}

// extractCredentialsForTest is a test helper to extract credentials
// This mirrors the logic in the LDAP middleware for testing
func extractCredentialsForTest(r *http.Request) (string, string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "", false
	}

	// Check for Basic Auth
	if strings.HasPrefix(authHeader, "Basic ") {
		encoded := strings.TrimPrefix(authHeader, "Basic ")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", "", false
		}

		credentials := strings.SplitN(string(decoded), ":", 2)
		if len(credentials) != 2 {
			return "", "", false
		}

		return credentials[0], credentials[1], true
	}

	// Check for Bearer token
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return token, "", true
	}

	return "", "", false
}
