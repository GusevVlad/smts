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
	LDAP       LDAPConfig       `mapstructure:"ldap" yaml:"ldap"`
	Vault      VaultConfig      `mapstructure:"vault" yaml:"vault"`
}

// DeploymentConfig contains deployment-specific settings
type DeploymentConfig struct {
	Type        string `mapstructure:"type" yaml:"type"` // "ext" or "int"
	Name        string `mapstructure:"name" yaml:"name"`
	Environment string `mapstructure:"environment" yaml:"environment"`
}

// NATSConfig contains NATS JetStream configuration
type NATSConfig struct {
	Embedded bool   `mapstructure:"embedded" yaml:"embedded"`
	Host     string `mapstructure:"host" yaml:"host"`
	Port     int    `mapstructure:"port" yaml:"port"`
	// Client stream for messages from local clients
	ClientStream   StreamConfig   `mapstructure:"client_stream" yaml:"client_stream"`
	ClientConsumer ConsumerConfig `mapstructure:"client_consumer" yaml:"client_consumer"`
	// External stream for messages from different network instances
	ExternalStream   StreamConfig   `mapstructure:"external_stream" yaml:"external_stream"`
	ExternalConsumer ConsumerConfig `mapstructure:"external_consumer" yaml:"external_consumer"`
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
	DurableName   string `mapstructure:"durable_name" yaml:"durable_name"`
	AckPolicy     string `mapstructure:"ack_policy" yaml:"ack_policy"`
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
	Enabled  bool          `mapstructure:"enabled" yaml:"enabled"`
	Endpoint string        `mapstructure:"endpoint" yaml:"endpoint"`
	Timeout  time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Retry    RetryConfig   `mapstructure:"retry" yaml:"retry"`
	// Provider specifies the DLP provider (legacy, traffic_monitor)
	Provider string `mapstructure:"provider" yaml:"provider"`
	// TrafficMonitor specific configuration
	TrafficMonitor TrafficMonitorConfig `mapstructure:"traffic_monitor" yaml:"traffic_monitor"`
}

// TrafficMonitorConfig contains Traffic Monitor specific configuration
type TrafficMonitorConfig struct {
	// BaseURL is the base URL of Traffic Monitor (e.g., https://server.company.ru:9106)
	BaseURL string `mapstructure:"base_url" yaml:"base_url"`
	// AuthToken is the X-API-Auth-Token header value
	AuthToken string `mapstructure:"auth_token" yaml:"auth_token"`
	// CompanyId is the X-API-CompanyId header value
	CompanyId string `mapstructure:"company_id" yaml:"company_id"`
	// Version is the X-API-Version header value (default: "1.8")
	Version string `mapstructure:"version" yaml:"version"`
	// CaptureServerIP is the capture_server_ip attribute value
	CaptureServerIP string `mapstructure:"capture_server_ip" yaml:"capture_server_ip"`
	// CaptureServerFQDN is the capture_server_fqdn attribute value
	CaptureServerFQDN string `mapstructure:"capture_server_fqdn" yaml:"capture_server_fqdn"`
	// VerdictPollingEnabled enables polling for verdict after event push
	VerdictPollingEnabled bool `mapstructure:"verdict_polling_enabled" yaml:"verdict_polling_enabled"`
	// VerdictPollingInterval is the interval between polling attempts
	VerdictPollingInterval time.Duration `mapstructure:"verdict_polling_interval" yaml:"verdict_polling_interval"`
	// VerdictPollingTimeout is the total timeout for verdict polling
	VerdictPollingTimeout time.Duration `mapstructure:"verdict_polling_timeout" yaml:"verdict_polling_timeout"`
}

