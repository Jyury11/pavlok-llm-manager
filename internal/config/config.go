// Package config provides configuration management for the application.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// DBType represents the database type.
type DBType string

const (
	DBTypeSQLite    DBType = "sqlite"
	DBTypeFirestore DBType = "firestore"
)

// Config holds all application configuration.
type Config struct {
	// LINE
	LineChannelSecret      string
	LineChannelAccessToken string
	AllowedLineUserID      string

	// Gemini
	GeminiAPIKey string

	// Pavlok
	PavlokAccessToken string

	// Server
	Port string

	// Database
	DBType       DBType
	SQLitePath   string
	GCPProjectID string

	// Safety
	MaxDailyShocks  int
	MaxHourlyShocks int
	ShockLevel      int
	QuietHoursStart int
	QuietHoursEnd   int

	// Review
	ReviewHour   int  // 振り返り通知の時間（24時間形式）
	ReviewMinute int  // 振り返り通知の分
	ReviewEnable bool // 振り返り機能の有効/無効
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		LineChannelSecret:      getEnv("LINE_CHANNEL_SECRET", ""),
		LineChannelAccessToken: getEnv("LINE_CHANNEL_ACCESS_TOKEN", ""),
		AllowedLineUserID:      getEnv("ALLOWED_LINE_USER_ID", ""),
		GeminiAPIKey:           getEnv("GEMINI_API_KEY", ""),
		PavlokAccessToken:      getEnv("PAVLOK_ACCESS_TOKEN", ""),
		Port:                   getEnv("PORT", "8080"),
		DBType:                 DBType(getEnv("DB_TYPE", "sqlite")),
		SQLitePath:             getEnv("SQLITE_PATH", "./data.db"),
		GCPProjectID:           getEnv("GCP_PROJECT_ID", ""),
		MaxDailyShocks:         getEnvInt("MAX_DAILY_SHOCKS", 10),
		MaxHourlyShocks:        getEnvInt("MAX_HOURLY_SHOCKS", 3),
		ShockLevel:             50, // 固定値
		QuietHoursStart:        getEnvInt("QUIET_HOURS_START", 23),
		QuietHoursEnd:          getEnvInt("QUIET_HOURS_END", 6),
		ReviewHour:             getEnvInt("REVIEW_HOUR", 21),
		ReviewMinute:           getEnvInt("REVIEW_MINUTE", 0),
		ReviewEnable:           getEnvBool("REVIEW_ENABLE", true),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.LineChannelSecret == "" {
		return fmt.Errorf("LINE_CHANNEL_SECRET is required")
	}
	if c.LineChannelAccessToken == "" {
		return fmt.Errorf("LINE_CHANNEL_ACCESS_TOKEN is required")
	}
	if c.AllowedLineUserID == "" {
		return fmt.Errorf("ALLOWED_LINE_USER_ID is required")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("GEMINI_API_KEY is required")
	}
	if c.PavlokAccessToken == "" {
		return fmt.Errorf("PAVLOK_ACCESS_TOKEN is required")
	}
	if c.DBType != DBTypeSQLite && c.DBType != DBTypeFirestore {
		return fmt.Errorf("DB_TYPE must be 'sqlite' or 'firestore'")
	}
	if c.DBType == DBTypeFirestore && c.GCPProjectID == "" {
		return fmt.Errorf("GCP_PROJECT_ID is required when DB_TYPE is 'firestore'")
	}
	return nil
}

// IsFirestore returns true if using Firestore.
func (c *Config) IsFirestore() bool {
	return c.DBType == DBTypeFirestore
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
