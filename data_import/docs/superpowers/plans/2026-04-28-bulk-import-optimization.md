# Bulk Import Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace row-by-row INSERT with database-native bulk import methods for each supported database, achieving 10-1000x import speedup.

**Architecture:** Each database type gets its own optimized `BulkInsert` method in `internal/db/`. The `Manager` struct dispatches to the correct implementation via `switch m.dbType`. Doris uses HTTP Stream Load (bypasses SQL entirely), MySQL uses `LOAD DATA LOCAL INFILE`, PostgreSQL uses `lib/pq` `CopyIn`, Oracle uses `go-ora` `NewBatch` array binding, SQLite uses multi-row INSERT with PRAGMA tuning. The engine layer calls `BulkInsert` instead of `InsertData`.

**Tech Stack:** Go 1.25+, existing drivers + `net/http` for Doris Stream Load. No new driver dependencies required — all optimizations use existing `go.mod` libraries.

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/db/manager.go` | Add `BulkInsert` dispatcher method, keep `InsertData` as fallback |
| `internal/db/bulk_doris.go` | Doris Stream Load via HTTP PUT — sends raw Parquet file |
| `internal/db/bulk_mysql.go` | MySQL `LOAD DATA LOCAL INFILE` via `RegisterReaderHandler` |
| `internal/db/bulk_postgres.go` | PostgreSQL `pq.CopyIn` COPY protocol |
| `internal/db/bulk_oracle.go` | Oracle `go_ora.NewBatch` array binding |
| `internal/db/bulk_sqlite.go` | SQLite multi-row INSERT + PRAGMA setup |
| `internal/importer/engine.go` | Call `BulkInsert` instead of `InsertData`, pass `filePath` for Doris |
| `internal/config/config.go` | Add `FEHTTPPort` for Doris Stream Load HTTP port (default 8030) |
| `cmd/data_import/main.go` | Pass config to engine for Doris path |

---

## Task 1: Add Doris HTTP Port Config

**Files:**
- Modify: `internal/config/config.go:12-23` (Config struct)
- Modify: `internal/config/config.go:31-48` (BindFlags)
- Modify: `internal/config/config.go:84-95` (Validate default ports)
- Modify: `cmd/data_import/main.go:44-53` (print config info)

**Context:** Doris has two ports: 9030 (MySQL protocol, used for checking DB/table existence) and 8030 (HTTP port, used for Stream Load). We need both. Add a new `FEHTTPPort` field. The existing `Port` stays as the MySQL protocol port (9030). `FEHTTPPort` defaults to 8030.

- [ ] **Step 1: Add FEHTTPPort field and flag to config**

In `internal/config/config.go`, add to `Config` struct:
```go
FEHTTPPort int // Doris FE HTTP 端口（Stream Load 使用，默认 8030）
```

In `BindFlags`, add:
```go
cmd.Flags().IntVar(&c.FEHTTPPort, "fe-http-port", 0, "Doris FE HTTP 端口（Stream Load 使用，默认 8030）")
```

In `Validate()`, inside the `if c.Port == 0` block, add:
```go
// Set default Doris HTTP port
if c.DBType == "doris" && c.FEHTTPPort == 0 {
    c.FEHTTPPort = 8030
}
```

- [ ] **Step 2: Build and verify compilation**

Run: `go build ./cmd/data_import`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add FEHTTPPort for Doris Stream Load"
```

---

## Task 2: Doris Stream Load Implementation

**Files:**
- Create: `internal/db/bulk_doris.go`

**Context:** Doris Stream Load sends the raw Parquet file via HTTP PUT. No row parsing needed. The Manager already has the MySQL connection for existence checks. Stream Load uses a separate HTTP request. The `Manager` needs access to `Config` to get `Host`, `User`, `Password`, `FEHTTPPort`, and `DBName`.

- [ ] **Step 1: Add SetConfig method and config field to Manager**

In `internal/db/manager.go`, add a `config` field and setter to `Manager`:

