# Doris Export Tool (Go)

使用 Go 语言重构的 Doris 数据导出工具，用于从 Doris 数据库导出指定表的数据到 Parquet 文件。

## 功能特性

- 🚀 **高性能**: 使用 Go 语言编写，性能优于 Python 版本
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
cd doris-export-go

# 编译
make build

# 编译后的二进制文件位于 ./bin/doris-export
```

### 依赖要求

- Go 1.22 或更高版本
- Doris 数据库 (MySQL 协议兼容)

## 使用方法

### 基本导出

```bash
./bin/doris-export \
  --host 127.0.0.1 \
  --port 9030 \
  --user root \
  --password 123456 \
  --database test_db \
  --table users \
  --output ./export_data
```

### 分批导出

```bash
./bin/doris-export \
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
./bin/doris-export \
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
./bin/doris-export \
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
./bin/doris-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  --info-only
```

### 导出并验证

```bash
./bin/doris-export \
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
./bin/doris-export \
  --verify-only ./export_data/*.parquet
```

### 指定日期格式导出

```bash
# 导出为 Unix 时间戳 (默认)
./bin/doris-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --date-format unix

# 导出为 ISO 8601 格式字符串
./bin/doris-export \
  --host 127.0.0.1 \
  -u root \
  -p 123456 \
  -d test_db \
  -t users \
  -o ./export_data \
  --date-format iso
```

## CLI 参数

| 参数 | 简写 | 说明 | 是否必填 |
|------|------|------|----------|
| `--host` | | Doris FE 主机地址 | ✅ |
| `--port` | | Doris FE 查询端口 (默认: 9030) | |
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
| `--date-format` | | 日期时间格式：`unix`(默认,毫秒时间戳), `iso`(ISO8601), `string`(字符串) | |

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
doris-export-go/
├── cmd/
│   └── doris-export/        # 主程序入口
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
连接失败: dial tcp 127.0.0.1:9030: connect: connection refused
```
- 检查 Doris FE 是否运行
- 确认端口 9030 是否正确
- 检查防火墙设置

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
- 检查 Doris 白名单配置

## 许可证

MIT License

## 致谢

本项目是对原 Python 版本 `doris_export.py` 的 Go 语言重构版本。
