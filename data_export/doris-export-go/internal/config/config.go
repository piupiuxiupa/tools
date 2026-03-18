package config

// Config holds the application configuration
type Config struct {
	Database DatabaseConfig
	Export   ExportConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// ExportConfig holds export settings
type ExportConfig struct {
	OutputDir   string
	BatchSize   int
	Concurrency int
}
