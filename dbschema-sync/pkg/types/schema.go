// Package types provides core data structures for database schema representation
// and manipulation in the schema synchronization tool.
package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DatabaseSchema represents the complete schema of a database.
type DatabaseSchema struct {
	Name      string            `json:"name"`
	Tables    []*Table          `json:"tables"`
	Comment   string            `json:"comment,omitempty"`
	Charset   string            `json:"charset,omitempty"`
	Collation string            `json:"collation,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	RawDDL    string            `json:"raw_ddl,omitempty"`
}

// String returns a formatted string representation of the database schema.
func (ds *DatabaseSchema) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Database: %s\n", ds.Name))
	if ds.Comment != "" {
		sb.WriteString(fmt.Sprintf("Comment: %s\n", ds.Comment))
	}
	sb.WriteString(fmt.Sprintf("Tables: %d\n", len(ds.Tables)))
	for _, table := range ds.Tables {
		sb.WriteString(table.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// GetTable retrieves a table by name, returns nil if not found.
func (ds *DatabaseSchema) GetTable(name string) *Table {
	for _, t := range ds.Tables {
		if strings.EqualFold(t.Name, name) {
			return t
		}
	}
	return nil
}

// IsEqual compares two database schemas for equality.
func (ds *DatabaseSchema) IsEqual(other *DatabaseSchema) bool {
	if ds == nil || other == nil {
		return ds == other
	}
	if ds.Name != other.Name {
		return false
	}
	if len(ds.Tables) != len(other.Tables) {
		return false
	}

	// Sort tables by name for consistent comparison
	tables1 := make([]*Table, len(ds.Tables))
	tables2 := make([]*Table, len(other.Tables))
	copy(tables1, ds.Tables)
	copy(tables2, other.Tables)

	sort.Slice(tables1, func(i, j int) bool { return tables1[i].Name < tables1[j].Name })
	sort.Slice(tables2, func(i, j int) bool { return tables2[i].Name < tables2[j].Name })

	for i := range tables1 {
		if !tables1[i].IsEqual(tables2[i]) {
			return false
		}
	}
	return true
}

// ToJSON returns the schema as a JSON string.
func (ds *DatabaseSchema) ToJSON() (string, error) {
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema to JSON: %w", err)
	}
	return string(data), nil
}

// Table represents a database table with all its components.
type Table struct {
	Name        string            `json:"name"`
	Columns     []*Column         `json:"columns"`
	Indexes     []*Index          `json:"indexes"`
	Constraints []*Constraint     `json:"constraints,omitempty"`
	ForeignKeys []*ForeignKey     `json:"foreign_keys,omitempty"`
	Comment     string            `json:"comment,omitempty"`
	Charset     string            `json:"charset,omitempty"`
	Collation   string            `json:"collation,omitempty"`
	Engine      string            `json:"engine,omitempty"`
	RowFormat   string            `json:"row_format,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RawDDL      string            `json:"raw_ddl,omitempty"`
}

// String returns a formatted string representation of the table.
func (t *Table) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Table: %s\n", t.Name))
	if t.Comment != "" {
		sb.WriteString(fmt.Sprintf("    Comment: %s\n", t.Comment))
	}
	sb.WriteString(fmt.Sprintf("    Columns (%d):\n", len(t.Columns)))
	for _, col := range t.Columns {
		sb.WriteString(fmt.Sprintf("      %s\n", col.String()))
	}
	if len(t.Indexes) > 0 {
		sb.WriteString(fmt.Sprintf("    Indexes (%d):\n", len(t.Indexes)))
		for _, idx := range t.Indexes {
			sb.WriteString(fmt.Sprintf("      %s\n", idx.String()))
		}
	}
	if len(t.ForeignKeys) > 0 {
		sb.WriteString(fmt.Sprintf("    Foreign Keys (%d):\n", len(t.ForeignKeys)))
		for _, fk := range t.ForeignKeys {
			sb.WriteString(fmt.Sprintf("      %s\n", fk.String()))
		}
	}
	return sb.String()
}

// GetColumn retrieves a column by name, returns nil if not found.
func (t *Table) GetColumn(name string) *Column {
	for _, c := range t.Columns {
		if strings.EqualFold(c.Name, name) {
			return c
		}
	}
	return nil
}

