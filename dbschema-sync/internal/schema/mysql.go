package schema

import (
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/pkg/types"
)

// MySQLGenerator generates DDL statements for MySQL.
type MySQLGenerator struct {
	baseGenerator
}

// NewMySQLGenerator creates a new MySQL DDL generator.
func NewMySQLGenerator(typeMapper types.TypeMapper) *MySQLGenerator {
	return &MySQLGenerator{
		baseGenerator: baseGenerator{
			typeMapper: typeMapper,
			dialect:    "mysql",
		},
	}
}

// GenerateCreateTable generates a CREATE TABLE statement for MySQL.
func (g *MySQLGenerator) GenerateCreateTable(table *types.Table) (string, error) {
	if table == nil {
		return "", fmt.Errorf("table is nil")
	}
	if table.Name == "" {
		return "", fmt.Errorf("table name is empty")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", g.quoteIdentifier(table.Name)))

	// Columns
	for i, col := range table.Columns {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("  ")
		sb.WriteString(g.generateColumnDefinition(col))
	}

	// Primary key (if exists and not auto-generated)
	if pk := table.GetPrimaryKey(); pk != nil {
		sb.WriteString(",\n")
		sb.WriteString(fmt.Sprintf("  PRIMARY KEY (%s)", g.quoteColumnList(pk.Columns)))
	}

	// Unique indexes
	for _, idx := range table.Indexes {
		if idx.Unique && !idx.Primary {
			sb.WriteString(",\n")
			sb.WriteString(fmt.Sprintf("  UNIQUE KEY %s (%s)",
				g.quoteIdentifier(idx.Name),
				g.quoteColumnList(idx.Columns)))
		}
	}

	sb.WriteString("\n)")

	// Table options
	if table.Engine != "" {
		sb.WriteString(fmt.Sprintf(" ENGINE=%s", table.Engine))
	} else {
		sb.WriteString(" ENGINE=InnoDB")
	}

	if table.Charset != "" {
		sb.WriteString(fmt.Sprintf(" DEFAULT CHARSET=%s", table.Charset))
	} else {
		sb.WriteString(" DEFAULT CHARSET=utf8mb4")
	}

	if table.Collation != "" {
		sb.WriteString(fmt.Sprintf(" COLLATE=%s", table.Collation))
	}

	sb.WriteString(";")

	return sb.String(), nil
}

// generateColumnDefinition generates a column definition for MySQL.
func (g *MySQLGenerator) generateColumnDefinition(col *types.Column) string {
	var parts []string

	// Column name
	parts = append(parts, g.quoteIdentifier(col.Name))

	// Data type
	dataType := g.mapMySQLType(col)
	parts = append(parts, dataType)

	// UNSIGNED (for numeric types)
	if col.Unsigned && isMySQLNumericType(col.DataType) {
		parts = append(parts, "UNSIGNED")
	}

	// NULL/NOT NULL
	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	} else {
		parts = append(parts, "NULL")
	}

	// DEFAULT
	if col.Default != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", g.formatDefaultValue(col.Default)))
	}

	// AUTO_INCREMENT
	if col.AutoIncrement {
		parts = append(parts, "AUTO_INCREMENT")
	}

	// COMMENT
	if col.Comment != "" {
		parts = append(parts, fmt.Sprintf("COMMENT '%s'", escapeMySQLString(col.Comment)))
	}

	return strings.Join(parts, " ")
}

// mapMySQLType maps a column type to MySQL-specific syntax.
func (g *MySQLGenerator) mapMySQLType(col *types.Column) string {
	dataType := strings.ToLower(col.DataType)

	// Handle specific MySQL types
	switch dataType {
	case "serial":
		return "INT AUTO_INCREMENT"
	case "bigserial":
		return "BIGINT AUTO_INCREMENT"
	}

	// Map through type mapper if available
	if g.typeMapper != nil {
		if mapped, err := g.typeMapper.MapType(col.DataType, "mysql", "mysql"); err == nil {
			dataType = mapped
		}
	}

	// Add size/precision - only for specific types
	if col.Length != nil && *col.Length > 0 && supportsLength(dataType) {
		// Check if type supports length
		if strings.Contains(dataType, "(") {
			// Replace existing size
			if idx := strings.Index(dataType, "("); idx != -1 {
				dataType = dataType[:idx]
			}
		}
		dataType = fmt.Sprintf("%s(%d)", dataType, *col.Length)
	} else if isDecimalType(dataType) && col.Precision != nil && *col.Precision > 0 {
		// Only DECIMAL/NUMERIC types support (precision,scale) syntax in MySQL
		// Integer types like INT should not have precision/scale applied
		if col.Scale != nil && *col.Scale >= 0 {
			dataType = fmt.Sprintf("%s(%d,%d)", dataType, *col.Precision, *col.Scale)
		} else {
			dataType = fmt.Sprintf("%s(%d)", dataType, *col.Precision)
		}
	}

	if strings.HasPrefix(dataType, "enum(") || strings.HasPrefix(dataType, "set(") {
		return col.DataType
	}

	return strings.ToUpper(dataType)
}

