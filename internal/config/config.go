package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"smts/pkg/types"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Loader handles configuration loading and validation
type Loader struct {
	logger *zap.Logger
}

// NewLoader creates a new configuration loader
func NewLoader(logger *zap.Logger) *Loader {
	return &Loader{
		logger: logger,
	}
}

// LoadConfig loads configuration from file and environment variables
func (l *Loader) LoadConfig(configPath string) (*types.Config, error) {
	// Set default configuration
	config := types.DefaultConfig()

	// Initialize Viper
	v := viper.New()
	v.SetConfigType("yaml")

	// Set configuration file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Default configuration paths
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// Bind environment variables
	v.SetEnvPrefix("SMTS")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			l.logger.Info("No configuration file found, using defaults and environment variables")
		} else {
			return nil, types.WrapSMTSError(err, types.ErrConfigLoad, types.MsgConfigLoadFailed)
		}
	} else {
		l.logger.Info("Configuration file loaded", zap.String("file", v.ConfigFileUsed()))
	}

	// Debug: Check if topics key exists in the configuration
	if v.IsSet("topics") {
		l.logger.Debug("Topics key found in configuration", zap.String("deployment", config.Deployment.Type))
		topicsMap := v.GetStringMap("topics")
		l.logger.Debug("Topics map content", zap.Any("topics", topicsMap), zap.Int("topics_count", len(topicsMap)))
	} else {
		l.logger.Debug("Topics key NOT found in configuration", zap.String("deployment", config.Deployment.Type))
	}

	// Unmarshal configuration
	if err := v.Unmarshal(config); err != nil {
		return nil, types.WrapSMTSError(err, types.ErrConfigLoad, "Failed to unmarshal configuration")
	}

	// Debug: Try to manually parse topics configuration
	if v.IsSet("topics") {
		topicsMap := v.GetStringMap("topics")
		l.logger.Debug("Raw topics map", zap.Any("topics", topicsMap), zap.Int("count", len(topicsMap)))
		
		// Manually parse topics configuration
		manuallyParsedTopics := make(map[string]types.TopicPermission)
		
		// Parse topics
		for topicName, topicData := range topicsMap {
			if topicMap, ok := topicData.(map[string]interface{}); ok {
				topicPermission := types.TopicPermission{}
				
				// Extract description
				if desc, ok := topicMap["description"].(string); ok {
					topicPermission.Description = desc
				}
				
				manuallyParsedTopics[topicName] = topicPermission
			}
		}
		
		l.logger.Debug("Manually parsed topics",
			zap.Any("topics", manuallyParsedTopics),
			zap.Int("topics_count", len(manuallyParsedTopics)))
		
		// If manual parsing worked, use the manually parsed topics
		if len(manuallyParsedTopics) > 0 {
			config.Topics.Topics = manuallyParsedTopics
			l.logger.Debug("Using manually parsed topics configuration")
		}
	}

	// Debug: Log topics configuration after unmarshaling
	l.logger.Debug("Configuration unmarshaled",
		zap.Int("topics_count", len(config.Topics.Topics)),
		zap.String("deployment", config.Deployment.Type))
	
	// Debug: Log all topics found
	for topicName, topic := range config.Topics.Topics {
		l.logger.Debug("Found topic in config",
			zap.String("topic", topicName),
			zap.String("description", topic.Description),
			zap.String("deployment", config.Deployment.Type))
	}

	// Validate configuration
	if err := l.ValidateConfig(config); err != nil {
		return nil, err
	}

	// Apply deployment-specific defaults
	l.applyDeploymentDefaults(config)

	l.logger.Info("Configuration loaded successfully", 
		zap.String("deployment", config.Deployment.Type),
		zap.String("environment", config.Deployment.Environment))

	return config, nil
}

// ValidateConfig validates the configuration
func (l *Loader) ValidateConfig(config *types.Config) error {
	// Validate deployment type
	if config.Deployment.Type != "ext" && config.Deployment.Type != "int" {
		return types.NewSMTSErrorWithDetails(
			types.ErrConfigValidate,
			"Invalid deployment type",
			fmt.Sprintf("deployment type must be 'ext' or 'int', got '%s'", config.Deployment.Type),
		)
	}

	// Validate NATS configuration
	if config.NATS.Host == "" {
		return types.NewSMTSError(types.ErrConfigValidate, "NATS host is required")
	}
	if config.NATS.Port <= 0 || config.NATS.Port > 65535 {
		return types.NewSMTSErrorWithDetails(
			types.ErrConfigValidate,
			"Invalid NATS port",
			fmt.Sprintf("port must be between 1 and 65535, got %d", config.NATS.Port),
		)
	}

	// Validate API configuration
	if config.API.BaseURL == "" {
		return types.NewSMTSError(types.ErrConfigValidate, "API base URL is required")
	}
	if config.API.Auth.Type == "api_key" && config.API.Auth.APIKey == "" {
		return types.NewSMTSError(types.ErrConfigValidate, "API key is required for API authentication")
	}

	// Validate DLP configuration for INT deployment
	if config.Deployment.Type == "int" && config.DLP.Enabled {
		if config.DLP.Endpoint == "" {
			return types.NewSMTSError(types.ErrConfigValidate, "DLP endpoint is required for INT deployment")
		}
	}

	// Validate Artemis configuration for INT deployment
	if config.Deployment.Type == "int" && config.Artemis.Enabled {
		if config.Artemis.Host == "" {
			return types.NewSMTSError(types.ErrConfigValidate, "Artemis host is required for INT deployment")
		}
		if config.Artemis.Queue == "" {
			return types.NewSMTSError(types.ErrConfigValidate, "Artemis queue name is required for INT deployment")
		}
	}

	// Validate topics configuration
	if err := l.validateTopics(config.Topics, config.Deployment.Type); err != nil {
		return err
	}

	return nil
}

