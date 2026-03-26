package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lush/dbschema-sync/internal/schema/extractor"
)

// Connect connects to a database using the specified driver
func Connect(driverName, dsn string) (*sql.DB, error) {
	driver, err := Get(driverName)
	if err != nil {
		return nil, err
	}

	return driver.Connect(context.Background(), dsn)
}

// GetExtractor returns a schema extractor for the given driver
func GetExtractor(driverName string) (extractor.SchemaExtractor, error) {
	driver, err := Get(driverName)
	if err != nil {
		return nil, err
	}

	// Check if driver implements SchemaExtractor
	if extractor, ok := driver.(extractor.SchemaExtractor); ok {
		return extractor, nil
	}

	return nil, fmt.Errorf("driver %s does not implement schema extractor", driverName)
}
