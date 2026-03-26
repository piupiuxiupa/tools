// Package oracle provides Oracle driver implementation for database schema extraction.
package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/godror/godror" // Oracle driver
	"github.com/lush/dbschema-sync/internal/database"
	"github.com/lush/dbschema-sync/pkg/types"
)

func init() {
	oracleDriver := &Driver{}
	if err := database.Register("oracle", oracleDriver); err != nil {
		panic(fmt.Sprintf("failed to register oracle driver: %v", err))
	}
}

// Driver implements the database.Driver and SchemaExtractor interfaces for Oracle.
type Driver struct {
	base database.BaseDriver
}

// Name returns the driver's name.
func (d *Driver) Name() string {
	return "oracle"
}

// Connect establishes a connection to the Oracle database.
func (d *Driver) Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("godror", dsn)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "connect", err)
	}

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, database.NewDriverError(d.Name(), "ping", err)
	}

	return db, nil
}

// Ping verifies the connection to the database is still alive.
func (d *Driver) Ping(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return database.NewDriverError(d.Name(), "ping", err)
	}
	return nil
}

// Close releases any resources associated with the driver implementation.
func (d *Driver) Close() error {
	return nil
}

// QuoteIdentifier returns the identifier properly quoted for Oracle.
func (d *Driver) QuoteIdentifier(name string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(name, `"`, `""`))
}

// QuoteLiteral returns a literal value properly quoted for Oracle.
func (d *Driver) QuoteLiteral(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "''"))
}

// ExtractSchema extracts the complete database schema.
func (d *Driver) ExtractSchema(ctx context.Context, db *sql.DB) (*types.DatabaseSchema, error) {
	schema := &types.DatabaseSchema{
		Tables: []*types.Table{},
	}

	// Get database name (service name)
	var dbName string
	if err := db.QueryRowContext(ctx, "SELECT SYS_CONTEXT('USERENV', 'SERVICE_NAME') FROM DUAL").Scan(&dbName); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_schema_get_dbname", err)
	}
	schema.Name = dbName

	// Get current user (schema owner)
	var schemaOwner string
	if err := db.QueryRowContext(ctx, "SELECT USER FROM DUAL").Scan(&schemaOwner); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_schema_get_owner", err)
	}

	// Get all tables
	tables, err := d.extractTablesWithOwner(ctx, db, schemaOwner)
	if err != nil {
		return nil, err
	}

	// Extract schema for each table
	for _, tableName := range tables {
		table, err := d.extractTableWithOwner(ctx, db, schemaOwner, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

// ExtractTables retrieves all table names from the current schema.
func (d *Driver) ExtractTables(ctx context.Context, db *sql.DB) ([]string, error) {
	var schemaOwner string
	if err := db.QueryRowContext(ctx, "SELECT USER FROM DUAL").Scan(&schemaOwner); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_tables_get_owner", err)
	}
	return d.extractTablesWithOwner(ctx, db, schemaOwner)
}

// extractTablesWithOwner retrieves all table names for a specific owner.
func (d *Driver) extractTablesWithOwner(ctx context.Context, db *sql.DB, owner string) ([]string, error) {
	query := `SELECT TABLE_NAME FROM ALL_TABLES WHERE OWNER = UPPER(?) ORDER BY TABLE_NAME`

	rows, err := db.QueryContext(ctx, query, owner)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_tables", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, database.NewDriverError(d.Name(), "scan_table", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError(d.Name(), "iterate_tables", err)
	}

	return tables, nil
}

// ExtractTable extracts complete information about a single table.
func (d *Driver) ExtractTable(ctx context.Context, db *sql.DB, tableName string) (*types.Table, error) {
	var schemaOwner string
	if err := db.QueryRowContext(ctx, "SELECT USER FROM DUAL").Scan(&schemaOwner); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_table_get_owner", err)
	}
	return d.extractTableWithOwner(ctx, db, schemaOwner, tableName)
}