```go
type Manager struct {
	db     *sql.DB
	dbType DBType
	dsn    string
	cfg    *config.Config // optional, needed for Doris Stream Load
}

// SetConfig sets the config for bulk import methods that need connection details.
func (m *Manager) SetConfig(cfg *config.Config) {
	m.cfg = cfg
}
```

Add import for config package.

- [ ] **Step 2: Create bulk_doris.go with Stream Load implementation**

Create `internal/db/bulk_doris.go`:

```go
package db

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lush/data_import/internal/config"
)

// streamLoadResponse represents Doris Stream Load API response
type streamLoadResponse struct {
	TxnID            int64  `json:"TxnId"`
	Label            string `json:"Label"`
	Status           string `json:"Status"`
	Message          string `json:"Message"`
	NumberTotalRows  int64  `json:"NumberTotalRows"`
	NumberLoadedRows int64  `json:"NumberLoadedRows"`
	NumberFilteredRows int64 `json:"NumberFilteredRows"`
	LoadBytes        int64  `json:"LoadBytes"`
	LoadTimeMs       int    `json:"LoadTimeMs"`
	ErrorURL         string `json:"ErrorURL"`
}

// bulkInsertDoris sends the Parquet file to Doris via Stream Load HTTP API.
// Doris natively reads Parquet schema and maps columns by name.
func (m *Manager) bulkInsertDoris(tableName string, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("[doris] config is required for Stream Load")
	}

	feHost := cfg.Host
	fePort := cfg.FEHTTPPort
	if fePort == 0 {
		fePort = 8030
	}

	url := fmt.Sprintf("http://%s:%d/api/%s/%s/_stream_load",
		feHost, fePort, cfg.DBName, tableName)

	file, err := os.Open(cfg.FilePath)
	if err != nil {
		return fmt.Errorf("[doris] failed to open parquet file for stream load: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("[doris] failed to stat parquet file: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, file)
	if err != nil {
		return fmt.Errorf("[doris] failed to create stream load request: %w", err)
	}

	// Basic Auth
	req.SetBasicAuth(cfg.User, cfg.Password)

	// Stream Load headers
	req.Header.Set("format", "parquet")
	req.Header.Set("label", fmt.Sprintf("data_import_%d", time.Now().UnixMilli()))
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	req.Header.Set("timeout", "3600")
	req.Header.Set("strict_mode", "true")
	req.Header.Set("max_filter_ratio", "0")

	client := &http.Client{Timeout: 3600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("[doris] stream load request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("[doris] failed to read stream load response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[doris] stream load HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result streamLoadResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("[doris] failed to parse stream load response: %w", err)
	}

	if result.Status != "Success" {
		errMsg := result.Message
		if result.ErrorURL != "" {
			errMsg += fmt.Sprintf(" (error details: %s)", result.ErrorURL)
		}
		return fmt.Errorf("[doris] stream load failed: %s, filtered %d/%d rows",
			errMsg, result.NumberFilteredRows, result.NumberTotalRows)
	}

	fmt.Printf("[导入引擎] Stream Load 完成: %d 行, %d 字节, 耗时 %dms\n",
		result.NumberLoadedRows, result.LoadBytes, result.LoadTimeMs)

	return nil
}
```

- [ ] **Step 3: Build and verify compilation**

