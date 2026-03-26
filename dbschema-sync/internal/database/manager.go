package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// ConnectionKey uniquely identifies a connection pool by driver and DSN.
type ConnectionKey struct {
	DriverName string
	DSN        string
}

// String returns a string representation of the connection key.
func (k ConnectionKey) String() string {
	return fmt.Sprintf("%s://%s", k.DriverName, k.DSN)
}

// PoolConfig contains configuration options for connection pools.
type PoolConfig struct {
	// MaxOpenConns is the maximum number of open connections to the database.
	// Default: 10
	MaxOpenConns int

	// MaxIdleConns is the maximum number of connections in the idle connection pool.
	// Default: 5
	MaxIdleConns int

	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	// Zero means no limit.
	// Default: 1 hour
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum amount of time a connection may be idle.
	// Zero means no limit.
	// Default: 30 minutes
	ConnMaxIdleTime time.Duration

	// PingTimeout is the timeout for ping operations.
	// Default: 5 seconds
	PingTimeout time.Duration
}

// DefaultPoolConfig returns a PoolConfig with sensible defaults.
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
		PingTimeout:     5 * time.Second,
	}
}

// ConnectionPool wraps *sql.DB with additional metadata.
type ConnectionPool struct {
	DB      *sql.DB
	Key     ConnectionKey
	Driver  Driver
	Created time.Time
}

// ConnectionManager manages database connection pools for multiple databases.
// It provides thread-safe access to connections and supports configuration
// of connection pool parameters.
//
// Inspired by golang-migrate's database connection handling and goose's
// connection management patterns.
type ConnectionManager struct {
	mu         sync.RWMutex
	pools      map[ConnectionKey]*ConnectionPool
	poolConfig *PoolConfig
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewConnectionManager creates a new ConnectionManager with the given pool configuration.
// If config is nil, DefaultPoolConfig() is used.
func NewConnectionManager(config *PoolConfig) *ConnectionManager {
	if config == nil {
		config = DefaultPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ConnectionManager{
		pools:      make(map[ConnectionKey]*ConnectionPool),
		poolConfig: config,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// GetConnection retrieves or creates a connection pool for the specified driver and DSN.
// If a connection already exists for this driver/DSN combination, it returns the existing pool.
// Otherwise, it creates a new connection using the registered driver.
//
// The returned *sql.DB is safe for concurrent use and should not be closed directly.
// Use CloseAll() to properly close all managed connections.
func (cm *ConnectionManager) GetConnection(driverName, dsn string) (*sql.DB, error) {
	key := ConnectionKey{DriverName: driverName, DSN: dsn}

	// Check if connection already exists
	cm.mu.RLock()
	if pool, exists := cm.pools[key]; exists {
		cm.mu.RUnlock()
		return pool.DB, nil
	}
	cm.mu.RUnlock()

	// Create new connection
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if pool, exists := cm.pools[key]; exists {
		return pool.DB, nil
	}

	// Get the driver from registry
	driver, err := Get(driverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver %q: %w", driverName, err)
	}

	// Establish connection
	ctx, cancel := context.WithTimeout(cm.ctx, cm.poolConfig.PingTimeout)
	defer cancel()

	db, err := driver.Connect(ctx, dsn)
	if err != nil {
		return nil, NewDriverError(driverName, "connect", err)
	}

	// Configure connection pool
	if cm.poolConfig.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cm.poolConfig.MaxOpenConns)
	}
	if cm.poolConfig.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cm.poolConfig.MaxIdleConns)
	}
	if cm.poolConfig.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cm.poolConfig.ConnMaxLifetime)
	}
	if cm.poolConfig.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cm.poolConfig.ConnMaxIdleTime)
	}

	// Verify connection with ping
	if err := driver.Ping(ctx, db); err != nil {
		db.Close()
		return nil, NewDriverError(driverName, "ping", err)
	}

	pool := &ConnectionPool{
		DB:      db,
		Key:     key,
		Driver:  driver,
		Created: time.Now(),
	}

	cm.pools[key] = pool

	return db, nil
}

// GetConnectionPool retrieves the full ConnectionPool struct for a given driver and DSN.
// This is useful when you need access to the Driver or metadata associated with the connection.
func (cm *ConnectionManager) GetConnectionPool(driverName, dsn string) (*ConnectionPool, error) {
	key := ConnectionKey{DriverName: driverName, DSN: dsn}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	pool, exists := cm.pools[key]
	if !exists {
		return nil, fmt.Errorf("no connection pool found for driver=%q dsn=%q", driverName, dsn)
	}

	return pool, nil
}

// CloseConnection closes a specific connection pool identified by driver name and DSN.
// Returns an error if the connection doesn't exist or fails to close.
func (cm *ConnectionManager) CloseConnection(driverName, dsn string) error {
	key := ConnectionKey{DriverName: driverName, DSN: dsn}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	pool, exists := cm.pools[key]
	if !exists {
		return fmt.Errorf("no connection pool found for driver=%q dsn=%q", driverName, dsn)
	}

	delete(cm.pools, key)

	if err := pool.DB.Close(); err != nil {
		return fmt.Errorf("failed to close connection pool for driver=%q: %w", driverName, err)
	}

	return nil
}

// CloseAll closes all managed connection pools and releases all resources.
// This method should be called when the application is shutting down.
// Returns an error if any connection pool fails to close properly.
func (cm *ConnectionManager) CloseAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var closeErrors []error

	// Cancel context to signal shutdown
	cm.cancel()

	// Close all connection pools
	for key, pool := range cm.pools {
		if err := pool.DB.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf(
				"failed to close connection pool %s: %w", key.String(), err))
		}
	}

	// Clear the pools map
	cm.pools = make(map[ConnectionKey]*ConnectionPool)

	if len(closeErrors) > 0 {
		return fmt.Errorf("errors closing connection pools: %v", closeErrors)
	}

	return nil
}

// ListConnections returns a list of all active connection keys.
// Useful for debugging and monitoring.
func (cm *ConnectionManager) ListConnections() []ConnectionKey {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	keys := make([]ConnectionKey, 0, len(cm.pools))
	for key := range cm.pools {
		keys = append(keys, key)
	}

	return keys
}

// Stats returns database statistics for all managed connections.
// The map key is the connection key string, and the value is sql.DBStats.
func (cm *ConnectionManager) Stats() map[string]sql.DBStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := make(map[string]sql.DBStats, len(cm.pools))
	for key, pool := range cm.pools {
		stats[key.String()] = pool.DB.Stats()
	}

	return stats
}

// PingAll verifies all managed connections are still alive.
// Returns a map of connection keys to errors for any failed pings.
func (cm *ConnectionManager) PingAll() map[string]error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	results := make(map[string]error)

	for key, pool := range cm.pools {
		ctx, cancel := context.WithTimeout(cm.ctx, cm.poolConfig.PingTimeout)
		if err := pool.Driver.Ping(ctx, pool.DB); err != nil {
			results[key.String()] = err
		}
		cancel()
	}

	return results
}
