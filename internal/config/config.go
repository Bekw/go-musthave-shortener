package config

import (
	"flag"
	"os"
)

const (
	DefaultAddress  = "localhost:8080"
	DefaultBaseURL  = "http://localhost:8080"
	DefaultFilePath = "storage.json"

	envServerAddr = "SERVER_ADDRESS"
	envBaseURL    = "BASE_URL"
	envFilePath   = "FILE_STORAGE_PATH"
)

type Config struct {
	Address  string
	BaseURL  string
	FilePath string
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

	return &Config{
		Address:  addr,
		BaseURL:  base,
		FilePath: file,
	}
}
func FromFlags() *Config {
	addrFlag := flag.String("a", DefaultAddress, "HTTP server address")
	baseFlag := flag.String("b", DefaultBaseURL, "Base URL for short links")
	fileFlag := flag.String("f", DefaultFilePath, "Path to JSON storage file")
	flag.Parse()
	return NewConfig(*addrFlag, *baseFlag, *fileFlag)
}
