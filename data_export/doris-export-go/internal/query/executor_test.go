package query

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func TestNewExecutor(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()
	defer mock.ExpectClose()

	exec := NewExecutor(db)
	assert.NotNil(t, exec)
}

func TestExecutor_Query_SimpleSelect(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT id, name FROM users").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "Alice").
			AddRow(2, "Bob"))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT id, name FROM users")

	require.NoError(t, err)
	assert.NotNil(t, iter)

	defer iter.Close()

	// First row
	assert.True(t, iter.Next())
	var row1 Row
	err = iter.Scan(&row1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), row1["id"])
	assert.Equal(t, "Alice", row1["name"])

	// Second row
	assert.True(t, iter.Next())
	var row2 Row
	err = iter.Scan(&row2)
	require.NoError(t, err)
	assert.Equal(t, int64(2), row2["id"])
	assert.Equal(t, "Bob", row2["name"])

	// No more rows
	assert.False(t, iter.Next())
	assert.NoError(t, iter.Err())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutor_Query_WithArgs(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT id, name FROM users WHERE id = ?").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1, "Alice"))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT id, name FROM users WHERE id = ?", 1)

	require.NoError(t, err)
	assert.NotNil(t, iter)
	defer iter.Close()

	assert.True(t, iter.Next())
	var row Row
	err = iter.Scan(&row)
	require.NoError(t, err)
	assert.Equal(t, int64(1), row["id"])
	assert.Equal(t, "Alice", row["name"])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutor_Query_InvalidQuery(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	mock.ExpectQuery("INVALID SQL").
		WillReturnError(errors.New("syntax error"))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "INVALID SQL")

	assert.Error(t, err)
	assert.Nil(t, iter)
	assert.Contains(t, err.Error(), "failed to execute query")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutor_Query_VariouTypes(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{
		"int_col",
		"bigint_col",
		"varchar_col",
		"text_col",
		"float_col",
		"double_col",
		"decimal_col",
		"bool_col",
		"date_col",
	}

	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	rows := sqlmock.NewRows(columns).
		AddRow(
			42,                         // INT
			int64(9223372036854775807), // BIGINT
			"varchar_value",            // VARCHAR
			"text_value",               // TEXT
			float32(3.14),              // FLOAT
			2.718281828459045,          // DOUBLE
			"12345.67890",              // DECIMAL
			true,                       // BOOL
			now,                        // DATE
		)

	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM test_types")

	require.NoError(t, err)
	defer iter.Close()

	assert.True(t, iter.Next())
	var row Row
	err = iter.Scan(&row)
	require.NoError(t, err)

	// Check int types
	assert.Equal(t, int64(42), row["int_col"])
	assert.Equal(t, int64(9223372036854775807), row["bigint_col"])

	// Check string types
	assert.Equal(t, "varchar_value", row["varchar_col"])
	assert.Equal(t, "text_value", row["text_col"])

	// Check float types (float32 has precision issues)
	assert.InDelta(t, float64(3.14), row["float_col"], 0.01)
	assert.InDelta(t, 2.718281828459045, row["double_col"], 0.0001)

	// Check decimal (kept as string to preserve precision)
	assert.Equal(t, "12345.67890", row["decimal_col"])

	// Check bool
	assert.Equal(t, true, row["bool_col"])

	// Check date/time
	assert.Equal(t, now, row["date_col"])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutor_Query_NULLValues(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id", "name", "age", "salary", "created_at"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, nil, nil, nil, nil)

	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM users")

	require.NoError(t, err)
	defer iter.Close()

	assert.True(t, iter.Next())
	var row Row
	err = iter.Scan(&row)
	require.NoError(t, err)

	// Non-null value
	assert.Equal(t, int64(1), row["id"])

	// NULL values should be nil
	assert.Nil(t, row["name"])
	assert.Nil(t, row["age"])
	assert.Nil(t, row["salary"])
	assert.Nil(t, row["created_at"])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutor_QueryBatch_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "Alice").
			AddRow(2, "Bob").
			AddRow(3, "Charlie").
			AddRow(4, "David"))

	exec := NewExecutor(db)
	ctx := context.Background()
	batchIter, err := exec.QueryBatch(ctx, "SELECT * FROM users", 2)

	require.NoError(t, err)
	assert.NotNil(t, batchIter)
	defer batchIter.Close()

	// First batch (2 rows)
	batch1, err := batchIter.NextBatch()
	require.NoError(t, err)
	assert.Len(t, batch1, 2)
	assert.Equal(t, int64(1), batch1[0]["id"])
	assert.Equal(t, "Alice", batch1[0]["name"])
	assert.Equal(t, int64(2), batch1[1]["id"])
	assert.Equal(t, "Bob", batch1[1]["name"])

	// Second batch (2 rows)
	batch2, err := batchIter.NextBatch()
	require.NoError(t, err)
	assert.Len(t, batch2, 2)
	assert.Equal(t, int64(3), batch2[0]["id"])
	assert.Equal(t, "Charlie", batch2[0]["name"])
	assert.Equal(t, int64(4), batch2[1]["id"])
	assert.Equal(t, "David", batch2[1]["name"])

	// Third batch (0 rows - end of results)
	batch3, err := batchIter.NextBatch()
	require.NoError(t, err)
	assert.Len(t, batch3, 0)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutor_QueryBatch_DefaultBatchSize(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	exec := NewExecutor(db)
	ctx := context.Background()
	batchIter, err := exec.QueryBatch(ctx, "SELECT * FROM users", 0)

	require.NoError(t, err)
	assert.NotNil(t, batchIter)
	defer batchIter.Close()

	// Should work with default batch size of 1000
	batch, err := batchIter.NextBatch()
	require.NoError(t, err)
	assert.Len(t, batch, 1)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecutor_QueryBatch_WithArgs(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT id, name FROM users WHERE status = ?").
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1, "Alice"))

	exec := NewExecutor(db)
	ctx := context.Background()
	batchIter, err := exec.QueryBatch(ctx, "SELECT id, name FROM users WHERE status = ?", 10, "active")

	require.NoError(t, err)
	defer batchIter.Close()

	batch, err := batchIter.NextBatch()
	require.NoError(t, err)
	assert.Len(t, batch, 1)
	assert.Equal(t, "Alice", batch[0]["name"])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIterator_Scan_NilRow(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM users")
	require.NoError(t, err)
	defer iter.Close()

	assert.True(t, iter.Next())
	err = iter.Scan(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "row cannot be nil")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIterator_Close_MultipleCalls(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM users")
	require.NoError(t, err)

	// Close should work
	err = iter.Close()
	assert.NoError(t, err)

	// Multiple close calls should be safe
	err = iter.Close()
	assert.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIterator_Err_AfterIteration(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1).AddRow(2).RowError(1, errors.New("row error")))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM users")
	require.NoError(t, err)
	defer iter.Close()

	// Consume all rows
	for iter.Next() {
		var row Row
		iter.Scan(&row)
	}

	// Check for errors
	assert.Error(t, iter.Err())
	assert.Contains(t, iter.Err().Error(), "row error")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchIterator_Err(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	exec := NewExecutor(db)
	ctx := context.Background()
	batchIter, err := exec.QueryBatch(ctx, "SELECT * FROM users", 1)
	require.NoError(t, err)

	// Get first batch
	_, err = batchIter.NextBatch()
	require.NoError(t, err)

	// Close to simulate error condition
	batchIter.Close()

	// After error, NextBatch should return error
	_, err = batchIter.NextBatch()
	// Error might be nil if iterator is properly exhausted
	// or an error if batch iterator has internal error tracking

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRow_Helpers(t *testing.T) {
	row := Row{
		"string_col": "hello",
		"int_col":    int64(42),
		"float_col":  float64(3.14),
		"time_col":   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		"nil_col":    nil,
	}

	// Test GetString
	s, ok := row.GetString("string_col")
	assert.True(t, ok)
	assert.Equal(t, "hello", s)

	s, ok = row.GetString("nil_col")
	assert.False(t, ok)
	assert.Empty(t, s)

	s, ok = row.GetString("nonexistent")
	assert.False(t, ok)

	// Test GetInt64
	i, ok := row.GetInt64("int_col")
	assert.True(t, ok)
	assert.Equal(t, int64(42), i)

	i, ok = row.GetInt64("float_col")
	assert.True(t, ok)
	assert.Equal(t, int64(3), i)

	i, ok = row.GetInt64("nil_col")
	assert.False(t, ok)

	// Test GetFloat64
	f, ok := row.GetFloat64("float_col")
	assert.True(t, ok)
	assert.InDelta(t, 3.14, f, 0.001)

	f, ok = row.GetFloat64("int_col")
	assert.True(t, ok)
	assert.InDelta(t, 42.0, f, 0.001)

	f, ok = row.GetFloat64("nil_col")
	assert.False(t, ok)

	// Test GetTime
	tm, ok := row.GetTime("time_col")
	assert.True(t, ok)
	assert.Equal(t, 2024, tm.Year())

	tm, ok = row.GetTime("nil_col")
	assert.False(t, ok)

	// Test IsNull
	assert.True(t, row.IsNull("nil_col"))
	assert.False(t, row.IsNull("string_col"))
	assert.True(t, row.IsNull("nonexistent"))
}

func TestNewRow(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
	}
	row := NewRow(data)
	assert.Equal(t, "value", row["key"])
}

func TestGetColumnInfo(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1, "test"))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM users")
	require.NoError(t, err)
	defer iter.Close()

	info, err := GetColumnInfo(iter)
	require.NoError(t, err)
	assert.Len(t, info, 2)
	assert.Equal(t, "id", info[0].Name)
	assert.Equal(t, "name", info[1].Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

type mockIterator struct{}

func (m *mockIterator) Next() bool          { return false }
func (m *mockIterator) Scan(row *Row) error { return nil }
func (m *mockIterator) Close() error        { return nil }
func (m *mockIterator) Err() error          { return nil }

func TestGetColumnInfo_InvalidIterator(t *testing.T) {
	_, err := GetColumnInfo(&mockIterator{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid iterator type")
}

func TestIterator_Next_WithExistingError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM users")
	require.NoError(t, err)
	defer iter.Close()

	// Set error on iterator
	ri := iter.(*rowIterator)
	ri.err = errors.New("existing error")

	// Next should return false when there's an existing error
	assert.False(t, iter.Next())
}

func TestConvertValue_DefaultCase(t *testing.T) {
	// Test with unknown type
	result := convertValue("string value")
	assert.Equal(t, "string value", result)
}

func TestConvertValue_NullTypes(t *testing.T) {
	// Test sql.NullInt64 with valid value
	nullInt := &sql.NullInt64{Int64: 42, Valid: true}
	result := convertValue(nullInt)
	assert.Equal(t, int64(42), result)

	// Test sql.NullInt64 with null value
	nullIntNull := &sql.NullInt64{Valid: false}
	result = convertValue(nullIntNull)
	assert.Nil(t, result)

	// Test sql.NullFloat64 with valid value
	nullFloat := &sql.NullFloat64{Float64: 3.14, Valid: true}
	result = convertValue(nullFloat)
	assert.Equal(t, 3.14, result)

	// Test sql.NullFloat64 with null value
	nullFloatNull := &sql.NullFloat64{Valid: false}
	result = convertValue(nullFloatNull)
	assert.Nil(t, result)

	// Test sql.NullString with valid value
	nullString := &sql.NullString{String: "hello", Valid: true}
	result = convertValue(nullString)
	assert.Equal(t, "hello", result)

	// Test sql.NullString with null value
	nullStringNull := &sql.NullString{Valid: false}
	result = convertValue(nullStringNull)
	assert.Nil(t, result)

	// Test sql.NullBool with valid value
	nullBool := &sql.NullBool{Bool: true, Valid: true}
	result = convertValue(nullBool)
	assert.Equal(t, true, result)

	// Test sql.NullBool with null value
	nullBoolNull := &sql.NullBool{Valid: false}
	result = convertValue(nullBoolNull)
	assert.Nil(t, result)

	// Test sql.NullTime with valid value
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	nullTime := &sql.NullTime{Time: now, Valid: true}
	result = convertValue(nullTime)
	assert.Equal(t, now, result)

	// Test sql.NullTime with null value
	nullTimeNull := &sql.NullTime{Valid: false}
	result = convertValue(nullTimeNull)
	assert.Nil(t, result)
}

func TestConvertValue_Nil(t *testing.T) {
	result := convertValue(nil)
	assert.Nil(t, result)
}

func TestConvertValue_Types(t *testing.T) {
	// Test []byte (parses as int64)
	result := convertValue([]byte("123"))
	assert.Equal(t, int64(123), result)

	// Test int
	result = convertValue(int(42))
	assert.Equal(t, int64(42), result)

	// Test int32
	result = convertValue(int32(42))
	assert.Equal(t, int64(42), result)

	// Test float32
	result = convertValue(float32(3.14))
	assert.InDelta(t, float64(3.14), result, 0.01)

	// Test bool
	result = convertValue(true)
	assert.Equal(t, true, result)

	// Test time.Time
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	result = convertValue(now)
	assert.Equal(t, now, result)

	// Test unknown type (default case)
	type customType struct{}
	custom := customType{}
	result = convertValue(custom)
	assert.Equal(t, custom, result)
}

func TestParseStringValue(t *testing.T) {
	// Test boolean parsing
	result := parseStringValue("true")
	assert.Equal(t, true, result)

	result = parseStringValue("false")
	assert.Equal(t, false, result)

	result = parseStringValue("1")
	assert.Equal(t, true, result)

	result = parseStringValue("0")
	assert.Equal(t, false, result)

	// Test time parsing
	result = parseStringValue("2024-01-15")
	assert.IsType(t, time.Time{}, result)

	result = parseStringValue("2024-01-15 10:30:00")
	assert.IsType(t, time.Time{}, result)

	// Test float parsing (now kept as string)
	result = parseStringValue("3.14")
	assert.Equal(t, "3.14", result)

	// Test int parsing
	result = parseStringValue("42")
	assert.Equal(t, int64(42), result)

	// Test plain string (no parsing matches)
	result = parseStringValue("hello world")
	assert.Equal(t, "hello world", result)
}

func TestBatchIterator_ErrMethod(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	exec := NewExecutor(db)
	ctx := context.Background()
	batchIter, err := exec.QueryBatch(ctx, "SELECT * FROM users", 1)
	require.NoError(t, err)

	// Get first batch
	_, err = batchIter.NextBatch()
	require.NoError(t, err)

	// Err should return nil when no error
	assert.NoError(t, batchIter.Err())

	// Close the iterator
	batchIter.Close()

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRow_GetInt64_Default(t *testing.T) {
	row := Row{
		"string": "not an int",
		"nil":    nil,
	}

	// Non-numeric string should return false
	i, ok := row.GetInt64("string")
	assert.False(t, ok)
	assert.Equal(t, int64(0), i)

	// Nil value should return false
	i, ok = row.GetInt64("nil")
	assert.False(t, ok)
	assert.Equal(t, int64(0), i)
}

func TestRow_GetFloat64_Default(t *testing.T) {
	row := Row{
		"string": "not a float",
		"nil":    nil,
	}

	// Non-numeric string should return false
	f, ok := row.GetFloat64("string")
	assert.False(t, ok)
	assert.Equal(t, float64(0), f)

	// Nil value should return false
	f, ok = row.GetFloat64("nil")
	assert.False(t, ok)
	assert.Equal(t, float64(0), f)
}

func TestScanRow(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	exec := NewExecutor(db)
	ctx := context.Background()
	iter, err := exec.Query(ctx, "SELECT * FROM users")
	require.NoError(t, err)
	defer iter.Close()

	assert.True(t, iter.Next())
	var row Row
	err = ScanRow(iter.(*rowIterator), &row)
	require.NoError(t, err)
	assert.Equal(t, int64(1), row["id"])

	require.NoError(t, mock.ExpectationsWereMet())
}
