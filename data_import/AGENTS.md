# AGENTS.md — data_import

Go CLI tool that imports Parquet files into MySQL, PostgreSQL, SQLite, Oracle, and Doris databases.

## Build / Test / Lint Commands

### Build
```bash
make build                    # Build for current platform → bin/data_import
make build-linux-amd64        # Cross-compile Linux AMD64
make build-linux-arm64        # Cross-compile Linux ARM64
make build-linux-armv7        # Cross-compile Linux ARM v7
make build-darwin-arm64       # Cross-compile macOS ARM64
make build-windows-amd64      # Cross-compile Windows AMD64
make build-all                # Build all platforms
go build -o bin/data_import ./cmd/data_import   # Manual build
```

Build injects version/time via ldflags: `-X main.Version=... -X main.BuildTime=...`

### Test
```bash
go test ./...                                    # All tests
go test -v ./internal/db/...                     # Single package (verbose)
go test -v ./internal/importer/... -run TestName  # Single test
go test -v -coverprofile=coverage.out ./...      # With coverage
go tool cover -html=coverage.out                 # View coverage
```

> **No tests exist yet.** When adding tests, place `*_test.go` alongside the source file in the same package (white-box testing).

### Lint / Format
```bash
make fmt         # go fmt ./...
make lint        # golangci-lint run (if installed, otherwise skipped)
make deps        # go mod download && go mod tidy
```

## Project Structure
```
cmd/data_import/main.go      # Entry point — cobra CLI, wires config → reader → db → engine
internal/
  config/config.go           # CLI flags, validation, DSN builder per DB type
  db/
    manager.go               # Connection management, BulkInsert dispatcher, table/column checks
    bulk_doris.go            # Doris Stream Load via HTTP PUT (sends raw Parquet file)
    bulk_mysql.go            # MySQL LOAD DATA LOCAL INFILE (in-memory CSV via RegisterReaderHandler)
    bulk_postgres.go         # PostgreSQL COPY protocol via pq.CopyIn
    bulk_oracle.go           # Oracle array binding via go-ora NewBatch (batch size 10000)
    bulk_sqlite.go           # SQLite multi-row INSERT (batch 500) + PRAGMA WAL optimization
    oracle_driver.go         # Oracle driver registration (import-only, no build tags)
  importer/engine.go         # Orchestrates: read parquet → validate DB/table → bulk insert (Doris fast-path skips row reading)
  parquet/reader.go          # Parquet file reader (schema discovery, row extraction via GenericReader)
```

## Code Style

### Go Version
Go 1.25+ (see `go.mod`).

### Imports
Group with blank lines: stdlib → third-party → local.

```go
import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"

	"github.com/lush/data_import/internal/config"
	"github.com/lush/data_import/internal/db"
)
```

### Naming
- **Packages**: lowercase, single word — `config`, `db`, `importer`, `parquet`
- **Exported**: PascalCase — `NewManager`, `ImportWithValidation`, `DBType`
- **Unexported**: camelCase — `buildMySQLDSN`, `normalizeValue`, `getColumnPaths`
- **Constants**: PascalCase exported, camelCase unexported — group in `const` blocks
- **Types**: Typed constants for domain concepts — `type DBType string`
- **Constructors**: `New*` pattern — `NewReader`, `NewManager`, `NewEngine`, `NewConfig`

### Error Handling
Always wrap with `fmt.Errorf` and `%w`. Include context. Prefix with `[dbType]` in db package.

```go
// db package — prefix with database type
return nil, fmt.Errorf("[%s] failed to open database: %w", m.dbType, err)

// Other packages — descriptive message
return nil, fmt.Errorf("failed to open parquet file: %w", err)
```

Never ignore errors. Never use `panic` in library code.

### CLI Framework
Uses **cobra** (`github.com/spf13/cobra`). Define flags in `config.BindFlags()`. Mark required flags with `cmd.MarkFlagRequired()`. Validate and set defaults in `Config.Validate()`.

**Important**: The host flag uses uppercase `-H` (not `-h`, which conflicts with help). The table flag uses uppercase `-T`.

### Struct Design
- Exported struct fields with inline Chinese comments describing purpose.
- Use `interface{}` for parquet column data (`map[string]interface{}`).
- Keep constructors simple — validation belongs in separate `Validate()` methods.
- `SetConfig()` pattern used to pass `*config.Config` into `Engine` and `Manager` (needed for Doris Stream Load).

### Database Patterns
- One `Manager` struct handles all DB types via `switch m.dbType`.
- Use `database/sql` directly — no ORM.
- Placeholder style varies by driver: MySQL/SQLite use `?`, PostgreSQL uses `$1`, Oracle uses `:1`.
- Quote identifiers: backticks for MySQL/SQLite, double quotes for PostgreSQL/Oracle.
- **Oracle always UPPER-cases table and column names** in queries.
- Always use parameterized queries. Use `fmt.Sprintf` only for table/column names, never for values.
- Use transactions for batch inserts with `defer tx.Rollback()`.
- `normalizeValue()` converts empty strings and empty byte slices to `nil` — critical for null-safe inserts.

### Resource Cleanup
Always `defer Close()` / `defer Rollback()` immediately after acquisition:

```go
reader, err := parquet.NewReader(cfg.FilePath)
if err != nil { return err }
defer reader.Close()

tx, err := db.Begin()
if err != nil { return err }
defer tx.Rollback()
```

### Logging / Output
Uses `fmt.Printf` for user-facing progress messages. Prefix with `[导入引擎]` in importer. Chinese-language output throughout.

### Testing Conventions (when adding tests)
- Place `*_test.go` in same package (white-box).
- Table-driven tests with `t.Run` subtests.
- Use `t.Helper()` in test helpers.
- Use `t.TempDir()` for temp files, `sqlmock` for DB mocking.
- No external test dependencies yet — consider `testify` if needed.

## Key Dependencies
| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/parquet-go/parquet-go` | Parquet file reading (**not** xitongsys/parquet-go) |
| `github.com/go-sql-driver/mysql` | MySQL + Doris driver |
| `github.com/lib/pq` | PostgreSQL driver |
| `github.com/mattn/go-sqlite3` | SQLite driver (requires CGO) |
| `github.com/sijms/go-ora/v2` | Oracle driver (pure Go, no CGO) |

## Architecture Notes

### Doris Fast-Path
Doris bypasses row-level reading entirely. `Engine.Import()` detects `db.Doris` and calls `BulkInsert` with nil rows and the config — which sends the raw Parquet file via HTTP PUT Stream Load to the Doris FE HTTP port (default 8030, separate from the MySQL protocol port 9030). The Stream Load handler follows 307 redirects from FE to BE automatically.

### Config Flow
`Config` holds CLI flags → `Validate()` sets defaults (ports per DB type, Doris FE HTTP port) → `BuildDSN()` constructs the connection string per DB type → `SetConfig()` propagates config to `Manager` for Doris Stream Load.

### SQLite Connection String
Uses `file:<path>?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-20000&_busy_timeout=5000&_txlock=immediate` — WAL mode and busy timeout are critical for bulk write performance.

## Entry Point Flow
1. `main.go` → `cobra.Command` with `RunE: runImport`
2. `cfg.Validate()` → validate + set default ports
3. `cfg.BuildDSN()` → build connection string for selected DB
4. `parquet.NewReader()` → open file, lazy-load schema via `openFile()`
5. `db.NewManager()` → open + ping connection
6. `engine.SetConfig(cfg)` → propagate config to engine + manager
7. `engine.ImportWithValidation()` → check DB exists → check table exists → bulk insert
