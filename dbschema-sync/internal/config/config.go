package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DatabaseConfig holds configuration for a single database connection.
type DatabaseConfig struct {
	// Driver is the database driver name (e.g., "mysql", "postgres", "sqlite3")
	Driver string `mapstructure:"driver"`

	// Host is the database server hostname or IP address
	Host string `mapstructure:"host"`

	// Port is the database server port
	Port int `mapstructure:"port"`

	// Database is the name of the database/schema
	Database string `mapstructure:"database"`

	// User is the database username
	User string `mapstructure:"user"`

	// Password is the database password
	Password string `mapstructure:"password"`

	// DSN is the direct Data Source Name connection string.
	// If provided, it takes precedence over individual connection parameters.
	DSN string `mapstructure:"dsn"`

	// SSLMode is the SSL mode for PostgreSQL connections (disable, require, verify-ca, verify-full)
	SSLMode string `mapstructure:"sslmode"`

	// Params are additional connection parameters
	Params map[string]string `mapstructure:"params"`
}

// BuildDSN constructs a DSN string from individual connection parameters.
// If DSN is already set, it returns that value.
func (c *DatabaseConfig) BuildDSN() (string, error) {
	if c.DSN != "" {
		return c.DSN, nil
	}

	switch strings.ToLower(c.Driver) {
	case "mysql":
		return c.buildMySQLDSN()
	case "postgres", "postgresql":
		return c.buildPostgresDSN()
	case "sqlite", "sqlite3":
		return c.buildSQLiteDSN()
	default:
		return "", fmt.Errorf("unsupported driver: %s", c.Driver)
	}
}

func (c *DatabaseConfig) buildMySQLDSN() (string, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		c.User, c.Password, c.Host, c.Port, c.Database)

	if c.Params == nil {
		c.Params = make(map[string]string)
	}

	// Default params for MySQL
	if _, ok := c.Params["parseTime"]; !ok {
		c.Params["parseTime"] = "true"
	}

	var params []string
	for k, v := range c.Params {
		params = append(params, fmt.Sprintf("%s=%s", k, v))
	}

	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}

	return dsn, nil
}

func (c *DatabaseConfig) buildPostgresDSN() (string, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		c.Host, c.Port, c.User, c.Password, c.Database)

	if c.SSLMode != "" {
		dsn += fmt.Sprintf(" sslmode=%s", c.SSLMode)
	} else {
		dsn += " sslmode=disable"
	}

	for k, v := range c.Params {
		dsn += fmt.Sprintf(" %s=%s", k, v)
	}

	return dsn, nil
}

func (c *DatabaseConfig) buildSQLiteDSN() (string, error) {
	if c.Database == "" {
		return ":memory:", nil
	}
	return c.Database, nil
}

