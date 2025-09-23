package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corporate/smts/pkg/types"
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

	// Unmarshal configuration
	if err := v.Unmarshal(config); err != nil {
		return nil, types.WrapSMTSError(err, types.ErrConfigLoad, "Failed to unmarshal configuration")
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
	if err := l.validateTopics(config.Topics); err != nil {
		return err
	}

	return nil
}

// validateTopics validates the topics configuration
func (l *Loader) validateTopics(topics types.TopicsConfig) error {
	if len(topics.Topics) == 0 {
		l.logger.Warn("No topics configured, service will not process any messages")
	}

	for topicName, topic := range topics.Topics {
		if topicName == "" {
			return types.NewSMTSError(types.ErrConfigValidate, "Topic name cannot be empty")
		}
		if len(topic.ReadRoles) == 0 {
			return types.NewSMTSErrorWithDetails(
				types.ErrConfigValidate,
				"Topic must have at least one read role",
				fmt.Sprintf("topic: %s", topicName),
			)
		}
		if len(topic.WriteRoles) == 0 {
			return types.NewSMTSErrorWithDetails(
				types.ErrConfigValidate,
				"Topic must have at least one write role",
				fmt.Sprintf("topic: %s", topicName),
			)
		}

		// Validate that referenced roles exist
		for _, role := range append(topic.ReadRoles, topic.WriteRoles...) {
			if _, exists := topics.Roles[role]; !exists {
				return types.NewSMTSErrorWithDetails(
					types.ErrConfigValidate,
					"Referenced role does not exist",
					fmt.Sprintf("topic: %s, role: %s", topicName, role),
				)
			}
		}
	}

	return nil
}

// applyDeploymentDefaults applies deployment-specific default settings
func (l *Loader) applyDeploymentDefaults(config *types.Config) {
	// Set stream and consumer names based on deployment type
	if config.NATS.Stream.Name == "SMTS" {
		config.NATS.Stream.Name = fmt.Sprintf("SMTS_%s", strings.ToUpper(config.Deployment.Type))
	}
	if config.NATS.Consumer.DurableName == "SMTS_CONSUMER" {
		config.NATS.Consumer.DurableName = fmt.Sprintf("SMTS_%s_CONSUMER", strings.ToUpper(config.Deployment.Type))
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
		zap.String("stream", config.NATS.Stream.Name),
		zap.String("consumer", config.NATS.Consumer.DurableName),
		zap.Bool("dlp_enabled", config.DLP.Enabled),
		zap.Bool("artemis_enabled", config.Artemis.Enabled))
}

// LoadTopicsConfig loads topics configuration from a separate file
func (l *Loader) LoadTopicsConfig(topicsPath string) (*types.TopicsConfig, error) {
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
			l.logger.Warn("No topics configuration file found, using empty configuration")
			return &types.TopicsConfig{
				Topics: make(map[string]types.TopicPermission),
				Roles:  make(map[string]types.RoleDefinition),
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