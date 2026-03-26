# dbschema-sync

A Go CLI tool for synchronizing database table structures across different database systems.

## Supported Databases

- **MySQL** / MariaDB
- **PostgreSQL**
- **Oracle**
- **Apache Doris**

## Features

- Schema comparison between different database types
- DDL generation for table synchronization
- Type mapping between database systems
- Dry-run mode to preview changes
- Configurable table filtering

## Installation

```bash
go install github.com/lush/dbschema-sync/cmd/dbschema-sync@latest
```

Or build from source:

```bash
git clone https://github.com/lush/dbschema-sync.git
cd dbschema-sync
go build -o dbschema-sync ./cmd/dbschema-sync
```

## Usage

### Synchronize Schemas

```bash
# Compare and preview changes (dry-run)
dbschema-sync sync \
  --source mysql://user:pass@localhost/db1 \
  --target postgres://user:pass@localhost/db2 \
  --dry-run

# Apply changes with confirmation
dbschema-sync sync \
  --source mysql://user:pass@localhost/db1 \
  --target postgres://user:pass@localhost/db2

# Auto-apply changes without confirmation
dbschema-sync sync \
  --source mysql://user:pass@localhost/db1 \
  --target postgres://user:pass@localhost/db2 \
  --auto-apply

# Ignore specific tables
dbschema-sync sync \
  --source mysql://user:pass@localhost/db1 \
  --target postgres://user:pass@localhost/db2 \
  --ignore-tables=logs,tmp_*,cache_*
```

### Show Differences

```bash
dbschema-sync diff \
  --source mysql://user:pass@localhost/db1 \
  --target postgres://user:pass@localhost/db2 \
  --format json
```

### Extract Schema

```bash
dbschema-sync extract \
  --source mysql://user:pass@localhost/db \
  --output schema.json
```

### Validate Compatibility

```bash
dbschema-sync validate \
  --source mysql://user:pass@localhost/db1 \
  --target postgres://user:pass@localhost/db2
```

## DSN Format

```
driver://[username[:password]@][host[:port]]/database[?param=value]
```

Examples:
- `mysql://user:pass@localhost:3306/mydb`
- `postgres://user:pass@localhost:5432/mydb?sslmode=disable`
- `oracle://user:pass@localhost:1521/ORCL`
- `doris://user:pass@localhost:9030/mydb`

## Configuration

Configuration can be provided via:
1. Command-line flags
2. Environment variables (prefix: `DBSYNC_`)
3. Config file (YAML/JSON)

### Environment Variables

```bash
export DBSYNC_SOURCE_DRIVER=mysql
export DBSYNC_SOURCE_HOST=localhost
export DBSYNC_SOURCE_PORT=3306
export DBSYNC_SOURCE_DATABASE=mydb
export DBSYNC_SOURCE_USER=root
export DBSYNC_SOURCE_PASSWORD=secret

export DBSYNC_TARGET_DRIVER=postgres
export DBSYNC_TARGET_HOST=localhost
export DBSYNC_TARGET_PORT=5432
export DBSYNC_TARGET_DATABASE=mydb
export DBSYNC_TARGET_USER=postgres
export DBSYNC_TARGET_PASSWORD=secret

export DBSYNC_DRY_RUN=true
```

### Config File

```yaml
# $HOME/.dbschema-sync/config.yaml
source:
  driver: mysql
  host: localhost
  port: 3306
  database: mydb
  user: root
  password: secret

target:
  driver: postgres
  host: localhost
  port: 5432
  database: mydb
  user: postgres
  password: secret

sync:
  dry_run: true
  ignore_tables:
    - logs
    - tmp_*
```

## Type Mapping

The tool automatically maps data types between databases:

| MySQL | PostgreSQL | Oracle | Doris |
|-------|-----------|--------|-------|
| INT | INTEGER | NUMBER(10) | INT |
| VARCHAR | VARCHAR | VARCHAR2 | VARCHAR |
| TEXT | TEXT | CLOB | STRING |
| DATETIME | TIMESTAMP | TIMESTAMP | DATETIME |
| JSON | JSONB | CLOB | STRING |

## Commands

- `sync` - Synchronize schemas between databases
- `diff` - Show differences between schemas
- `extract` - Extract schema to a file
- `validate` - Validate schema compatibility

## Global Flags

- `-c, --config` - Config file path
- `-v, --verbose` - Enable verbose output

## Development

```bash
# Run tests
go test ./...

# Build
go build -o dbschema-sync ./cmd/dbschema-sync

# Install locally
go install ./cmd/dbschema-sync
```

## License

MIT
