package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lush/dbschema-sync/internal/database"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newExtractCmd() *cobra.Command {
	var (
		sourceDSN string
		output    string
		format    string
	)

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract database schema to file",
		Long:  `Extract schema from a database and export it to a file in JSON, YAML, or SQL format.`,
		Example: `  # Extract to JSON (default)
  dbschema-sync extract --source mysql://user:pass@localhost/db --output schema.json

  # Extract to YAML
  dbschema-sync extract --source mysql://user:pass@localhost/db --output schema.yaml --format yaml

  # Extract to SQL DDL
  dbschema-sync extract --source mysql://user:pass@localhost/db --output schema.sql --format sql`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := getLogger(ctx)

			if sourceDSN == "" {
				return fmt.Errorf("--source DSN is required")
			}

			driverName, connStr, err := parseDSN(sourceDSN)
			if err != nil {
				return fmt.Errorf("invalid DSN: %w", err)
			}

			// Auto-detect format from output extension if not specified
			if format == "" && output != "" {
				ext := strings.ToLower(filepath.Ext(output))
				switch ext {
				case ".json":
					format = "json"
				case ".yaml", ".yml":
					format = "yaml"
				case ".sql":
					format = "sql"
				}
			}

			if format == "" {
				format = "json"
			}

			log.Infof("Extracting schema from %s...", driverName)

			// Connect to database
			db, err := database.Connect(driverName, connStr)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()

			// Get extractor
			extractor, err := database.GetExtractor(driverName)
			if err != nil {
				return fmt.Errorf("failed to get extractor: %w", err)
			}

			// Extract schema
			schema, err := extractor.ExtractSchema(ctx, db)
			if err != nil {
				return fmt.Errorf("failed to extract schema: %w", err)
			}

			log.Infof("Extracted schema: %d tables", len(schema.Tables))

			// Export based on format
			var data []byte
			switch strings.ToLower(format) {
			case "json":
				data, err = json.MarshalIndent(schema, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal schema to JSON: %w", err)
				}
			case "yaml", "yml":
				data, err = yaml.Marshal(schema)
				if err != nil {
					return fmt.Errorf("failed to marshal schema to YAML: %w", err)
				}
			case "sql":
				// For SQL format, we'll just output a placeholder
				// In a real implementation, you'd use a DDL generator
				data = []byte(fmt.Sprintf("-- Schema extraction for %s\n-- Tables: %d\n\n", driverName, len(schema.Tables)))
				for _, table := range schema.Tables {
					data = append(data, []byte(fmt.Sprintf("-- Table: %s\n", table.Name))...)
				}
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}

			// Write to file or stdout
			if output == "" || output == "-" {
				_, err = os.Stdout.Write(data)
				if err != nil {
					return fmt.Errorf("failed to write to stdout: %w", err)
				}
			} else {
				err = os.WriteFile(output, data, 0644)
				if err != nil {
					return fmt.Errorf("failed to write to file %s: %w", output, err)
				}
				log.Infof("Schema exported to %s", output)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sourceDSN, "source", "", "Source database DSN (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVarP(&format, "format", "f", "", "Output format (json, yaml, sql). Auto-detected from file extension if not specified")

	cmd.MarkFlagRequired("source")

	return cmd
}
