// Package config reads the process configuration from the environment and
// fails fast, naming every missing or invalid variable at once.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Config is the validated process configuration.
type Config struct {
	Port        string
	LogLevel    slog.Level
	DatabaseURL string
	// DataDir holds uploaded images and export files (the VM's data disk, D9).
	DataDir string
	// ChannelsDir holds the channel rule files (REQ-017).
	ChannelsDir string
	// OpenRouterKey selects the real AI provider; empty means the local
	// stand-in, which costs nothing (development and demos without a key).
	OpenRouterKey string
	// AIModel is the model id sent to OpenRouter (PRD Constraints).
	AIModel string
	// SecureCookies marks the session cookie Secure; off only for plain-HTTP local runs.
	SecureCookies bool
}

// Load reads and validates the environment.
func Load() (Config, error) {
	var problems []string
	c := Config{
		Port:          envOr("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		DataDir:       envOr("DATA_DIR", "data"),
		ChannelsDir:   envOr("CHANNELS_DIR", "config/channels"),
		OpenRouterKey: os.Getenv("OPENROUTER_API_KEY"),
		AIModel:       envOr("AI_MODEL", "anthropic/claude-haiku-4.5"),
		SecureCookies: envOr("SECURE_COOKIES", "true") != "false",
	}
	switch strings.ToLower(envOr("LOG_LEVEL", "info")) {
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "info":
		c.LogLevel = slog.LevelInfo
	case "warn":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	default:
		problems = append(problems, "LOG_LEVEL must be debug, info, warn or error")
	}
	if len(problems) > 0 {
		return Config{}, fmt.Errorf("config: %s", strings.Join(problems, "; "))
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
