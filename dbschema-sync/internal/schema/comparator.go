// Package schema provides database schema introspection and synchronization functionality.
package schema

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/lush/dbschema-sync/pkg/types"
)

// TypeMapper defines the interface for mapping types between different databases.
type TypeMapper interface {
	// MapType maps a source type to the target database type.
	MapType(sourceType string) (string, error)
	// AreTypesCompatible checks if two types are compatible.
	AreTypesCompatible(type1, type2 string) bool
}

// Option is a functional option for configuring the Comparator.
type Option func(*Comparator)

// WithIgnoreTables sets the tables to ignore during comparison.
func WithIgnoreTables(tables ...string) Option {
	return func(c *Comparator) {
		c.ignoreTables = tables
	}
}

// WithTypeMapper sets the type mapper for the comparator.
func WithTypeMapper(mapper TypeMapper) Option {
	return func(c *Comparator) {
		c.typeMapper = mapper
	}
}

// Comparator compares two DatabaseSchema objects and produces SchemaDiff.
type Comparator struct {
	ignoreTables []string
	typeMapper   TypeMapper
}

// NewComparator creates a new Comparator with the given options.
func NewComparator(options ...Option) *Comparator {
	c := &Comparator{
		ignoreTables: make([]string, 0),
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

// Compare compares two DatabaseSchema objects and produces a SchemaDiff.
func (c *Comparator) Compare(source, target *types.DatabaseSchema) (*types.SchemaDiff, error) {
	if source == nil {
		return nil, NewSchemaError("source schema is nil")
	}
	if target == nil {
		return nil, NewSchemaError("target schema is nil")
	}

	diff := &types.SchemaDiff{
		SourceSchema:   source.Name,
		TargetSchema:   target.Name,
		TablesAdded:    make([]*types.Table, 0),
		TablesRemoved:  make([]*types.Table, 0),
		TablesModified: make([]*types.TableDiff, 0),
		Metadata:       make(map[string]string),
	}

	// Build lookup maps for efficient comparison
	sourceTables := make(map[string]*types.Table)
	targetTables := make(map[string]*types.Table)

	for _, table := range source.Tables {
		if !c.shouldIgnoreTable(table.Name) {
			sourceTables[strings.ToLower(table.Name)] = table
		}
	}

	for _, table := range target.Tables {
		if !c.shouldIgnoreTable(table.Name) {
			targetTables[strings.ToLower(table.Name)] = table
		}
	}

	// Find added tables (in source, not in target)
	for name, table := range sourceTables {
		if _, exists := targetTables[name]; !exists {
			diff.TablesAdded = append(diff.TablesAdded, table)
		}
	}

	// Find removed tables (in target, not in source)
	for name, table := range targetTables {
		if _, exists := sourceTables[name]; !exists {
			diff.TablesRemoved = append(diff.TablesRemoved, table)
		}
	}

	// Find modified tables (same name, different structure)
	for name, sourceTable := range sourceTables {
		if targetTable, exists := targetTables[name]; exists {
			tableDiff, err := c.CompareTables(sourceTable, targetTable)
			if err != nil {
				return nil, fmt.Errorf("failed to compare table %s: %w", name, err)
			}

			if !tableDiff.IsEmpty() {
				diff.TablesModified = append(diff.TablesModified, tableDiff)
			}
		}
	}

	return diff, nil
}

// CompareTables compares two Table objects and produces a TableDiff.
func (c *Comparator) CompareTables(source, target *types.Table) (*types.TableDiff, error) {
	if source == nil {
		return nil, NewSchemaError("source table is nil")
	}
	if target == nil {
		return nil, NewSchemaError("target table is nil")
	}

	diff := &types.TableDiff{
		TableName:       source.Name,
		ColumnsAdded:    make([]*types.Column, 0),
		ColumnsRemoved:  make([]*types.Column, 0),
		ColumnsModified: make([]*types.ColumnDiff, 0),
		IndexesAdded:    make([]*types.Index, 0),
		IndexesRemoved:  make([]*types.Index, 0),
		IndexesModified: make([]*types.IndexDiff, 0),
		FKsAdded:        make([]*types.ForeignKey, 0),
		FKsRemoved:      make([]*types.ForeignKey, 0),
		FKsModified:     make([]*types.FKDiff, 0),
	}

	// Compare columns
	c.compareColumns(source, target, diff)

	// Compare indexes
	c.compareIndexes(source, target, diff)

	// Compare foreign keys
	c.compareForeignKeys(source, target, diff)

	// Compare table-level properties
	c.compareTableProperties(source, target, diff)

	return diff, nil
}

// compareColumns compares columns between two tables.
func (c *Comparator) compareColumns(source, target *types.Table, diff *types.TableDiff) {
	sourceColumns := make(map[string]*types.Column)
	targetColumns := make(map[string]*types.Column)

	for _, col := range source.Columns {
		sourceColumns[strings.ToLower(col.Name)] = col
	}

	for _, col := range target.Columns {
		targetColumns[strings.ToLower(col.Name)] = col
	}

	// Find added columns
	for name, col := range sourceColumns {
		if _, exists := targetColumns[name]; !exists {
			diff.ColumnsAdded = append(diff.ColumnsAdded, col)
		}
	}

	// Find removed columns
	for name, col := range targetColumns {
		if _, exists := sourceColumns[name]; !exists {
			diff.ColumnsRemoved = append(diff.ColumnsRemoved, col)
		}
	}

	// Find modified columns
	for name, sourceCol := range sourceColumns {
		if targetCol, exists := targetColumns[name]; exists {
			changes, err := c.CompareColumns(sourceCol, targetCol)
			if err == nil && len(changes) > 0 {
				columnDiff := &types.ColumnDiff{
					Column:     sourceCol,
					OldColumn:  targetCol,
					NewColumn:  sourceCol,
					ChangeType: types.ChangeModify,
					Changes:    changes,
				}
				diff.ColumnsModified = append(diff.ColumnsModified, columnDiff)
			}
		}
	}
}

// compareIndexes compares indexes between two tables.
func (c *Comparator) compareIndexes(source, target *types.Table, diff *types.TableDiff) {
	sourceIndexes := make(map[string]*types.Index)
	targetIndexes := make(map[string]*types.Index)

	for _, idx := range source.Indexes {
		sourceIndexes[strings.ToLower(idx.Name)] = idx
	}

	for _, idx := range target.Indexes {
		targetIndexes[strings.ToLower(idx.Name)] = idx
	}

	// Find added indexes
	for name, idx := range sourceIndexes {
		if _, exists := targetIndexes[name]; !exists {
			diff.IndexesAdded = append(diff.IndexesAdded, idx)
		}
	}

	// Find removed indexes
	for name, idx := range targetIndexes {
		if _, exists := sourceIndexes[name]; !exists {
			diff.IndexesRemoved = append(diff.IndexesRemoved, idx)
		}
	}

	// Find modified indexes
	for name, sourceIdx := range sourceIndexes {
		if targetIdx, exists := targetIndexes[name]; exists {
			if !sourceIdx.IsEqual(targetIdx) {
				changes := c.detectIndexChanges(sourceIdx, targetIdx)
				indexDiff := &types.IndexDiff{
					Index:      sourceIdx,
					OldIndex:   targetIdx,
					NewIndex:   sourceIdx,
					ChangeType: types.ChangeModify,
					Changes:    changes,
				}
				diff.IndexesModified = append(diff.IndexesModified, indexDiff)
			}
		}
	}
}

// compareForeignKeys compares foreign keys between two tables.
func (c *Comparator) compareForeignKeys(source, target *types.Table, diff *types.TableDiff) {
	sourceFKs := make(map[string]*types.ForeignKey)
	targetFKs := make(map[string]*types.ForeignKey)

	for _, fk := range source.ForeignKeys {
		sourceFKs[strings.ToLower(fk.Name)] = fk
	}

	for _, fk := range target.ForeignKeys {
		targetFKs[strings.ToLower(fk.Name)] = fk
	}

	// Find added foreign keys
	for name, fk := range sourceFKs {
		if _, exists := targetFKs[name]; !exists {
			diff.FKsAdded = append(diff.FKsAdded, fk)
		}
	}

	// Find removed foreign keys
	for name, fk := range targetFKs {
		if _, exists := sourceFKs[name]; !exists {
			diff.FKsRemoved = append(diff.FKsRemoved, fk)
		}
	}

	// Find modified foreign keys
	for name, sourceFK := range sourceFKs {
		if targetFK, exists := targetFKs[name]; exists {
			if !sourceFK.IsEqual(targetFK) {
				changes := c.detectFKChanges(sourceFK, targetFK)
				fkDiff := &types.FKDiff{
					ForeignKey: sourceFK,
					OldFK:      targetFK,
					NewFK:      sourceFK,
					ChangeType: types.ChangeModify,
					Changes:    changes,
				}
				diff.FKsModified = append(diff.FKsModified, fkDiff)
			}
		}
	}
}

// compareTableProperties compares table-level properties.
func (c *Comparator) compareTableProperties(source, target *types.Table, diff *types.TableDiff) {
	// Compare comments
	if source.Comment != target.Comment {
		changed := "changed"
		diff.CommentChanged = &changed
		diff.OldComment = target.Comment
		diff.NewComment = source.Comment
	}

	// Compare charset
	if source.Charset != target.Charset {
		diff.CharsetChanged = true
		diff.OldCharset = target.Charset
		diff.NewCharset = source.Charset
	}

	// Compare collation
	if source.Collation != target.Collation {
		diff.CollationChanged = true
		diff.OldCollation = target.Collation
		diff.NewCollation = source.Collation
	}
}

func (c *Comparator) CompareColumns(source, target *types.Column) ([]string, error) {
	if source == nil {
		return nil, NewSchemaError("source column is nil")
	}
	if target == nil {
		return nil, NewSchemaError("target column is nil")
	}

	var changes []string

	// Check data type change
	if !c.areTypesEqual(source.DataType, target.DataType) {
		changes = append(changes, "data_type")
	}

	// Check nullable change
	if source.Nullable != target.Nullable {
		changes = append(changes, "nullable")
	}

	// Check default value change
	if !c.compareStringPtr(source.Default, target.Default) {
		changes = append(changes, "default")
	}

	// Check length/precision/scale changes
	if !c.compareIntPtr(source.Length, target.Length) {
		changes = append(changes, "length")
	}
	if !c.compareIntPtr(source.Precision, target.Precision) {
		changes = append(changes, "precision")
	}
	if !c.compareIntPtr(source.Scale, target.Scale) {
		changes = append(changes, "scale")
	}

	// Check unsigned change
	if source.Unsigned != target.Unsigned {
		changes = append(changes, "unsigned")
	}

	// Check auto_increment change
	if source.AutoIncrement != target.AutoIncrement {
		changes = append(changes, "auto_increment")
	}

	// Check comment change
	if source.Comment != target.Comment {
		changes = append(changes, "comment")
	}

	// Check charset change
	if source.Charset != target.Charset {
		changes = append(changes, "charset")
	}

	// Check collation change
	if source.Collation != target.Collation {
		changes = append(changes, "collation")
	}

	return changes, nil
}

// areTypesEqual checks if two data types are equal, using TypeMapper if available.
func (c *Comparator) areTypesEqual(type1, type2 string) bool {
	// Direct comparison first
	if strings.EqualFold(type1, type2) {
		return true
	}

	// Use TypeMapper for compatibility check if available
	if c.typeMapper != nil {
		return c.typeMapper.AreTypesCompatible(type1, type2)
	}

	return false
}

// detectIndexChanges detects what changed between two indexes.
func (c *Comparator) detectIndexChanges(source, target *types.Index) []string {
	var changes []string

	if !strings.EqualFold(source.Name, target.Name) {
		changes = append(changes, "name")
	}

	if source.Unique != target.Unique {
		changes = append(changes, "unique")
	}

	if source.Primary != target.Primary {
		changes = append(changes, "primary")
	}

	if !reflect.DeepEqual(source.Columns, target.Columns) {
		changes = append(changes, "columns")
	}

	if !strings.EqualFold(source.IndexType, target.IndexType) {
		changes = append(changes, "index_type")
	}

	return changes
}

// detectFKChanges detects what changed between two foreign keys.
func (c *Comparator) detectFKChanges(source, target *types.ForeignKey) []string {
	var changes []string

	if !strings.EqualFold(source.Name, target.Name) {
		changes = append(changes, "name")
	}

	if !reflect.DeepEqual(source.Columns, target.Columns) {
		changes = append(changes, "columns")
	}

	if !strings.EqualFold(source.RefTable, target.RefTable) {
		changes = append(changes, "ref_table")
	}

	if !reflect.DeepEqual(source.RefColumns, target.RefColumns) {
		changes = append(changes, "ref_columns")
	}

	if !strings.EqualFold(source.OnDelete, target.OnDelete) {
		changes = append(changes, "on_delete")
	}

	if !strings.EqualFold(source.OnUpdate, target.OnUpdate) {
		changes = append(changes, "on_update")
	}

	return changes
}

// shouldIgnoreTable checks if a table should be ignored.
func (c *Comparator) shouldIgnoreTable(tableName string) bool {
	for _, ignore := range c.ignoreTables {
		if strings.EqualFold(ignore, tableName) {
			return true
		}
	}
	return false
}

// compareStringPtr compares two string pointers.
func (c *Comparator) compareStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// compareIntPtr compares two int pointers.
func (c *Comparator) compareIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