// ConnectionPoolConfig holds connection pool settings.
type ConnectionPoolConfig struct {
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// SyncConfig holds synchronization-specific settings.
type SyncConfig struct {
	// DryRun when true, only shows what would be done without making changes
	DryRun bool `mapstructure:"dry_run"`

	// AutoApply when true, automatically applies schema changes without confirmation
	AutoApply bool `mapstructure:"auto_apply"`

	// IgnoreTables is a list of table names to exclude from synchronization
	IgnoreTables []string `mapstructure:"ignore_tables"`

	// IncludeTables is a list of table names to include (empty = all tables)
	IncludeTables []string `mapstructure:"include_tables"`

	// SkipForeignKeys when true, skips foreign key constraint synchronization
	SkipForeignKeys bool `mapstructure:"skip_foreign_keys"`

	// SkipIndexes when true, skips index synchronization
	SkipIndexes bool `mapstructure:"skip_indexes"`

	// BatchSize is the number of operations to process in a single transaction
	BatchSize int `mapstructure:"batch_size"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, text
	Output string `mapstructure:"output"` // stdout, stderr, file path
}

// Config is the root configuration structure.
type Config struct {
	// Source is the source database configuration (reference schema)
	Source DatabaseConfig `mapstructure:"source"`

	// Target is the target database configuration (to be synchronized)
	Target DatabaseConfig `mapstructure:"target"`

	// Sync holds synchronization-specific settings
	Sync SyncConfig `mapstructure:"sync"`

	// Pool holds connection pool configuration
	Pool ConnectionPoolConfig `mapstructure:"pool"`

	// Log holds logging configuration
	Log LogConfig `mapstructure:"log"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Source: DatabaseConfig{
			Host:    "localhost",
			Port:    3306,
			SSLMode: "disable",
			Params:  make(map[string]string),
		},
		Target: DatabaseConfig{
			Host:    "localhost",
			Port:    3306,
			SSLMode: "disable",
			Params:  make(map[string]string),
		},
		Sync: SyncConfig{
			DryRun:          false,
			AutoApply:       false,
			IgnoreTables:    []string{},
			IncludeTables:   []string{},
			SkipForeignKeys: false,
			SkipIndexes:     false,
			BatchSize:       100,
		},
		Pool: ConnectionPoolConfig{
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 30 * time.Minute,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
	}
}

// Load reads configuration from file and environment variables.
// Configuration precedence (highest to lowest):
// 1. Environment variables (DBSYNC_*)
// 2. Config file values
// 3. Default values
//
// Supported config file formats: YAML, JSON, TOML
// Environment variable prefix: DBSYNC_
//
// Example environment variables:
//
//	DBSYNC_SOURCE_DRIVER=mysql
//	DBSYNC_SOURCE_HOST=localhost
//	DBSYNC_SOURCE_PORT=3306
//	DBSYNC_SOURCE_DATABASE=mydb
//	DBSYNC_SOURCE_USER=root
//	DBSYNC_SOURCE_PASSWORD=secret
//	DBSYNC_TARGET_DRIVER=postgres
//	DBSYNC_SYNC_DRY_RUN=true
func Load(cfgFile string) (*Config, error) {
	cfg := DefaultConfig()
	v := viper.New()

	// Set up environment variable handling
	v.SetEnvPrefix("DBSYNC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	// Load from config file if provided
	if cfgFile != "" {
		if err := loadFromFile(v, cfgFile); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Unmarshal into struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply environment variable overrides for nested structs
	applyEnvOverrides(cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Source defaults
	v.SetDefault("source.host", "localhost")
	v.SetDefault("source.port", 3306)
	v.SetDefault("source.sslmode", "disable")

	// Target defaults
	v.SetDefault("target.host", "localhost")
	v.SetDefault("target.port", 3306)
	v.SetDefault("target.sslmode", "disable")

	// Sync defaults
	v.SetDefault("sync.dry_run", false)
	v.SetDefault("sync.auto_apply", false)
	v.SetDefault("sync.batch_size", 100)

	// Pool defaults
	v.SetDefault("pool.max_open_conns", 10)
	v.SetDefault("pool.max_idle_conns", 5)
	v.SetDefault("pool.conn_max_lifetime", time.Hour)
	v.SetDefault("pool.conn_max_idle_time", 30*time.Minute)

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("log.output", "stdout")
}

func loadFromFile(v *viper.Viper, cfgFile string) error {
	// Get absolute path
	absPath, err := filepath.Abs(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to resolve config file path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", absPath)
	}

	// Set config file
	v.SetConfigFile(absPath)

	// Read config
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	return nil
}

func applyEnvOverrides(cfg *Config) {
	// Source overrides
	if v := os.Getenv("DBSYNC_SOURCE_DRIVER"); v != "" {
		cfg.Source.Driver = v
	}
	if v := os.Getenv("DBSYNC_SOURCE_HOST"); v != "" {
		cfg.Source.Host = v
	}
	if v := os.Getenv("DBSYNC_SOURCE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Source.Port = port
		}
	}
	if v := os.Getenv("DBSYNC_SOURCE_DATABASE"); v != "" {
		cfg.Source.Database = v
	}
	if v := os.Getenv("DBSYNC_SOURCE_USER"); v != "" {
		cfg.Source.User = v
	}
	if v := os.Getenv("DBSYNC_SOURCE_PASSWORD"); v != "" {
		cfg.Source.Password = v
	}
	if v := os.Getenv("DBSYNC_SOURCE_DSN"); v != "" {
		cfg.Source.DSN = v
	}

	// Target overrides
	if v := os.Getenv("DBSYNC_TARGET_DRIVER"); v != "" {
		cfg.Target.Driver = v
	}
	if v := os.Getenv("DBSYNC_TARGET_HOST"); v != "" {
		cfg.Target.Host = v
	}
	if v := os.Getenv("DBSYNC_TARGET_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Target.Port = port
		}
	}
	if v := os.Getenv("DBSYNC_TARGET_DATABASE"); v != "" {
		cfg.Target.Database = v
	}
	if v := os.Getenv("DBSYNC_TARGET_USER"); v != "" {
		cfg.Target.User = v
	}
	if v := os.Getenv("DBSYNC_TARGET_PASSWORD"); v != "" {
		cfg.Target.Password = v
	}
	if v := os.Getenv("DBSYNC_TARGET_DSN"); v != "" {
		cfg.Target.DSN = v
	}

	// Sync overrides
	if v := os.Getenv("DBSYNC_SYNC_DRY_RUN"); v != "" {
		cfg.Sync.DryRun = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("DBSYNC_SYNC_AUTO_APPLY"); v != "" {
		cfg.Sync.AutoApply = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("DBSYNC_SYNC_IGNORE_TABLES"); v != "" {
		cfg.Sync.IgnoreTables = strings.Split(v, ",")
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate source
	if err := validateDatabaseConfig("source", &c.Source); err != nil {
		return err
	}

	// Validate target
	if err := validateDatabaseConfig("target", &c.Target); err != nil {
		return err
	}

	// Validate pool config
	if c.Pool.MaxOpenConns < 1 {
		return fmt.Errorf("pool.max_open_conns must be at least 1")
	}
	if c.Pool.MaxIdleConns < 0 {
		return fmt.Errorf("pool.max_idle_conns must be non-negative")
	}
	if c.Pool.MaxIdleConns > c.Pool.MaxOpenConns {
		return fmt.Errorf("pool.max_idle_conns cannot exceed pool.max_open_conns")
	}

	return nil
}

func validateDatabaseConfig(name string, cfg *DatabaseConfig) error {
	// If DSN is provided, we don't need other parameters
	if cfg.DSN != "" {
		if cfg.Driver == "" {
			return fmt.Errorf("%s.driver is required even when DSN is provided", name)
		}
		return nil
	}

	// Without DSN, we need at least driver, host, and database
	if cfg.Driver == "" {
		return fmt.Errorf("%s.driver is required", name)
	}

	if cfg.Host == "" {
		return fmt.Errorf("%s.host is required", name)
	}

	if cfg.Database == "" {
		return fmt.Errorf("%s.database is required", name)
	}

	if cfg.Port == 0 {
		// Set default ports based on driver
		switch cfg.Driver {
		case "mysql":
			cfg.Port = 3306
		case "postgres", "postgresql":
			cfg.Port = 5432
		}
	}

	return nil
}

// String returns a string representation of the config (sensitive data redacted).
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{Source:{Driver:%s Host:%s Port:%d Database:%s}, Target:{Driver:%s Host:%s Port:%d Database:%s}, Sync:{DryRun:%v AutoApply:%v}}",
		c.Source.Driver, c.Source.Host, c.Source.Port, c.Source.Database,
		c.Target.Driver, c.Target.Host, c.Target.Port, c.Target.Database,
		c.Sync.DryRun, c.Sync.AutoApply,
	)
}
