package config

import (
	"flag"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	// DefaultAddress is the default HTTP server address.
	DefaultAddress = "localhost:8080"
	// DefaultBaseURL is the default base URL used to build short links.
	DefaultBaseURL = "http://localhost:8080"
	// DefaultFilePath is the default path to the JSON storage file.
	DefaultFilePath = "storage.json"
)

// Config holds runtime configuration for the shortener server.
type Config struct {
	Address     string `env:"SERVER_ADDRESS" env-default:"localhost:8080"`
	BaseURL     string `env:"BASE_URL" env-default:"http://localhost:8080"`
	FilePath    string `env:"FILE_STORAGE_PATH" env-default:"storage.json"`
	DatabaseDSN string `env:"DATABASE_DSN"`
	AuditFile   string `env:"AUDIT_FILE"`
	AuditURL    string `env:"AUDIT_URL"`
}

// FromFlags parses command-line flags and environment variables and returns Config.
func FromFlags() *Config {
	var cfg Config

	addrFlag := flag.String("a", DefaultAddress, "HTTP server address")
	baseFlag := flag.String("b", DefaultBaseURL, "Base URL for short links")
	fileFlag := flag.String("f", DefaultFilePath, "Path to JSON storage file")
	dsnFlag := flag.String("d", "", "PostgreSQL DSN (e.g. postgres://user:pass@host:5432/db?sslmode=disable)")
	auditFileFlag := flag.String("audit-file", "", "Path to audit log file (newline-delimited JSON)")
	auditURLFlag := flag.String("audit-url", "", "Remote audit receiver URL")
	flag.Parse()

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		cfg = Config{
			Address:  DefaultAddress,
			BaseURL:  DefaultBaseURL,
			FilePath: DefaultFilePath,
		}
	}

	if cfg.Address == DefaultAddress && *addrFlag != DefaultAddress {
		cfg.Address = *addrFlag
	}
	if cfg.BaseURL == DefaultBaseURL && *baseFlag != DefaultBaseURL {
		cfg.BaseURL = *baseFlag
	}
	if cfg.FilePath == DefaultFilePath && *fileFlag != DefaultFilePath {
		cfg.FilePath = *fileFlag
	}

	if cfg.DatabaseDSN == "" {
		cfg.DatabaseDSN = *dsnFlag
	}
	if cfg.AuditFile == "" {
		cfg.AuditFile = *auditFileFlag
	}
	if cfg.AuditURL == "" {
		cfg.AuditURL = *auditURLFlag
	}

	return &cfg
}

func NewConfig(flagAddr, flagBase, flagFile string) *Config {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		cfg = Config{
			Address:  DefaultAddress,
			BaseURL:  DefaultBaseURL,
			FilePath: DefaultFilePath,
		}
	}

	if cfg.Address == DefaultAddress && flagAddr != "" {
		cfg.Address = flagAddr
	}
	if cfg.BaseURL == DefaultBaseURL && flagBase != "" {
		cfg.BaseURL = flagBase
	}
	if cfg.FilePath == DefaultFilePath && flagFile != "" {
		cfg.FilePath = flagFile
	}

	return &cfg
}
