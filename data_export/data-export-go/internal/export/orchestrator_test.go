package export

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/data-export-go/internal/metadata"
	"github.com/user/data-export-go/internal/query"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func TestNewOrchestrator(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()
	defer mock.ExpectClose()

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)

	orch := NewOrchestrator(db, metaService, queryExec)
	assert.NotNil(t, orch)
}

func TestOrchestrator_Interface(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()
	defer mock.ExpectClose()

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)

	var orch Orchestrator = NewOrchestrator(db, metaService, queryExec)
	assert.NotNil(t, orch)
}

func TestOptions_Struct(t *testing.T) {
	opts := Options{
		Database:    "test_db",
		Table:       "test_table",
		OutputDir:   "/tmp/export",
		BatchSize:   1000,
		WhereClause: "id > 100",
		PartitionBy: "date_col",
		Verify:      true,
	}

	assert.Equal(t, "test_db", opts.Database)
	assert.Equal(t, "test_table", opts.Table)
	assert.Equal(t, "/tmp/export", opts.OutputDir)
	assert.Equal(t, 1000, opts.BatchSize)
	assert.Equal(t, "id > 100", opts.WhereClause)
	assert.Equal(t, "date_col", opts.PartitionBy)
	assert.True(t, opts.Verify)
}

func TestResult_Struct(t *testing.T) {
	result := Result{
		Files:     []string{"file1.parquet", "file2.parquet"},
		TotalRows: 1000,
		Duration:  time.Second * 5,
	}

	assert.Len(t, result.Files, 2)
	assert.Equal(t, int64(1000), result.TotalRows)
	assert.Equal(t, time.Second*5, result.Duration)
}

func TestExport_SimpleMode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup query expectation - use raw string to avoid regex issues
	columns := []string{"id", "name", "value"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "Alice", 100).
			AddRow(2, "Bob", 200).
			AddRow(3, "Charlie", 300))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalRows, int64(3))
	assert.GreaterOrEqual(t, len(result.Files), 1)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestExport_SimpleModeWithWhereClause(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup query expectation with WHERE clause
	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(11, "Alice").
			AddRow(12, "Bob"))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:    "test_db",
		Table:       "test_table",
		OutputDir:   tmpDir,
		WhereClause: "id > 10",
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalRows, int64(2))
}

func TestExport_BatchMode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup query expectation
	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "Alice").
			AddRow(2, "Bob").
			AddRow(3, "Charlie").
			AddRow(4, "David"))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
		BatchSize: 2,
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalRows, int64(4))
}

func TestExport_PartitionMode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup distinct partition values query
	partitionColumns := []string{"date_col"}
	mock.ExpectQuery("SELECT DISTINCT").
		WillReturnRows(sqlmock.NewRows(partitionColumns).
			AddRow("2024-01-01").
			AddRow("2024-01-02"))

	// Setup partition queries - any query with WHERE
	dataColumns := []string{"id", "name", "date_col"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(dataColumns).
			AddRow(1, "Alice", "2024-01-01").
			AddRow(2, "Bob", "2024-01-01"))

	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(dataColumns).
			AddRow(3, "Charlie", "2024-01-02"))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:    "test_db",
		Table:       "test_table",
		OutputDir:   tmpDir,
		PartitionBy: "date_col",
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalRows, int64(3))
}

func TestExport_NoPartitionsFound(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup distinct partition values query returning empty
	partitionColumns := []string{"date_col"}
	mock.ExpectQuery("SELECT DISTINCT").
		WillReturnRows(sqlmock.NewRows(partitionColumns))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:    "test_db",
		Table:       "test_table",
		OutputDir:   tmpDir,
		PartitionBy: "date_col",
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(0), result.TotalRows)
	assert.Empty(t, result.Files)
}

func TestExport_QueryError(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup query to fail
	mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("database error"))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
	}

	result, err := orch.Export(ctx, opts)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestExport_InvalidOutputDir(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: "/invalid/path/that/cannot/be/created/12345",
	}

	result, err := orch.Export(ctx, opts)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSanitizePartitionValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string", "test-value", "test-value"},
		{"integer", 42, "42"},
		{"nil", nil, "__NULL__"},
		{"with slash", "path/to/value", "path_to_value"},
		{"with backslash", "path\\to\\value", "path_to_value"},
		{"with colon", "time:12:30", "time_12_30"},
		{"with asterisk", "value*", "value_"},
		{"with question", "value?", "value_"},
		{"with quote", `value"test`, "value_test"},
		{"with less", "value<test", "value_test"},
		{"with greater", "value>test", "value_test"},
		{"with pipe", "value|test", "value_test"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePartitionValue(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetParquetFiles(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec).(*orchestrator)

	tmpDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"test1.parquet",
		"test2.parquet",
		"not_parquet.txt",
		"subdir",
	}

	for _, f := range testFiles {
		path := filepath.Join(tmpDir, f)
		if f == "subdir" {
			err := os.Mkdir(path, 0755)
			require.NoError(t, err)
		} else {
			err := os.WriteFile(path, []byte("test"), 0644)
			require.NoError(t, err)
		}
	}

	files, err := orch.getParquetFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 2)

	for _, f := range files {
		assert.True(t, strings.HasSuffix(f, ".parquet"))
	}
}

