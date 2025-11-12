package config

import (
	"flag"
	"os"
)

const (
	DefaultAddress  = "localhost:8080"
	DefaultBaseURL  = "http://localhost:8080"
	DefaultFilePath = "storage.json"

	envServerAddr  = "SERVER_ADDRESS"
	envBaseURL     = "BASE_URL"
	envFilePath    = "FILE_STORAGE_PATH"
	envDatabaseDSN = "DATABASE_DSN"
)

type Config struct {
	Address     string
	BaseURL     string
	FilePath    string
	DatabaseDSN string
}

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
	if v, ok := os.LookupEnv(envDatabaseDSN); ok {
		cfg.DatabaseDSN = v
	}
	return cfg
}

func FromFlags() *Config {
	addrFlag := flag.String("a", DefaultAddress, "HTTP server address")
	baseFlag := flag.String("b", DefaultBaseURL, "Base URL for short links")
	fileFlag := flag.String("f", DefaultFilePath, "Path to JSON storage file")
	dsnFlag := flag.String("d", "", "PostgreSQL DSN (e.g. postgres://user:pass@host:5432/db?sslmode=disable)")
	flag.Parse()

	cfg := NewConfig(*addrFlag, *baseFlag, *fileFlag)
	if _, ok := os.LookupEnv(envDatabaseDSN); !ok {
		cfg.DatabaseDSN = *dsnFlag
	}
	return cfg
}
