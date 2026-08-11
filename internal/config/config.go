package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port     int
	Database Database
	Relay    Relay
	TLS      TLS
	LogLevel slog.Level
}

type Database struct {
	URL          string
	Username     string
	Password     string
	MaxOpenConns int
	MaxIdleConns int
}

type Relay struct {
	Domain                 string
	Region                 string
	DefaultExpirationHours int
	CleanupRetentionDays   int
	CleanupInitialDelay    time.Duration
	CleanupInterval        time.Duration
	RateLimitEnabled       bool
	RequestsPerMinute      int
	JWTIssuer              string
	JWTAudience            string
	JWTKeyID               string
	JWTPrivateKey          string
	JWTTokenTTL            time.Duration
	DefaultPlanCode        string
	BillingEnforcement     bool
	BillingSettlement      bool
	SettlementInterval     time.Duration
	SettlementBatchSize    int
	PartitionInitialDelay  time.Duration
	PartitionInterval      time.Duration
}

type TLS struct {
	Enabled            bool
	KeyStoreBase64     string
	KeyStorePassword   string
	TrustStoreBase64   string
	TrustStorePassword string
}

func Load() (Config, error) {
	tlsEnabled, err := envBool("SERVER_TLS_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	defaultPort := 8080
	if tlsEnabled {
		defaultPort = 8443
	}

	port, err := envInt("SERVER_PORT", defaultPort)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("SERVER_PORT must be between 1 and 65535")
	}
	rateLimitEnabled, err := envBool("RELAY_RATE_LIMIT_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	billingEnforcement, err := envBool("RELAY_BILLING_ENFORCEMENT_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	billingSettlement, err := envBool("RELAY_BILLING_SETTLEMENT_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port: port,
		Database: Database{
			URL:          os.Getenv("DATASOURCE_URL"),
			Username:     os.Getenv("DATASOURCE_USERNAME"),
			Password:     os.Getenv("DATASOURCE_PASSWORD"),
			MaxOpenConns: envIntDefault("DATASOURCE_MAX_OPEN_CONNS", 20),
			MaxIdleConns: envIntDefault("DATASOURCE_MAX_IDLE_CONNS", 10),
		},
		Relay: Relay{
			Domain:                 os.Getenv("RELAY_DOMAIN"),
			Region:                 os.Getenv("RELAY_REGION"),
			DefaultExpirationHours: envIntDefault("RELAY_DEFAULT_EXPIRATION_HOURS", 72),
			CleanupRetentionDays:   envIntDefault("RELAY_TUNNEL_CLEANUP_RETENTION_DAYS", 3),
			CleanupInitialDelay:    envDuration("RELAY_TUNNEL_CLEANUP_INITIAL_DELAY", time.Hour),
			CleanupInterval:        envDuration("RELAY_TUNNEL_CLEANUP_INTERVAL", time.Hour),
			RateLimitEnabled:       rateLimitEnabled,
			RequestsPerMinute:      envIntDefault("RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE", 120),
			JWTIssuer:              envDefault("RELAY_JWT_ISSUER", "devbridge"),
			JWTAudience:            envDefault("RELAY_JWT_AUDIENCE", "relay-gateway"),
			JWTKeyID:               envDefault("RELAY_JWT_KEY_ID", "1"),
			JWTPrivateKey:          os.Getenv("RELAY_JWT_PRIVATE_KEY"),
			JWTTokenTTL:            envDuration("RELAY_JWT_TOKEN_TTL", 24*time.Hour),
			DefaultPlanCode:        envDefault("RELAY_BILLING_DEFAULT_PLAN_CODE", "trial"),
			BillingEnforcement:     billingEnforcement,
			BillingSettlement:      billingSettlement,
			SettlementInterval:     envDuration("RELAY_BILLING_SETTLEMENT_INTERVAL", time.Minute),
			SettlementBatchSize:    envIntDefault("RELAY_BILLING_SETTLEMENT_BATCH_SIZE", 500),
			PartitionInitialDelay:  envDuration("RELAY_PARTITION_INITIAL_DELAY", time.Minute),
			PartitionInterval:      envDuration("RELAY_PARTITION_INTERVAL", time.Hour),
		},
		TLS: TLS{
			Enabled:            tlsEnabled,
			KeyStoreBase64:     os.Getenv("SERVER_SSL_KEY_STORE_BASE64"),
			KeyStorePassword:   os.Getenv("SERVER_SSL_KEY_STORE_PASSWORD"),
			TrustStoreBase64:   os.Getenv("SERVER_SSL_TRUST_STORE_BASE64"),
			TrustStorePassword: os.Getenv("SERVER_SSL_TRUST_STORE_PASSWORD"),
		},
		LogLevel: parseLogLevel(os.Getenv("LOG_LEVEL")),
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	required := [][2]string{
		{"DATASOURCE_URL", c.Database.URL},
		{"DATASOURCE_USERNAME", c.Database.Username},
		{"DATASOURCE_PASSWORD", c.Database.Password},
		{"RELAY_DOMAIN", c.Relay.Domain},
		{"RELAY_REGION", c.Relay.Region},
		{"RELAY_JWT_PRIVATE_KEY", c.Relay.JWTPrivateKey},
	}
	for _, item := range required {
		name, value := item[0], item[1]
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if looksEncrypted(value) {
			return fmt.Errorf("%s must be decrypted before it is injected", name)
		}
	}
	if c.Database.MaxOpenConns < 1 || c.Database.MaxIdleConns < 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("database connection pool settings are invalid")
	}
	if c.Relay.DefaultExpirationHours < 1 || c.Relay.DefaultExpirationHours > 720 {
		return fmt.Errorf("RELAY_DEFAULT_EXPIRATION_HOURS must be between 1 and 720")
	}
	if c.Relay.CleanupRetentionDays < 0 || c.Relay.CleanupInitialDelay < 0 || c.Relay.CleanupInterval <= 0 {
		return fmt.Errorf("tunnel cleanup settings are invalid")
	}
	if c.Relay.RequestsPerMinute < 0 || c.Relay.JWTTokenTTL <= 0 || c.Relay.SettlementInterval <= 0 || c.Relay.SettlementBatchSize < 1 {
		return fmt.Errorf("relay runtime settings are invalid")
	}
	if c.Relay.PartitionInitialDelay < 0 || c.Relay.PartitionInterval <= 0 {
		return fmt.Errorf("partition maintenance settings are invalid")
	}
	if c.TLS.Enabled {
		tlsValues := [][2]string{
			{"SERVER_SSL_KEY_STORE_BASE64", c.TLS.KeyStoreBase64},
			{"SERVER_SSL_KEY_STORE_PASSWORD", c.TLS.KeyStorePassword},
			{"SERVER_SSL_TRUST_STORE_BASE64", c.TLS.TrustStoreBase64},
			{"SERVER_SSL_TRUST_STORE_PASSWORD", c.TLS.TrustStorePassword},
		}
		for _, item := range tlsValues {
			name, value := item[0], item[1]
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s is required when TLS is enabled", name)
			}
			if looksEncrypted(value) {
				return fmt.Errorf("%s must be decrypted before it is injected", name)
			}
		}
	}
	return nil
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func envIntDefault(name string, fallback int) int {
	value, err := envInt(name, fallback)
	if err != nil {
		return -1
	}
	return value
}

func envBool(name string, fallback bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return -1
	}
	return parsed
}

func looksEncrypted(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "ENC(") || strings.HasPrefix(trimmed, "${ENC(")
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
