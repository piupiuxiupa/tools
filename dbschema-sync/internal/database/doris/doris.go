// Package doris provides a database driver and schema extractor for Apache Doris.
// Doris is compatible with MySQL protocol and uses MySQL driver internally.
package doris

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/lush/dbschema-sync/internal/database"
	"github.com/lush/dbschema-sync/internal/schema/extractor"
	"github.com/lush/dbschema-sync/pkg/types"
)

func init() {
	database.Register("doris", &Driver{})
}

// Driver implements the database.Driver and extractor.SchemaExtractor interfaces for Doris.
type Driver struct {
	database.BaseDriver
}

// NewDriver creates a new Doris driver instance.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: database.BaseDriver{},
	}
}

// Name returns the driver name.
func (d *Driver) Name() string {
	return "doris"
}

// Connect establishes a connection to the Doris database.
func (d *Driver) Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, database.NewDriverError("doris", "connect", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, database.NewDriverError("doris", "ping", err)
	}

	return db, nil
}

// Ping verifies the connection to the database is still alive.
func (d *Driver) Ping(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return database.NewDriverError("doris", "ping", err)
	}
	return nil
}

// QuoteIdentifier returns the identifier properly quoted for Doris.
func (d *Driver) QuoteIdentifier(name string) string {
	return fmt.Sprintf("`%s`", strings.ReplaceAll(name, "`", "``"))
}

// QuoteLiteral returns a literal value properly quoted for Doris.
func (d *Driver) QuoteLiteral(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "\\'"))
}

// Ensure Driver implements both interfaces.
var _ database.Driver = (*Driver)(nil)
var _ extractor.SchemaExtractor = (*Driver)(nil)

