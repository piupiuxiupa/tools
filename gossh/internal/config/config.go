package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	Port        int `yaml:"port"`
	Timeout     int `yaml:"timeout"`
	Concurrency int `yaml:"concurrency"`
}

type ServerConfig struct {
	Host                 string            `yaml:"host"`
	Port                 int               `yaml:"port"`
	User                 string            `yaml:"user"`
	Password             string            `yaml:"password"`
	PrivateKey           string            `yaml:"private_key"`
	PrivateKeyPassphrase string            `yaml:"private_key_passphrase"`
	Tags                 []string          `yaml:"tags"`
	Vars                 map[string]string `yaml:"vars"`
}

func (s *ServerConfig) GetPort() int {
	if s.Port == 0 {
		return 22
	}
	return s.Port
}

func (s *ServerConfig) ExpandPrivateKey() string {
	if s.PrivateKey == "" {
		return ""
	}

	keyPath := s.PrivateKey
	if strings.HasPrefix(keyPath, "~/") {
		home, _ := os.UserHomeDir()
		keyPath = filepath.Join(home, keyPath[2:])
	}
	return keyPath
}

type Config struct {
	Global  GlobalConfig              `yaml:"global"`
	Groups  map[string][]ServerConfig `yaml:"groups"`
	Servers []ServerConfig            `yaml:"servers"`
}

func (c *Config) GetConcurrency() int {
	if c.Global.Concurrency == 0 {
		return 10
	}
	return c.Global.Concurrency
}

func (c *Config) GetTimeout() int {
	if c.Global.Timeout == 0 {
		return 10
	}
	return c.Global.Timeout
}

func (c *Config) GetDefaultPort() int {
	if c.Global.Port == 0 {
		return 22
	}
	return c.Global.Port
}

func (c *Config) GetAllServers() []ServerConfig {
	var all []ServerConfig

	for _, servers := range c.Groups {
		all = append(all, servers...)
	}

	all = append(all, c.Servers...)
	return all
}

func (c *Config) GetServersByGroup(groupName string) ([]ServerConfig, bool) {
	servers, exists := c.Groups[groupName]
	return servers, exists
}

func (c *Config) GetServersByTags(tags []string) []ServerConfig {
	var result []ServerConfig
	allServers := c.GetAllServers()

	for _, server := range allServers {
		if server.HasAnyTag(tags) {
			result = append(result, server)
		}
	}

	return result
}

func (s *ServerConfig) HasAnyTag(tags []string) bool {
	for _, tag := range tags {
		for _, serverTag := range s.Tags {
			if tag == serverTag {
				return true
			}
		}
	}
	return false
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	config.applyDefaults()

	return &config, nil
}

func (c *Config) applyDefaults() {
	defaultPort := c.GetDefaultPort()

	for groupName := range c.Groups {
		for i := range c.Groups[groupName] {
			if c.Groups[groupName][i].Port == 0 {
				c.Groups[groupName][i].Port = defaultPort
			}
		}
	}

	for i := range c.Servers {
		if c.Servers[i].Port == 0 {
			c.Servers[i].Port = defaultPort
		}
	}
}
