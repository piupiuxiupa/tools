// Package types provides core data structures for database schema representation.
package types

import (
	"fmt"
	"strings"
	"sync"
)

// TypeMapper defines the interface for mapping data types between different database systems.
type TypeMapper interface {
	// MapType converts a source database type to a target database type.
	// Parameters:
	//   - sourceType: the data type from the source database
	//   - sourceDB: the source database system (e.g., "mysql", "postgresql", "oracle", "doris")
	//   - targetDB: the target database system
	// Returns the mapped type string and an error if mapping fails.
	MapType(sourceType, sourceDB, targetDB string) (string, error)

	// RegisterMapping adds a custom type mapping for a specific database pair.
	RegisterMapping(sourceDB, targetDB, sourceType, targetType string)

	// GetSupportedDatabases returns a list of supported database systems.
	GetSupportedDatabases() []string

	// IsSupported checks if a database system is supported.
	IsSupported(dbName string) bool
}

// DefaultTypeMapper provides comprehensive type mappings for popular database systems.
type DefaultTypeMapper struct {
	mappings map[string]map[string]map[string]string // sourceDB -> targetDB -> sourceType -> targetType
	mu       sync.RWMutex
}

// NewDefaultTypeMapper creates a new DefaultTypeMapper with predefined mappings.
func NewDefaultTypeMapper() *DefaultTypeMapper {
	mapper := &DefaultTypeMapper{
		mappings: make(map[string]map[string]map[string]string),
	}
	mapper.initializeMappings()
	return mapper
}

