package ldap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
}

// AuthRequest represents an authentication request
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Success bool                   `json:"success"`
	User    map[string]interface{} `json:"user,omitempty"`
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
func NewHTTPLDAPClient(host string, port int, base string, bindDN string, bindPassword string, userFilter string, groupFilter string, attributes []string) *HTTPLDAPClient {
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
	}
}

// Authenticate authenticates a user against the HTTP LDAP server
func (hc *HTTPLDAPClient) Authenticate(username, password string) (bool, map[string]string, error) {
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

	var authResp AuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return false, nil, fmt.Errorf("failed to unmarshal auth response: %w", err)
	}

	if !authResp.Success {
		return false, nil, fmt.Errorf("authentication failed: %s", authResp.Error)
	}

	// Convert the user map to the expected format
	userInfo := make(map[string]string)
	if authResp.User != nil {
		for key, value := range authResp.User {
			if strValue, ok := value.(string); ok {
				userInfo[key] = strValue
			}
		}
	}

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