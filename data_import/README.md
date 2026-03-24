# Parquet Import Tool

一个 Go 语言开发的命令行工具，用于将 Parquet 文件数据导入到各种数据库。

## 功能特性

- **多数据库支持**: MySQL、PostgreSQL、SQLite、Oracle、Doris
- **细粒度连接参数**: 支持 `-h`、`-u`、`-p`、`-P` 分别指定主机、用户、密码、端口
- **GNU 标准 CLI**: 支持短参 `-f` 和长参 `--file` 形式
- **友好错误提示**: 当数据库或表不存在时提供清晰的错误信息和解决建议
- **批量导入**: 支持批量插入提高性能

## 注意事项

- **表名必需**: 导入时必须指定目标表名 (`-T` 或 `--table`)
- **表必须存在**: 工具不会自动创建表，请确保目标表已存在

## 安装

```bash
# 克隆项目
git clone <repository>

# 进入目录
cd data_import

# 构建
go build -o data_import ./cmd/data_import

# 或者安装到 $GOPATH/bin
go install ./cmd/data_import
```

## Oracle 支持说明

Oracle 支持需要安装 Oracle Instant Client 和启用 CGO：

```bash
# macOS (使用 Homebrew)
brew install instantclient-basic
export CGO_ENABLED=1

# Linux
# 下载并安装 Oracle Instant Client
# https://www.oracle.com/database/technologies/instant-client/downloads.html
```

## 使用方法

### 基本用法

```bash
# MySQL 导入
./data_import -f data.parquet -n mydb -T mytable -u root -p password

# PostgreSQL 导入
./data_import -f data.parquet -t postgres -n mydb -T mytable -u postgres -p secret

# SQLite 导入
./data_import -f data.parquet -t sqlite -n mydb -T mytable -h ./mydata.db

# Oracle 导入
./data_import -f data.parquet -t oracle -n ORCL -T mytable -u scott -p tiger

# Doris 导入
./data_import -f data.parquet -t doris -n mydb -T mytable -u root -p password
```

### 命令行参数

| 短参 | 长参 | 必需 | 默认值 | 说明 |
|------|------|------|--------|------|
| `-f` | `--file` | 是 | - | Parquet 文件路径 |
| `-n` | `--name` | 是 | - | 数据库名称（Oracle为Service Name） |
| `-T` | `--table` | 是 | - | 目标表名（必须已存在） |
| `-u` | `--user` | 是* | - | 数据库用户名（*SQLite可不填） |
| `-p` | `--password` | 否 | - | 数据库密码 |
| `-h` | `--host` | 否 | localhost | 数据库主机地址 |
| `-P` | `--port` | 否 | 见下方 | 数据库端口 |
| `-t` | `--type` | 否 | mysql | 数据库类型：mysql, postgres, sqlite, oracle, doris |

### 默认端口

| 数据库 | 默认端口 |
|--------|----------|
| MySQL | 3306 |
| PostgreSQL | 5432 |
| Oracle | 1521 |
| Doris | 9030 |
| SQLite | - |

### 使用示例

#### MySQL 导入（本地默认端口）
```bash
./data_import \
  -f users.parquet \
  -n myapp \
  -T users \
  -u root \
  -p mypassword
```

#### MySQL 导入（指定主机和端口）
```bash
./data_import \
  -f users.parquet \
  -n myapp \
  -T users \
  -u root \
  -p mypassword \
  -h 192.168.1.100 \
  -P 3307
```

#### PostgreSQL 导入
```bash
./data_import \
  -f sales.parquet \
  -t postgres \
  -n analytics \
  -u postgres \
  -p secret \
  -h localhost \
  -P 5432 \
  -T sales_data
```

#### SQLite 导入
SQLite 使用 `-h` 参数指定数据库文件路径：
```bash
./data_import \
  -f events.parquet \
  -t sqlite \
  -n mydata \
  -T events \
  -h ./mydata.db
```

#### Oracle 导入
```bash
./data_import \
  -f employees.parquet \
  -t oracle \
  -n ORCL \
  -u scott \
  -p tiger \
  -h localhost \
  -P 1521 \
  -T employees
```

#### Doris 导入
Doris 兼容 MySQL 协议：
```bash
./data_import \
  -f events.parquet \
  -t doris \
  -n mydb \
  -u root \
  -p password \
  -h localhost \
  -P 9030 \
  -T events
```

### 长参形式示例

```bash
./data_import \
  --file data.parquet \
  --type mysql \
  --name mydb \
  --user root \
  --password secret \
  --host localhost \
  --port 3306 \
  --table mytable
```

## 项目结构

```
data_import/
├── cmd/data_import/
│   └── main.go              # 程序入口
├── internal/
│   ├── config/
│   │   └── config.go        # 配置解析
│   ├── db/
│   │   └── manager.go       # 数据库连接管理
│   ├── importer/
│   │   └── engine.go        # 导入引擎
│   └── parquet/
│       └── reader.go        # Parquet 文件读取
├── go.mod
├── go.sum
└── README.md
```

## 错误处理

### 数据库不存在

当指定的数据库不存在时，工具会输出友好的错误提示：

```
[导入引擎] 检查数据库 'mydb' 是否存在...
[导入引擎] 错误: 数据库 'mydb' 不存在

错误: 数据库 'mydb' 不存在，请先创建数据库

提示: 手动创建数据库命令:
  CREATE DATABASE mydb;
```

### 表不存在

当指定的表不存在时，工具会输出友好的错误提示：

```
[导入引擎] 检查表 'mytable' 是否存在...
[导入引擎] 错误: 表 'mytable' 不存在

错误: 表 'mytable' 不存在，请先创建表

提示: 请确保目标表已存在后再执行导入
```

### 缺少必需参数

当缺少必需的库名或表名时：

```
错误: 缺少必需参数: -n/--name (数据库名), -T/--table (表名)

提示: 使用 -h 或 --help 查看帮助信息

示例:
  ./data_import -f data.parquet -n mydb -T mytable -u root -p password
```

## 开发

### 依赖管理

```bash
go mod download
go mod tidy
```

### 构建

```bash
# Linux/Mac
go build -o data_import ./cmd/data_import

# Windows
go build -o data_import.exe ./cmd/data_import

# 启用 Oracle 支持（需要 CGO）
CGO_ENABLED=1 go build -o data_import ./cmd/data_import
```

### 运行测试

```bash
go test ./...
```

## 技术栈

- **Go 1.25+**
- **CLI**: 标准库 `flag` 包
- **Parquet**: `github.com/xitongsys/parquet-go`
- **数据库驱动**:
  - MySQL: `github.com/go-sql-driver/mysql`
  - PostgreSQL: `github.com/lib/pq`
  - SQLite: `github.com/mattn/go-sqlite3`
  - Oracle: `github.com/godror/godror` (需要 CGO)
  - Doris: `github.com/go-sql-driver/mysql` (兼容 MySQL 协议)

## License

MIT
