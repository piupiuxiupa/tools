package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ConfigTemplate represents the configuration template with comments
type ConfigTemplate struct {
	Source DatabaseConfigTemplate `yaml:"source" json:"source"`
	Target DatabaseConfigTemplate `yaml:"target" json:"target"`
	Sync   SyncConfigTemplate     `yaml:"sync" json:"sync"`
	Pool   PoolConfigTemplate     `yaml:"pool" json:"pool"`
	Log    LogConfigTemplate      `yaml:"log" json:"log"`
}

// DatabaseConfigTemplate represents database configuration with comments
type DatabaseConfigTemplate struct {
	Driver   string            `yaml:"driver" json:"driver"`
	Host     string            `yaml:"host" json:"host"`
	Port     int               `yaml:"port" json:"port"`
	Database string            `yaml:"database" json:"database"`
	User     string            `yaml:"user" json:"user"`
	Password string            `yaml:"password" json:"password"`
	DSN      string            `yaml:"dsn,omitempty" json:"dsn,omitempty"`
	SSLMode  string            `yaml:"sslmode,omitempty" json:"sslmode,omitempty"`
	Params   map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
}

// SyncConfigTemplate represents sync configuration with comments
type SyncConfigTemplate struct {
	DryRun          bool     `yaml:"dry_run" json:"dry_run"`
	AutoApply       bool     `yaml:"auto_apply" json:"auto_apply"`
	IgnoreTables    []string `yaml:"ignore_tables" json:"ignore_tables"`
	IncludeTables   []string `yaml:"include_tables" json:"include_tables"`
	SkipForeignKeys bool     `yaml:"skip_foreign_keys" json:"skip_foreign_keys"`
	SkipIndexes     bool     `yaml:"skip_indexes" json:"skip_indexes"`
	BatchSize       int      `yaml:"batch_size" json:"batch_size"`
}

// PoolConfigTemplate represents pool configuration with comments
type PoolConfigTemplate struct {
	MaxOpenConns    int    `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime string `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
}

// LogConfigTemplate represents log configuration with comments
type LogConfigTemplate struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

