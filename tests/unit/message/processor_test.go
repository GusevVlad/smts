package message_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"smts/internal/message"
	"smts/pkg/types"
	"smts/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestProcessor_HandleMessage_EXT_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for EXT deployment
	config := mocks.CreateTestConfig("ext")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"event": "test"}`),
	}
	
	// Mock successful delivery
	deliveryResult := &types.DeliveryResult{
		Success:   true,
		MessageID: "test-message-123",
		Timestamp: time.Now().UTC(),
	}
	
	mockAPIClient.On("DeliverMessage", mock.Anything, msg).Return(deliveryResult, nil)
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "test-message-123", result.MessageID)
	mockAPIClient.AssertExpectations(t)
}

func TestProcessor_HandleMessage_INT_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for INT deployment
	config := mocks.CreateTestConfig("int")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"event": "test"}`),
	}
	
	// Mock DLP validation success
	dlpResponse := &types.DLPValidationResponse{
		Approved:  true,
		MessageID: "test-message-123",
	}
	
	// Mock successful delivery
	deliveryResult := &types.DeliveryResult{
		Success:   true,
		MessageID: "test-message-123",
		Timestamp: time.Now().UTC(),
	}
	
	mockAPIClient.On("ValidateMessage", mock.Anything, msg).Return(dlpResponse, nil)
	mockAPIClient.On("DeliverMessage", mock.Anything, msg).Return(deliveryResult, nil)
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "test-message-123", result.MessageID)
	mockAPIClient.AssertExpectations(t)
}

func TestProcessor_HandleMessage_INT_DLPRejected(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for INT deployment
	config := mocks.CreateTestConfig("int")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"sensitive": "data"}`),
	}
	
	// Mock DLP validation rejection
	dlpResponse := &types.DLPValidationResponse{
		Approved:  false,
		MessageID: "test-message-123",
		Reasons:   []string{"Contains sensitive information"},
	}
	
	mockAPIClient.On("ValidateMessage", mock.Anything, msg).Return(dlpResponse, nil)
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "DLP validation failed")
	mockAPIClient.AssertExpectations(t)
}

func TestProcessor_HandleMessage_INT_DLPRejected_WithClientSender_SecurityWarning(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for INT deployment
	config := mocks.CreateTestConfig("int")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message from SMTS-INT client with client sender
	msg := &types.Message{
		ID:           "test-message-456",
		Timestamp:    time.Now().UTC(),
		Topic:        "monterra.event",
		Source:       "smts-int-client",
		ClientSender: "internal-app-1",
		Body:         []byte(`{"sensitive": "credit_card_data", "number": "4111111111111111"}`),
	}
	
	// Mock DLP validation rejection with multiple reasons
	dlpResponse := &types.DLPValidationResponse{
		Approved:  false,
		MessageID: "test-message-456",
		Reasons:   []string{"Contains credit card information", "Contains PII data"},
	}
	
	mockAPIClient.On("ValidateMessage", mock.Anything, msg).Return(dlpResponse, nil)
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "DLP validation failed")
	assert.Contains(t, result.Error, "Contains credit card information")
	assert.Contains(t, result.Error, "Contains PII data")
	mockAPIClient.AssertExpectations(t)
}

func TestProcessor_HandleMessage_INT_DLPError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for INT deployment
	config := mocks.CreateTestConfig("int")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"event": "test"}`),
	}
	
	// Mock DLP validation error
	mockAPIClient.On("ValidateMessage", mock.Anything, msg).Return(nil, types.NewSMTSError(types.ErrDLPConnection, "DLP service unavailable"))
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "DLP service unavailable")
	mockAPIClient.AssertExpectations(t)
}

func TestProcessor_HandleMessage_EXT_DeliveryError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for EXT deployment
	config := mocks.CreateTestConfig("ext")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"event": "test"}`),
	}
	
	// Mock delivery error
	deliveryResult := &types.DeliveryResult{
		Success:    false,
		MessageID:  "test-message-123",
		Timestamp:  time.Now().UTC(),
		Error:      "API connection failed",
	}
	
	mockAPIClient.On("DeliverMessage", mock.Anything, msg).Return(deliveryResult, types.NewSMTSError(types.ErrAPIConnection, "API connection failed"))
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "API connection failed")
	mockAPIClient.AssertExpectations(t)
}

func TestProcessor_HandleMessage_UnknownDeployment(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config with unknown deployment type
	config := mocks.CreateTestConfig("ext")
	config.Deployment.Type = "unknown"
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"event": "test"}`),
	}
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "Unknown deployment type")
}

func TestProcessor_ValidateTopic_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config with topics configuration
	config := mocks.CreateTestConfig("ext")
	config.Topics.Topics = map[string]types.TopicPermission{
		"monterra.event": {
			Description: "Monterra events",
		},
	}
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	err := processor.ValidateTopic("monterra.event")
	
	assert.NoError(t, err)
}

func TestProcessor_ValidateTopic_UnknownTopic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config with topics configuration
	config := mocks.CreateTestConfig("ext")
	config.Topics.Topics = map[string]types.TopicPermission{
		"monterra.event": {
			Description: "Monterra events",
		},
	}
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	err := processor.ValidateTopic("unknown.topic")
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Topic not configured")
}

