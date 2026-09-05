package config

import "testing"

func TestAIConfigDefaultsToDisabled(t *testing.T) {
	isolateEnvironment(t)

	cfg := NewConfig()

	if cfg.AI.ServiceURL != "" {
		t.Errorf("AI.ServiceURL = %q, mau kosong tanpa env", cfg.AI.ServiceURL)
	}
	if cfg.AI.Timeout != 3500 {
		t.Errorf("AI.Timeout = %d, mau 3500", cfg.AI.Timeout)
	}
	if cfg.AI.WarmupInterval != 0 {
		t.Errorf("AI.WarmupInterval = %d, mau 0", cfg.AI.WarmupInterval)
	}
}

func TestAIConfigReadsEnvironment(t *testing.T) {
	isolateEnvironment(t)

	t.Setenv("AI_SERVICE_URL", "https://ai.example.test")
	t.Setenv("AI_SERVICE_TOKEN", "rahasia")
	t.Setenv("AI_SERVICE_TIMEOUT_MS", "2000")
	t.Setenv("AI_WARMUP_INTERVAL", "240")

	cfg := NewConfig()

	if cfg.AI.ServiceURL != "https://ai.example.test" {
		t.Errorf("AI.ServiceURL = %q", cfg.AI.ServiceURL)
	}
	if cfg.AI.Token != "rahasia" {
		t.Errorf("AI.Token = %q", cfg.AI.Token)
	}
	if cfg.AI.Timeout != 2000 {
		t.Errorf("AI.Timeout = %d", cfg.AI.Timeout)
	}
	if cfg.AI.WarmupInterval != 240 {
		t.Errorf("AI.WarmupInterval = %d", cfg.AI.WarmupInterval)
	}
}
