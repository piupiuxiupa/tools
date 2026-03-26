package schema

import (
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/pkg/types"
)

// Comparator compares database schemas
type Comparator struct {
	ignoreTables []string
	typeMapper   types.TypeMapper
}

// Option configures the Comparator
type Option func(*Comparator)

// WithIgnoreTables sets tables to ignore during comparison
func WithIgnoreTables(tables ...string) Option {
	return func(c *Comparator) {
		c.ignoreTables = tables
	}
}

// WithTypeMapper sets a custom type mapper
func WithTypeMapper(tm types.TypeMapper) Option {
	return func(c *Comparator) {
		c.typeMapper = tm
	}
}

// NewComparator creates a new schema comparator
func NewComparator(options ...Option) *Comparator {
	c := &Comparator{
		ignoreTables: []string{},
		typeMapper:   types.NewDefaultTypeMapper(),
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// Compare compares two database schemas and returns the differences
func (c *Comparator) Compare(source, target *types.DatabaseSchema) (*types.SchemaDiff, error) {
	diff := &types.SchemaDiff{
		SourceName: source.Name,
		TargetName: target.Name,
	}

	// Build maps for easier lookup
	sourceTables := make(map[string]*types.Table)
	targetTables := make(map[string]*types.Table)

	for _, t := range source.Tables {
		if !c.shouldIgnoreTable(t.Name) {
			sourceTables[t.Name] = t
		}
	}

	for _, t := range target.Tables {
		if !c.shouldIgnoreTable(t.Name) {
			targetTables[t.Name] = t
		}
	}

	// Find added tables (in source, not in target)
	for name, table := range sourceTables {
		if _, exists := targetTables[name]; !exists {
			diff.TablesAdded = append(diff.TablesAdded, *table)
		}
	}

	// Find removed tables (in target, not in source)
	for name, table := range targetTables {
		if _, exists := sourceTables[name]; !exists {
			diff.TablesRemoved = append(diff.TablesRemoved, *table)
		}
	}

	// Find modified tables
	for name, sourceTable := range sourceTables {
		if targetTable, exists := targetTables[name]; exists {
			tableDiff, err := c.compareTables(sourceTable, targetTable)
			if err != nil {
				return nil, err
			}
			if tableDiff.HasChanges() {
				diff.TablesModified = append(diff.TablesModified, *tableDiff)
			}
		}
	}

	return diff, nil
}

// shouldIgnoreTable checks if a table should be ignored
func (c *Comparator) shouldIgnoreTable(tableName string) bool {
	for _, pattern := range c.ignoreTables {
		if matched, _ := matchWildcard(pattern, tableName); matched {
			return true
		}
	}
	return false
}

// compareTables compares two tables and returns the differences
func (c *Comparator) compareTables(source, target *types.Table) (*types.TableDiff, error) {
	diff := &types.TableDiff{
		TableName: source.Name,
	}

	// Compare columns
	sourceCols := make(map[string]*types.Column)
	targetCols := make(map[string]*types.Column)

	for _, col := range source.Columns {
		sourceCols[col.Name] = col
	}
	for _, col := range target.Columns {
		targetCols[col.Name] = col
	}

	// Find added columns
	for name, col := range sourceCols {
		if _, exists := targetCols[name]; !exists {
			diff.ColumnsAdded = append(diff.ColumnsAdded, *col)
		}
	}

	// Find removed columns
	for name, col := range targetCols {
		if _, exists := sourceCols[name]; !exists {
			diff.ColumnsRemoved = append(diff.ColumnsRemoved, *col)
		}
	}

	// Find modified columns
	for name, sourceCol := range sourceCols {
		if targetCol, exists := targetCols[name]; exists {
			if !sourceCol.IsEqual(targetCol) {
				diff.ColumnsModified = append(diff.ColumnsModified, types.ColumnDiff{
					ColumnName: name,
					ChangeType: types.ChangeTypeModify,
				})
			}
		}
	}

	// Compare indexes
	sourceIdxs := make(map[string]*types.Index)
	targetIdxs := make(map[string]*types.Index)

	for _, idx := range source.Indexes {
		sourceIdxs[idx.Name] = idx
	}
	for _, idx := range target.Indexes {
		targetIdxs[idx.Name] = idx
	}

	// Find added indexes
	for name, idx := range sourceIdxs {
		if _, exists := targetIdxs[name]; !exists {
			diff.IndexesAdded = append(diff.IndexesAdded, *idx)
		}
	}

	// Find removed indexes
	for name, idx := range targetIdxs {
		if _, exists := sourceIdxs[name]; !exists {
			diff.IndexesRemoved = append(diff.IndexesRemoved, *idx)
		}
	}

	// Compare foreign keys
	sourceFKs := make(map[string]*types.ForeignKey)
	targetFKs := make(map[string]*types.ForeignKey)

	for _, fk := range source.ForeignKeys {
		sourceFKs[fk.Name] = fk
	}
	for _, fk := range target.ForeignKeys {
		targetFKs[fk.Name] = fk
	}

	// Find added FKs
	for name, fk := range sourceFKs {
		if _, exists := targetFKs[name]; !exists {
			diff.ForeignKeysAdded = append(diff.ForeignKeysAdded, *fk)
		}
	}

	// Find removed FKs
	for name, fk := range targetFKs {
		if _, exists := sourceFKs[name]; !exists {
			diff.ForeignKeysRemoved = append(diff.ForeignKeysRemoved, *fk)
		}
	}

	return diff, nil
}

// matchWildcard matches a string against a pattern with wildcards
func matchWildcard(pattern, str string) (bool, error) {
	if pattern == str {
		return true, nil
	}
	// Simple wildcard matching - * matches any characters
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			if strings.HasPrefix(str, parts[0]) && strings.HasSuffix(str, parts[1]) {
				return true, nil
			}
		}
	}
	return false, nil
}