// GetIndex retrieves an index by name, returns nil if not found.
func (t *Table) GetIndex(name string) *Index {
	for _, idx := range t.Indexes {
		if strings.EqualFold(idx.Name, name) {
			return idx
		}
	}
	return nil
}

// GetPrimaryKey returns the primary key index if it exists.
func (t *Table) GetPrimaryKey() *Index {
	for _, idx := range t.Indexes {
		if idx.Primary {
			return idx
		}
	}
	return nil
}

// IsEqual compares two tables for equality.
func (t *Table) IsEqual(other *Table) bool {
	if t == nil || other == nil {
		return t == other
	}
	if !strings.EqualFold(t.Name, other.Name) {
		return false
	}
	if t.Comment != other.Comment {
		return false
	}

	// Compare columns
	if len(t.Columns) != len(other.Columns) {
		return false
	}
	for i := range t.Columns {
		if !t.Columns[i].IsEqual(other.Columns[i]) {
			return false
		}
	}

	// Compare indexes
	if len(t.Indexes) != len(other.Indexes) {
		return false
	}
	for i := range t.Indexes {
		if !t.Indexes[i].IsEqual(other.Indexes[i]) {
			return false
		}
	}

	// Compare foreign keys
	if len(t.ForeignKeys) != len(other.ForeignKeys) {
		return false
	}
	for i := range t.ForeignKeys {
		if !t.ForeignKeys[i].IsEqual(other.ForeignKeys[i]) {
			return false
		}
	}

	return true
}

// Column represents a column in a database table.
type Column struct {
	Name          string            `json:"name"`
	DataType      string            `json:"data_type"`
	Nullable      bool              `json:"nullable"`
	Default       *string           `json:"default,omitempty"`
	Length        *int              `json:"length,omitempty"`
	Precision     *int              `json:"precision,omitempty"`
	Scale         *int              `json:"scale,omitempty"`
	Comment       string            `json:"comment,omitempty"`
	Position      int               `json:"position"`
	Charset       string            `json:"charset,omitempty"`
	Collation     string            `json:"collation,omitempty"`
	Unsigned      bool              `json:"unsigned,omitempty"`
	AutoIncrement bool              `json:"auto_increment,omitempty"`
	IsGenerated   bool              `json:"is_generated,omitempty"`
	GeneratedExpr string            `json:"generated_expr,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// String returns a formatted string representation of the column.
func (c *Column) String() string {
	parts := []string{c.Name, c.DataType}

	if c.Length != nil {
		parts = append(parts, fmt.Sprintf("(%d)", *c.Length))
	} else if c.Precision != nil {
		if c.Scale != nil {
			parts = append(parts, fmt.Sprintf("(%d,%d)", *c.Precision, *c.Scale))
		} else {
			parts = append(parts, fmt.Sprintf("(%d)", *c.Precision))
		}
	}

	if c.Unsigned {
		parts = append(parts, "UNSIGNED")
	}

	if !c.Nullable {
		parts = append(parts, "NOT NULL")
	}

	if c.Default != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT '%s'", *c.Default))
	}

	if c.AutoIncrement {
		parts = append(parts, "AUTO_INCREMENT")
	}

	if c.Comment != "" {
		parts = append(parts, fmt.Sprintf("COMMENT '%s'", c.Comment))
	}

	return strings.Join(parts, " ")
}

// IsEqual compares two columns for equality.
func (c *Column) IsEqual(other *Column) bool {
	if c == nil || other == nil {
		return c == other
	}
	if !strings.EqualFold(c.Name, other.Name) {
		return false
	}
	if !strings.EqualFold(c.DataType, other.DataType) {
		return false
	}
	if c.Nullable != other.Nullable {
		return false
	}
	if !compareStringPtr(c.Default, other.Default) {
		return false
	}
	if !compareIntPtr(c.Length, other.Length) {
		return false
	}
	if !compareIntPtr(c.Precision, other.Precision) {
		return false
	}
	if !compareIntPtr(c.Scale, other.Scale) {
		return false
	}
	if c.Comment != other.Comment {
		return false
	}
	if c.Unsigned != other.Unsigned {
		return false
	}
	if c.AutoIncrement != other.AutoIncrement {
		return false
	}
	return true
}

// GetDefaultValue returns the default value as a string, or empty string if nil.
func (c *Column) GetDefaultValue() string {
	if c.Default == nil {
		return ""
	}
	return *c.Default
}

// Index represents a database index on one or more columns.
type Index struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
	Primary   bool     `json:"primary"`
	IndexType string   `json:"index_type,omitempty"` // BTREE, HASH, etc.
	Comment   string   `json:"comment,omitempty"`
	Visible   bool     `json:"visible,omitempty"`
}

// String returns a formatted string representation of the index.
func (i *Index) String() string {
	parts := []string{i.Name}

	if i.Primary {
		parts = append(parts, "PRIMARY")
	} else if i.Unique {
		parts = append(parts, "UNIQUE")
	}

	parts = append(parts, fmt.Sprintf("(%s)", strings.Join(i.Columns, ", ")))

	if i.IndexType != "" {
		parts = append(parts, fmt.Sprintf("USING %s", i.IndexType))
	}

	return strings.Join(parts, " ")
}

// IsEqual compares two indexes for equality.
func (i *Index) IsEqual(other *Index) bool {
	if i == nil || other == nil {
		return i == other
	}
	if !strings.EqualFold(i.Name, other.Name) {
		return false
	}
	if i.Unique != other.Unique || i.Primary != other.Primary {
		return false
	}
	if !stringSliceEqual(i.Columns, other.Columns) {
		return false
	}
	if !strings.EqualFold(i.IndexType, other.IndexType) {
		return false
	}
	return true
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`
	OnDelete   string   `json:"on_delete,omitempty"` // CASCADE, SET NULL, RESTRICT, NO ACTION
	OnUpdate   string   `json:"on_update,omitempty"` // CASCADE, SET NULL, RESTRICT, NO ACTION
	Comment    string   `json:"comment,omitempty"`
}

