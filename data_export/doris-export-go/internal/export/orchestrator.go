package export

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/user/doris-export-go/internal/metadata"
	"github.com/user/doris-export-go/internal/parquet"
	"github.com/user/doris-export-go/internal/query"
)

// Options configures the export operation
type Options struct {
	Database    string
	Table       string
	OutputDir   string
	BatchSize   int    // 0 = no batching
	WhereClause string // Optional WHERE clause
	PartitionBy string // Optional partition column
	Verify      bool   // Verify after export
}

// Result holds export results
type Result struct {
	Files     []string
	TotalRows int64
	Duration  time.Duration
}

// Orchestrator coordinates the export process
type Orchestrator interface {
	Export(ctx context.Context, opts Options) (*Result, error)
}

// orchestrator implements the Orchestrator interface
type orchestrator struct {
	db       *sql.DB
	metadata metadata.Service
	query    query.Executor
	logger   *logrus.Logger
}

// NewOrchestrator creates a new export orchestrator
func NewOrchestrator(db *sql.DB, metadata metadata.Service, query query.Executor) Orchestrator {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &orchestrator{
		db:       db,
		metadata: metadata,
		query:    query,
		logger:   logger,
	}
}

// Export performs the export operation
func (o *orchestrator) Export(ctx context.Context, opts Options) (*Result, error) {
	start := time.Now()
	o.logger.WithFields(logrus.Fields{
		"database":     opts.Database,
		"table":        opts.Table,
		"output_dir":   opts.OutputDir,
		"batch_size":   opts.BatchSize,
		"partition_by": opts.PartitionBy,
		"where_clause": opts.WhereClause,
	}).Info("Starting export")

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var result *Result
	var err error

	// Determine export mode
	if opts.PartitionBy != "" {
		result, err = o.exportPartitioned(ctx, opts)
	} else if opts.BatchSize > 0 {
		result, err = o.exportBatched(ctx, opts)
	} else {
		result, err = o.exportSimple(ctx, opts)
	}

	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)
	o.logger.WithFields(logrus.Fields{
		"files":      len(result.Files),
		"total_rows": result.TotalRows,
		"duration":   result.Duration,
	}).Info("Export completed")

	return result, nil
}

