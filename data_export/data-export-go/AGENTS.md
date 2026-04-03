# AGENTS.md - Data Export Tool (Go)

## Build/Test/Lint Commands

### Build
```bash
make build                    # Build for current architecture → bin/data-export
make build-all                # Build for all platforms
make build-linux-amd64        # Cross-compile for Linux AMD64
make build-linux-arm64        # Cross-compile for Linux ARM64
make build-darwin-amd64       # Cross-compile for macOS AMD64
make build-windows            # Cross-compile for Windows AMD64
```

### Test
```bash
make test                     # Run all tests (go test -v ./...)

# Run tests for specific packages
go test ./internal/database/... -v
go test ./internal/export/... -v
go test ./internal/parquet/... -v

# Run a single test by name
go test ./internal/export/... -v -run TestExport_SimpleMode

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run benchmarks
go test ./internal/parquet/... -bench=. -benchmem
```

### Lint/Format
```bash
make fmt                      # Format code (go fmt ./...)
make lint                     # Run golangci-lint (default config, no .golangci.yml)
make tidy                     # Tidy dependencies (go mod tidy)
```

## Code Style Guidelines

### Project Structure
```
cmd/data_export/          # Main entry point (main.go)
internal/
  cli/                    # CLI command handling (Cobra)
  config/                 # Configuration types (DBType, Config structs)
  database/               # Database connection management (MySQL, PostgreSQL, Oracle)
  export/                 # Export orchestration (simple, batched, partitioned)
  metadata/               # Table metadata service
  parquet/                # Parquet writer + schema inference
  query/                  # Query execution with streaming iterators
  verify/                 # Parquet file verification
pkg/types/                # Public types (ExportResult, TableInfo, ColumnInfo)
```

### Imports
Standard library first, then local packages, then third-party. Blank imports for database drivers grouped with third-party. Separate groups with blank lines.

```go
import (
	"context"
	"database/sql"
	"fmt"

	"github.com/user/data-export-go/internal/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
	goora "github.com/sijms/go-ora/v2"
)
```

### Naming Conventions
- **Packages**: lowercase, no underscores (e.g., `database`, `export`, `query`)
- **Exported types/functions**: PascalCase (e.g., `ConnectionManager`, `NewOrchestrator`)
- **Unexported types/functions**: camelCase (e.g., `buildDSN`, `setDefaults`, `sanitizePartitionValue`)
- **Constants**: PascalCase for exported, camelCase for unexported; group in `const` blocks
- **Interfaces**: Named as nouns describing role (e.g., `ConnectionManager`, `Service`, `Executor`, `Writer`, `Verifier`)
- **Implementations**: Unexported struct with lowercase name (e.g., `manager` implements `ConnectionManager`, `orchestrator` implements `Orchestrator`)
- **Constructors**: `New*` functions returning the interface type, not the concrete struct
- **Test files**: `*_test.go`, test functions prefixed with `Test`

### Error Handling
Always wrap errors with context using `fmt.Errorf("...: %w", err)`. Never ignore errors.

```go
if err != nil {
	return nil, fmt.Errorf("failed to open database connection: %w", err)
}
```

### Context Usage
Always pass `context.Context` as the first parameter. Use `context.WithTimeout()` with immediate `defer cancel()`.

```go
pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

if err := db.PingContext(pingCtx); err != nil {
	return fmt.Errorf("failed to ping database: %w", err)
}
```

### Logging
Use `logrus` for structured logging. Create loggers per component with `logrus.New()`.

```go
logger.WithFields(logrus.Fields{
	"database": opts.Database,
	"table":    opts.Table,
}).Info("Starting export")
```

### Interfaces and Dependency Injection
Define interfaces at the consumer side. Each package exposes a small interface consumed by others. Constructors accept dependencies as interfaces.

```go
type Orchestrator interface {
	Export(ctx context.Context, opts Options) (*Result, error)
}

func NewOrchestrator(db *sql.DB, metadata metadata.Service, query query.Executor) Orchestrator {
```

### SQL Queries
Use backticks for MySQL identifiers, double quotes for PostgreSQL, unquoted for Oracle. Use `?` placeholders (MySQL), `$1` (Postgres). Build dynamic queries with `fmt.Sprintf()` only for table/column names — never for WHERE clause values.

```go
// MySQL
query := fmt.Sprintf("SELECT * FROM `%s`.`%s`", database, table)
// PostgreSQL
query := fmt.Sprintf("SELECT * FROM \"%s\".\"%s\"", database, table)
```

### Testing
Table-driven tests with `[]struct` pattern. Use `testify/assert` and `testify/require` for assertions. Use `sqlmock` for database mocking. Create temp directories with `t.TempDir()`. Test helpers named `setupMockDB`, etc.

```go
func TestConfig_setDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{name: "all defaults applied", config: Config{}, expected: Config{MaxOpenConns: DefaultMaxOpenConns}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			cfg.setDefaults()
			assert.Equal(t, tt.expected.MaxOpenConns, cfg.MaxOpenConns)
		})
	}
}

// Mock helper pattern
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}
```

### Concurrency
Use `sync.Mutex` for shared state. Always unlock with `defer mu.Unlock()`. Pass mutex pointers when sharing across function boundaries.

### Typed Constants for Enums
Use typed string constants for enum-like values with comments.

```go
type DBType string

const (
	DBTypeMySQL    DBType = "mysql"
	DBTypeOracle   DBType = "oracle"
	DBTypePostgres DBType = "postgres"
)

type DateFormat string

const (
	DateFormatUnix   DateFormat = "unix"   // Unix timestamp in milliseconds
	DateFormatISO    DateFormat = "iso"    // ISO 8601 format (RFC3339)
	DateFormatString DateFormat = "string" // Original string format
)
```

## Key Dependencies
- `github.com/spf13/cobra` - CLI framework
- `github.com/go-sql-driver/mysql` - MySQL driver
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/sijms/go-ora/v2` - Oracle driver
- `github.com/parquet-go/parquet-go` - Parquet file handling
- `github.com/sirupsen/logrus` - Structured logging
- `github.com/stretchr/testify` - Testing assertions
- `github.com/DATA-DOG/go-sqlmock` - SQL mocking for tests

## Go Version
- Go 1.24+ (see `go.mod`)

## Notes
- No `.golangci.yml` — linter runs with default configuration
- No CI/CD pipelines configured
- Module path is `github.com/user/data-export-go`