// extractTableWithOwner extracts complete information about a single table with owner.
func (d *Driver) extractTableWithOwner(ctx context.Context, db *sql.DB, owner, tableName string) (*types.Table, error) {
	table := &types.Table{
		Name:        tableName,
		Columns:     []*types.Column{},
		Indexes:     []*types.Index{},
		ForeignKeys: []*types.ForeignKey{},
	}

	// Extract columns
	columns, err := d.extractColumnsWithOwner(ctx, db, owner, tableName)
	if err != nil {
		return nil, err
	}
	for i := range columns {
		table.Columns = append(table.Columns, &columns[i])
	}

	// Extract indexes
	indexes, err := d.extractIndexesWithOwner(ctx, db, owner, tableName)
	if err != nil {
		return nil, err
	}
	for i := range indexes {
		table.Indexes = append(table.Indexes, &indexes[i])
	}

	// Extract foreign keys
	foreignKeys, err := d.extractForeignKeysWithOwner(ctx, db, owner, tableName)
	if err != nil {
		return nil, err
	}
	for i := range foreignKeys {
		table.ForeignKeys = append(table.ForeignKeys, &foreignKeys[i])
	}

	return table, nil
}

// ExtractColumns retrieves all columns for a given table.
func (d *Driver) ExtractColumns(ctx context.Context, db *sql.DB, tableName string) ([]types.Column, error) {
	var schemaOwner string
	if err := db.QueryRowContext(ctx, "SELECT USER FROM DUAL").Scan(&schemaOwner); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_columns_get_owner", err)
	}
	return d.extractColumnsWithOwner(ctx, db, schemaOwner, tableName)
}

// extractColumnsWithOwner retrieves all columns for a given table with owner.
func (d *Driver) extractColumnsWithOwner(ctx context.Context, db *sql.DB, owner, tableName string) ([]types.Column, error) {
	query := `SELECT 
		COLUMN_NAME, 
		DATA_TYPE, 
		DATA_LENGTH, 
		DATA_PRECISION, 
		DATA_SCALE, 
		NULLABLE, 
		DATA_DEFAULT, 
		COMMENTS 
	FROM ALL_TAB_COLUMNS 
	WHERE OWNER = UPPER(?) AND TABLE_NAME = UPPER(?) 
	ORDER BY COLUMN_ID`

	rows, err := db.QueryContext(ctx, query, owner, tableName)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_columns", err)
	}
	defer rows.Close()

	var columns []types.Column
	position := 1
	for rows.Next() {
		var col types.Column
		var dataLength sql.NullInt64
		var dataPrecision sql.NullInt64
		var dataScale sql.NullInt64
		var nullable string
		var defaultValue sql.NullString
		var comment sql.NullString

		if err := rows.Scan(
			&col.Name,
			&col.DataType,
			&dataLength,
			&dataPrecision,
			&dataScale,
			&nullable,
			&defaultValue,
			&comment,
		); err != nil {
			return nil, database.NewDriverError(d.Name(), "scan_column", err)
		}

		col.Position = position
		position++
		col.Nullable = nullable == "Y"

		if defaultValue.Valid {
			val := strings.TrimSpace(defaultValue.String)
			col.Default = &val
		}

		if comment.Valid {
			col.Comment = comment.String
		}

		// Set length/precision/scale based on data type
		if dataLength.Valid && dataLength.Int64 > 0 {
			length := int(dataLength.Int64)
			col.Length = &length
		}

		if dataPrecision.Valid {
			precision := int(dataPrecision.Int64)
			col.Precision = &precision
		}

		if dataScale.Valid {
			scale := int(dataScale.Int64)
			col.Scale = &scale
		}

		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError(d.Name(), "iterate_columns", err)
	}

	return columns, nil
}

// ExtractIndexes retrieves all indexes for a given table.
func (d *Driver) ExtractIndexes(ctx context.Context, db *sql.DB, tableName string) ([]types.Index, error) {
	var schemaOwner string
	if err := db.QueryRowContext(ctx, "SELECT USER FROM DUAL").Scan(&schemaOwner); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_indexes_get_owner", err)
	}
	return d.extractIndexesWithOwner(ctx, db, schemaOwner, tableName)
}

