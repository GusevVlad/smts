package api

import (
	"bytes"
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

// Traffic Monitor API structures
type trafficMonitorAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type trafficMonitorContact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type trafficMonitorContactWithMeta struct {
	Contact trafficMonitorContact `json:"contact"`
}

type trafficMonitorIdentity struct {
	IdentityID               int                             `json:"identity_id"`
	IdentityType             int                             `json:"identity_type"`
	IdentityContacts         []trafficMonitorContact         `json:"identity_contacts,omitempty"`
	IdentityContactsWithMeta []trafficMonitorContactWithMeta `json:"identity_contacts_with_meta,omitempty"`
	IdentityAttributes       []trafficMonitorAttribute       `json:"identity_attributes,omitempty"`
}

type trafficMonitorData struct {
	DataID         int                       `json:"data_id"`
	DataAttributes []trafficMonitorAttribute `json:"data_attributes,omitempty"`
}

type trafficMonitorEvent struct {
	EvtClass       int                       `json:"evt_class"`
	EvtService     string                    `json:"evt_service"`
	EvtAttributes  []trafficMonitorAttribute `json:"evt_attributes"`
	EvtSenders     []trafficMonitorIdentity  `json:"evt_senders"`
	EvtReceivers   []trafficMonitorIdentity  `json:"evt_receivers"`
	EvtData        []trafficMonitorData      `json:"evt_data"`
	EvtDestination *trafficMonitorIdentity   `json:"evt_destination,omitempty"`
}

type trafficMonitorPushResponse struct {
	Data struct {
		DocumentID string `json:"document_id"`
	} `json:"data"`
}

type trafficMonitorVerdictResponse struct {
	Verdict string   `json:"verdict"`
	Reasons []string `json:"reasons,omitempty"`
}

// APIClient defines the interface for corporate API operations
type APIClient interface {
	DeliverMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error)
	ValidateMessage(ctx context.Context, msg *types.Message) (*types.DLPValidationResponse, error)
	HealthCheck(ctx context.Context) error
}

// Client represents a corporate API client
type Client struct {
	client       *resty.Client
	apiConfig    *types.APIConfig
	dlpConfig    *types.DLPConfig
	logger       *zap.Logger
	baseURL      string
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
		client:       client,
		apiConfig:    apiConfig,
		dlpConfig:    dlpConfig,
		logger:       logger,
		baseURL:      apiConfig.BaseURL,
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
			Success:   false,
			MessageID: messageID,
			Timestamp: time.Now().UTC(),
			Error:     err.Error(),
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
			Success:   false,
			MessageID: msg.ID,
			Timestamp: time.Now().UTC(),
			Error:     err.Error(),
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
			Success:   false,
			MessageID: msg.ID,
			Timestamp: time.Now().UTC(),
			Error:     err.Error(),
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
			Success:   false,
			MessageID: msg.ID,
			Timestamp: time.Now().UTC(),
			Error:     err.Error(),
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

	if !c.dlpConfig.Enabled {
		// If DLP is disabled, return a mock approved response
		return &types.DLPValidationResponse{
			Approved:  true,
			MessageID: msg.ID,
			Reasons:   []string{},
		}, nil
	}

	// Branch based on provider
	switch c.dlpConfig.Provider {
	case "traffic_monitor":
		return c.validateMessageTrafficMonitor(ctx, msg, operation, startTime)
	default: // "legacy" or empty
		return c.validateMessageLegacy(ctx, msg, operation, startTime)
	}
}

