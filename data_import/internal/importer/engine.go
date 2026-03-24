package importer

import (
	"fmt"
	"github.com/lush/data_import/internal/db"
	"github.com/lush/data_import/internal/parquet"
)

// Engine 数据导入引擎，协调 Parquet 读取和数据库写入
type Engine struct {
	reader    *parquet.Reader
	dbManager *db.Manager
}

// NewEngine 创建新的导入引擎实例
func NewEngine(reader *parquet.Reader, dbManager *db.Manager) *Engine {
	return &Engine{
		reader:    reader,
		dbManager: dbManager,
	}
}

// Import 执行完整的数据导入流程
func (e *Engine) Import(tableName string) error {
	fmt.Printf("[导入引擎] 开始导入数据到表 '%s'\n", tableName)

	// 步骤1: 获取列名
	fmt.Printf("[导入引擎] 步骤 1/3: 读取 Parquet 文件列信息...\n")
	columns := e.reader.GetColumns()
	if columns == nil {
		fmt.Printf("[导入引擎] 错误: 获取列信息失败\n")
		return fmt.Errorf("获取列信息失败")
	}
	fmt.Printf("[导入引擎] 成功获取 %d 个列: %v\n", len(columns), columns)

	// 步骤2: 读取数据
	fmt.Printf("[导入引擎] 步骤 2/3: 读取 Parquet 文件数据...\n")
	rows, err := e.reader.ReadRows()
	if err != nil {
		fmt.Printf("[导入引擎] 错误: 读取数据失败 - %v\n", err)
		return fmt.Errorf("读取数据失败: %w", err)
	}
	fmt.Printf("[导入引擎] 成功读取 %d 行数据\n", len(rows))

	// 步骤3: 插入数据
	fmt.Printf("[导入引擎] 步骤 3/3: 插入数据到表 '%s'...\n", tableName)
	if err := e.dbManager.InsertData(tableName, columns, rows); err != nil {
		fmt.Printf("[导入引擎] 错误: 插入数据失败 - %v\n", err)
		return fmt.Errorf("插入数据失败: %w", err)
	}
	fmt.Printf("[导入引擎] 成功插入 %d 行数据\n", len(rows))

	fmt.Printf("[导入引擎] 导入完成！总计导入 %d 行数据到表 '%s'\n", len(rows), tableName)
	return nil
}

// ImportWithValidation 带数据库和表存在性验证的导入方法
func (e *Engine) ImportWithValidation(dbName string, tableName string) error {
	fmt.Printf("[导入引擎] 开始带验证的导入流程\n")
	fmt.Printf("[导入引擎] 检查数据库 '%s' 是否存在...\n", dbName)

	// 检查数据库是否存在
	exists, err := e.dbManager.CheckDatabaseExists(dbName)
	if err != nil {
		fmt.Printf("[导入引擎] 错误: 检查数据库失败 - %v\n", err)
		return fmt.Errorf("检查数据库失败: %w", err)
	}

	if !exists {
		fmt.Printf("[导入引擎] 错误: 数据库 '%s' 不存在\n", dbName)
		return fmt.Errorf("数据库 '%s' 不存在，请先创建数据库\n\n提示: 手动创建数据库命令:\n  CREATE DATABASE %s;", dbName, dbName)
	}

	fmt.Printf("[导入引擎] 数据库 '%s' 已存在\n", dbName)

	// 检查表是否存在
	fmt.Printf("[导入引擎] 检查表 '%s' 是否存在...\n", tableName)
	tableExists, err := e.dbManager.CheckTableExists(tableName)
	if err != nil {
		fmt.Printf("[导入引擎] 错误: 检查表失败 - %v\n", err)
		return fmt.Errorf("检查表失败: %w", err)
	}

	if !tableExists {
		fmt.Printf("[导入引擎] 错误: 表 '%s' 不存在\n", tableName)
		return fmt.Errorf("表 '%s' 不存在，请先创建表\n\n提示: 请确保目标表已存在后再执行导入", tableName)
	}

	fmt.Printf("[导入引擎] 表 '%s' 已存在，开始执行导入...\n", tableName)

	// 执行导入
	return e.Import(tableName)
}
