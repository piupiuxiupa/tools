package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lush/dbschema-sync/pkg/types"
)

// Driver defines the interface for database-specific implementations.
// Each supported database (MySQL, PostgreSQL, SQLite, etc.) implements this interface.
type Driver interface {
	// Name returns the unique identifier for this driver (e.g., "mysql", "postgres", "sqlite3")
	Name() string

	// Connect establishes a connection to the database using the provided DSN.
	// Returns a *sql.DB handle or an error if connection fails.
	Connect(ctx context.Context, dsn string) (*sql.DB, error)

	// Ping verifies the connection to the database is still alive.
	Ping(ctx context.Context, db *sql.DB) error

	// Close releases any resources associated with the driver implementation.
	Close() error

	// QuoteIdentifier returns the identifier (table/column name) properly quoted
	// for the specific database dialect.
	QuoteIdentifier(name string) string

	// QuoteLiteral returns a literal value (string, number) properly quoted
	// for safe use in SQL queries.
	QuoteLiteral(value string) string
}

// BaseDriver provides common functionality for all drivers.
// Embed this in your driver implementation for convenience.
type BaseDriver struct {
	driverName string
}

// Name returns the driver's name.
func (d *BaseDriver) Name() string {
	return d.driverName
}

// Close is a no-op for base driver. Override in implementations if needed.
func (d *BaseDriver) Close() error {
	return nil
}

// DriverError wraps errors with driver context.
type DriverError struct {
	Driver string
	Op     string
	Err    error
}

func (e *DriverError) Error() string {
	return fmt.Sprintf("driver %s: %s: %v", e.Driver, e.Op, e.Err)
}

func (e *DriverError) Unwrap() error {
	return e.Err
}

// NewDriverError creates a new DriverError.
func NewDriverError(driver, op string, err error) error {
	return &DriverError{
		Driver: driver,
		Op:     op,
		Err:    err,
	}
}

type SchemaExtractor interface {
	ExtractSchema(ctx context.Context, db *sql.DB) (*types.DatabaseSchema, error)
	ExtractTables(ctx context.Context, db *sql.DB) ([]string, error)
	ExtractTable(ctx context.Context, db *sql.DB, tableName string) (*types.Table, error)
	ExtractColumns(ctx context.Context, db *sql.DB, tableName string) ([]*types.Column, error)
	ExtractIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*types.Index, error)
	ExtractForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]*types.ForeignKey, error)
}
