package schema

import (
	"testing"

	"github.com/lush/dbschema-sync/pkg/types"
)

func TestNewGenerator(t *testing.T) {
	mapper := types.NewDefaultTypeMapper()

	tests := []struct {
		name    string
		dialect string
		wantErr bool
	}{
		{"MySQL", "mysql", false},
		{"PostgreSQL", "postgresql", false},
		{"Postgres", "postgres", false},
		{"Oracle", "oracle", false},
		{"Doris", "doris", false},
		{"Invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := NewGenerator(tt.dialect, mapper)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGenerator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gen == nil {
				t.Error("NewGenerator() returned nil generator")
			}
		})
	}
}

func TestMySQLGenerator(t *testing.T) {
	mapper := types.NewDefaultTypeMapper()
	gen := NewMySQLGenerator(mapper)

	t.Run("GenerateCreateTable", func(t *testing.T) {
		table := &types.Table{
			Name: "users",
			Columns: []*types.Column{
				{
					Name:          "id",
					DataType:      "int",
					Nullable:      false,
					AutoIncrement: true,
				},
				{
					Name:     "name",
					DataType: "varchar",
					Length:   intPtr(255),
					Nullable: false,
				},
				{
					Name:     "email",
					DataType: "varchar",
					Length:   intPtr(255),
					Nullable: true,
				},
			},
			Indexes: []*types.Index{
				{
					Name:    "idx_email",
					Columns: []string{"email"},
					Unique:  true,
				},
			},
			Engine:  "InnoDB",
			Charset: "utf8mb4",
		}

		ddl, err := gen.GenerateCreateTable(table)
		if err != nil {
			t.Fatalf("GenerateCreateTable() error = %v", err)
		}

		// Check basic structure
		if ddl == "" {
			t.Error("GenerateCreateTable() returned empty DDL")
		}
		if !contains(ddl, "CREATE TABLE") {
			t.Error("DDL missing CREATE TABLE")
		}
		if !contains(ddl, "`users`") {
			t.Error("DDL missing table name")
		}
		if !contains(ddl, "`id`") {
			t.Error("DDL missing id column")
		}
		if !contains(ddl, "AUTO_INCREMENT") {
			t.Error("DDL missing AUTO_INCREMENT")
		}
		if !contains(ddl, "UNIQUE KEY") {
			t.Error("DDL missing UNIQUE KEY")
		}
	})

	t.Run("GenerateDropTable", func(t *testing.T) {
		ddl, err := gen.GenerateDropTable("users")
		if err != nil {
			t.Fatalf("GenerateDropTable() error = %v", err)
		}
		expected := "DROP TABLE IF EXISTS `users`;"
		if ddl != expected {
			t.Errorf("GenerateDropTable() = %v, want %v", ddl, expected)
		}
	})
}

func TestPostgreSQLGenerator(t *testing.T) {
	mapper := types.NewDefaultTypeMapper()
	gen := NewPostgreSQLGenerator(mapper)

	t.Run("GenerateCreateTable", func(t *testing.T) {
		table := &types.Table{
			Name: "users",
			Columns: []*types.Column{
				{
					Name:          "id",
					DataType:      "integer",
					Nullable:      false,
					AutoIncrement: true,
				},
				{
					Name:     "name",
					DataType: "varchar",
					Length:   intPtr(255),
					Nullable: false,
				},
			},
		}

		ddl, err := gen.GenerateCreateTable(table)
		if err != nil {
			t.Fatalf("GenerateCreateTable() error = %v", err)
		}

		if ddl == "" {
			t.Error("GenerateCreateTable() returned empty DDL")
		}
		if !contains(ddl, "CREATE TABLE") {
			t.Error("DDL missing CREATE TABLE")
		}
		if !contains(ddl, "\"users\"") {
			t.Error("DDL missing quoted table name")
		}
	})

	t.Run("GenerateDropTable", func(t *testing.T) {
		ddl, err := gen.GenerateDropTable("users")
		if err != nil {
			t.Fatalf("GenerateDropTable() error = %v", err)
		}
		if !contains(ddl, "DROP TABLE") {
			t.Error("DDL missing DROP TABLE")
		}
		if !contains(ddl, "CASCADE") {
			t.Error("DDL missing CASCADE")
		}
	})
}

func TestOracleGenerator(t *testing.T) {
	mapper := types.NewDefaultTypeMapper()
	gen := NewOracleGenerator(mapper)

	t.Run("GenerateCreateTable", func(t *testing.T) {
		table := &types.Table{
			Name: "users",
			Columns: []*types.Column{
				{
					Name:     "id",
					DataType: "NUMBER",
					Nullable: false,
				},
				{
					Name:     "name",
					DataType: "VARCHAR2",
					Length:   intPtr(255),
					Nullable: false,
				},
			},
		}

		ddl, err := gen.GenerateCreateTable(table)
		if err != nil {
			t.Fatalf("GenerateCreateTable() error = %v", err)
		}

		if ddl == "" {
			t.Error("GenerateCreateTable() returned empty DDL")
		}
		if !contains(ddl, "CREATE TABLE") {
			t.Error("DDL missing CREATE TABLE")
		}
	})
}

func TestDorisGenerator(t *testing.T) {
	mapper := types.NewDefaultTypeMapper()
	gen := NewDorisGenerator(mapper)

	t.Run("GenerateCreateTable", func(t *testing.T) {
		table := &types.Table{
			Name: "users",
			Columns: []*types.Column{
				{
					Name:     "id",
					DataType: "INT",
					Nullable: false,
				},
				{
					Name:     "name",
					DataType: "VARCHAR",
					Length:   intPtr(255),
					Nullable: false,
				},
			},
		}

		ddl, err := gen.GenerateCreateTable(table)
		if err != nil {
			t.Fatalf("GenerateCreateTable() error = %v", err)
		}

		if ddl == "" {
			t.Error("GenerateCreateTable() returned empty DDL")
		}
		if !contains(ddl, "CREATE TABLE") {
			t.Error("DDL missing CREATE TABLE")
		}
		// Doris-specific features
		if !contains(ddl, "ENGINE=OLAP") {
			t.Error("DDL missing ENGINE=OLAP")
		}
		if !contains(ddl, "DISTRIBUTED BY HASH") {
			t.Error("DDL missing DISTRIBUTED BY HASH")
		}
	})
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
