package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any env vars that might interfere
	os.Unsetenv("PORT")
	os.Unsetenv("REDIS_ADDR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != "8082" {
		t.Errorf("Port = %q, want 8082", cfg.Port)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("RedisAddr = %q, want redis:6379", cfg.RedisAddr)
	}
	if cfg.RedisDB != 1 {
		t.Errorf("RedisDB = %d, want 1", cfg.RedisDB)
	}
	if cfg.FutureScheduleTTL != 8*time.Hour {
		t.Errorf("FutureScheduleTTL = %v, want 8h", cfg.FutureScheduleTTL)
	}
	if cfg.PastScheduleTTL != 192*time.Hour {
		t.Errorf("PastScheduleTTL = %v, want 192h", cfg.PastScheduleTTL)
	}
	if cfg.AthensLocation == nil {
		t.Error("AthensLocation is nil")
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("REDIS_DB", "5")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("REDIS_DB")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	if cfg.RedisDB != 5 {
		t.Errorf("RedisDB = %d, want 5", cfg.RedisDB)
	}
}

func TestParseDuration(t *testing.T) {
	if d := parseDuration("6h"); d != 6*time.Hour {
		t.Errorf("parseDuration(6h) = %v", d)
	}
	if d := parseDuration("invalid"); d != 6*time.Hour {
		t.Errorf("parseDuration(invalid) = %v, want fallback 6h", d)
	}
}
