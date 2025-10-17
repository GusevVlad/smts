package message

import (
	"context"
	"strings"
	"time"

	"smts/internal/api"
	"smts/internal/nats"
	"smts/pkg/types"
	"smts/pkg/utils"
	"go.uber.org/zap"
)

// ArtemisPublisher defines the interface for publishing messages to ArtemisMQ
type ArtemisPublisher interface {
	PublishMessage(msg *types.Message) error
}

// Processor handles message processing for SMTS
type Processor struct {
	config          *types.Config
	apiClient       api.APIClient
	natsClient      nats.NATSClient
	natsPublisher   *nats.Publisher
	artemisPublisher ArtemisPublisher
	logger          *zap.Logger
	deployment      string
}

// NewProcessor creates a new message processor
func NewProcessor(config *types.Config, apiClient api.APIClient, natsClient nats.NATSClient, natsPublisher *nats.Publisher, artemisPublisher ArtemisPublisher, logger *zap.Logger) *Processor {
	return &Processor{
		config:          config,
		apiClient:       apiClient,
		natsClient:      natsClient,
		natsPublisher:   natsPublisher,
		artemisPublisher: artemisPublisher,
		logger:          logger,
		deployment:      config.Deployment.Type,
	}
}

// HandleMessage processes a single message according to the deployment type
func (p *Processor) HandleMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	operation := "process_message"
	startTime := time.Now()

	p.logger.Info("Starting message processing",
		append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
			zap.String("source", msg.Source),
			zap.String("client_sender", msg.ClientSender),
			zap.Time("message_timestamp", msg.Timestamp))...)

	// Skip topic validation for external topics (they use external.* pattern)
	if !strings.HasPrefix(msg.Topic, "external.") {
		// Validate topic configuration for non-external topics
		if err := p.ValidateTopic(msg.Topic); err != nil {
			p.logger.Warn("Topic validation failed",
				append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
					utils.WithError(err))...)

			return &types.DeliveryResult{
				Success:    false,
				MessageID:  msg.ID,
				Timestamp:  time.Now().UTC(),
				Error:      err.Error(),
			}, err
		}
	}

	// Process based on deployment type and source
	var result *types.DeliveryResult
	var err error

	switch p.deployment {
	case "ext":
		// For EXT deployment, check if message is from corporate API (Flow 2)
		isFromCorporateAPI := msg.Source == "corporate-api" || msg.Headers["smts-source"] == "corporate-api" || msg.Headers["X-SMTS-Source"] == "corporate-api"
		
		p.logger.Debug("EXT message source detection",
			zap.String("message_id", msg.ID),
			zap.String("source", msg.Source),
			zap.String("smts-source", msg.Headers["smts-source"]),
			zap.String("x-smts-source", msg.Headers["X-SMTS-Source"]),
			zap.Bool("is_from_corporate_api", isFromCorporateAPI))
		
		if isFromCorporateAPI {
			// Flow 2: Message from corporate API (via ArtemisMQ) - store in external NATS stream for external clients
			p.logger.Info("Processing EXT message from corporate API (Flow 2)",
				append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
					zap.String("source", msg.Source))...)
			result, err = p.processEXTMessageFromCorporateAPI(ctx, msg)
		} else {
			// Flow 1: Message from NATS (external client) - deliver to corporate API
			p.logger.Info("Processing EXT message from NATS client (Flow 1)",
				append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
					zap.String("source", msg.Source))...)
			result, err = p.processEXTMessage(ctx, msg)
		}
	case "int":
		// For INT deployment, check if message is from ArtemisMQ (Flow 1)
		isFromArtemis := msg.Source == "artemis" || msg.Headers["smts-source"] == "artemis" || msg.Headers["X-SMTS-Source"] == "artemis"
		
		p.logger.Debug("INT message source detection",
			zap.String("message_id", msg.ID),
			zap.String("source", msg.Source),
			zap.String("smts-source", msg.Headers["smts-source"]),
			zap.String("x-smts-source", msg.Headers["X-SMTS-Source"]),
			zap.Bool("is_from_artemis", isFromArtemis))
		
		if isFromArtemis {
			// Flow 1: Message from ArtemisMQ (via corporate API) - store in NATS for internal clients
			p.logger.Info("Processing INT message from ArtemisMQ (Flow 1)",
				append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
					zap.String("source", msg.Source))...)
			result, err = p.processINTMessageFromArtemis(ctx, msg)
		} else {
			// Flow 2: Message from NATS (internal client) - publish to ArtemisMQ
			p.logger.Info("Processing INT message from NATS client (Flow 2)",
				append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
					zap.String("source", msg.Source))...)
			result, err = p.processINTMessageToArtemis(ctx, msg)
		}
	default:
		err = types.NewSMTSError(types.ErrMessageRouting,
			"Unknown deployment type: "+p.deployment)
		result = &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}
	}

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		p.logger.Error("Message processing failed",
			append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
				utils.WithError(err),
				utils.WithDuration(duration))...)
	} else if result.Success {
		p.logger.Info("Message processed successfully",
			append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
				utils.WithDuration(duration))...)
	} else {
		p.logger.Warn("Message processing completed with failure",
			append(utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic),
				zap.String("error", result.Error),
				utils.WithDuration(duration))...)
	}

	return result, err
}

