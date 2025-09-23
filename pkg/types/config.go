package types

import (
	"time"
)

// Config represents the main configuration structure for SMTS
type Config struct {
	Deployment DeploymentConfig `mapstructure:"deployment" yaml:"deployment"`
	NATS       NATSConfig       `mapstructure:"nats" yaml:"nats"`
	API        APIConfig        `mapstructure:"api" yaml:"api"`
	DLP        DLPConfig        `mapstructure:"dlp" yaml:"dlp"`
	Artemis    ArtemisConfig    `mapstructure:"artemis" yaml:"artemis"`
	Logging    LoggingConfig    `mapstructure:"logging" yaml:"logging"`
	Health     HealthConfig     `mapstructure:"health" yaml:"health"`
	Topics     TopicsConfig     `mapstructure:"topics" yaml:"topics"`
}

// DeploymentConfig contains deployment-specific settings
type DeploymentConfig struct {
	Type        string `mapstructure:"type" yaml:"type"` // "ext" or "int"
	Name        string `mapstructure:"name" yaml:"name"`
	Environment string `mapstructure:"environment" yaml:"environment"`
}

// NATSConfig contains NATS JetStream configuration
type NATSConfig struct {
	Embedded bool          `mapstructure:"embedded" yaml:"embedded"`
	Host     string        `mapstructure:"host" yaml:"host"`
	Port     int           `mapstructure:"port" yaml:"port"`
	Stream   StreamConfig  `mapstructure:"stream" yaml:"stream"`
	Consumer ConsumerConfig `mapstructure:"consumer" yaml:"consumer"`
}

// StreamConfig contains NATS stream configuration
type StreamConfig struct {
	Name      string   `mapstructure:"name" yaml:"name"`
	Subjects  []string `mapstructure:"subjects" yaml:"subjects"`
	Retention string   `mapstructure:"retention" yaml:"retention"`
	MaxAge    string   `mapstructure:"max_age" yaml:"max_age"`
	Storage   string   `mapstructure:"storage" yaml:"storage"`
	Replicas  int      `mapstructure:"replicas" yaml:"replicas"`
}

// ConsumerConfig contains NATS consumer configuration
type ConsumerConfig struct {
	DurableName  string `mapstructure:"durable_name" yaml:"durable_name"`
	AckPolicy    string `mapstructure:"ack_policy" yaml:"ack_policy"`
	DeliverPolicy string `mapstructure:"deliver_policy" yaml:"deliver_policy"`
}

// APIConfig contains corporate API configuration
type APIConfig struct {
	BaseURL string        `mapstructure:"base_url" yaml:"base_url"`
	Timeout time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Retry   RetryConfig   `mapstructure:"retry" yaml:"retry"`
	Auth    AuthConfig    `mapstructure:"auth" yaml:"auth"`
}

// DLPConfig contains DLP validation configuration
type DLPConfig struct {
	Enabled  bool         `mapstructure:"enabled" yaml:"enabled"`
	Endpoint string       `mapstructure:"endpoint" yaml:"endpoint"`
	Timeout  time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Retry    RetryConfig  `mapstructure:"retry" yaml:"retry"`
}

// ArtemisConfig contains ArtemisMQ configuration
type ArtemisConfig struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled"`
	Host     string `mapstructure:"host" yaml:"host"`
	Port     int    `mapstructure:"port" yaml:"port"`
	Queue    string `mapstructure:"queue" yaml:"queue"`
	Username string `mapstructure:"username" yaml:"username"`
	Password string `mapstructure:"password" yaml:"password"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	Type   string `mapstructure:"type" yaml:"type"` // "api_key"
	APIKey string `mapstructure:"api_key" yaml:"api_key"`
}

// RetryConfig contains retry settings for external calls
type RetryConfig struct {
	MaxAttempts int           `mapstructure:"max_attempts" yaml:"max_attempts"`
	Backoff     time.Duration `mapstructure:"backoff" yaml:"backoff"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level" yaml:"level"`
	Format string `mapstructure:"format" yaml:"format"`
	Output string `mapstructure:"output" yaml:"output"`
}

// HealthConfig contains health check settings
type HealthConfig struct {
	Port     int           `mapstructure:"port" yaml:"port"`
	Path     string        `mapstructure:"path" yaml:"path"`
	Interval time.Duration `mapstructure:"interval" yaml:"interval"`
}

// TopicsConfig contains topic and privilege configuration
type TopicsConfig struct {
	Topics map[string]TopicPermission `mapstructure:"topics" yaml:"topics"`
	Roles  map[string]RoleDefinition  `mapstructure:"roles" yaml:"roles"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Deployment: DeploymentConfig{
			Type:        "ext",
			Name:        "smts",
			Environment: "production",
		},
		NATS: NATSConfig{
			Embedded: true,
			Host:     "localhost",
			Port:     4222,
			Stream: StreamConfig{
				Name:      "SMTS",
				Subjects:  []string{"monterra.>", "pact_update.>"},
				Retention: "workqueue",
				MaxAge:    "24h",
				Storage:   "file",
				Replicas:  1,
			},
			Consumer: ConsumerConfig{
				DurableName:  "SMTS_CONSUMER",
				AckPolicy:    "explicit",
				DeliverPolicy: "all",
			},
		},
		API: APIConfig{
			BaseURL: "https://api.corporate.com",
			Timeout: 30 * time.Second,
			Retry: RetryConfig{
				MaxAttempts: 3,
				Backoff:     2 * time.Second,
			},
			Auth: AuthConfig{
				Type: "api_key",
			},
		},
		DLP: DLPConfig{
			Enabled:  false,
			Endpoint: "https://dlp.corporate.com/validate",
			Timeout:  10 * time.Second,
			Retry: RetryConfig{
				MaxAttempts: 2,
				Backoff:     1 * time.Second,
			},
		},
		Artemis: ArtemisConfig{
			Enabled: false,
			Host:    "localhost",
			Port:    61616,
			Queue:   "SMTS_QUEUE",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Health: HealthConfig{
			Port:     8080,
			Path:     "/health",
			Interval: 30 * time.Second,
		},
		Topics: TopicsConfig{
			Topics: make(map[string]TopicPermission),
			Roles:  make(map[string]RoleDefinition),
		},
	}
}