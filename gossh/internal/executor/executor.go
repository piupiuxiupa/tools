package executor

import (
	"fmt"
	"sync"
	"time"

	"gossh/internal/config"
	"gossh/internal/ssh"
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