// initializeMappings sets up the default type mappings.
func (m *DefaultTypeMapper) initializeMappings() {
	// Initialize nested maps
	supportedDBs := []string{"mysql", "postgresql", "oracle", "doris", "sqlite", "mssql"}

	for _, sourceDB := range supportedDBs {
		m.mappings[sourceDB] = make(map[string]map[string]string)
		for _, targetDB := range supportedDBs {
			if sourceDB != targetDB {
				m.mappings[sourceDB][targetDB] = make(map[string]string)
			}
		}
	}

	// MySQL to PostgreSQL mappings
	m.addMapping("mysql", "postgresql", map[string]string{
		"tinyint":    "smallint",
		"tinyint(1)": "boolean",
		"smallint":   "smallint",
		"mediumint":  "integer",
		"int":        "integer",
		"integer":    "integer",
		"bigint":     "bigint",
		"float":      "real",
		"double":     "double precision",
		"decimal":    "decimal",
		"numeric":    "numeric",
		"char":       "char",
		"varchar":    "varchar",
		"tinytext":   "text",
		"text":       "text",
		"mediumtext": "text",
		"longtext":   "text",
		"binary":     "bytea",
		"varbinary":  "bytea",
		"tinyblob":   "bytea",
		"blob":       "bytea",
		"mediumblob": "bytea",
		"longblob":   "bytea",
		"date":       "date",
		"time":       "time",
		"datetime":   "timestamp",
		"timestamp":  "timestamp",
		"year":       "smallint",
		"json":       "jsonb",
		"enum":       "varchar",
		"set":        "text",
	})

	// MySQL to Oracle mappings
	m.addMapping("mysql", "oracle", map[string]string{
		"tinyint":    "NUMBER(3)",
		"smallint":   "NUMBER(5)",
		"mediumint":  "NUMBER(7)",
		"int":        "NUMBER(10)",
		"integer":    "NUMBER(10)",
		"bigint":     "NUMBER(19)",
		"float":      "BINARY_FLOAT",
		"double":     "BINARY_DOUBLE",
		"decimal":    "NUMBER",
		"numeric":    "NUMBER",
		"char":       "CHAR",
		"varchar":    "VARCHAR2",
		"tinytext":   "CLOB",
		"text":       "CLOB",
		"mediumtext": "CLOB",
		"longtext":   "CLOB",
		"binary":     "RAW",
		"varbinary":  "RAW",
		"tinyblob":   "BLOB",
		"blob":       "BLOB",
		"mediumblob": "BLOB",
		"longblob":   "BLOB",
		"date":       "DATE",
		"time":       "TIMESTAMP",
		"datetime":   "TIMESTAMP",
		"timestamp":  "TIMESTAMP",
		"year":       "NUMBER(4)",
		"json":       "CLOB",
		"enum":       "VARCHAR2",
		"set":        "CLOB",
	})

	// MySQL to Doris mappings
	m.addMapping("mysql", "doris", map[string]string{
		"tinyint":    "TINYINT",
		"smallint":   "SMALLINT",
		"mediumint":  "INT",
		"int":        "INT",
		"integer":    "INT",
		"bigint":     "BIGINT",
		"float":      "FLOAT",
		"double":     "DOUBLE",
		"decimal":    "DECIMAL",
		"numeric":    "DECIMAL",
		"char":       "CHAR",
		"varchar":    "VARCHAR",
		"tinytext":   "STRING",
		"text":       "STRING",
		"mediumtext": "STRING",
		"longtext":   "STRING",
		"binary":     "STRING",
		"varbinary":  "STRING",
		"tinyblob":   "STRING",
		"blob":       "STRING",
		"mediumblob": "STRING",
		"longblob":   "STRING",
		"date":       "DATE",
		"time":       "STRING",
		"datetime":   "DATETIME",
		"timestamp":  "DATETIME",
		"year":       "SMALLINT",
		"json":       "JSON",
		"enum":       "VARCHAR",
		"set":        "STRING",
	})

	// PostgreSQL to MySQL mappings
	m.addMapping("postgresql", "mysql", map[string]string{
		"smallint":                    "smallint",
		"integer":                     "int",
		"bigint":                      "bigint",
		"serial":                      "int",
		"bigserial":                   "bigint",
		"real":                        "float",
		"double precision":            "double",
		"numeric":                     "decimal",
		"decimal":                     "decimal",
		"money":                       "decimal(19,2)",
		"character":                   "char",
		"character varying":           "varchar",
		"text":                        "text",
		"bytea":                       "blob",
		"timestamp":                   "timestamp",
		"timestamp without time zone": "timestamp",
		"timestamp with time zone":    "timestamp",
		"date":                        "date",
		"time":                        "time",
		"time without time zone":      "time",
		"time with time zone":         "time",
		"interval":                    "varchar(50)",
		"boolean":                     "tinyint(1)",
		"point":                       "varchar(50)",
		"line":                        "varchar(50)",
		"lseg":                        "varchar(50)",
		"box":                         "varchar(50)",
		"path":                        "text",
		"polygon":                     "text",
		"circle":                      "varchar(50)",
		"cidr":                        "varchar(50)",
		"inet":                        "varchar(50)",
		"macaddr":                     "varchar(50)",
		"macaddr8":                    "varchar(50)",
		"bit":                         "bit",
		"bit varying":                 "varbinary",
		"tsvector":                    "text",
		"tsquery":                     "text",
		"uuid":                        "char(36)",
		"xml":                         "text",
		"json":                        "json",
		"jsonb":                       "json",
		"pg_lsn":                      "varchar(50)",
		"pg_snapshot":                 "varchar(50)",
		"txid_snapshot":               "varchar(50)",
		"array":                       "json",
		"composite":                   "json",
		"enum":                        "varchar",
		"range":                       "varchar(100)",
		"domain":                      "varchar(100)",
	})

	// PostgreSQL to Oracle mappings
	m.addMapping("postgresql", "oracle", map[string]string{
		"smallint":          "NUMBER(5)",
		"integer":           "NUMBER(10)",
		"bigint":            "NUMBER(19)",
		"serial":            "NUMBER(10)",
		"bigserial":         "NUMBER(19)",
		"real":              "BINARY_FLOAT",
		"double precision":  "BINARY_DOUBLE",
		"numeric":           "NUMBER",
		"decimal":           "NUMBER",
		"money":             "NUMBER(19,2)",
		"character":         "CHAR",
		"character varying": "VARCHAR2",
		"text":              "CLOB",
		"bytea":             "BLOB",
		"timestamp":         "TIMESTAMP",
		"date":              "DATE",
		"time":              "TIMESTAMP",
		"interval":          "VARCHAR(50)",
		"boolean":           "NUMBER(1)",
		"uuid":              "RAW(16)",
		"json":              "CLOB",
		"jsonb":             "CLOB",
		"xml":               "XMLTYPE",
	})

	// Oracle to MySQL mappings
	m.addMapping("oracle", "mysql", map[string]string{
		"NUMBER":                         "decimal",
		"NUMBER(1)":                      "tinyint(1)",
		"NUMBER(3)":                      "tinyint",
		"NUMBER(5)":                      "smallint",
		"NUMBER(7)":                      "mediumint",
		"NUMBER(10)":                     "int",
		"NUMBER(19)":                     "bigint",
		"FLOAT":                          "float",
		"BINARY_FLOAT":                   "float",
		"BINARY_DOUBLE":                  "double",
		"CHAR":                           "char",
		"VARCHAR":                        "varchar",
		"VARCHAR2":                       "varchar",
		"NCHAR":                          "char",
		"NVARCHAR2":                      "varchar",
		"CLOB":                           "longtext",
		"NCLOB":                          "longtext",
		"BLOB":                           "longblob",
		"BFILE":                          "longblob",
		"LONG":                           "longtext",
		"LONG RAW":                       "longblob",
		"RAW":                            "varbinary",
		"DATE":                           "datetime",
		"TIMESTAMP":                      "timestamp",
		"TIMESTAMP WITH TIME ZONE":       "timestamp",
		"TIMESTAMP WITH LOCAL TIME ZONE": "timestamp",
		"INTERVAL YEAR TO MONTH":         "varchar(30)",
		"INTERVAL DAY TO SECOND":         "varchar(30)",
		"ROWID":                          "varchar(50)",
		"UROWID":                         "varchar(50)",
		"XMLTYPE":                        "longtext",
	})

	// Oracle to PostgreSQL mappings
	m.addMapping("oracle", "postgresql", map[string]string{
		"NUMBER":                         "numeric",
		"NUMBER(1)":                      "boolean",
		"FLOAT":                          "double precision",
		"BINARY_FLOAT":                   "real",
		"BINARY_DOUBLE":                  "double precision",
		"CHAR":                           "char",
		"VARCHAR":                        "varchar",
		"VARCHAR2":                       "varchar",
		"NCHAR":                          "char",
		"NVARCHAR2":                      "varchar",
		"CLOB":                           "text",
		"NCLOB":                          "text",
		"BLOB":                           "bytea",
		"BFILE":                          "bytea",
		"LONG":                           "text",
		"LONG RAW":                       "bytea",
		"RAW":                            "bytea",
		"DATE":                           "timestamp",
		"TIMESTAMP":                      "timestamp",
		"TIMESTAMP WITH TIME ZONE":       "timestamp with time zone",
		"TIMESTAMP WITH LOCAL TIME ZONE": "timestamp",
		"XMLTYPE":                        "xml",
	})

	// Oracle to Doris mappings
	m.addMapping("oracle", "doris", map[string]string{
		"NUMBER":        "DECIMAL",
		"FLOAT":         "DOUBLE",
		"BINARY_FLOAT":  "FLOAT",
		"BINARY_DOUBLE": "DOUBLE",
		"CHAR":          "CHAR",
		"VARCHAR":       "VARCHAR",
		"VARCHAR2":      "VARCHAR",
		"NCHAR":         "CHAR",
		"NVARCHAR2":     "VARCHAR",
		"CLOB":          "STRING",
		"NCLOB":         "STRING",
		"BLOB":          "STRING",
		"BFILE":         "STRING",
		"LONG":          "STRING",
		"LONG RAW":      "STRING",
		"RAW":           "STRING",
		"DATE":          "DATETIME",
		"TIMESTAMP":     "DATETIME",
		"XMLTYPE":       "STRING",
	})

	// Doris to MySQL mappings
	m.addMapping("doris", "mysql", map[string]string{
		"BOOLEAN":        "tinyint(1)",
		"TINYINT":        "tinyint",
		"SMALLINT":       "smallint",
		"INT":            "int",
		"BIGINT":         "bigint",
		"LARGEINT":       "bigint",
		"FLOAT":          "float",
		"DOUBLE":         "double",
		"DECIMAL":        "decimal",
		"DECIMALV3":      "decimal",
		"DATE":           "date",
		"DATETIME":       "datetime",
		"DATETIMEV2":     "datetime",
		"CHAR":           "char",
		"VARCHAR":        "varchar",
		"STRING":         "longtext",
		"HLL":            "blob",
		"BITMAP":         "blob",
		"QUANTILE_STATE": "blob",
		"AGG_STATE":      "blob",
		"JSON":           "json",
		"JSONB":          "json",
		"ARRAY":          "json",
		"MAP":            "json",
		"STRUCT":         "json",
	})

	// SQLite to MySQL mappings
	m.addMapping("sqlite", "mysql", map[string]string{
		"INTEGER": "int",
		"REAL":    "double",
		"TEXT":    "text",
		"BLOB":    "blob",
		"NUMERIC": "decimal",
	})

	// MSSQL to MySQL mappings
	m.addMapping("mssql", "mysql", map[string]string{
		"bit":              "tinyint(1)",
		"tinyint":          "tinyint",
		"smallint":         "smallint",
		"int":              "int",
		"bigint":           "bigint",
		"decimal":          "decimal",
		"numeric":          "decimal",
		"float":            "double",
		"real":             "float",
		"smallmoney":       "decimal(10,4)",
		"money":            "decimal(19,4)",
		"char":             "char",
		"varchar":          "varchar",
		"text":             "text",
		"nchar":            "char",
		"nvarchar":         "varchar",
		"ntext":            "text",
		"binary":           "binary",
		"varbinary":        "varbinary",
		"image":            "longblob",
		"date":             "date",
		"time":             "time",
		"datetime":         "datetime",
		"datetime2":        "datetime",
		"smalldatetime":    "datetime",
		"datetimeoffset":   "timestamp",
		"timestamp":        "timestamp",
		"uniqueidentifier": "char(36)",
		"xml":              "longtext",
		"sql_variant":      "longtext",
	})
}

