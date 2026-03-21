package output

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gossh/internal/ssh"
)

type Formatter struct {
	verbose bool
	quiet   bool
	logFile *os.File
	mu      sync.Mutex
}

func NewFormatter(verbose, quiet bool, logPath string) *Formatter {
	f := &Formatter{
		verbose: verbose,
		quiet:   quiet,
	}

	if logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_TRUNC, 0644)
		if err == nil {
			f.logFile = logFile
			f.logFile.WriteString(fmt.Sprintf("# GoSSH Log - %s\n", time.Now().Format("2006-01-02 15:04:05")))
			f.logFile.WriteString("# ========================================\n\n")
		}
	}

	return f
}

func (f *Formatter) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logFile != nil {
		f.logFile.WriteString(fmt.Sprintf("\n# Log closed at %s\n", time.Now().Format("2006-01-02 15:04:05")))
		f.logFile.Close()
		f.logFile = nil
	}
}

func (f *Formatter) writeToLog(content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.logFile != nil {
		f.logFile.WriteString(content)
		f.logFile.Sync()
	}
}

func (f *Formatter) FormatResults(results []ssh.Result) string {
	if f.quiet {
		return f.formatQuietResults(results)
	}

	var sb strings.Builder

	successCount := 0
	failCount := 0

	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	sb.WriteString(fmt.Sprintf("\n========== 执行结果汇总 ==========\n"))
	sb.WriteString(fmt.Sprintf("总计: %d | 成功: %d | 失败: %d\n", len(results), successCount, failCount))
	sb.WriteString(fmt.Sprintf("================================\n\n"))

	for _, result := range results {
		sb.WriteString(f.formatSingleResult(result))
		sb.WriteString("\n")
	}

	output := sb.String()
	f.writeToLog(output)
	return output
}

func (f *Formatter) formatQuietResults(results []ssh.Result) string {
	var sb strings.Builder

	successCount := 0
	failCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	sb.WriteString(fmt.Sprintf("总计: %d | 成功: %d | 失败: %d\n", len(results), successCount, failCount))

	for _, result := range results {
		if result.Success {
			sb.WriteString(fmt.Sprintf("[OK] %s\n", result.Host))
		} else {
			sb.WriteString(fmt.Sprintf("[FAIL] %s: %s\n", result.Host, result.Error))
		}
	}

	output := sb.String()
	f.writeToLog(output)
	return output
}

func (f *Formatter) formatSingleResult(result ssh.Result) string {
	var sb strings.Builder

	status := "✓ SUCCESS"
	if !result.Success {
		status = "✗ FAILED"
	}

	sb.WriteString(fmt.Sprintf("[%s] %s\n", status, result.Host))
	sb.WriteString(fmt.Sprintf("  命令: %s\n", result.Command))

	if result.ExitCode != 0 {
		sb.WriteString(fmt.Sprintf("  退出码: %d\n", result.ExitCode))
	}

	if result.Error != "" {
		sb.WriteString(fmt.Sprintf("  错误: %s\n", result.Error))
	}

	if f.verbose || result.Output != "" {
		output := strings.TrimSpace(result.Output)
		if output != "" {
			sb.WriteString(fmt.Sprintf("  输出:\n%s\n", f.indent(output, "    ")))
		}
	}

	return sb.String()
}

func (f *Formatter) indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, prefix+line)
	}
	return strings.Join(result, "\n")
}

func (f *Formatter) FormatSimple(results []ssh.Result) string {
	var sb strings.Builder

	for _, result := range results {
		if result.Success {
			sb.WriteString(fmt.Sprintf("%s: OK\n", result.Host))
		} else {
			sb.WriteString(fmt.Sprintf("%s: FAILED - %s\n", result.Host, result.Error))
		}
	}

	output := sb.String()
	f.writeToLog(output)
	return output
}

func (f *Formatter) PrintQuiet(format string, args ...interface{}) string {
	content := fmt.Sprintf(format, args...)
	f.writeToLog(content)
	if !f.quiet {
		fmt.Print(content)
	}
	return content
}

func (f *Formatter) IsQuiet() bool {
	return f.quiet
}
