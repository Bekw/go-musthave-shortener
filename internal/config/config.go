package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
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
	Address       string
	BaseURL       string
	FilePath      string
	DatabaseDSN   string
	EnableHTTPS   bool
	TrustedSubnet string

	// Not part of the YP track options, but used in this repo.
	AuditFile string
	AuditURL  string
}

// jsonConfig matches the config.json format from the task.
// We use pointers to distinguish “field is absent” vs “field is present with zero value”.
type jsonConfig struct {
	Address       *string `json:"server_address"`
	BaseURL       *string `json:"base_url"`
	FilePath      *string `json:"file_storage_path"`
	DatabaseDSN   *string `json:"database_dsn"`
	EnableHTTPS   *bool   `json:"enable_https"`
	TrustedSubnet *string `json:"trusted_subnet"`

	// Optional fields (not required by the task but supported if present).
	AuditFile *string `json:"audit_file,omitempty"`
	AuditURL  *string `json:"audit_url,omitempty"`
}

func defaultConfig() Config {
	return Config{
		Address:  DefaultAddress,
		BaseURL:  DefaultBaseURL,
		FilePath: DefaultFilePath,
	}
}

func applyJSONFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: can't read %q: %w", path, err)
	}

	var jc jsonConfig
	if err := json.Unmarshal(data, &jc); err != nil {
		return fmt.Errorf("config: can't parse %q: %w", path, err)
	}

	if jc.Address != nil {
		cfg.Address = *jc.Address
	}
	if jc.BaseURL != nil {
		cfg.BaseURL = *jc.BaseURL
	}
	if jc.FilePath != nil {
		cfg.FilePath = *jc.FilePath
	}
	if jc.DatabaseDSN != nil {
		cfg.DatabaseDSN = *jc.DatabaseDSN
	}
	if jc.EnableHTTPS != nil {
		cfg.EnableHTTPS = *jc.EnableHTTPS
	}
	if jc.TrustedSubnet != nil {
		cfg.TrustedSubnet = *jc.TrustedSubnet
	}
	if jc.AuditFile != nil {
		cfg.AuditFile = *jc.AuditFile
	}
	if jc.AuditURL != nil {
		cfg.AuditURL = *jc.AuditURL
	}

	return nil
}

func applyEnv(cfg *Config) {
	if v, ok := os.LookupEnv("SERVER_ADDRESS"); ok {
		cfg.Address = v
	}
	if v, ok := os.LookupEnv("BASE_URL"); ok {
		cfg.BaseURL = v
	}
	if v, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.FilePath = v
	}
	if v, ok := os.LookupEnv("DATABASE_DSN"); ok {
		cfg.DatabaseDSN = v
	}
	if v, ok := os.LookupEnv("ENABLE_HTTPS"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.EnableHTTPS = b
		}
	}
	if v, ok := os.LookupEnv("TRUSTED_SUBNET"); ok {
		cfg.TrustedSubnet = v
	}
	if v, ok := os.LookupEnv("AUDIT_FILE"); ok {
		cfg.AuditFile = v
	}
	if v, ok := os.LookupEnv("AUDIT_URL"); ok {
		cfg.AuditURL = v
	}
}

func FromFlags() (*Config, error) {
	cfg := defaultConfig()

	var (
		addrFlag      string
		baseFlag      string
		fileFlag      string
		dsnFlag       string
		enableHTTPS   bool
		trustedSubnet string
		configPath    string
		auditFileFlag string
		auditURLFlag  string
	)

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&addrFlag, "a", "", "HTTP server address")
	fs.StringVar(&baseFlag, "b", "", "Base URL for short links")
	fs.StringVar(&fileFlag, "f", "", "Path to JSON storage file")
	fs.StringVar(&dsnFlag, "d", "", "PostgreSQL DSN (e.g. postgres://user:pass@host:5432/db?sslmode=disable)")
	fs.BoolVar(&enableHTTPS, "s", false, "Enable HTTPS")
	fs.StringVar(&trustedSubnet, "t", "", "Trusted subnet CIDR for /api/internal/* endpoints")
	fs.StringVar(&configPath, "c", "", "Path to JSON config file")
	fs.StringVar(&configPath, "config", "", "Path to JSON config file")
	fs.StringVar(&auditFileFlag, "audit-file", "", "Path to audit log file (newline-delimited JSON)")
	fs.StringVar(&auditURLFlag, "audit-url", "", "Remote audit receiver URL")
	_ = fs.Parse(os.Args[1:])

	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})

	path := os.Getenv("CONFIG")
	if (provided["c"] || provided["config"]) && configPath != "" {
		path = configPath
	}
	if path != "" {
		if err := applyJSONFile(&cfg, path); err != nil {
			return nil, err
		}
	}

	applyEnv(&cfg)

	if provided["a"] {
		cfg.Address = addrFlag
	}
	if provided["b"] {
		cfg.BaseURL = baseFlag
	}
	if provided["f"] {
		cfg.FilePath = fileFlag
	}
	if provided["d"] {
		cfg.DatabaseDSN = dsnFlag
	}
	if provided["s"] {
		cfg.EnableHTTPS = enableHTTPS
	}
	if provided["t"] {
		cfg.TrustedSubnet = trustedSubnet
	}
	if provided["audit-file"] {
		cfg.AuditFile = auditFileFlag
	}
	if provided["audit-url"] {
		cfg.AuditURL = auditURLFlag
	}

	return &cfg, nil
}

func NewConfig(flagAddr, flagBase, flagFile string) *Config {
	cfg := defaultConfig()
	applyEnv(&cfg)

	if flagAddr != "" {
		cfg.Address = flagAddr
	}
	if flagBase != "" {
		cfg.BaseURL = flagBase
	}
	if flagFile != "" {
		cfg.FilePath = flagFile
	}

	return &cfg
}
