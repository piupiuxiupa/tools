package executor

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gossh/internal/config"
	"gossh/internal/ssh"
	"gossh/internal/transfer"
)

type Executor struct {
	config      *config.Config
	concurrency int
	timeout     time.Duration
}

func New(cfg *config.Config) *Executor {
	return &Executor{
		config:      cfg,
		concurrency: cfg.GetConcurrency(),
		timeout:     time.Duration(cfg.GetTimeout()) * time.Second,
	}
}

func (e *Executor) ExecuteOnAll(command string, excludeHosts []string) []ssh.Result {
	servers := e.config.GetAllServers()
	servers = e.config.FilterExcludedHosts(servers, excludeHosts)
	return e.executeOnServers(servers, command)
}

func (e *Executor) ExecuteOnGroup(groupName, command string, excludeHosts []string) ([]ssh.Result, error) {
	servers, exists := e.config.GetServersByGroup(groupName)
	if !exists {
		return nil, fmt.Errorf("group '%s' not found", groupName)
	}
	servers = e.config.FilterExcludedHosts(servers, excludeHosts)
	return e.executeOnServers(servers, command), nil
}

func (e *Executor) ExecuteOnTags(tags []string, command string, excludeHosts []string) []ssh.Result {
	servers := e.config.GetServersByTags(tags)
	servers = e.config.FilterExcludedHosts(servers, excludeHosts)
	return e.executeOnServers(servers, command)
}

func (e *Executor) executeOnServers(servers []config.ServerConfig, command string) []ssh.Result {
	if len(servers) == 0 {
		return []ssh.Result{}
	}

	results := make([]ssh.Result, len(servers))
	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for i, server := range servers {
		wg.Add(1)
		go func(index int, srv config.ServerConfig) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := e.executeOnServer(&srv, command)
			results[index] = *result
		}(i, server)
	}

	wg.Wait()
	return results
}

func (e *Executor) executeOnServer(serverConfig *config.ServerConfig, command string) *ssh.Result {
	client := ssh.NewClient(serverConfig)

	if err := client.Connect(e.timeout); err != nil {
		return &ssh.Result{
			Host:    serverConfig.Host,
			Command: command,
			Success: false,
			Error:   err.Error(),
		}
	}
	defer client.Close()

	result, err := client.ExecuteCommand(command)
	if err != nil {
		return &ssh.Result{
			Host:    serverConfig.Host,
			Command: command,
			Success: false,
			Error:   err.Error(),
		}
	}

	return result
}

func (e *Executor) ExecuteSequential(servers []config.ServerConfig, command string) []ssh.Result {
	results := make([]ssh.Result, 0, len(servers))

	for _, server := range servers {
		result := e.executeOnServer(&server, command)
		results = append(results, *result)
	}

	return results
}

func (e *Executor) TransferPutOnServers(servers []config.ServerConfig, localPath, remotePath string) []ssh.Result {
	if len(servers) == 0 {
		return []ssh.Result{}
	}

	results := make([]ssh.Result, len(servers))
	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for i, server := range servers {
		wg.Add(1)
		go func(index int, srv config.ServerConfig) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := e.transferPutOnServer(&srv, localPath, remotePath)
			results[index] = *result
		}(i, server)
	}

	wg.Wait()
	return results
}

func (e *Executor) transferPutOnServer(serverConfig *config.ServerConfig, localPath, remotePath string) *ssh.Result {
	client := ssh.NewClient(serverConfig)

	if err := client.Connect(e.timeout); err != nil {
		return &ssh.Result{
			Host:    serverConfig.Host,
			Command: fmt.Sprintf("upload %s -> %s", localPath, remotePath),
			Success: false,
			Error:   err.Error(),
		}
	}
	defer client.Close()

	trans := transfer.New(client.GetClient())
	if err := trans.Upload(localPath, remotePath); err != nil {
		return &ssh.Result{
			Host:    serverConfig.Host,
			Command: fmt.Sprintf("upload %s -> %s", localPath, remotePath),
			Success: false,
			Error:   err.Error(),
		}
	}

	return &ssh.Result{
		Host:    serverConfig.Host,
		Command: fmt.Sprintf("upload %s -> %s", localPath, remotePath),
		Success: true,
		Output:  "文件上传成功",
	}
}

