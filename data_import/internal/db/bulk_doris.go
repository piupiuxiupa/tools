package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lush/data_import/internal/config"
)

// streamLoadResponse represents Doris Stream Load API response
type streamLoadResponse struct {
	TxnID              int64  `json:"TxnId"`
	Label              string `json:"Label"`
	Status             string `json:"status"`
	Message            string `json:"msg"`
	NumberTotalRows    int64  `json:"NumberTotalRows"`
	NumberLoadedRows   int64  `json:"NumberLoadedRows"`
	NumberFilteredRows int64  `json:"NumberFilteredRows"`
	LoadBytes          int64  `json:"LoadBytes"`
	LoadTimeMs         int    `json:"LoadTimeMs"`
	ErrorURL           string `json:"ErrorURL"`
}

// bulkInsertDoris sends the Parquet file to Doris via Stream Load HTTP API.
// Doris natively reads Parquet schema and maps columns by name.
func (m *Manager) bulkInsertDoris(tableName string, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("[doris] config is required for Stream Load")
	}

	feHost := cfg.Host
	fePort := cfg.FEHTTPPort
	if fePort == 0 {
		fePort = 8030
	}

	url := fmt.Sprintf("http://%s:%d/api/%s/%s/_stream_load",
		feHost, fePort, cfg.DBName, tableName)

	data, err := os.ReadFile(cfg.FilePath)
	if err != nil {
		return fmt.Errorf("[doris] failed to read parquet file for stream load: %w", err)
	}

	fmt.Printf("[导入引擎] Stream Load URL: %s\n", url)
	fmt.Printf("[导入引擎] 文件: %s, 大小: %d 字节\n", cfg.FilePath, len(data))

	client := &http.Client{
		Timeout: 3600 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, body, err := m.doStreamLoad(client, data, url, cfg)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("[导入引擎] Stream Load 响应: %s\n", string(body))

	var result streamLoadResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("[doris] failed to parse stream load response: %w", err)
	}

	if result.Status != "Success" {
		return fmt.Errorf("[doris] stream load status=%s, message=%s, label=%s, txn=%d, filtered %d/%d rows, errorURL=%s",
			result.Status, result.Message, result.Label, result.TxnID,
			result.NumberFilteredRows, result.NumberTotalRows, result.ErrorURL)
	}

	fmt.Printf("[导入引擎] Stream Load 完成: %d 行, %d 字节, 耗时 %dms\n",
		result.NumberLoadedRows, result.LoadBytes, result.LoadTimeMs)

	return nil
}

func (m *Manager) doStreamLoad(client *http.Client, data []byte, url string, cfg *config.Config) (*http.Response, []byte, error) {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("[doris] failed to create stream load request: %w", err)
	}

	req.SetBasicAuth(cfg.User, cfg.Password)
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("format", "parquet")
	req.Header.Set("label", fmt.Sprintf("data_import_%d", time.Now().UnixMilli()))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))
	req.Header.Set("timeout", "3600")
	req.Header.Set("strict_mode", "true")
	req.Header.Set("max_filter_ratio", "0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("[doris] stream load request failed: %w", err)
	}

	if resp.StatusCode == http.StatusTemporaryRedirect {
		redirectURL := resp.Header.Get("Location")
		resp.Body.Close()

		if redirectURL == "" {
			return nil, nil, fmt.Errorf("[doris] FE returned 307 with empty Location header")
		}

		fmt.Printf("[导入引擎] FE 重定向到 BE: %s\n", redirectURL)
		return m.doStreamLoad(client, data, redirectURL, cfg)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("[doris] failed to read stream load response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("[doris] stream load HTTP %d: %s", resp.StatusCode, string(body))
	}

	return resp, body, nil
}
