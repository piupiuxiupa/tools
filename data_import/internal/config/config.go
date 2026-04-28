package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Config 保存命令行参数配置
type Config struct {
	FilePath  string // Parquet 文件路径
	DBType    string // 数据库类型：mysql, postgres, sqlite, oracle, doris
	DBName    string // 数据库名
	TableName string // 表名

	// 数据库连接参数
	Host       string // 数据库主机
	Port       int    // 数据库端口
	User       string // 数据库用户名
	Password   string // 数据库密码
	FEHTTPPort int    // Doris FE HTTP 端口（Stream Load 使用，默认 8030）
}

// NewConfig 创建新的配置实例
func NewConfig() *Config {
	return &Config{}
}

// BindFlags 将配置绑定到 cobra 命令的 flags
func (c *Config) BindFlags(cmd *cobra.Command) {
	// 必需参数
	cmd.Flags().StringVarP(&c.FilePath, "file", "f", "", "Parquet 文件路径")
	cmd.Flags().StringVarP(&c.DBName, "name", "n", "", "数据库名称")
	cmd.Flags().StringVarP(&c.TableName, "table", "T", "", "表名")
	cmd.Flags().StringVarP(&c.User, "user", "u", "", "数据库用户名（SQLite可不填）")

	// 可选参数
	cmd.Flags().StringVarP(&c.DBType, "type", "t", "mysql", "数据库类型：mysql, postgres, sqlite, oracle, doris")
	cmd.Flags().StringVarP(&c.Host, "host", "H", "localhost", "数据库主机地址")
	cmd.Flags().IntVarP(&c.Port, "port", "P", 0, "数据库端口。MySQL默认3306，PostgreSQL默认5432，Oracle默认1521，Doris默认9030")
	cmd.Flags().IntVar(&c.FEHTTPPort, "fe-http-port", 0, "Doris FE HTTP 端口（Stream Load 使用，默认 8030）")
	cmd.Flags().StringVarP(&c.Password, "password", "p", "", "数据库密码")

	// 标记必需参数
	_ = cmd.MarkFlagRequired("file")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("table")
}

// Validate 验证配置参数并设置默认值
func (c *Config) Validate() error {
	var missing []string

	if c.FilePath == "" {
		missing = append(missing, "-f/--file")
	}
	if c.DBName == "" {
		missing = append(missing, "-n/--name (数据库名)")
	}
	if c.TableName == "" {
		missing = append(missing, "-T/--table (表名)")
	}
	if c.User == "" && c.DBType != "sqlite" {
		missing = append(missing, "-u/--user")
	}

	if len(missing) > 0 {
		return fmt.Errorf("缺少必需参数: %s\n\n提示: 使用 -h 或 --help 查看帮助信息\n\n示例:\n  ./data_import -f data.parquet -n mydb -T mytable -u root -p password", strings.Join(missing, ", "))
	}

	// 验证数据库类型
	validDBTypes := map[string]bool{
		"mysql":    true,
		"postgres": true,
		"sqlite":   true,
		"oracle":   true,
		"doris":    true,
	}
	if !validDBTypes[c.DBType] {
		return fmt.Errorf("无效的数据库类型: %s，可选值: mysql, postgres, sqlite, oracle, doris", c.DBType)
	}

	// 设置默认端口
	if c.Port == 0 {
		switch c.DBType {
		case "mysql":
			c.Port = 3306
		case "postgres":
			c.Port = 5432
		case "oracle":
			c.Port = 1521
		case "doris":
			c.Port = 9030
		}
	}

	// Set default Doris HTTP port
	if c.DBType == "doris" && c.FEHTTPPort == 0 {
		c.FEHTTPPort = 8030
	}

	return nil
}

// BuildDSN 根据配置构建数据源连接字符串
func (c *Config) BuildDSN() string {
	switch c.DBType {
	case "mysql":
		return c.buildMySQLDSN()
	case "postgres":
		return c.buildPostgresDSN()
	case "sqlite":
		return c.buildSQLiteDSN()
	case "oracle":
		return c.buildOracleDSN()
	case "doris":
		return c.buildDorisDSN()
	default:
		return ""
	}
}

// buildMySQLDSN 构建 MySQL DSN
func (c *Config) buildMySQLDSN() string {
	// 格式: user:password@tcp(host:port)/dbname?params
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?allowAllFiles=true&interpolateParams=true",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

// buildPostgresDSN 构建 PostgreSQL DSN
func (c *Config) buildPostgresDSN() string {
	// 格式: host=localhost port=5432 user=postgres password=secret sslmode=disable
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Password)
	return dsn
}

// buildSQLiteDSN 构建 SQLite DSN
func (c *Config) buildSQLiteDSN() string {
	var dbPath string
	if c.Host == "" || c.Host == "localhost" {
		dbPath = c.DBName + ".db"
	} else {
		dbPath = c.Host
	}
	return fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-20000&_busy_timeout=5000&_txlock=immediate",
		dbPath)
}

// buildOracleDSN 构建 Oracle DSN (使用 go-ora 驱动)
func (c *Config) buildOracleDSN() string {
	// go-ora 支持的格式: oracle://user:password@host:port/service_name
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s", c.User, c.Password, c.Host, c.Port, c.DBName)
	return dsn
}

// buildDorisDSN 构建 Doris DSN
func (c *Config) buildDorisDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?allowAllFiles=true&interpolateParams=true",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

// GetHelpMessage 返回使用帮助信息
func GetHelpMessage() string {
	var sb strings.Builder
	sb.WriteString("用法: data_import [选项]\n\n")
	sb.WriteString("将 Parquet 文件导入到数据库\n\n")
	sb.WriteString("必需参数:\n")
	sb.WriteString("  -f, --file string    Parquet 文件路径\n")
	sb.WriteString("  -n, --name string    数据库名称\n")
	sb.WriteString("  -T, --table string   表名\n")
	sb.WriteString("  -u, --user string    数据库用户名（SQLite可不填）\n\n")
	sb.WriteString("可选参数:\n")
	sb.WriteString("  -t, --type string    数据库类型，可选：mysql, postgres, sqlite, oracle, doris（默认：mysql）\n")
	sb.WriteString("  -H, --host string    数据库主机地址（默认：localhost）\n")
	sb.WriteString("  -P, --port int       数据库端口（MySQL默认3306，PostgreSQL默认5432，Oracle默认1521，Doris默认9030）\n")
	sb.WriteString("  -p, --password string 数据库密码\n\n")
	sb.WriteString("示例:\n")
	sb.WriteString("  ./data_import -f data.parquet -n mydb -T mytable -u root -p password\n")
	return sb.String()
}

// ErrHelpRequested 表示用户请求帮助
type ErrHelpRequested struct{}

func (e ErrHelpRequested) Error() string {
	return "帮助信息"
}

// ErrInvalidConfig 表示配置验证失败
var ErrInvalidConfig = errors.New("配置验证失败")
