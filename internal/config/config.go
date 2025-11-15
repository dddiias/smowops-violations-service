package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type HTTPConfig struct {
	Host string
	Port int
}

type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type AuthConfig struct {
	AccessSecret string
}

type FilesConfig struct {
	MaxAttachmentsPerAction int
}

type Config struct {
	Environment string
	HTTP        HTTPConfig
	DB          DBConfig
	Auth        AuthConfig
	Files       FilesConfig
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("app")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("./deploy")
	v.AddConfigPath("./internal/config")

	v.AutomaticEnv()

	_ = v.ReadInConfig()

	cfg := &Config{
		Environment: v.GetString("APP_ENV"),
		HTTP: HTTPConfig{
			Host: v.GetString("HTTP_HOST"),
			Port: v.GetInt("HTTP_PORT"),
		},
		DB: DBConfig{
			DSN:             v.GetString("DB_DSN"),
			MaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: v.GetDuration("DB_CONN_MAX_LIFETIME"),
		},
		Auth: AuthConfig{
			AccessSecret: v.GetString("JWT_ACCESS_SECRET"),
		},
		Files: FilesConfig{
			MaxAttachmentsPerAction: v.GetInt("APPEAL_MAX_ATTACHMENTS"),
		},
	}

	if cfg.HTTP.Host == "" {
		cfg.HTTP.Host = "0.0.0.0"
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 7086
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.Files.MaxAttachmentsPerAction <= 0 {
		cfg.Files.MaxAttachmentsPerAction = 5
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {
	if cfg.DB.DSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	if cfg.Auth.AccessSecret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET is required")
	}
	return nil
}
