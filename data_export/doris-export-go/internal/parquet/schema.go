package parquet

import (
	"fmt"
	"reflect"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress"
)

// Column represents a column definition in the schema
type Column struct {
	Name     string
	Type     string
	Required bool
}

// Schema represents a Parquet schema
type Schema struct {
	Columns []Column
	schema  *parquet.Schema
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		Columns: make([]Column, 0),
	}
}

// AddColumn adds a column to the schema
func (s *Schema) AddColumn(name, colType string) {
	s.Columns = append(s.Columns, Column{
		Name:     name,
		Type:     colType,
		Required: true,
	})
}

// ToParquetSchema converts our schema representation to parquet-go schema
func (s *Schema) ToParquetSchema() *parquet.Schema {
	if s.schema != nil {
		return s.schema
	}

	opts := SchemaOptions{EnableDictionary: true}
	group := make(parquet.Group)
	for _, col := range s.Columns {
		group[col.Name] = createParquetNode(col.Type, opts)
	}

	s.schema = parquet.NewSchema("record", group)
	return s.schema
}

type SchemaOptions struct {
	EnableDictionary bool
}

// createParquetNode creates a parquet node based on type name
func createParquetNode(typeName string, opts SchemaOptions) parquet.Node {
	switch typeName {
	case "INT32":
		return parquet.Int(32)
	case "INT64":
		return parquet.Int(64)
	case "DOUBLE":
		return parquet.Leaf(parquet.DoubleType)
	case "BYTE_ARRAY", "STRING":
		return parquet.String()
	case "BOOLEAN":
		return parquet.Leaf(parquet.BooleanType)
	case "TIMESTAMP":
		return parquet.Timestamp(parquet.Millisecond)
	default:
		return parquet.String()
	}
}

// InferSchema infers a Parquet schema from column names and a sample row
func InferSchema(columns []string, sampleRow map[string]interface{}) *Schema {
	schema := NewSchema()

	for _, col := range columns {
		if val, ok := sampleRow[col]; ok {
			parquetType := inferType(val)
			schema.AddColumn(col, parquetType)
		} else {
			// Default to string if not in sample
			schema.AddColumn(col, "STRING")
		}
	}

	return schema
}

// inferType infers the Parquet type from a Go value
// Uses INT32 for smaller integer types to save space
func inferType(value interface{}) string {
	if value == nil {
		return "STRING"
	}

	switch value.(type) {
	case int8, int16, uint8, uint16:
		return "INT32"
	case int, int32, int64, uint, uint32, uint64:
		return "INT64"
	case float32, float64:
		return "DOUBLE"
	case string:
		return "STRING"
	case bool:
		return "BOOLEAN"
	case time.Time:
		return "TIMESTAMP"
	default:
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Ptr && !val.IsNil() {
			return inferType(val.Elem().Interface())
		}
		return "STRING"
	}
}

// GetCompressionCodec returns the parquet compression codec based on name
func GetCompressionCodec(name string) compress.Codec {
	switch name {
	case "snappy":
		return &parquet.Snappy
	case "gzip":
		return &parquet.Gzip
	case "zstd":
		return &parquet.Zstd
	case "none", "":
		return &parquet.Uncompressed
	default:
		return &parquet.Snappy
	}
}

// convertToParquetValue converts a Go value to a parquet-compatible value
func convertToParquetValue(value interface{}, targetType string) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch targetType {
	case "INT32":
		switch v := value.(type) {
		case int8:
			return int32(v), nil
		case int16:
			return int32(v), nil
		case int32:
			return v, nil
		case uint8:
			return int32(v), nil
		case uint16:
			return int32(v), nil
		case string:
			var result int32
			_, err := fmt.Sscanf(v, "%d", &result)
			return result, err
		default:
			return int32(0), fmt.Errorf("cannot convert %T to INT32", value)
		}
	case "INT64":
		switch v := value.(type) {
		case int:
			return int64(v), nil
		case int8:
			return int64(v), nil
		case int16:
			return int64(v), nil
		case int32:
			return int64(v), nil
		case int64:
			return v, nil
		case uint:
			return int64(v), nil
		case uint8:
			return int64(v), nil
		case uint16:
			return int64(v), nil
		case uint32:
			return int64(v), nil
		case uint64:
			return int64(v), nil
		case float32:
			return int64(v), nil
		case float64:
			return int64(v), nil
		case string:
			var result int64
			_, err := fmt.Sscanf(v, "%d", &result)
			return result, err
		default:
			return 0, fmt.Errorf("cannot convert %T to INT64", value)
		}
	case "DOUBLE":
		switch v := value.(type) {
		case float32:
			return float64(v), nil
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case string:
			var result float64
			_, err := fmt.Sscanf(v, "%f", &result)
			return result, err
		default:
			return 0.0, fmt.Errorf("cannot convert %T to DOUBLE", value)
		}
	case "STRING":
		return fmt.Sprintf("%v", value), nil
	case "BOOLEAN":
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return v == "true" || v == "1" || v == "TRUE", nil
		case int:
			return v != 0, nil
		case int64:
			return v != 0, nil
		default:
			return false, fmt.Errorf("cannot convert %T to BOOLEAN", value)
		}
	case "TIMESTAMP":
		switch v := value.(type) {
		case time.Time:
			return v.UnixMilli(), nil
		case int64:
			return v, nil
		case string:
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return 0, fmt.Errorf("cannot parse timestamp: %w", err)
			}
			return t.UnixMilli(), nil
		default:
			return 0, fmt.Errorf("cannot convert %T to TIMESTAMP", value)
		}
	default:
		return fmt.Sprintf("%v", value), nil
	}
}
