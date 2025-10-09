package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"smts/pkg/types"
	"go.uber.org/zap"
)

// TokenResponse represents the OAuth2 token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// TokenManager handles OAuth2 client credentials token management
type TokenManager struct {
	config     *types.AuthConfig
	logger     *zap.Logger
	httpClient *http.Client

	mu           sync.RWMutex
	token        string
	tokenType    string
	expiresAt    time.Time
	refreshMutex sync.Mutex
}

// NewTokenManager creates a new token manager
func NewTokenManager(config *types.AuthConfig, logger *zap.Logger) *TokenManager {
	return &TokenManager{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetToken returns a valid access token, refreshing if necessary
func (tm *TokenManager) GetToken(ctx context.Context) (string, error) {
	tm.mu.RLock()
	token := tm.token
	expiresAt := tm.expiresAt
	tm.mu.RUnlock()

	// Check if token is valid and not expired (with 30 second buffer)
	if token != "" && time.Now().Add(30*time.Second).Before(expiresAt) {
		return token, nil
	}

	// Acquire refresh lock to prevent multiple concurrent refreshes
	tm.refreshMutex.Lock()
	defer tm.refreshMutex.Unlock()

	// Double-check after acquiring lock
	tm.mu.RLock()
	token = tm.token
	expiresAt = tm.expiresAt
	tm.mu.RUnlock()

	if token != "" && time.Now().Add(30*time.Second).Before(expiresAt) {
		return token, nil
	}

	// Token needs refresh
	return tm.refreshToken(ctx)
}

// refreshToken obtains a new access token using client credentials
func (tm *TokenManager) refreshToken(ctx context.Context) (string, error) {
	tm.logger.Debug("Refreshing OAuth2 token")

	// Prepare form data
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", tm.config.ClientID)
	form.Set("client_secret", tm.config.ClientSecret)
	if tm.config.Scopes != "" {
		form.Set("scope", tm.config.Scopes)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", tm.config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", types.WrapSMTSError(err, types.ErrAPIAuth, "Failed to create token request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return "", types.WrapSMTSError(err, types.ErrAPIAuth, "Failed to execute token request")
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", types.NewSMTSError(types.ErrAPIAuth,
			fmt.Sprintf("Token request failed with status %d", resp.StatusCode))
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", types.WrapSMTSError(err, types.ErrAPIAuth, "Failed to parse token response")
	}

	if tokenResp.AccessToken == "" {
		return "", types.NewSMTSError(types.ErrAPIAuth, "Empty access token in response")
	}

	// Calculate expiration time
	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn == 0 {
		expiresIn = 3600 * time.Second // Default to 1 hour if not provided
	}
	expiresAt := time.Now().Add(expiresIn)

	// Update token state
	tm.mu.Lock()
	tm.token = tokenResp.AccessToken
	tm.tokenType = tokenResp.TokenType
	tm.expiresAt = expiresAt
	tm.mu.Unlock()

	tm.logger.Debug("OAuth2 token refreshed successfully",
		zap.String("token_type", tokenResp.TokenType),
		zap.Duration("expires_in", expiresIn))

	return tokenResp.AccessToken, nil
}

// ClearToken clears the cached token (useful for testing or re-authentication)
func (tm *TokenManager) ClearToken() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.token = ""
	tm.tokenType = ""
	tm.expiresAt = time.Time{}
}

// IsTokenValid checks if the current token is valid
func (tm *TokenManager) IsTokenValid() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.token != "" && time.Now().Add(30*time.Second).Before(tm.expiresAt)
}