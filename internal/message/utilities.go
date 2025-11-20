package message

import (
	"context"
	"time"

	"smts/internal/nats"
	"smts/pkg/types"
	"smts/pkg/utils"
	"go.uber.org/zap"
)

// MessageUtilities provides shared functionality for message processing
type MessageUtilities struct {
	config        *types.Config
	natsPublisher *nats.Publisher
	logger        *zap.Logger
}

// NewMessageUtilities creates a new message utilities instance
func NewMessageUtilities(config *types.Config, natsPublisher *nats.Publisher, logger *zap.Logger) *MessageUtilities {
	return &MessageUtilities{
		config:        config,
		natsPublisher: natsPublisher,
		logger:        logger,
	}
}

// StoreMessageInExternalStream stores a message in the external NATS stream
// This is used by both Flow 1 (INT from Artemis) and Flow 2 (EXT from Corporate API)
func (u *MessageUtilities) StoreMessageInExternalStream(ctx context.Context, msg *types.Message, source string) (*types.DeliveryResult, error) {
	operation := "store_external_stream"
	
	u.logger.Info("Storing message in external NATS stream",
		append(utils.LoggerFields(operation, u.config.Deployment.Type, msg.ID, msg.Topic),
			zap.String("source", source))...)

	// Create a copy of the message with the correct subject for external stream
	externalMsg := &types.Message{
		ID:           msg.ID,
		Timestamp:    msg.Timestamp,
		Topic:        "external." + msg.Topic, // Use external subject pattern
		Source:       source,
		ClientSender: msg.ClientSender,
		Headers:      msg.Headers,
		Body:         msg.Body, // Store only the actual content, not the full message structure
	}

	u.logger.Info("Publishing message to external NATS stream",
		append(utils.LoggerFields("publish_ext_stream", u.config.Deployment.Type, msg.ID, msg.Topic),
			zap.String("external_topic", externalMsg.Topic),
			zap.String("stream", u.config.NATS.ExternalStream.Name),
			zap.String("source", source))...)

	// Store message in external NATS stream for clients to consume via REST API
	if err := u.natsPublisher.PublishMessageToStream(externalMsg, u.config.NATS.ExternalStream.Name); err != nil {
		u.logger.Error("Failed to publish message to external NATS stream",
			append(utils.LoggerFields("publish_ext_stream", u.config.Deployment.Type, msg.ID, msg.Topic),
				utils.WithError(err))...)
		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}

	u.logger.Info("Message stored in external NATS stream",
		append(utils.LoggerFields("publish_ext_stream", u.config.Deployment.Type, msg.ID, msg.Topic),
			zap.String("external_topic", externalMsg.Topic),
			zap.String("source", source),
			zap.String("stream", u.config.NATS.ExternalStream.Name))...)

	return &types.DeliveryResult{
		Success:   true,
		MessageID: msg.ID,
		Timestamp: time.Now().UTC(),
	}, nil
}

// HandleProcessingError provides unified error handling for message processing
func (u *MessageUtilities) HandleProcessingError(operation string, deployment string, msg *types.Message, err error) {
	if types.IsRetryableError(err) {
		u.logger.Info("Retryable error in message processing",
			append(utils.LoggerFields(operation, deployment, msg.ID, msg.Topic),
				utils.WithError(err))...)
	} else {
		u.logger.Error("Non-retryable error in message processing",
			append(utils.LoggerFields(operation, deployment, msg.ID, msg.Topic),
				utils.WithError(err))...)
	}
}

// CreateDeliveryResult creates a standardized delivery result
func (u *MessageUtilities) CreateDeliveryResult(success bool, messageID string, err error) *types.DeliveryResult {
	result := &types.DeliveryResult{
		Success:   success,
		MessageID: messageID,
		Timestamp: time.Now().UTC(),
	}
	
	if err != nil {
		result.Error = err.Error()
	}
	
	return result
}