// validateTopics validates the topics configuration
func (l *Loader) validateTopics(topics types.TopicsConfig, deploymentType string) error {
	if len(topics.Topics) == 0 {
		l.logger.Warn("No topics configured, service will not process any messages",
			zap.String("deployment", deploymentType))
	}

	for topicName, topic := range topics.Topics {
		if topicName == "" {
			return types.NewSMTSError(types.ErrConfigValidate, "Topic name cannot be empty")
		}
		// Topic validation is now simplified - only check if topic exists
		l.logger.Debug("Topic validated",
			zap.String("topic", topicName),
			zap.String("description", topic.Description))
	}

	return nil
}

// applyDeploymentDefaults applies deployment-specific default settings
func (l *Loader) applyDeploymentDefaults(config *types.Config) {
	// Set stream and consumer names based on deployment type
	if config.NATS.ClientStream.Name == "SMTS_CLIENT" {
		config.NATS.ClientStream.Name = fmt.Sprintf("SMTS_%s_CLIENT", strings.ToUpper(config.Deployment.Type))
	}
	if config.NATS.ClientConsumer.DurableName == "SMTS_CLIENT_CONSUMER" {
		config.NATS.ClientConsumer.DurableName = fmt.Sprintf("SMTS_%s_CLIENT_CONSUMER", strings.ToUpper(config.Deployment.Type))
	}
	if config.NATS.ExternalStream.Name == "SMTS_EXTERNAL" {
		config.NATS.ExternalStream.Name = fmt.Sprintf("SMTS_%s_EXTERNAL", strings.ToUpper(config.Deployment.Type))
	}
	if config.NATS.ExternalConsumer.DurableName == "SMTS_EXTERNAL_CONSUMER" {
		config.NATS.ExternalConsumer.DurableName = fmt.Sprintf("SMTS_%s_EXTERNAL_CONSUMER", strings.ToUpper(config.Deployment.Type))
	}

	// Enable/disable features based on deployment type
	if config.Deployment.Type == "int" {
		config.DLP.Enabled = true
		config.Artemis.Enabled = true
	} else {
		config.DLP.Enabled = false
		config.Artemis.Enabled = false
	}

	l.logger.Debug("Applied deployment-specific defaults",
		zap.String("client_stream", config.NATS.ClientStream.Name),
		zap.String("client_consumer", config.NATS.ClientConsumer.DurableName),
		zap.String("external_stream", config.NATS.ExternalStream.Name),
		zap.String("external_consumer", config.NATS.ExternalConsumer.DurableName),
		zap.Bool("dlp_enabled", config.DLP.Enabled),
		zap.Bool("artemis_enabled", config.Artemis.Enabled))
}

// LoadTopicsConfig loads topics configuration from a separate file
func (l *Loader) LoadTopicsConfig(topicsPath string, deploymentType string) (*types.TopicsConfig, error) {
	if topicsPath == "" {
		// Try default locations
		defaultPaths := []string{
			"./configs/topics.yaml",
			"./topics.yaml",
			filepath.Join(filepath.Dir(viper.GetViper().ConfigFileUsed()), "topics.yaml"),
		}

		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				topicsPath = path
				break
			}
		}

		if topicsPath == "" {
			l.logger.Warn("No topics configuration file found, using empty configuration",
				zap.String("deployment", deploymentType))
			return &types.TopicsConfig{
				Topics: make(map[string]types.TopicPermission),
			}, nil
		}
	}

	v := viper.New()
	v.SetConfigFile(topicsPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, types.WrapSMTSError(err, types.ErrConfigLoad, "Failed to load topics configuration")
	}

	var topicsConfig types.TopicsConfig
	if err := v.Unmarshal(&topicsConfig); err != nil {
		return nil, types.WrapSMTSError(err, types.ErrConfigLoad, "Failed to unmarshal topics configuration")
	}

	l.logger.Info("Topics configuration loaded", zap.String("file", topicsPath))
	return &topicsConfig, nil
}