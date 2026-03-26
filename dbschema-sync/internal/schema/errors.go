// Package schema provides database schema introspection and synchronization functionality.
package schema

import (
	"errors"
	"fmt"
)

// SchemaError represents an error that occurred during schema operations.
type SchemaError struct {
	Table   string
	Column  string
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *SchemaError) Error() string {
	if e.Table != "" && e.Column != "" {
		return fmt.Sprintf("schema error [table=%s, column=%s]: %s", e.Table, e.Column, e.Message)
	}
	if e.Table != "" {
		return fmt.Sprintf("schema error [table=%s]: %s", e.Table, e.Message)
	}
	return fmt.Sprintf("schema error: %s", e.Message)
}

// Unwrap returns the underlying error cause.
func (e *SchemaError) Unwrap() error {
	return e.Cause
}

// WithTable returns a new SchemaError with the specified table.
func (e *SchemaError) WithTable(table string) *SchemaError {
	return &SchemaError{
		Table:   table,
		Column:  e.Column,
		Message: e.Message,
		Cause:   e.Cause,
	}
}

// WithColumn returns a new SchemaError with the specified column.
func (e *SchemaError) WithColumn(column string) *SchemaError {
	return &SchemaError{
		Table:   e.Table,
		Column:  column,
		Message: e.Message,
		Cause:   e.Cause,
	}
}

// WithMessage returns a new SchemaError with the specified message.
func (e *SchemaError) WithMessage(message string) *SchemaError {
	return &SchemaError{
		Table:   e.Table,
		Column:  e.Column,
		Message: message,
		Cause:   e.Cause,
	}
}

// WithCause returns a new SchemaError with the specified cause.
func (e *SchemaError) WithCause(cause error) *SchemaError {
	return &SchemaError{
		Table:   e.Table,
		Column:  e.Column,
		Message: e.Message,
		Cause:   cause,
	}
}

// IsSchemaError checks if an error is a SchemaError.
func IsSchemaError(err error) bool {
	var schemaErr *SchemaError
	return errors.As(err, &schemaErr)
}

// AsSchemaError attempts to convert an error to a SchemaError.
// Returns the SchemaError and true if successful, nil and false otherwise.
func AsSchemaError(err error) (*SchemaError, bool) {
	var schemaErr *SchemaError
	if errors.As(err, &schemaErr) {
		return schemaErr, true
	}
	return nil, false
}

// NewSchemaError creates a new SchemaError with the given message.
func NewSchemaError(message string) *SchemaError {
	return &SchemaError{Message: message}
}

// NewSchemaErrorf creates a new SchemaError with a formatted message.
func NewSchemaErrorf(format string, args ...interface{}) *SchemaError {
	return &SchemaError{Message: fmt.Sprintf(format, args...)}
}

// NewSchemaErrorWithCause creates a new SchemaError with a cause.
func NewSchemaErrorWithCause(message string, cause error) *SchemaError {
	return &SchemaError{Message: message, Cause: cause}
}

// Sentinel errors for common schema-related failures.
var (
	// ErrSchemaMismatch indicates that the source and target schemas are incompatible.
	ErrSchemaMismatch = errors.New("schema mismatch detected")

	// ErrConnectionFailed indicates a database connection failure.
	ErrConnectionFailed = errors.New("database connection failed")

	// ErrDriverNotSupported indicates that the specified database driver is not supported.
	ErrDriverNotSupported = errors.New("database driver not supported")

	// ErrTableNotFound indicates that a requested table does not exist.
	ErrTableNotFound = errors.New("table not found")

	// ErrColumnNotFound indicates that a requested column does not exist.
	ErrColumnNotFound = errors.New("column not found")

	// ErrIndexNotFound indicates that a requested index does not exist.
	ErrIndexNotFound = errors.New("index not found")

	// ErrForeignKeyNotFound indicates that a requested foreign key does not exist.
	ErrForeignKeyNotFound = errors.New("foreign key not found")

	// ErrInvalidSchema indicates that the schema definition is invalid.
	ErrInvalidSchema = errors.New("invalid schema definition")

	// ErrIntrospectionFailed indicates that schema introspection failed.
	ErrIntrospectionFailed = errors.New("schema introspection failed")

	// ErrSyncFailed indicates that schema synchronization failed.
	ErrSyncFailed = errors.New("schema synchronization failed")

	// ErrTypeMappingFailed indicates that type mapping between databases failed.
	ErrTypeMappingFailed = errors.New("type mapping failed")

	// ErrCircularDependency indicates a circular dependency between tables.
	ErrCircularDependency = errors.New("circular dependency detected")

	// ErrConstraintViolation indicates a constraint violation.
	ErrConstraintViolation = errors.New("constraint violation")

	// ErrPermissionDenied indicates insufficient permissions.
	ErrPermissionDenied = errors.New("permission denied")
)

