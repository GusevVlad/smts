package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"smts/pkg/types"
	"smts/pkg/utils"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// APIClient defines the interface for corporate API operations
type APIClient interface {
	DeliverMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error)
	ValidateMessage(ctx context.Context, msg *types.Message) (*types.DLPValidationResponse, error)
	HealthCheck(ctx context.Context) error
}

// Client represents a corporate API client
type Client struct {
	client      *resty.Client
	apiConfig   *types.APIConfig
	dlpConfig   *types.DLPConfig
	logger      *zap.Logger
	baseURL     string
	tokenManager *TokenManager
}

// NewClient creates a new corporate API client
func NewClient(apiConfig *types.APIConfig, dlpConfig *types.DLPConfig, logger *zap.Logger) *Client {
	client := resty.New().
		SetTimeout(apiConfig.Timeout).
		SetRetryCount(0). // We'll handle retries ourselves
		SetHeader("User-Agent", "SMTS/1.0")

	// Note: Authentication headers are set per-request to avoid conflicts

	return &Client{
		client:      client,
		apiConfig:   apiConfig,
		dlpConfig:   dlpConfig,
		logger:      logger,
		baseURL:     apiConfig.BaseURL,
		tokenManager: nil, // No longer needed for client_credentials header-based auth
	}
}

// setAuthHeaders sets the appropriate authentication headers for the request
func (c *Client) setAuthHeaders(ctx context.Context, request *resty.Request) error {
	switch c.apiConfig.Auth.Type {
	case "api_key":
		if c.apiConfig.Auth.APIKey != "" {
			request.SetHeader("X-API-Key", c.apiConfig.Auth.APIKey)
		}
	case "client_credentials":
		// For client_credentials, send client ID and secret as headers
		if c.apiConfig.Auth.ClientID != "" {
			request.SetHeader("X-Client-ID", c.apiConfig.Auth.ClientID)
		}
		if c.apiConfig.Auth.ClientSecret != "" {
			request.SetHeader("X-Client-Secret", c.apiConfig.Auth.ClientSecret)
		}
	}
	return nil
}

// DeliverMessage delivers a message to the corporate API
func (c *Client) DeliverMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	operation := "deliver_message"
	startTime := time.Now()

	// Validate message
	if err := c.validateMessage(msg); err != nil {
		messageID := ""
		if msg != nil {
			messageID = msg.ID
		}
		return &types.DeliveryResult{
			Success:    false,
			MessageID:  messageID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}

	// Build the endpoint URL - use the new /smts/message endpoint
	endpoint := fmt.Sprintf("%s/smts/message", c.baseURL)

	// Parse the message body to extract the actual data
	var messageData map[string]interface{}
	if err := json.Unmarshal(msg.Body, &messageData); err != nil {
		// If parsing fails, use the raw body as data
		messageData = map[string]interface{}{
			"content": string(msg.Body),
		}
	}

	// Prepare the request body for /smts/message endpoint
	requestBody := map[string]interface{}{
		"topic": msg.Topic,
		"data":  messageData,
	}

	// Prepare the request
	request := c.client.R().
		SetContext(ctx).
		SetBody(requestBody).
		SetHeader("Content-Type", "application/json")

	// Set authentication headers
	if err := c.setAuthHeaders(ctx, request); err != nil {
		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}

	// Add SMTS headers as required
	request.SetHeader("X-SMTS-Message-ID", msg.ID)
	request.SetHeader("X-SMTS-Timestamp", msg.Timestamp.Format(time.RFC3339))
	request.SetHeader("X-SMTS-Source", msg.Source)

	// Add custom headers from the message
	for key, value := range msg.Headers {
		request.SetHeader(key, value)
	}

	// Execute the request with retry
	var resp *resty.Response
	var err error

	retryConfig := utils.RetryConfig{
		MaxAttempts: c.apiConfig.Retry.MaxAttempts,
		Backoff:     c.apiConfig.Retry.Backoff,
		Jitter:      true,
	}

	c.logger.Debug("Starting retry logic",
		append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
			zap.Int("max_attempts", retryConfig.MaxAttempts),
			zap.Duration("backoff", retryConfig.Backoff))...)

	err = utils.Retry(ctx, retryConfig, func(attempt int) error {
		c.logger.Debug("Sending message to corporate API",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				zap.Int("attempt", attempt),
				zap.String("endpoint", endpoint))...)

		resp, err = request.Post(endpoint)
		if err != nil {
			c.logger.Error("HTTP request failed",
				append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
					zap.Int("attempt", attempt),
					zap.String("endpoint", endpoint),
					zap.Error(err))...)
			return types.WrapSMTSError(err, types.ErrAPIConnection, "Failed to connect to corporate API")
		}

		// Check HTTP status code
		if resp.StatusCode() >= 400 {
			if resp.StatusCode() == http.StatusUnauthorized {
				return types.NewSMTSError(types.ErrAPIAuth, "API authentication failed")
			} else if resp.StatusCode() >= 500 {
				return types.WrapSMTSError(
					fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String()),
					types.ErrAPIResponse,
					"Server error from corporate API",
				)
			} else {
				// Client errors (4xx) are not retryable
				return types.WrapSMTSError(
					fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String()),
					types.ErrAPIRequest,
					"Client error from corporate API",
				)
			}
		}

		return nil
	}, c.logger, operation)

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		c.logger.Error("Failed to deliver message",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				utils.WithError(err),
				utils.WithDuration(duration))...)

		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}

	// Check if resp is nil before accessing StatusCode
	if resp == nil {
		err := types.NewSMTSError(types.ErrAPIConnection, "No response received from corporate API")
		c.logger.Error("Failed to deliver message - no response",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				utils.WithError(err),
				utils.WithDuration(duration))...)

		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}

	c.logger.Info("Message delivered successfully",
		append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
			zap.Int("status_code", resp.StatusCode()),
			utils.WithDuration(duration))...)

	return &types.DeliveryResult{
		Success:   true,
		MessageID: msg.ID,
		Timestamp: time.Now().UTC(),
	}, nil
}