// validateMessageLegacy sends a message for DLP validation using the legacy endpoint
func (c *Client) validateMessageLegacy(ctx context.Context, msg *types.Message, operation string, startTime time.Time) (*types.DLPValidationResponse, error) {
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

// validateMessageTrafficMonitor sends a message for DLP validation using Traffic Monitor API
func (c *Client) validateMessageTrafficMonitor(ctx context.Context, msg *types.Message, operation string, startTime time.Time) (*types.DLPValidationResponse, error) {
	// Build Traffic Monitor event
	event, err := c.buildTrafficMonitorEvent(msg)
	if err != nil {
		return nil, types.WrapSMTSError(err, types.ErrDLPValidation, "Failed to build Traffic Monitor event")
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return nil, types.WrapSMTSError(err, types.ErrDLPValidation, "Failed to marshal Traffic Monitor event")
	}

	// Build endpoint URL
	endpoint := c.dlpConfig.TrafficMonitor.BaseURL + "/push/event"

	// Prepare multipart request
	request := c.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "multipart/form-data").
		SetMultipartField("event", "", "application/json", bytes.NewReader(eventJSON))

	// Set Traffic Monitor authentication headers
	c.setTrafficMonitorAuthHeaders(request)

	// Execute the request with retry
	var resp *resty.Response
	retryConfig := utils.RetryConfig{
		MaxAttempts: 2,
		Backoff:     1 * time.Second,
		Jitter:      true,
	}

	err = utils.Retry(ctx, retryConfig, func(attempt int) error {
		c.logger.Debug("Sending message to Traffic Monitor",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				zap.Int("attempt", attempt),
				zap.String("endpoint", endpoint))...)

		resp, err = request.Post(endpoint)
		if err != nil {
			return types.WrapSMTSError(err, types.ErrDLPConnection, "Failed to connect to Traffic Monitor")
		}

		// Check HTTP status code
		if resp.StatusCode() >= 400 {
			if resp.StatusCode() == http.StatusUnauthorized {
				return types.NewSMTSError(types.ErrAPIAuth, "Traffic Monitor authentication failed")
			} else if resp.StatusCode() >= 500 {
				return types.WrapSMTSError(
					fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String()),
					types.ErrDLPRequest,
					"Server error from Traffic Monitor",
				)
			} else {
				return types.WrapSMTSError(
					fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String()),
					types.ErrDLPRequest,
					"Client error from Traffic Monitor",
				)
			}
		}

		return nil
	}, c.logger, operation)

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		c.logger.Error("Failed to validate message via Traffic Monitor",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				utils.WithError(err),
				utils.WithDuration(duration))...)
		return nil, err
	}

	// Parse push response
	var pushResp trafficMonitorPushResponse
	if err := json.Unmarshal(resp.Body(), &pushResp); err != nil {
		c.logger.Error("Failed to parse Traffic Monitor push response",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				utils.WithError(err))...)
		return nil, types.WrapSMTSError(err, types.ErrDLPValidation, "Failed to parse Traffic Monitor push response")
	}

	documentID := pushResp.Data.DocumentID
	if documentID == "" {
		return nil, types.NewSMTSError(types.ErrDLPValidation, "Empty document ID received from Traffic Monitor")
	}

	// If verdict polling is enabled, poll for verdict
	if c.dlpConfig.TrafficMonitor.VerdictPollingEnabled {
		verdictResp, err := c.pollVerdict(ctx, documentID, operation, msg)
		if err != nil {
			return nil, err
		}
		return verdictResp, nil
	}

	// If polling is disabled, assume approval (or maybe we should treat as unknown?)
	// For safety, we'll treat as approved but log a warning.
	c.logger.Warn("Verdict polling disabled, assuming message approved",
		append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
			zap.String("document_id", documentID))...)

	return &types.DLPValidationResponse{
		Approved:  true,
		MessageID: msg.ID,
		Reasons:   []string{},
	}, nil
}

// buildTrafficMonitorEvent constructs a Traffic Monitor event from a message
func (c *Client) buildTrafficMonitorEvent(msg *types.Message) (*trafficMonitorEvent, error) {
	// Use current time for capture timestamp
	captureTime := time.Now().Format(time.RFC3339)

	// Build evt_attributes
	evtAttributes := []trafficMonitorAttribute{
		{Name: "event_name", Value: "SMTS Message"},
		{Name: "capture_ts", Value: captureTime},
		{Name: "capture_server_ip", Value: c.dlpConfig.TrafficMonitor.CaptureServerIP},
		{Name: "capture_server_fqdn", Value: c.dlpConfig.TrafficMonitor.CaptureServerFQDN},
		{Name: "message_id", Value: msg.ID},
		{Name: "topic", Value: msg.Topic},
		{Name: "source", Value: msg.Source},
	}

	// Build evt_senders (simplified - single sender representing SMTS)
	evtSenders := []trafficMonitorIdentity{
		{
			IdentityID:   1,
			IdentityType: 0, // user
			IdentityContactsWithMeta: []trafficMonitorContactWithMeta{
				{
					Contact: trafficMonitorContact{
						Name:  "auth",
						Value: "smts@system",
					},
				},
			},
		},
	}

	// Build evt_receivers (empty for now)
	evtReceivers := []trafficMonitorIdentity{}

	// Build evt_data with message body
	evtData := []trafficMonitorData{
		{
			DataID: 1,
			DataAttributes: []trafficMonitorAttribute{
				{Name: "filename", Value: "message.json"},
				{Name: "content_type", Value: "application/json"},
				{Name: "size", Value: fmt.Sprintf("%d", len(msg.Body))},
			},
		},
	}

	event := &trafficMonitorEvent{
		EvtClass:      3, // web_common
		EvtService:    "smts",
		EvtAttributes: evtAttributes,
		EvtSenders:    evtSenders,
		EvtReceivers:  evtReceivers,
		EvtData:       evtData,
		// EvtDestination omitted
	}
	return event, nil
}

