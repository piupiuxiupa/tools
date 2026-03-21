package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/term"
	"gossh/internal/config"
	"gossh/internal/executor"
	"gossh/internal/output"
	"gossh/internal/ssh"
	"gossh/internal/transfer"
)

func isLocalHost(host string) bool {
	lower := strings.ToLower(host)
	return lower == "localhost" || lower == "127.0.0.1" || lower == "::1"
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(bytePassword), nil
}

func main() {
	var (
		configFile = pflag.StringP("config", "c", "config.yaml", "配置文件路径")
		group      = pflag.StringP("group", "g", "", "执行命令的服务器组")
		tags       = pflag.StringP("tags", "t", "", "按标签筛选服务器（逗号分隔）")
		hosts      = pflag.String("hosts", "", "指定配置中的主机（逗号分隔）")
		verbose    = pflag.BoolP("verbose", "v", false, "显示详细输出")
		list       = pflag.BoolP("list", "l", false, "列出所有服务器")
		quiet      = pflag.BoolP("quiet", "q", false, "静默模式，只显示简短状态")
		logFile    = pflag.String("log", "", "将输出保存到日志文件")

		host       = pflag.StringP("host", "h", "", "直接连接：目标主机IP或域名")
		port       = pflag.IntP("port", "P", 22, "直接连接：SSH端口")
		user       = pflag.StringP("user", "u", "", "直接连接：用户名")
		password   = pflag.StringP("password", "p", "", "直接连接：密码（不指定则交互式输入）")
		privateKey = pflag.String("key", "", "直接连接：私钥文件路径")
		keyPass    = pflag.String("key-pass", "", "直接连接：私钥密码（如果私钥有密码保护）")
		timeout    = pflag.Int("timeout", 10, "直接连接：连接超时时间（秒）")

		put    = pflag.String("put", "", "上传：本地文件路径（格式: local:remote）")
		get    = pflag.String("get", "", "下载：远程文件路径（格式: remote:local）")
		script = pflag.String("script", "", "本地脚本文件路径，在远程服务器上执行")
	)

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [选项] [命令]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "模式1 - 使用配置文件:\n")
		fmt.Fprintf(os.Stderr, "  %s -c config.yaml \"uptime\"                    # 在所有服务器上执行uptime\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -g web \"systemctl status nginx\"             # 在web组执行命令\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -t production \"df -h\"                       # 在production标签的服务器执行\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --hosts 192.168.1.1,192.168.1.2 \"ls -la\"    # 在配置中指定主机执行\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -l                                            # 列出所有配置的服务器\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n模式2 - 直接连接（无需配置文件）:\n")
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root \"uptime\"                      # 自动使用 ~/.ssh/id_rsa 密钥\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root -p \"xxx\" \"uptime\"            # 使用密码执行命令\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root \"uptime\"                     # 交互式输入密码\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root --key ~/.ssh/id_rsa \"df -h\"    # 指定密钥执行命令\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -P 2222 -u admin -p \"xxx\" \"whoami\"   # 指定端口\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h localhost \"whoami\"                                 # 本地执行，无需认证\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n模式3 - 文件传输（直接连接模式）:\n")
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root -p \"xxx\" --put ./file.txt:/tmp/file.txt    # 上传文件\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root -p \"xxx\" --get /etc/passwd:./passwd       # 下载文件\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n模式4 - 本地脚本在远程执行:\n")
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root -p \"xxx\" --script ./deploy.sh             # 执行本地脚本\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n高级选项:\n")
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root \"uptime\" --log result.log     # 保存日志到文件\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -h 192.168.1.1 -u root \"uptime\" -q                   # 静默模式\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n选项:\n")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	if *list {
		cfg, err := config.LoadConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		listServers(cfg)
		return
	}

	formatter := output.NewFormatter(*verbose, *quiet, *logFile)
	defer formatter.Close()

	if *host != "" {
		if isLocalHost(*host) {
			handleLocalMode(pflag.Args(), put, get, script, formatter)
		} else {
			handleDirectMode(host, port, user, password, privateKey, keyPass, timeout, pflag.Args(), put, get, script, formatter)
		}
		return
	}

	if *put != "" || *get != "" || *script != "" {
		fmt.Fprintf(os.Stderr, "错误: 文件传输和脚本执行功能需要使用 -h/--host 指定目标主机\n")
		os.Exit(1)
	}

	if pflag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "错误: 请提供要执行的命令\n\n")
		pflag.Usage()
		os.Exit(1)
	}

	command := pflag.Arg(0)
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	exec := executor.New(cfg)
	var results []ssh.Result

	switch {
	case *hosts != "":
		results = executeOnHosts(cfg, exec, *hosts, command)
	case *group != "":
		var err error
		results, err = exec.ExecuteOnGroup(*group, command)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
	case *tags != "":
		tagList := strings.Split(*tags, ",")
		for i := range tagList {
			tagList[i] = strings.TrimSpace(tagList[i])
		}
		results = exec.ExecuteOnTags(tagList, command)
	default:
		results = exec.ExecuteOnAll(command)
	}

	fmt.Print(formatter.FormatResults(results))
}