// addMapping adds a batch of type mappings.
func (m *DefaultTypeMapper) addMapping(sourceDB, targetDB string, mappings map[string]string) {
	for sourceType, targetType := range mappings {
		m.mappings[sourceDB][targetDB][strings.ToLower(sourceType)] = targetType
	}
}

// MapType implements the TypeMapper interface.
func (m *DefaultTypeMapper) MapType(sourceType, sourceDB, targetDB string) (string, error) {
	if sourceDB == targetDB {
		return sourceType, nil
	}

	sourceDB = strings.ToLower(sourceDB)
	targetDB = strings.ToLower(targetDB)
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))

	m.mu.RLock()
	defer m.mu.RUnlock()

	targetMappings, ok := m.mappings[sourceDB]
	if !ok {
		return "", fmt.Errorf("unsupported source database: %s", sourceDB)
	}

	typeMappings, ok := targetMappings[targetDB]
	if !ok {
		return "", fmt.Errorf("unsupported target database: %s", targetDB)
	}

	// Try exact match first
	if targetType, ok := typeMappings[sourceType]; ok {
		return targetType, nil
	}

	// Try matching without size/precision info (e.g., "varchar(255)" -> "varchar")
	baseType := extractBaseType(sourceType)
	if targetType, ok := typeMappings[baseType]; ok {
		return targetType, nil
	}

	// Try with common synonyms
	normalizedType := normalizeTypeName(sourceType)
	if targetType, ok := typeMappings[normalizedType]; ok {
		return targetType, nil
	}

	return "", fmt.Errorf("no mapping found for type '%s' from %s to %s", sourceType, sourceDB, targetDB)
}

