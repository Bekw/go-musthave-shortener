package config

import (
	"flag"
	"os"
)

const (
	// DefaultAddress is the default HTTP server address.
	DefaultAddress = "localhost:8080"
	// DefaultBaseURL is the default base URL used to build short links.
	DefaultBaseURL = "http://localhost:8080"
	// DefaultFilePath is the default path to the JSON storage file.
	DefaultFilePath = "storage.json"

	envServerAddr  = "SERVER_ADDRESS"
	envBaseURL     = "BASE_URL"
	envFilePath    = "FILE_STORAGE_PATH"
	envDatabaseDSN = "DATABASE_DSN"

	envAuditFile = "AUDIT_FILE"
	envAuditURL  = "AUDIT_URL"
)

// Config holds runtime configuration for the shortener server.
type Config struct {
	Address     string
	BaseURL     string
	FilePath    string
	DatabaseDSN string

	AuditFile string
	AuditURL  string
}

// NewConfig builds Config from flag values overridden by environment variables.
func NewConfig(flagAddr, flagBase, flagFile string) *Config {
	addr := flagAddr
	base := flagBase
	file := flagFile

	if addr == "" {
		addr = DefaultAddress
	}
	if base == "" {
		base = DefaultBaseURL
	}
	if file == "" {
		file = DefaultFilePath
	}

	if v, ok := os.LookupEnv(envServerAddr); ok {
		addr = v
	}
	if v, ok := os.LookupEnv(envBaseURL); ok {
		base = v
	}
	if v, ok := os.LookupEnv(envFilePath); ok {
		file = v
	}

	cfg := &Config{
		Address:  addr,
		BaseURL:  base,
		FilePath: file,
	}
	if v, ok := os.LookupEnv(envAuditFile); ok {
		cfg.AuditFile = v
	}
	if v, ok := os.LookupEnv(envAuditURL); ok {
		cfg.AuditURL = v
	}
	if v, ok := os.LookupEnv(envDatabaseDSN); ok {
		cfg.DatabaseDSN = v
	}
	return cfg
}

// FromFlags parses command-line flags and environment variables and returns Config.
func FromFlags() *Config {
	addrFlag := flag.String("a", DefaultAddress, "HTTP server address")
	baseFlag := flag.String("b", DefaultBaseURL, "Base URL for short links")
	fileFlag := flag.String("f", DefaultFilePath, "Path to JSON storage file")
	dsnFlag := flag.String("d", "", "PostgreSQL DSN (e.g. postgres://user:pass@host:5432/db?sslmode=disable)")
	auditFileFlag := flag.String("audit-file", "", "Path to audit log file (newline-delimited JSON)")
	auditURLFlag := flag.String("audit-url", "", "Remote audit receiver URL")
	flag.Parse()

	cfg := NewConfig(*addrFlag, *baseFlag, *fileFlag)
	if _, ok := os.LookupEnv(envDatabaseDSN); !ok {
		cfg.DatabaseDSN = *dsnFlag
	}
	if _, ok := os.LookupEnv(envAuditFile); !ok {
		cfg.AuditFile = *auditFileFlag
	}
	if _, ok := os.LookupEnv(envAuditURL); !ok {
		cfg.AuditURL = *auditURLFlag
	}
	return cfg
}
