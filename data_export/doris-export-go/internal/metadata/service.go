package metadata

import (
	"context"
	"database/sql"
	"fmt"
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
	db *sql.DB
}

// NewService creates a new metadata service
func NewService(db *sql.DB) Service {
	return &service{db: db}
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
	query := `SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION`

	rows, err := s.db.QueryContext(ctx, query, database, table)
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
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", database, table)

	var count int64
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get row count: %w", err)
	}

	return count, nil
}

// GetTableSize retrieves the data size of a table in MB
func (s *service) GetTableSize(ctx context.Context, database, table string) (float64, error) {
	query := `SELECT ROUND(SUM(data_length) / 1024 / 1024, 2) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`

	var size sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, query, database, table).Scan(&size); err != nil {
		return 0, fmt.Errorf("failed to get table size: %w", err)
	}

	if !size.Valid {
		return 0, nil
	}

	return size.Float64, nil
}