func (e *Executor) TransferGetOnServers(servers []config.ServerConfig, remotePath, localDir string) []ssh.Result {
	if len(servers) == 0 {
		return []ssh.Result{}
	}

	results := make([]ssh.Result, len(servers))
	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for i, server := range servers {
		wg.Add(1)
		go func(index int, srv config.ServerConfig) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := e.transferGetOnServer(&srv, remotePath, localDir)
			results[index] = *result
		}(i, server)
	}

	wg.Wait()
	return results
}

func (e *Executor) transferGetOnServer(serverConfig *config.ServerConfig, remotePath, localDir string) *ssh.Result {
	client := ssh.NewClient(serverConfig)

	if err := client.Connect(e.timeout); err != nil {
		return &ssh.Result{
			Host:    serverConfig.Host,
			Command: fmt.Sprintf("download %s", remotePath),
			Success: false,
			Error:   err.Error(),
		}
	}
	defer client.Close()

	remoteFileName := filepath.Base(remotePath)
	localFileName := fmt.Sprintf("%s_%s", strings.ReplaceAll(serverConfig.Host, ".", "_"), remoteFileName)
	localPath := filepath.Join(localDir, localFileName)

	trans := transfer.New(client.GetClient())
	if err := trans.Download(remotePath, localPath); err != nil {
		return &ssh.Result{
			Host:    serverConfig.Host,
			Command: fmt.Sprintf("download %s -> %s", remotePath, localPath),
			Success: false,
			Error:   err.Error(),
		}
	}

	return &ssh.Result{
		Host:    serverConfig.Host,
		Command: fmt.Sprintf("download %s -> %s", remotePath, localPath),
		Success: true,
		Output:  fmt.Sprintf("文件下载成功: %s", localPath),
	}
}

func (e *Executor) ExecuteScriptOnServers(servers []config.ServerConfig, scriptPath string, args []string) []ssh.Result {
	if len(servers) == 0 {
		return []ssh.Result{}
	}

	results := make([]ssh.Result, len(servers))
	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for i, server := range servers {
		wg.Add(1)
		go func(index int, srv config.ServerConfig) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := e.executeScriptOnServer(&srv, scriptPath, args)
			results[index] = *result
		}(i, server)
	}

	wg.Wait()
	return results
}

func (e *Executor) executeScriptOnServer(serverConfig *config.ServerConfig, scriptPath string, args []string) *ssh.Result {
	client := ssh.NewClient(serverConfig)

	if err := client.Connect(e.timeout); err != nil {
		return &ssh.Result{
			Host:    serverConfig.Host,
			Command: fmt.Sprintf("script %s", scriptPath),
			Success: false,
			Error:   err.Error(),
		}
	}
	defer client.Close()

	trans := transfer.New(client.GetClient())
	output, err := trans.UploadAndExecute(scriptPath, args)

	result := &ssh.Result{
		Host:    serverConfig.Host,
		Command: fmt.Sprintf("script %s", scriptPath),
		Output:  output,
		Success: err == nil,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func (e *Executor) GetServersByGroup(groupName string) ([]config.ServerConfig, bool) {
	servers, exists := e.config.GetServersByGroup(groupName)
	return servers, exists
}

func (e *Executor) GetAllServers() []config.ServerConfig {
	return e.config.GetAllServers()
}

func (e *Executor) GetServersByTags(tags []string) []config.ServerConfig {
	return e.config.GetServersByTags(tags)
}

func (e *Executor) FilterExcludedHosts(servers []config.ServerConfig, excludeHosts []string) []config.ServerConfig {
	return e.config.FilterExcludedHosts(servers, excludeHosts)
}
