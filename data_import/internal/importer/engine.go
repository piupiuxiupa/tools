package importer

import (
	"fmt"

	"github.com/lush/data_import/internal/config"
	"github.com/lush/data_import/internal/db"
	"github.com/lush/data_import/internal/parquet"
)

type Engine struct {
	reader    *parquet.Reader
	dbManager *db.Manager
	cfg       *config.Config
}

func NewEngine(reader *parquet.Reader, dbManager *db.Manager) *Engine {
	return &Engine{
		reader:    reader,
		dbManager: dbManager,
	}
}

func (e *Engine) SetConfig(cfg *config.Config) {
	e.cfg = cfg
	e.dbManager.SetConfig(cfg)
}

func (e *Engine) Import(tableName string) error {
	fmt.Printf("[导入引擎] 开始导入数据到表 '%s'\n", tableName)

	dbType := e.dbManager.Type()

	// Doris Stream Load: 直接发送 Parquet 文件，跳过行解析
	if dbType == db.Doris {
		fmt.Printf("[导入引擎] 使用 Stream Load 直接发送 Parquet 文件\n")
		if err := e.dbManager.BulkInsert(tableName, nil, nil, e.cfg); err != nil {
			return fmt.Errorf("Stream Load 失败: %w", err)
		}
		fmt.Printf("[导入引擎] 导入完成！\n")
		return nil
	}

	columns := e.reader.GetColumns()
	if columns == nil {
		return fmt.Errorf("获取列信息失败")
	}

	fmt.Printf("[导入引擎] 读取 Parquet 文件数据...\n")
	rows, err := e.reader.ReadRows()
	if err != nil {
		return fmt.Errorf("读取数据失败: %w", err)
	}
	fmt.Printf("[导入引擎] 成功读取 %d 行数据\n", len(rows))

	fmt.Printf("[导入引擎] 批量导入数据到表 '%s'...\n", tableName)
	if err := e.dbManager.BulkInsert(tableName, columns, rows, e.cfg); err != nil {
		return fmt.Errorf("批量导入失败: %w", err)
	}

	fmt.Printf("[导入引擎] 导入完成！总计导入 %d 行数据到表 '%s'\n", len(rows), tableName)
	return nil
}

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