func handleLocalMode(args []string, put, get, script *string, formatter *output.Formatter) {
	if *put != "" {
		parts := strings.Split(*put, ":")
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "错误: --put 参数格式应为 local:remote\n")
			os.Exit(1)
		}
		localPath, remotePath := parts[0], parts[1]
		formatter.PrintQuiet("本地模式：复制 %s -> %s\n", localPath, remotePath)
		cmd := exec.Command("cp", localPath, remotePath)
		output, err := cmd.CombinedOutput()
		result := &ssh.Result{
			Host:    "localhost",
			Command: fmt.Sprintf("cp %s %s", localPath, remotePath),
			Output:  string(output),
			Success: err == nil,
		}
		if err != nil {
			result.Error = err.Error()
		}
		fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
		return
	}

	if *get != "" {
		parts := strings.Split(*get, ":")
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "错误: --get 参数格式应为 remote:local\n")
			os.Exit(1)
		}
		remotePath, localPath := parts[0], parts[1]
		formatter.PrintQuiet("本地模式：复制 %s -> %s\n", remotePath, localPath)
		cmd := exec.Command("cp", remotePath, localPath)
		output, err := cmd.CombinedOutput()
		result := &ssh.Result{
			Host:    "localhost",
			Command: fmt.Sprintf("cp %s %s", remotePath, localPath),
			Output:  string(output),
			Success: err == nil,
		}
		if err != nil {
			result.Error = err.Error()
		}
		fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
		return
	}

	if *script != "" {
		formatter.PrintQuiet("本地模式：执行脚本 %s\n", *script)
		cmdArgs := append([]string{*script}, args...)
		cmd := exec.Command("sh", cmdArgs...)
		output, err := cmd.CombinedOutput()
		result := &ssh.Result{
			Host:    "localhost",
			Command: fmt.Sprintf("sh %s", *script),
			Output:  string(output),
			Success: err == nil,
		}
		if err != nil {
			result.Error = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			}
		}
		fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
		return
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "错误: 请提供要执行的命令\n")
		os.Exit(1)
	}

	result := executeLocal(args[0])
	fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
}

func getDefaultSSHKey() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.ssh/id_rsa"
}

