package config

import "testing"

func TestPriority_EnvWins(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "env:9000")
	t.Setenv("BASE_URL", "http://env:9000")
	t.Setenv("FILE_STORAGE_PATH", "/tmp/env.json")

	cfg := NewConfig("", "", "")
	if cfg.Address != "env:9000" || cfg.BaseURL != "http://env:9000" || cfg.FilePath != "/tmp/env.json" {
		t.Fatalf("env should win, got %+v", cfg)
	}
}

func TestFlagWinsOverDefault(t *testing.T) {
	cfg := NewConfig("flag:8081", "http://flag:8081", "flag.json")

	if cfg.Address != "flag:8081" {
		t.Fatalf("Address from flag expected, got %q", cfg.Address)
	}
	if cfg.BaseURL != "http://flag:8081" {
		t.Fatalf("BaseURL from flag expected, got %q", cfg.BaseURL)
	}
	if cfg.FilePath != "flag.json" {
		t.Fatalf("FilePath from flag expected, got %q", cfg.FilePath)
	}
}

func TestDefaultValues(t *testing.T) {
	cfg := NewConfig("", "", "")

	if cfg.Address != DefaultAddress {
		t.Fatalf("Default Address expected, got %q", cfg.Address)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("Default BaseURL expected, got %q", cfg.BaseURL)
	}
	if cfg.FilePath != DefaultFilePath {
		t.Fatalf("Default FilePath expected, got %q", cfg.FilePath)
	}
}
