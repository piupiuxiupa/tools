package parquet

import (
	"fmt"
	"io"
	"os"

	"github.com/parquet-go/parquet-go"
)

// Reader 提供 Parquet 文件的读取功能
type Reader struct {
	path string
	file *os.File
	pf   *parquet.File // 缓存的 Parquet 文件元数据
	size int64
}

// NewReader 创建一个新的 Parquet 文件读取器
// 参数 path: Parquet 文件的路径
// 返回: 指向 Reader 的指针和可能的错误
func NewReader(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file: %w", err)
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to stat parquet file: %w", err)
	}

	return &Reader{
		path: path,
		file: f,
		size: fi.Size(),
	}, nil
}

// openFile 懒加载并缓存 Parquet 文件元数据
func (r *Reader) openFile() (*parquet.File, error) {
	if r.file == nil {
		return nil, fmt.Errorf("parquet reader is not initialized")
	}
	if r.pf != nil {
		return r.pf, nil
	}
	pf, err := parquet.OpenFile(r.file, r.size)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parquet file: %w", err)
	}
	r.pf = pf
	return pf, nil
}

// GetColumns 获取 Parquet 文件中的所有列名
// 返回: 列名切片（简化名称，不含路径前缀）
func (r *Reader) GetColumns() []string {
	pf, err := r.openFile()
	if err != nil {
		return nil
	}

	// Schema.Columns() 返回所有叶列的路径，如 [["col1"], ["col2"]]
	columnPaths := pf.Schema().Columns()
	columns := make([]string, 0, len(columnPaths))
	for _, path := range columnPaths {
		if len(path) > 0 {
			columns = append(columns, path[len(path)-1])
		}
	}
	return columns
}

// ReadRows 读取 Parquet 文件中的所有行数据
// 返回: 每行数据以 map[string]interface{} 形式存储在切片中
func (r *Reader) ReadRows() ([]map[string]interface{}, error) {
	if r.file == nil {
		return nil, fmt.Errorf("parquet reader is not initialized")
	}

	// 重置文件读取位置
	if _, err := r.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek parquet file: %w", err)
	}

	// 使用 GenericReader 流式读取，避免大文件一次性加载问题
	pf, err := r.openFile()
	if err != nil {
		return nil, err
	}

	reader := parquet.NewGenericReader[any](pf)
	defer reader.Close()

	totalRows := int(reader.NumRows())
	if totalRows == 0 {
		return []map[string]interface{}{}, nil
	}

	rows := make([]map[string]interface{}, 0, totalRows)
	buf := make([]any, min(1024, totalRows))

	for {
		n, err := reader.Read(buf)
		for i := 0; i < n; i++ {
			if m, ok := buf[i].(map[string]interface{}); ok {
				rows = append(rows, m)
			} else {
				rows = append(rows, map[string]interface{}{"value": buf[i]})
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read parquet rows: %w", err)
		}
	}

	return rows, nil
}

// ReadRowsByColumns 按指定列读取行数据
// 参数 columnNames: 要读取的列名列表
// 返回: 每行数据只包含指定的列
func (r *Reader) ReadRowsByColumns(columnNames []string) ([]map[string]interface{}, error) {
	if r.file == nil {
		return nil, fmt.Errorf("parquet reader is not initialized")
	}

	if len(columnNames) == 0 {
		return r.ReadRows()
	}

	// 读取所有行后过滤列
	allRows, err := r.ReadRows()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(allRows))
	for i, row := range allRows {
		filtered := make(map[string]interface{}, len(columnNames))
		for _, col := range columnNames {
			if v, ok := row[col]; ok {
				filtered[col] = v
			} else {
				filtered[col] = nil
			}
		}
		result[i] = filtered
	}
	return result, nil
}

// Close 关闭 Parquet 文件读取器
// 返回: 可能的关闭错误
func (r *Reader) Close() error {
	r.pf = nil
	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}

// GetNumRows 返回总行数
func (r *Reader) GetNumRows() int64 {
	pf, err := r.openFile()
	if err != nil {
		return 0
	}
	return pf.NumRows()
}

// GetPath 返回文件路径
func (r *Reader) GetPath() string {
	return r.path
}

// ColumnExists 检查列是否存在
func (r *Reader) ColumnExists(columnName string) bool {
	pf, err := r.openFile()
	if err != nil {
		return false
	}
	return pf.Root().Column(columnName) != nil
}

// GetColumnType 获取列的数据类型
func (r *Reader) GetColumnType(columnName string) (string, error) {
	pf, err := r.openFile()
	if err != nil {
		return "", fmt.Errorf("parquet reader is not initialized")
	}

	col := pf.Root().Column(columnName)
	if col == nil {
		return "", fmt.Errorf("column %s not found", columnName)
	}
	return col.Type().String(), nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