// exportSimple performs a simple export (single query, single file)
func (o *orchestrator) exportSimple(ctx context.Context, opts Options) (*Result, error) {
	o.logger.Info("Performing simple export")

	// Build query
	sqlQuery := fmt.Sprintf("SELECT * FROM `%s`.`%s`", opts.Database, opts.Table)
	if opts.WhereClause != "" {
		sqlQuery += " WHERE " + opts.WhereClause
	}

	// Execute query
	iter, err := o.query.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer iter.Close()

	// Create parquet writer
	writer, err := parquet.NewWriter(parquet.Config{
		OutputPath:         opts.OutputDir,
		TableName:          opts.Table,
		MaxRowsPerRowGroup: 1000000,
		EnableDict:         true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet writer: %w", err)
	}
	defer writer.Close()

	// Stream rows and write
	var totalRows int64
	var batch []map[string]interface{}
	batchSize := 1000 // Internal batch size for memory efficiency

	for iter.Next() {
		var row query.Row
		if err := iter.Scan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		batch = append(batch, row)
		totalRows++

		if len(batch) >= batchSize {
			if err := writer.WriteRows(batch); err != nil {
				return nil, fmt.Errorf("failed to write batch: %w", err)
			}
			batch = batch[:0]
		}
	}

	// Write remaining rows
	if len(batch) > 0 {
		if err := writer.WriteRows(batch); err != nil {
			return nil, fmt.Errorf("failed to write final batch: %w", err)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error during iteration: %w", err)
	}

	// Close writer to flush data
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	// Get the generated file path
	files, err := o.getParquetFiles(opts.OutputDir)
	if err != nil {
		return nil, err
	}

	o.logger.WithField("rows", totalRows).Info("Simple export completed")

	return &Result{
		Files:     files,
		TotalRows: totalRows,
	}, nil
}

// exportBatched performs a batched export (multiple files based on batch size)
func (o *orchestrator) exportBatched(ctx context.Context, opts Options) (*Result, error) {
	o.logger.WithField("batch_size", opts.BatchSize).Info("Performing batched export")

	// Build query
	sqlQuery := fmt.Sprintf("SELECT * FROM `%s`.`%s`", opts.Database, opts.Table)
	if opts.WhereClause != "" {
		sqlQuery += " WHERE " + opts.WhereClause
	}

	// Execute query with batch iterator
	batchIter, err := o.query.QueryBatch(ctx, sqlQuery, opts.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer batchIter.Close()

	var totalRows int64
	var fileCount int
	var files []string

	// Process batches
	for {
		batch, err := batchIter.NextBatch()
		if err != nil {
			return nil, fmt.Errorf("failed to get next batch: %w", err)
		}

		if len(batch) == 0 {
			break
		}

		// Convert query.Row to map[string]interface{}
		rows := make([]map[string]interface{}, len(batch))
		for i, row := range batch {
			rows[i] = row
		}

		// Create a new writer for each batch file
		writer, err := parquet.NewWriter(parquet.Config{
			OutputPath:         opts.OutputDir,
			TableName:          opts.Table,
			MaxRowsPerRowGroup: 1000000,
			EnableDict:         true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create parquet writer: %w", err)
		}

		if err := writer.WriteRows(rows); err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to write batch: %w", err)
		}

		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("failed to close writer: %w", err)
		}

		totalRows += int64(len(batch))
		fileCount++

		o.logger.WithFields(logrus.Fields{
			"batch": fileCount,
			"rows":  len(batch),
		}).Info("Exported batch")
	}

	// Get all generated files
	files, err = o.getParquetFiles(opts.OutputDir)
	if err != nil {
		return nil, err
	}

	o.logger.WithFields(logrus.Fields{
		"files": fileCount,
		"rows":  totalRows,
	}).Info("Batched export completed")

	return &Result{
		Files:     files,
		TotalRows: totalRows,
	}, nil
}

// exportPartitioned exports data by partition values
func (o *orchestrator) exportPartitioned(ctx context.Context, opts Options) (*Result, error) {
	o.logger.WithField("partition_column", opts.PartitionBy).Info("Performing partitioned export")

	// Get distinct partition values
	partitionValues, err := o.getPartitionValues(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get partition values: %w", err)
	}

	if len(partitionValues) == 0 {
		o.logger.Warn("No partition values found")
		return &Result{
			Files:     []string{},
			TotalRows: 0,
		}, nil
	}

	o.logger.WithField("partition_count", len(partitionValues)).Info("Found partition values")

	var mu sync.Mutex
	var allFiles []string
	var totalRows int64
	var errors []error

	// Export each partition
	for _, partitionValue := range partitionValues {
		// Sanitize partition value for directory name
		safeValue := sanitizePartitionValue(partitionValue)
		partitionDir := filepath.Join(opts.OutputDir, fmt.Sprintf("%s=%s", opts.PartitionBy, safeValue))

		if err := os.MkdirAll(partitionDir, 0755); err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to create partition directory for %s=%s: %w",
				opts.PartitionBy, partitionValue, err))
			mu.Unlock()
			continue
		}

		// Build partition query
		sqlQuery := fmt.Sprintf("SELECT * FROM `%s`.`%s` WHERE `%s` = ?",
			opts.Database, opts.Table, opts.PartitionBy)

		// Add WHERE clause if provided
		if opts.WhereClause != "" {
			sqlQuery += " AND (" + opts.WhereClause + ")"
		}

		var iter query.Iterator
		var err error

		if opts.BatchSize > 0 {
			// Batched export for this partition
			batchIter, err := o.query.QueryBatch(ctx, sqlQuery, opts.BatchSize, partitionValue)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to query partition %s=%s: %w",
					opts.PartitionBy, partitionValue, err))
				mu.Unlock()
				continue
			}
			err = o.exportPartitionBatched(batchIter, partitionDir, opts.Table, &allFiles, &totalRows, &mu)
			batchIter.Close()
		} else {
			// Simple export for this partition
			iter, err = o.query.Query(ctx, sqlQuery, partitionValue)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to query partition %s=%s: %w",
					opts.PartitionBy, partitionValue, err))
				mu.Unlock()
				continue
			}
			err = o.exportPartitionSimple(iter, partitionDir, opts.Table, &allFiles, &totalRows, &mu)
			iter.Close()
		}

		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("failed to export partition %s=%s: %w",
				opts.PartitionBy, partitionValue, err))
			mu.Unlock()
		}

		o.logger.WithFields(logrus.Fields{
			"partition": fmt.Sprintf("%s=%s", opts.PartitionBy, partitionValue),
		}).Info("Exported partition")
	}

	// Aggregate errors if any
	if len(errors) > 0 {
		var errMsgs []string
		for _, e := range errors {
			errMsgs = append(errMsgs, e.Error())
		}
		// Return partial result with errors
		o.logger.WithField("errors", len(errors)).Warn("Some partitions failed to export")
		return &Result{
			Files:     allFiles,
			TotalRows: totalRows,
		}, fmt.Errorf("partition export errors: %s", strings.Join(errMsgs, "; "))
	}

	o.logger.WithFields(logrus.Fields{
		"partitions": len(partitionValues),
		"files":      len(allFiles),
		"rows":       totalRows,
	}).Info("Partitioned export completed")

	return &Result{
		Files:     allFiles,
		TotalRows: totalRows,
	}, nil
}

