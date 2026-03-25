package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/user/doris-export-go/internal/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// Config holds database connection configuration
type Config struct {
	DBType          config.DBType
	Host            string
	Port            int
	Username        string
	Password        string
	Database        string
	MaxOpenConns    int           // Default: 10
	MaxIdleConns    int           // Default: 10
	ConnMaxLifetime time.Duration // Default: 3 minutes
	ConnMaxIdleTime time.Duration // Default: 1 minute
	ReadTimeout     time.Duration // Default: 300s (5min)
	WriteTimeout    time.Duration // Default: 300s (5min)
	ConnectTimeout  time.Duration // Default: 30s
}

// Default values for configuration
const (
	DefaultMaxOpenConns    = 10
	DefaultMaxIdleConns    = 10
	DefaultConnMaxLifetime = 3 * time.Minute
	DefaultConnMaxIdleTime = 1 * time.Minute
	DefaultReadTimeout     = 300 * time.Second
	DefaultWriteTimeout    = 300 * time.Second
	DefaultConnectTimeout  = 30 * time.Second
)

// setDefaults sets default values for unset configuration fields
func (c *Config) setDefaults() {
	if c.MaxOpenConns <= 0 {
		c.MaxOpenConns = DefaultMaxOpenConns
	}
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = DefaultMaxIdleConns
	}
	if c.ConnMaxLifetime <= 0 {
		c.ConnMaxLifetime = DefaultConnMaxLifetime
	}
	if c.ConnMaxIdleTime <= 0 {
		c.ConnMaxIdleTime = DefaultConnMaxIdleTime
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = DefaultReadTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = DefaultConnectTimeout
	}
	// Set default DBType to MySQL if not specified
	if c.DBType == "" {
		c.DBType = config.DBTypeMySQL
	}
}

// ConnectionManager defines the interface for database connection management
type ConnectionManager interface {
	Connect(ctx context.Context) (*sql.DB, error)
	Ping(ctx context.Context) error
	Close() error
}

// manager implements ConnectionManager interface
type manager struct {
	config *Config
	db     *sql.DB
}

// NewConnectionManager creates a new ConnectionManager instance
func NewConnectionManager(config *Config) ConnectionManager {
	// Make a copy to avoid external modifications
	cfg := *config
	cfg.setDefaults()
	return &manager{
		config: &cfg,
	}
}

// buildDSN constructs the DSN string from configuration based on DBType
func (m *manager) buildDSN() (string, string, error) {
	switch m.config.DBType {
	case config.DBTypeMySQL:
		return m.buildMySQLDSN()
	case config.DBTypePostgres:
		return m.buildPostgresDSN()
	case config.DBTypeOracle:
		return m.buildOracleDSN()
	default:
		return "", "", fmt.Errorf("unsupported database type: %s", m.config.DBType)
	}
}

// buildMySQLDSN constructs the MySQL DSN string from configuration
func (m *manager) buildMySQLDSN() (string, string, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%s&readTimeout=%s&writeTimeout=%s&parseTime=true&loc=Local&charset=utf8mb4",
		m.config.Username,
		m.config.Password,
		m.config.Host,
		m.config.Port,
		m.config.Database,
		m.config.ConnectTimeout,
		m.config.ReadTimeout,
		m.config.WriteTimeout,
	)
	return "mysql", dsn, nil
}

// buildPostgresDSN constructs the PostgreSQL DSN string from configuration
func (m *manager) buildPostgresDSN() (string, string, error) {
	// PostgreSQL uses port as integer in DSN
	port := m.config.Port
	if port == 0 {
		port = 5432
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=%d",
		m.config.Host,
		port,
		m.config.Username,
		m.config.Password,
		m.config.Database,
		int(m.config.ConnectTimeout.Seconds()),
	)
	return "postgres", dsn, nil
}

// buildOracleDSN constructs the Oracle DSN string from configuration
func (m *manager) buildOracleDSN() (string, string, error) {
	// Oracle default port
	port := m.config.Port
	if port == 0 {
		port = 1521
	}
	// Oracle connection string format: user/password@host:port/service_name
	dsn := fmt.Sprintf("%s/%s@%s:%d/%s",
		m.config.Username,
		m.config.Password,
		m.config.Host,
		port,
		m.config.Database,
	)
	return "oci8", dsn, nil
}

// Connect establishes a database connection and configures the connection pool
func (m *manager) Connect(ctx context.Context) (*sql.DB, error) {
	driverName, dsn, err := m.buildDSN()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(m.config.MaxOpenConns)
	db.SetMaxIdleConns(m.config.MaxIdleConns)
	db.SetConnMaxLifetime(m.config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(m.config.ConnMaxIdleTime)

	// Verify connection with ping using context timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	m.db = db
	return db, nil
}

// Ping verifies the database connection is alive
func (m *manager) Ping(ctx context.Context) error {
	if m.db == nil {
		return fmt.Errorf("database connection not established")
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := m.db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// Close closes the database connection
func (m *manager) Close() error {
	if m.db == nil {
		return nil
	}

	if err := m.db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	m.db = nil
	return nil
}

// ParseDBType converts a string to DBType, returning error if invalid
func ParseDBType(s string) (config.DBType, error) {
	switch s {
	case "mysql":
		return config.DBTypeMySQL, nil
	case "oracle":
		return config.DBTypeOracle, nil
	case "postgres", "postgresql":
		return config.DBTypePostgres, nil
	default:
		return "", fmt.Errorf("invalid database type: %s (supported: mysql, oracle, postgres)", s)
	}
}

// GetDefaultPort returns the default port for a given database type
func GetDefaultPort(dbType config.DBType) int {
	switch dbType {
	case config.DBTypeMySQL:
		return 3306
	case config.DBTypePostgres:
		return 5432
	case config.DBTypeOracle:
		return 1521
	default:
		return 3306
	}
}

// FormatValue formats a value for use in SQL queries based on database type
func FormatValue(dbType config.DBType, value interface{}) string {
	switch v := value.(type) {
	case string:
		// Escape single quotes
		escaped := strconv.Quote(v)
		// Remove surrounding quotes added by strconv.Quote
		escaped = escaped[1 : len(escaped)-1]
		return "'" + escaped + "'"
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("%v", v)
	}
}
