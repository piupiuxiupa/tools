package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestConfig_setDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name:   "all defaults applied",
			config: Config{},
			expected: Config{
				MaxOpenConns:    DefaultMaxOpenConns,
				MaxIdleConns:    DefaultMaxIdleConns,
				ConnMaxLifetime: DefaultConnMaxLifetime,
				ConnMaxIdleTime: DefaultConnMaxIdleTime,
				ReadTimeout:     DefaultReadTimeout,
				WriteTimeout:    DefaultWriteTimeout,
				ConnectTimeout:  DefaultConnectTimeout,
			},
		},
		{
			name: "partial defaults - some values set",
			config: Config{
				MaxOpenConns: 20,
				ReadTimeout:  600 * time.Second,
			},
			expected: Config{
				MaxOpenConns:    20,
				MaxIdleConns:    DefaultMaxIdleConns,
				ConnMaxLifetime: DefaultConnMaxLifetime,
				ConnMaxIdleTime: DefaultConnMaxIdleTime,
				ReadTimeout:     600 * time.Second,
				WriteTimeout:    DefaultWriteTimeout,
				ConnectTimeout:  DefaultConnectTimeout,
			},
		},
		{
			name: "all values set - no defaults",
			config: Config{
				MaxOpenConns:    5,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
				ConnMaxIdleTime: 2 * time.Minute,
				ReadTimeout:     120 * time.Second,
				WriteTimeout:    120 * time.Second,
				ConnectTimeout:  10 * time.Second,
			},
			expected: Config{
				MaxOpenConns:    5,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
				ConnMaxIdleTime: 2 * time.Minute,
				ReadTimeout:     120 * time.Second,
				WriteTimeout:    120 * time.Second,
				ConnectTimeout:  10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			cfg.setDefaults()

			if cfg.MaxOpenConns != tt.expected.MaxOpenConns {
				t.Errorf("MaxOpenConns = %v, want %v", cfg.MaxOpenConns, tt.expected.MaxOpenConns)
			}
			if cfg.MaxIdleConns != tt.expected.MaxIdleConns {
				t.Errorf("MaxIdleConns = %v, want %v", cfg.MaxIdleConns, tt.expected.MaxIdleConns)
			}
			if cfg.ConnMaxLifetime != tt.expected.ConnMaxLifetime {
				t.Errorf("ConnMaxLifetime = %v, want %v", cfg.ConnMaxLifetime, tt.expected.ConnMaxLifetime)
			}
			if cfg.ConnMaxIdleTime != tt.expected.ConnMaxIdleTime {
				t.Errorf("ConnMaxIdleTime = %v, want %v", cfg.ConnMaxIdleTime, tt.expected.ConnMaxIdleTime)
			}
			if cfg.ReadTimeout != tt.expected.ReadTimeout {
				t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, tt.expected.ReadTimeout)
			}
			if cfg.WriteTimeout != tt.expected.WriteTimeout {
				t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, tt.expected.WriteTimeout)
			}
			if cfg.ConnectTimeout != tt.expected.ConnectTimeout {
				t.Errorf("ConnectTimeout = %v, want %v", cfg.ConnectTimeout, tt.expected.ConnectTimeout)
			}
		})
	}
}

