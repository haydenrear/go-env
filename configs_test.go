package lib

import (
	"os"
	"testing"
	"time"
)

// 1. Nested struct + pointer-to-struct test.
func TestLoadEnvNested(t *testing.T) {
	type NestedConfig struct {
		Level1 struct {
			Name   string `env:"LEVEL1_NAME"`
			Level2 *struct {
				Value int    `env:"LEVEL2_VALUE"`
				Flag  bool   `env:"LEVEL2_FLAG"`
				Tags  []int  `env:"LEVEL2_TAGS"`
				Label string `env:"LEVEL2_LABEL"`
			}
		}
	}

	t.Setenv("LEVEL1_NAME", "root")
	t.Setenv("LEVEL2_VALUE", "42")
	t.Setenv("LEVEL2_FLAG", "true")
	t.Setenv("LEVEL2_TAGS", "1,2,3")
	t.Setenv("LEVEL2_LABEL", "nested")

	var cfg NestedConfig

	if err := LoadEnv(&cfg); err != nil {
		t.Fatalf("LoadEnv failed: %v", err)
	}

	if cfg.Level1.Name != "root" {
		t.Errorf("expected Level1.Name = %q, got %q", "root", cfg.Level1.Name)
	}

	if cfg.Level1.Level2 == nil {
		t.Fatalf("expected Level1.Level2 to be allocated, got nil")
	}

	if cfg.Level1.Level2.Value != 42 {
		t.Errorf("expected Level2.Value = 42, got %d", cfg.Level1.Level2.Value)
	}

	if !cfg.Level1.Level2.Flag {
		t.Errorf("expected Level2.Flag = true, got false")
	}

	expectedTags := []int{1, 2, 3}
	if len(cfg.Level1.Level2.Tags) != len(expectedTags) {
		t.Fatalf("expected %d tags, got %d", len(expectedTags), len(cfg.Level1.Level2.Tags))
	}
	for i, v := range expectedTags {
		if cfg.Level1.Level2.Tags[i] != v {
			t.Errorf("expected Tags[%d] = %d, got %d", i, v, cfg.Level1.Level2.Tags[i])
		}
	}

	if cfg.Level1.Level2.Label != "nested" {
		t.Errorf("expected Level2.Label = %q, got %q", "nested", cfg.Level1.Level2.Label)
	}
}

// 2. Duration parsing test.
func TestLoadEnvDuration(t *testing.T) {
	type DurConfig struct {
		Timeout   time.Duration `env:"TIMEOUT"`
		Backoff   time.Duration `env:"BACKOFF"`
		ZeroValue time.Duration `env:"ZERO_TIMEOUT"`
	}

	t.Setenv("TIMEOUT", "5s")
	t.Setenv("BACKOFF", "150ms")
	t.Setenv("ZERO_TIMEOUT", "0s")

	var cfg DurConfig
	if err := LoadEnv(&cfg); err != nil {
		t.Fatalf("LoadEnv failed: %v", err)
	}

	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected Timeout = 5s, got %s", cfg.Timeout)
	}
	if cfg.Backoff != 150*time.Millisecond {
		t.Errorf("expected Backoff = 150ms, got %s", cfg.Backoff)
	}
	if cfg.ZeroValue != 0 {
		t.Errorf("expected ZeroValue = 0s, got %s", cfg.ZeroValue)
	}
}

// 3. TOML precedence vs env overrides.
func TestLoadConfigTomlThenEnvOverride(t *testing.T) {
	// Use a local struct with env tags to exercise the precedence.
	type ServerConfig struct {
		Port int    `env:"SERVER_PORT"`
		Host string `env:"SERVER_HOST"`
	}
	type DatabaseConfig struct {
		User     string `env:"DB_USER"`
		Password string `env:"DB_PASSWORD"`
		Name     string `env:"DB_NAME"`
	}
	type TestConfig struct {
		Server   ServerConfig
		Database DatabaseConfig
	}

	// Create a temp TOML file with some defaults.
	tomlContent := `
[Server]
Port = 1234
Host = "from-toml"

[Database]
User = "toml-user"
Password = "toml-pass"
Name = "toml-db"
`
	tmpFile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(tomlContent); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp config: %v", err)
	}

	// Env vars that should override TOML where present.
	t.Setenv("SERVER_PORT", "5678") // override TOML 1234
	// SERVER_HOST not set -> should stay "from-toml"
	t.Setenv("DB_USER", "env-user")     // override TOML
	t.Setenv("DB_PASSWORD", "env-pass") // override TOML
	// DB_NAME not set -> should stay "toml-db"

	var cfg TestConfig
	if err := LoadConfig(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Server assertions
	if cfg.Server.Port != 5678 {
		t.Errorf("expected Server.Port from env (5678), got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "from-toml" {
		t.Errorf("expected Server.Host from toml %q, got %q", "from-toml", cfg.Server.Host)
	}

	// Database assertions
	if cfg.Database.User != "env-user" {
		t.Errorf("expected Database.User from env %q, got %q", "env-user", cfg.Database.User)
	}
	if cfg.Database.Password != "env-pass" {
		t.Errorf("expected Database.Password from env %q, got %q", "env-pass", cfg.Database.Password)
	}
	if cfg.Database.Name != "toml-db" {
		t.Errorf("expected Database.Name from toml %q, got %q", "toml-db", cfg.Database.Name)
	}
}
