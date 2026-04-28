package db

import (
	"fmt"
	"strings"
)

// bulkInsertSQLite uses multi-row INSERT with batch size 500.
// Combined with PRAGMA WAL + NORMAL sync, this gives 500-1000x speedup.
func (m *Manager) bulkInsertSQLite(tableName string, columns []string, rows []map[string]any) error {
	const batchSize = 500

	colList := make([]string, len(columns))
	for i, col := range columns {
		colList[i] = fmt.Sprintf("`%s`", col)
	}
	colListStr := strings.Join(colList, ", ")

	// Single-row placeholder template: (?, ?, ?)
	singleRowPH := "(" + strings.Repeat("?,", len(columns))
	singleRowPH = singleRowPH[:len(singleRowPH)-1] + ")"

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("[sqlite] failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for offset := 0; offset < len(rows); offset += batchSize {
		end := offset + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[offset:end]

		placeholders := make([]string, len(batch))
		for i := range placeholders {
			placeholders[i] = singleRowPH
		}

		query := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s",
			tableName, colListStr, strings.Join(placeholders, ","))

		values := make([]any, 0, len(batch)*len(columns))
		for _, row := range batch {
			for _, col := range columns {
				values = append(values, normalizeValue(row[col]))
			}
		}

		_, err := tx.Exec(query, values...)
		if err != nil {
			return fmt.Errorf("[sqlite] batch insert failed at row %d: %w", offset, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[sqlite] failed to commit: %w", err)
	}

	fmt.Printf("[导入引擎] Multi-row INSERT 完成: %d 行\n", len(rows))
	return nil
}