// processEXTMessage processes messages for EXT deployment (direct delivery)
func (p *Processor) processEXTMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	p.logger.Info("Delivering EXT message to corporate API",
		append(utils.LoggerFields("ext_deliver", p.deployment, msg.ID, msg.Topic),
			zap.String("source", msg.Source))...)
	// EXT deployment: Direct delivery to corporate API
	return p.apiClient.DeliverMessage(ctx, msg)
}

// processINTMessageToArtemis processes messages for INT deployment that need to go to ArtemisMQ (Flow 2)
func (p *Processor) processINTMessageToArtemis(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	p.logger.Info("Processing INT message for ArtemisMQ delivery",
		append(utils.LoggerFields("int_to_artemis", p.deployment, msg.ID, msg.Topic),
			zap.String("source", msg.Source))...)
	// INT deployment: DLP validation followed by delivery to ArtemisMQ
	
	// Step 1: DLP validation
	if p.config.DLP.Enabled {
		p.logger.Info("Performing DLP validation for INT message",
			append(utils.LoggerFields("dlp_validation", p.deployment, msg.ID, msg.Topic),
				zap.String("source", msg.Source))...)
		dlpResult, err := p.apiClient.ValidateMessage(ctx, msg)
		if err != nil {
			p.logger.Warn("DLP validation failed",
				append(utils.LoggerFields("dlp_validation", p.deployment, msg.ID, msg.Topic),
					utils.WithError(err))...)
			return &types.DeliveryResult{
				Success:    false,
				MessageID:  msg.ID,
				Timestamp:  time.Now().UTC(),
				Error:      err.Error(),
			}, err
		}

		if !dlpResult.Approved {
			p.logger.Warn("Message rejected by DLP validation",
				append(utils.SecurityWarningFields("dlp_validation", p.deployment, msg.ID, msg.Topic, msg.ClientSender, "dlp_rejection"),
					zap.Strings("rejection_reasons", dlpResult.Reasons))...)
			return &types.DeliveryResult{
				Success:    false,
				MessageID:  msg.ID,
				Timestamp:  time.Now().UTC(),
				Error:      "DLP validation failed: " + p.formatDLPReasons(dlpResult.Reasons),
			}, types.NewSMTSError(types.ErrDLPValidation, "Message rejected by DLP")
		}
		p.logger.Info("DLP validation passed",
			append(utils.LoggerFields("dlp_validation", p.deployment, msg.ID, msg.Topic),
				zap.String("source", msg.Source))...)
	}

	// Step 2: For INT deployment, publish to ArtemisMQ instead of direct delivery
	// This enables Flow 2: INT → ArtemisMQ → Corporate API → EXT
	if p.artemisPublisher != nil {
		p.logger.Info("Publishing INT message to ArtemisMQ",
			append(utils.LoggerFields("publish_artemis", p.deployment, msg.ID, msg.Topic),
				zap.String("queue", p.config.Artemis.Queue),
				zap.String("source", msg.Source))...)
		if err := p.artemisPublisher.PublishMessage(msg); err != nil {
			p.logger.Error("Failed to publish message to ArtemisMQ",
				append(utils.LoggerFields("publish_artemis", p.deployment, msg.ID, msg.Topic),
					utils.WithError(err))...)
			return &types.DeliveryResult{
				Success:    false,
				MessageID:  msg.ID,
				Timestamp:  time.Now().UTC(),
				Error:      err.Error(),
			}, err
		}
		
		p.logger.Info("Message published to ArtemisMQ",
			append(utils.LoggerFields("publish_artemis", p.deployment, msg.ID, msg.Topic),
				zap.String("queue", p.config.Artemis.Queue),
				zap.String("source", msg.Source))...)
		
		return &types.DeliveryResult{
			Success:   true,
			MessageID: msg.ID,
			Timestamp: time.Now().UTC(),
		}, nil
	}
	
	// Fallback: If Artemis publisher is not available, use API client
	return p.apiClient.DeliverMessage(ctx, msg)
}