Run: `go build ./cmd/data_import`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/db/bulk_doris.go internal/db/manager.go
git commit -m "feat(db): add Doris Stream Load bulk import via HTTP PUT"
```

---

## Task 3: MySQL LOAD DATA LOCAL INFILE Implementation

**Files:**
- Create: `internal/db/bulk_mysql.go`

**Context:** Use `go-sql-driver/mysql`'s `RegisterReaderHandler` to stream CSV data from Parquet rows. Convert Parquet rows to CSV in-memory, then use `LOAD DATA LOCAL INFILE 'Reader::name'`. This avoids writing temp files and leverages MySQL's fastest import protocol.

- [ ] **Step 1: Create bulk_mysql.go**

Create `internal/db/bulk_mysql.go`:

```go
package db

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// bulkInsertMySQL uses LOAD DATA LOCAL INFILE with an in-memory CSV reader.
// This is 20-50x faster than row-by-row INSERT.
func (m *Manager) bulkInsertMySQL(tableName string, columns []string, rows []map[string]any) error {
	// Convert rows to CSV in memory
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			val := normalizeValue(row[col])
			if val == nil {
				record[i] = "\\N" // MySQL LOAD DATA NULL representation
			} else {
				record[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := w.Write(record); err != nil {
			return fmt.Errorf("[mysql] failed to write CSV row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("[mysql] CSV flush error: %w", err)
	}

	// Register the CSV data as a reader handler
	handlerName := fmt.Sprintf("parquet_data_%d", len(rows))
	mysql.RegisterReaderHandler(handlerName, func() io.Reader {
		return bytes.NewReader(buf.Bytes())
	})
	defer mysql.DeregisterReaderHandler(handlerName)

	// Build column list
	colList := make([]string, len(columns))
	for i, col := range columns {
		colList[i] = fmt.Sprintf("`%s`", col)
	}

	query := fmt.Sprintf(
		"LOAD DATA LOCAL INFILE 'Reader::%s' INTO TABLE `%s` "+
			"FIELDS TERMINATED BY ',' ENCLOSED BY '\"' ESCAPED BY '\\\\' "+
			"LINES TERMINATED BY '\\n' (%s)",
		handlerName, tableName, strings.Join(colList, ", "))

	result, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("[mysql] LOAD DATA LOCAL INFILE failed: %w", err)
	}

	affected, _ := result.RowsAffected()
	fmt.Printf("[导入引擎] LOAD DATA 完成: %d 行\n", affected)

	return nil
}
```

**Note:** Requires `import "io"` and `import "github.com/go-sql-driver/mysql"` in the file. The DSN also needs `allowAllFiles=true` or use the `RegisterReaderHandler` approach shown above (which doesn't need DSN changes).

**DSN change required:** In `config.go` `buildMySQLDSN()`, append `?allowAllFiles=true&interpolateParams=true` to the DSN. Also update `buildDorisDSN()` similarly since Doris uses MySQL driver.

- [ ] **Step 2: Update MySQL/Doris DSN to include performance params**

In `internal/config/config.go`, update `buildMySQLDSN()`:
```go
func (c *Config) buildMySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?allowAllFiles=true&interpolateParams=true",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}
```

Update `buildDorisDSN()` similarly:
```go
func (c *Config) buildDorisDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?allowAllFiles=true&interpolateParams=true",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}
```

- [ ] **Step 3: Build and verify compilation**

Run: `go build ./cmd/data_import`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/db/bulk_mysql.go internal/config/config.go
git commit -m "feat(db): add MySQL LOAD DATA LOCAL INFILE bulk import"
```

---

## Task 4: PostgreSQL CopyIn Implementation

**Files:**
- Create: `internal/db/bulk_postgres.go`

**Context:** Use `lib/pq`'s `CopyIn` function which implements the PostgreSQL COPY protocol. This is ~6x faster than row-by-row INSERT and doesn't require adding `pgx` as a new dependency. If the user wants even more speed later, they can migrate to `pgx CopyFrom` (264x faster).

- [ ] **Step 1: Create bulk_postgres.go**

Create `internal/db/bulk_postgres.go`:

```go
package db

import (
	"fmt"

	"github.com/lib/pq"
)

// bulkInsertPostgreSQL uses pq.CopyIn for COPY protocol bulk import.
// This is ~6x faster than row-by-row INSERT.
func (m *Manager) bulkInsertPostgreSQL(tableName string, columns []string, rows []map[string]any) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("[postgresql] failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build quoted column names for CopyIn
	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = fmt.Sprintf(`"%s"`, col)
	}

	// pq.CopyIn creates a COPY FROM STDIN statement
	stmt, err := tx.Prepare(pq.CopyIn(tableName, colNames...))
	if err != nil {
		return fmt.Errorf("[postgresql] failed to prepare COPY statement: %w", err)
	}
	defer stmt.Close()

	for i, row := range rows {
		values := make([]any, len(columns))
		for j, col := range columns {
			values[j] = normalizeValue(row[col])
		}
		if _, err := stmt.Exec(values...); err != nil {
			return fmt.Errorf("[postgresql] COPY row %d failed: %w", i, err)
		}
	}

	// Flush the COPY stream
	if _, err := stmt.Exec(); err != nil {
		return fmt.Errorf("[postgresql] COPY flush failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[postgresql] failed to commit COPY transaction: %w", err)
	}

	fmt.Printf("[导入引擎] COPY 导入完成: %d 行\n", len(rows))
	return nil
}
```

- [ ] **Step 2: Build and verify compilation**

Run: `go build ./cmd/data_import`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/db/bulk_postgres.go
git commit -m "feat(db): add PostgreSQL CopyIn bulk import"
```

---

## Task 5: Oracle Array Binding Implementation

**Files:**
- Create: `internal/db/bulk_oracle.go`

**Context:** Use `go-ora/v2`'s `NewBatch` array binding. Build columnar arrays from row data, then `Exec` with `NewBatch` wrapped params. This gives 10-50x speedup with zero new dependencies.

- [ ] **Step 1: Create bulk_oracle.go**

Create `internal/db/bulk_oracle.go`:

```go
package db

import (
	"fmt"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"
)

// bulkInsertOracle uses go-ora NewBatch array binding for bulk import.
// This is 10-50x faster than row-by-row INSERT.
func (m *Manager) bulkInsertOracle(tableName string, columns []string, rows []map[string]any) error {
	const batchSize = 10000

	// Build INSERT statement with Oracle placeholders
	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = fmt.Sprintf(`"%s"`, strings.ToUpper(col))
	}
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf(":%d", i+1)
	}
	sqlText := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
		strings.ToUpper(tableName),
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "))

	totalRows := len(rows)
	for start := 0; start < totalRows; start += batchSize {
		end := start + batchSize
		if end > totalRows {
			end = totalRows
		}
		batch := rows[start:end]
		nRows := end - start

		// Build columnar arrays for array binding
		args := make([]any, len(columns))
		for j, col := range columns {
			arr := make([]any, nRows)
			for k, row := range batch {
				arr[k] = normalizeValue(row[col])
			}
			args[j] = go_ora.NewBatch(arr)
		}

		result, err := m.db.Exec(sqlText, args...)
		if err != nil {
			return fmt.Errorf("[oracle] bulk insert failed at row %d: %w", start, err)
		}

		affected, _ := result.RowsAffected()
		fmt.Printf("[导入引擎] Array Binding 批次 %d-%d: %d 行\n", start, end-1, affected)
	}

	return nil
}
```

- [ ] **Step 2: Build and verify compilation**

Run: `go build ./cmd/data_import`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/db/bulk_oracle.go
git commit -m "feat(db): add Oracle array binding bulk import via go-ora NewBatch"
```

