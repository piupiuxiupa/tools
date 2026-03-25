package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Config holds database connection configuration
type Config struct {
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

// buildDSN constructs the MySQL DSN string from configuration
func (m *manager) buildDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%s&readTimeout=%s&writeTimeout=%s&parseTime=true&loc=Local&charset=utf8mb4",
		m.config.Username,
		m.config.Password,
		m.config.Host,
		m.config.Port,
		m.config.Database,
		m.config.ConnectTimeout,
		m.config.ReadTimeout,
		m.config.WriteTimeout,
	)
}

// Connect establishes a database connection and configures the connection pool
func (m *manager) Connect(ctx context.Context) (*sql.DB, error) {
	dsn := m.buildDSN()

	db, err := sql.Open("mysql", dsn)
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