func TestProcessor_HealthCheck_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config
	config := mocks.CreateTestConfig("ext")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Mock successful health checks
	mockAPIClient.On("HealthCheck", mock.Anything).Return(nil)
	mockNATSClient.On("HealthCheck", mock.Anything).Return(nil)
	
	err := processor.HealthCheck(context.Background())
	
	assert.NoError(t, err)
	mockAPIClient.AssertExpectations(t)
	mockNATSClient.AssertExpectations(t)
}

func TestProcessor_HealthCheck_APIError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config
	config := mocks.CreateTestConfig("ext")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Mock API health check failure
	mockAPIClient.On("HealthCheck", mock.Anything).Return(types.NewSMTSError(types.ErrHealthCheck, "API unavailable"))
	
	err := processor.HealthCheck(context.Background())
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API client health check failed")
	mockAPIClient.AssertExpectations(t)
}

func TestProcessor_HealthCheck_NATSError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config
	config := mocks.CreateTestConfig("ext")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Mock successful API health check but NATS failure
	mockAPIClient.On("HealthCheck", mock.Anything).Return(nil)
	mockNATSClient.On("HealthCheck", mock.Anything).Return(types.NewSMTSError(types.ErrHealthCheck, "NATS unavailable"))
	
	err := processor.HealthCheck(context.Background())
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NATS client health check failed")
	mockAPIClient.AssertExpectations(t)
	mockNATSClient.AssertExpectations(t)
}

func TestProcessor_HandleMessage_INT_DLPRejected_SecurityWarningLogging(t *testing.T) {
	// Create a test logger that captures log output
	var logBuffer bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(&logBuffer), zapcore.InfoLevel)
	logger := zap.New(core)
	
	// Create test config for INT deployment
	config := mocks.CreateTestConfig("int")
	// Add the test topic to the configuration
	config.Topics.Topics["monterra.security.event"] = types.TopicPermission{
		Description: "Security events",
	}
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
	
	// Create test message from SMTS-INT client with client sender
	msg := &types.Message{
		ID:           "test-message-security-789",
		Timestamp:    time.Now().UTC(),
		Topic:        "monterra.security.event",
		Source:       "smts-int-client",
		ClientSender: "security-app-2",
		Body:         []byte(`{"sensitive": "ssn_data", "ssn": "123-45-6789"}`),
	}
	
	// Mock DLP validation rejection with security-related reasons
	dlpResponse := &types.DLPValidationResponse{
		Approved:  false,
		MessageID: "test-message-security-789",
		Reasons:   []string{"Contains SSN information", "Violates data privacy policy"},
	}
	
	mockAPIClient.On("ValidateMessage", mock.Anything, msg).Return(dlpResponse, nil)
	
	result, err := processor.HandleMessage(context.Background(), msg)
	
	// Force logger to flush
	logger.Sync()
	
	// Parse the log output
	var logEntries []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
	for _, line := range lines {
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			logEntries = append(logEntries, logEntry)
		}
	}
	
	// Verify the result
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "DLP validation failed")
	mockAPIClient.AssertExpectations(t)
	
	// Find the security warning log entry
	var securityWarningLog map[string]interface{}
	for _, entry := range logEntries {
		if msg, ok := entry["msg"].(string); ok && msg == "Message rejected by DLP validation" {
			securityWarningLog = entry
			break
		}
	}
	
	// Verify security warning log fields
	assert.NotNil(t, securityWarningLog, "Security warning log entry should be present")
	assert.Equal(t, "dlp_validation", securityWarningLog["operation"])
	assert.Equal(t, "int", securityWarningLog["deployment"])
	assert.Equal(t, "dlp_rejection", securityWarningLog["security_event"])
	assert.Equal(t, "SECURITY_WARNING", securityWarningLog["log_type"])
	assert.Equal(t, "test-message-security-789", securityWarningLog["message_id"])
	assert.Equal(t, "monterra.security.event", securityWarningLog["topic"])
	assert.Equal(t, "security-app-2", securityWarningLog["client_sender"])
	
	// Verify rejection reasons
	reasons, ok := securityWarningLog["rejection_reasons"].([]interface{})
	assert.True(t, ok, "rejection_reasons should be present")
	assert.Len(t, reasons, 2)
	assert.Contains(t, reasons, "Contains SSN information")
	assert.Contains(t, reasons, "Violates data privacy policy")
}

func TestProcessor_DeploymentTypeMethods(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	tests := []struct {
		name           string
		deploymentType string
		isEXT          bool
		isINT          bool
	}{
		{
			name:           "EXT deployment",
			deploymentType: "ext",
			isEXT:          true,
			isINT:          false,
		},
		{
			name:           "INT deployment",
			deploymentType: "int",
			isEXT:          false,
			isINT:          true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := mocks.CreateTestConfig(tt.deploymentType)
			
			// Mock clients
			mockAPIClient := &mocks.MockAPIClient{}
			mockNATSClient := &mocks.MockNATSClient{}
			
			processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, nil, nil, logger)
			
			assert.Equal(t, tt.deploymentType, processor.GetDeploymentType())
			assert.Equal(t, tt.isEXT, processor.IsEXTDeployment())
			assert.Equal(t, tt.isINT, processor.IsINTDeployment())
		})
	}
}