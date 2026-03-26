package schema

import (
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/pkg/types"
)

// OracleGenerator generates DDL statements for Oracle Database.
type OracleGenerator struct {
	baseGenerator
}

// NewOracleGenerator creates a new Oracle DDL generator.
func NewOracleGenerator(typeMapper types.TypeMapper) *OracleGenerator {
	return &OracleGenerator{
		baseGenerator: baseGenerator{
			typeMapper: typeMapper,
			dialect:    "oracle",
		},
	}
}

// GenerateCreateTable generates a CREATE TABLE statement for Oracle.
func (g *OracleGenerator) GenerateCreateTable(table *types.Table) (string, error) {
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

	// Primary key constraint
	if pk := table.GetPrimaryKey(); pk != nil {
		sb.WriteString(",\n")
		sb.WriteString(fmt.Sprintf("  CONSTRAINT %s PRIMARY KEY (%s)",
			g.quoteIdentifier(fmt.Sprintf("pk_%s", table.Name)),
			g.quoteColumnList(pk.Columns)))
	}

	// Unique constraints
	for _, idx := range table.Indexes {
		if idx.Unique && !idx.Primary {
			sb.WriteString(",\n")
			sb.WriteString(fmt.Sprintf("  CONSTRAINT %s UNIQUE (%s)",
				g.quoteIdentifier(idx.Name),
				g.quoteColumnList(idx.Columns)))
		}
	}

	sb.WriteString("\n);")

	return sb.String(), nil
}

// generateColumnDefinition generates a column definition for Oracle.
func (g *OracleGenerator) generateColumnDefinition(col *types.Column) string {
	var parts []string

	// Column name
	parts = append(parts, g.quoteIdentifier(col.Name))

	// Data type
	dataType := g.mapOracleType(col)
	parts = append(parts, dataType)

	// NULL/NOT NULL
	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	}

	// DEFAULT
	if col.Default != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", g.formatOracleDefault(col.Default)))
	}

	return strings.Join(parts, " ")
}

// mapOracleType maps a column type to Oracle-specific syntax.
func (g *OracleGenerator) mapOracleType(col *types.Column) string {
	dataType := strings.ToLower(col.DataType)

	// Map through type mapper if available
	if g.typeMapper != nil {
		if mapped, err := g.typeMapper.MapType(col.DataType, "oracle", "oracle"); err == nil {
			dataType = mapped
		}
	}

	// Handle Oracle-specific types
	upperType := strings.ToUpper(dataType)

	// NUMBER precision handling
	if strings.HasPrefix(upperType, "NUMBER") {
		if col.Precision != nil {
			if col.Scale != nil && *col.Scale > 0 {
				return fmt.Sprintf("NUMBER(%d,%d)", *col.Precision, *col.Scale)
			}
			return fmt.Sprintf("NUMBER(%d)", *col.Precision)
		}
		return "NUMBER"
	}

	// VARCHAR2, CHAR length
	if strings.HasPrefix(upperType, "VARCHAR") || strings.HasPrefix(upperType, "VARCHAR2") {
		if col.Length != nil && *col.Length > 0 {
			return fmt.Sprintf("VARCHAR2(%d)", *col.Length)
		}
		return "VARCHAR2(255)"
	}

	if strings.HasPrefix(upperType, "CHAR") && !strings.HasPrefix(upperType, "CLOB") {
		if col.Length != nil && *col.Length > 0 {
			return fmt.Sprintf("CHAR(%d)", *col.Length)
		}
		return "CHAR(1)"
	}

	// RAW
	if strings.HasPrefix(upperType, "RAW") {
		if col.Length != nil && *col.Length > 0 {
			return fmt.Sprintf("RAW(%d)", *col.Length)
		}
		return "RAW(2000)"
	}

	// TIMESTAMP precision
	if strings.HasPrefix(upperType, "TIMESTAMP") {
		if col.Precision != nil && *col.Precision > 0 {
			return fmt.Sprintf("TIMESTAMP(%d)", *col.Precision)
		}
		return "TIMESTAMP"
	}

	return upperType
}

// formatOracleDefault formats a default value for Oracle.
func (g *OracleGenerator) formatOracleDefault(value *string) string {
	if value == nil {
		return ""
	}
	v := *value
	upperValue := strings.ToUpper(v)

	// Oracle-specific functions
	if upperValue == "CURRENT_TIMESTAMP" ||
		upperValue == "SYSDATE" ||
		upperValue == "SYSTIMESTAMP" ||
		strings.HasPrefix(upperValue, "SYS_GUID()") ||
		strings.HasPrefix(upperValue, "SEQ_") {
		return v
	}

	// Numeric values
	if _, err := fmt.Sscanf(v, "%f", new(float64)); err == nil {
		return v
	}

	// String values need quotes
	return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
}

// GenerateDropTable generates a DROP TABLE statement for Oracle.
func (g *OracleGenerator) GenerateDropTable(tableName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	return fmt.Sprintf("DROP TABLE %s CASCADE CONSTRAINTS PURGE;", g.quoteIdentifier(tableName)), nil
}

