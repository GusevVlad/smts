package message

import (
	"context"
	"time"

	"github.com/corporate/smts/internal/api"
	"github.com/corporate/smts/internal/nats"
	"github.com/corporate/smts/pkg/types"
	"github.com/corporate/smts/pkg/utils"
	"go.uber.org/zap"
)

// Processor handles message processing for SMTS
type Processor struct {
	config      *types.Config
	apiClient   *api.Client
	natsClient  *nats.Client
	logger      *zap.Logger
	deployment  string
}

// NewProcessor creates a new message processor
func NewProcessor(config *types.Config, apiClient *api.Client, natsClient *nats.Client, logger *zap.Logger) *Processor {
	return &Processor{
		config:     config,
		apiClient:  apiClient,
		natsClient: natsClient,
		logger:     logger,
		deployment: config.Deployment.Type,
	}
}

// HandleMessage processes a single message according to the deployment type
func (p *Processor) HandleMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	operation := "process_message"
	startTime := time.Now()

	// Validate message permissions
	if err := p.validatePermissions(msg); err != nil {
		p.logger.Warn("Message permission validation failed",
			utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic)...,
			utils.WithError(err))

		return &types.DeliveryResult{
			Success:    false,
			MessageID:  msg.ID,
			Timestamp:  time.Now().UTC(),
			Error:      err.Error(),
		}, err
	}

	// Process based on deployment type
	var result *types.DeliveryResult
	var err error

	switch p.deployment {
	case "ext":
		result, err = p.processEXTMessage(ctx, msg)
	case "int":
		result, err = p.processINTMessage(ctx, msg)
	default:
		err = types.NewSMTSError(types.ErrMessageRouting, 
			"Unknown deployment type: "+p.deployment)
	}

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		p.logger.Error("Message processing failed",
			utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic)...,
			utils.WithError(err),
			utils.WithDuration(duration))
	} else if result.Success {
		p.logger.Info("Message processed successfully",
			utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic)...,
			utils.WithDuration(duration))
	} else {
		p.logger.Warn("Message processing completed with failure",
			utils.LoggerFields(operation, p.deployment, msg.ID, msg.Topic)...,
			zap.String("error", result.Error),
			utils.WithDuration(duration))
	}

	return result, err
}

// processEXTMessage processes messages for EXT deployment (direct delivery)
func (p *Processor) processEXTMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	// EXT deployment: Direct delivery to corporate API
	return p.apiClient.DeliverMessage(ctx, msg)
}

// processINTMessage processes messages for INT deployment (with DLP validation)
func (p *Processor) processINTMessage(ctx context.Context, msg *types.Message) (*types.DeliveryResult, error) {
	// INT deployment: DLP validation followed by delivery
	
	// Step 1: DLP validation
	if p.config.DLP.Enabled {
		dlpResult, err := p.apiClient.ValidateMessage(ctx, msg)
		if err != nil {
			return &types.DeliveryResult{
				Success:    false,
				MessageID:  msg.ID,
				Timestamp:  time.Now().UTC(),
				Error:      err.Error(),
			}, err
		}

		if !dlpResult.Approved {
			return &types.DeliveryResult{
				Success:    false,
				MessageID:  msg.ID,
				Timestamp:  time.Now().UTC(),
				Error:      "DLP validation failed: " + p.formatDLPReasons(dlpResult.Reasons),
			}, types.NewSMTSError(types.ErrDLPValidation, "Message rejected by DLP")
		}
	}

	// Step 2: Deliver to corporate API
	return p.apiClient.DeliverMessage(ctx, msg)
}

// validatePermissions checks if the message has permission to be processed
func (p *Processor) validatePermissions(msg *types.Message) error {
	// Check if topic is configured
	topicConfig, exists := p.config.Topics.Topics[msg.Topic]
	if !exists {
		return types.NewSMTSErrorWithDetails(
			types.ErrPermissionDenied,
			"Topic not configured",
			"topic: "+msg.Topic,
		)
	}

	// For now, we'll assume all messages from NATS have read permission
	// In a real implementation, you would check the source/role against the topic's read roles
	
	p.logger.Debug("Message permission validated",
		zap.String("topic", msg.Topic),
		zap.String("message_id", msg.ID))

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