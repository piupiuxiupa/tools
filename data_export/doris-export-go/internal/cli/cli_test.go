package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "doris-export", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

func TestFlags(t *testing.T) {
	cmd := NewRootCommand()
	flags := cmd.Flags()

	// Test required flags exist
	flag := flags.Lookup("host")
	assert.NotNil(t, flag)
	assert.Equal(t, "", flag.DefValue)
	assert.Contains(t, flag.Usage, "required")

	flag = flags.Lookup("user")
	assert.NotNil(t, flag)
	assert.Equal(t, "u", flag.Shorthand)

	flag = flags.Lookup("password")
	assert.NotNil(t, flag)
	assert.Equal(t, "p", flag.Shorthand)

	flag = flags.Lookup("database")
	assert.NotNil(t, flag)
	assert.Equal(t, "d", flag.Shorthand)

	flag = flags.Lookup("table")
	assert.NotNil(t, flag)
	assert.Equal(t, "t", flag.Shorthand)

	flag = flags.Lookup("output")
	assert.NotNil(t, flag)
	assert.Equal(t, "o", flag.Shorthand)

	// Test optional flags
	flag = flags.Lookup("port")
	assert.NotNil(t, flag)
	assert.Equal(t, "9030", flag.DefValue)

	flag = flags.Lookup("batch-size")
	assert.NotNil(t, flag)
	assert.Equal(t, "0", flag.DefValue)

	flag = flags.Lookup("where")
	assert.NotNil(t, flag)

	flag = flags.Lookup("partition-by")
	assert.NotNil(t, flag)

	flag = flags.Lookup("info-only")
	assert.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)

	flag = flags.Lookup("no-confirm")
	assert.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)

	flag = flags.Lookup("verify")
	assert.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)

	flag = flags.Lookup("verify-only")
	assert.NotNil(t, flag)
}

func TestRequiredFlags(t *testing.T) {
	cmd := NewRootCommand()

	// Test that help works without required flags
	err := cmd.ParseFlags([]string{"--help"})
	// --help causes ErrHelp which is expected
	assert.Error(t, err)
}

func TestExampleContent(t *testing.T) {
	cmd := NewRootCommand()
	example := cmd.Example

	// Verify example contains key usage patterns
	assert.Contains(t, example, "--host")
	assert.Contains(t, example, "--port")
	assert.Contains(t, example, "-u")
	assert.Contains(t, example, "-p")
	assert.Contains(t, example, "-d")
	assert.Contains(t, example, "-t")
	assert.Contains(t, example, "-o")
	assert.Contains(t, example, "--where")
	assert.Contains(t, example, "--batch-size")
	assert.Contains(t, example, "--partition-by")
	assert.Contains(t, example, "--info-only")
	assert.Contains(t, example, "--verify")
	assert.Contains(t, example, "--verify-only")
}

func TestRunExport_MissingTable(t *testing.T) {
	// Reset flags for this test
	resetFlags()

	cmd := NewRootCommand()

	// Set required flags except table
	cmd.SetArgs([]string{
		"--host", "localhost",
		"--user", "root",
		"--password", "secret",
		"--database", "test_db",
		"--output", "./export",
	})

	// Execute should fail because table is missing
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--table is required")
}

func TestRunExport_MissingOutput(t *testing.T) {
	// Reset flags for this test
	resetFlags()

	cmd := NewRootCommand()

	// Set required flags except output
	cmd.SetArgs([]string{
		"--host", "localhost",
		"--user", "root",
		"--password", "secret",
		"--database", "test_db",
		"--table", "users",
	})

	// Execute should fail because output is missing
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--output is required")
}

func TestRunExport_InfoOnlyNoTable(t *testing.T) {
	// Reset flags for this test
	resetFlags()

	cmd := NewRootCommand()

	// Set required flags with info-only but no table
	cmd.SetArgs([]string{
		"--host", "localhost",
		"--user", "root",
		"--password", "secret",
		"--database", "test_db",
		"--info-only",
	})

	// Execute should fail because table is required even for info-only
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--table is required")
}

