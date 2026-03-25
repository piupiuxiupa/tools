# AGENTS.md - Data Export Tool (Go)

## Build/Test/Lint Commands

### Build
```bash
make build                    # Build for current architecture
make build-all                # Build for all platforms
make build-linux-amd64        # Cross-compile for Linux AMD64
make build-linux-arm64        # Cross-compile for Linux ARM64
make build-darwin-amd64       # Cross-compile for macOS AMD64
make build-windows            # Cross-compile for Windows AMD64
```

### Test
```bash
make test                     # Run all tests

# Run tests for specific packages
go test ./internal/database/... -v
go test ./internal/export/... -v
go test ./internal/parquet/... -v

# Run a single test by name
go test ./internal/export/... -v -run TestNewOrchestrator

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Lint/Format
```bash
make fmt                      # Format code (go fmt ./...)
make lint                     # Run linter (golangci-lint run ./...)
make tidy                     # Tidy dependencies (go mod tidy)
```

## Code Style Guidelines

### Project Structure
```
cmd/data_export/          # Main entry point
internal/
  cli/                    # CLI command handling
  config/                 # Configuration types
  database/               # Database connection management
  export/                 # Export orchestration
  metadata/               # Metadata service
  parquet/                # Parquet writer
  query/                  # Query execution
  verify/                 # File verification
pkg/types/                # Public types
```

### Imports
Standard library first, then blank imports, then third-party, then local. Group with blank lines between.

```go
import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/user/data-export-go/internal/database"
	"github.com/user/data-export-go/internal/export"
)
```

### Naming Conventions
- **Packages**: lowercase, no underscores (e.g., `database`, `export`)
- **Exported types/functions**: PascalCase (e.g., `ConnectionManager`, `NewOrchestrator`)
- **Unexported types/functions**: camelCase (e.g., `buildDSN`, `setDefaults`)
- **Constants**: PascalCase for exported, camelCase for unexported
- **Interfaces**: Suffix with descriptive noun (e.g., `ConnectionManager`)
- **Test files**: `*_test.go`, test functions start with `Test`
- **Test helpers**: `setupMockDB`, use `t.Helper()` if helper fails tests

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
Use `logrus` for structured logging. Create loggers per component.

```go
logger := logrus.New()
logger.SetLevel(logrus.InfoLevel)

logger.WithFields(logrus.Fields{
	"database": opts.Database,
	"table":    opts.Table,
}).Info("Starting export")
```

### Testing
Use table-driven tests with `[]struct` pattern. Use `testify/assert` and `testify/require`. Use `sqlmock` for database mocking. Create temp directories with `t.TempDir()`.

```go
func TestConfig_setDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name:   "all defaults applied",
			config: Config{},
			expected: Config{
				MaxOpenConns: DefaultMaxOpenConns,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			cfg.setDefaults()
			assert.Equal(t, tt.expected.MaxOpenConns, cfg.MaxOpenConns)
		})
	}
}
```

### Concurrency
Use `sync.Mutex` for shared state. Pass mutex pointers when needed. Always unlock (prefer `defer mu.Unlock()`).

### Constants
Group related constants in `const` blocks. Use typed constants when type safety matters. Document units in comments.

```go
const (
	DefaultMaxOpenConns    = 10
	DefaultConnMaxLifetime = 3 * time.Minute
)

type DateFormat string

const (
	DateFormatUnix   DateFormat = "unix"
	DateFormatISO    DateFormat = "iso"
)
```

### SQL Queries
Use backticks for table/column names. Use `?` placeholders for parameterized queries. Build queries with `fmt.Sprintf()` only for table/column names. Never use string concatenation for WHERE clause values.

```go
sqlQuery := fmt.Sprintf("SELECT * FROM `%s`.`%s`", database, table)
if whereClause != "" {
    sqlQuery += " WHERE " + whereClause
}
```

## Key Dependencies
- `github.com/spf13/cobra` - CLI framework
- `github.com/go-sql-driver/mysql` - MySQL driver
- `github.com/parquet-go/parquet-go` - Parquet file handling
- `github.com/sirupsen/logrus` - Structured logging
- `github.com/stretchr/testify` - Testing assertions
- `github.com/DATA-DOG/go-sqlmock` - SQL mocking for tests

## Go Version
- Go 1.24+ (see `go.mod`)