// extractBaseType extracts the base type name without size/precision.
func extractBaseType(typeName string) string {
	// Remove size specification (e.g., varchar(255) -> varchar)
	if idx := strings.Index(typeName, "("); idx != -1 {
		return strings.TrimSpace(typeName[:idx])
	}
	return typeName
}

// normalizeTypeName normalizes type names for better matching.
func normalizeTypeName(typeName string) string {
	// Handle common variations
	replacements := map[string]string{
		"int4":      "integer",
		"int8":      "bigint",
		"float4":    "float",
		"float8":    "double",
		"bool":      "boolean",
		"character": "char",
		"string":    "varchar",
	}

	if normalized, ok := replacements[typeName]; ok {
		return normalized
	}
	return typeName
}

// RegisterMapping implements the TypeMapper interface.
func (m *DefaultTypeMapper) RegisterMapping(sourceDB, targetDB, sourceType, targetType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sourceDB = strings.ToLower(sourceDB)
	targetDB = strings.ToLower(targetDB)
	sourceType = strings.ToLower(sourceType)

	if _, ok := m.mappings[sourceDB]; !ok {
		m.mappings[sourceDB] = make(map[string]map[string]string)
	}
	if _, ok := m.mappings[sourceDB][targetDB]; !ok {
		m.mappings[sourceDB][targetDB] = make(map[string]string)
	}

	m.mappings[sourceDB][targetDB][sourceType] = targetType
}

