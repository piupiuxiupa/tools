package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/lush/dbschema-sync/internal/config"
	"github.com/lush/dbschema-sync/internal/logger"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	log     *logger.Logger
)

// Execute runs the root command
func Execute(ctx context.Context, version string) error {
	rootCmd := &cobra.Command{
		Use:   "dbschema-sync",
		Short: "Database schema synchronization tool",
		Long: `dbschema-sync is a CLI tool for synchronizing database schemas across 
different database systems including MySQL, PostgreSQL, Oracle, and Doris.

It supports comparing schemas, generating DDL statements, and applying changes.`,
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip config loading for certain commands
			if cmd.Name() == "help" || cmd.Name() == "version" || cmd.Name() == "init" {
				return nil
			}

			// Load configuration
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Initialize logger
			logLevel := "info"
			if verbose {
				logLevel = "debug"
			}
			log = logger.New(logLevel)

			// Store config and logger in context
			cmd.SetContext(config.WithConfig(cmd.Context(), cfg))
			cmd.SetContext(logger.WithLogger(cmd.Context(), log))

			return nil
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.dbschema-sync/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newDiffCmd())
	rootCmd.AddCommand(newExtractCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newInitCmd())

	return rootCmd.ExecuteContext(ctx)
}

// getConfig retrieves config from context
func getConfig(ctx context.Context) *config.Config {
	return config.GetConfig(ctx)
}

// getLogger retrieves logger from context
func getLogger(ctx context.Context) *logger.Logger {
	return logger.GetLogger(ctx)
}

// parseDSN parses a DSN string and returns driver name and DSN
// Format: driver://user:password@host:port/database?param=value
// Or: driver://host:port/database?param=value
func parseDSN(dsn string) (driver string, connStr string, err error) {
	parts := strings.SplitN(dsn, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid DSN format: %s", dsn)
	}
	return parts[0], parts[1], nil
}
