package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/user/doris-export-go/internal/database"
	"github.com/user/doris-export-go/internal/export"
	"github.com/user/doris-export-go/internal/metadata"
	"github.com/user/doris-export-go/internal/query"
	"github.com/user/doris-export-go/internal/verify"
)

// Constants
const (
	LargeTableThreshold = 1000000 // 1 million rows
	DefaultPort         = 9030
)

// CLI flags
var (
	// Required flags
	host      string
	port      int
	user      string
	password  string
	dbName    string
	tableName string
	outputDir string

	// Optional flags
	batchSize   int
	where       string
	partitionBy string
	dateFormat  string
	infoOnly    bool
	noConfirm   bool
	verifyFlag  bool
	verifyOnly  []string
)

// NewRootCommand creates the root command
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "doris-export",
		Short: "Export Apache Doris data to Parquet format",
		Long: `doris-export is a CLI tool for exporting data from Apache Doris databases to Parquet files.

It supports various export modes including batched exports and partitioned exports,
with built-in verification capabilities.`,
		Example: `  # Export a table
  doris-export --host 127.0.0.1 --port 9030 -u root -p 123456 -d test_db -t users -o ./export

  # Export with WHERE clause
  doris-export --host 127.0.0.1 -u root -p 123456 -d test_db -t users -o ./export --where "age > 18"

  # Export with batching (creates multiple files)
  doris-export --host 127.0.0.1 -u root -p 123456 -d test_db -t users -o ./export --batch-size 10000

  # Export partitioned by a column
  doris-export --host 127.0.0.1 -u root -p 123456 -d test_db -t users -o ./export --partition-by country

  # Show table info only
  doris-export --host 127.0.0.1 -u root -p 123456 -d test_db -t users --info-only

  # Export with verification
  doris-export --host 127.0.0.1 -u root -p 123456 -d test_db -t users -o ./export --verify

  # Verify existing parquet files
  doris-export --verify-only ./export/*.parquet`,
		RunE: runExport,
	}

	// Add persistent flags
	addFlags(rootCmd)

	return rootCmd
}

// addFlags adds all CLI flags to the command
func addFlags(cmd *cobra.Command) {
	// Required flags
	cmd.Flags().StringVar(&host, "host", "", "Doris FE host (required)")
	cmd.Flags().IntVar(&port, "port", DefaultPort, "Doris FE port")
	cmd.Flags().StringVarP(&user, "user", "u", "", "Username (required)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password (required)")
	cmd.Flags().StringVarP(&dbName, "database", "d", "", "Database name (required)")
	cmd.Flags().StringVarP(&tableName, "table", "t", "", "Table name")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory")

	// Optional flags
	cmd.Flags().IntVar(&batchSize, "batch-size", 0, "Batch size for streaming (0 = no batching)")
	cmd.Flags().StringVar(&where, "where", "", "WHERE clause for filtering data")
	cmd.Flags().StringVar(&partitionBy, "partition-by", "", "Partition column for partitioned export")
	cmd.Flags().StringVar(&dateFormat, "date-format", "unix", "Date format: unix (timestamp), iso (ISO8601), string (original)")
	cmd.Flags().BoolVar(&infoOnly, "info-only", false, "Show table info only, no export")
	cmd.Flags().BoolVar(&noConfirm, "no-confirm", false, "Skip confirmation for large tables")
	cmd.Flags().BoolVar(&verifyFlag, "verify", false, "Verify after export")
	cmd.Flags().StringArrayVar(&verifyOnly, "verify-only", nil, "Only verify files (accepts file paths)")

	// Mark required flags
	cmd.MarkFlagRequired("host")
	cmd.MarkFlagRequired("user")
	cmd.MarkFlagRequired("password")
	cmd.MarkFlagRequired("database")
}

