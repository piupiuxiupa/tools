// Package postgres provides PostgreSQL driver implementation for database schema extraction.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/lush/dbschema-sync/internal/database"
	"github.com/lush/dbschema-sync/pkg/types"
)

func init() {
	postgresDriver := &Driver{}
	if err := database.Register("postgres", postgresDriver); err != nil {
		panic(fmt.Sprintf("failed to register postgres driver: %v", err))
	}
}

// Driver implements the database.Driver and SchemaExtractor interfaces for PostgreSQL.
type Driver struct {
	base database.BaseDriver
}

// Name returns the driver's name.
func (d *Driver) Name() string {
	return "postgres"
}

// Connect establishes a connection to the PostgreSQL database.
func (d *Driver) Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
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

// QuoteIdentifier returns the identifier properly quoted for PostgreSQL.
func (d *Driver) QuoteIdentifier(name string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(name, `"`, `""`))
}

// QuoteLiteral returns a literal value properly quoted for PostgreSQL.
func (d *Driver) QuoteLiteral(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "''"))
}

// ExtractSchema extracts the complete database schema.
func (d *Driver) ExtractSchema(ctx context.Context, db *sql.DB) (*types.DatabaseSchema, error) {
	schema := &types.DatabaseSchema{
		Tables: []*types.Table{},
	}

	// Get database name
	var dbName string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_schema_get_dbname", err)
	}
	schema.Name = dbName

	// Get all tables
	tables, err := d.ExtractTables(ctx, db)
	if err != nil {
		return nil, err
	}

	// Extract schema for each table
	for _, tableName := range tables {
		table, err := d.ExtractTable(ctx, db, tableName)
		if err != nil {
			return nil, err
		}
		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

// ExtractTables retrieves all table names from the public schema.
func (d *Driver) ExtractTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`

	rows, err := db.QueryContext(ctx, query)
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
	table := &types.Table{
		Name:        tableName,
		Columns:     []*types.Column{},
		Indexes:     []*types.Index{},
		ForeignKeys: []*types.ForeignKey{},
	}

	// Extract columns
	columns, err := d.ExtractColumns(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	for i := range columns {
		table.Columns = append(table.Columns, &columns[i])
	}

	// Extract indexes
	indexes, err := d.ExtractIndexes(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	for i := range indexes {
		table.Indexes = append(table.Indexes, &indexes[i])
	}

	// Extract foreign keys
	foreignKeys, err := d.ExtractForeignKeys(ctx, db, tableName)
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
	query := `
		SELECT 
			a.attname as column_name,
			pg_catalog.format_type(a.atttypid, a.atttypmod) as data_type,
			NOT a.attnotnull as is_nullable,
			pg_get_expr(d.adbin, d.adrelid) as column_default,
			a.attnum as position
		FROM pg_catalog.pg_attribute a
		LEFT JOIN pg_catalog.pg_attrdef d ON (a.attrelid = d.adrelid AND a.attnum = d.adnum)
		WHERE a.attrelid = $1::regclass
			AND a.attnum > 0
			AND NOT a.attisdropped
		ORDER BY a.attnum
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_columns", err)
	}
	defer rows.Close()

	var columns []types.Column
	for rows.Next() {
		var col types.Column
		var defaultValue sql.NullString

		if err := rows.Scan(
			&col.Name,
			&col.DataType,
			&col.Nullable,
			&defaultValue,
			&col.Position,
		); err != nil {
			return nil, database.NewDriverError(d.Name(), "scan_column", err)
		}

		if defaultValue.Valid {
			col.Default = &defaultValue.String
			// Check for auto-increment
			if strings.Contains(defaultValue.String, "nextval") {
				col.AutoIncrement = true
			}
		}

		// Parse data type to extract length/precision/scale
		d.parseDataType(&col)

		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError(d.Name(), "iterate_columns", err)
	}

	return columns, nil
}

// parseDataType parses PostgreSQL data type to extract length, precision, and scale.
func (d *Driver) parseDataType(col *types.Column) {
	// Pattern to match type with length/precision/scale
	// Examples: varchar(255), numeric(10,2), character varying(100)
	re := regexp.MustCompile(`^(.*?)(?:\((\d+)(?:,(\d+))?\))?$`)
	matches := re.FindStringSubmatch(col.DataType)

	if len(matches) >= 3 && matches[2] != "" {
		length := 0
		fmt.Sscanf(matches[2], "%d", &length)

		if len(matches) >= 4 && matches[3] != "" {
			// Has precision and scale (e.g., numeric(10,2))
			precision := length
			col.Precision = &precision
			scale := 0
			fmt.Sscanf(matches[3], "%d", &scale)
			col.Scale = &scale
		} else {
			// Has only length (e.g., varchar(255))
			col.Length = &length
		}
	}
}