// processEXTMessageFromCorporateAPI processes messages for EXT deployment that come from corporate API (Flow 2)
func (p *Processor) processEXTMessageFromCorporateAPI(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	p.logger.Info("Processing EXT message from corporate API for external stream",
		append(utils.LoggerFields("ext_from_corp_api", p.deployment, msg.ID, msg.Topic),
			zap.String("source", msg.Source))...)
	
	// Flow 2: Message from corporate API (via ArtemisMQ) - store in external NATS stream for external clients
	
	// Create a copy of the message with the correct subject for external stream
	// Store only the actual content in the body to avoid duplication
	externalMsg := &types.Message{
		ID:           msg.ID,
		Timestamp:    msg.Timestamp,
		Topic:        "external." + msg.Topic,  // Use external subject pattern
		Source:       msg.Source,
		ClientSender: msg.ClientSender,
		Headers:      msg.Headers,
		Body:         msg.Body,  // Store only the actual content, not the full message structure
	}
	
	p.logger.Info("Publishing EXT message to external NATS stream",
		append(utils.LoggerFields("publish_ext_stream", p.deployment, msg.ID, msg.Topic),
			zap.String("external_topic", externalMsg.Topic),
			zap.String("stream", p.config.NATS.ExternalStream.Name),
			zap.String("source", msg.Source))...)
	
	// Store message in external NATS stream for external clients to consume via REST API
	if err := p.natsPublisher.PublishMessageToStream(externalMsg, p.config.NATS.ExternalStream.Name); err != nil {
		p.logger.Error("Failed to publish message to external NATS stream",
			append(utils.LoggerFields("publish_ext_stream", p.deployment, msg.ID, msg.Topic),
				utils.WithError(err))...)
		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}
	
	p.logger.Info("Message stored in external NATS stream from corporate API",
		append(utils.LoggerFields("publish_ext_stream", p.deployment, msg.ID, msg.Topic),
			zap.String("external_topic", externalMsg.Topic),
			zap.String("source", msg.Source),
			zap.String("stream", p.config.NATS.ExternalStream.Name))...)
	
	return &types.DeliveryResult{
		Success:   true,
		MessageID: msg.ID,
		Timestamp: time.Now().UTC(),
	}, nil
}

// processINTMessageFromArtemis processes messages for INT deployment that come from ArtemisMQ (Flow 1)
func (p *Processor) processINTMessageFromArtemis(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	p.logger.Info("Processing INT message from ArtemisMQ for external stream",
		append(utils.LoggerFields("int_from_artemis", p.deployment, msg.ID, msg.Topic),
			zap.String("source", msg.Source))...)
	
	// Flow 1: Message from ArtemisMQ (via corporate API) - store in external NATS stream for internal clients
	
	// Create a copy of the message with the correct subject for external stream
	// Store only the actual content in the body to avoid duplication
	externalMsg := &types.Message{
		ID:           msg.ID,
		Timestamp:    msg.Timestamp,
		Topic:        "external." + msg.Topic,  // Use external subject pattern
		Source:       msg.Source,
		ClientSender: msg.ClientSender,
		Headers:      msg.Headers,
		Body:         msg.Body,  // Store only the actual content, not the full message structure
	}
	
	p.logger.Info("Publishing INT message to external NATS stream",
		append(utils.LoggerFields("publish_ext_stream", p.deployment, msg.ID, msg.Topic),
			zap.String("external_topic", externalMsg.Topic),
			zap.String("stream", p.config.NATS.ExternalStream.Name),
			zap.String("source", msg.Source))...)
	
	// Store message in external NATS stream for internal clients to consume via REST API
	if err := p.natsPublisher.PublishMessageToStream(externalMsg, p.config.NATS.ExternalStream.Name); err != nil {
		p.logger.Error("Failed to publish message to external NATS stream",
			append(utils.LoggerFields("publish_ext_stream", p.deployment, msg.ID, msg.Topic),
				utils.WithError(err))...)
		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}
	
	p.logger.Info("Message stored in external NATS stream from ArtemisMQ",
		append(utils.LoggerFields("publish_ext_stream", p.deployment, msg.ID, msg.Topic),
			zap.String("external_topic", externalMsg.Topic),
			zap.String("source", msg.Source),
			zap.String("stream", p.config.NATS.ExternalStream.Name))...)
	
	return &types.DeliveryResult{
		Success:   true,
		MessageID: msg.ID,
		Timestamp: time.Now().UTC(),
	}, nil
}

// ValidateTopic checks if the topic is configured
func (p *Processor) ValidateTopic(topic string) error {
	// Check if topic is configured
	_, exists := p.config.Topics.Topics[topic]
	if !exists {
		return types.NewSMTSErrorWithDetails(
			types.ErrPermissionDenied,
			"Topic not configured",
			"topic: "+topic,
		)
	}

	p.logger.Debug("Topic validated",
		zap.String("topic", topic))

	return nil
}

// formatDLPReasons formats DLP rejection reasons for logging
func (p *Processor) formatDLPReasons(reasons []string) string {
	if len(reasons) == 0 {
		return "no specific reasons provided"
	}

	result := ""
	for i, reason := range reasons {
		if i > 0 {
			result += "; "
		}
		result += reason
	}
	return result
}

// HealthCheck performs a health check on the processor
func (p *Processor) HealthCheck(ctx context.Context) error {
	// Check API client health
	if err := p.apiClient.HealthCheck(ctx); err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "API client health check failed")
	}

	// Check NATS client health
	if err := p.natsClient.HealthCheck(ctx); err != nil {
		return types.WrapSMTSError(err, types.ErrHealthCheck, "NATS client health check failed")
	}

	return nil
}

// GetDeploymentType returns the processor's deployment type
func (p *Processor) GetDeploymentType() string {
	return p.deployment
}

// IsEXTDeployment returns true if this is an EXT deployment
func (p *Processor) IsEXTDeployment() bool {
	return p.deployment == "ext"
}

// IsINTDeployment returns true if this is an INT deployment
func (p *Processor) IsINTDeployment() bool {
	return p.deployment == "int"
}