// ExtractSchema extracts the complete database schema.
func (d *Driver) ExtractSchema(ctx context.Context, db *sql.DB) (*types.DatabaseSchema, error) {
	tables, err := d.ExtractTables(ctx, db)
	if err != nil {
		return nil, err
	}

	schema := &types.DatabaseSchema{
		Tables: make([]*types.Table, 0, len(tables)),
	}

	for _, tableName := range tables {
		table, err := d.ExtractTable(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to extract table %s: %w", tableName, err)
		}
		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

// ExtractTables retrieves a list of all table names in the current database.
func (d *Driver) ExtractTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE()"

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, database.NewDriverError("doris", "extract_tables", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, database.NewDriverError("doris", "scan_table", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("doris", "iterate_tables", err)
	}

	return tables, nil
}

// ExtractTable extracts complete information about a specific table.
func (d *Driver) ExtractTable(ctx context.Context, db *sql.DB, tableName string) (*types.Table, error) {
	table := &types.Table{
		Name: tableName,
	}

	columns, err := d.ExtractColumns(ctx, db, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract columns: %w", err)
	}
	table.Columns = columns

	indexes, err := d.ExtractIndexes(ctx, db, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract indexes: %w", err)
	}
	table.Indexes = indexes

	foreignKeys, err := d.ExtractForeignKeys(ctx, db, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract foreign keys: %w", err)
	}
	table.ForeignKeys = foreignKeys

	return table, nil
}

// ExtractColumns retrieves column information for a specific table.
func (d *Driver) ExtractColumns(ctx context.Context, db *sql.DB, tableName string) ([]*types.Column, error) {
	query := `
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_COMMENT,
			ORDINAL_POSITION,
			CHARACTER_MAXIMUM_LENGTH,
			NUMERIC_PRECISION,
			NUMERIC_SCALE,
			EXTRA,
			COLUMN_KEY
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError("doris", "extract_columns", err)
	}
	defer rows.Close()

	var columns []*types.Column
	for rows.Next() {
		var col types.Column
		var isNullable, extra, columnKey string
		var charMaxLength, numericPrecision, numericScale sql.NullInt64
		var columnDefault sql.NullString
		var columnComment sql.NullString

		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&isNullable,
			&columnDefault,
			&columnComment,
			&col.Position,
			&charMaxLength,
			&numericPrecision,
			&numericScale,
			&extra,
			&columnKey,
		)
		if err != nil {
			return nil, database.NewDriverError("doris", "scan_column", err)
		}

		col.Nullable = isNullable == "YES"

		if columnDefault.Valid {
			col.Default = &columnDefault.String
		}

		if columnComment.Valid {
			col.Comment = columnComment.String
		}

		if charMaxLength.Valid {
			length := int(charMaxLength.Int64)
			col.Length = &length
		}

		if numericPrecision.Valid {
			precision := int(numericPrecision.Int64)
			col.Precision = &precision
		}

		if numericScale.Valid {
			scale := int(numericScale.Int64)
			col.Scale = &scale
		}

		col.AutoIncrement = strings.Contains(extra, "auto_increment")

		columns = append(columns, &col)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("doris", "iterate_columns", err)
	}

	return columns, nil
}

// ExtractIndexes retrieves index information for a specific table.
func (d *Driver) ExtractIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*types.Index, error) {
	query := `
		SELECT 
			INDEX_NAME,
			COLUMN_NAME,
			NON_UNIQUE,
			INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError("doris", "extract_indexes", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*types.Index)
	for rows.Next() {
		var indexName, columnName, indexType string
		var nonUnique int

		err := rows.Scan(&indexName, &columnName, &nonUnique, &indexType)
		if err != nil {
			return nil, database.NewDriverError("doris", "scan_index", err)
		}

		idx, exists := indexMap[indexName]
		if !exists {
			idx = &types.Index{
				Name:      indexName,
				Columns:   make([]string, 0),
				Unique:    nonUnique == 0,
				Primary:   indexName == "PRIMARY",
				IndexType: indexType,
			}
			indexMap[indexName] = idx
		}
		idx.Columns = append(idx.Columns, columnName)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("doris", "iterate_indexes", err)
	}

	indexes := make([]*types.Index, 0, len(indexMap))
	for _, idx := range indexMap {
		indexes = append(indexes, idx)
	}

	return indexes, nil
}

// ExtractForeignKeys retrieves foreign key information for a specific table.
func (d *Driver) ExtractForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]*types.ForeignKey, error) {
	query := `
		SELECT 
			CONSTRAINT_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE 
		WHERE TABLE_SCHEMA = DATABASE() 
			AND TABLE_NAME = ? 
			AND REFERENCED_TABLE_NAME IS NOT NULL
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError("doris", "extract_foreign_keys", err)
	}
	defer rows.Close()

	fkMap := make(map[string]*types.ForeignKey)
	for rows.Next() {
		var constraintName, columnName, refTable, refColumn string

		err := rows.Scan(&constraintName, &columnName, &refTable, &refColumn)
		if err != nil {
			return nil, database.NewDriverError("doris", "scan_foreign_key", err)
		}

		fk, exists := fkMap[constraintName]
		if !exists {
			fk = &types.ForeignKey{
				Name:       constraintName,
				Columns:    make([]string, 0),
				RefTable:   refTable,
				RefColumns: make([]string, 0),
			}
			fkMap[constraintName] = fk
		}
		fk.Columns = append(fk.Columns, columnName)
		fk.RefColumns = append(fk.RefColumns, refColumn)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("doris", "iterate_foreign_keys", err)
	}

	foreignKeys := make([]*types.ForeignKey, 0, len(fkMap))
	for _, fk := range fkMap {
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}

// ParseShowCreateTable parses the output of SHOW CREATE TABLE to extract Doris-specific properties.
func (d *Driver) ParseShowCreateTable(ctx context.Context, db *sql.DB, tableName string) (string, map[string]string, error) {
	query := fmt.Sprintf("SHOW CREATE TABLE %s", d.QuoteIdentifier(tableName))

	var name, createStmt string
	err := db.QueryRowContext(ctx, query).Scan(&name, &createStmt)
	if err != nil {
		return "", nil, database.NewDriverError("doris", "show_create_table", err)
	}

	properties := make(map[string]string)

	re := regexp.MustCompile(`"([^"]+)"\s*=\s*"([^"]+)"`)
	matches := re.FindAllStringSubmatch(createStmt, -1)
	for _, match := range matches {
		if len(match) == 3 {
			properties[match[1]] = match[2]
		}
	}

	return createStmt, properties, nil
}

// IsDorisType checks if a data type is Doris-specific.
func (d *Driver) IsDorisType(dataType string) bool {
	dorisTypes := []string{"HLL", "BITMAP", "QUANTILE_STATE"}
	upperType := strings.ToUpper(dataType)
	for _, t := range dorisTypes {
		if upperType == t {
			return true
		}
	}
	return false
}

// NormalizeType normalizes a Doris data type for compatibility.
func (d *Driver) NormalizeType(dataType string) string {
	upperType := strings.ToUpper(dataType)

	switch upperType {
	case "STRING":
		return "VARCHAR"
	case "LARGEINT":
		return "BIGINT"
	default:
		return dataType
	}
}

// intPtr returns a pointer to an int value.
func intPtr(v int) *int {
	return &v
}

// ParseColumnType parses a column type string and extracts precision and scale.
func (d *Driver) ParseColumnType(typeStr string) (dataType string, length, precision, scale *int) {
	dataType = typeStr

	re := regexp.MustCompile(`^(\w+)\s*\(\s*(\d+)\s*(?:,\s*(\d+)\s*)?\)`)
	matches := re.FindStringSubmatch(typeStr)

	if len(matches) > 1 {
		dataType = matches[1]
		if len(matches) > 2 {
			if l, err := strconv.Atoi(matches[2]); err == nil {
				length = intPtr(l)
			}
		}
		if len(matches) > 3 && matches[3] != "" {
			if s, err := strconv.Atoi(matches[3]); err == nil {
				scale = intPtr(s)
			}
		}
	}

	return dataType, length, precision, scale
}
