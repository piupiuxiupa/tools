package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// DBType 表示数据库类型
type DBType string

const (
	MySQL      DBType = "mysql"
	PostgreSQL DBType = "postgresql"
	SQLite     DBType = "sqlite"
	Oracle     DBType = "oracle"
	Doris      DBType = "doris"
)

// DBTypeFromString 从字符串解析数据库类型
func DBTypeFromString(s string) DBType {
	switch strings.ToLower(s) {
	case "mysql":
		return MySQL
	case "postgres", "postgresql":
		return PostgreSQL
	case "sqlite", "sqlite3":
		return SQLite
	case "oracle":
		return Oracle
	case "doris":
		return Doris
	default:
		return MySQL // 默认为 MySQL
	}
}

// Manager 管理数据库连接和操作
type Manager struct {
	db     *sql.DB
	dbType DBType
	dsn    string
}

func getDriverName(dbType DBType) (string, error) {
	switch dbType {
	case MySQL, Doris:
		return "mysql", nil
	case PostgreSQL:
		return "postgres", nil
	case SQLite:
		return "sqlite3", nil
	case Oracle:
		return oracleDriverName()
	default:
		return "", fmt.Errorf("[%s] unsupported database type", dbType)
	}
}

// NewManager 创建一个新的数据库管理器
func NewManager(dbType DBType, dsn string) (*Manager, error) {
	if dsn == "" {
		return nil, fmt.Errorf("[%s] DSN cannot be empty", dbType)
	}

	driverName, err := getDriverName(dbType)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to open database: %w", dbType, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("[%s] failed to ping database: %w", dbType, err)
	}

	return &Manager{
		db:     db,
		dbType: dbType,
		dsn:    dsn,
	}, nil
}

// CheckDatabaseExists 检查数据库/库是否存在
func (m *Manager) CheckDatabaseExists(dbName string) (bool, error) {
	if dbName == "" {
		return false, fmt.Errorf("[%s] database name cannot be empty", m.dbType)
	}

	switch m.dbType {
	case MySQL, Doris:
		return m.checkMySQLDatabaseExists(dbName)
	case PostgreSQL:
		return m.checkPostgreSQLDatabaseExists(dbName)
	case SQLite:
		return m.checkSQLiteDatabaseExists(dbName)
	case Oracle:
		return m.checkOracleDatabaseExists(dbName)
	default:
		return false, fmt.Errorf("[%s] unsupported database type", m.dbType)
	}
}

func (m *Manager) checkMySQLDatabaseExists(dbName string) (bool, error) {
	query := "SELECT 1 FROM information_schema.schemata WHERE schema_name = ?"
	var result int
	err := m.db.QueryRow(query, dbName).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("[%s] failed to check database existence: %w", m.dbType, err)
	}
	return true, nil
}

func (m *Manager) checkPostgreSQLDatabaseExists(dbName string) (bool, error) {
	query := "SELECT 1 FROM pg_database WHERE datname = $1"
	var result int
	err := m.db.QueryRow(query, dbName).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("[%s] failed to check database existence: %w", m.dbType, err)
	}
	return true, nil
}

func (m *Manager) checkSQLiteDatabaseExists(_ string) (bool, error) {
	if m.dsn == ":memory:" {
		return true, nil
	}

	// 移除可能存在的连接参数
	dsn := m.dsn
	if idx := strings.Index(dsn, "?"); idx != -1 {
		dsn = dsn[:idx]
	}

	_, err := os.Stat(dsn)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("[%s] failed to check database file: %w", m.dbType, err)
	}
	return true, nil
}

func (m *Manager) checkOracleDatabaseExists(dbName string) (bool, error) {
	// Oracle 检查数据库通过查询 v$database 或尝试连接
	// 简化处理：尝试执行一个简单查询
	var result int
	err := m.db.QueryRow("SELECT 1 FROM dual").Scan(&result)
	if err != nil {
		return false, fmt.Errorf("[%s] failed to check database existence: %w", m.dbType, err)
	}
	return true, nil
}

