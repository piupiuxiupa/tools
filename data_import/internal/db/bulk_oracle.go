package db

import (
	"fmt"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"
)

// bulkInsertOracle uses go-ora NewBatch array binding for bulk import.
func (m *Manager) bulkInsertOracle(tableName string, columns []string, rows []map[string]any) error {
	const batchSize = 10000

	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = fmt.Sprintf(`"%s"`, strings.ToUpper(col))
	}
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf(":%d", i+1)
	}
	sqlText := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
		strings.ToUpper(tableName),
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "))

	totalRows := len(rows)
	for start := 0; start < totalRows; start += batchSize {
		end := start + batchSize
		if end > totalRows {
			end = totalRows
		}
		batch := rows[start:end]
		nRows := end - start

		args := make([]any, len(columns))
		for j, col := range columns {
			arr := make([]any, nRows)
			for k, row := range batch {
				arr[k] = normalizeValue(row[col])
			}
			args[j] = go_ora.NewBatch(arr)
		}

		result, err := m.db.Exec(sqlText, args...)
		if err != nil {
			return fmt.Errorf("[oracle] bulk insert failed at row %d: %w", start, err)
		}

		affected, _ := result.RowsAffected()
		fmt.Printf("[导入引擎] Array Binding 批次 %d-%d: %d 行\n", start, end-1, affected)
	}

	return nil
}