---

## Task 6: SQLite Multi-Row INSERT + PRAGMA Implementation

**Files:**
- Create: `internal/db/bulk_sqlite.go`
- Modify: `internal/config/config.go:134-139` (buildSQLiteDSN — add PRAGMA params)

**Context:** SQLite optimization is two-layer: (1) PRAGMA settings in DSN connection string for WAL mode and relaxed sync, (2) multi-row INSERT with batch size 500. Together these give 500-1000x speedup.

- [ ] **Step 1: Update SQLite DSN with PRAGMA parameters**

In `internal/config/config.go`, update `buildSQLiteDSN()`:

```go
func (c *Config) buildSQLiteDSN() string {
	var dbPath string
	if c.Host == "" || c.Host == "localhost" {
		dbPath = c.DBName + ".db"
	} else {
		dbPath = c.Host
	}
	// PRAGMA optimizations for bulk import performance
	return fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-20000&_busy_timeout=5000&_txlock=immediate",
		dbPath)
}
```

- [ ] **Step 2: Create bulk_sqlite.go**

Create `internal/db/bulk_sqlite.go`:

```go
package db

import (
	"fmt"
	"strings"
)

// bulkInsertSQLite uses multi-row INSERT with batch size 500.
// Combined with PRAGMA WAL + NORMAL sync, this gives 500-1000x speedup.
func (m *Manager) bulkInsertSQLite(tableName string, columns []string, rows []map[string]any) error {
	const batchSize = 500

	// Build column list
	colList := make([]string, len(columns))
	for i, col := range columns {
		colList[i] = fmt.Sprintf("`%s`", col)
	}
	colListStr := strings.Join(colList, ", ")

	// Single-row placeholder template: (?, ?, ?)
	singleRowPH := "(" + strings.Repeat("?,", len(columns))
	singleRowPH = singleRowPH[:len(singleRowPH)-1] + ")"

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("[sqlite] failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for offset := 0; offset < len(rows); offset += batchSize {
		end := offset + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[offset:end]

		// Build multi-row VALUES: (?,?,?),(?,?,?),...
		placeholders := make([]string, len(batch))
		for i := range placeholders {
			placeholders[i] = singleRowPH
		}

		query := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s",
			tableName, colListStr, strings.Join(placeholders, ","))

		// Flatten all row values into a single slice
		values := make([]any, 0, len(batch)*len(columns))
		for _, row := range batch {
			for _, col := range columns {
				values = append(values, normalizeValue(row[col]))
			}
		}

		result, err := tx.Exec(query, values...)
		if err != nil {
			return fmt.Errorf("[sqlite] batch insert failed at row %d: %w", offset, err)
		}

		affected, _ := result.RowsAffected()
		_ = affected
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[sqlite] failed to commit: %w", err)
	}

	fmt.Printf("[导入引擎] Multi-row INSERT 完成: %d 行\n", len(rows))
	return nil
}
```