// CheckTableExists 检查表是否存在
func (m *Manager) CheckTableExists(tableName string) (bool, error) {
	if tableName == "" {
		return false, fmt.Errorf("[%s] table name cannot be empty", m.dbType)
	}

	switch m.dbType {
	case MySQL, Doris:
		return m.checkMySQLTableExists(tableName)
	case PostgreSQL:
		return m.checkPostgreSQLTableExists(tableName)
	case SQLite:
		return m.checkSQLiteTableExists(tableName)
	case Oracle:
		return m.checkOracleTableExists(tableName)
	default:
		return false, fmt.Errorf("[%s] unsupported database type", m.dbType)
	}
}

func (m *Manager) checkMySQLTableExists(tableName string) (bool, error) {
	query := "SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
	var result int
	err := m.db.QueryRow(query, tableName).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("[%s] failed to check table existence: %w", m.dbType, err)
	}
	return true, nil
}

func (m *Manager) checkPostgreSQLTableExists(tableName string) (bool, error) {
	query := "SELECT 1 FROM information_schema.tables WHERE table_name = $1"
	var result int
	err := m.db.QueryRow(query, tableName).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("[%s] failed to check table existence: %w", m.dbType, err)
	}
	return true, nil
}

func (m *Manager) checkSQLiteTableExists(tableName string) (bool, error) {
	query := "SELECT 1 FROM sqlite_master WHERE type='table' AND name = ?"
	var result int
	err := m.db.QueryRow(query, tableName).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("[%s] failed to check table existence: %w", m.dbType, err)
	}
	return true, nil
}

func (m *Manager) checkOracleTableExists(tableName string) (bool, error) {
	query := "SELECT 1 FROM user_tables WHERE table_name = UPPER(:1)"
	var result int
	err := m.db.QueryRow(query, tableName).Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("[%s] failed to check table existence: %w", m.dbType, err)
	}
	return true, nil
}