// ExtractIndexes retrieves all indexes for a given table.
func (d *Driver) ExtractIndexes(ctx context.Context, db *sql.DB, tableName string) ([]types.Index, error) {
	query := `SELECT indexname, indexdef FROM pg_indexes WHERE tablename = $1`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_indexes", err)
	}
	defer rows.Close()

	var indexes []types.Index
	for rows.Next() {
		var idx types.Index
		var indexDef string

		if err := rows.Scan(&idx.Name, &indexDef); err != nil {
			return nil, database.NewDriverError(d.Name(), "scan_index", err)
		}

		// Parse index definition to extract columns and properties
		d.parseIndexDefinition(&idx, indexDef, tableName)

		indexes = append(indexes, idx)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError(d.Name(), "iterate_indexes", err)
	}

	return indexes, nil
}

// parseIndexDefinition parses PostgreSQL index definition to extract columns and properties.
func (d *Driver) parseIndexDefinition(idx *types.Index, indexDef, tableName string) {
	// Check if it's a unique index
	if strings.Contains(strings.ToUpper(indexDef), "UNIQUE") {
		idx.Unique = true
	}

	// Check if it's a primary key
	if strings.Contains(strings.ToUpper(indexDef), "PRIMARY KEY") {
		idx.Primary = true
		idx.Unique = true
	}

	// Extract index type (USING clause)
	if matches := regexp.MustCompile(`USING\s+(\w+)`).FindStringSubmatch(indexDef); len(matches) > 1 {
		idx.IndexType = strings.ToUpper(matches[1])
	} else {
		idx.IndexType = "BTREE" // Default
	}

	// Extract columns - pattern: ON tablename (col1, col2, ...)
	columnPattern := regexp.MustCompile(`ON\s+` + regexp.QuoteMeta(tableName) + `\s*\(([^)]+)\)`)
	if matches := columnPattern.FindStringSubmatch(indexDef); len(matches) > 1 {
		columnList := matches[1]
		// Split by comma and clean up
		cols := strings.Split(columnList, ",")
		for _, col := range cols {
			col = strings.TrimSpace(col)
			// Remove sort order (ASC/DESC)
			col = regexp.MustCompile(`\s+(ASC|DESC)$`).ReplaceAllString(col, "")
			// Remove expression/function calls - just extract the column name
			if !strings.Contains(col, "(") {
				idx.Columns = append(idx.Columns, col)
			}
		}
	}
}

// ExtractForeignKeys retrieves all foreign keys for a given table.
func (d *Driver) ExtractForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]types.ForeignKey, error) {
	query := `
		SELECT 
			conname,
			contype,
			pg_get_constraintdef(oid) as condef
		FROM pg_constraint
		WHERE conrelid = $1::regclass
			AND contype = 'f'
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError(d.Name(), "extract_foreign_keys", err)
	}
	defer rows.Close()

	var foreignKeys []types.ForeignKey
	for rows.Next() {
		var fk types.ForeignKey
		var conType string
		var conDef string

		if err := rows.Scan(&fk.Name, &conType, &conDef); err != nil {
			return nil, database.NewDriverError(d.Name(), "scan_foreign_key", err)
		}

		// Parse the constraint definition
		// Format: FOREIGN KEY (col1, col2) REFERENCES reftable(refcol1, refcol2) ON DELETE action ON UPDATE action
		d.parseForeignKeyDefinition(&fk, conDef)

		foreignKeys = append(foreignKeys, fk)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError(d.Name(), "iterate_foreign_keys", err)
	}

	return foreignKeys, nil
}

// parseForeignKeyDefinition parses PostgreSQL foreign key constraint definition.
func (d *Driver) parseForeignKeyDefinition(fk *types.ForeignKey, conDef string) {
	// Extract local columns: FOREIGN KEY (col1, col2)
	columnPattern := regexp.MustCompile(`FOREIGN\s+KEY\s*\(([^)]+)\)`)
	if matches := columnPattern.FindStringSubmatch(conDef); len(matches) > 1 {
		cols := strings.Split(matches[1], ",")
		for _, col := range cols {
			fk.Columns = append(fk.Columns, strings.TrimSpace(col))
		}
	}

	// Extract reference: REFERENCES tablename(col1, col2)
	refPattern := regexp.MustCompile(`REFERENCES\s+(\w+)\s*\(([^)]+)\)`)
	if matches := refPattern.FindStringSubmatch(conDef); len(matches) > 2 {
		fk.RefTable = matches[1]
		refCols := strings.Split(matches[2], ",")
		for _, col := range refCols {
			fk.RefColumns = append(fk.RefColumns, strings.TrimSpace(col))
		}
	}

	// Extract ON DELETE action
	if matches := regexp.MustCompile(`ON\s+DELETE\s+(\w+(?:\s+\w+)?)`).FindStringSubmatch(conDef); len(matches) > 1 {
		fk.OnDelete = strings.ToUpper(matches[1])
	}

	// Extract ON UPDATE action
	if matches := regexp.MustCompile(`ON\s+UPDATE\s+(\w+(?:\s+\w+)?)`).FindStringSubmatch(conDef); len(matches) > 1 {
		fk.OnUpdate = strings.ToUpper(matches[1])
	}
}