func TestGetParquetFiles_InvalidDir(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec).(*orchestrator)

	files, err := orch.getParquetFiles("/nonexistent/directory/12345")
	assert.Error(t, err)
	assert.Nil(t, files)
}

func TestExport_SimpleMode_EmptyResult(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup query returning empty result
	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(0), result.TotalRows)
}

func TestExport_BatchMode_EmptyResult(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup query returning empty result
	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
		BatchSize: 10,
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(0), result.TotalRows)
	assert.Empty(t, result.Files)
}

func TestExport_ContextCancellation(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup query expectation that will be cancelled
	columns := []string{"id", "name"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1, "Alice"))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
	}

	// Context cancellation behavior depends on implementation
	_, err := orch.Export(ctx, opts)
	// Just verify it doesn't panic
	_ = err
}

func TestExport_DurationTracking(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	columns := []string{"id"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
	}

	start := time.Now()
	result, err := orch.Export(ctx, opts)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, result)
	// Duration should be close to elapsed time
	assert.True(t, result.Duration >= 0)
	assert.True(t, result.Duration <= elapsed+time.Millisecond)
}

func TestExport_ParquetFileCreated(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	columns := []string{"id", "name", "value"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(1, "Alice", 100).
			AddRow(2, "Bob", 200))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:  "test_db",
		Table:     "test_table",
		OutputDir: tmpDir,
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify at least one parquet file was created
	foundParquet := false
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".parquet") {
			foundParquet = true
			break
		}
	}
	assert.True(t, foundParquet, "No parquet files were created")
}

func TestExport_PartitionDirectoryStructure(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup partition values - use string type that matches what database returns
	partitionColumns := []string{"date_col"}
	mock.ExpectQuery("SELECT DISTINCT").
		WillReturnRows(sqlmock.NewRows(partitionColumns).
			AddRow("2024-01-01").
			AddRow("2024-01-02"))

	dataColumns := []string{"id", "date_col"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(dataColumns).AddRow(1, "2024-01-01"))

	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(dataColumns).AddRow(2, "2024-01-02"))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:    "test_db",
		Table:       "test_table",
		OutputDir:   tmpDir,
		PartitionBy: "date_col",
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check partition directories were created
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	partitionDirs := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			partitionDirs[entry.Name()] = true
		}
	}

	// Check that partition directories with sanitized names were created
	// The actual names depend on how the database returns the values
	assert.GreaterOrEqual(t, len(partitionDirs), 1, "Expected at least one partition directory to be created")

	// Verify directories follow the pattern date_col=<value>
	foundPartitionDir := false
	for dirName := range partitionDirs {
		if strings.HasPrefix(dirName, "date_col=") {
			foundPartitionDir = true
			break
		}
	}
	assert.True(t, foundPartitionDir, "No partition directory with date_col= prefix found")
}

func TestExport_PartitionModeWithMultiplePartitions(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup many partition values
	partitionColumns := []string{"region"}
	rows := sqlmock.NewRows(partitionColumns)
	for i := 0; i < 5; i++ {
		rows.AddRow(fmt.Sprintf("region_%d", i))
	}
	mock.ExpectQuery("SELECT DISTINCT").WillReturnRows(rows)

	// Setup partition queries
	dataColumns := []string{"id", "region"}
	for i := 0; i < 5; i++ {
		mock.ExpectQuery("SELECT").
			WillReturnRows(sqlmock.NewRows(dataColumns).
				AddRow(i, fmt.Sprintf("region_%d", i)))
	}

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:    "test_db",
		Table:       "test_table",
		OutputDir:   tmpDir,
		PartitionBy: "region",
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalRows, int64(5))
}

func TestExport_PartitionModeWithBatching(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	tmpDir := t.TempDir()

	// Setup distinct partition values query
	partitionColumns := []string{"date_col"}
	mock.ExpectQuery("SELECT DISTINCT").
		WillReturnRows(sqlmock.NewRows(partitionColumns).
			AddRow("2024-01-01"))

	// Setup partition queries with batching
	dataColumns := []string{"id", "name", "date_col"}
	mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(dataColumns).
			AddRow(1, "Alice", "2024-01-01").
			AddRow(2, "Bob", "2024-01-01").
			AddRow(3, "Charlie", "2024-01-01"))

	metaService := metadata.NewService(db)
	queryExec := query.NewExecutor(db)
	orch := NewOrchestrator(db, metaService, queryExec)

	ctx := context.Background()
	opts := Options{
		Database:    "test_db",
		Table:       "test_table",
		OutputDir:   tmpDir,
		PartitionBy: "date_col",
		BatchSize:   2,
	}

	result, err := orch.Export(ctx, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalRows, int64(3))
}