// GetSupportedDatabases implements the TypeMapper interface.
func (m *DefaultTypeMapper) GetSupportedDatabases() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	databases := make([]string, 0, len(m.mappings))
	for db := range m.mappings {
		databases = append(databases, db)
	}
	return databases
}

// IsSupported implements the TypeMapper interface.
func (m *DefaultTypeMapper) IsSupported(dbName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.mappings[strings.ToLower(dbName)]
	return ok
}

// GetMapping returns the raw mapping for a source database.
func (m *DefaultTypeMapper) GetMapping(sourceDB string) map[string]map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if mappings, ok := m.mappings[strings.ToLower(sourceDB)]; ok {
		// Return a copy to prevent external modification
		result := make(map[string]map[string]string)
		for targetDB, typeMappings := range mappings {
			result[targetDB] = make(map[string]string)
			for k, v := range typeMappings {
				result[targetDB][k] = v
			}
		}
		return result
	}
	return nil
}

// TypeMappingInfo provides detailed information about a type mapping.
type TypeMappingInfo struct {
	SourceType     string
	TargetType     string
	SourceDB       string
	TargetDB       string
	PrecisionLoss  bool
	SizeLimitation bool
	Notes          string
}

// GetMappingInfo returns detailed information about a type mapping.
func (m *DefaultTypeMapper) GetMappingInfo(sourceType, sourceDB, targetDB string) (*TypeMappingInfo, error) {
	targetType, err := m.MapType(sourceType, sourceDB, targetDB)
	if err != nil {
		return nil, err
	}

	info := &TypeMappingInfo{
		SourceType: sourceType,
		TargetType: targetType,
		SourceDB:   sourceDB,
		TargetDB:   targetDB,
	}

	// Add notes based on common transformation issues
	baseType := extractBaseType(strings.ToLower(sourceType))

	switch {
	case baseType == "json" && targetDB == "oracle":
		info.Notes = "JSON mapped to CLOB in Oracle; consider using Oracle 12c+ JSON features"
	case baseType == "enum":
		info.Notes = "ENUM mapped to VARCHAR; consider using CHECK constraints"
	case baseType == "set":
		info.Notes = "SET mapped to string type; consider normalizing to separate table"
	case strings.Contains(sourceType, "text") && targetDB == "oracle":
		info.SizeLimitation = true
		info.Notes = "Large TEXT types may have size limitations in Oracle"
	case baseType == "blob" && targetDB == "postgresql":
		info.Notes = "BLOB mapped to BYTEA in PostgreSQL"
	case baseType == "boolean" && sourceDB == "mysql":
		info.Notes = "MySQL BOOLEAN is actually TINYINT(1)"
	}

	return info, nil
}