// ValidateMessage sends a message for DLP validation
func (c *Client) ValidateMessage(ctx context.Context, msg *types.Message) (*types.DLPValidationResponse, error) {
	operation := "validate_message"
	startTime := time.Now()

	// Build the DLP validation request
	dlpRequest := &types.DLPValidationRequest{
		MessageID: msg.ID,
		Topic:     msg.Topic,
		Content:   msg.Body,
		Metadata: map[string]any{
			"source":    msg.Source,
			"timestamp": msg.Timestamp,
			"headers":   msg.Headers,
		},
	}

	// Build the endpoint URL - use DLP endpoint if DLP is enabled
	endpoint := c.dlpConfig.Endpoint
	if !c.dlpConfig.Enabled {
		// If DLP is disabled, return a mock approved response
		return &types.DLPValidationResponse{
			Approved: true,
			Reasons:  []string{},
		}, nil
	}

	// Prepare the request
	request := c.client.R().
		SetContext(ctx).
		SetBody(dlpRequest).
		SetHeader("Content-Type", "application/json")

	// Set authentication headers
	if err := c.setAuthHeaders(ctx, request); err != nil {
		return nil, err
	}

	// Execute the request with retry
	var resp *resty.Response
	var err error

	retryConfig := utils.RetryConfig{
		MaxAttempts: 2, // Fewer retries for DLP validation
		Backoff:     1 * time.Second,
		Jitter:      true,
	}

	err = utils.Retry(ctx, retryConfig, func(attempt int) error {
		c.logger.Debug("Sending message for DLP validation",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				zap.Int("attempt", attempt))...)

		resp, err = request.Post(endpoint)
		if err != nil {
			return types.WrapSMTSError(err, types.ErrDLPConnection, "Failed to connect to DLP service")
		}

		// Check HTTP status code
		if resp.StatusCode() >= 400 {
			if resp.StatusCode() == http.StatusUnauthorized {
				return types.NewSMTSError(types.ErrAPIAuth, "DLP authentication failed")
			} else if resp.StatusCode() >= 500 {
				return types.WrapSMTSError(
					fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String()),
					types.ErrDLPRequest,
					"Server error from DLP service",
				)
			} else {
				return types.WrapSMTSError(
					fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String()),
					types.ErrDLPRequest,
					"Client error from DLP service",
				)
			}
		}

		return nil
	}, c.logger, operation)

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		c.logger.Error("Failed to validate message",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				utils.WithError(err),
				utils.WithDuration(duration))...)

		return nil, err
	}

	// Parse the response
	var dlpResponse types.DLPValidationResponse
	if err := json.Unmarshal(resp.Body(), &dlpResponse); err != nil {
		c.logger.Error("Failed to parse DLP validation response",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				utils.WithError(err))...)

		return nil, types.WrapSMTSError(err, types.ErrDLPValidation, "Failed to parse DLP validation response")
	}

	if !dlpResponse.Approved {
		c.logger.Warn("Message rejected by DLP validation",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				zap.Strings("reasons", dlpResponse.Reasons),
				utils.WithDuration(duration))...)

		return &dlpResponse, types.NewSMTSError(types.ErrDLPValidation, "Message rejected by DLP validation")
	}

	c.logger.Info("Message approved by DLP validation",
		append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
			utils.WithDuration(duration))...)

	return &dlpResponse, nil
}

// validateMessage validates a message before delivery
func (c *Client) validateMessage(msg *types.Message) error {
	if msg == nil {
		return types.NewSMTSError(types.ErrMessageValidation, "Message cannot be nil")
	}

	if msg.Topic == "" {
		return types.NewSMTSError(types.ErrMessageValidation, "Message topic is required")
	}

	if msg.ID == "" {
		return types.NewSMTSError(types.ErrMessageValidation, "Message ID is required")
	}

	if len(msg.Body) == 0 {
		return types.NewSMTSError(types.ErrMessageValidation, "Message body cannot be empty")
	}

	return nil
}

// HealthCheck performs a health check on the API client
func (c *Client) HealthCheck(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/health", c.baseURL)

	request := c.client.R().
		SetContext(ctx)

	// Set authentication headers
	if err := c.setAuthHeaders(ctx, request); err != nil {
		return err
	}

	resp, err := request.Get(endpoint)

	if err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "API health check failed")
	}

	if resp.StatusCode() != http.StatusOK {
		return types.NewSMTSError(types.ErrHealthCheck,
			fmt.Sprintf("API health check returned status %d", resp.StatusCode()))
	}

	return nil
}