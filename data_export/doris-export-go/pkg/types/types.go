package types

// ExportResult represents the result of an export operation
type ExportResult struct {
	TableName string
	RowCount  int64
	FilePath  string
	Success   bool
	Error     error
}

// TableInfo holds information about a table
type TableInfo struct {
	Name    string
	Columns []ColumnInfo
}

// ColumnInfo holds information about a column
type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
}

// ExportOptions contains export configuration
type ExportOptions struct {
	OutputDir   string
	BatchSize   int
	Tables      []string
	Concurrency int
}
