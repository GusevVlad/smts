package server

import (
	"encoding/base64"
	"net/http"
	"strings"

	"smts/internal/ldap"
	"smts/pkg/types"
	"go.uber.org/zap"
)

// LDAPClient interface defines the methods required for LDAP authentication
type LDAPClient interface {
	Authenticate(username, password string) (bool, map[string]string, error)
	GetGroupsOfUser(user map[string]string) ([]string, error)
	GetGroupUsers(group string) ([]string, error)
	Close()
}

// LDAPMiddleware handles LDAP authentication for the message API
type LDAPMiddleware struct {
	ldapClient LDAPClient
	logger     *zap.Logger
	enabled    bool
}

// NewLDAPMiddleware creates a new LDAP middleware instance
func NewLDAPMiddleware(ldapConfig *types.LDAPConfig, logger *zap.Logger) *LDAPMiddleware {
	if !ldapConfig.Enabled {
		return &LDAPMiddleware{
			enabled: false,
			logger:  logger,
		}
	}

	var ldapClient LDAPClient

	if ldapConfig.UseHTTP {
		// Use HTTP-based LDAP client
		ldapClient = ldap.NewHTTPLDAPClient(
			ldapConfig.Host,
			ldapConfig.Port,
			ldapConfig.Base,
			ldapConfig.BindDN,
			ldapConfig.BindPassword,
			ldapConfig.UserFilter,
			ldapConfig.GroupFilter,
			ldapConfig.Attributes,
		)
	} else {
		// Use regular LDAP client
		ldapClient = &ldap.LDAPClient{
			Attributes:         ldapConfig.Attributes,
			Base:               ldapConfig.Base,
			BindDN:             ldapConfig.BindDN,
			BindPassword:       ldapConfig.BindPassword,
			GroupFilter:        ldapConfig.GroupFilter,
			Host:               ldapConfig.Host,
			ServerName:         ldapConfig.ServerName,
			UserFilter:         ldapConfig.UserFilter,
			Port:               ldapConfig.Port,
			InsecureSkipVerify: ldapConfig.InsecureSkipVerify,
			UseSSL:             ldapConfig.UseSSL,
			SkipTLS:            ldapConfig.SkipTLS,
		}
	}

	return &LDAPMiddleware{
		ldapClient: ldapClient,
		logger:     logger,
		enabled:    true,
	}
}

// Authenticate handles LDAP authentication for HTTP requests
func (lm *LDAPMiddleware) Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If LDAP is not enabled, skip authentication
		if !lm.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Extract credentials from Authorization header
		username, password, ok := lm.extractCredentials(r)
		if !ok {
			lm.logger.Warn("Missing or invalid Authorization header")
			http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
			return
		}

		// Authenticate against LDAP
		authenticated, userInfo, err := lm.ldapClient.Authenticate(username, password)
		if err != nil {
			lm.logger.Error("LDAP authentication failed",
				zap.String("username", username),
				zap.Error(err))
			http.Error(w, `{"error": "Authentication failed"}`, http.StatusUnauthorized)
			return
		}

		if !authenticated {
			lm.logger.Warn("LDAP authentication rejected",
				zap.String("username", username))
			http.Error(w, `{"error": "Invalid credentials"}`, http.StatusUnauthorized)
			return
		}

		// Get user groups for authorization
		groups, err := lm.ldapClient.GetGroupsOfUser(userInfo)
		if err != nil {
			lm.logger.Warn("Failed to get user groups",
				zap.String("username", username),
				zap.Error(err))
			// Continue without groups - authorization will be limited
		}

		// Add user info to request context for authorization
		ctx := r.Context()
		ctx = contextWithUserInfo(ctx, userInfo, groups)
		r = r.WithContext(ctx)

		lm.logger.Info("LDAP authentication successful",
			zap.String("username", username),
			zap.Strings("groups", groups))

		next.ServeHTTP(w, r)
	}
}

// extractCredentials extracts username and password from Authorization header
func (lm *LDAPMiddleware) extractCredentials(r *http.Request) (string, string, bool) {
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

	// Check for Bearer token (could be used for API key integration)
	if strings.HasPrefix(authHeader, "Bearer ") {
		// For now, we'll use the token as username with empty password
		// This can be extended to validate API keys against LDAP
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return token, "", true
	}

	return "", "", false
}

// AuthorizeTopic checks if the user is authorized to access the requested topic
func (lm *LDAPMiddleware) AuthorizeTopic(r *http.Request, topic string) bool {
	// If LDAP is not enabled, allow all access
	if !lm.enabled {
		return true
	}

	// Get user info from context
	userInfo, groups := userInfoFromContext(r.Context())
	if userInfo == nil {
		lm.logger.Warn("No user info found in context for topic authorization")
		return false
	}

	// Simple authorization logic - can be extended based on requirements
	// For now, we'll allow access if user is in any group
	// This can be enhanced to check specific group memberships or user attributes
	if len(groups) > 0 {
		lm.logger.Debug("Topic authorization granted",
			zap.String("username", userInfo["uid"]),
			zap.String("topic", topic),
			zap.Strings("groups", groups))
		return true
	}

	lm.logger.Warn("Topic authorization denied - user has no groups",
		zap.String("username", userInfo["uid"]),
		zap.String("topic", topic))
	return false
}

// AuthorizeEndpoint checks if the user is authorized to access the specific endpoint
// based on LDAP group membership and deployment type
func (lm *LDAPMiddleware) AuthorizeEndpoint(r *http.Request, deploymentType string, endpointType string) bool {
	// If LDAP is not enabled, allow all access
	if !lm.enabled {
		return true
	}

	// Get user info from context
	userInfo, groups := userInfoFromContext(r.Context())
	if userInfo == nil {
		lm.logger.Warn("No user info found in context for endpoint authorization")
		return false
	}

	username := userInfo["uid"]
	if username == "" {
		username = "unknown"
	}

	// Define required groups based on deployment type and endpoint
	requiredGroups := lm.getRequiredGroups(deploymentType, endpointType)

	// Check if user has any of the required groups
	for _, group := range groups {
		for _, requiredGroup := range requiredGroups {
			if group == requiredGroup {
				lm.logger.Debug("Endpoint authorization granted",
					zap.String("username", username),
					zap.String("deployment", deploymentType),
					zap.String("endpoint", endpointType),
					zap.String("group", group))
				return true
			}
		}
	}

	lm.logger.Warn("Endpoint authorization denied",
		zap.String("username", username),
		zap.String("deployment", deploymentType),
		zap.String("endpoint", endpointType),
		zap.Strings("user_groups", groups),
		zap.Strings("required_groups", requiredGroups))
	return false
}

// getRequiredGroups returns the required LDAP groups for a given deployment and endpoint
func (lm *LDAPMiddleware) getRequiredGroups(deploymentType string, endpointType string) []string {
	switch deploymentType {
	case "ext":
		switch endpointType {
		case "send":
			return []string{"ext_writer", "admin"}
		case "read":
			return []string{"ext_reader", "admin"}
		case "corp_message":
			return []string{"ext_writer", "admin"}
		}
	case "int":
		switch endpointType {
		case "send":
			return []string{"int_writer", "admin"}
		case "read":
			return []string{"int_reader", "admin"}
		case "corp_message":
			return []string{"int_writer", "admin"}
		}
	}

	// Default: require at least one group for any access
	return []string{"ext_writer", "ext_reader", "int_writer", "int_reader", "admin"}
}

// Close closes the LDAP connection
func (lm *LDAPMiddleware) Close() {
	if lm.ldapClient != nil {
		lm.ldapClient.Close()
	}
}