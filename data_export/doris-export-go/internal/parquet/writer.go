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

// Config holds configuration for the Parquet writer
type Config struct {
	OutputPath         string
	Compression        string
	BatchSize          int
	TableName          string
	MaxRowsPerRowGroup int64 // Maximum rows per row group (default 1M)
	EnableDict         bool  // Enable dictionary encoding (default true)
}

// FileInfo holds metadata about a written file
type FileInfo struct {
	Path        string
	RowCount    int64
	ColumnCount int
	SizeBytes   int64
}

type parquetWriter struct {
	config   Config
	file     *os.File
	writer   *parquet.Writer
	schema   *parquet.Schema
	columns  []string
	fileInfo FileInfo
	batchNum int
	mu       sync.Mutex
	closed   bool
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
	return fmt.Sprintf("%s_%s_batch%04d.parquet", w.config.TableName, timestamp, w.batchNum)
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
	for _, col := range columns {
		value := sampleRow[col]
		node := inferParquetNode(value)
		group[col] = node
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

func inferParquetNode(value interface{}) parquet.Node {
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
		return parquet.Timestamp(parquet.Millisecond)
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
		parquetRow := w.schema.Deconstruct(nil, row)
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
