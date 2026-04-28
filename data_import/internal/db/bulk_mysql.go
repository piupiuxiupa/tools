package db

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// bulkInsertMySQL uses LOAD DATA LOCAL INFILE with an in-memory CSV reader.
// This is 20-50x faster than row-by-row INSERT.
func (m *Manager) bulkInsertMySQL(tableName string, columns []string, rows []map[string]any) error {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			val := normalizeValue(row[col])
			if val == nil {
				record[i] = "\\N"
			} else {
				record[i] = fmt.Sprintf("%v", val)
			}
		}
		if err := w.Write(record); err != nil {
			return fmt.Errorf("[mysql] failed to write CSV row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("[mysql] CSV flush error: %w", err)
	}

	handlerName := fmt.Sprintf("parquet_data_%d", len(rows))
	mysql.RegisterReaderHandler(handlerName, func() io.Reader {
		return bytes.NewReader(buf.Bytes())
	})
	defer mysql.DeregisterReaderHandler(handlerName)

	colList := make([]string, len(columns))
	for i, col := range columns {
		colList[i] = fmt.Sprintf("`%s`", col)
	}

	query := fmt.Sprintf(
		"LOAD DATA LOCAL INFILE 'Reader::%s' INTO TABLE `%s` "+
			"FIELDS TERMINATED BY ',' ENCLOSED BY '\"' ESCAPED BY '\\\\' "+
			"LINES TERMINATED BY '\\n' (%s)",
		handlerName, tableName, strings.Join(colList, ", "))

	result, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("[mysql] LOAD DATA LOCAL INFILE failed: %w", err)
	}

	affected, _ := result.RowsAffected()
	fmt.Printf("[导入引擎] LOAD DATA 完成: %d 行\n", affected)
	return nil
}
