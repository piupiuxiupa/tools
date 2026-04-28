package parquet

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress"
)

// Writer defines the interface for writing data to Parquet files
type Writer interface {
	WriteRows(rows []map[string]interface{}) error
	Close() error
}

// DateFormat represents how date/time values should be exported
type DateFormat string

const (
	DateFormatUnix   DateFormat = "unix"   // Unix timestamp in milliseconds (INT64)
	DateFormatISO    DateFormat = "iso"    // ISO 8601 format string
	DateFormatString DateFormat = "string" // Original string format
)

// Config holds configuration for the Parquet writer
type Config struct {
	OutputPath         string
	Compression        string
	BatchSize          int
	DatabaseName       string
	TableName          string
	MaxRowsPerRowGroup int64      // Maximum rows per row group (default 1M)
	EnableDict         bool       // Enable dictionary encoding (default true)
	DateFormat         DateFormat // Date format: unix (default), iso, string
	DateTimeLayout     string     // Custom date format layout for string format (Go time layout)
	BatchNumber        int        // Current batch number (1-indexed), set to > 0 for batched exports
}

// FileInfo holds metadata about a written file
type FileInfo struct {
	Path        string
	RowCount    int64
	ColumnCount int
	SizeBytes   int64
}

type parquetWriter struct {
	config            Config
	file              *os.File
	writer            *parquet.Writer
	schema            *parquet.Schema
	columns           []string
	columnTargetTypes map[string]string // inferred target Go type per column
	fileInfo          FileInfo
	batchNum          int
	mu                sync.Mutex
	closed            bool
}

// NewWriter creates a new Parquet writer
func NewWriter(config Config) (Writer, error) {
	if config.Compression == "" {
		config.Compression = "snappy"
	}
	if config.BatchSize < 0 {
		config.BatchSize = 10000
	}
	if config.TableName == "" {
		config.TableName = "export"
	}
	if config.MaxRowsPerRowGroup <= 0 {
		config.MaxRowsPerRowGroup = 1000000
	}

	if err := os.MkdirAll(config.OutputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &parquetWriter{
		config:   config,
		batchNum: 1,
	}, nil
}

func (w *parquetWriter) generateFilename() string {
	timestamp := time.Now().Format("20060102_150405")

	dbName := w.config.DatabaseName
	if dbName == "" {
		dbName = "export"
	}
	tableName := w.config.TableName
	if tableName == "" {
		tableName = "data"
	}

	// Format: dbname__table__time__batchXXXX.parquet (if batched)
	//         dbname__table__time.parquet (if not batched)
	if w.config.BatchNumber > 0 {
		return fmt.Sprintf("%s__%s__%s__batch%04d.parquet", dbName, tableName, timestamp, w.config.BatchNumber)
	}
	return fmt.Sprintf("%s__%s__%s.parquet", dbName, tableName, timestamp)
}

func (w *parquetWriter) getFullPath() string {
	return filepath.Join(w.config.OutputPath, w.generateFilename())
}

func (w *parquetWriter) initializeWriter(columns []string, sampleRow map[string]interface{}) error {
	if w.file != nil {
		if err := w.closeCurrentFile(); err != nil {
			return err
		}
	}

	sort.Strings(columns)
	w.columns = columns

	filePath := w.getFullPath()
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	w.file = file
	w.fileInfo = FileInfo{
		Path:        filePath,
		ColumnCount: len(columns),
	}

	group := make(parquet.Group)
	w.columnTargetTypes = make(map[string]string, len(columns))
	for _, col := range columns {
		value := sampleRow[col]
		node := inferParquetNode(value, w.config.DateFormat)
		group[col] = node
		w.columnTargetTypes[col] = inferTargetType(value, w.config.DateFormat)
	}

	schema := parquet.NewSchema("record", group)
	w.schema = schema

	compression := getCompressionCodec(w.config.Compression)
	writerConfig := &parquet.WriterConfig{
		Schema:             schema,
		Compression:        compression,
		MaxRowsPerRowGroup: w.config.MaxRowsPerRowGroup,
	}
	w.writer = parquet.NewWriter(file, writerConfig)

	return nil
}

func inferParquetNode(value interface{}, dateFormat DateFormat) parquet.Node {
	if value == nil {
		return parquet.Optional(parquet.String())
	}

	switch v := value.(type) {
	case int8, int16, uint8, uint16:
		return parquet.Int(32)
	case int, int32, int64, uint, uint32, uint64:
		return parquet.Int(64)
	case float32, float64:
		return parquet.Leaf(parquet.DoubleType)
	case string:
		return parquet.String()
	case bool:
		return parquet.Leaf(parquet.BooleanType)
	case time.Time:
		return inferDateTimeNode(dateFormat)
	default:
		if s, ok := v.(string); ok {
			if _, err := fmt.Sscanf(s, "%d", new(int64)); err == nil {
				return parquet.Int(64)
			}
			if _, err := fmt.Sscanf(s, "%f", new(float64)); err == nil {
				return parquet.Leaf(parquet.DoubleType)
			}
		}
		return parquet.String()
	}
}

func inferDateTimeNode(dateFormat DateFormat) parquet.Node {
	switch dateFormat {
	case DateFormatISO, DateFormatString:
		return parquet.String()
	case DateFormatUnix, "":
		return parquet.Int(64)
	default:
		return parquet.Int(64)
	}
}

func inferTargetType(value interface{}, dateFormat DateFormat) string {
	if value == nil {
		return "STRING"
	}
	switch value.(type) {
	case int8, int16, uint8, uint16:
		return "INT32"
	case int, int32, int64, uint, uint32, uint64:
		return "INT64"
	case float32, float64:
		return "DOUBLE"
	case string:
		return "STRING"
	case bool:
		return "BOOLEAN"
	case time.Time:
		switch dateFormat {
		case DateFormatISO, DateFormatString:
			return "STRING"
		default:
			return "INT64"
		}
	default:
		return "STRING"
	}
}

func convertValueToTarget(value interface{}, targetType string, config Config) interface{} {
	if value == nil {
		return nil
	}

	switch targetType {
	case "INT64":
		switch v := value.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case int32:
			return int64(v)
		case uint:
			return int64(v)
		case uint32:
			return int64(v)
		case uint64:
			return int64(v)
		case float64:
			return int64(v)
		case float32:
			return int64(v)
		case string:
			var result int64
			if _, err := fmt.Sscanf(v, "%d", &result); err == nil {
				return result
			}
			return int64(0)
		case time.Time:
			return v.UnixMilli()
		default:
			return int64(0)
		}
	case "INT32":
		switch v := value.(type) {
		case int32:
			return v
		case int:
			return int32(v)
		case int64:
			return int32(v)
		case int8:
			return int32(v)
		case int16:
			return int32(v)
		case uint8:
			return int32(v)
		case uint16:
			return int32(v)
		case float64:
			return int32(v)
		case string:
			var result int32
			if _, err := fmt.Sscanf(v, "%d", &result); err == nil {
				return result
			}
			return int32(0)
		default:
			return int32(0)
		}
	case "DOUBLE":
		switch v := value.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			var result float64
			if _, err := fmt.Sscanf(v, "%f", &result); err == nil {
				return result
			}
			return float64(0)
		default:
			return float64(0)
		}
	case "STRING":
		switch v := value.(type) {
		case string:
			return v
		case time.Time:
			layout := getDateTimeLayout(config)
			return v.Format(layout)
		default:
			return fmt.Sprintf("%v", v)
		}
	case "BOOLEAN":
		switch v := value.(type) {
		case bool:
			return v
		case int:
			return v != 0
		case int64:
			return v != 0
		case string:
			return v == "true" || v == "1" || v == "TRUE"
		default:
			return false
		}
	default:
		return fmt.Sprintf("%v", value)
	}
}

