package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type SavedConnection struct {
	Name     string `json:"name"`
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
	URI      string `json:"uri,omitempty"`
}

type Config struct {
	Connections []SavedConnection `json:"connections"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cli-sql"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "connections.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return &Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return &Config{}, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{}, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := filepath.Join(dir, "connections.json")
	return os.WriteFile(path, data, 0600)
}

func (c *Config) Add(conn SavedConnection) {
	for i, existing := range c.Connections {
		if existing.Name == conn.Name {
			c.Connections[i] = conn
			return
		}
	}
	c.Connections = append(c.Connections, conn)
}

func (c *Config) Delete(index int) {
	if index < 0 || index >= len(c.Connections) {
		return
	}
	c.Connections = append(c.Connections[:index], c.Connections[index+1:]...)
}