// extractIndexesWithOwner retrieves all indexes for a given table with owner.
func (d *Driver) extractIndexesWithOwner(ctx context.Context, db *sql.DB, owner, tableName string) ([]types.Index, error) {
	// Get index information from ALL_INDEXES and ALL_IND_COLUMNS
	query := `SELECT 
		i.INDEX_NAME,
		i.UNIQUENESS,
		ic.COLUMN_NAME,
		ic.COLUMN_POSITION,
		i.INDEX_TYPE
	FROM ALL_INDEXES i
	JOIN ALL_IND_COLUMNS ic ON i.INDEX_NAME = ic.INDEX_NAME AND i.OWNER = ic.INDEX_OWNER
	WHERE i.TABLE_OWNER = UPPER(?) AND i.TABLE_NAME = UPPER(?)
	ORDER BY i.INDEX_NAME, ic.COLUMN_POSITION`

	rows, err := db.QueryContext(ctx, query, owner, tableName)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_indexes", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*types.Index)
	for rows.Next() {
		var indexName string
		var uniqueness string
		var columnName string
		var columnPosition int
		var indexType string

		if err := rows.Scan(&indexName, &uniqueness, &columnName, &columnPosition, &indexType); err != nil {
			return nil, database.NewDriverError(d.Name(), "scan_index", err)
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &types.Index{
				Name:      indexName,
				Columns:   []string{columnName},
				Unique:    uniqueness == "UNIQUE",
				Primary:   strings.HasPrefix(indexName, "PK_") || strings.HasSuffix(indexName, "_PK"),
				IndexType: indexType,
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError(d.Name(), "iterate_indexes", err)
	}

	// Convert map to slice
	var indexes []types.Index
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	return indexes, nil
}

// ExtractForeignKeys retrieves all foreign keys for a given table.
func (d *Driver) ExtractForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]types.ForeignKey, error) {
	var schemaOwner string
	if err := db.QueryRowContext(ctx, "SELECT USER FROM DUAL").Scan(&schemaOwner); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_foreign_keys_get_owner", err)
	}
	return d.extractForeignKeysWithOwner(ctx, db, schemaOwner, tableName)
}

// extractForeignKeysWithOwner retrieves all foreign keys for a given table with owner.
func (d *Driver) extractForeignKeysWithOwner(ctx context.Context, db *sql.DB, owner, tableName string) ([]types.ForeignKey, error) {
	query := `SELECT 
		c.CONSTRAINT_NAME,
		cc.COLUMN_NAME,
		rcc.TABLE_NAME AS R_TABLE_NAME,
		rcc.COLUMN_NAME AS R_COLUMN_NAME,
		c.DELETE_RULE,
		c.STATUS
	FROM ALL_CONSTRAINTS c 
	JOIN ALL_CONS_COLUMNS cc ON c.CONSTRAINT_NAME = cc.CONSTRAINT_NAME AND c.OWNER = cc.OWNER
	LEFT JOIN ALL_CONS_COLUMNS rcc ON c.R_CONSTRAINT_NAME = rcc.CONSTRAINT_NAME 
		AND c.R_OWNER = rcc.OWNER 
		AND cc.POSITION = rcc.POSITION
	WHERE c.TABLE_NAME = UPPER(?) 
		AND c.CONSTRAINT_TYPE = 'R'
		AND c.OWNER = UPPER(?)
	ORDER BY c.CONSTRAINT_NAME, cc.POSITION`

	rows, err := db.QueryContext(ctx, query, tableName, owner)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_foreign_keys", err)
	}
	defer rows.Close()

	fkMap := make(map[string]*types.ForeignKey)
	for rows.Next() {
		var constraintName string
		var columnName string
		var refTableName string
		var refColumnName string
		var deleteRule sql.NullString
		var status string

		if err := rows.Scan(&constraintName, &columnName, &refTableName, &refColumnName, &deleteRule, &status); err != nil {
			return nil, database.NewDriverError(d.Name(), "scan_foreign_key", err)
		}

		if fk, exists := fkMap[constraintName]; exists {
			fk.Columns = append(fk.Columns, columnName)
			fk.RefColumns = append(fk.RefColumns, refColumnName)
		} else {
			onDelete := ""
			if deleteRule.Valid && deleteRule.String != "NO ACTION" {
				onDelete = deleteRule.String
			}

			fkMap[constraintName] = &types.ForeignKey{
				Name:       constraintName,
				Columns:    []string{columnName},
				RefTable:   refTableName,
				RefColumns: []string{refColumnName},
				OnDelete:   onDelete,
				OnUpdate:   "", // Oracle doesn't support ON UPDATE
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError(d.Name(), "iterate_foreign_keys", err)
	}

	// Convert map to slice
	var foreignKeys []types.ForeignKey
	for _, fk := range fkMap {
		foreignKeys = append(foreignKeys, *fk)
	}

	return foreignKeys, nil
}
