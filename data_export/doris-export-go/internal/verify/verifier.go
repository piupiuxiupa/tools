package verify

import (
	"fmt"
	"os"

	"github.com/parquet-go/parquet-go"
)

// Result holds verification results
type Result struct {
	FilePath    string
	Valid       bool
	RowCount    int64
	ColumnCount int
	SizeMB      float64
	Error       string
}

// Verifier validates Parquet files
type Verifier interface {
	Verify(filePath string) (*Result, error)
	VerifyBatch(filePaths []string) ([]*Result, bool)
}

// verifier implements the Verifier interface
type verifier struct{}

// NewVerifier creates a new Verifier instance
func NewVerifier() Verifier {
	return &verifier{}
}

// Verify validates a single Parquet file
// It checks file existence, reads metadata, and returns verification results
func (v *verifier) Verify(filePath string) (*Result, error) {
	result := &Result{
		FilePath: filePath,
		Valid:    false,
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Error = fmt.Sprintf("file not found: %s", filePath)
			return result, nil
		}
		result.Error = fmt.Sprintf("failed to stat file: %v", err)
		return result, nil
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		result.Error = fmt.Sprintf("not a regular file: %s", filePath)
		return result, nil
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to open file: %v", err)
		return result, nil
	}
	defer file.Close()

	// Get file size in MB
	result.SizeMB = float64(fileInfo.Size()) / (1024 * 1024)

	// Open as parquet file to read metadata
	pf, err := parquet.OpenFile(file, fileInfo.Size())
	if err != nil {
		result.Error = fmt.Sprintf("invalid parquet format: %v", err)
		return result, nil
	}

	// Extract row count
	result.RowCount = pf.NumRows()

	// Extract column count from schema
	columns := pf.Schema().Columns()
	result.ColumnCount = len(columns)

	// Mark as valid
	result.Valid = true

	return result, nil
}

// VerifyBatch validates multiple Parquet files
// Returns slice of Results and boolean indicating if all files are valid
func (v *verifier) VerifyBatch(filePaths []string) ([]*Result, bool) {
	results := make([]*Result, 0, len(filePaths))
	allValid := true

	for _, filePath := range filePaths {
		result, _ := v.Verify(filePath)
		results = append(results, result)

		if !result.Valid {
			allValid = false
		}
	}

	return results, allValid
}
