package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Vault string `yaml:"vault"`
}

func Resolve(explicit string) (string, error) {
	if explicit != "" {
		return absolute(explicit)
	}
	if value := os.Getenv("PAPER_VAULT"); value != "" {
		return absolute(value)
	}
	path, err := Path()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		config, err := parse(data)
		if err != nil {
			return "", fmt.Errorf("read config %s: %w", path, err)
		}
		return absolute(config.Vault)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read config %s: %w", path, err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".paper"), nil
}

func Write(vault string) (string, error) {
	vault, err := absolute(vault)
	if err != nil {
		return "", err
	}
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	data, err := yaml.Marshal(Config{Vault: vault})
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	if err := atomicWrite(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func Path() (string, error) {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		root = filepath.Join(home, ".config")
	}
	return filepath.Join(root, "paper", "config.yaml"), nil
}

func parse(data []byte) (Config, error) {
	var result Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return Config{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, fmt.Errorf("multiple YAML documents are not allowed")
		}
		return Config{}, err
	}
	if strings.TrimSpace(result.Vault) == "" {
		return Config{}, fmt.Errorf("vault must not be empty")
	}
	return result, nil
}

func absolute(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("vault path must not be empty")
	}
	if strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand vault path: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"+string(filepath.Separator)))
	}
	result, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve vault path: %w", err)
	}
	return filepath.Clean(result), nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".paper-config-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
