package parquet

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateFormatUnix(t *testing.T) {
	tmpDir := t.TempDir()
	testTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	writer, err := NewWriter(Config{
		OutputPath: tmpDir,
		TableName:  "test",
		DateFormat: DateFormatUnix,
	})
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{
			"id":         1,
			"created_at": testTime,
		},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	// Verify the file was created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	t.Logf("Unix format test passed, file created: %s", files[0].Name())
}

func TestDateFormatISO(t *testing.T) {
	tmpDir := t.TempDir()
	testTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	writer, err := NewWriter(Config{
		OutputPath: tmpDir,
		TableName:  "test",
		DateFormat: DateFormatISO,
	})
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{
			"id":         1,
			"created_at": testTime,
		},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	// Verify the file was created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	t.Logf("ISO format test passed, file created: %s", files[0].Name())
}

func TestDateFormatStringWithCustomLayout(t *testing.T) {
	tmpDir := t.TempDir()
	testTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	// Test with custom layout
	writer, err := NewWriter(Config{
		OutputPath:     tmpDir,
		TableName:      "test",
		DateFormat:     DateFormatString,
		DateTimeLayout: "2006-01-02",
	})
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{
			"id":         1,
			"created_at": testTime,
		},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	// Verify the file was created
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	t.Logf("String format with custom layout test passed, file created: %s", files[0].Name())
}

func TestDateFormatInference(t *testing.T) {
	testTime := time.Now()

	// Test unix format inference - should return Int(64)
	node := inferParquetNode(testTime, DateFormatUnix)
	assert.NotNil(t, node)

	// Test iso format inference - should return String()
	node = inferParquetNode(testTime, DateFormatISO)
	assert.NotNil(t, node)

	// Test string format inference - should return String()
	node = inferParquetNode(testTime, DateFormatString)
	assert.NotNil(t, node)

	// Test empty date format defaults to unix
	node = inferParquetNode(testTime, "")
	assert.NotNil(t, node)
}

func TestConvertRowForDateFormat(t *testing.T) {
	testTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	expectedISO := testTime.Format(time.RFC3339)
	expectedDefaultString := testTime.Format("2006-01-02 15:04:05")
	expectedCustomString := testTime.Format("2006-01-02")

	// Test with unix format - should not convert
	w := &parquetWriter{
		config: Config{DateFormat: DateFormatUnix},
	}
	row := map[string]interface{}{
		"id":         1,
		"created_at": testTime,
	}
	converted := w.convertRowForDateFormat(row)
	assert.Equal(t, testTime, converted["created_at"])

	// Test with iso format - should convert to RFC3339 string
	w = &parquetWriter{
		config: Config{DateFormat: DateFormatISO},
	}
	converted = w.convertRowForDateFormat(row)
	assert.Equal(t, expectedISO, converted["created_at"])

	// Test with string format (default layout) - should use SQL datetime format
	w = &parquetWriter{
		config: Config{DateFormat: DateFormatString},
	}
	converted = w.convertRowForDateFormat(row)
	assert.Equal(t, expectedDefaultString, converted["created_at"])

	// Test with string format (custom layout)
	w = &parquetWriter{
		config: Config{
			DateFormat:     DateFormatString,
			DateTimeLayout: "2006-01-02",
		},
	}
	converted = w.convertRowForDateFormat(row)
	assert.Equal(t, expectedCustomString, converted["created_at"])
}
