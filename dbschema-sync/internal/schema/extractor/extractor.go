// Package extractor provides interfaces and implementations for extracting database schema information.
package extractor

import (
	"context"
	"database/sql"

	"github.com/lush/dbschema-sync/pkg/types"
)

// SchemaExtractor defines the interface for extracting database schema information.
// Implementations are database-specific and handle the particularities of each
// database system's metadata schema.
type SchemaExtractor interface {
	// ExtractSchema extracts the complete database schema including all tables,
	// columns, indexes, and foreign keys.
	ExtractSchema(ctx context.Context, db *sql.DB) (*types.DatabaseSchema, error)

	// ExtractTables retrieves a list of all table names in the database.
	ExtractTables(ctx context.Context, db *sql.DB) ([]string, error)

	// ExtractTable extracts complete information about a specific table including
	// its columns, indexes, and foreign keys.
	ExtractTable(ctx context.Context, db *sql.DB, tableName string) (*types.Table, error)

	// ExtractColumns retrieves column information for a specific table.
	ExtractColumns(ctx context.Context, db *sql.DB, tableName string) ([]*types.Column, error)

	// ExtractIndexes retrieves index information for a specific table.
	ExtractIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*types.Index, error)

	// ExtractForeignKeys retrieves foreign key information for a specific table.
	ExtractForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]*types.ForeignKey, error)
}