// String returns a formatted string representation of the foreign key.
func (fk *ForeignKey) String() string {
	parts := []string{
		fk.Name,
		fmt.Sprintf("(%s)", strings.Join(fk.Columns, ", ")),
		"REFERENCES",
		fk.RefTable,
		fmt.Sprintf("(%s)", strings.Join(fk.RefColumns, ", ")),
	}

	if fk.OnDelete != "" {
		parts = append(parts, fmt.Sprintf("ON DELETE %s", fk.OnDelete))
	}

	if fk.OnUpdate != "" {
		parts = append(parts, fmt.Sprintf("ON UPDATE %s", fk.OnUpdate))
	}

	return strings.Join(parts, " ")
}

// IsEqual compares two foreign keys for equality.
func (fk *ForeignKey) IsEqual(other *ForeignKey) bool {
	if fk == nil || other == nil {
		return fk == other
	}
	if !strings.EqualFold(fk.Name, other.Name) {
		return false
	}
	if !strings.EqualFold(fk.RefTable, other.RefTable) {
		return false
	}
	if !stringSliceEqual(fk.Columns, other.Columns) {
		return false
	}
	if !stringSliceEqual(fk.RefColumns, other.RefColumns) {
		return false
	}
	if !strings.EqualFold(fk.OnDelete, other.OnDelete) {
		return false
	}
	if !strings.EqualFold(fk.OnUpdate, other.OnUpdate) {
		return false
	}
	return true
}

// Constraint represents a general table constraint.
type Constraint struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // CHECK, UNIQUE, PRIMARY, FOREIGN
	Definition string `json:"definition"`
	Comment    string `json:"comment,omitempty"`
}

// String returns a formatted string representation of the constraint.
func (c *Constraint) String() string {
	if c.Type != "" {
		return fmt.Sprintf("CONSTRAINT %s %s: %s", c.Name, c.Type, c.Definition)
	}
	return fmt.Sprintf("CONSTRAINT %s: %s", c.Name, c.Definition)
}

// IsEqual compares two constraints for equality.
func (c *Constraint) IsEqual(other *Constraint) bool {
	if c == nil || other == nil {
		return c == other
	}
	if !strings.EqualFold(c.Name, other.Name) {
		return false
	}
	if !strings.EqualFold(c.Type, other.Type) {
		return false
	}
	if c.Definition != other.Definition {
		return false
	}
	return true
}

// Helper functions

func compareStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func compareIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}
