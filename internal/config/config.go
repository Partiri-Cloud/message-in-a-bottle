package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type RateLimitChannelConfig struct {
	MaxPerWindow  int `json:"maxPerWindow"`
	WindowMinutes int `json:"windowMinutes"`
}

type Config struct {
	MongoURI      string
	MongoDB       string
	RedisAddr     string
	RedisPassword string

	APIPort string
	WSPort  string

	// CredentialsEncryptionKey holds the raw hex string (for reference/logging purposes only).
	// Use CredentialsEncryptionKeyBytes for all cryptographic operations.
	CredentialsEncryptionKey      string
	CredentialsEncryptionKeyBytes []byte
	SubscriberHMACSecret          string

	WSAllowedOrigins    []string
	MaxRequestBodyBytes int64

	NotificationRetentionDays int
	ActivityLogRetentionDays  int

	RateLimitConfig map[string]RateLimitChannelConfig

	AdminSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		MongoURI:                  getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:                   getEnv("MONGO_DB", "message_in_a_bottle"),
		RedisAddr:                 getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:             getEnv("REDIS_PASSWORD", ""),
		APIPort:                   getEnv("API_PORT", "3000"),
		WSPort:                    getEnv("WS_PORT", "3001"),
		CredentialsEncryptionKey:  getEnv("CREDENTIALS_ENCRYPTION_KEY", ""),
		SubscriberHMACSecret:      getEnv("SUBSCRIBER_HMAC_SECRET", ""),
		WSAllowedOrigins:          parseOrigins(getEnv("WS_ALLOWED_ORIGINS", "")),
		MaxRequestBodyBytes:       int64(getEnvInt("MAX_REQUEST_BODY_BYTES", 2*1024*1024)), // 2MB default
		NotificationRetentionDays: getEnvInt("NOTIFICATION_RETENTION_DAYS", 90),
		ActivityLogRetentionDays:  getEnvInt("ACTIVITY_LOG_RETENTION_DAYS", 30),
	}

	cfg.AdminSecret = getEnv("ADMIN_SECRET", "")
	if cfg.AdminSecret == "" {
		return nil, fmt.Errorf("ADMIN_SECRET is required")
	}
	if len(cfg.AdminSecret) < 32 {
		return nil, fmt.Errorf("ADMIN_SECRET must be at least 32 characters")
	}

	if cfg.SubscriberHMACSecret == "" {
		return nil, fmt.Errorf("SUBSCRIBER_HMAC_SECRET is required and must be at least 32 characters")
	}
	if len(cfg.SubscriberHMACSecret) < 32 {
		return nil, fmt.Errorf("SUBSCRIBER_HMAC_SECRET is required and must be at least 32 characters")
	}

	if cfg.CredentialsEncryptionKey == "" {
		return nil, fmt.Errorf("CREDENTIALS_ENCRYPTION_KEY must be 64 hex characters (32 bytes)")
	}
	keyBytes, err := hex.DecodeString(cfg.CredentialsEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("CREDENTIALS_ENCRYPTION_KEY must be 64 hex characters (32 bytes)")
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("CREDENTIALS_ENCRYPTION_KEY must be 64 hex characters (32 bytes)")
	}
	cfg.CredentialsEncryptionKeyBytes = keyBytes

	cfg.RateLimitConfig = defaultRateLimits()
	if raw := os.Getenv("RATE_LIMIT_CONFIG"); raw != "" {
		var rl map[string]RateLimitChannelConfig
		if err := json.Unmarshal([]byte(raw), &rl); err != nil {
			fmt.Printf("RATE_LIMIT_CONFIG parse error: %v — using defaults\n", err)
		} else {
			cfg.RateLimitConfig = rl
		}
	}

	return cfg, nil
}

func defaultRateLimits() map[string]RateLimitChannelConfig {
	return map[string]RateLimitChannelConfig{
		"email":    {MaxPerWindow: 50, WindowMinutes: 60},
		"sms":      {MaxPerWindow: 10, WindowMinutes: 60},
		"push":     {MaxPerWindow: 100, WindowMinutes: 60},
		"in_app":   {MaxPerWindow: 200, WindowMinutes: 60},
		"slack":    {MaxPerWindow: 30, WindowMinutes: 60},
		"ms_teams": {MaxPerWindow: 30, WindowMinutes: 60},
	}
}

func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
