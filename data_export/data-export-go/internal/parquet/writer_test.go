package parquet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWriter(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		BatchSize:   1000,
		TableName:   "test_table",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)
	assert.NotNil(t, writer)

	err = writer.Close()
	assert.NoError(t, err)
}

func TestNewWriterDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath: tmpDir,
		// Leave other fields empty to test defaults
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)
	assert.NotNil(t, writer)

	// Access internal writer to check defaults
	pw := writer.(*parquetWriter)
	assert.Equal(t, "snappy", pw.config.Compression)
	assert.Equal(t, 10000, pw.config.BatchSize)
	assert.Equal(t, "export", pw.config.TableName)

	err = writer.Close()
	assert.NoError(t, err)
}

func TestNewWriterInvalidPath(t *testing.T) {
	config := Config{
		OutputPath: "/invalid/path/that/cannot/be/created",
	}

	_, err := NewWriter(config)
	assert.Error(t, err)
}

func TestWriteRows(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		BatchSize:   1000,
		TableName:   "test_table",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)
	defer writer.Close()

	rows := []map[string]interface{}{
		{"id": int64(1), "name": "Alice", "score": 95.5, "active": true},
		{"id": int64(2), "name": "Bob", "score": 87.3, "active": false},
		{"id": int64(3), "name": "Charlie", "score": 92.0, "active": true},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	// Close to flush data
	err = writer.Close()
	require.NoError(t, err)

	// Check file was created
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	assert.NotEmpty(t, fileInfo.Path)
	assert.Equal(t, int64(3), fileInfo.RowCount)
	assert.Greater(t, fileInfo.SizeBytes, int64(0))

	// Verify file exists
	_, err = os.Stat(fileInfo.Path)
	assert.NoError(t, err)
}

func TestWriteRowsVariousTypes(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_types",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)
	defer writer.Close()

	now := time.Now()
	rows := []map[string]interface{}{
		{
			"int_val":       int(42),
			"int8_val":      int8(8),
			"int16_val":     int16(16),
			"int32_val":     int32(32),
			"int64_val":     int64(64),
			"uint_val":      uint(100),
			"float32_val":   float32(3.14),
			"float64_val":   float64(2.718),
			"string_val":    "test string",
			"bool_val":      true,
			"timestamp_val": now,
		},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Verify file was created
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	assert.Equal(t, int64(1), fileInfo.RowCount)

	// Read back and verify
	file, err := os.Open(fileInfo.Path)
	require.NoError(t, err)
	defer file.Close()

	reader := parquet.NewReader(file)
	assert.Equal(t, int64(1), reader.NumRows())

	// Verify schema
	schema := reader.Schema()
	assert.NotNil(t, schema)
}

func TestWriteRowsEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_empty",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)
	defer writer.Close()

	// Write empty batch
	err = writer.WriteRows([]map[string]interface{}{})
	assert.NoError(t, err)

	// Close without writing any data
	err = writer.Close()
	assert.NoError(t, err)
}

func TestWriteRowsMultipleBatches(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		BatchSize:   1000,
		TableName:   "test_batches",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)
	defer writer.Close()

	// Write first batch
	rows1 := []map[string]interface{}{
		{"id": int64(1), "value": "first"},
		{"id": int64(2), "value": "second"},
	}
	err = writer.WriteRows(rows1)
	require.NoError(t, err)

	// Write second batch
	rows2 := []map[string]interface{}{
		{"id": int64(3), "value": "third"},
	}
	err = writer.WriteRows(rows2)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Verify total rows
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	assert.Equal(t, int64(3), fileInfo.RowCount)
}

func TestFileNaming(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:   tmpDir,
		Compression:  "snappy",
		DatabaseName: "testdb",
		TableName:    "my_table",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)
	defer writer.Close()

	// Write some data to create a file
	rows := []map[string]interface{}{
		{"id": int64(1)},
	}
	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Check file naming pattern
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	filename := filepath.Base(fileInfo.Path)

	// Should contain database name and table name in format: dbname__table__timestamp.parquet
	assert.True(t, strings.HasPrefix(filename, "testdb__my_table__"), "filename should start with testdb__my_table__")
	// Should NOT contain batch suffix (no BatchNumber set)
	assert.NotContains(t, filename, "batch", "non-batch export should not contain 'batch'")
	// Should have .parquet extension
	assert.True(t, strings.HasSuffix(filename, ".parquet"))
}

func TestCompressionSnappy(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_snappy",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{"data": strings.Repeat("a", 1000)},
		{"data": strings.Repeat("b", 1000)},
		{"data": strings.Repeat("c", 1000)},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// File should be created successfully
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	_, err = os.Stat(fileInfo.Path)
	assert.NoError(t, err)
}

func TestCompressionGzip(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "gzip",
		TableName:   "test_gzip",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{"data": strings.Repeat("a", 1000)},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// File should be created successfully
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	_, err = os.Stat(fileInfo.Path)
	assert.NoError(t, err)
}

