package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DbUrl string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

const configFileName = ".gatorconfig.json"

func ReadConfig() (*Config, error) {
	filePath, err := getConfigFilePath()
	if err != nil {
		return &Config{}, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return &Config{}, err
	}
	defer file.Close()

	var cfg Config
	err = json.NewDecoder(file).Decode(&cfg)
	if err != nil {
		return &Config{}, err
	}

	return &cfg, nil
}

func (c *Config) SetUser(username string) error {
	c.CurrentUserName = username
	return write(*c)
}

func getConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(home, configFileName)

	return path, nil
}

func write(cfg Config) error {
	path, err := getConfigFilePath()
	if err != nil {
		return err
	}

	file, err := os.Create(path)
    if err != nil {
        return err
    }
    defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(cfg)
}