func newInitCmd() *cobra.Command {
	var (
		output string
		format string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a configuration file template",
		Long: `Generate a configuration file template for dbschema-sync.

This command creates a sample configuration file with all available options
and helpful comments. You can edit this file and use it with the --config flag.`,
		Example: `  # Generate config and print to stdout
  dbschema-sync init

  # Generate config and save to file
  dbschema-sync init --output config.yaml

  # Generate config in JSON format
  dbschema-sync init --format json --output config.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Auto-detect format from output extension if not specified
			if format == "" && output != "" {
				ext := strings.ToLower(filepath.Ext(output))
				switch ext {
				case ".json":
					format = "json"
				case ".yaml", ".yml":
					format = "yaml"
				}
			}

			if format == "" {
				format = "yaml"
			}

			template := getConfigTemplate()

			var data []byte
			var err error

			switch strings.ToLower(format) {
			case "yaml", "yml":
				data, err = marshalYAMLWithComments(template)
				if err != nil {
					return fmt.Errorf("failed to marshal config to YAML: %w", err)
				}
			case "json":
				data, err = marshalJSONWithComments(template)
				if err != nil {
					return fmt.Errorf("failed to marshal config to JSON: %w", err)
				}
			default:
				return fmt.Errorf("unsupported format: %s (supported: yaml, json)", format)
			}

			// Write to file or stdout
			if output == "" || output == "-" {
				_, err = os.Stdout.Write(data)
				if err != nil {
					return fmt.Errorf("failed to write to stdout: %w", err)
				}
			} else {
				// Create directory if it doesn't exist
				dir := filepath.Dir(output)
				if dir != "." && dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory %s: %w", dir, err)
					}
				}

				err = os.WriteFile(output, data, 0644)
				if err != nil {
					return fmt.Errorf("failed to write to file %s: %w", output, err)
				}
				fmt.Fprintf(os.Stderr, "Configuration template generated: %s\n", output)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVarP(&format, "format", "f", "", "Output format (yaml, json). Auto-detected from file extension if not specified")

	return cmd
}

func getConfigTemplate() ConfigTemplate {
	return ConfigTemplate{
		Source: DatabaseConfigTemplate{
			Driver:   "mysql",
			Host:     "localhost",
			Port:     3306,
			Database: "source_db",
			User:     "root",
			Password: "password",
			SSLMode:  "disable",
			Params: map[string]string{
				"parseTime": "true",
			},
		},
		Target: DatabaseConfigTemplate{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Database: "target_db",
			User:     "postgres",
			Password: "password",
			SSLMode:  "disable",
			Params:   map[string]string{},
		},
		Sync: SyncConfigTemplate{
			DryRun:          true,
			AutoApply:       false,
			IgnoreTables:    []string{"logs", "tmp_*"},
			IncludeTables:   []string{},
			SkipForeignKeys: false,
			SkipIndexes:     false,
			BatchSize:       100,
		},
		Pool: PoolConfigTemplate{
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: "1h",
			ConnMaxIdleTime: "30m",
		},
		Log: LogConfigTemplate{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
	}
}

func marshalYAMLWithComments(t ConfigTemplate) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString("# dbschema-sync Configuration File\n")
	sb.WriteString("# \n")
	sb.WriteString("# This file configures database schema synchronization settings.\n")
	sb.WriteString("# You can use this file with: dbschema-sync --config config.yaml\n")
	sb.WriteString("# \n")
	sb.WriteString("# Environment variables can override these values (prefix: DBSYNC_)\n")
	sb.WriteString("# Example: DBSYNC_SOURCE_HOST=remotehost\n")
	sb.WriteString("\n")

	// Source section
	sb.WriteString("# ============================================\n")
	sb.WriteString("# Source Database Configuration\n")
	sb.WriteString("# The source database is used as the reference schema\n")
	sb.WriteString("# ============================================\n")
	sb.WriteString("source:\n")
	sb.WriteString("  # Database driver: mysql, postgres, oracle, doris\n")
	sb.WriteString(fmt.Sprintf("  driver: %s\n", t.Source.Driver))
	sb.WriteString("  # Database host (hostname or IP address)\n")
	sb.WriteString(fmt.Sprintf("  host: %s\n", t.Source.Host))
	sb.WriteString("  # Database port (default: mysql=3306, postgres=5432)\n")
	sb.WriteString(fmt.Sprintf("  port: %d\n", t.Source.Port))
	sb.WriteString("  # Database name\n")
	sb.WriteString(fmt.Sprintf("  database: %s\n", t.Source.Database))
	sb.WriteString("  # Database user\n")
	sb.WriteString(fmt.Sprintf("  user: %s\n", t.Source.User))
	sb.WriteString("  # Database password\n")
	sb.WriteString(fmt.Sprintf("  password: %s\n", t.Source.Password))
	sb.WriteString("  # SSL mode for PostgreSQL (disable, require, verify-ca, verify-full)\n")
	sb.WriteString(fmt.Sprintf("  sslmode: %s\n", t.Source.SSLMode))
	sb.WriteString("  # Additional connection parameters\n")
	sb.WriteString("  params:\n")
	for k, v := range t.Source.Params {
		sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
	}
	sb.WriteString("\n")

	// Target section
	sb.WriteString("# ============================================\n")
	sb.WriteString("# Target Database Configuration\n")
	sb.WriteString("# The target database will be synchronized to match the source\n")
	sb.WriteString("# ============================================\n")
	sb.WriteString("target:\n")
	sb.WriteString("  # Database driver: mysql, postgres, oracle, doris\n")
	sb.WriteString(fmt.Sprintf("  driver: %s\n", t.Target.Driver))
	sb.WriteString("  # Database host (hostname or IP address)\n")
	sb.WriteString(fmt.Sprintf("  host: %s\n", t.Target.Host))
	sb.WriteString("  # Database port (default: mysql=3306, postgres=5432)\n")
	sb.WriteString(fmt.Sprintf("  port: %d\n", t.Target.Port))
	sb.WriteString("  # Database name\n")
	sb.WriteString(fmt.Sprintf("  database: %s\n", t.Target.Database))
	sb.WriteString("  # Database user\n")
	sb.WriteString(fmt.Sprintf("  user: %s\n", t.Target.User))
	sb.WriteString("  # Database password\n")
	sb.WriteString(fmt.Sprintf("  password: %s\n", t.Target.Password))
	sb.WriteString("  # SSL mode for PostgreSQL (disable, require, verify-ca, verify-full)\n")
	sb.WriteString(fmt.Sprintf("  sslmode: %s\n", t.Target.SSLMode))
	sb.WriteString("  # Additional connection parameters\n")
	sb.WriteString("  params:\n")
	for k, v := range t.Target.Params {
		sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
	}
	sb.WriteString("\n")

	// Sync section
	sb.WriteString("# ============================================\n")
	sb.WriteString("# Synchronization Settings\n")
	sb.WriteString("# ============================================\n")
	sb.WriteString("sync:\n")
	sb.WriteString("  # Dry run mode: show changes without applying them\n")
	sb.WriteString(fmt.Sprintf("  dry_run: %v\n", t.Sync.DryRun))
	sb.WriteString("  # Auto-apply: apply changes without confirmation (use with caution!)\n")
	sb.WriteString(fmt.Sprintf("  auto_apply: %v\n", t.Sync.AutoApply))
	sb.WriteString("  # Tables to ignore (supports wildcards like tmp_*)\n")
	sb.WriteString("  ignore_tables:\n")
	for _, table := range t.Sync.IgnoreTables {
		sb.WriteString(fmt.Sprintf("    - %s\n", table))
	}
	sb.WriteString("  # Tables to include (empty = all tables)\n")
	sb.WriteString("  include_tables:\n")
	for _, table := range t.Sync.IncludeTables {
		sb.WriteString(fmt.Sprintf("    - %s\n", table))
	}
	sb.WriteString("  # Skip foreign key constraints during sync\n")
	sb.WriteString(fmt.Sprintf("  skip_foreign_keys: %v\n", t.Sync.SkipForeignKeys))
	sb.WriteString("  # Skip indexes during sync\n")
	sb.WriteString(fmt.Sprintf("  skip_indexes: %v\n", t.Sync.SkipIndexes))
	sb.WriteString("  # Batch size for operations\n")
	sb.WriteString(fmt.Sprintf("  batch_size: %d\n", t.Sync.BatchSize))
	sb.WriteString("\n")

	// Pool section
	sb.WriteString("# ============================================\n")
	sb.WriteString("# Connection Pool Settings\n")
	sb.WriteString("# ============================================\n")
	sb.WriteString("pool:\n")
	sb.WriteString("  # Maximum number of open connections\n")
	sb.WriteString(fmt.Sprintf("  max_open_conns: %d\n", t.Pool.MaxOpenConns))
	sb.WriteString("  # Maximum number of idle connections\n")
	sb.WriteString(fmt.Sprintf("  max_idle_conns: %d\n", t.Pool.MaxIdleConns))
	sb.WriteString("  # Maximum lifetime of a connection (e.g., 1h, 30m)\n")
	sb.WriteString(fmt.Sprintf("  conn_max_lifetime: %s\n", t.Pool.ConnMaxLifetime))
	sb.WriteString("  # Maximum idle time of a connection (e.g., 30m, 1h)\n")
	sb.WriteString(fmt.Sprintf("  conn_max_idle_time: %s\n", t.Pool.ConnMaxIdleTime))
	sb.WriteString("\n")

	// Log section
	sb.WriteString("# ============================================\n")
	sb.WriteString("# Logging Settings\n")
	sb.WriteString("# ============================================\n")
	sb.WriteString("log:\n")
	sb.WriteString("  # Log level: debug, info, warn, error\n")
	sb.WriteString(fmt.Sprintf("  level: %s\n", t.Log.Level))
	sb.WriteString("  # Log format: json, text\n")
	sb.WriteString(fmt.Sprintf("  format: %s\n", t.Log.Format))
	sb.WriteString("  # Log output: stdout, stderr, or file path\n")
	sb.WriteString(fmt.Sprintf("  output: %s\n", t.Log.Output))

	return []byte(sb.String()), nil
}

func marshalJSONWithComments(t ConfigTemplate) ([]byte, error) {
	// For JSON, we'll add comments as a header
	var sb strings.Builder

	sb.WriteString("{\n")
	sb.WriteString("  \"_comment\": \"dbschema-sync Configuration File\",\n")
	sb.WriteString("  \"_description\": \"Configuration for database schema synchronization\",\n")
	sb.WriteString("  \"_usage\": \"Use with: dbschema-sync --config config.json\",\n")
	sb.WriteString("\n")

	// Source section
	sb.WriteString("  \"_comment_source\": \"Source Database Configuration (reference schema)\",\n")
	sb.WriteString("  \"source\": {\n")
	sb.WriteString("    \"driver\": \"mysql\",\n")
	sb.WriteString("    \"host\": \"localhost\",\n")
	sb.WriteString("    \"port\": 3306,\n")
	sb.WriteString("    \"database\": \"source_db\",\n")
	sb.WriteString("    \"user\": \"root\",\n")
	sb.WriteString("    \"password\": \"password\",\n")
	sb.WriteString("    \"sslmode\": \"disable\",\n")
	sb.WriteString("    \"params\": {\n")
	sb.WriteString("      \"parseTime\": \"true\"\n")
	sb.WriteString("    }\n")
	sb.WriteString("  },\n")
	sb.WriteString("\n")

	// Target section
	sb.WriteString("  \"_comment_target\": \"Target Database Configuration (to be synchronized)\",\n")
	sb.WriteString("  \"target\": {\n")
	sb.WriteString("    \"driver\": \"postgres\",\n")
	sb.WriteString("    \"host\": \"localhost\",\n")
	sb.WriteString("    \"port\": 5432,\n")
	sb.WriteString("    \"database\": \"target_db\",\n")
	sb.WriteString("    \"user\": \"postgres\",\n")
	sb.WriteString("    \"password\": \"password\",\n")
	sb.WriteString("    \"sslmode\": \"disable\",\n")
	sb.WriteString("    \"params\": {}\n")
	sb.WriteString("  },\n")
	sb.WriteString("\n")

	// Sync section
	sb.WriteString("  \"_comment_sync\": \"Synchronization Settings\",\n")
	sb.WriteString("  \"sync\": {\n")
	sb.WriteString("    \"dry_run\": true,\n")
	sb.WriteString("    \"auto_apply\": false,\n")
	sb.WriteString("    \"ignore_tables\": [\"logs\", \"tmp_*\"],\n")
	sb.WriteString("    \"include_tables\": [],\n")
	sb.WriteString("    \"skip_foreign_keys\": false,\n")
	sb.WriteString("    \"skip_indexes\": false,\n")
	sb.WriteString("    \"batch_size\": 100\n")
	sb.WriteString("  },\n")
	sb.WriteString("\n")

	// Pool section
	sb.WriteString("  \"_comment_pool\": \"Connection Pool Settings\",\n")
	sb.WriteString("  \"pool\": {\n")
	sb.WriteString("    \"max_open_conns\": 10,\n")
	sb.WriteString("    \"max_idle_conns\": 5,\n")
	sb.WriteString("    \"conn_max_lifetime\": \"1h\",\n")
	sb.WriteString("    \"conn_max_idle_time\": \"30m\"\n")
	sb.WriteString("  },\n")
	sb.WriteString("\n")

	// Log section
	sb.WriteString("  \"_comment_log\": \"Logging Settings\",\n")
	sb.WriteString("  \"log\": {\n")
	sb.WriteString("    \"level\": \"info\",\n")
	sb.WriteString("    \"format\": \"text\",\n")
	sb.WriteString("    \"output\": \"stdout\"\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return []byte(sb.String()), nil
}