// IsSchemaMismatch checks if an error is ErrSchemaMismatch.
func IsSchemaMismatch(err error) bool {
	return errors.Is(err, ErrSchemaMismatch)
}

// IsConnectionFailed checks if an error is ErrConnectionFailed.
func IsConnectionFailed(err error) bool {
	return errors.Is(err, ErrConnectionFailed)
}

// IsDriverNotSupported checks if an error is ErrDriverNotSupported.
func IsDriverNotSupported(err error) bool {
	return errors.Is(err, ErrDriverNotSupported)
}

// IsTableNotFound checks if an error is ErrTableNotFound.
func IsTableNotFound(err error) bool {
	return errors.Is(err, ErrTableNotFound)
}

// IsColumnNotFound checks if an error is ErrColumnNotFound.
func IsColumnNotFound(err error) bool {
	return errors.Is(err, ErrColumnNotFound)
}

// ErrorCategory represents the category of a schema error.
type ErrorCategory string

const (
	// CategoryConnection errors related to database connections.
	CategoryConnection ErrorCategory = "connection"
	// CategoryIntrospection errors during schema introspection.
	CategoryIntrospection ErrorCategory = "introspection"
	// CategoryValidation errors related to schema validation.
	CategoryValidation ErrorCategory = "validation"
	// CategorySynchronization errors during schema synchronization.
	CategorySynchronization ErrorCategory = "synchronization"
	// CategoryMapping errors related to type mapping.
	CategoryMapping ErrorCategory = "mapping"
	// CategoryNotFound errors when resources are not found.
	CategoryNotFound ErrorCategory = "not_found"
	// CategoryPermission errors related to permissions.
	CategoryPermission ErrorCategory = "permission"
	// CategoryUnknown unknown error category.
	CategoryUnknown ErrorCategory = "unknown"
)

// CategorizedError wraps an error with a category.
type CategorizedError struct {
	Category ErrorCategory
	Err      error
}

// Error implements the error interface.
func (e *CategorizedError) Error() string {
	return fmt.Sprintf("[%s] %v", e.Category, e.Err)
}

// Unwrap returns the wrapped error.
func (e *CategorizedError) Unwrap() error {
	return e.Err
}

// NewCategorizedError creates a new categorized error.
func NewCategorizedError(category ErrorCategory, err error) *CategorizedError {
	return &CategorizedError{Category: category, Err: err}
}

// NewCategorizedErrorf creates a new categorized error with a formatted message.
func NewCategorizedErrorf(category ErrorCategory, format string, args ...interface{}) *CategorizedError {
	return &CategorizedError{Category: category, Err: fmt.Errorf(format, args...)}
}

// IsCategory checks if an error belongs to a specific category.
func IsCategory(err error, category ErrorCategory) bool {
	var catErr *CategorizedError
	if errors.As(err, &catErr) {
		return catErr.Category == category
	}
	return false
}

// GetErrorCategory determines the category of an error.
func GetErrorCategory(err error) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}

	// Check for categorized errors first
	var catErr *CategorizedError
	if errors.As(err, &catErr) {
		return catErr.Category
	}

	// Map sentinel errors to categories
	switch {
	case errors.Is(err, ErrConnectionFailed):
		return CategoryConnection
	case errors.Is(err, ErrIntrospectionFailed):
		return CategoryIntrospection
	case errors.Is(err, ErrInvalidSchema), errors.Is(err, ErrSchemaMismatch):
		return CategoryValidation
	case errors.Is(err, ErrSyncFailed):
		return CategorySynchronization
	case errors.Is(err, ErrTypeMappingFailed):
		return CategoryMapping
	case errors.Is(err, ErrTableNotFound), errors.Is(err, ErrColumnNotFound), errors.Is(err, ErrIndexNotFound):
		return CategoryNotFound
	case errors.Is(err, ErrPermissionDenied):
		return CategoryPermission
	default:
		return CategoryUnknown
	}
}