// isMySQLNumericType checks if a type is a numeric type in MySQL.
func isMySQLNumericType(dataType string) bool {
	numericTypes := []string{"tinyint", "smallint", "mediumint", "int", "integer", "bigint", "float", "double", "decimal", "numeric"}
	lowerType := strings.ToLower(dataType)
	for _, nt := range numericTypes {
		if strings.HasPrefix(lowerType, nt) {
			return true
		}
	}
	return false
}

// supportsLength checks if a MySQL type supports length specification.
// Types like ENUM, SET, TEXT, BLOB, DATE, JSON do not support length.
func supportsLength(dataType string) bool {
	lowerType := strings.ToLower(dataType)
	// Types that do NOT support length
	noLengthTypes := []string{
		"enum", "set", "date", "time", "timestamp", "datetime", "year",
		"tinytext", "text", "mediumtext", "longtext",
		"tinyblob", "blob", "mediumblob", "longblob",
		"json", "geometry", "point", "linestring", "polygon",
	}
	for _, t := range noLengthTypes {
		if strings.HasPrefix(lowerType, t) {
			return false
		}
	}
	return true
}

// GenerateDropTable generates a DROP TABLE statement for MySQL.
func (g *MySQLGenerator) GenerateDropTable(tableName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", g.quoteIdentifier(tableName)), nil
}

// GenerateAlterTable generates ALTER TABLE statements for MySQL.
func (g *MySQLGenerator) GenerateAlterTable(diff *types.TableDiff) ([]string, error) {
	if diff == nil {
		return nil, fmt.Errorf("table diff is nil")
	}

	var statements []string
	tableName := g.quoteIdentifier(diff.TableName)

	// Add columns
	for _, col := range diff.ColumnsAdded {
		stmt, err := g.GenerateAddColumn(diff.TableName, *col)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Drop columns
	for _, col := range diff.ColumnsRemoved {
		stmt, err := g.GenerateDropColumn(diff.TableName, col.Name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Modify columns
	for _, colDiff := range diff.ColumnsModified {
		if colDiff.NewColumn != nil {
			stmt, err := g.GenerateModifyColumn(diff.TableName, *colDiff.NewColumn)
			if err != nil {
				return nil, err
			}
			statements = append(statements, stmt)
		}
	}

	// Add indexes
	for _, idx := range diff.IndexesAdded {
		stmt, err := g.GenerateCreateIndex(diff.TableName, *idx)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Drop indexes
	for _, idx := range diff.IndexesRemoved {
		stmt, err := g.GenerateDropIndex(diff.TableName, idx.Name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Add foreign keys
	for _, fk := range diff.FKsAdded {
		stmt, err := g.GenerateAddForeignKey(diff.TableName, *fk)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Drop foreign keys
	for _, fk := range diff.FKsRemoved {
		stmt, err := g.GenerateDropForeignKey(diff.TableName, fk.Name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Handle comment change
	if diff.CommentChanged != nil {
		stmt := fmt.Sprintf("ALTER TABLE %s COMMENT = '%s';",
			tableName,
			escapeMySQLString(*diff.CommentChanged))
		statements = append(statements, stmt)
	}

	return statements, nil
}

// GenerateAddColumn generates an ADD COLUMN statement for MySQL.
func (g *MySQLGenerator) GenerateAddColumn(tableName string, column types.Column) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if column.Name == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
		g.quoteIdentifier(tableName),
		g.generateColumnDefinition(&column)), nil
}

// GenerateDropColumn generates a DROP COLUMN statement for MySQL.
func (g *MySQLGenerator) GenerateDropColumn(tableName, columnName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if columnName == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(columnName)), nil
}

// GenerateModifyColumn generates a MODIFY COLUMN statement for MySQL.
func (g *MySQLGenerator) GenerateModifyColumn(tableName string, column types.Column) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if column.Name == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;",
		g.quoteIdentifier(tableName),
		g.generateColumnDefinition(&column)), nil
}

// GenerateCreateIndex generates a CREATE INDEX statement for MySQL.
func (g *MySQLGenerator) GenerateCreateIndex(tableName string, index types.Index) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if index.Name == "" {
		return "", fmt.Errorf("index name is empty")
	}
	if len(index.Columns) == 0 {
		return "", fmt.Errorf("index has no columns")
	}

	var sb strings.Builder

	if index.Primary {
		sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD PRIMARY KEY (%s);",
			g.quoteIdentifier(tableName),
			g.quoteColumnList(index.Columns)))
	} else if index.Unique {
		sb.WriteString(fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);",
			g.quoteIdentifier(index.Name),
			g.quoteIdentifier(tableName),
			g.quoteColumnList(index.Columns)))
	} else {
		sb.WriteString(fmt.Sprintf("CREATE INDEX %s ON %s (%s);",
			g.quoteIdentifier(index.Name),
			g.quoteIdentifier(tableName),
			g.quoteColumnList(index.Columns)))
	}

	return sb.String(), nil
}