// CreateTable 创建数据表
func (m *Manager) CreateTable(tableName string, columns []string) error {
	if tableName == "" {
		return fmt.Errorf("[%s] table name cannot be empty", m.dbType)
	}
	if len(columns) == 0 {
		return fmt.Errorf("[%s] columns cannot be empty", m.dbType)
	}

	// 构建列定义
	var columnDefs []string
	for i, col := range columns {
		col = strings.TrimSpace(col)
		if col == "" {
			return fmt.Errorf("[%s] column name at index %d cannot be empty", m.dbType, i)
		}
		// 根据数据库类型添加列类型
		colType := m.getColumnType()
		columnDefs = append(columnDefs, fmt.Sprintf("`%s` %s", col, colType))
	}

	// 根据数据库类型构建创建表语句
	var query string
	switch m.dbType {
	case MySQL, Doris:
		// Doris 兼容 MySQL 语法
		query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (%s)", tableName, strings.Join(columnDefs, ", "))
	case PostgreSQL:
		// PostgreSQL 使用双引号
		pgColumns := make([]string, len(columns))
		for i, col := range columns {
			pgColumns[i] = fmt.Sprintf(`"%s" TEXT`, col)
		}
		query = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS "%s" (%s)`, tableName, strings.Join(pgColumns, ", "))
	case SQLite:
		query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (%s)", tableName, strings.Join(columnDefs, ", "))
	case Oracle:
		// Oracle 语法
		oraColumns := make([]string, len(columns))
		for i, col := range columns {
			oraColumns[i] = fmt.Sprintf(`"%s" VARCHAR2(4000)`, strings.ToUpper(col))
		}
		query = fmt.Sprintf(`CREATE TABLE "%s" (%s)`, strings.ToUpper(tableName), strings.Join(oraColumns, ", "))
	default:
		return fmt.Errorf("[%s] unsupported database type", m.dbType)
	}

	_, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("[%s] failed to create table %s: %w", m.dbType, tableName, err)
	}

	return nil
}

// getColumnType 返回适合当前数据库类型的列类型
func (m *Manager) getColumnType() string {
	switch m.dbType {
	case MySQL, Doris:
		return "TEXT"
	case PostgreSQL:
		return "TEXT"
	case SQLite:
		return "TEXT"
	case Oracle:
		return "VARCHAR2(4000)"
	default:
		return "TEXT"
	}
}

// InsertData 插入数据
func (m *Manager) InsertData(tableName string, columns []string, rows []map[string]any) error {
	if tableName == "" {
		return fmt.Errorf("[%s] table name cannot be empty", m.dbType)
	}
	if len(columns) == 0 {
		return fmt.Errorf("[%s] columns cannot be empty", m.dbType)
	}
	if len(rows) == 0 {
		return nil // 没有数据需要插入
	}

	// 验证所有行都有必需的列
	for i, row := range rows {
		for _, col := range columns {
			if _, ok := row[col]; !ok {
				return fmt.Errorf("[%s] row %d missing column '%s'", m.dbType, i, col)
			}
		}
	}

	// 构建插入语句
	placeholders := make([]string, len(columns))
	var query string

	switch m.dbType {
	case MySQL, Doris, SQLite:
		for i := range placeholders {
			placeholders[i] = "?"
		}
		colNames := make([]string, len(columns))
		for i, col := range columns {
			colNames[i] = fmt.Sprintf("`%s`", col)
		}
		query = fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
			tableName,
			strings.Join(colNames, ", "),
			strings.Join(placeholders, ", "))
	case PostgreSQL:
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		colNames := make([]string, len(columns))
		for i, col := range columns {
			colNames[i] = fmt.Sprintf(`"%s"`, col)
		}
		query = fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
			tableName,
			strings.Join(colNames, ", "),
			strings.Join(placeholders, ", "))
	case Oracle:
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf(":%d", i+1)
		}
		colNames := make([]string, len(columns))
		for i, col := range columns {
			colNames[i] = fmt.Sprintf(`"%s"`, strings.ToUpper(col))
		}
		query = fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
			strings.ToUpper(tableName),
			strings.Join(colNames, ", "),
			strings.Join(placeholders, ", "))
	default:
		return fmt.Errorf("[%s] unsupported database type", m.dbType)
	}

	// 准备语句
	stmt, err := m.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("[%s] failed to prepare insert statement: %w", m.dbType, err)
	}
	defer stmt.Close()

	// 开始事务
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("[%s] failed to begin transaction: %w", m.dbType, err)
	}
	defer tx.Rollback()

	// 使用事务准备语句
	txStmt := tx.Stmt(stmt)

	// 插入每一行数据
	for i, row := range rows {
		values := make([]any, len(columns))
		for j, col := range columns {
			values[j] = normalizeValue(row[col])
		}

		_, err := txStmt.Exec(values...)
		if err != nil {
			return fmt.Errorf("[%s] failed to insert row %d: %w", m.dbType, i, err)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[%s] failed to commit transaction: %w", m.dbType, err)
	}

	return nil
}

// Close 关闭数据库连接
func (m *Manager) Close() error {
	if m.db != nil {
		if err := m.db.Close(); err != nil {
			return fmt.Errorf("[%s] failed to close database connection: %w", m.dbType, err)
		}
	}
	return nil
}

// DB 返回底层 sql.DB 实例（供测试或高级用法使用）
func (m *Manager) DB() *sql.DB {
	return m.db
}

// Type 返回数据库类型
func (m *Manager) Type() DBType {
	return m.dbType
}

// normalizeValue 将空字符串转换为 nil，以便正确处理 JSON 等类型字段
func normalizeValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return v
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return v
	default:
		return value
	}
}
