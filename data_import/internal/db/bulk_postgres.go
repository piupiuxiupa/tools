package db

import (
	"fmt"

	"github.com/lib/pq"
)

// bulkInsertPostgreSQL uses pq.CopyIn for COPY protocol bulk import.
func (m *Manager) bulkInsertPostgreSQL(tableName string, columns []string, rows []map[string]any) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("[postgresql] failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = fmt.Sprintf(`"%s"`, col)
	}

	stmt, err := tx.Prepare(pq.CopyIn(tableName, colNames...))
	if err != nil {
		return fmt.Errorf("[postgresql] failed to prepare COPY statement: %w", err)
	}
	defer stmt.Close()

	for i, row := range rows {
		values := make([]any, len(columns))
		for j, col := range columns {
			values[j] = normalizeValue(row[col])
		}
		if _, err := stmt.Exec(values...); err != nil {
			return fmt.Errorf("[postgresql] COPY row %d failed: %w", i, err)
		}
	}

	if _, err := stmt.Exec(); err != nil {
		return fmt.Errorf("[postgresql] COPY flush failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[postgresql] failed to commit COPY transaction: %w", err)
	}

	fmt.Printf("[导入引擎] COPY 导入完成: %d 行\n", len(rows))
	return nil
}