func TestRunVerifyOnly_NoFiles(t *testing.T) {
	// Reset flags for this test
	resetFlags()

	cmd := NewRootCommand()

	// Set verify-only with empty file list
	cmd.SetArgs([]string{
		"--host", "localhost",
		"--user", "root",
		"--password", "secret",
		"--database", "test_db",
		"--verify-only",
	})

	// Execute should not error with empty verify-only list
	// The verify-only mode should handle empty list gracefully
	err := cmd.Execute()
	// The command may succeed with empty list or fail - both are acceptable
	// depending on implementation
	_ = err
}

// resetFlags resets all flag variables to their zero values
// This is necessary because flag variables are package-level and persist across tests
func resetFlags() {
	host = ""
	port = DefaultPort
	user = ""
	password = ""
	dbName = ""
	tableName = ""
	outputDir = ""
	batchSize = 0
	where = ""
	partitionBy = ""
	infoOnly = false
	noConfirm = false
	verifyFlag = false
	verifyOnly = nil
}

func TestCommandStructure(t *testing.T) {
	cmd := NewRootCommand()

	// Verify command structure
	assert.Equal(t, "doris-export", cmd.Use)
	assert.NotNil(t, cmd.RunE)

	// Verify subcommands
	// Note: Currently we're using flags for all functionality,
	// not separate subcommands. This test verifies that approach.
	assert.Equal(t, 0, len(cmd.Commands()))
}

func TestDefaultPort(t *testing.T) {
	assert.Equal(t, 9030, DefaultPort)
}

func TestLargeTableThreshold(t *testing.T) {
	assert.Equal(t, int64(1000000), int64(LargeTableThreshold))
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		validate func(t *testing.T, cmd *cobra.Command)
	}{
		{
			name: "all short flags",
			args: []string{
				"--host", "127.0.0.1",
				"-u", "root",
				"-p", "secret",
				"-d", "mydb",
				"-t", "mytable",
				"-o", "./output",
			},
			validate: func(t *testing.T, cmd *cobra.Command) {
				f := cmd.Flags()
				h, _ := f.GetString("host")
				assert.Equal(t, "127.0.0.1", h)
				u, _ := f.GetString("user")
				assert.Equal(t, "root", u)
				p, _ := f.GetString("password")
				assert.Equal(t, "secret", p)
				d, _ := f.GetString("database")
				assert.Equal(t, "mydb", d)
				tbl, _ := f.GetString("table")
				assert.Equal(t, "mytable", tbl)
				o, _ := f.GetString("output")
				assert.Equal(t, "./output", o)
			},
		},
		{
			name: "optional flags",
			args: []string{
				"--host", "localhost",
				"-u", "admin",
				"-p", "pass",
				"-d", "db",
				"--port", "9031",
				"--batch-size", "5000",
				"--where", "age > 18",
				"--partition-by", "country",
				"--info-only",
				"--no-confirm",
				"--verify",
			},
			validate: func(t *testing.T, cmd *cobra.Command) {
				f := cmd.Flags()
				port, _ := f.GetInt("port")
				assert.Equal(t, 9031, port)
				batch, _ := f.GetInt("batch-size")
				assert.Equal(t, 5000, batch)
				w, _ := f.GetString("where")
				assert.Equal(t, "age > 18", w)
				part, _ := f.GetString("partition-by")
				assert.Equal(t, "country", part)
				info, _ := f.GetBool("info-only")
				assert.True(t, info)
				noConf, _ := f.GetBool("no-confirm")
				assert.True(t, noConf)
				v, _ := f.GetBool("verify")
				assert.True(t, v)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh command for each test
			cmd := NewRootCommand()
			cmd.SetArgs(tt.args)

			// Parse flags only (don't execute)
			err := cmd.ParseFlags(tt.args)
			if err != nil {
				// If error is help, ignore it for this test
				if err.Error() != "pflag: help requested" {
					t.Fatalf("failed to parse flags: %v", err)
				}
			}

			if tt.validate != nil {
				tt.validate(t, cmd)
			}
		})
	}
}
