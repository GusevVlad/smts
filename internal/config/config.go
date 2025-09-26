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
		manuallyParsedRoles := make(map[string]types.RoleDefinition)
		
		// Parse topics
		for topicName, topicData := range topicsMap {
			if topicMap, ok := topicData.(map[string]interface{}); ok {
				topicPermission := types.TopicPermission{}
				
				// Extract read_roles
				if readRoles, ok := topicMap["read_roles"].([]interface{}); ok {
					for _, role := range readRoles {
						if roleStr, ok := role.(string); ok {
							topicPermission.ReadRoles = append(topicPermission.ReadRoles, roleStr)
						}
					}
				}
				
				// Extract write_roles
				if writeRoles, ok := topicMap["write_roles"].([]interface{}); ok {
					for _, role := range writeRoles {
						if roleStr, ok := role.(string); ok {
							topicPermission.WriteRoles = append(topicPermission.WriteRoles, roleStr)
						}
					}
				}
				
				// Extract description
				if desc, ok := topicMap["description"].(string); ok {
					topicPermission.Description = desc
				}
				
				manuallyParsedTopics[topicName] = topicPermission
			}
		}
		
		// Parse roles if they exist at the same level as topics
		if v.IsSet("roles") {
			rolesMap := v.GetStringMap("roles")
			for roleName, roleData := range rolesMap {
				if roleMap, ok := roleData.(map[string]interface{}); ok {
					roleDefinition := types.RoleDefinition{}
					
					// Extract description
					if desc, ok := roleMap["description"].(string); ok {
						roleDefinition.Description = desc
					}
					
					// Extract topics
					if topics, ok := roleMap["topics"].([]interface{}); ok {
						for _, topic := range topics {
							if topicStr, ok := topic.(string); ok {
								roleDefinition.Topics = append(roleDefinition.Topics, topicStr)
							}
						}
					}
					
					manuallyParsedRoles[roleName] = roleDefinition
				}
			}
		}
		
		l.logger.Debug("Manually parsed topics",
			zap.Any("topics", manuallyParsedTopics),
			zap.Any("roles", manuallyParsedRoles),
			zap.Int("topics_count", len(manuallyParsedTopics)),
			zap.Int("roles_count", len(manuallyParsedRoles)))
		
		// If manual parsing worked, use the manually parsed topics
		if len(manuallyParsedTopics) > 0 {
			config.Topics.Topics = manuallyParsedTopics
			config.Topics.Roles = manuallyParsedRoles
			l.logger.Debug("Using manually parsed topics configuration")
		}
	}

	// Debug: Log topics configuration after unmarshaling
	l.logger.Debug("Configuration unmarshaled",
		zap.Int("topics_count", len(config.Topics.Topics)),
		zap.Int("roles_count", len(config.Topics.Roles)),
		zap.String("deployment", config.Deployment.Type))
	
	// Debug: Log all topics found
	for topicName, topic := range config.Topics.Topics {
		l.logger.Debug("Found topic in config",
			zap.String("topic", topicName),
			zap.Strings("read_roles", topic.ReadRoles),
			zap.Strings("write_roles", topic.WriteRoles),
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