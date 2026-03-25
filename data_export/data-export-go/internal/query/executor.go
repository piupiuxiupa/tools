package query

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Row represents a single row as map[string]interface{}
type Row map[string]interface{}

// Iterator provides streaming access to query results
type Iterator interface {
	Next() bool
	Scan(row *Row) error
	Close() error
	Err() error
}

// Executor executes queries with streaming support
type Executor interface {
	Query(ctx context.Context, query string, args ...interface{}) (Iterator, error)
	QueryBatch(ctx context.Context, query string, batchSize int, args ...interface{}) (BatchIterator, error)
}

// BatchIterator iterates over result batches
type BatchIterator interface {
	NextBatch() ([]Row, error)
	Close() error
	Err() error
}

// executor implements the Executor interface
type executor struct {
	db *sql.DB
}

// rowIterator implements the Iterator interface
type rowIterator struct {
	rows *sql.Rows
	err  error
}

// batchIterator implements the BatchIterator interface
type batchIterator struct {
	iter      *rowIterator
	batchSize int
	err       error
}

// NewExecutor creates a new query executor
func NewExecutor(db *sql.DB) Executor {
	return &executor{db: db}
}

// Query executes a query and returns a streaming iterator
func (e *executor) Query(ctx context.Context, query string, args ...interface{}) (Iterator, error) {
	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return &rowIterator{rows: rows}, nil
}

// QueryBatch executes a query and returns a batch iterator
func (e *executor) QueryBatch(ctx context.Context, query string, batchSize int, args ...interface{}) (BatchIterator, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}

	iter, err := e.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return &batchIterator{
		iter:      iter.(*rowIterator),
		batchSize: batchSize,
	}, nil
}

// Next advances to the next row
func (ri *rowIterator) Next() bool {
	if ri.err != nil {
		return false
	}

	next := ri.rows.Next()
	if !next {
		ri.err = ri.rows.Err()
	}
	return next
}

func (ri *rowIterator) Scan(row *Row) error {
	if ri.err != nil {
		return ri.err
	}

	if row == nil {
		return fmt.Errorf("row cannot be nil")
	}

	columns, err := ri.rows.Columns()
	if err != nil {
		ri.err = err
		return fmt.Errorf("failed to get columns: %w", err)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := ri.rows.Scan(valuePtrs...); err != nil {
		ri.err = err
		return fmt.Errorf("failed to scan row: %w", err)
	}

	*row = make(Row, len(columns))
	for i, col := range columns {
		(*row)[col] = convertValue(values[i])
	}

	return nil
}

// Close closes the iterator and releases resources
func (ri *rowIterator) Close() error {
	if ri.rows != nil {
		err := ri.rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Err returns any error encountered during iteration
func (ri *rowIterator) Err() error {
	return ri.err
}

// NextBatch returns the next batch of rows
func (bi *batchIterator) NextBatch() ([]Row, error) {
	if bi.err != nil {
		return nil, bi.err
	}

	var batch []Row
	count := 0

	for bi.iter.Next() {
		var row Row
		if err := bi.iter.Scan(&row); err != nil {
			bi.err = err
			return nil, err
		}
		batch = append(batch, row)
		count++
		if count >= bi.batchSize {
			break
		}
	}

	if err := bi.iter.Err(); err != nil {
		bi.err = err
		return nil, err
	}

	return batch, nil
}

// Close closes the batch iterator
func (bi *batchIterator) Close() error {
	return bi.iter.Close()
}

// Err returns any error encountered during batch iteration
func (bi *batchIterator) Err() error {
	return bi.err
}

// createScanDestination creates a destination variable for scanning based on the SQL type
func createScanDestination(dbType string) interface{} {
	switch dbType {
	case "INT", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT":
		return new(sql.NullInt64)
	case "FLOAT", "DOUBLE", "REAL":
		return new(sql.NullFloat64)
	case "VARCHAR", "CHAR", "TEXT", "LONGTEXT", "MEDIUMTEXT", "TINYTEXT", "DECIMAL", "NUMERIC":
		return new(sql.NullString)
	case "DATE", "TIME", "DATETIME", "TIMESTAMP":
		return new(sql.NullTime)
	case "BOOL", "BOOLEAN":
		return new(sql.NullBool)
	default:
		return new(sql.NullString)
	}
}

func convertValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case *sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
		return nil
	case *sql.NullFloat64:
		if val.Valid {
			return val.Float64
		}
		return nil
	case *sql.NullString:
		if val.Valid {
			return val.String
		}
		return nil
	case *sql.NullBool:
		if val.Valid {
			return val.Bool
		}
		return nil
	case *sql.NullTime:
		if val.Valid {
			return val.Time
		}
		return nil
	case string:
		return parseStringValue(val)
	case []byte:
		return parseStringValue(string(val))
	case int64:
		return val
	case int:
		return int64(val)
	case int32:
		return int64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	case bool:
		return val
	case time.Time:
		return val
	default:
		return v
	}
}

func parseStringValue(s string) interface{} {
	// Try int64 first to avoid "0" and "1" being parsed as boolean
	// Skip float parsing: DECIMAL/NUMERIC columns are scanned as strings
	// and must remain as strings to preserve precision.
	if i, err := parseInt64(s); err == nil {
		return i
	}
	if t, err := parseTime(s); err == nil {
		return t
	}
	if b, err := parseBool(s); err == nil {
		return b
	}
	return s
}

func parseInt64(s string) (int64, error) {
	var result int64
	n, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return 0, err
	}
	// Ensure entire string was consumed (no partial matches like "123.45" → 123)
	if n != 1 || fmt.Sprintf("%d", result) != s {
		return 0, fmt.Errorf("not a valid integer: %s", s)
	}
	return result, nil
}

