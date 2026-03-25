package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsRunnerAgentKeyFromDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	writeEnvFile(t, tempDir, "RUNNER_AGENT_KEY=dotenv-agent\nRUNNER_BASE_URL=http://127.0.0.1:11949\nAUTO_START_POLL=true\n")
	unsetEnv(t, "RUNNER_AGENT_KEY", "RUNNER_BASE_URL", "AUTO_START_POLL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RunnerAgentKey != "dotenv-agent" {
		t.Fatalf("RunnerAgentKey = %q, want %q", cfg.RunnerAgentKey, "dotenv-agent")
	}
	if cfg.RunnerBaseURL != "http://127.0.0.1:11949" {
		t.Fatalf("RunnerBaseURL = %q, want %q", cfg.RunnerBaseURL, "http://127.0.0.1:11949")
	}
	if !cfg.AutoStartPoll {
		t.Fatalf("AutoStartPoll = false, want true")
	}
}

func TestLoadPrefersShellEnvironmentOverDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	writeEnvFile(t, tempDir, "RUNNER_AGENT_KEY=dotenv-agent\nRUNNER_BASE_URL=http://127.0.0.1:11949\nAUTO_START_POLL=false\n")
	unsetEnv(t, "RUNNER_AGENT_KEY", "RUNNER_BASE_URL", "AUTO_START_POLL")
	t.Setenv("RUNNER_AGENT_KEY", "shell-agent")
	t.Setenv("RUNNER_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("AUTO_START_POLL", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RunnerAgentKey != "shell-agent" {
		t.Fatalf("RunnerAgentKey = %q, want %q", cfg.RunnerAgentKey, "shell-agent")
	}
	if cfg.RunnerBaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("RunnerBaseURL = %q, want %q", cfg.RunnerBaseURL, "http://127.0.0.1:8080")
	}
	if !cfg.AutoStartPoll {
		t.Fatalf("AutoStartPoll = false, want true from shell env")
	}
}

func TestLoadWithoutDotEnvDefaultsAutoStartPollToTrue(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	unsetEnv(t, "RUNNER_AGENT_KEY", "RUNNER_BASE_URL", "AUTO_START_POLL")
	t.Setenv("RUNNER_AGENT_KEY", "shell-agent")
	t.Setenv("RUNNER_BASE_URL", "http://127.0.0.1:8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RunnerAgentKey != "shell-agent" {
		t.Fatalf("RunnerAgentKey = %q, want %q", cfg.RunnerAgentKey, "shell-agent")
	}
	if cfg.BridgeHTTPAddr != ":11958" {
		t.Fatalf("BridgeHTTPAddr = %q, want %q", cfg.BridgeHTTPAddr, ":11958")
	}
	if !cfg.AutoStartPoll {
		t.Fatalf("AutoStartPoll = false, want true default")
	}
}

func TestLoadAllowsExplicitAutoStartPollFalse(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	unsetEnv(t, "RUNNER_AGENT_KEY", "RUNNER_BASE_URL", "AUTO_START_POLL")
	t.Setenv("RUNNER_AGENT_KEY", "shell-agent")
	t.Setenv("RUNNER_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("AUTO_START_POLL", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AutoStartPoll {
		t.Fatalf("AutoStartPoll = true, want false")
	}
}

func TestLoadRequiresRunnerAgentKeyWhenMissingEverywhere(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	unsetEnv(t, "RUNNER_AGENT_KEY", "RUNNER_BASE_URL", "AUTO_START_POLL")
	t.Setenv("RUNNER_BASE_URL", "http://127.0.0.1:8080")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "RUNNER_AGENT_KEY is required") {
		t.Fatalf("Load() error = %v, want RUNNER_AGENT_KEY validation", err)
	}
}

func writeEnvFile(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		originalKey := key
		value, ok := os.LookupEnv(originalKey)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%q) error = %v", key, err)
		}
		if ok {
			originalValue := value
			t.Cleanup(func() {
				_ = os.Setenv(originalKey, originalValue)
			})
		}
	}
}
