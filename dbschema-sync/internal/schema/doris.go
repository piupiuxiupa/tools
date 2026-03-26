package schema

import (
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/pkg/types"
)

// DorisGenerator generates DDL statements for Apache Doris.
// Doris is compatible with MySQL syntax but has some specific features.
type DorisGenerator struct {
	MySQLGenerator
}

// NewDorisGenerator creates a new Doris DDL generator.
func NewDorisGenerator(typeMapper types.TypeMapper) *DorisGenerator {
	return &DorisGenerator{
		MySQLGenerator: MySQLGenerator{
			baseGenerator: baseGenerator{
				typeMapper: typeMapper,
				dialect:    "doris",
			},
		},
	}
}

// GenerateCreateTable generates a CREATE TABLE statement for Doris.
// Note: Doris requires specific considerations for data model (DUPLICATE/UNIQUE/AGGREGATE).
func (g *DorisGenerator) GenerateCreateTable(table *types.Table) (string, error) {
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
		sb.WriteString(g.generateDorisColumnDefinition(col))
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
			sb.WriteString(fmt.Sprintf("  UNIQUE KEY %s (%s)",
				g.quoteIdentifier(idx.Name),
				g.quoteColumnList(idx.Columns)))
		}
	}

	sb.WriteString("\n)")

	// Doris-specific: Add engine
	if table.Engine != "" {
		sb.WriteString(fmt.Sprintf(" ENGINE=%s", table.Engine))
	} else {
		sb.WriteString(" ENGINE=OLAP")
	}

	// Doris-specific: Add DUPLICATE KEY model if no primary key
	if table.GetPrimaryKey() == nil && len(table.Columns) > 0 {
		// Use first few columns as duplicate key
		keyCols := []string{}
		for i, col := range table.Columns {
			if i >= 3 {
				break
			}
			keyCols = append(keyCols, col.Name)
		}
		if len(keyCols) > 0 {
			sb.WriteString(fmt.Sprintf("\nDUPLICATE KEY(%s)", g.quoteColumnList(keyCols)))
		}
	}

	// Distribution (required for Doris)
	sb.WriteString("\nDISTRIBUTED BY HASH(")
	if pk := table.GetPrimaryKey(); pk != nil && len(pk.Columns) > 0 {
		sb.WriteString(g.quoteIdentifier(pk.Columns[0]))
	} else if len(table.Columns) > 0 {
		sb.WriteString(g.quoteIdentifier(table.Columns[0].Name))
	}
	sb.WriteString(") BUCKETS 10")

	// Properties
	sb.WriteString("\nPROPERTIES (\n")
	sb.WriteString("  \"replication_allocation\" = \"tag.location.default: 3\"\n")
	sb.WriteString(")")

	sb.WriteString(";")

	return sb.String(), nil
}

// generateDorisColumnDefinition generates a column definition for Doris.
func (g *DorisGenerator) generateDorisColumnDefinition(col *types.Column) string {
	var parts []string

	// Column name
	parts = append(parts, g.quoteIdentifier(col.Name))

	// Data type (Doris-specific)
	dataType := g.mapDorisType(col)
	parts = append(parts, dataType)

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

	// COMMENT
	if col.Comment != "" {
		parts = append(parts, fmt.Sprintf("COMMENT '%s'", escapeMySQLString(col.Comment)))
	}

	return strings.Join(parts, " ")
}

// mapDorisType maps a column type to Doris-specific syntax.
func (g *DorisGenerator) mapDorisType(col *types.Column) string {
	dataType := strings.ToLower(col.DataType)

	// Map through type mapper if available
	if g.typeMapper != nil {
		if mapped, err := g.typeMapper.MapType(col.DataType, "doris", "doris"); err == nil {
			dataType = mapped
		}
	}

	// Add size/precision
	if col.Length != nil && *col.Length > 0 {
		// Check if type supports length
		switch dataType {
		case "char", "varchar", "string", "hll", "bitmap":
			if dataType == "string" {
				// STRING in Doris doesn't take length
				return dataType
			}
			if idx := strings.Index(dataType, "("); idx != -1 {
				dataType = dataType[:idx]
			}
			dataType = fmt.Sprintf("%s(%d)", dataType, *col.Length)
		}
	} else if col.Precision != nil {
		if col.Scale != nil {
			dataType = fmt.Sprintf("%s(%d,%d)", dataType, *col.Precision, *col.Scale)
		} else {
			dataType = fmt.Sprintf("%s(%d)", dataType, *col.Precision)
		}
	}

	return strings.ToUpper(dataType)
}

