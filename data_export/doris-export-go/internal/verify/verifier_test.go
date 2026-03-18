package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestParquetFile creates a temporary parquet file for testing
func createTestParquetFile(t *testing.T, dir string, name string, numRows int, columns map[string]string) string {
	t.Helper()

	filePath := filepath.Join(dir, name)
	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer file.Close()

	// Build schema from columns
	group := make(parquet.Group)
	for colName, colType := range columns {
		switch colType {
		case "int":
			group[colName] = parquet.Int(64)
		case "float":
			group[colName] = parquet.Leaf(parquet.DoubleType)
		case "bool":
			group[colName] = parquet.Leaf(parquet.BooleanType)
		default:
			group[colName] = parquet.String()
		}
	}

	schema := parquet.NewSchema("test", group)

	writer := parquet.NewWriter(file, &parquet.WriterConfig{
		Schema: schema,
	})

	// Write rows
	for i := 0; i < numRows; i++ {
		row := make(map[string]interface{})
		for colName, colType := range columns {
			switch colType {
			case "int":
				row[colName] = int64(i)
			case "float":
				row[colName] = float64(i) * 1.5
			case "bool":
				row[colName] = i%2 == 0
			default:
				row[colName] = "value_" + string(rune('a'+i%26))
			}
		}
		err := writer.Write(row)
		require.NoError(t, err)
	}

	require.NoError(t, writer.Close())

	return filePath
}

// createInvalidParquetFile creates a file with invalid parquet content
func createInvalidParquetFile(t *testing.T, dir string, name string) string {
	t.Helper()

	filePath := filepath.Join(dir, name)
	err := os.WriteFile(filePath, []byte("not a valid parquet file"), 0644)
	require.NoError(t, err)

	return filePath
}

func TestNewVerifier(t *testing.T) {
	v := NewVerifier()
	assert.NotNil(t, v)
}

func TestVerify_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	result, err := v.Verify(filepath.Join(tmpDir, "nonexistent.parquet"))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "file not found")
}

func TestVerify_InvalidParquetFormat(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	invalidFile := createInvalidParquetFile(t, tmpDir, "invalid.parquet")

	result, err := v.Verify(invalidFile)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "invalid parquet format")
}

func TestVerify_ValidParquetFile(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	columns := map[string]string{
		"id":    "int",
		"name":  "string",
		"score": "float",
	}
	filePath := createTestParquetFile(t, tmpDir, "valid.parquet", 100, columns)

	result, err := v.Verify(filePath)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid, "Expected file to be valid, got error: %s", result.Error)
	assert.Equal(t, int64(100), result.RowCount)
	assert.Equal(t, 3, result.ColumnCount)
	assert.Greater(t, result.SizeMB, float64(0))
	assert.Empty(t, result.Error)
}

func TestVerify_EmptyParquetFile(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	columns := map[string]string{
		"id":   "int",
		"name": "string",
	}
	filePath := createTestParquetFile(t, tmpDir, "empty.parquet", 0, columns)

	result, err := v.Verify(filePath)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(0), result.RowCount)
	assert.Equal(t, 2, result.ColumnCount)
}

func TestVerify_LargeParquetFile(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	columns := map[string]string{
		"id":     "int",
		"name":   "string",
		"value1": "float",
		"value2": "float",
		"active": "bool",
	}
	filePath := createTestParquetFile(t, tmpDir, "large.parquet", 10000, columns)

	result, err := v.Verify(filePath)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(10000), result.RowCount)
	assert.Equal(t, 5, result.ColumnCount)
}

func TestVerify_SingleColumn(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	columns := map[string]string{
		"data": "string",
	}
	filePath := createTestParquetFile(t, tmpDir, "single.parquet", 50, columns)

	result, err := v.Verify(filePath)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(50), result.RowCount)
	assert.Equal(t, 1, result.ColumnCount)
}

func TestVerifyBatch_AllValid(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	columns := map[string]string{
		"id":   "int",
		"name": "string",
	}

	files := []string{
		createTestParquetFile(t, tmpDir, "file1.parquet", 10, columns),
		createTestParquetFile(t, tmpDir, "file2.parquet", 20, columns),
		createTestParquetFile(t, tmpDir, "file3.parquet", 30, columns),
	}

	results, allValid := v.VerifyBatch(files)

	assert.True(t, allValid)
	assert.Len(t, results, 3)

	for i, result := range results {
		assert.True(t, result.Valid, "File %d should be valid", i)
		assert.Equal(t, files[i], result.FilePath)
	}

	// Verify individual results
	assert.Equal(t, int64(10), results[0].RowCount)
	assert.Equal(t, int64(20), results[1].RowCount)
	assert.Equal(t, int64(30), results[2].RowCount)
}

func TestVerifyBatch_SomeInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	columns := map[string]string{
		"id":   "int",
		"name": "string",
	}

	files := []string{
		createTestParquetFile(t, tmpDir, "valid.parquet", 10, columns),
		filepath.Join(tmpDir, "nonexistent.parquet"),
		createTestParquetFile(t, tmpDir, "valid2.parquet", 20, columns),
	}

	results, allValid := v.VerifyBatch(files)

	assert.False(t, allValid)
	assert.Len(t, results, 3)

	// First file should be valid
	assert.True(t, results[0].Valid)
	assert.Equal(t, int64(10), results[0].RowCount)

	// Second file should be invalid
	assert.False(t, results[1].Valid)
	assert.Contains(t, results[1].Error, "file not found")

	// Third file should be valid
	assert.True(t, results[2].Valid)
	assert.Equal(t, int64(20), results[2].RowCount)
}

func TestVerifyBatch_AllInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	files := []string{
		filepath.Join(tmpDir, "missing1.parquet"),
		filepath.Join(tmpDir, "missing2.parquet"),
	}

	results, allValid := v.VerifyBatch(files)

	assert.False(t, allValid)
	assert.Len(t, results, 2)

	for _, result := range results {
		assert.False(t, result.Valid)
		assert.Contains(t, result.Error, "file not found")
	}
}

func TestVerifyBatch_EmptySlice(t *testing.T) {
	v := NewVerifier()

	results, allValid := v.VerifyBatch([]string{})

	assert.True(t, allValid)
	assert.Empty(t, results)
}

func TestVerify_ResultFields(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewVerifier()

	columns := map[string]string{
		"id":      "int",
		"name":    "string",
		"email":   "string",
		"balance": "float",
	}
	filePath := createTestParquetFile(t, tmpDir, "test.parquet", 500, columns)

	result, err := v.Verify(filePath)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify all fields are populated correctly
	assert.Equal(t, filePath, result.FilePath)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(500), result.RowCount)
	assert.Equal(t, 4, result.ColumnCount)
	assert.Greater(t, result.SizeMB, float64(0))
	assert.Empty(t, result.Error)
}

func TestVerify_DirectoryInsteadOfFile(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	v := NewVerifier()

	result, err := v.Verify(subDir)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "not a regular file")
}