func handleDirectMode(host *string, port *int, user, password, privateKey, keyPass *string, timeout *int, args []string, put, get, script *string, formatter *output.Formatter) {
	if *user == "" {
		fmt.Fprintf(os.Stderr, "错误: 必须指定用户名 (-u/--user)\n")
		os.Exit(1)
	}

	useDefaultKey := false
	defaultKey := getDefaultSSHKey()

	if *privateKey == "" && *password == "" {
		if defaultKey != "" {
			if _, err := os.Stat(defaultKey); err == nil {
				*privateKey = defaultKey
				useDefaultKey = true
				formatter.PrintQuiet("使用默认密钥: %s\n", defaultKey)
			}
		}
	}

	if *privateKey == "" && *password == "" {
		var err error
		*password, err = readPassword("请输入密码: ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 读取密码失败: %v\n", err)
			os.Exit(1)
		}
		if *password == "" {
			fmt.Fprintf(os.Stderr, "错误: 密码不能为空\n")
			os.Exit(1)
		}
	}

	serverConfig := &config.ServerConfig{
		Host:                 *host,
		Port:                 *port,
		User:                 *user,
		Password:             *password,
		PrivateKey:           *privateKey,
		PrivateKeyPassphrase: *keyPass,
	}

	formatter.PrintQuiet("正在连接 %s@%s:%d ...\n", *user, *host, *port)

	client := ssh.NewClient(serverConfig)
	if err := client.Connect(time.Duration(*timeout) * time.Second); err != nil {
		result := &ssh.Result{
			Host:    *host,
			Success: false,
			Error:   err.Error(),
		}
		fmt.Print(formatter.FormatResults([]ssh.Result{*result}))

		if useDefaultKey {
			fmt.Fprintf(os.Stderr, "\n提示: 使用默认密钥 '%s' 连接失败\n", defaultKey)
			fmt.Fprintf(os.Stderr, "可能的原因:\n")
			fmt.Fprintf(os.Stderr, "  1. 密钥文件不存在或路径不正确\n")
			fmt.Fprintf(os.Stderr, "  2. 密钥文件权限不正确（应为 600）\n")
			fmt.Fprintf(os.Stderr, "  3. 服务器上没有配置该密钥对应的公钥\n")
			fmt.Fprintf(os.Stderr, "  4. 密钥需要密码保护但未提供\n")
			fmt.Fprintf(os.Stderr, "\n建议:\n")
			fmt.Fprintf(os.Stderr, "  - 使用 --key 指定其他密钥文件路径\n")
			fmt.Fprintf(os.Stderr, "  - 使用 -p/--password 使用密码认证\n")
			fmt.Fprintf(os.Stderr, "  - 检查服务器上的 ~/.ssh/authorized_keys 是否包含正确公钥\n")
		}

		os.Exit(1)
	}
	defer client.Close()

	if *put != "" {
		parts := strings.Split(*put, ":")
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "错误: --put 参数格式应为 local:remote\n")
			os.Exit(1)
		}
		localPath, remotePath := parts[0], parts[1]

		trans := transfer.New(client.GetClient())
		if err := trans.Upload(localPath, remotePath); err != nil {
			result := &ssh.Result{
				Host:    *host,
				Command: fmt.Sprintf("upload %s -> %s", localPath, remotePath),
				Success: false,
				Error:   err.Error(),
			}
			fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
			os.Exit(1)
		}

		result := &ssh.Result{
			Host:    *host,
			Command: fmt.Sprintf("upload %s -> %s", localPath, remotePath),
			Success: true,
			Output:  "文件上传成功",
		}
		fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
		return
	}

	if *get != "" {
		parts := strings.Split(*get, ":")
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "错误: --get 参数格式应为 remote:local\n")
			os.Exit(1)
		}
		remotePath, localPath := parts[0], parts[1]

		trans := transfer.New(client.GetClient())
		if err := trans.Download(remotePath, localPath); err != nil {
			result := &ssh.Result{
				Host:    *host,
				Command: fmt.Sprintf("download %s -> %s", remotePath, localPath),
				Success: false,
				Error:   err.Error(),
			}
			fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
			os.Exit(1)
		}

		result := &ssh.Result{
			Host:    *host,
			Command: fmt.Sprintf("download %s -> %s", remotePath, localPath),
			Success: true,
			Output:  "文件下载成功",
		}
		fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
		return
	}

	if *script != "" {
		trans := transfer.New(client.GetClient())
		output, err := trans.UploadAndExecute(*script, args)

		result := &ssh.Result{
			Host:    *host,
			Command: fmt.Sprintf("script %s", *script),
			Output:  output,
			Success: err == nil,
		}
		if err != nil {
			result.Error = err.Error()
		}
		fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
		return
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "错误: 请提供要执行的命令\n")
		os.Exit(1)
	}

	result, err := client.ExecuteCommand(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(formatter.FormatResults([]ssh.Result{*result}))
}

func executeLocal(command string) *ssh.Result {
	result := &ssh.Result{
		Host:    "localhost",
		Command: command,
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()

	result.Output = string(output)
	result.Success = err == nil

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = err.Error()
		} else {
			result.Error = err.Error()
		}
	}

	return result
}

func listServers(cfg *config.Config) {
	fmt.Println("配置的服务器列表:")
	fmt.Println()

	if len(cfg.Groups) > 0 {
		fmt.Println("服务器组:")
		for groupName, servers := range cfg.Groups {
			fmt.Printf("  [%s]\n", groupName)
			for _, s := range servers {
				fmt.Printf("    - %s@%s:%d\n", s.User, s.Host, s.GetPort())
				if len(s.Tags) > 0 {
					fmt.Printf("      标签: %s\n", strings.Join(s.Tags, ", "))
				}
			}
		}
		fmt.Println()
	}

	if len(cfg.Servers) > 0 {
		fmt.Println("独立服务器:")
		for _, s := range cfg.Servers {
			fmt.Printf("  - %s@%s:%d\n", s.User, s.Host, s.GetPort())
			if len(s.Tags) > 0 {
				fmt.Printf("    标签: %s\n", strings.Join(s.Tags, ", "))
			}
		}
	}
}

func executeOnHosts(cfg *config.Config, exec *executor.Executor, hostStr, command string) []ssh.Result {
	hostList := strings.Split(hostStr, ",")
	allServers := cfg.GetAllServers()

	var targetServers []config.ServerConfig

	for _, h := range hostList {
		h = strings.TrimSpace(h)
		found := false

		for _, s := range allServers {
			if s.Host == h {
				targetServers = append(targetServers, s)
				found = true
				break
			}
		}

		if !found {
			fmt.Fprintf(os.Stderr, "警告: 未找到主机 %s\n", h)
		}
	}

	return exec.ExecuteSequential(targetServers, command)
}
