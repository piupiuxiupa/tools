package schema

import (
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/pkg/types"
)

// PostgreSQLGenerator generates DDL statements for PostgreSQL.
type PostgreSQLGenerator struct {
	baseGenerator
}

// NewPostgreSQLGenerator creates a new PostgreSQL DDL generator.
func NewPostgreSQLGenerator(typeMapper types.TypeMapper) *PostgreSQLGenerator {
	return &PostgreSQLGenerator{
		baseGenerator: baseGenerator{
			typeMapper: typeMapper,
			dialect:    "postgresql",
		},
	}
}

// GenerateCreateTable generates a CREATE TABLE statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateCreateTable(table *types.Table) (string, error) {
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

	// Primary key
	if pk := table.GetPrimaryKey(); pk != nil {
		sb.WriteString(",\n")
		sb.WriteString(fmt.Sprintf("  PRIMARY KEY (%s)", g.quoteColumnList(pk.Columns)))
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

// generateColumnDefinition generates a column definition for PostgreSQL.
func (g *PostgreSQLGenerator) generateColumnDefinition(col *types.Column) string {
	var parts []string

	// Column name
	parts = append(parts, g.quoteIdentifier(col.Name))

	// Data type
	dataType := g.mapPostgreSQLType(col)
	parts = append(parts, dataType)

	// NULL/NOT NULL
	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	}

	// DEFAULT
	if col.Default != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", g.formatDefaultValue(col.Default)))
	}

	return strings.Join(parts, " ")
}

// mapPostgreSQLType maps a column type to PostgreSQL-specific syntax.
func (g *PostgreSQLGenerator) mapPostgreSQLType(col *types.Column) string {
	dataType := strings.ToLower(col.DataType)

	// Handle SERIAL types for auto-increment
	if col.AutoIncrement {
		switch dataType {
		case "smallint":
			return "SMALLSERIAL"
		case "integer", "int":
			return "SERIAL"
		case "bigint":
			return "BIGSERIAL"
		}
	}

	// Map through type mapper if available
	if g.typeMapper != nil {
		if mapped, err := g.typeMapper.MapType(col.DataType, "postgresql", "postgresql"); err == nil {
			dataType = mapped
		}
	}

	// Add size/precision for types that support it
	if col.Length != nil && *col.Length > 0 {
		// Only certain types support length in PostgreSQL
		switch dataType {
		case "character varying", "varchar", "character", "char", "bit", "bit varying":
			if idx := strings.Index(dataType, "("); idx != -1 {
				dataType = dataType[:idx]
			}
			dataType = fmt.Sprintf("%s(%d)", dataType, *col.Length)
		}
	} else if col.Precision != nil {
		switch dataType {
		case "numeric", "decimal":
			if col.Scale != nil {
				dataType = fmt.Sprintf("%s(%d,%d)", dataType, *col.Precision, *col.Scale)
			} else {
				dataType = fmt.Sprintf("%s(%d)", dataType, *col.Precision)
			}
		}
	}

	return dataType
}

// GenerateDropTable generates a DROP TABLE statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateDropTable(tableName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", g.quoteIdentifier(tableName)), nil
}

// GenerateAlterTable generates ALTER TABLE statements for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateAlterTable(diff *types.TableDiff) ([]string, error) {
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

// generateModifyColumnStatements generates statements to modify a column in PostgreSQL.
func (g *PostgreSQLGenerator) generateModifyColumnStatements(tableName string, colDiff *types.ColumnDiff) ([]string, error) {
	if colDiff.NewColumn == nil {
		return nil, fmt.Errorf("new column is nil")
	}

	var statements []string
	col := colDiff.NewColumn
	colName := g.quoteIdentifier(col.Name)
	table := g.quoteIdentifier(tableName)

	// Type change
	if colDiff.IsDataTypeChange() {
		stmt := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;",
			table, colName, g.mapPostgreSQLType(col))
		statements = append(statements, stmt)
	}

	// Nullable change
	if colDiff.IsNullableChange() {
		if col.Nullable {
			stmt := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;",
				table, colName)
			statements = append(statements, stmt)
		} else {
			stmt := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;",
				table, colName)
			statements = append(statements, stmt)
		}
	}

	// Default change
	if colDiff.IsDefaultChange() {
		if col.Default == nil {
			stmt := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;",
				table, colName)
			statements = append(statements, stmt)
		} else {
			stmt := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;",
				table, colName, g.formatDefaultValue(col.Default))
			statements = append(statements, stmt)
		}
	}

	return statements, nil
}

// GenerateAddColumn generates an ADD COLUMN statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateAddColumn(tableName string, column types.Column) (string, error) {
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

// GenerateDropColumn generates a DROP COLUMN statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateDropColumn(tableName, columnName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if columnName == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(columnName)), nil
}

// GenerateModifyColumn generates a MODIFY COLUMN statement for PostgreSQL.
// Note: PostgreSQL uses separate ALTER statements for different modifications.
func (g *PostgreSQLGenerator) GenerateModifyColumn(tableName string, column types.Column) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if column.Name == "" {
		return "", fmt.Errorf("column name is empty")
	}

	// For simplicity, generate a TYPE change statement
	return fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(column.Name),
		g.mapPostgreSQLType(&column)), nil
}

// GenerateCreateIndex generates a CREATE INDEX statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateCreateIndex(tableName string, index types.Index) (string, error) {
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
		// Primary key is usually added during table creation
		// If adding later, use ALTER TABLE
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

// GenerateDropIndex generates a DROP INDEX statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateDropIndex(tableName, indexName string) (string, error) {
	if indexName == "" {
		return "", fmt.Errorf("index name is empty")
	}

	return fmt.Sprintf("DROP INDEX IF EXISTS %s;",
		g.quoteIdentifier(indexName)), nil
}

// GenerateAddForeignKey generates an ADD FOREIGN KEY statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateAddForeignKey(tableName string, fk types.ForeignKey) (string, error) {
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

// GenerateDropForeignKey generates a DROP CONSTRAINT statement for PostgreSQL.
func (g *PostgreSQLGenerator) GenerateDropForeignKey(tableName, fkName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if fkName == "" {
		return "", fmt.Errorf("foreign key name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;",
		g.quoteIdentifier(tableName),
		g.quoteIdentifier(fkName)), nil
}

// quoteColumnList quotes a list of column names.
func (g *PostgreSQLGenerator) quoteColumnList(columns []string) string {
	quoted := make([]string, len(columns))
	for i, col := range columns {
		quoted[i] = g.quoteIdentifier(col)
	}
	return strings.Join(quoted, ", ")
}

// GenerateDiff generates DDL statements from a schema diff.
func (g *PostgreSQLGenerator) GenerateDiff(diff *types.SchemaDiff) ([]string, error) {
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
