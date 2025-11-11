package config

import (
	"os"
)

const (
	DefaultAddress = "localhost:8080"
	DefaultBaseURL = "http://localhost:8080"

	envServerAddr = "SERVER_ADDRESS"
	envBaseURL    = "BASE_URL"
)

type Config struct {
	Address string
	BaseURL string
}

func NewConfig(flagAddr, flagBase string) *Config {
	addr := flagAddr
	base := flagBase
	if addr == "" {
		addr = DefaultAddress
	}
	if base == "" {
		base = DefaultBaseURL
	}

	// ENV перекрывает флаги
	if v := os.Getenv(envServerAddr); v != "" {
		addr = v
	}
	if v := os.Getenv(envBaseURL); v != "" {
		base = v
	}

	return &Config{
		Address: addr,
		BaseURL: base,
	}
}