func getDateTimeLayout(config Config) string {
	switch config.DateFormat {
	case DateFormatISO:
		return time.RFC3339
	case DateFormatString:
		if config.DateTimeLayout != "" {
			return config.DateTimeLayout
		}
		return "2006-01-02 15:04:05"
	default:
		return time.RFC3339
	}
}

func (w *parquetWriter) convertRowToMatchSchema(row map[string]interface{}) map[string]interface{} {
	converted := make(map[string]interface{}, len(row))
	for k, v := range row {
		targetType, ok := w.columnTargetTypes[k]
		if !ok {
			converted[k] = v
			continue
		}
		converted[k] = convertValueToTarget(v, targetType, w.config)
	}
	return converted
}

func getCompressionCodec(name string) compress.Codec {
	switch name {
	case "snappy":
		return &parquet.Snappy
	case "gzip":
		return &parquet.Gzip
	case "zstd":
		return &parquet.Zstd
	case "none", "":
		return &parquet.Uncompressed
	default:
		return &parquet.Snappy
	}
}

func (w *parquetWriter) WriteRows(rows []map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	if len(rows) == 0 {
		return nil
	}

	if w.writer == nil {
		columns := make([]string, 0, len(rows[0]))
		for k := range rows[0] {
			columns = append(columns, k)
		}
		sort.Strings(columns)

		if err := w.initializeWriter(columns, rows[0]); err != nil {
			return err
		}
	}

	for _, row := range rows {
		convertedRow := w.convertRowToMatchSchema(row)
		parquetRow := w.schema.Deconstruct(nil, convertedRow)
		if _, err := w.writer.WriteRows([]parquet.Row{parquetRow}); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
		w.fileInfo.RowCount++
	}

	return nil
}


func (w *parquetWriter) closeCurrentFile() error {
	if w.writer != nil {
		if err := w.writer.Close(); err != nil {
			return fmt.Errorf("failed to close parquet writer: %w", err)
		}
		w.writer = nil
	}

	if w.file != nil {
		info, err := w.file.Stat()
		if err == nil {
			w.fileInfo.SizeBytes = info.Size()
		}

		if err := w.file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
		w.file = nil
	}

	w.batchNum++
	return nil
}

func (w *parquetWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	return w.closeCurrentFile()
}

func (w *parquetWriter) GetFileInfo() FileInfo {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		info, err := w.file.Stat()
		if err == nil {
			w.fileInfo.SizeBytes = info.Size()
		}
	}

	return w.fileInfo
}
