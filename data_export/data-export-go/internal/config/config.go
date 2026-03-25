package config

// DBType represents the database type
type DBType string

const (
	DBTypeMySQL    DBType = "mysql"
	DBTypeOracle   DBType = "oracle"
	DBTypePostgres DBType = "postgres"
)

// Config holds the application configuration
type Config struct {
	Database DatabaseConfig
	Export   ExportConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	DBType   DBType
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// DateFormat represents the format for date/time export
type DateFormat string

const (
	DateFormatUnix   DateFormat = "unix"   // Unix timestamp in milliseconds
	DateFormatISO    DateFormat = "iso"    // ISO 8601 format (RFC3339)
	DateFormatString DateFormat = "string" // Original string format from database
)

// ExportConfig holds export settings
type ExportConfig struct {
	OutputDir   string
	BatchSize   int
	Concurrency int
	DateFormat  DateFormat // Format for date/time columns: unix (default), iso, string
}
