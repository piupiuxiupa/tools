package metadata

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/user/data-export-go/internal/config"
)

// TableMetadata holds information about a table
type TableMetadata struct {
	Database    string
	Table       string
	Columns     []string
	ColumnCount int
	RowCount    int64
	DataSizeMB  float64
}

// Service provides metadata operations
type Service interface {
	GetTableMetadata(ctx context.Context, database, table string) (*TableMetadata, error)
	GetColumns(ctx context.Context, database, table string) ([]string, error)
	GetRowCount(ctx context.Context, database, table string) (int64, error)
	GetTableSize(ctx context.Context, database, table string) (float64, error)
}

// service implements the Service interface
type service struct {
	db     *sql.DB
	dbType config.DBType
}

// NewService creates a new metadata service
func NewService(db *sql.DB, dbType config.DBType) Service {
	return &service{db: db, dbType: dbType}
}

// GetTableMetadata retrieves complete metadata for a table
func (s *service) GetTableMetadata(ctx context.Context, database, table string) (*TableMetadata, error) {
	columns, err := s.GetColumns(ctx, database, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for %s.%s: %w", database, table, err)
	}

	rowCount, err := s.GetRowCount(ctx, database, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get row count for %s.%s: %w", database, table, err)
	}

	dataSize, err := s.GetTableSize(ctx, database, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table size for %s.%s: %w", database, table, err)
	}

	return &TableMetadata{
		Database:    database,
		Table:       table,
		Columns:     columns,
		ColumnCount: len(columns),
		RowCount:    rowCount,
		DataSizeMB:  dataSize,
	}, nil
}

// GetColumns retrieves column names for a table
func (s *service) GetColumns(ctx context.Context, database, table string) ([]string, error) {
	var query string
	var args []interface{}

	switch s.dbType {
	case config.DBTypeOracle:
		// Oracle: use all_tab_columns, owner is case-insensitive in Oracle
		query = `SELECT column_name FROM all_tab_columns WHERE table_name = UPPER(?) AND owner = UPPER(?) ORDER BY column_id`
		args = []interface{}{table, database}
	case config.DBTypePostgres:
		// PostgreSQL: use information_schema
		query = `SELECT column_name FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`
		args = []interface{}{database, table}
	default:
		// MySQL and default: use backtick-quoted identifiers and information_schema
		query = `SELECT column_name FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`
		args = []interface{}{database, table}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("failed to scan column name: %w", err)
		}
		columns = append(columns, columnName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating columns: %w", err)
	}

	return columns, nil
}

// GetRowCount retrieves the number of rows in a table
func (s *service) GetRowCount(ctx context.Context, database, table string) (int64, error) {
	var query string

	switch s.dbType {
	case config.DBTypeOracle:
		// Oracle uses schema.table format, identifiers are case-insensitive unless quoted
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", database, table)
	case config.DBTypePostgres:
		// PostgreSQL uses quoted identifiers
		query = fmt.Sprintf("SELECT COUNT(*) FROM \"%s\".\"%s\"", database, table)
	default:
		// MySQL uses backtick-quoted identifiers
		query = fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", database, table)
	}

	var count int64
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get row count: %w", err)
	}

	return count, nil
}

// GetTableSize retrieves the data size of a table in MB
func (s *service) GetTableSize(ctx context.Context, database, table string) (float64, error) {
	var query string
	var args []interface{}

	switch s.dbType {
	case config.DBTypeOracle:
		// Oracle: use all_tables and all_segments for size calculation
		query = `SELECT ROUND(SUM(bytes) / 1024 / 1024, 2) FROM all_segments WHERE owner = UPPER(?) AND segment_name = UPPER(?)`
		args = []interface{}{database, table}
	case config.DBTypePostgres:
		// PostgreSQL: use pg_total_relation_size
		query = `SELECT ROUND(pg_total_relation_size($1 || '.' || $2) / 1024.0 / 1024.0, 2)`
		args = []interface{}{database, table}
	default:
		// MySQL: use information_schema
		query = `SELECT ROUND(SUM(data_length) / 1024 / 1024, 2) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`
		args = []interface{}{database, table}
	}

	var size sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&size); err != nil {
		return 0, fmt.Errorf("failed to get table size: %w", err)
	}

	if !size.Valid {
		return 0, nil
	}

	return size.Float64, nil
}
