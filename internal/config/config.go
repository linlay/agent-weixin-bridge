package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	BridgeHTTPAddr    string
	WeixinBaseURL     string
	WeixinBotType     string
	RunnerBaseURL     string
	RunnerAgentKey    string
	RunnerBearerToken string
	StateDir          string
	LogLevel          string
	AutoStartPoll     bool
	ShutdownTimeout   time.Duration
}

func Load() (Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("getwd: %w", err)
	}
	_ = godotenv.Load(filepath.Join(wd, ".env"))
	cfg := Config{
		BridgeHTTPAddr:    envOrDefault("BRIDGE_HTTP_ADDR", ":11958"),
		WeixinBaseURL:     envOrDefault("WEIXIN_BASE_URL", "https://ilinkai.weixin.qq.com"),
		WeixinBotType:     envOrDefault("WEIXIN_BOT_TYPE", "3"),
		RunnerBaseURL:     envOrDefault("RUNNER_BASE_URL", "http://127.0.0.1:8080"),
		RunnerAgentKey:    strings.TrimSpace(os.Getenv("RUNNER_AGENT_KEY")),
		RunnerBearerToken: strings.TrimSpace(os.Getenv("RUNNER_BEARER_TOKEN")),
		StateDir:          envOrDefault("STATE_DIR", filepath.Join(wd, "var", "state")),
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
		AutoStartPoll:     parseBool("AUTO_START_POLL", true),
		ShutdownTimeout:   10 * time.Second,
	}
	if strings.TrimSpace(cfg.RunnerBaseURL) == "" {
		return Config{}, fmt.Errorf("RUNNER_BASE_URL is required")
	}
	if strings.TrimSpace(cfg.RunnerAgentKey) == "" {
		return Config{}, fmt.Errorf("RUNNER_AGENT_KEY is required")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func parseBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}
