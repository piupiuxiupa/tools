package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lush/dbschema-sync/internal/database"
	"github.com/lush/dbschema-sync/pkg/types"
)

type Driver struct {
	database.BaseDriver
}

func init() {
	d := &Driver{}
	d.BaseDriver = database.BaseDriver{}
	if err := database.Register("mysql", d); err != nil {
		panic(fmt.Sprintf("failed to register MySQL driver: %v", err))
	}
}

func (d *Driver) Name() string {
	return "mysql"
}

func (d *Driver) Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, database.NewDriverError("mysql", "connect", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, database.NewDriverError("mysql", "ping", err)
	}

	return db, nil
}

func (d *Driver) Ping(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return database.NewDriverError("mysql", "ping", err)
	}
	return nil
}

func (d *Driver) QuoteIdentifier(name string) string {
	return fmt.Sprintf("`%s`", strings.ReplaceAll(name, "`", "``"))
}

func (d *Driver) QuoteLiteral(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "''"))
}

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
			return nil, err
		}
		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func (d *Driver) ExtractTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE()`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, database.NewDriverError("mysql", "extract_tables", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, database.NewDriverError("mysql", "scan_table_name", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("mysql", "iterate_tables", err)
	}

	return tables, nil
}

func (d *Driver) ExtractTable(ctx context.Context, db *sql.DB, tableName string) (*types.Table, error) {
	columns, err := d.ExtractColumns(ctx, db, tableName)
	if err != nil {
		return nil, err
	}

	indexes, err := d.ExtractIndexes(ctx, db, tableName)
	if err != nil {
		return nil, err
	}

	foreignKeys, err := d.ExtractForeignKeys(ctx, db, tableName)
	if err != nil {
		return nil, err
	}

	return &types.Table{
		Name:        tableName,
		Columns:     columns,
		Indexes:     indexes,
		ForeignKeys: foreignKeys,
	}, nil
}

func (d *Driver) ExtractColumns(ctx context.Context, db *sql.DB, tableName string) ([]*types.Column, error) {
	query := `SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT,
		CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, EXTRA,
		COLUMN_COMMENT, ORDINAL_POSITION, COLUMN_TYPE
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	ORDER BY ORDINAL_POSITION`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError("mysql", "extract_columns", err)
	}
	defer rows.Close()

	var columns []*types.Column
	for rows.Next() {
		var col types.Column
		var maxLength, numPrecision, numScale sql.NullInt64
		var defaultValue, extra, comment sql.NullString
		var isNullable string
		var ordinalPos int
		var columnType sql.NullString

		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&isNullable,
			&defaultValue,
			&maxLength,
			&numPrecision,
			&numScale,
			&extra,
			&comment,
			&ordinalPos,
			&columnType,
		)
		if err != nil {
			return nil, database.NewDriverError("mysql", "scan_column", err)
		}

		if columnType.Valid && strings.Contains(columnType.String, "(") {
			lowerDataType := strings.ToLower(col.DataType)
			if lowerDataType == "enum" || lowerDataType == "set" {
				col.DataType = columnType.String
			}
		}

		col.Nullable = isNullable == "YES"

		if defaultValue.Valid {
			col.Default = &defaultValue.String
		}

		if maxLength.Valid {
			length := int(maxLength.Int64)
			col.Length = &length
		}

		if numPrecision.Valid {
			precision := int(numPrecision.Int64)
			col.Precision = &precision
		}

		if numScale.Valid {
			scale := int(numScale.Int64)
			col.Scale = &scale
		}

		if extra.Valid && strings.Contains(extra.String, "auto_increment") {
			col.AutoIncrement = true
		}

		if comment.Valid {
			col.Comment = comment.String
		}

		col.Position = ordinalPos

		columns = append(columns, &col)
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("mysql", "iterate_columns", err)
	}

	return columns, nil
}

func (d *Driver) ExtractIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*types.Index, error) {
	query := `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE, SEQ_IN_INDEX 
	FROM INFORMATION_SCHEMA.STATISTICS 
	WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError("mysql", "extract_indexes", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*types.Index)

	for rows.Next() {
		var indexName, columnName string
		var nonUnique int
		var seqInIndex int

		if err := rows.Scan(&indexName, &columnName, &nonUnique, &seqInIndex); err != nil {
			return nil, database.NewDriverError("mysql", "scan_index", err)
		}

		if idx, exists := indexMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			indexMap[indexName] = &types.Index{
				Name:    indexName,
				Columns: []string{columnName},
				Unique:  nonUnique == 0,
				Primary: indexName == "PRIMARY",
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("mysql", "iterate_indexes", err)
	}

	indexes := make([]*types.Index, 0, len(indexMap))
	for _, idx := range indexMap {
		indexes = append(indexes, idx)
	}

	return indexes, nil
}

func (d *Driver) ExtractForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]*types.ForeignKey, error) {
	query := `SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME 
	FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE 
	WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
	ORDER BY CONSTRAINT_NAME, ORDINAL_POSITION`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, database.NewDriverError("mysql", "extract_foreign_keys", err)
	}
	defer rows.Close()

	fkMap := make(map[string]*types.ForeignKey)

	for rows.Next() {
		var constraintName, columnName, refTable, refColumn string

		if err := rows.Scan(&constraintName, &columnName, &refTable, &refColumn); err != nil {
			return nil, database.NewDriverError("mysql", "scan_foreign_key", err)
		}

		if fk, exists := fkMap[constraintName]; exists {
			fk.Columns = append(fk.Columns, columnName)
			fk.RefColumns = append(fk.RefColumns, refColumn)
		} else {
			fkMap[constraintName] = &types.ForeignKey{
				Name:       constraintName,
				Columns:    []string{columnName},
				RefTable:   refTable,
				RefColumns: []string{refColumn},
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, database.NewDriverError("mysql", "iterate_foreign_keys", err)
	}

	foreignKeys := make([]*types.ForeignKey, 0, len(fkMap))
	for _, fk := range fkMap {
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, nil
}
