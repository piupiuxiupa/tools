// Package types provides core data structures for database schema representation.
package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ChangeType represents the type of change in a schema diff.
type ChangeType string

const (
	// ChangeAdd represents adding a new element.
	ChangeAdd ChangeType = "add"
	// ChangeRemove represents removing an existing element.
	ChangeRemove ChangeType = "remove"
	// ChangeModify represents modifying an existing element.
	ChangeModify ChangeType = "modify"
)

// String returns the string representation of the change type.
func (ct ChangeType) String() string {
	return string(ct)
}

// Valid returns true if the change type is valid.
func (ct ChangeType) Valid() bool {
	switch ct {
	case ChangeAdd, ChangeRemove, ChangeModify:
		return true
	}
	return false
}

// SchemaDiff represents the differences between two database schemas.
type SchemaDiff struct {
	SourceSchema   string            `json:"source_schema"`
	TargetSchema   string            `json:"target_schema"`
	TablesAdded    []*Table          `json:"tables_added"`
	TablesRemoved  []*Table          `json:"tables_removed"`
	TablesModified []*TableDiff      `json:"tables_modified"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// IsEmpty returns true if there are no differences.
func (sd *SchemaDiff) IsEmpty() bool {
	return len(sd.TablesAdded) == 0 &&
		len(sd.TablesRemoved) == 0 &&
		len(sd.TablesModified) == 0
}

// HasChanges returns true if there are any changes.
func (sd *SchemaDiff) HasChanges() bool {
	return !sd.IsEmpty()
}

// Summary returns a summary of the changes.
func (sd *SchemaDiff) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Schema Diff: %s → %s\n", sd.SourceSchema, sd.TargetSchema))
	sb.WriteString(fmt.Sprintf("  Tables Added: %d\n", len(sd.TablesAdded)))
	sb.WriteString(fmt.Sprintf("  Tables Removed: %d\n", len(sd.TablesRemoved)))
	sb.WriteString(fmt.Sprintf("  Tables Modified: %d\n", len(sd.TablesModified)))

	for _, td := range sd.TablesModified {
		sb.WriteString(fmt.Sprintf("\n  Table '%s':\n", td.TableName))
		if len(td.ColumnsAdded) > 0 {
			sb.WriteString(fmt.Sprintf("    Columns Added: %d\n", len(td.ColumnsAdded)))
		}
		if len(td.ColumnsRemoved) > 0 {
			sb.WriteString(fmt.Sprintf("    Columns Removed: %d\n", len(td.ColumnsRemoved)))
		}
		if len(td.ColumnsModified) > 0 {
			sb.WriteString(fmt.Sprintf("    Columns Modified: %d\n", len(td.ColumnsModified)))
		}
		if len(td.IndexesAdded) > 0 {
			sb.WriteString(fmt.Sprintf("    Indexes Added: %d\n", len(td.IndexesAdded)))
		}
		if len(td.IndexesRemoved) > 0 {
			sb.WriteString(fmt.Sprintf("    Indexes Removed: %d\n", len(td.IndexesRemoved)))
		}
		if len(td.IndexesModified) > 0 {
			sb.WriteString(fmt.Sprintf("    Indexes Modified: %d\n", len(td.IndexesModified)))
		}
	}

	return sb.String()
}

// String returns a formatted string representation of the schema diff.
func (sd *SchemaDiff) String() string {
	return sd.Summary()
}

// ToJSON returns the diff as a JSON string.
func (sd *SchemaDiff) ToJSON() (string, error) {
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema diff to JSON: %w", err)
	}
	return string(data), nil
}

// GetAddedTableNames returns a list of names of tables that were added.
func (sd *SchemaDiff) GetAddedTableNames() []string {
	names := make([]string, len(sd.TablesAdded))
	for i, t := range sd.TablesAdded {
		names[i] = t.Name
	}
	return names
}

// GetRemovedTableNames returns a list of names of tables that were removed.
func (sd *SchemaDiff) GetRemovedTableNames() []string {
	names := make([]string, len(sd.TablesRemoved))
	for i, t := range sd.TablesRemoved {
		names[i] = t.Name
	}
	return names
}

// GetModifiedTableNames returns a list of names of tables that were modified.
func (sd *SchemaDiff) GetModifiedTableNames() []string {
	names := make([]string, len(sd.TablesModified))
	for i, td := range sd.TablesModified {
		names[i] = td.TableName
	}
	return names
}

// TableDiff represents the differences within a single table.
type TableDiff struct {
	TableName        string        `json:"table_name"`
	ColumnsAdded     []*Column     `json:"columns_added"`
	ColumnsRemoved   []*Column     `json:"columns_removed"`
	ColumnsModified  []*ColumnDiff `json:"columns_modified"`
	IndexesAdded     []*Index      `json:"indexes_added"`
	IndexesRemoved   []*Index      `json:"indexes_removed"`
	IndexesModified  []*IndexDiff  `json:"indexes_modified"`
	FKsAdded         []*ForeignKey `json:"fks_added"`
	FKsRemoved       []*ForeignKey `json:"fks_removed"`
	FKsModified      []*FKDiff     `json:"fks_modified"`
	CommentChanged   *string       `json:"comment_changed,omitempty"`
	OldComment       string        `json:"old_comment,omitempty"`
	NewComment       string        `json:"new_comment,omitempty"`
	CharsetChanged   bool          `json:"charset_changed"`
	OldCharset       string        `json:"old_charset,omitempty"`
	NewCharset       string        `json:"new_charset,omitempty"`
	CollationChanged bool          `json:"collation_changed"`
	OldCollation     string        `json:"old_collation,omitempty"`
	NewCollation     string        `json:"new_collation,omitempty"`
}

// IsEmpty returns true if there are no differences in this table.
func (td *TableDiff) IsEmpty() bool {
	return len(td.ColumnsAdded) == 0 &&
		len(td.ColumnsRemoved) == 0 &&
		len(td.ColumnsModified) == 0 &&
		len(td.IndexesAdded) == 0 &&
		len(td.IndexesRemoved) == 0 &&
		len(td.IndexesModified) == 0 &&
		len(td.FKsAdded) == 0 &&
		len(td.FKsRemoved) == 0 &&
		len(td.FKsModified) == 0 &&
		!td.CharsetChanged &&
		!td.CollationChanged &&
		td.CommentChanged == nil
}

// HasColumnChanges returns true if there are any column changes.
func (td *TableDiff) HasColumnChanges() bool {
	return len(td.ColumnsAdded) > 0 || len(td.ColumnsRemoved) > 0 || len(td.ColumnsModified) > 0
}

// HasIndexChanges returns true if there are any index changes.
func (td *TableDiff) HasIndexChanges() bool {
	return len(td.IndexesAdded) > 0 || len(td.IndexesRemoved) > 0 || len(td.IndexesModified) > 0
}

// HasFKChanges returns true if there are any foreign key changes.
func (td *TableDiff) HasFKChanges() bool {
	return len(td.FKsAdded) > 0 || len(td.FKsRemoved) > 0 || len(td.FKsModified) > 0
}

// GetAllChangeTypes returns a list of all change types present in this diff.
func (td *TableDiff) GetAllChangeTypes() []ChangeType {
	changes := make(map[ChangeType]bool)

	if len(td.ColumnsAdded) > 0 || len(td.IndexesAdded) > 0 || len(td.FKsAdded) > 0 {
		changes[ChangeAdd] = true
	}
	if len(td.ColumnsRemoved) > 0 || len(td.IndexesRemoved) > 0 || len(td.FKsRemoved) > 0 {
		changes[ChangeRemove] = true
	}
	if len(td.ColumnsModified) > 0 || len(td.IndexesModified) > 0 || len(td.FKsModified) > 0 ||
		td.CommentChanged != nil || td.CharsetChanged || td.CollationChanged {
		changes[ChangeModify] = true
	}

	result := make([]ChangeType, 0, len(changes))
	for ct := range changes {
		result = append(result, ct)
	}
	return result
}

// ColumnDiff represents a change to a column.
type ColumnDiff struct {
	Column     *Column    `json:"column"`
	OldColumn  *Column    `json:"old_column,omitempty"`
	NewColumn  *Column    `json:"new_column,omitempty"`
	ChangeType ChangeType `json:"change_type"`
	Changes    []string   `json:"changes,omitempty"` // List of what changed (e.g., ["data_type", "nullable"])
}

// String returns a formatted string representation of the column diff.
func (cd *ColumnDiff) String() string {
	switch cd.ChangeType {
	case ChangeAdd:
		return fmt.Sprintf("ADD COLUMN %s", cd.NewColumn.String())
	case ChangeRemove:
		return fmt.Sprintf("DROP COLUMN %s", cd.Column.Name)
	case ChangeModify:
		return fmt.Sprintf("MODIFY COLUMN %s (%s changed)", cd.Column.Name, strings.Join(cd.Changes, ", "))
	default:
		return fmt.Sprintf("UNKNOWN CHANGE: %s", cd.Column.Name)
	}
}

// IsDataTypeChange returns true if the data type was modified.
func (cd *ColumnDiff) IsDataTypeChange() bool {
	if cd.ChangeType != ChangeModify {
		return false
	}
	for _, change := range cd.Changes {
		if strings.EqualFold(change, "data_type") {
			return true
		}
	}
	return false
}

// IsNullableChange returns true if the nullable property was modified.
func (cd *ColumnDiff) IsNullableChange() bool {
	if cd.ChangeType != ChangeModify {
		return false
	}
	for _, change := range cd.Changes {
		if strings.EqualFold(change, "nullable") {
			return true
		}
	}
	return false
}

// IsDefaultChange returns true if the default value was modified.
func (cd *ColumnDiff) IsDefaultChange() bool {
	if cd.ChangeType != ChangeModify {
		return false
	}
	for _, change := range cd.Changes {
		if strings.EqualFold(change, "default") {
			return true
		}
	}
	return false
}

// IndexDiff represents a change to an index.
type IndexDiff struct {
	Index      *Index     `json:"index"`
	OldIndex   *Index     `json:"old_index,omitempty"`
	NewIndex   *Index     `json:"new_index,omitempty"`
	ChangeType ChangeType `json:"change_type"`
	Changes    []string   `json:"changes,omitempty"`
}

// String returns a formatted string representation of the index diff.
func (id *IndexDiff) String() string {
	switch id.ChangeType {
	case ChangeAdd:
		return fmt.Sprintf("ADD INDEX %s", id.NewIndex.String())
	case ChangeRemove:
		return fmt.Sprintf("DROP INDEX %s", id.Index.Name)
	case ChangeModify:
		return fmt.Sprintf("MODIFY INDEX %s (%s changed)", id.Index.Name, strings.Join(id.Changes, ", "))
	default:
		return fmt.Sprintf("UNKNOWN CHANGE: %s", id.Index.Name)
	}
}

// FKDiff represents a change to a foreign key.
type FKDiff struct {
	ForeignKey *ForeignKey `json:"foreign_key"`
	OldFK      *ForeignKey `json:"old_fk,omitempty"`
	NewFK      *ForeignKey `json:"new_fk,omitempty"`
	ChangeType ChangeType  `json:"change_type"`
	Changes    []string    `json:"changes,omitempty"`
}

// String returns a formatted string representation of the foreign key diff.
func (fkd *FKDiff) String() string {
	switch fkd.ChangeType {
	case ChangeAdd:
		return fmt.Sprintf("ADD FOREIGN KEY %s", fkd.NewFK.String())
	case ChangeRemove:
		return fmt.Sprintf("DROP FOREIGN KEY %s", fkd.ForeignKey.Name)
	case ChangeModify:
		return fmt.Sprintf("MODIFY FOREIGN KEY %s (%s changed)", fkd.ForeignKey.Name, strings.Join(fkd.Changes, ", "))
	default:
		return fmt.Sprintf("UNKNOWN CHANGE: %s", fkd.ForeignKey.Name)
	}
}

// DiffOptions contains options for schema comparison.
type DiffOptions struct {
	IgnoreComments    bool     `json:"ignore_comments"`
	IgnoreCase        bool     `json:"ignore_case"`
	IgnoreCharset     bool     `json:"ignore_charset"`
	IgnoreCollation   bool     `json:"ignore_collation"`
	IgnoreIndexes     bool     `json:"ignore_indexes"`
	IgnoreForeignKeys bool     `json:"ignore_foreign_keys"`
	IgnoreConstraints bool     `json:"ignore_constraints"`
	IgnoreDefaults    bool     `json:"ignore_defaults"`
	IgnorePositions   bool     `json:"ignore_positions"`
	IncludeTables     []string `json:"include_tables,omitempty"`
	ExcludeTables     []string `json:"exclude_tables,omitempty"`
}

// DefaultDiffOptions returns the default diff options.
func DefaultDiffOptions() *DiffOptions {
	return &DiffOptions{
		IgnoreComments:    false,
		IgnoreCase:        false,
		IgnoreCharset:     false,
		IgnoreCollation:   false,
		IgnoreIndexes:     false,
		IgnoreForeignKeys: false,
		IgnoreConstraints: false,
		IgnoreDefaults:    false,
		IgnorePositions:   false,
	}
}

// ShouldIncludeTable returns true if the table should be included in the diff.
func (do *DiffOptions) ShouldIncludeTable(tableName string) bool {
	// Check exclude list first
	for _, exclude := range do.ExcludeTables {
		if strings.EqualFold(exclude, tableName) {
			return false
		}
	}

	// If include list is specified, table must be in it
	if len(do.IncludeTables) > 0 {
		for _, include := range do.IncludeTables {
			if strings.EqualFold(include, tableName) {
				return true
			}
		}
		return false
	}

	return true
}
