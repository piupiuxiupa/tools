package metadata

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/data-export-go/internal/config"
)

func setupMockDB(t *testing.T) (*sqlmock.Sqlmock, Service) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	svc := NewService(db, config.DBTypeMySQL)
	return &mock, svc
}

func TestNewService(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewService(db, config.DBTypeMySQL)
	assert.NotNil(t, svc)
}

func TestService_GetColumns(t *testing.T) {
	mock, svc := setupMockDB(t)

	// Test with multiple columns
	rows := sqlmock.NewRows([]string{"COLUMN_NAME"}).
		AddRow("id").
		AddRow("name").
		AddRow("email")

	(*mock).ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnRows(rows)

	columns, err := svc.GetColumns(context.Background(), "testdb", "users")
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "email"}, columns)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetColumns_EmptyTable(t *testing.T) {
	mock, svc := setupMockDB(t)

	rows := sqlmock.NewRows([]string{"COLUMN_NAME"})

	(*mock).ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "empty").
		WillReturnRows(rows)

	columns, err := svc.GetColumns(context.Background(), "testdb", "empty")
	require.NoError(t, err)
	assert.Empty(t, columns)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetColumns_QueryError(t *testing.T) {
	mock, svc := setupMockDB(t)

	(*mock).ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnError(errors.New("connection refused"))

	columns, err := svc.GetColumns(context.Background(), "testdb", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query columns")
	assert.Nil(t, columns)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetRowCount(t *testing.T) {
	mock, svc := setupMockDB(t)

	rows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1000)

	(*mock).ExpectQuery("SELECT COUNT\\(\\*\\) FROM `testdb`.`users`").
		WillReturnRows(rows)

	count, err := svc.GetRowCount(context.Background(), "testdb", "users")
	require.NoError(t, err)
	assert.Equal(t, int64(1000), count)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetRowCount_ZeroRows(t *testing.T) {
	mock, svc := setupMockDB(t)

	rows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0)

	(*mock).ExpectQuery("SELECT COUNT\\(\\*\\) FROM `testdb`.`empty`").
		WillReturnRows(rows)

	count, err := svc.GetRowCount(context.Background(), "testdb", "empty")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetRowCount_Error(t *testing.T) {
	mock, svc := setupMockDB(t)

	(*mock).ExpectQuery("SELECT COUNT\\(\\*\\) FROM `testdb`.`users`").
		WillReturnError(errors.New("table not found"))

	count, err := svc.GetRowCount(context.Background(), "testdb", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get row count")
	assert.Equal(t, int64(0), count)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetTableSize_WithData(t *testing.T) {
	mock, svc := setupMockDB(t)

	rows := sqlmock.NewRows([]string{"ROUND(SUM(data_length) / 1024 / 1024, 2)"}).AddRow(15.5)

	(*mock).ExpectQuery(`SELECT ROUND\(SUM\(data_length\) / 1024 / 1024, 2\) FROM information_schema.tables WHERE table_schema = \? AND table_name = \?`).
		WithArgs("testdb", "users").
		WillReturnRows(rows)

	size, err := svc.GetTableSize(context.Background(), "testdb", "users")
	require.NoError(t, err)
	assert.Equal(t, 15.5, size)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetTableSize_Null(t *testing.T) {
	mock, svc := setupMockDB(t)

	rows := sqlmock.NewRows([]string{"ROUND(SUM(data_length) / 1024 / 1024, 2)"}).AddRow(nil)

	(*mock).ExpectQuery(`SELECT ROUND\(SUM\(data_length\) / 1024 / 1024, 2\) FROM information_schema.tables WHERE table_schema = \? AND table_name = \?`).
		WithArgs("testdb", "empty").
		WillReturnRows(rows)

	size, err := svc.GetTableSize(context.Background(), "testdb", "empty")
	require.NoError(t, err)
	assert.Equal(t, 0.0, size)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetTableSize_Error(t *testing.T) {
	mock, svc := setupMockDB(t)

	(*mock).ExpectQuery(`SELECT ROUND\(SUM\(data_length\) / 1024 / 1024, 2\) FROM information_schema.tables WHERE table_schema = \? AND table_name = \?`).
		WithArgs("testdb", "users").
		WillReturnError(errors.New("permission denied"))

	size, err := svc.GetTableSize(context.Background(), "testdb", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get table size")
	assert.Equal(t, 0.0, size)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetTableMetadata(t *testing.T) {
	mock, svc := setupMockDB(t)

	// Columns query
	columnRows := sqlmock.NewRows([]string{"COLUMN_NAME"}).
		AddRow("id").
		AddRow("name")
	(*mock).ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnRows(columnRows)

	// Row count query
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(500)
	(*mock).ExpectQuery("SELECT COUNT\\(\\*\\) FROM `testdb`.`users`").
		WillReturnRows(countRows)

	// Table size query
	sizeRows := sqlmock.NewRows([]string{"ROUND(SUM(data_length) / 1024 / 1024, 2)"}).AddRow(10.0)
	(*mock).ExpectQuery(`SELECT ROUND\(SUM\(data_length\) / 1024 / 1024, 2\) FROM information_schema.tables WHERE table_schema = \? AND table_name = \?`).
		WithArgs("testdb", "users").
		WillReturnRows(sizeRows)

	metadata, err := svc.GetTableMetadata(context.Background(), "testdb", "users")
	require.NoError(t, err)
	assert.Equal(t, "testdb", metadata.Database)
	assert.Equal(t, "users", metadata.Table)
	assert.Equal(t, []string{"id", "name"}, metadata.Columns)
	assert.Equal(t, 2, metadata.ColumnCount)
	assert.Equal(t, int64(500), metadata.RowCount)
	assert.Equal(t, 10.0, metadata.DataSizeMB)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetTableMetadata_ColumnsError(t *testing.T) {
	mock, svc := setupMockDB(t)

	(*mock).ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnError(errors.New("table not found"))

	metadata, err := svc.GetTableMetadata(context.Background(), "testdb", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get columns")
	assert.Nil(t, metadata)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetTableMetadata_RowCountError(t *testing.T) {
	mock, svc := setupMockDB(t)

	// Columns query succeeds
	columnRows := sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("id")
	(*mock).ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnRows(columnRows)

	// Row count query fails
	(*mock).ExpectQuery("SELECT COUNT\\(\\*\\) FROM `testdb`.`users`").
		WillReturnError(errors.New("table does not exist"))

	metadata, err := svc.GetTableMetadata(context.Background(), "testdb", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get row count")
	assert.Nil(t, metadata)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_GetTableMetadata_TableSizeError(t *testing.T) {
	mock, svc := setupMockDB(t)

	// Columns query succeeds
	columnRows := sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("id")
	(*mock).ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnRows(columnRows)

	// Row count query succeeds
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(100)
	(*mock).ExpectQuery("SELECT COUNT\\(\\*\\) FROM `testdb`.`users`").
		WillReturnRows(countRows)

	// Table size query fails
	(*mock).ExpectQuery(`SELECT ROUND\(SUM\(data_length\) / 1024 / 1024, 2\) FROM information_schema.tables WHERE table_schema = \? AND table_name = \?`).
		WithArgs("testdb", "users").
		WillReturnError(errors.New("access denied"))

	metadata, err := svc.GetTableMetadata(context.Background(), "testdb", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get table size")
	assert.Nil(t, metadata)
	require.NoError(t, (*mock).ExpectationsWereMet())
}

func TestService_ContextCancellation_GetColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewService(db, config.DBTypeMySQL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mock.ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnError(context.Canceled)

	columns, err := svc.GetColumns(ctx, "testdb", "users")
	assert.Error(t, err)
	assert.Nil(t, columns)
}

func TestService_ContextTimeout_GetRowCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewService(db, config.DBTypeMySQL)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure timeout

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM `testdb`.`users`").
		WillReturnError(context.DeadlineExceeded)

	count, err := svc.GetRowCount(ctx, "testdb", "users")
	assert.Error(t, err)
	assert.Equal(t, int64(0), count)
}

func TestService_GetTableSize_WithNullFloat64(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewService(db, config.DBTypeMySQL)

	// Create a null value using driver.Value
	nullValue := driver.Value(nil)
	rows := sqlmock.NewRows([]string{"ROUND(SUM(data_length) / 1024 / 1024, 2)"}).AddRow(nullValue)

	mock.ExpectQuery(`SELECT ROUND\(SUM\(data_length\) / 1024 / 1024, 2\) FROM information_schema.tables WHERE table_schema = \? AND table_name = \?`).
		WithArgs("testdb", "new_table").
		WillReturnRows(rows)

	size, err := svc.GetTableSize(context.Background(), "testdb", "new_table")
	require.NoError(t, err)
	assert.Equal(t, 0.0, size)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestService_GetColumns_IterationError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewService(db, config.DBTypeMySQL)

	rows := sqlmock.NewRows([]string{"COLUMN_NAME"}).
		AddRow("id").
		RowError(0, errors.New("scan error"))

	mock.ExpectQuery(`SELECT column_name FROM information_schema.columns WHERE table_schema = \? AND table_name = \? ORDER BY ordinal_position`).
		WithArgs("testdb", "users").
		WillReturnRows(rows)

	columns, err := svc.GetColumns(context.Background(), "testdb", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error iterating columns")
	assert.Nil(t, columns)
	require.NoError(t, mock.ExpectationsWereMet())
}
