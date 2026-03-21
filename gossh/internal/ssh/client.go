package ssh

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"gossh/internal/config"
)

type Client struct {
	config *config.ServerConfig
	client *ssh.Client
}

func NewClient(cfg *config.ServerConfig) *Client {
	return &Client{
		config: cfg,
	}
}

func (c *Client) Connect(timeout time.Duration) error {
	sshConfig, err := c.buildSSHConfig(timeout)
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.GetPort())
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.client = client
	return nil
}

func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Client) GetClient() *ssh.Client {
	return c.client
}

func (c *Client) ExecuteCommand(command string) (*Result, error) {
	if c.client == nil {
		return nil, fmt.Errorf("SSH client not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	result := &Result{
		Host:    c.config.Host,
		Command: command,
	}

	output, err := session.CombinedOutput(command)
	result.Output = string(output)
	result.Success = err == nil

	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
			result.Error = err.Error()
		} else {
			result.Error = err.Error()
		}
	}

	return result, nil
}

func (c *Client) buildSSHConfig(timeout time.Duration) (*ssh.ClientConfig, error) {
	authMethods := []ssh.AuthMethod{}

	if c.config.PrivateKey != "" {
		keyPath := c.config.ExpandPrivateKey()
		signer, err := loadPrivateKey(keyPath, c.config.PrivateKeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to load private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if c.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(c.config.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	return &ssh.ClientConfig{
		User:            c.config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}, nil
}

func loadPrivateKey(path, passphrase string) (ssh.Signer, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %s: %w", path, err)
	}

	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("failed to parse encrypted private key: %w", err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return signer, nil
}