- [ ] **Step 3: Build and verify compilation**

Run: `go build ./cmd/data_import`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/db/bulk_sqlite.go internal/config/config.go
git commit -m "feat(db): add SQLite multi-row INSERT bulk import with PRAGMA optimization"
```

---

## Task 7: Add BulkInsert Dispatcher and Wire Engine

**Files:**
- Modify: `internal/db/manager.go` — add `BulkInsert` method
- Modify: `internal/importer/engine.go` — add `cfg` field, call `BulkInsert` with Doris fast-path
- Modify: `cmd/data_import/main.go` — pass config to engine

**Context:** The `BulkInsert` method dispatches to the correct bulk implementation. For Doris, it takes a special path: it doesn't need rows at all — it sends the raw Parquet file. The engine needs to know this and skip row reading for Doris.

- [ ] **Step 1: Add BulkInsert dispatcher to Manager**

In `internal/db/manager.go`, add the dispatcher method:

```go
// BulkInsert performs a bulk insert using the database-native fastest method.
// For Doris, pass nil rows and non-nil cfg — it uses Stream Load (sends raw Parquet file).
// For other databases, rows is required.
func (m *Manager) BulkInsert(tableName string, columns []string, rows []map[string]any, cfg *config.Config) error {
	switch m.dbType {
	case Doris:
		return m.bulkInsertDoris(tableName, cfg)
	case MySQL:
		return m.bulkInsertMySQL(tableName, columns, rows)
	case PostgreSQL:
		return m.bulkInsertPostgreSQL(tableName, columns, rows)
	case Oracle:
		return m.bulkInsertOracle(tableName, columns, rows)
	case SQLite:
		return m.bulkInsertSQLite(tableName, columns, rows)
	default:
		// Fallback to row-by-row for unknown types
		return m.InsertData(tableName, columns, rows)
	}
}
```

Add import for config package.

- [ ] **Step 2: Update Engine to support config and Doris fast-path**

In `internal/importer/engine.go`, update the `Engine` struct and methods:

```go
package importer

