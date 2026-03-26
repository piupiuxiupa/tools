package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/lush/data_import/internal/config"
	"github.com/lush/data_import/internal/db"
	"github.com/lush/data_import/internal/importer"
	"github.com/lush/data_import/internal/parquet"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "data_import",
	Short: "将 Parquet 文件导入到数据库",
	Long: `数据导入工具 - 支持 MySQL、PostgreSQL、SQLite、Oracle、Doris

将 Parquet 文件数据高效导入到各种数据库中。`,
	RunE: runImport,
}

var cfg = config.NewConfig()

func init() {
	// 绑定配置到命令行参数
	cfg.BindFlags(rootCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	// 验证配置
	if err := cfg.Validate(); err != nil {
		return err
	}

	// 打印配置信息
	fmt.Printf("=== Parquet 数据导入工具 ===\n")
	fmt.Printf("文件: %s\n", cfg.FilePath)
	fmt.Printf("数据库类型: %s\n", cfg.DBType)
	fmt.Printf("数据库: %s\n", cfg.DBName)
	fmt.Printf("表名: %s\n", cfg.TableName)
	if cfg.DBType != "sqlite" {
		fmt.Printf("主机: %s:%d\n", cfg.Host, cfg.Port)
		fmt.Printf("用户: %s\n", cfg.User)
	}
	fmt.Println()

	// 打开 Parquet 文件
	fmt.Printf("[1/4] 正在打开 Parquet 文件...\n")
	reader, err := parquet.NewReader(cfg.FilePath)
	if err != nil {
		return fmt.Errorf("无法打开 Parquet 文件: %w", err)
	}
	defer reader.Close()

	// 获取列信息
	columns := reader.GetColumns()
	fmt.Printf("      发现 %d 个列: %s\n", len(columns), strings.Join(columns, ", "))

	// 构建 DSN
	dsn := cfg.BuildDSN()

	// 创建数据库管理器
	fmt.Printf("[2/4] 正在连接数据库...\n")
	dbType := db.DBTypeFromString(cfg.DBType)
	dbManager, err := db.NewManager(dbType, dsn)
	if err != nil {
		return fmt.Errorf("无法连接数据库: %w", err)
	}
	defer dbManager.Close()
	fmt.Printf("      数据库连接成功\n")

	// 创建导入引擎
	fmt.Printf("[3/4] 准备导入...\n")
	engine := importer.NewEngine(reader, dbManager)

	// 执行导入（带数据库存在性验证）
	fmt.Printf("[4/4] 开始导入数据...\n")
	fmt.Println()

	if err := engine.ImportWithValidation(cfg.DBName, cfg.TableName); err != nil {
		// 检查是否是数据库不存在的错误
		if strings.Contains(err.Error(), "数据库") && strings.Contains(err.Error(), "不存在") {
			fmt.Println()
			fmt.Println("提示: 如果要创建数据库，请先手动创建:")
			switch cfg.DBType {
			case "mysql":
				fmt.Printf("  CREATE DATABASE %s;\n", cfg.DBName)
			case "postgres":
				fmt.Printf("  CREATE DATABASE %s;\n", cfg.DBName)
			case "sqlite":
				fmt.Printf("  将自动创建 SQLite 数据库文件\n")
			}
		}
		return err
	}

	fmt.Println()
	fmt.Println("导入完成！")
	return nil
}