func TestCompressionZstd(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "zstd",
		TableName:   "test_zstd",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{"data": strings.Repeat("a", 1000)},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// File should be created successfully
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	_, err = os.Stat(fileInfo.Path)
	assert.NoError(t, err)
}

func TestCompressionNone(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "none",
		TableName:   "test_none",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{"data": strings.Repeat("a", 1000)},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// File should be created successfully
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	_, err = os.Stat(fileInfo.Path)
	assert.NoError(t, err)
}

func TestCloseFlushesData(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_flush",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{"id": int64(1), "name": "test"},
		{"id": int64(2), "name": "test2"},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	// Close should flush data
	err = writer.Close()
	require.NoError(t, err)

	// Try to close again (should be idempotent)
	err = writer.Close()
	assert.NoError(t, err)

	// Read back the data
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()

	file, err := os.Open(fileInfo.Path)
	require.NoError(t, err)
	defer file.Close()

	reader := parquet.NewReader(file)
	assert.Equal(t, int64(2), reader.NumRows())
}

func TestWriteAfterClose(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_close",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	rows := []map[string]interface{}{
		{"id": int64(1)},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Try to write after close
	err = writer.WriteRows(rows)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestReadBackData(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_readback",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	// Write data with various types
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	rows := []map[string]interface{}{
		{"id": int64(1), "name": "Alice", "score": 95.5, "active": true, "created_at": now},
		{"id": int64(2), "name": "Bob", "score": 87.3, "active": false, "created_at": now},
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Read back the data
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()

	file, err := os.Open(fileInfo.Path)
	require.NoError(t, err)
	defer file.Close()

	reader := parquet.NewReader(file)
	assert.Equal(t, int64(2), reader.NumRows())

	// Read all rows
	type Row struct {
		ID        int64   `parquet:"id"`
		Name      string  `parquet:"name"`
		Score     float64 `parquet:"score"`
		Active    bool    `parquet:"active"`
		CreatedAt int64   `parquet:"created_at"`
	}

	var readRows []Row
	for {
		var row Row
		err := reader.Read(&row)
		if err != nil {
			break
		}
		readRows = append(readRows, row)
	}

	assert.Len(t, readRows, 2)
	assert.Equal(t, int64(1), readRows[0].ID)
	assert.Equal(t, "Alice", readRows[0].Name)
	assert.Equal(t, 95.5, readRows[0].Score)
	assert.Equal(t, true, readRows[0].Active)
}

func TestSchemaInference(t *testing.T) {
	columns := []string{"id", "name", "score", "active", "created_at"}
	sampleRow := map[string]interface{}{
		"id":         int64(1),
		"name":       "test",
		"score":      95.5,
		"active":     true,
		"created_at": time.Now(),
	}

	schema := InferSchema(columns, sampleRow)
	require.NotNil(t, schema)
	assert.Len(t, schema.Columns, 5)

	// Verify column types
	colMap := make(map[string]string)
	for _, col := range schema.Columns {
		colMap[col.Name] = col.Type
	}

	assert.Equal(t, "INT64", colMap["id"])
	assert.Equal(t, "STRING", colMap["name"])
	assert.Equal(t, "DOUBLE", colMap["score"])
	assert.Equal(t, "BOOLEAN", colMap["active"])
	assert.Equal(t, "TIMESTAMP", colMap["created_at"])
}

func TestInferType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"int", int(1), "INT64"},
		{"int8", int8(1), "INT64"},
		{"int16", int16(1), "INT64"},
		{"int32", int32(1), "INT64"},
		{"int64", int64(1), "INT64"},
		{"uint", uint(1), "INT64"},
		{"uint8", uint8(1), "INT64"},
		{"uint16", uint16(1), "INT64"},
		{"uint32", uint32(1), "INT64"},
		{"uint64", uint64(1), "INT64"},
		{"float32", float32(1.5), "DOUBLE"},
		{"float64", float64(1.5), "DOUBLE"},
		{"string", "test", "STRING"},
		{"bool", true, "BOOLEAN"},
		{"time", time.Now(), "TIMESTAMP"},
		{"nil", nil, "STRING"},
		{"pointer to int", func() *int { i := 42; return &i }(), "INT64"},
		{"unknown", struct{}{}, "STRING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferType(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToParquetValue(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		targetType string
		expected   interface{}
		wantErr    bool
	}{
		{"int to int64", int(42), "INT64", int64(42), false},
		{"int64 to int64", int64(42), "INT64", int64(42), false},
		{"string to int64", "42", "INT64", int64(42), false},
		{"float64 to double", float64(3.14), "DOUBLE", float64(3.14), false},
		{"float32 to double", float32(3.5), "DOUBLE", float64(3.5), false},
		{"int to double", int(42), "DOUBLE", float64(42), false},
		{"string to double", "3.14", "DOUBLE", float64(3.14), false},
		{"string to string", "hello", "STRING", "hello", false},
		{"int to string", 123, "STRING", "123", false},
		{"bool to bool", true, "BOOLEAN", true, false},
		{"int to bool (true)", 1, "BOOLEAN", true, false},
		{"int to bool (false)", 0, "BOOLEAN", false, false},
		{"string to bool (true)", "true", "BOOLEAN", true, false},
		{"string to bool (1)", "1", "BOOLEAN", true, false},
		{"time to timestamp", time.Unix(1234567890, 0), "TIMESTAMP", int64(1234567890000), false},
		{"int64 to timestamp", int64(1234567890000), "TIMESTAMP", int64(1234567890000), false},
		{"nil to any", nil, "INT64", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToParquetValue(tt.value, tt.targetType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetCompressionCodec(t *testing.T) {
	tests := []struct {
		name  string
		codec string
		check func(t *testing.T, result interface{})
	}{
		{"snappy", "snappy", func(t *testing.T, result interface{}) {
			assert.Equal(t, &parquet.Snappy, result)
		}},
		{"gzip", "gzip", func(t *testing.T, result interface{}) {
			assert.Equal(t, &parquet.Gzip, result)
		}},
		{"zstd", "zstd", func(t *testing.T, result interface{}) {
			assert.Equal(t, &parquet.Zstd, result)
		}},
		{"none", "none", func(t *testing.T, result interface{}) {
			assert.Equal(t, &parquet.Uncompressed, result)
		}},
		{"empty", "", func(t *testing.T, result interface{}) {
			assert.Equal(t, &parquet.Uncompressed, result)
		}},
		{"unknown", "unknown", func(t *testing.T, result interface{}) {
			assert.Equal(t, &parquet.Snappy, result)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCompressionCodec(tt.codec)
			tt.check(t, result)
		})
	}
}

func TestSchemaToParquetSchema(t *testing.T) {
	schema := NewSchema()
	schema.AddColumn("id", "INT64")
	schema.AddColumn("name", "STRING")
	schema.AddColumn("score", "DOUBLE")

	parquetSchema := schema.ToParquetSchema()
	assert.NotNil(t, parquetSchema)
}

func TestParquetWriterWithNullValues(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_nulls",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	// Write data with some nil values
	rows := []map[string]interface{}{
		{"id": int64(1), "name": "Alice", "optional": "value"},
		{"id": int64(2), "name": "Bob", "optional": nil},
		{"id": int64(3), "name": "Charlie"}, // missing optional field
	}

	err = writer.WriteRows(rows)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Verify file was created
	pw := writer.(*parquetWriter)
	fileInfo := pw.GetFileInfo()
	assert.Equal(t, int64(3), fileInfo.RowCount)
}

func TestGetFileInfo(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "test_fileinfo",
	}

	writer, err := NewWriter(config)
	require.NoError(t, err)

	pw := writer.(*parquetWriter)
	info1 := pw.GetFileInfo()
	assert.Empty(t, info1.Path)
	assert.Equal(t, int64(0), info1.RowCount)

	rows := []map[string]interface{}{
		{"id": int64(1), "name": "test"},
	}
	err = writer.WriteRows(rows)
	require.NoError(t, err)

	info2 := pw.GetFileInfo()
	assert.NotEmpty(t, info2.Path)
	assert.Equal(t, int64(1), info2.RowCount)
	assert.Equal(t, 2, info2.ColumnCount)

	err = writer.Close()
	require.NoError(t, err)

	info3 := pw.GetFileInfo()
	assert.Greater(t, info3.SizeBytes, int64(0))
}

func TestConvertToParquetValueErrors(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		targetType string
	}{
		{"invalid string to int64", "not-a-number", "INT64"},
		{"struct to int64", struct{}{}, "INT64"},
		{"struct to double", struct{}{}, "DOUBLE"},
		{"struct to bool", struct{}{}, "BOOLEAN"},
		{"struct to timestamp", struct{}{}, "TIMESTAMP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertToParquetValue(tt.value, tt.targetType)
			assert.Error(t, err)
		})
	}
}

func TestInferSchemaMissingColumns(t *testing.T) {
	columns := []string{"id", "name", "missing"}
	sampleRow := map[string]interface{}{
		"id":   int64(1),
		"name": "test",
	}

	schema := InferSchema(columns, sampleRow)
	require.NotNil(t, schema)
	assert.Len(t, schema.Columns, 3)

	colMap := make(map[string]string)
	for _, col := range schema.Columns {
		colMap[col.Name] = col.Type
	}
	assert.Equal(t, "INT64", colMap["id"])
	assert.Equal(t, "STRING", colMap["name"])
	assert.Equal(t, "STRING", colMap["missing"])
}

func BenchmarkWriteRows(b *testing.B) {
	tmpDir := b.TempDir()

	config := Config{
		OutputPath:  tmpDir,
		Compression: "snappy",
		TableName:   "bench",
	}

	writer, err := NewWriter(config)
	require.NoError(b, err)
	defer writer.Close()

	// Prepare batch
	batchSize := 1000
	rows := make([]map[string]interface{}, batchSize)
	for i := 0; i < batchSize; i++ {
		rows[i] = map[string]interface{}{
			"id":     int64(i),
			"name":   "test name",
			"value":  float64(i) * 1.5,
			"active": i%2 == 0,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := writer.WriteRows(rows)
		require.NoError(b, err)
	}
}