// ArtemisConfig contains ArtemisMQ configuration
type ArtemisConfig struct {
	Enabled      bool   `mapstructure:"enabled" yaml:"enabled"`
	Host         string `mapstructure:"host" yaml:"host"`
	Port         int    `mapstructure:"port" yaml:"port"`
	Queue        string `mapstructure:"queue" yaml:"queue"`
	PublishQueue string `mapstructure:"publish_queue" yaml:"publish_queue"`
	Username     string `mapstructure:"username" yaml:"username"`
	Password     string `mapstructure:"password" yaml:"password"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	Type         string `mapstructure:"type" yaml:"type"` // "api_key", "client_credentials"
	APIKey       string `mapstructure:"api_key" yaml:"api_key"`
	ClientID     string `mapstructure:"client_id" yaml:"client_id"`
	ClientSecret string `mapstructure:"client_secret" yaml:"client_secret"`
	TokenURL     string `mapstructure:"token_url" yaml:"token_url"`
	Scopes       string `mapstructure:"scopes" yaml:"scopes"`
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
}

// LDAPConfig contains LDAP authentication and authorization configuration
type LDAPConfig struct {
	Enabled            bool     `mapstructure:"enabled" yaml:"enabled"`
	Host               string   `mapstructure:"host" yaml:"host"`
	Port               int      `mapstructure:"port" yaml:"port"`
	Base               string   `mapstructure:"base" yaml:"base"`
	BindDN             string   `mapstructure:"bind_dn" yaml:"bind_dn"`
	BindPassword       string   `mapstructure:"bind_password" yaml:"bind_password"`
	UserFilter         string   `mapstructure:"user_filter" yaml:"user_filter"`
	GroupFilter        string   `mapstructure:"group_filter" yaml:"group_filter"`
	Attributes         []string `mapstructure:"attributes" yaml:"attributes"`
	ServerName         string   `mapstructure:"server_name" yaml:"server_name"`
	UseSSL             bool     `mapstructure:"use_ssl" yaml:"use_ssl"`
	SkipTLS            bool     `mapstructure:"skip_tls" yaml:"skip_tls"`
	InsecureSkipVerify bool     `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	UseHTTP            bool     `mapstructure:"use_http" yaml:"use_http"`
}

// VaultConfig contains HashiCorp Vault configuration
type VaultConfig struct {
	Enabled          bool          `mapstructure:"enabled" yaml:"enabled"`
	Address          string        `mapstructure:"address" yaml:"address"`
	RoleID           string        `mapstructure:"role_id" yaml:"role_id"`
	SecretID         string        `mapstructure:"secret_id" yaml:"secret_id"`
	PathPrefix       string        `mapstructure:"path_prefix" yaml:"path_prefix"`
	AutoRenew        bool          `mapstructure:"auto_renew" yaml:"auto_renew"`
	RenewalThreshold time.Duration `mapstructure:"renewal_threshold" yaml:"renewal_threshold"`
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
			ClientStream: StreamConfig{
				Name:      "SMTS_CLIENT",
				Subjects:  []string{"client.monterra.>", "client.pact_update.>"},
				Retention: "workqueue",
				MaxAge:    "24h",
				Storage:   "file",
				Replicas:  1,
			},
			ClientConsumer: ConsumerConfig{
				DurableName:   "SMTS_CLIENT_CONSUMER",
				AckPolicy:     "explicit",
				DeliverPolicy: "all",
			},
			ExternalStream: StreamConfig{
				Name:      "SMTS_EXTERNAL",
				Subjects:  []string{"external.monterra.>", "external.pact_update.>"},
				Retention: "workqueue",
				MaxAge:    "24h",
				Storage:   "file",
				Replicas:  1,
			},
			ExternalConsumer: ConsumerConfig{
				DurableName:   "SMTS_EXTERNAL_CONSUMER",
				AckPolicy:     "explicit",
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
			Provider: "legacy",
			TrafficMonitor: TrafficMonitorConfig{
				BaseURL:                "",
				AuthToken:              "",
				CompanyId:              "",
				Version:                "1.8",
				CaptureServerIP:        "",
				CaptureServerFQDN:      "",
				VerdictPollingEnabled:  false,
				VerdictPollingInterval: 5 * time.Second,
				VerdictPollingTimeout:  30 * time.Second,
			},
		},
		Artemis: ArtemisConfig{
			Enabled: false,
			Host:    "artemis",
			Port:    61613,
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
		},
		LDAP: LDAPConfig{
			Enabled:            false,
			Host:               "localhost",
			Port:               389,
			Base:               "dc=example,dc=com",
			BindDN:             "",
			BindPassword:       "",
			UserFilter:         "(uid=%s)",
			GroupFilter:        "(member=%s)",
			Attributes:         []string{"uid", "cn", "mail", "distinguishedName"},
			ServerName:         "",
			UseSSL:             false,
			SkipTLS:            false,
			InsecureSkipVerify: false,
			UseHTTP:            false,
		},
		Vault: VaultConfig{
			Enabled:          false,
			Address:          "http://localhost:8200",
			RoleID:           "",
			SecretID:         "",
			PathPrefix:       "smts",
			AutoRenew:        true,
			RenewalThreshold: 5 * time.Minute,
		},
	}
}
