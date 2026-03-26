package cli

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/lush/dbschema-sync/internal/database"
	"github.com/lush/dbschema-sync/internal/logger"
	"github.com/lush/dbschema-sync/pkg/types"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var (
		sourceDSN string
		targetDSN string
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate schema compatibility",
		Long: `Validate that schemas are compatible for synchronization.
		
This command checks:
- Database connectivity
- Type mapping compatibility
- Unsupported features between databases
- Schema validation rules`,
		Example: `  # Validate MySQL to PostgreSQL compatibility
  dbschema-sync validate --source mysql://user:pass@localhost/db1 --target postgres://user:pass@localhost/db2

  # Validate single database
  dbschema-sync validate --source mysql://user:pass@localhost/db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := getLogger(ctx)

			if sourceDSN == "" {
				return fmt.Errorf("--source DSN is required")
			}

			sourceDriver, sourceConn, err := parseDSN(sourceDSN)
			if err != nil {
				return fmt.Errorf("invalid source DSN: %w", err)
			}

			fmt.Printf("\n%s\n\n", color.CyanString("Schema Validation Report"))

			// Validate source database
			fmt.Printf("Source Database: %s\n", sourceDriver)
			sourceValid, sourceIssues := validateDatabase(ctx, log, sourceDriver, sourceConn)

			if !sourceValid {
				fmt.Printf("  Status: %s\n", color.RedString("✗ Failed"))
				for _, issue := range sourceIssues {
					fmt.Printf("    - %s\n", issue)
				}
				return fmt.Errorf("source database validation failed")
			}
			fmt.Printf("  Status: %s\n", color.GreenString("✓ Valid"))

			// Validate target if provided
			if targetDSN != "" {
				targetDriver, targetConn, err := parseDSN(targetDSN)
				if err != nil {
					return fmt.Errorf("invalid target DSN: %w", err)
				}

				fmt.Printf("\nTarget Database: %s\n", targetDriver)
				targetValid, targetIssues := validateDatabase(ctx, log, targetDriver, targetConn)

				if !targetValid {
					fmt.Printf("  Status: %s\n", color.RedString("✗ Failed"))
					for _, issue := range targetIssues {
						fmt.Printf("    - %s\n", issue)
					}
					return fmt.Errorf("target database validation failed")
				}
				fmt.Printf("  Status: %s\n", color.GreenString("✓ Valid"))

				// Validate type mappings
				fmt.Printf("\n%s\n", color.CyanString("Type Mapping Compatibility:"))
				if err := validateTypeMappings(ctx, sourceDriver, sourceConn, targetDriver); err != nil {
					fmt.Printf("  Status: %s\n", color.RedString("✗ Failed"))
					fmt.Printf("  Error: %v\n", err)
					return fmt.Errorf("type mapping validation failed")
				}
				fmt.Printf("  Status: %s\n", color.GreenString("✓ Compatible"))
			}

			fmt.Printf("\n%s\n\n", color.GreenString("✓ Validation completed successfully"))
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceDSN, "source", "", "Source database DSN (required)")
	cmd.Flags().StringVar(&targetDSN, "target", "", "Target database DSN (optional, for cross-database validation)")

	cmd.MarkFlagRequired("source")

	return cmd
}

func validateDatabase(ctx context.Context, log *logger.Logger, driverName, dsn string) (bool, []string) {
	var issues []string

	// Test connection
	db, err := database.Connect(driverName, dsn)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Failed to connect: %v", err))
		return false, issues
	}
	defer db.Close()

	// Check if driver supports schema extraction
	extractor, err := database.GetExtractor(driverName)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Schema extraction not supported: %v", err))
		return false, issues
	}

	// Try to extract schema
	schema, err := extractor.ExtractSchema(ctx, db)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Failed to extract schema: %v", err))
		return false, issues
	}

	log.Debugf("Validating %d tables", len(schema.Tables))

	// Validate each table
	for _, table := range schema.Tables {
		if len(table.Columns) == 0 {
			issues = append(issues, fmt.Sprintf("Table %s has no columns", table.Name))
		}

		// Check for primary key
		hasPK := false
		for _, idx := range table.Indexes {
			if idx.Primary {
				hasPK = true
				break
			}
		}
		if !hasPK {
			log.Debugf("Table %s has no primary key", table.Name)
		}
	}

	return len(issues) == 0, issues
}

func validateTypeMappings(ctx context.Context, sourceDriver, sourceConn, targetDriver string) error {
	// Create type mapper
	mapper := types.NewDefaultTypeMapper()

	// Connect to source to get schema
	db, err := database.Connect(sourceDriver, sourceConn)
	if err != nil {
		return err
	}
	defer db.Close()

	// Extract schema
	extractor, err := database.GetExtractor(sourceDriver)
	if err != nil {
		return err
	}

	schema, err := extractor.ExtractSchema(ctx, db)
	if err != nil {
		return err
	}

	// Check each column's type mapping
	var unsupportedTypes []string
	for _, table := range schema.Tables {
		for _, col := range table.Columns {
			_, err := mapper.MapType(col.DataType, sourceDriver, targetDriver)
			if err != nil {
				unsupportedTypes = append(unsupportedTypes,
					fmt.Sprintf("%s.%s: %s", table.Name, col.Name, col.DataType))
			}
		}
	}

	if len(unsupportedTypes) > 0 {
		return fmt.Errorf("unsupported type mappings: %v", unsupportedTypes)
	}

	return nil
}
