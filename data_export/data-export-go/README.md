# Data Export Tool (Go)

使用 Go 语言开发的数据库导出工具，支持 MySQL、PostgreSQL 等数据库，用于导出指定表的数据到 Parquet 文件。

## 功能特性

- 🚀 **高性能**: 使用 Go 语言编写，性能优于 Python 版本
- 🗄️ **多数据库支持**: 支持 MySQL、PostgreSQL (Oracle 支持开发中)
- 📦 **Parquet 格式**: 导出为标准 Parquet 格式，支持 Snappy 压缩
- 🔄 **批量导出**: 支持大数据量的分批导出，避免内存溢出
- 📁 **分区导出**: 支持按列分区导出到不同子目录
- 🔍 **数据验证**: 导出后自动验证 Parquet 文件完整性
- 🖥️ **友好 CLI**: 使用 Cobra 框架，支持完整的命令行参数

## 安装

### 从源码编译

```bash
# 克隆仓库
git clone <repository-url>
cd data-export-go

# 编译
make build

# 编译后的二进制文件位于 ./bin/data-export
```

### 依赖要求

- Go 1.24 或更高版本
- 支持的数据库:
  - MySQL (默认)
  - PostgreSQL
  - Oracle (开发中，需要 CGO 和 Oracle Instant Client)

## 使用方法

### 基本导出 (MySQL)

```bash
./bin/data-export \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --password 123456 \
  --database test_db \
  --table users \
  --output ./export_data
```

### 导出 PostgreSQL

```bash
./bin/data-export \
  --db-type postgres \
  --host 127.0.0.1 \
  --port 5432 \
  --user postgres \
  --password 123456 \
  --database test_db \
  --table users \
  --output ./export_data
```

### 分批导出

```bash
./bin/data-export \
  --host 127.0.0.1 \
  --port 9030 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --batch-size 50000
```

### 带 WHERE 条件

```bash
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t orders \
  -o ./export_data \
  --where "create_time >= '2024-01-01'"
```

### 分区导出

```bash
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --partition-by country
```

输出结构：
```
export_data/
├── country=CN/
│   └── users_20240318_120000_batch0001.parquet
├── country=US/
│   └── users_20240318_120000_batch0001.parquet
└── ...
```

### 仅查看表信息

```bash
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  --info-only
```

### 导出并验证

```bash
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --verify
```

### 仅验证文件

```bash
./bin/data-export \
  --verify-only ./export_data/*.parquet
```

### 指定日期格式导出

```bash
# 导出为 Unix 时间戳 (默认)
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --date-format unix

# 导出为 ISO 8601 格式字符串
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --date-format iso

# 导出为自定义格式字符串 (默认: 2006-01-02 15:04:05)
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --date-format string

# 导出为指定自定义格式
./bin/data-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --date-format string \
  --date-time-layout "2006-01-02"
```

## CLI 参数

| 参数 | 简写 | 说明 | 是否必填 |
|------|------|------|----------|
| `--db-type` | | 数据库类型: `mysql`(默认), `postgres`, `oracle` | |
| `--host` | | 数据库主机地址 | ✅ |
| `--port` | | 数据库端口 (默认: mysql=3306, postgres=5432, oracle=1521) | |
| `--user` | `-u` | 用户名 | ✅ |
| `--password` | `-p` | 密码 | ✅ |
| `--database` | `-d` | 数据库名 | ✅ |
| `--table` | `-t` | 表名 | ✅ (导出模式) |
| `--output` | `-o` | 输出目录 | ✅ (导出模式) |
| `--batch-size` | | 每批读取的行数 | |
| `--where` | | WHERE 过滤条件 | |
| `--partition-by` | | 按列分区导出 | |
| `--info-only` | | 仅显示表信息 | |
| `--no-confirm` | | 大表导出时不提示确认 | |
| `--verify` | | 导出后验证文件 | |
| `--verify-only` | | 仅验证指定文件 | |
| `--date-format` | | 日期时间格式：`unix`(默认,毫秒时间戳), `iso`(ISO8601), `string`(自定义字符串) | |
| `--date-time-layout` | | 自定义日期格式模板（用于 string 格式，Go time layout） | |

## 与 Python 版本的对比

| 特性 | Python 版本 | Go 版本 |
|------|------------|---------|
| 性能 | 中等 | 高 (2-5x 提升) |
| 内存使用 | 较高 | 低 (流式处理) |
| 二进制大小 | 大 (需 Python 环境) | 小 (单二进制文件) |
| 启动速度 | 慢 | 快 |
| 并发支持 | 有限 | 原生支持 |

## 项目结构

```
data-export-go/
├── cmd/
│   └── data-export/        # 主程序入口
│       └── main.go
├── internal/
│   ├── cli/                 # CLI 命令处理
│   ├── config/              # 配置管理
│   ├── database/            # 数据库连接管理
│   ├── export/              # 导出编排器
│   ├── metadata/            # 元数据服务
│   ├── parquet/             # Parquet 写入器
│   ├── query/               # 查询执行器
│   └── verify/              # 文件验证器
├── pkg/
│   └── types/               # 公共类型定义
├── go.mod                   # Go 模块定义
├── go.sum                   # 依赖校验
├── Makefile                 # 构建脚本
└── README.md                # 本文档
```

## 开发

### 运行测试

```bash
# 运行所有测试
make test

# 运行特定包测试
go test ./internal/export/... -v

# 生成测试覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 代码规范

```bash
# 格式化代码
go fmt ./...

# 运行 linter
golangci-lint run

# 检查依赖漏洞
govulncheck ./...
```

## 性能优化建议

1. **批处理大小**: 根据可用内存调整 `--batch-size` (建议 10000-100000)
2. **连接池**: 默认配置适合大多数场景，可根据并发需求调整
3. **分区导出**: 对于大表，使用 `--partition-by` 分散导出压力
4. **网络优化**: 确保 Doris FE 和导出工具之间的网络带宽充足

## 故障排除

### 连接失败

```
连接失败: dial tcp 127.0.0.1:3306: connect: connection refused
```
- 检查数据库服务是否运行
- 确认端口是否正确 (MySQL: 3306, PostgreSQL: 5432, Oracle: 1521)
- 检查防火墙设置
- 确认 `--db-type` 参数是否设置正确

### 内存不足

```
fatal error: runtime: out of memory
```
- 减小 `--batch-size` 参数
- 使用 `--partition-by` 分批导出
- 增加系统内存

### 权限不足

```
Error 1045: Access denied for user
```
- 检查用户名和密码
- 确认用户有 SELECT 权限
- 检查数据库白名单/访问控制配置

## 许可证

MIT License

## 致谢

本项目最初是对原 Python 版本 `doris_export.py` 的 Go 语言重构版本，现已扩展支持多种数据库类型。
