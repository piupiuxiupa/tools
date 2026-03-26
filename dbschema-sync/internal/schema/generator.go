// Package schema provides database schema DDL generation functionality.
package schema

import (
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/pkg/types"
)

// DDLGenerator defines the interface for generating database DDL statements.
// Implementations are database-specific and handle the particularities of each
// database system's SQL syntax.
type DDLGenerator interface {
	// GenerateCreateTable generates a CREATE TABLE statement.
	GenerateCreateTable(table *types.Table) (string, error)

	// GenerateDropTable generates a DROP TABLE statement.
	GenerateDropTable(tableName string) (string, error)

	// GenerateAlterTable generates ALTER TABLE statements for table modifications.
	GenerateAlterTable(diff *types.TableDiff) ([]string, error)

	// GenerateAddColumn generates an ADD COLUMN statement.
	GenerateAddColumn(tableName string, column types.Column) (string, error)

	// GenerateDropColumn generates a DROP COLUMN statement.
	GenerateDropColumn(tableName, columnName string) (string, error)

	// GenerateModifyColumn generates a MODIFY COLUMN statement.
	GenerateModifyColumn(tableName string, column types.Column) (string, error)

	// GenerateCreateIndex generates a CREATE INDEX statement.
	GenerateCreateIndex(tableName string, index types.Index) (string, error)

	// GenerateDropIndex generates a DROP INDEX statement.
	GenerateDropIndex(tableName, indexName string) (string, error)

	// GenerateAddForeignKey generates an ADD FOREIGN KEY statement.
	GenerateAddForeignKey(tableName string, fk types.ForeignKey) (string, error)

	// GenerateDropForeignKey generates a DROP FOREIGN KEY/CONSTRAINT statement.
	GenerateDropForeignKey(tableName, fkName string) (string, error)

	// GenerateDiff generates DDL statements from a schema diff.
	GenerateDiff(diff *types.SchemaDiff) ([]string, error)
}

// GeneratorType represents the type of DDL generator.
type GeneratorType string

const (
	// MySQLGeneratorType is the generator type for MySQL.
	MySQLGeneratorType GeneratorType = "mysql"
	// PostgreSQLGeneratorType is the generator type for PostgreSQL.
	PostgreSQLGeneratorType GeneratorType = "postgresql"
	// OracleGeneratorType is the generator type for Oracle.
	OracleGeneratorType GeneratorType = "oracle"
	// DorisGeneratorType is the generator type for Doris.
	DorisGeneratorType GeneratorType = "doris"
)

// NewGenerator creates a new DDL generator for the specified dialect.
// The typeMapper is used for mapping data types between different database systems.
func NewGenerator(dialect string, typeMapper types.TypeMapper) (DDLGenerator, error) {
	switch strings.ToLower(dialect) {
	case "mysql":
		return NewMySQLGenerator(typeMapper), nil
	case "postgresql", "postgres":
		return NewPostgreSQLGenerator(typeMapper), nil
	case "oracle":
		return NewOracleGenerator(typeMapper), nil
	case "doris":
		return NewDorisGenerator(typeMapper), nil
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

// baseGenerator provides common functionality for all DDL generators.
type baseGenerator struct {
	typeMapper types.TypeMapper
	dialect    string
}

// quoteIdentifier quotes an identifier based on the dialect.
func (g *baseGenerator) quoteIdentifier(name string) string {
	switch g.dialect {
	case "mysql", "doris":
		return fmt.Sprintf("`%s`", name)
	case "postgresql", "oracle":
		return fmt.Sprintf("\"%s\"", name)
	default:
		return name
	}
}

// formatDefaultValue formats a default value for SQL.
func (g *baseGenerator) formatDefaultValue(value *string) string {
	if value == nil {
		return ""
	}
	v := *value
	// Check if it's a SQL function/expression
	upperValue := strings.ToUpper(v)
	if strings.Contains(upperValue, "(") ||
		upperValue == "CURRENT_TIMESTAMP" ||
		upperValue == "NOW()" ||
		upperValue == "NULL" ||
		upperValue == "TRUE" ||
		upperValue == "FALSE" {
		return v
	}
	return fmt.Sprintf("'%s'", v)
}

// buildColumnDefinition builds the column definition part of a CREATE TABLE statement.
func (g *baseGenerator) buildColumnDefinition(col *types.Column) string {
	var parts []string

	// Column name and type
	parts = append(parts, g.quoteIdentifier(col.Name))

	// Data type with size/precision if applicable
	dataType := g.mapDataType(col.DataType)
	if col.Length != nil && *col.Length > 0 {
		dataType = fmt.Sprintf("%s(%d)", dataType, *col.Length)
	} else if isDecimalType(dataType) && col.Precision != nil && *col.Precision > 0 {
		// Only DECIMAL/NUMERIC types support (precision,scale)
		if col.Scale != nil && *col.Scale >= 0 {
			dataType = fmt.Sprintf("%s(%d,%d)", dataType, *col.Precision, *col.Scale)
		} else {
			dataType = fmt.Sprintf("%s(%d)", dataType, *col.Precision)
		}
	}
	parts = append(parts, dataType)

	return strings.Join(parts, " ")
}

// mapDataType maps a data type to the target dialect.
func (g *baseGenerator) mapDataType(sourceType string) string {
	if g.typeMapper == nil {
		return sourceType
	}
	// For same dialect, return as-is
	targetType, err := g.typeMapper.MapType(sourceType, g.dialect, g.dialect)
	if err != nil {
		return sourceType
	}
	return targetType
}

// isDecimalType checks if a type is DECIMAL or NUMERIC (supports precision/scale).
func isDecimalType(dataType string) bool {
	lowerType := strings.ToLower(dataType)
	return strings.HasPrefix(lowerType, "decimal") || strings.HasPrefix(lowerType, "numeric")
}