// exportPartitionSimple exports a single partition using simple mode
func (o *orchestrator) exportPartitionSimple(
	iter query.Iterator,
	partitionDir string,
	tableName string,
	allFiles *[]string,
	totalRows *int64,
	mu *sync.Mutex,
) error {
	writer, err := parquet.NewWriter(parquet.Config{
		OutputPath:         partitionDir,
		TableName:          tableName,
		MaxRowsPerRowGroup: 1000000,
		EnableDict:         true,
	})
	if err != nil {
		return fmt.Errorf("failed to create parquet writer: %w", err)
	}
	defer writer.Close()

	var batch []map[string]interface{}
	batchSize := 1000
	var partitionRows int64

	for iter.Next() {
		var row query.Row
		if err := iter.Scan(&row); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		batch = append(batch, row)
		partitionRows++

		if len(batch) >= batchSize {
			if err := writer.WriteRows(batch); err != nil {
				return fmt.Errorf("failed to write batch: %w", err)
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := writer.WriteRows(batch); err != nil {
			return fmt.Errorf("failed to write final batch: %w", err)
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("error during iteration: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Get files from partition directory
	files, err := o.getParquetFiles(partitionDir)
	if err != nil {
		return err
	}

	mu.Lock()
	*allFiles = append(*allFiles, files...)
	*totalRows += partitionRows
	mu.Unlock()

	return nil
}

// exportPartitionBatched exports a single partition using batched mode
func (o *orchestrator) exportPartitionBatched(
	batchIter query.BatchIterator,
	partitionDir string,
	tableName string,
	allFiles *[]string,
	totalRows *int64,
	mu *sync.Mutex,
) error {
	for {
		batch, err := batchIter.NextBatch()
		if err != nil {
			return fmt.Errorf("failed to get next batch: %w", err)
		}

		if len(batch) == 0 {
			break
		}

		rows := make([]map[string]interface{}, len(batch))
		for i, row := range batch {
			rows[i] = row
		}

		writer, err := parquet.NewWriter(parquet.Config{
			OutputPath:         partitionDir,
			TableName:          tableName,
			MaxRowsPerRowGroup: 1000000,
			EnableDict:         true,
		})
		if err != nil {
			return fmt.Errorf("failed to create parquet writer: %w", err)
		}

		if err := writer.WriteRows(rows); err != nil {
			writer.Close()
			return fmt.Errorf("failed to write batch: %w", err)
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close writer: %w", err)
		}

		mu.Lock()
		*totalRows += int64(len(batch))
		mu.Unlock()
	}

	// Get files from partition directory
	files, err := o.getParquetFiles(partitionDir)
	if err != nil {
		return err
	}

	mu.Lock()
	*allFiles = append(*allFiles, files...)
	mu.Unlock()

	return nil
}

// getPartitionValues retrieves distinct values for the partition column
func (o *orchestrator) getPartitionValues(ctx context.Context, opts Options) ([]interface{}, error) {
	sqlQuery := fmt.Sprintf("SELECT DISTINCT `%s` FROM `%s`.`%s`",
		opts.PartitionBy, opts.Database, opts.Table)

	if opts.WhereClause != "" {
		sqlQuery += " WHERE " + opts.WhereClause
	}

	iter, err := o.query.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query partition values: %w", err)
	}
	defer iter.Close()

	var values []interface{}
	for iter.Next() {
		row := make(query.Row)
		if err := iter.Scan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan partition value: %w", err)
		}

		if val, ok := row[opts.PartitionBy]; ok {
			values = append(values, val)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("error during partition value iteration: %w", err)
	}

	return values, nil
}

// getParquetFiles returns all parquet files in a directory
func (o *orchestrator) getParquetFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".parquet") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files, nil
}

// sanitizePartitionValue sanitizes a partition value for use in directory names
func sanitizePartitionValue(value interface{}) string {
	if value == nil {
		return "__NULL__"
	}

	s := fmt.Sprintf("%v", value)
	// Replace problematic characters
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "*", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "\"", "_")
	s = strings.ReplaceAll(s, "<", "_")
	s = strings.ReplaceAll(s, ">", "_")
	s = strings.ReplaceAll(s, "|", "_")
	return s
}
