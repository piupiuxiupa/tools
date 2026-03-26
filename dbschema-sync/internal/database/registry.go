package database

import (
	"fmt"
	"sync"
)

// registry is the global driver registry protected by a mutex for thread safety.
var (
	registry = make(map[string]Driver)
	mu       sync.RWMutex
)

// Register adds a driver to the global registry.
// This function is typically called in init() functions of driver packages.
// It is safe for concurrent use.
//
// Example:
//
//	func init() {
//	    database.Register("mysql", &MySQLDriver{})
//	}
func Register(name string, driver Driver) error {
	if name == "" {
		return fmt.Errorf("driver name cannot be empty")
	}
	if driver == nil {
		return fmt.Errorf("driver cannot be nil")
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := registry[name]; exists {
		return fmt.Errorf("driver %q is already registered", name)
	}

	registry[name] = driver
	return nil
}

// Get retrieves a driver from the registry by name.
// Returns an error if the driver is not registered.
// It is safe for concurrent use.
func Get(name string) (Driver, error) {
	mu.RLock()
	defer mu.RUnlock()

	driver, exists := registry[name]
	if !exists {
		return nil, fmt.Errorf("driver %q not registered (available: %v)", name, List())
	}

	return driver, nil
}

// List returns a sorted list of all registered driver names.
// It is safe for concurrent use.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}

	// Sort for consistent output
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[i] > names[j] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}

	return names
}

// Unregister removes a driver from the registry.
// This is primarily useful for testing.
// It is safe for concurrent use.
func Unregister(name string) bool {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := registry[name]; exists {
		delete(registry, name)
		return true
	}
	return false
}

// Clear removes all drivers from the registry.
// This is primarily useful for testing.
// It is safe for concurrent use.
func Clear() {
	mu.Lock()
	defer mu.Unlock()

	registry = make(map[string]Driver)
}
