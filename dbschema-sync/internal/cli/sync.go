package cli

import (
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/internal/database"
	"github.com/lush/dbschema-sync/internal/schema"
	"github.com/lush/dbschema-sync/pkg/types"

	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var (
		sourceDSN    string
		targetDSN    string
		dryRun       bool
		ignoreTables []string
		autoApply    bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize database schemas",
		Long:  `Compare source and target database schemas and apply changes to make them identical.`,
		Example: `  # Sync MySQL to PostgreSQL (dry-run)
  dbschema-sync sync --source mysql://user:pass@localhost/db1 --target postgres://user:pass@localhost/db2 --dry-run

  # Sync with auto-apply
  dbschema-sync sync --source mysql://localhost/db1 --target postgres://localhost/db2 --auto-apply

  # Sync ignoring specific tables
  dbschema-sync sync --source mysql://localhost/db1 --target postgres://localhost/db2 --ignore-tables=logs,tmp_*`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := getLogger(ctx)

			if sourceDSN == "" || targetDSN == "" {
				return fmt.Errorf("both --source and --target DSNs are required")
			}

			sourceDriver, sourceConn, err := parseDSN(sourceDSN)
			if err != nil {
				return fmt.Errorf("invalid source DSN: %w", err)
			}

			targetDriver, targetConn, err := parseDSN(targetDSN)
			if err != nil {
				return fmt.Errorf("invalid target DSN: %w", err)
			}

			log.Infof("Starting schema synchronization from %s to %s", sourceDriver, targetDriver)

			// Connect to source
			sourceDB, err := database.Connect(sourceDriver, sourceConn)
			if err != nil {
				return fmt.Errorf("failed to connect to source: %w", err)
			}
			defer sourceDB.Close()

			// Connect to target
			targetDB, err := database.Connect(targetDriver, targetConn)
			if err != nil {
				return fmt.Errorf("failed to connect to target: %w", err)
			}
			defer targetDB.Close()

			// Get extractors
			sourceExtractor, err := database.GetExtractor(sourceDriver)
			if err != nil {
				return fmt.Errorf("failed to get source extractor: %w", err)
			}

			// Extract source schema
			log.Info("Extracting source schema...")
			sourceSchema, err := sourceExtractor.ExtractSchema(ctx, sourceDB)
			if err != nil {
				return fmt.Errorf("failed to extract source schema: %w", err)
			}
			log.Infof("Source schema: %d tables", len(sourceSchema.Tables))

			// Extract target schema
			log.Info("Extracting target schema...")
			targetSchema, err := sourceExtractor.ExtractSchema(ctx, targetDB)
			if err != nil {
				return fmt.Errorf("failed to extract target schema: %w", err)
			}
			log.Infof("Target schema: %d tables", len(targetSchema.Tables))

			// Compare schemas
			log.Info("Comparing schemas...")
			comparator := schema.NewComparator(schema.WithIgnoreTables(ignoreTables...))
			diff, err := comparator.Compare(sourceSchema, targetSchema)
			if err != nil {
				return fmt.Errorf("failed to compare schemas: %w", err)
			}

			if !diff.HasChanges() {
				log.Info("Schemas are already in sync")
				return nil
			}

			log.Infof("Found differences: %s", diff.Summary())

			// Generate DDL
			generator, err := schema.NewGenerator(targetDriver, types.NewDefaultTypeMapper())
			if err != nil {
				return fmt.Errorf("failed to create DDL generator: %w", err)
			}

			statements, err := generator.GenerateDiff(diff)
			if err != nil {
				return fmt.Errorf("failed to generate DDL: %w", err)
			}

			// Output DDL
			if dryRun {
				fmt.Println("\n-- Generated DDL (dry-run):\n")
				for _, stmt := range statements {
					fmt.Printf("%s;\n\n", stmt)
				}
				return nil
			}

			// Apply changes
			if !autoApply {
				fmt.Println("\n-- Generated DDL:\n")
				for _, stmt := range statements {
					fmt.Printf("%s;\n\n", stmt)
				}
				fmt.Print("Apply these changes? [y/N]: ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					log.Info("Aborted")
					return nil
				}
			}

			// Execute DDL
			log.Info("Applying changes...")
			for _, stmt := range statements {
				log.Debugf("Executing: %s", stmt)
				if _, err := targetDB.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("failed to execute DDL: %w", err)
				}
			}

			log.Info("Schema synchronization completed successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceDSN, "source", "", "Source database DSN (required)")
	cmd.Flags().StringVar(&targetDSN, "target", "", "Target database DSN (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show DDL without executing")
	cmd.Flags().BoolVar(&autoApply, "auto-apply", false, "Apply changes without confirmation")
	cmd.Flags().StringSliceVar(&ignoreTables, "ignore-tables", nil, "Tables to ignore (comma-separated, supports wildcards)")

	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("target")

	return cmd
}
