package message_test

import (
	"context"
	"testing"
	"time"

	"github.com/corporate/smts/internal/message"
	"github.com/corporate/smts/pkg/types"
	"github.com/corporate/smts/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestProcessor_HandleMessage_EXT_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for EXT deployment
	config := mocks.CreateTestConfig("ext")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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

func TestProcessor_HandleMessage_INT_DLPError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config for INT deployment
	config := mocks.CreateTestConfig("int")
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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

func TestProcessor_ValidatePermissions_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config with topics configuration
	config := mocks.CreateTestConfig("ext")
	config.Topics.Topics = map[string]types.TopicPermission{
		"monterra.event": {
			ReadRoles:   []string{"ext_reader"},
			WriteRoles:  []string{"ext_writer"},
			Description: "Monterra events",
		},
	}
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
	// Create test message with configured topic
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "monterra.event",
		Source:    "test",
		Body:      []byte(`{"event": "test"}`),
	}
	
	err := processor.ValidatePermissions(msg)
	
	assert.NoError(t, err)
}

func TestProcessor_ValidatePermissions_UnknownTopic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config with topics configuration
	config := mocks.CreateTestConfig("ext")
	config.Topics.Topics = map[string]types.TopicPermission{
		"monterra.event": {
			ReadRoles:   []string{"ext_reader"},
			WriteRoles:  []string{"ext_writer"},
			Description: "Monterra events",
		},
	}
	
	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
	// Create test message with unknown topic
	msg := &types.Message{
		ID:        "test-message-123",
		Timestamp: time.Now().UTC(),
		Topic:     "unknown.topic",
		Source:    "test",
		Body:      []byte(`{"event": "test"}`),
	}
	
	err := processor.ValidatePermissions(msg)
	
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
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
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
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
	
	// Mock successful API health check but NATS failure
	mockAPIClient.On("HealthCheck", mock.Anything).Return(nil)
	mockNATSClient.On("HealthCheck", mock.Anything).Return(types.NewSMTSError(types.ErrHealthCheck, "NATS unavailable"))
	
	err := processor.HealthCheck(context.Background())
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NATS client health check failed")
	mockAPIClient.AssertExpectations(t)
	mockNATSClient.AssertExpectations(t)
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
			
			processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)
			
			assert.Equal(t, tt.deploymentType, processor.GetDeploymentType())
			assert.Equal(t, tt.isEXT, processor.IsEXTDeployment())
			assert.Equal(t, tt.isINT, processor.IsINTDeployment())
		})
	}
}

func TestProcessor_ValidatePermissions_Rejected(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config with topics configuration
	config := mocks.CreateTestConfig("ext")
	config.Topics.Topics = map[string]types.TopicPermission{
		"monterra.event": {
			ReadRoles:   []string{"ext_reader"}, // Only ext_reader can read
			WriteRoles:  []string{"ext_writer"},
			Description: "Monterra events",
		},
		"pact_update.event": {
			ReadRoles:   []string{"int_reader"}, // Only int_reader can read
			WriteRoles:  []string{"int_writer"},
			Description: "Pact update events",
		},
	}

	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)

	tests := []struct {
		name        string
		message     *types.Message
		expectedErr string
	}{
		{
			name: "INT reader trying to read EXT-only topic",
			message: &types.Message{
				ID:        "test-message-1",
				Timestamp: time.Now().UTC(),
				Topic:     "monterra.event",
				Source:    "int_smts", // This will map to int_reader role
				Body:      []byte(`{"event": "test"}`),
			},
			expectedErr: "Role does not have read permission for topic",
		},
		{
			name: "EXT reader trying to read INT-only topic",
			message: &types.Message{
				ID:        "test-message-2",
				Timestamp: time.Now().UTC(),
				Topic:     "pact_update.event",
				Source:    "ext_smts", // This will map to ext_reader role
				Body:      []byte(`{"update": "test"}`),
			},
			expectedErr: "Role does not have read permission for topic",
		},
		{
			name: "Unauthorized role via header",
			message: &types.Message{
				ID:        "test-message-3",
				Timestamp: time.Now().UTC(),
				Topic:     "monterra.event",
				Source:    "unknown_source",
				Headers:   map[string]string{"smts-role": "unauthorized_role"},
				Body:      []byte(`{"event": "test"}`),
			},
			expectedErr: "Role does not have read permission for topic",
		},
		{
			name: "Default deployment role without permission",
			message: &types.Message{
				ID:        "test-message-4",
				Timestamp: time.Now().UTC(),
				Topic:     "pact_update.event",
				Source:    "unknown_source", // Will default to ext_reader in EXT deployment
				Body:      []byte(`{"update": "test"}`),
			},
			expectedErr: "Role does not have read permission for topic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidatePermissions(tt.message)
			
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
			assert.Contains(t, err.Error(), "PERMISSION_DENIED_ERROR")
		})
	}
}

func TestProcessor_ValidatePermissions_RoleExtraction(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create test config with topics configuration
	config := mocks.CreateTestConfig("ext")
	config.Topics.Topics = map[string]types.TopicPermission{
		"monterra.event": {
			ReadRoles:   []string{"ext_reader", "int_reader"},
			WriteRoles:  []string{"ext_writer", "int_writer"},
			Description: "Monterra events",
		},
	}

	// Mock API client
	mockAPIClient := &mocks.MockAPIClient{}
	
	// Mock NATS client
	mockNATSClient := &mocks.MockNATSClient{}
	
	processor := message.NewProcessor(config, mockAPIClient, mockNATSClient, logger)

	tests := []struct {
		name           string
		message        *types.Message
		expectedRole   string
		shouldSucceed  bool
	}{
		{
			name: "Role from header takes precedence",
			message: &types.Message{
				ID:        "test-message-1",
				Timestamp: time.Now().UTC(),
				Topic:     "monterra.event",
				Source:    "ext_smts",
				Headers:   map[string]string{"smts-role": "int_reader"},
				Body:      []byte(`{"event": "test"}`),
			},
			expectedRole:  "int_reader",
			shouldSucceed: true,
		},
		{
			name: "Role from ext_smts source",
			message: &types.Message{
				ID:        "test-message-2",
				Timestamp: time.Now().UTC(),
				Topic:     "monterra.event",
				Source:    "ext_smts",
				Body:      []byte(`{"event": "test"}`),
			},
			expectedRole:  "ext_reader",
			shouldSucceed: true,
		},
		{
			name: "Role from int_smts source",
			message: &types.Message{
				ID:        "test-message-3",
				Timestamp: time.Now().UTC(),
				Topic:     "monterra.event",
				Source:    "int_smts",
				Body:      []byte(`{"event": "test"}`),
			},
			expectedRole:  "int_reader",
			shouldSucceed: true,
		},
		{
			name: "Role from artemis source",
			message: &types.Message{
				ID:        "test-message-4",
				Timestamp: time.Now().UTC(),
				Topic:     "monterra.event",
				Source:    "artemis",
				Body:      []byte(`{"event": "test"}`),
			},
			expectedRole:  "int_reader",
			shouldSucceed: true,
		},
		{
			name: "Role from smts-publisher in EXT deployment",
			message: &types.Message{
				ID:        "test-message-5",
				Timestamp: time.Now().UTC(),
				Topic:     "monterra.event",
				Source:    "smts-publisher",
				Body:      []byte(`{"event": "test"}`),
			},
			expectedRole:  "ext_reader",
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidatePermissions(tt.message)
			
			if tt.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Note: formatDLPReasons is a private method, so we can't test it directly
// The functionality is tested indirectly through the DLP rejection tests