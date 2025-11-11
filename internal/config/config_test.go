package config

import "testing"

func TestPriority_EnvWins(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "env:9000")
	t.Setenv("BASE_URL", "http://env:9000")

	cfg := NewConfig("", "") // без флагов
	if cfg.Address != "env:9000" || cfg.BaseURL != "http://env:9000" {
		t.Fatalf("env should win, got %+v", cfg)
	}
}

func TestFlagWinsOverDefault(t *testing.T) {
	// нет env — победят значения, пришедшие как «флаги»
	cfg := NewConfig("flag:8081", "http://flag:8081")
	if cfg.Address != "flag:8081" || cfg.BaseURL != "http://flag:8081" {
		t.Fatalf("flag should win over default, got %+v", cfg)
	}
}

func TestDefaultValues(t *testing.T) {
	cfg := NewConfig("", "")
	if cfg.Address != DefaultAddress || cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("defaults expected, got %+v", cfg)
	}
}