// runExport executes the main export logic
func runExport(cmd *cobra.Command, args []string) error {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Handle verify-only mode
	if len(verifyOnly) > 0 {
		return runVerifyOnly(logger)
	}

	// Validate table is required for export/info modes
	if tableName == "" && !infoOnly {
		return fmt.Errorf("--table is required for export")
	}
	if tableName == "" && infoOnly {
		return fmt.Errorf("--table is required for info-only mode")
	}

	// Validate output is required for export
	if outputDir == "" && !infoOnly {
		return fmt.Errorf("--output is required for export")
	}

	// Create database connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConfig := &database.Config{
		Host:     host,
		Port:     port,
		Username: user,
		Password: password,
		Database: dbName,
	}

	connManager := database.NewConnectionManager(dbConfig)
	db, err := connManager.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer connManager.Close()

	// Create services
	metadataService := metadata.NewService(db)
	queryExecutor := query.NewExecutor(db)

	// Get table metadata
	meta, err := metadataService.GetTableMetadata(ctx, dbName, tableName)
	if err != nil {
		return fmt.Errorf("failed to get table metadata: %w", err)
	}

	// Display table info
	printTableInfo(meta)

	// If info-only mode, we're done
	if infoOnly {
		return nil
	}

	// Check for large table confirmation
	if !noConfirm && meta.RowCount > LargeTableThreshold {
		if !confirmLargeTable(meta.RowCount) {
			logger.Info("Export cancelled by user")
			return nil
		}
	}

	// Perform export
	orchestrator := export.NewOrchestrator(db, metadataService, queryExecutor)
	exportOpts := export.Options{
		Database:    dbName,
		Table:       tableName,
		OutputDir:   outputDir,
		BatchSize:   batchSize,
		WhereClause: where,
		PartitionBy: partitionBy,
		Verify:      verifyFlag,
		DateFormat:  dateFormat,
	}

	result, err := orchestrator.Export(ctx, exportOpts)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Print results
	printExportResult(result)

	// Verify if requested
	if verifyFlag {
		return verifyExport(result.Files, logger)
	}

	return nil
}

// runVerifyOnly runs verification on provided files
func runVerifyOnly(logger *logrus.Logger) error {
	if len(verifyOnly) == 0 {
		return fmt.Errorf("no files specified for verification")
	}

	logger.WithField("files", len(verifyOnly)).Info("Verifying files")

	verifier := verify.NewVerifier()
	results, allValid := verifier.VerifyBatch(verifyOnly)

	printVerifyResults(results)

	if !allValid {
		return fmt.Errorf("some files failed verification")
	}

	logger.Info("All files verified successfully")
	return nil
}

// verifyExport verifies exported files
func verifyExport(files []string, logger *logrus.Logger) error {
	if len(files) == 0 {
		return nil
	}

	logger.WithField("files", len(files)).Info("Verifying exported files")

	verifier := verify.NewVerifier()
	results, allValid := verifier.VerifyBatch(files)

	printVerifyResults(results)

	if !allValid {
		return fmt.Errorf("some exported files failed verification")
	}

	logger.Info("All exported files verified successfully")
	return nil
}

// confirmLargeTable prompts user for confirmation for large tables
func confirmLargeTable(rowCount int64) bool {
	fmt.Printf("\n⚠️  Warning: Table has %d rows (threshold: %d)\n", rowCount, LargeTableThreshold)
	fmt.Print("Do you want to continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// printTableInfo prints table metadata
func printTableInfo(meta *metadata.TableMetadata) {
	fmt.Printf("\n📋 Table Information\n")
	fmt.Printf("====================\n")
	fmt.Printf("Database:    %s\n", meta.Database)
	fmt.Printf("Table:       %s\n", meta.Table)
	fmt.Printf("Columns:     %d\n", meta.ColumnCount)
	fmt.Printf("Rows:        %d\n", meta.RowCount)
	fmt.Printf("Data Size:   %.2f MB\n", meta.DataSizeMB)
	fmt.Printf("\nColumn List:\n")
	for i, col := range meta.Columns {
		fmt.Printf("  %d. %s\n", i+1, col)
	}
	fmt.Println()
}

// printExportResult prints export results
func printExportResult(result *export.Result) {
	fmt.Printf("\n✅ Export Complete\n")
	fmt.Printf("==================\n")
	fmt.Printf("Files:      %d\n", len(result.Files))
	fmt.Printf("Total Rows: %d\n", result.TotalRows)
	fmt.Printf("Duration:   %s\n", result.Duration)
	fmt.Printf("\nExported Files:\n")
	for _, file := range result.Files {
		fmt.Printf("  - %s\n", file)
	}
	fmt.Println()
}

// printVerifyResults prints verification results
func printVerifyResults(results []*verify.Result) {
	fmt.Printf("\n🔍 Verification Results\n")
	fmt.Printf("=======================\n")

	for _, r := range results {
		if r.Valid {
			fmt.Printf("✅ %s\n", r.FilePath)
			fmt.Printf("   Rows: %d, Columns: %d, Size: %.2f MB\n", r.RowCount, r.ColumnCount, r.SizeMB)
		} else {
			fmt.Printf("❌ %s\n", r.FilePath)
			fmt.Printf("   Error: %s\n", r.Error)
		}
	}
	fmt.Println()
}
