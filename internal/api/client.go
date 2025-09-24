package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/corporate/smts/pkg/types"
	"github.com/corporate/smts/pkg/utils"
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
	client  *resty.Client
	config  *types.APIConfig
	logger  *zap.Logger
	baseURL string
}

// NewClient creates a new corporate API client
func NewClient(config *types.APIConfig, logger *zap.Logger) *Client {
	client := resty.New().
		SetTimeout(config.Timeout).
		SetRetryCount(0). // We'll handle retries ourselves
		SetHeader("User-Agent", "SMTS/1.0")

	// Configure authentication
	if config.Auth.Type == "api_key" && config.Auth.APIKey != "" {
		client.SetHeader("X-API-Key", config.Auth.APIKey)
	}

	return &Client{
		client:  client,
		config:  config,
		logger:  logger,
		baseURL: config.BaseURL,
	}
}

// DeliverMessage delivers a message to the corporate API
func (c *Client) DeliverMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	operation := "deliver_message"
	startTime := time.Now()

	// Validate message
	if err := c.validateMessage(msg); err != nil {
		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}

	// Build the endpoint URL
	endpoint := fmt.Sprintf("%s/%s", c.baseURL, msg.Topic)

	// Prepare the request
	request := c.client.R().
		SetContext(ctx).
		SetBody(msg.Body).
		SetHeader("Content-Type", "application/json")

	// Add SMTS headers
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
		MaxAttempts: c.config.Retry.MaxAttempts,
		Backoff:     c.config.Retry.Backoff,
		Jitter:      true,
	}

	err = utils.Retry(ctx, retryConfig, func(attempt int) error {
		c.logger.Debug("Sending message to corporate API",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				zap.Int("attempt", attempt),
				zap.String("endpoint", endpoint))...)

		resp, err = request.Post(endpoint)
		if err != nil {
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

	// Build the endpoint URL
	endpoint := fmt.Sprintf("%s/validate", c.baseURL)

	// Prepare the request
	request := c.client.R().
		SetContext(ctx).
		SetBody(dlpRequest).
		SetHeader("Content-Type", "application/json")

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

	resp, err := c.client.R().
		SetContext(ctx).
		Get(endpoint)

	if err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "API health check failed")
	}

	if resp.StatusCode() != http.StatusOK {
		return types.NewSMTSError(types.ErrHealthCheck,
			fmt.Sprintf("API health check returned status %d", resp.StatusCode()))
	}

	return nil
}