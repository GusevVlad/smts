package ldap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// HTTPLDAPClient represents an HTTP-based LDAP client for authentication and authorization
type HTTPLDAPClient struct {
	BaseURL     string
	Timeout     time.Duration
	BindDN      string
	BindPassword string
	Base        string
	UserFilter  string
	GroupFilter string
	Attributes  []string
	logger      *zap.Logger
}

// AuthRequest represents an authentication request
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Success bool                   `json:"success"`
	User    interface{}            `json:"user,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	BaseDN string `json:"baseDN"`
	Filter string `json:"filter"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	Success bool              `json:"success"`
	Entries []map[string]string `json:"entries,omitempty"`
	Groups  []map[string]interface{} `json:"groups,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// NewHTTPLDAPClient creates a new HTTP-based LDAP client
func NewHTTPLDAPClient(host string, port int, base string, bindDN string, bindPassword string, userFilter string, groupFilter string, attributes []string, logger *zap.Logger) *HTTPLDAPClient {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	
	return &HTTPLDAPClient{
		BaseURL:     baseURL,
		Timeout:     10 * time.Second,
		BindDN:      bindDN,
		BindPassword: bindPassword,
		Base:        base,
		UserFilter:  userFilter,
		GroupFilter: groupFilter,
		Attributes:  attributes,
		logger:      logger,
	}
}

// Authenticate authenticates a user against the HTTP LDAP server
func (hc *HTTPLDAPClient) Authenticate(username, password string) (bool, map[string]string, error) {
	hc.logger.Info("Starting LDAP authentication", zap.String("username", username))
	
	authReq := AuthRequest{
		Username: username,
		Password: password,
	}

	reqBody, err := json.Marshal(authReq)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal auth request: %w", err)
	}

	client := &http.Client{Timeout: hc.Timeout}
	resp, err := client.Post(hc.BaseURL+"/auth", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return false, nil, fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("auth request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read auth response: %w", err)
	}

	hc.logger.Debug("Raw LDAP auth response", zap.String("body", string(body)))

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return false, nil, fmt.Errorf("failed to unmarshal auth response: %w", err)
	}

	hc.logger.Debug("Parsed auth response",
		zap.Bool("success", authResp.Success),
		zap.Any("user", authResp.User),
		zap.String("error", authResp.Error))

	if !authResp.Success {
		return false, nil, fmt.Errorf("authentication failed: %s", authResp.Error)
	}

	// Convert the user object to the expected format
	userInfo := make(map[string]string)
	if authResp.User != nil {
		// Debug: log the user structure to see what we're getting
		hc.logger.Debug("LDAP authentication response user object",
			zap.String("type", fmt.Sprintf("%T", authResp.User)),
			zap.Any("content", authResp.User))
		
		// Try to handle as map first (for backward compatibility)
		if userMap, ok := authResp.User.(map[string]interface{}); ok {
			// Handle structured LDAPUser response from mock server
			if username, ok := userMap["username"].(string); ok && username != "" {
				userInfo["uid"] = username
				userInfo["cn"] = username
				hc.logger.Debug("Found username in map", zap.String("username", username))
			} else {
				hc.logger.Debug("No username found in user map")
			}
			if email, ok := userMap["email"].(string); ok && email != "" {
				userInfo["mail"] = email
			}
			if displayName, ok := userMap["displayName"].(string); ok && displayName != "" {
				userInfo["displayName"] = displayName
			}
			if dn, ok := userMap["distinguishedName"].(string); ok && dn != "" {
				userInfo["distinguishedName"] = dn
			}
			if dn, ok := userMap["dn"].(string); ok && dn != "" {
				userInfo["distinguishedName"] = dn
			}
		} else {
			// Try to handle as JSON object (for structured responses)
			userJSON, err := json.Marshal(authResp.User)
			if err == nil {
				var userMap map[string]interface{}
				if err := json.Unmarshal(userJSON, &userMap); err == nil {
					// Handle structured LDAPUser response from mock server
					if username, ok := userMap["username"].(string); ok && username != "" {
						userInfo["uid"] = username
						userInfo["cn"] = username
						hc.logger.Debug("Found username in JSON", zap.String("username", username))
					} else {
						hc.logger.Debug("No username found in user JSON")
					}
					if email, ok := userMap["email"].(string); ok && email != "" {
						userInfo["mail"] = email
					}
					if displayName, ok := userMap["displayName"].(string); ok && displayName != "" {
						userInfo["displayName"] = displayName
					}
					if dn, ok := userMap["distinguishedName"].(string); ok && dn != "" {
						userInfo["distinguishedName"] = dn
					}
					if dn, ok := userMap["dn"].(string); ok && dn != "" {
						userInfo["distinguishedName"] = dn
					}
				} else {
					hc.logger.Debug("Failed to unmarshal user JSON", zap.Error(err))
				}
			} else {
				hc.logger.Debug("Failed to marshal user object", zap.Error(err))
			}
		}
	} else {
		hc.logger.Debug("authResp.User is nil")
	}
	
	// Fallback: if we still don't have a username, use the original username from the request
	if userInfo["uid"] == "" {
		userInfo["uid"] = username
		userInfo["cn"] = username
		hc.logger.Debug("Using original username as fallback", zap.String("username", username))
	}
	
	hc.logger.Debug("Final userInfo", zap.Any("userInfo", userInfo))
	hc.logger.Info("LDAP authentication completed successfully",
		zap.String("username", username),
		zap.Any("user_info", userInfo))

	return true, userInfo, nil
}

// GetGroupsOfUser retrieves the groups a user belongs to
func (hc *HTTPLDAPClient) GetGroupsOfUser(user map[string]string) ([]string, error) {
	dn := user["distinguishedName"]
	if dn == "" {
		return nil, fmt.Errorf("user DN not found")
	}

	searchReq := SearchRequest{
		BaseDN: hc.Base,
		Filter: fmt.Sprintf(hc.GroupFilter, dn),
	}

	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	client := &http.Client{Timeout: hc.Timeout}
	resp, err := client.Post(hc.BaseURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to send search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search response: %w", err)
	}

	if !searchResp.Success {
		return nil, fmt.Errorf("search failed: %s", searchResp.Error)
	}

	groups := []string{}
	for _, group := range searchResp.Groups {
		if name, ok := group["name"].(string); ok {
			groups = append(groups, name)
		}
	}

	return groups, nil
}

// GetGroupUsers retrieves users belonging to a specific group
func (hc *HTTPLDAPClient) GetGroupUsers(group string) ([]string, error) {
	searchReq := SearchRequest{
		BaseDN: hc.Base,
		Filter: fmt.Sprintf(hc.UserFilter, group),
	}

	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	client := &http.Client{Timeout: hc.Timeout}
	resp, err := client.Post(hc.BaseURL+"/search", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to send search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search response: %w", err)
	}

	if !searchResp.Success {
		return nil, fmt.Errorf("search failed: %s", searchResp.Error)
	}

	users := []string{}
	for _, entry := range searchResp.Entries {
		if username, ok := entry["uid"]; ok && username != "" {
			users = append(users, username)
		}
	}

	return users, nil
}

// Close is a no-op for HTTP client (for interface compatibility)
func (hc *HTTPLDAPClient) Close() {
	// No connection to close for HTTP client
}