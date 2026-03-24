package parquet

import (
	"fmt"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

// Reader 提供 Parquet 文件的读取功能
type Reader struct {
	path string
	fr   interface{ Close() error }
	pr   *reader.ParquetReader
}

// NewReader 创建一个新的 Parquet 文件读取器
// 参数 path: Parquet 文件的路径
// 返回: 指向 Reader 的指针和可能的错误
func NewReader(path string) (*Reader, error) {
	fr, err := local.NewLocalFileReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file: %w", err)
	}

	// 使用 nil 作为 schema 参数，表示动态解析 schema
	pr, err := reader.NewParquetReader(fr, nil, 4)
	if err != nil {
		fr.Close()
		return nil, fmt.Errorf("failed to create parquet reader: %w", err)
	}

	return &Reader{
		path: path,
		fr:   fr,
		pr:   pr,
	}, nil
}

// GetColumns 获取 Parquet 文件中的所有列名
// 返回: 列名切片（简化名称，不含路径前缀）
func (r *Reader) GetColumns() []string {
	if r.pr == nil || r.pr.SchemaHandler == nil {
		return nil
	}

	schemaHandler := r.pr.SchemaHandler
	columns := make([]string, 0)

	for i, element := range schemaHandler.SchemaElements {
		if i == 0 {
			// 跳过根元素
			continue
		}
		// NumChildren == 0 表示这是一个实际的列（非嵌套结构）
		if element.GetNumChildren() == 0 && element.Name != "" {
			columns = append(columns, element.Name)
		}
	}

	return columns
}

// getColumnPaths 获取所有列的完整路径（用于读取数据）
func (r *Reader) getColumnPaths() []string {
	if r.pr == nil || r.pr.SchemaHandler == nil {
		return nil
	}

	schemaHandler := r.pr.SchemaHandler
	paths := make([]string, 0)

	for i, element := range schemaHandler.SchemaElements {
		if i == 0 {
			continue
		}
		if element.GetNumChildren() == 0 && element.Name != "" {
			pathStr := schemaHandler.IndexMap[int32(i)]
			paths = append(paths, pathStr)
		}
	}

	return paths
}

// ReadRows 读取 Parquet 文件中的所有行数据
// 返回: 每行数据以 map[string]interface{} 形式存储在切片中
func (r *Reader) ReadRows() ([]map[string]interface{}, error) {
	if r.pr == nil {
		return nil, fmt.Errorf("parquet reader is not initialized")
	}

	columns := r.GetColumns()
	if len(columns) == 0 {
		return []map[string]interface{}{}, nil
	}

	columnPaths := r.getColumnPaths()
	if len(columnPaths) == 0 {
		return []map[string]interface{}{}, nil
	}

	numRows := int(r.pr.GetNumRows())
	if numRows == 0 {
		return []map[string]interface{}{}, nil
	}

	// 预读取所有列数据
	columnData := make(map[string][]interface{})
	for i, path := range columnPaths {
		values, _, _, err := r.pr.ReadColumnByPath(path, int64(numRows))
		if err != nil {
			return nil, fmt.Errorf("failed to read column %s: %w", columns[i], err)
		}
		columnData[columns[i]] = values
	}

	// 组装行数据
	rows := make([]map[string]interface{}, numRows)
	for i := 0; i < numRows; i++ {
		row := make(map[string]interface{}, len(columns))
		for _, col := range columns {
			if data, ok := columnData[col]; ok && i < len(data) {
				row[col] = data[i]
			} else {
				row[col] = nil
			}
		}
		rows[i] = row
	}

	return rows, nil
}

// ReadRowsByColumns 按指定列读取行数据
// 参数 columnNames: 要读取的列名列表
// 返回: 每行数据只包含指定的列
func (r *Reader) ReadRowsByColumns(columnNames []string) ([]map[string]interface{}, error) {
	if r.pr == nil {
		return nil, fmt.Errorf("parquet reader is not initialized")
	}

	if len(columnNames) == 0 {
		return r.ReadRows()
	}

	numRows := int(r.pr.GetNumRows())
	if numRows == 0 {
		return []map[string]interface{}{}, nil
	}

	// 获取所有列路径映射
	columnPathMap := r.getColumnPathMap()

	// 读取指定列数据
	columnData := make(map[string][]interface{})
	for _, colName := range columnNames {
		path, ok := columnPathMap[colName]
		if !ok {
			return nil, fmt.Errorf("column %s not found", colName)
		}

		values, _, _, err := r.pr.ReadColumnByPath(path, int64(numRows))
		if err != nil {
			return nil, fmt.Errorf("failed to read column %s: %w", colName, err)
		}
		columnData[colName] = values
	}

	// 组装行数据
	rows := make([]map[string]interface{}, numRows)
	for i := 0; i < numRows; i++ {
		row := make(map[string]interface{}, len(columnNames))
		for _, col := range columnNames {
			if data, ok := columnData[col]; ok && i < len(data) {
				row[col] = data[i]
			} else {
				row[col] = nil
			}
		}
		rows[i] = row
	}

	return rows, nil
}

// getColumnPathMap 获取列名到路径的映射
func (r *Reader) getColumnPathMap() map[string]string {
	if r.pr == nil || r.pr.SchemaHandler == nil {
		return nil
	}

	schemaHandler := r.pr.SchemaHandler
	result := make(map[string]string)

	for i, element := range schemaHandler.SchemaElements {
		if i == 0 {
			continue
		}
		if element.GetNumChildren() == 0 && element.Name != "" {
			pathStr := schemaHandler.IndexMap[int32(i)]
			result[element.Name] = pathStr
		}
	}

	return result
}

// Close 关闭 Parquet 文件读取器
// 返回: 可能的关闭错误
func (r *Reader) Close() error {
	if r.pr != nil {
		r.pr.ReadStop()
		r.pr = nil
	}
	if r.fr != nil {
		r.fr.Close()
		r.fr = nil
	}
	return nil
}

// GetNumRows 返回总行数
func (r *Reader) GetNumRows() int64 {
	if r.pr == nil {
		return 0
	}
	return r.pr.GetNumRows()
}

// GetPath 返回文件路径
func (r *Reader) GetPath() string {
	return r.path
}

// ColumnExists 检查列是否存在
func (r *Reader) ColumnExists(columnName string) bool {
	columns := r.GetColumns()
	for _, col := range columns {
		if col == columnName {
			return true
		}
	}
	return false
}

// GetColumnType 获取列的数据类型
func (r *Reader) GetColumnType(columnName string) (string, error) {
	if r.pr == nil || r.pr.SchemaHandler == nil {
		return "", fmt.Errorf("parquet reader is not initialized")
	}

	for i, element := range r.pr.SchemaHandler.SchemaElements {
		if i == 0 {
			continue
		}
		if element.GetNumChildren() == 0 && element.Name == columnName {
			return element.GetType().String(), nil
		}
	}

	return "", fmt.Errorf("column %s not found", columnName)
}
