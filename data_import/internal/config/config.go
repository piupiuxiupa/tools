package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Config 保存命令行参数配置
type Config struct {
	FilePath  string // Parquet 文件路径
	DBType    string // 数据库类型：mysql, postgres, sqlite, oracle, doris
	DBName    string // 数据库名
	TableName string // 表名

	// 数据库连接参数
	Host     string // 数据库主机
	Port     int    // 数据库端口
	User     string // 数据库用户名
	Password string // 数据库密码
}

// ParseFlags 解析命令行参数并返回配置
func ParseFlags() (*Config, error) {
	config := &Config{}

	// 定义命令行参数 - GNU 标准格式：短参 -f，长参 --file
	flag.StringVar(&config.FilePath, "f", "", "Parquet 文件路径（短参 -f，长参 --file）")
	flag.StringVar(&config.FilePath, "file", "", "Parquet 文件路径")

	flag.StringVar(&config.DBType, "t", "mysql", "数据库类型（短参 -t，长参 --type）：mysql, postgres, sqlite, oracle, doris")
	flag.StringVar(&config.DBType, "type", "mysql", "数据库类型：mysql, postgres, sqlite, oracle, doris（默认：mysql）")

	flag.StringVar(&config.DBName, "n", "", "数据库名称（短参 -n，长参 --name）")
	flag.StringVar(&config.DBName, "name", "", "数据库名称")

	flag.StringVar(&config.TableName, "T", "", "表名（短参 -T，长参 --table）")
	flag.StringVar(&config.TableName, "table", "", "表名")

	// 数据库连接参数
	flag.StringVar(&config.Host, "h", "localhost", "数据库主机地址（短参 -h，长参 --host）")
	flag.StringVar(&config.Host, "host", "localhost", "数据库主机地址（默认：localhost）")

	flag.IntVar(&config.Port, "P", 0, "数据库端口（短参 -P，长参 --port）。MySQL默认3306，PostgreSQL默认5432，Oracle默认1521，Doris默认9030")
	flag.IntVar(&config.Port, "port", 0, "数据库端口。MySQL默认3306，PostgreSQL默认5432，Oracle默认1521，Doris默认9030")

	flag.StringVar(&config.User, "u", "", "数据库用户名（短参 -u，长参 --user）")
	flag.StringVar(&config.User, "user", "", "数据库用户名")

	flag.StringVar(&config.Password, "p", "", "数据库密码（短参 -p，长参 --password）")
	flag.StringVar(&config.Password, "password", "", "数据库密码")

	// 自定义帮助信息
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "将 Parquet 文件导入到数据库\n\n")
		fmt.Fprintf(os.Stderr, "选项:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  # MySQL 导入\n")
		fmt.Fprintf(os.Stderr, "  %s -f data.parquet -n mydb -T mytable -u root -p password -h localhost -P 3306\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # PostgreSQL 导入\n")
		fmt.Fprintf(os.Stderr, "  %s -f data.parquet -t postgres -n mydb -T mytable -u postgres -p secret -h localhost\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # SQLite 导入\n")
		fmt.Fprintf(os.Stderr, "  %s -f data.parquet -t sqlite -n mydb -T mytable -h ./mydata.db\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Oracle 导入\n")
		fmt.Fprintf(os.Stderr, "  %s -f data.parquet -t oracle -n ORCL -T mytable -u scott -p tiger -h localhost -P 1521\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Doris 导入\n")
		fmt.Fprintf(os.Stderr, "  %s -f data.parquet -t doris -n mydb -T mytable -u root -p password -h localhost -P 9030\n", os.Args[0])
	}

	flag.Parse()

	// 验证必需参数
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 设置默认端口
	if config.Port == 0 {
		switch config.DBType {
		case "mysql":
			config.Port = 3306
		case "postgres":
			config.Port = 5432
		case "oracle":
			config.Port = 1521
		case "doris":
			config.Port = 9030
		}
	}

	return config, nil
}

// Validate 验证配置参数
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
	// 格式: user:password@tcp(host:port)/dbname
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", c.User, c.Password, c.Host, c.Port, c.DBName)
	return dsn
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
	// SQLite 使用主机作为文件路径
	if c.Host == "" || c.Host == "localhost" {
		return c.DBName + ".db"
	}
	return c.Host
}

// buildOracleDSN 构建 Oracle DSN (使用 go-ora 驱动)
func (c *Config) buildOracleDSN() string {
	// go-ora 支持的格式: oracle://user:password@host:port/service_name
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s", c.User, c.Password, c.Host, c.Port, c.DBName)
	return dsn
}

// buildDorisDSN 构建 Doris DSN
func (c *Config) buildDorisDSN() string {
	// Doris 使用 MySQL 协议，格式与 MySQL 相同
	// 格式: user:password@tcp(host:port)/dbname
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", c.User, c.Password, c.Host, c.Port, c.DBName)
	return dsn
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
	sb.WriteString("  -h, --host string    数据库主机地址（默认：localhost）\n")
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