import (
	"fmt"

	"github.com/lush/data_import/internal/config"
	"github.com/lush/data_import/internal/db"
	"github.com/lush/data_import/internal/parquet"
)

// Engine 数据导入引擎，协调 Parquet 读取和数据库写入
type Engine struct {
	reader    *parquet.Reader
	dbManager *db.Manager
	cfg       *config.Config
}

// NewEngine 创建新的导入引擎实例
func NewEngine(reader *parquet.Reader, dbManager *db.Manager) *Engine {
	return &Engine{
		reader:    reader,
		dbManager: dbManager,
	}
}

// SetConfig sets the config for bulk import methods that need connection details.
func (e *Engine) SetConfig(cfg *config.Config) {
	e.cfg = cfg
	e.dbManager.SetConfig(cfg)
}

// Import 执行完整的数据导入流程
func (e *Engine) Import(tableName string) error {
	fmt.Printf("[导入引擎] 开始导入数据到表 '%s'\n", tableName)

	columns := e.reader.GetColumns()
	dbType := e.dbManager.Type()

	// Doris Stream Load fast-path: send raw Parquet file, skip row parsing
	if dbType == db.Doris {
		fmt.Printf("[导入引擎] 使用 Stream Load 直接发送 Parquet 文件（跳过行解析）\n")
		if err := e.dbManager.BulkInsert(tableName, nil, nil, e.cfg); err != nil {
			return fmt.Errorf("Stream Load 失败: %w", err)
		}
		fmt.Printf("[导入引擎] 导入完成！\n")
		return nil
	}

	// For other databases: read rows, then bulk insert
	fmt.Printf("[导入引擎] 步骤 1/2: 读取 Parquet 文件数据...\n")
	rows, err := e.reader.ReadRows()
	if err != nil {
		return fmt.Errorf("读取数据失败: %w", err)
	}
	fmt.Printf("[导入引擎] 成功读取 %d 行数据\n", len(rows))

	fmt.Printf("[导入引擎] 步骤 2/2: 批量导入数据到表 '%s'...\n", tableName)
	if err := e.dbManager.BulkInsert(tableName, columns, rows, e.cfg); err != nil {
		return fmt.Errorf("批量导入失败: %w", err)
	}

	fmt.Printf("[导入引擎] 导入完成！总计导入 %d 行数据到表 '%s'\n", len(rows), tableName)
	return nil
}