func parseFloat64(s string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(s, "%f", &result)
	return result, err
}

func parseBool(s string) (bool, error) {
	switch s {
	case "true", "TRUE", "True", "1":
		return true, nil
	case "false", "FALSE", "False", "0":
		return false, nil
	}
	return false, fmt.Errorf("not a boolean")
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05.000000",
		"2006-01-02",
		"15:04:05",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown time format")
}

func ScanRow(ri *rowIterator, dest interface{}) error {
	return ri.Scan(dest.(*Row))
}

// ColumnInfo provides metadata about a result column
type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
}

// GetColumnInfo retrieves column information from the iterator
func GetColumnInfo(iter Iterator) ([]ColumnInfo, error) {
	ri, ok := iter.(*rowIterator)
	if !ok {
		return nil, fmt.Errorf("invalid iterator type")
	}

	if ri.rows == nil {
		return nil, fmt.Errorf("no rows available")
	}

	columnTypes, err := ri.rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	var info []ColumnInfo
	for _, ct := range columnTypes {
		nullable, _ := ct.Nullable()
		info = append(info, ColumnInfo{
			Name:     ct.Name(),
			Type:     ct.DatabaseTypeName(),
			Nullable: nullable,
		})
	}

	return info, nil
}

// Helper to create a Row from a map
func NewRow(data map[string]interface{}) Row {
	return Row(data)
}

// Helper to get a value from a row with type safety
func (r Row) GetString(key string) (string, bool) {
	v, ok := r[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// Helper to get int64 value from a row
func (r Row) GetInt64(key string) (int64, bool) {
	v, ok := r[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}

// Helper to get float64 value from a row
func (r Row) GetFloat64(key string) (float64, bool) {
	v, ok := r[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int64:
		return float64(val), true
	case int:
		return float64(val), true
	default:
		return 0, false
	}
}

// Helper to get time value from a row
func (r Row) GetTime(key string) (time.Time, bool) {
	v, ok := r[key]
	if !ok || v == nil {
		return time.Time{}, false
	}
	t, ok := v.(time.Time)
	return t, ok
}

// Helper to check if a value is nil
func (r Row) IsNull(key string) bool {
	v, ok := r[key]
	return !ok || v == nil
}