// setTrafficMonitorAuthHeaders sets the required authentication headers for Traffic Monitor API
func (c *Client) setTrafficMonitorAuthHeaders(request *resty.Request) {
	if token := c.dlpConfig.TrafficMonitor.AuthToken; token != "" {
		request.SetHeader("X-API-Auth-Token", token)
	}
	if companyID := c.dlpConfig.TrafficMonitor.CompanyId; companyID != "" {
		request.SetHeader("X-API-CompanyId", companyID)
	}
	version := c.dlpConfig.TrafficMonitor.Version
	if version == "" {
		version = "1.8"
	}
	request.SetHeader("X-API-Version", version)
}

// pollVerdict polls the Traffic Monitor DataExport API for verdict of a document
func (c *Client) pollVerdict(ctx context.Context, documentID string, operation string, msg *types.Message) (*types.DLPValidationResponse, error) {
	// Build endpoint URL
	endpoint := c.dlpConfig.TrafficMonitor.BaseURL + "/xapi/event/verdict"
	// Use current date in YYYY-MM-DD format
	date := time.Now().Format("2006-01-02")

	// Prepare request
	request := c.client.R().
		SetContext(ctx).
		SetQueryParam("documentId", documentID).
		SetQueryParam("date", date)

	c.setTrafficMonitorAuthHeaders(request)

	// Poll with timeout and interval
	timeout := c.dlpConfig.TrafficMonitor.VerdictPollingTimeout
	interval := c.dlpConfig.TrafficMonitor.VerdictPollingInterval
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if interval == 0 {
		interval = 5 * time.Second
	}

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		c.logger.Debug("Polling verdict from Traffic Monitor",
			append(utils.LoggerFields(operation, "", msg.ID, msg.Topic),
				zap.String("document_id", documentID))...)

		resp, err := request.Get(endpoint)
		if err != nil {
			lastErr = types.WrapSMTSError(err, types.ErrDLPConnection, "Failed to connect to Traffic Monitor verdict endpoint")
			time.Sleep(interval)
			continue
		}

		if resp.StatusCode() >= 400 {
			if resp.StatusCode() == http.StatusNotFound {
				// Verdict not yet ready, sleep and retry
				time.Sleep(interval)
				continue
			}
			lastErr = types.NewSMTSError(types.ErrDLPRequest, fmt.Sprintf("Verdict endpoint returned HTTP %d", resp.StatusCode()))
			time.Sleep(interval)
			continue
		}

		// Parse verdict response
		var verdictResp trafficMonitorVerdictResponse
		if err := json.Unmarshal(resp.Body(), &verdictResp); err != nil {
			lastErr = types.WrapSMTSError(err, types.ErrDLPValidation, "Failed to parse verdict response")
			time.Sleep(interval)
			continue
		}

		// Map verdict to DLPValidationResponse
		approved := false
		if verdictResp.Verdict == "approved" {
			approved = true
		} else if verdictResp.Verdict == "rejected" {
			approved = false
		} else {
			// Unknown verdict, treat as rejected with reason
			return &types.DLPValidationResponse{
				Approved:  false,
				MessageID: msg.ID,
				Reasons:   []string{fmt.Sprintf("Unknown verdict: %s", verdictResp.Verdict)},
			}, nil
		}

		return &types.DLPValidationResponse{
			Approved:  approved,
			MessageID: msg.ID,
			Reasons:   verdictResp.Reasons,
		}, nil
	}

	// Timeout reached
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, types.NewSMTSError(types.ErrDLPValidation, "Timeout waiting for verdict from Traffic Monitor")
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
