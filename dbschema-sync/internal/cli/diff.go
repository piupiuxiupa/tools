package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/lush/dbschema-sync/internal/database"
	"github.com/lush/dbschema-sync/internal/schema"
	"github.com/lush/dbschema-sync/pkg/types"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var (
		sourceDSN    string
		targetDSN    string
		outputFormat string
		ignoreTables []string
	)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between two database schemas",
		Long:  `Compare source and target database schemas and display the differences without applying any changes.`,
		Example: `  # Show diff between MySQL and PostgreSQL
  dbschema-sync diff --source mysql://user:pass@localhost/db1 --target postgres://user:pass@localhost/db2

  # Show diff using config file
  dbschema-sync diff --config config.yaml

  # Output as JSON
  dbschema-sync diff --source mysql://localhost/db1 --target postgres://localhost/db2 --format json

  # Ignore specific tables
  dbschema-sync diff --source mysql://localhost/db1 --target postgres://localhost/db2 --ignore-tables=logs,tmp_*`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := getLogger(ctx)
			cfg := getConfig(ctx)

			flagSourceProvided := cmd.Flags().Changed("source")
			flagTargetProvided := cmd.Flags().Changed("target")

			var sourceDriver, sourceConn string
			var targetDriver, targetConn string

			if sourceDSN != "" {
				var err error
				sourceDriver, sourceConn, err = parseDSN(sourceDSN)
				if err != nil {
					return fmt.Errorf("invalid source DSN: %w", err)
				}
			} else if cfg.Source.Driver != "" {
				sourceDriver = cfg.Source.Driver
				sourceConn, _ = cfg.Source.BuildDSN()
			} else if !flagSourceProvided {
				return fmt.Errorf("source is required: provide via --source flag or set in config file (source.driver, source.host, etc.)")
			}

			if targetDSN != "" {
				var err error
				targetDriver, targetConn, err = parseDSN(targetDSN)
				if err != nil {
					return fmt.Errorf("invalid target DSN: %w", err)
				}
			} else if cfg.Target.Driver != "" {
				targetDriver = cfg.Target.Driver
				targetConn, _ = cfg.Target.BuildDSN()
			} else if !flagTargetProvided {
				return fmt.Errorf("target is required: provide via --target flag or set in config file (target.driver, target.host, etc.)")
			}

			log.Infof("Comparing schemas: %s -> %s", sourceDriver, targetDriver)

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

			// Extract target schema
			log.Info("Extracting target schema...")
			targetSchema, err := sourceExtractor.ExtractSchema(ctx, targetDB)
			if err != nil {
				return fmt.Errorf("failed to extract target schema: %w", err)
			}

			// Compare schemas
			log.Info("Comparing schemas...")
			comparator := schema.NewComparator(schema.WithIgnoreTables(ignoreTables...))
			diff, err := comparator.Compare(sourceSchema, targetSchema)
			if err != nil {
				return fmt.Errorf("failed to compare schemas: %w", err)
			}

			// Output results
			switch strings.ToLower(outputFormat) {
			case "json":
				return outputDiffAsJSON(diff)
			case "table", "text":
				return outputDiffAsTable(diff)
			default:
				return fmt.Errorf("unsupported output format: %s", outputFormat)
			}
		},
	}

	cmd.Flags().StringVar(&sourceDSN, "source", "", "Source database DSN (can also be set in config file)")
	cmd.Flags().StringVar(&targetDSN, "target", "", "Target database DSN (can also be set in config file)")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "table", "Output format (table, json)")
	cmd.Flags().StringSliceVar(&ignoreTables, "ignore-tables", nil, "Tables to ignore (comma-separated, supports wildcards)")

	return cmd
}

func outputDiffAsJSON(diff *types.SchemaDiff) error {
	jsonData, err := json.MarshalIndent(diff, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal diff to JSON: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

func outputDiffAsTable(diff *types.SchemaDiff) error {
	if diff.IsEmpty() {
		fmt.Println(color.GreenString("✓ Schemas are identical"))
		return nil
	}

	fmt.Printf("\n%s %s → %s\n\n",
		color.CyanString("Schema Diff:"),
		diff.SourceSchema,
		diff.TargetSchema)

	// Tables added
	if len(diff.TablesAdded) > 0 {
		fmt.Println(color.GreenString("Tables Added:"))
		for _, t := range diff.TablesAdded {
			fmt.Printf("  + %s (%d columns)\n", t.Name, len(t.Columns))
		}
		fmt.Println()
	}

	// Tables removed
	if len(diff.TablesRemoved) > 0 {
		fmt.Println(color.RedString("Tables Removed:"))
		for _, t := range diff.TablesRemoved {
			fmt.Printf("  - %s\n", t.Name)
		}
		fmt.Println()
	}

	// Tables modified
	if len(diff.TablesModified) > 0 {
		fmt.Println(color.YellowString("Tables Modified:"))
		for _, td := range diff.TablesModified {
			fmt.Printf("  ~ %s\n", td.TableName)

			if len(td.ColumnsAdded) > 0 {
				for _, c := range td.ColumnsAdded {
					fmt.Printf("      + %s %s\n", c.Name, c.DataType)
				}
			}
			if len(td.ColumnsRemoved) > 0 {
				for _, c := range td.ColumnsRemoved {
					fmt.Printf("      - %s\n", c.Name)
				}
			}
			if len(td.ColumnsModified) > 0 {
				for _, c := range td.ColumnsModified {
					fmt.Printf("      ~ %s (%s)\n", c.Column.Name, strings.Join(c.Changes, ", "))
				}
			}
			if len(td.IndexesAdded) > 0 {
				for _, idx := range td.IndexesAdded {
					fmt.Printf("      + INDEX %s\n", idx.Name)
				}
			}
			if len(td.IndexesRemoved) > 0 {
				for _, idx := range td.IndexesRemoved {
					fmt.Printf("      - INDEX %s\n", idx.Name)
				}
			}
		}
		fmt.Println()
	}

	// Summary
	fmt.Printf("%s\n", color.CyanString("Summary:"))
	fmt.Printf("  Tables Added:    %d\n", len(diff.TablesAdded))
	fmt.Printf("  Tables Removed:  %d\n", len(diff.TablesRemoved))
	fmt.Printf("  Tables Modified: %d\n", len(diff.TablesModified))

	return nil
}