func TestManager_buildDSN(t *testing.T) {
	config := &Config{
		Host:            "localhost",
		Port:            9030,
		Username:        "admin",
		Password:        "secret123",
		Database:        "testdb",
		ConnectTimeout:  30 * time.Second,
		ReadTimeout:     300 * time.Second,
		WriteTimeout:    300 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    10,
		ConnMaxLifetime: 3 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	mgr := NewConnectionManager(config)
	m := mgr.(*manager)

	dsn := m.buildDSN()

	// time.Duration formats differently (e.g., 5m0s vs 300s), so we check parts of the DSN
	expectedParts := []string{
		"admin:secret123@tcp(localhost:9030)/testdb",
		"timeout=30s",
		"readTimeout=",
		"writeTimeout=",
		"parseTime=true",
		"loc=Local",
		"charset=utf8mb4",
	}

	for _, part := range expectedParts {
		if !strings.Contains(dsn, part) {
			t.Errorf("buildDSN() = %v, does not contain %v", dsn, part)
		}
	}
}

func TestNewConnectionManager(t *testing.T) {
	config := &Config{
		Host:     "localhost",
		Port:     9030,
		Username: "admin",
		Password: "secret",
		Database: "testdb",
	}

	mgr := NewConnectionManager(config)
	if mgr == nil {
		t.Fatal("NewConnectionManager() returned nil")
	}

	// Check that defaults are applied
	m := mgr.(*manager)
	if m.config.MaxOpenConns != DefaultMaxOpenConns {
		t.Errorf("MaxOpenConns not set to default: got %v, want %v", m.config.MaxOpenConns, DefaultMaxOpenConns)
	}
}

func TestManager_Close(t *testing.T) {
	// Create a mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	config := &Config{
		Host:     "localhost",
		Port:     9030,
		Username: "admin",
		Password: "secret",
		Database: "testdb",
	}

	mgr := NewConnectionManager(config).(*manager)
	mgr.db = db

	mock.ExpectClose()

	err = mgr.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestManager_Close_NoConnection(t *testing.T) {
	config := &Config{
		Host:     "localhost",
		Port:     9030,
		Username: "admin",
		Password: "secret",
		Database: "testdb",
	}

	mgr := NewConnectionManager(config)

	// Close without connection should not error
	err := mgr.Close()
	if err != nil {
		t.Errorf("Close() without connection error = %v", err)
	}
}

func TestManager_Ping_NotConnected(t *testing.T) {
	config := &Config{
		Host:     "localhost",
		Port:     9030,
		Username: "admin",
		Password: "secret",
		Database: "testdb",
	}

	mgr := NewConnectionManager(config)

	ctx := context.Background()
	err := mgr.Ping(ctx)
	if err == nil {
		t.Error("Ping() without connection should return error")
	}
}

func TestManager_Ping_WithMock(t *testing.T) {
	// Create a mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	config := &Config{
		Host:     "localhost",
		Port:     9030,
		Username: "admin",
		Password: "secret",
		Database: "testdb",
	}

	mgr := NewConnectionManager(config).(*manager)
	mgr.db = db

	mock.ExpectPing()

	ctx := context.Background()
	err = mgr.Ping(ctx)
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestManager_Connect(t *testing.T) {
	// This test requires a real database connection
	// We'll skip it if no database is available
	// Instead, we'll test the error case

	config := &Config{
		Host:            "invalid-host-xyz",
		Port:            9999,
		Username:        "test",
		Password:        "test",
		Database:        "test",
		ConnectTimeout:  1 * time.Second,
		ReadTimeout:     1 * time.Second,
		WriteTimeout:    1 * time.Second,
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: 1 * time.Second,
		ConnMaxIdleTime: 1 * time.Second,
	}

	mgr := NewConnectionManager(config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should fail to connect to invalid host
	_, err := mgr.Connect(ctx)
	if err == nil {
		t.Error("Connect() with invalid host should return error")
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultMaxOpenConns != 10 {
		t.Errorf("DefaultMaxOpenConns = %v, want %v", DefaultMaxOpenConns, 10)
	}
	if DefaultMaxIdleConns != 10 {
		t.Errorf("DefaultMaxIdleConns = %v, want %v", DefaultMaxIdleConns, 10)
	}
	if DefaultConnMaxLifetime != 3*time.Minute {
		t.Errorf("DefaultConnMaxLifetime = %v, want %v", DefaultConnMaxLifetime, 3*time.Minute)
	}
	if DefaultConnMaxIdleTime != 1*time.Minute {
		t.Errorf("DefaultConnMaxIdleTime = %v, want %v", DefaultConnMaxIdleTime, 1*time.Minute)
	}
	if DefaultReadTimeout != 300*time.Second {
		t.Errorf("DefaultReadTimeout = %v, want %v", DefaultReadTimeout, 300*time.Second)
	}
	if DefaultWriteTimeout != 300*time.Second {
		t.Errorf("DefaultWriteTimeout = %v, want %v", DefaultWriteTimeout, 300*time.Second)
	}
	if DefaultConnectTimeout != 30*time.Second {
		t.Errorf("DefaultConnectTimeout = %v, want %v", DefaultConnectTimeout, 30*time.Second)
	}
}