// GenerateDropTable generates a DROP TABLE statement for Doris.
func (g *DorisGenerator) GenerateDropTable(tableName string) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", g.quoteIdentifier(tableName)), nil
}

// GenerateAlterTable generates ALTER TABLE statements for Doris.
// Note: Doris has limited ALTER TABLE support compared to MySQL.
func (g *DorisGenerator) GenerateAlterTable(diff *types.TableDiff) ([]string, error) {
	if diff == nil {
		return nil, fmt.Errorf("table diff is nil")
	}

	var statements []string

	// Doris supports: ADD COLUMN, DROP COLUMN (with limitations)
	// Modifying columns often requires recreation

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

	// Modify columns - note: limited support in Doris
	for _, colDiff := range diff.ColumnsModified {
		if colDiff.NewColumn != nil {
			// Doris: Use MODIFY COLUMN syntax
			stmt, err := g.GenerateModifyColumn(diff.TableName, *colDiff.NewColumn)
			if err != nil {
				return nil, err
			}
			statements = append(statements, stmt)
		}
	}

	// Add indexes (ROLLUP in Doris terminology)
	for _, idx := range diff.IndexesAdded {
		if !idx.Primary {
			// Doris uses ROLLUP for materialized views/indexes
			// For now, skip or use standard index syntax
			continue
		}
	}

	// Drop indexes
	for _, idx := range diff.IndexesRemoved {
		if !idx.Primary {
			stmt, err := g.GenerateDropIndex(diff.TableName, idx.Name)
			if err != nil {
				return nil, err
			}
			statements = append(statements, stmt)
		}
	}

	return statements, nil
}

// GenerateAddColumn generates an ADD COLUMN statement for Doris.
func (g *DorisGenerator) GenerateAddColumn(tableName string, column types.Column) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if column.Name == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;",
		g.quoteIdentifier(tableName),
		g.generateDorisColumnDefinition(&column)), nil
}

// GenerateDropColumn generates a DROP COLUMN statement for Doris.
func (g *DorisGenerator) GenerateDropColumn(tableName, columnName string) (string, error) {
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

// GenerateModifyColumn generates a MODIFY COLUMN statement for Doris.
// Note: Doris has limited support for column modification.
func (g *DorisGenerator) GenerateModifyColumn(tableName string, column types.Column) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if column.Name == "" {
		return "", fmt.Errorf("column name is empty")
	}

	return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;",
		g.quoteIdentifier(tableName),
		g.generateDorisColumnDefinition(&column)), nil
}

// GenerateCreateIndex generates a CREATE INDEX statement for Doris.
// Note: Doris uses ROLLUP for materialized indexes, but also supports standard indexes.
func (g *DorisGenerator) GenerateCreateIndex(tableName string, index types.Index) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("table name is empty")
	}
	if index.Name == "" {
		return "", fmt.Errorf("index name is empty")
	}
	if len(index.Columns) == 0 {
		return "", fmt.Errorf("index has no columns")
	}

	// Doris standard index syntax (similar to MySQL)
	if index.Unique {
		return fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);",
			g.quoteIdentifier(index.Name),
			g.quoteIdentifier(tableName),
			g.quoteColumnList(index.Columns)), nil
	}

	return fmt.Sprintf("CREATE INDEX %s ON %s (%s);",
		g.quoteIdentifier(index.Name),
		g.quoteIdentifier(tableName),
		g.quoteColumnList(index.Columns)), nil
}

// GenerateDropIndex generates a DROP INDEX statement for Doris.
func (g *DorisGenerator) GenerateDropIndex(tableName, indexName string) (string, error) {
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

// GenerateAddForeignKey generates an ADD FOREIGN KEY statement for Doris.
// Note: Doris doesn't natively support foreign keys, but we'll generate the syntax.
func (g *DorisGenerator) GenerateAddForeignKey(tableName string, fk types.ForeignKey) (string, error) {
	// Doris doesn't support foreign keys natively
	// Return empty or a comment indicating this
	return fmt.Sprintf("-- Foreign keys are not supported in Doris: %s", fk.Name), nil
}

// GenerateDropForeignKey generates a DROP FOREIGN KEY statement for Doris.
// Note: Doris doesn't natively support foreign keys.
func (g *DorisGenerator) GenerateDropForeignKey(tableName, fkName string) (string, error) {
	// Doris doesn't support foreign keys natively
	return fmt.Sprintf("-- Foreign keys are not supported in Doris: %s", fkName), nil
}

// GenerateDiff generates DDL statements from a schema diff.
func (g *DorisGenerator) GenerateDiff(diff *types.SchemaDiff) ([]string, error) {
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