// ImportWithValidation 带数据库和表存在性验证的导入方法
func (e *Engine) ImportWithValidation(dbName string, tableName string) error {
	fmt.Printf("[导入引擎] 开始带验证的导入流程\n")
	fmt.Printf("[导入引擎] 检查数据库 '%s' 是否存在...\n", dbName)

	exists, err := e.dbManager.CheckDatabaseExists(dbName)
	if err != nil {
		return fmt.Errorf("检查数据库失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("数据库 '%s' 不存在，请先创建数据库\n\n提示: 手动创建数据库命令:\n  CREATE DATABASE %s;", dbName, dbName)
	}
	fmt.Printf("[导入引擎] 数据库 '%s' 已存在\n", dbName)

	fmt.Printf("[导入引擎] 检查表 '%s' 是否存在...\n", tableName)
	tableExists, err := e.dbManager.CheckTableExists(tableName)
	if err != nil {
		return fmt.Errorf("检查表失败: %w", err)
	}
	if !tableExists {
		return fmt.Errorf("表 '%s' 不存在，请先创建表\n\n提示: 请确保目标表已存在后再执行导入", tableName)
	}
	fmt.Printf("[导入引擎] 表 '%s' 已存在，开始执行导入...\n", tableName)

	return e.Import(tableName)
}
```

- [ ] **Step 3: Update main.go to pass config to engine**

In `cmd/data_import/main.go`, after creating the engine, add:

```go
engine := importer.NewEngine(reader, dbManager)
engine.SetConfig(cfg) // Pass config for Doris Stream Load and future optimizations
```

Replace the existing single line `engine := importer.NewEngine(reader, dbManager)` with these two lines.

- [ ] **Step 4: Build and verify compilation**

Run: `go build ./cmd/data_import`
Expected: clean build

- [ ] **Step 5: Commit**

```bash
git add internal/db/manager.go internal/importer/engine.go cmd/data_import/main.go
git commit -m "feat(engine): wire BulkInsert dispatcher with Doris Stream Load fast-path"
```

---

## Task 8: Fix Compilation Issues and Integration Test

**Files:**
- Any files with compilation errors from previous tasks

**Context:** The bulk_mysql.go uses `mysql.RegisterReaderHandler` and `mysql.DeregisterReaderHandler` which need the correct import. Also need to verify `io` import is present. Run a full build and fix any issues.

- [ ] **Step 1: Run full build**

Run: `go build ./cmd/data_import`

If errors, fix them. Common issues to watch for:
- Missing `io` import in `bulk_mysql.go`
- Missing `database/sql` import if referenced
- Config import path must be `"github.com/lush/data_import/internal/config"`

- [ ] **Step 2: Run `go vet` for static analysis**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Run `go fmt` for consistent formatting**

Run: `go fmt ./...`

- [ ] **Step 4: Final commit if any fixes**

```bash
git add -A
git commit -m "fix: resolve compilation issues in bulk import implementations"
```

---

## Task 9: Update AGENTS.md with New Architecture

**Files:**
- Modify: `AGENTS.md`

**Context:** Update the project documentation to reflect the new bulk import architecture.

- [ ] **Step 1: Update Project Structure section in AGENTS.md**

Add the new bulk import files to the project structure:

```
internal/
  db/
    manager.go               # Connection management, BulkInsert dispatcher
    bulk_doris.go            # Doris Stream Load via HTTP PUT
    bulk_mysql.go            # MySQL LOAD DATA LOCAL INFILE
    bulk_postgres.go         # PostgreSQL COPY protocol via pq.CopyIn
    bulk_oracle.go           # Oracle array binding via go-ora NewBatch
    bulk_sqlite.go           # SQLite multi-row INSERT + PRAGMA optimization
    oracle_driver.go         # Build-tag-gated Oracle driver registration
```

- [ ] **Step 2: Commit**

```bash
git add AGENTS.md
git commit -m "docs: update AGENTS.md with bulk import architecture"
```

---

## Dependency Summary

| Task | Depends On | New Dependencies |
|------|-----------|-----------------|
| Task 1 (Config) | None | None |
| Task 2 (Doris) | Task 1 | None (uses `net/http`) |
| Task 3 (MySQL) | None | None (uses existing `go-sql-driver/mysql`) |
| Task 4 (PostgreSQL) | None | None (uses existing `lib/pq`) |
| Task 5 (Oracle) | None | None (uses existing `go-ora/v2`) |
| Task 6 (SQLite) | None | None (uses existing `go-sqlite3`) |
| Task 7 (Wire Engine) | Tasks 1-6 | None |
| Task 8 (Fix/Verify) | Task 7 | None |
| Task 9 (Docs) | Task 8 | None |

**Tasks 1-6 can be parallelized** — they are independent implementations.

---

## Expected Performance Improvements

| Database | Method | Expected Speedup |
|----------|--------|-----------------|
| Doris | Stream Load (raw Parquet HTTP PUT) | 10-100x |
| MySQL | LOAD DATA LOCAL INFILE | 20-50x |
| PostgreSQL | pq.CopyIn (COPY protocol) | ~6x |
| Oracle | go-ora NewBatch (Array Binding) | 10-50x |
| SQLite | Multi-row INSERT + PRAGMA | 500-1000x |

**Future optimizations (not in this plan):**
- PostgreSQL: migrate from `lib/pq` to `pgx/v5` `CopyFrom` for binary COPY (264x vs row-by-row)
- Doris: use `doris-streamloader` CLI for parallel multi-file loading
- Oracle: explore `BulkCopy` Direct Path API for multi-million row loads