// GenerateDropIndex generates a DROP INDEX statement for MySQL.
func (g *MySQLGenerator) GenerateDropIndex(tableName, indexName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if indexName == "" {
		return "", fmt.Errorf("index name is empty")
	}

	return fmt.Sprintf("DROP INDEX %s ON %s;",
		g.quoteIdentifier(indexName),
		g.quoteIdentifier(tableName)), nil
}

// GenerateAddForeignKey generates an ADD FOREIGN KEY statement for MySQL.
func (g *MySQLGenerator) GenerateAddForeignKey(tableName string, fk types.ForeignKey) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if fk.Name == "" {
		return "", fmt.Errorf("foreign key name is empty")
	}
	if len(fk.Columns) == 0 {
		return "", fmt.Errorf("foreign key has no columns")
	}
	if fk.RefTable == "" {
		return "", fmt.Errorf("referenced table is empty")
	}
	if len(fk.RefColumns) == 0 {
		return "", fmt.Errorf("foreign key has no referenced columns")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(fk.Name),
		g.quoteColumnList(fk.Columns),
		g.quoteIdentifier(fk.RefTable),
		g.quoteColumnList(fk.RefColumns)))

	if fk.OnDelete != "" {
		sb.WriteString(fmt.Sprintf(" ON DELETE %s", strings.ToUpper(fk.OnDelete)))
	}
	if fk.OnUpdate != "" {
		sb.WriteString(fmt.Sprintf(" ON UPDATE %s", strings.ToUpper(fk.OnUpdate)))
	}

	sb.WriteString(";")

	return sb.String(), nil
}

// GenerateDropForeignKey generates a DROP FOREIGN KEY statement for MySQL.
func (g *MySQLGenerator) GenerateDropForeignKey(tableName, fkName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if fkName == "" {
		return "", fmt.Errorf("foreign key name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s;",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(fkName)), nil
}

// quoteColumnList quotes a list of column names.
func (g *MySQLGenerator) quoteColumnList(columns []string) string {
	quoted := make([]string, len(columns))
	for i, col := range columns {
		quoted[i] = g.quoteIdentifier(col)
	}
	return strings.Join(quoted, ", ")
}

// GenerateDiff generates DDL statements from a schema diff.
func (g *MySQLGenerator) GenerateDiff(diff *types.SchemaDiff) ([]string, error) {
	if diff == nil {
		return nil, fmt.Errorf("schema diff is nil")
	}

	var statements []string

	// Drop tables
	for _, table := range diff.TablesRemoved {
		stmt, err := g.GenerateDropTable(table.Name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Create new tables
	for _, table := range diff.TablesAdded {
		stmt, err := g.GenerateCreateTable(table)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	// Alter modified tables
	for _, tableDiff := range diff.TablesModified {
		stmts, err := g.GenerateAlterTable(tableDiff)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmts...)
	}

	return statements, nil
}

func escapeMySQLString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}