// GenerateAlterTable generates ALTER TABLE statements for Oracle.
func (g *OracleGenerator) GenerateAlterTable(diff *types.TableDiff) ([]string, error) {
	if diff == nil {
		return nil, fmt.Errorf("table diff is nil")
	}

	var statements []string

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
			stmts, err := g.generateModifyColumnStatements(diff.TableName, colDiff)
			if err != nil {
				return nil, err
			}
			statements = append(statements, stmts...)
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

	return statements, nil
}

// generateModifyColumnStatements generates statements to modify a column in Oracle.
func (g *OracleGenerator) generateModifyColumnStatements(tableName string, colDiff *types.ColumnDiff) ([]string, error) {
	if colDiff.NewColumn == nil {
		return nil, fmt.Errorf("new column is nil")
	}

	var statements []string
	col := colDiff.NewColumn
	colName := g.quoteIdentifier(col.Name)
	table := g.quoteIdentifier(tableName)

	// Type change
	if colDiff.IsDataTypeChange() {
		stmt := fmt.Sprintf("ALTER TABLE %s MODIFY %s %s;",
			table, colName, g.mapOracleType(col))
		statements = append(statements, stmt)
	}

	// Nullable change
	if colDiff.IsNullableChange() {
		if col.Nullable {
			stmt := fmt.Sprintf("ALTER TABLE %s MODIFY %s NULL;",
				table, colName)
			statements = append(statements, stmt)
		} else {
			stmt := fmt.Sprintf("ALTER TABLE %s MODIFY %s NOT NULL;",
				table, colName)
			statements = append(statements, stmt)
		}
	}

	// Default change
	if colDiff.IsDefaultChange() {
		if col.Default == nil {
			stmt := fmt.Sprintf("ALTER TABLE %s MODIFY %s DEFAULT NULL;",
				table, colName)
			statements = append(statements, stmt)
		} else {
			stmt := fmt.Sprintf("ALTER TABLE %s MODIFY %s DEFAULT %s;",
				table, colName, g.formatOracleDefault(col.Default))
			statements = append(statements, stmt)
		}
	}

	return statements, nil
}

// GenerateAddColumn generates an ADD COLUMN statement for Oracle.
func (g *OracleGenerator) GenerateAddColumn(tableName string, column types.Column) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if column.Name == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s ADD %s;",
		g.quoteIdentifier(tableName),
		g.generateColumnDefinition(&column)), nil
}

// GenerateDropColumn generates a DROP COLUMN statement for Oracle.
func (g *OracleGenerator) GenerateDropColumn(tableName, columnName string) (string, error) {
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

// GenerateModifyColumn generates a MODIFY COLUMN statement for Oracle.
func (g *OracleGenerator) GenerateModifyColumn(tableName string, column types.Column) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if column.Name == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s MODIFY %s %s;",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(column.Name),
		g.mapOracleType(&column)), nil
}

// GenerateCreateIndex generates a CREATE INDEX statement for Oracle.
func (g *OracleGenerator) GenerateCreateIndex(tableName string, index types.Index) (string, error) {
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

	if index.Unique {
		sb.WriteString("CREATE UNIQUE INDEX ")
	} else {
		sb.WriteString("CREATE INDEX ")
	}

	sb.WriteString(fmt.Sprintf("%s ON %s (%s);",
		g.quoteIdentifier(index.Name),
		g.quoteIdentifier(tableName),
		g.quoteColumnList(index.Columns)))

	return sb.String(), nil
}

// GenerateDropIndex generates a DROP INDEX statement for Oracle.
func (g *OracleGenerator) GenerateDropIndex(tableName, indexName string) (string, error) {
	if indexName == "" {
		return "", fmt.Errorf("index name is empty")
	}

	return fmt.Sprintf("DROP INDEX %s;",
		g.quoteIdentifier(indexName)), nil
}

// GenerateAddForeignKey generates an ADD FOREIGN KEY statement for Oracle.
func (g *OracleGenerator) GenerateAddForeignKey(tableName string, fk types.ForeignKey) (string, error) {
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

	// Oracle doesn't support ON UPDATE, so we skip it

	sb.WriteString(";")

	return sb.String(), nil
}

// GenerateDropForeignKey generates a DROP CONSTRAINT statement for Oracle.
func (g *OracleGenerator) GenerateDropForeignKey(tableName, fkName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if fkName == "" {
		return "", fmt.Errorf("foreign key name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(fkName)), nil
}

// quoteColumnList quotes a list of column names.
func (g *OracleGenerator) quoteColumnList(columns []string) string {
	quoted := make([]string, len(columns))
	for i, col := range columns {
		quoted[i] = g.quoteIdentifier(col)
	}
	return strings.Join(quoted, ", ")
}

// GenerateDiff generates DDL statements from a schema diff.
func (g *OracleGenerator) GenerateDiff(diff *types.SchemaDiff) ([]string, error) {
	if diff == nil {
		return nil, fmt.Errorf("schema diff is nil")
	}

	var statements []string

	for _, table := range diff.TablesRemoved {
		stmt, err := g.GenerateDropTable(table.Name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	for _, table := range diff.TablesAdded {
		stmt, err := g.GenerateCreateTable(table)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	for _, tableDiff := range diff.TablesModified {
		stmts, err := g.GenerateAlterTable(tableDiff)
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmts...)
	}

	return statements, nil
}
