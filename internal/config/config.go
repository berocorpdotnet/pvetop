package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Host     string `json:"host"`
	Port	string `json:"port"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type EncryptedConfig struct {
	Data []byte `json:"data"`
	IV   []byte `json:"iv"`
}

func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	return filepath.Join(home, ".config", "pvetop"), nil
}

func getConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.enc"), nil
}

func getMachineKey() ([]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	
	home, err := os.UserHomeDir()
	if err != nil {
		home = "unknown"
	}
	
	machineID := fmt.Sprintf("%s:%s:%s", hostname, home, runtime.GOOS)
	
	hash := sha256.Sum256([]byte(machineID))
	return hash[:], nil
}

func Exists() bool {
	configPath, err := getConfigPath()
	if err != nil {
		return false
	}
	
	_, err = os.Stat(configPath)
	return err == nil
}

func Save(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	key, err := getMachineKey()
	if err != nil {
		return fmt.Errorf("failed to generate machine key: %w", err)
	}
	
	encryptedData, err := encrypt(data, key)
	if err != nil {
		return fmt.Errorf("failed to encrypt config: %w", err)
	}
	
	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(encryptedData); err != nil {
		return fmt.Errorf("failed to write encrypted config: %w", err)
	}
	
	return nil
}

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist")
	}
	
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()
	
	var encryptedData EncryptedConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&encryptedData); err != nil {
		return nil, fmt.Errorf("failed to read encrypted config: %w", err)
	}
	
	key, err := getMachineKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate machine key: %w", err)
	}
	
	data, err := decrypt(encryptedData.Data, encryptedData.IV, key)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config (wrong machine?): %w", err)
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	return &config, nil
}

func encrypt(data, key []byte) (*EncryptedConfig, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	iv := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	
	ciphertext := gcm.Seal(nil, iv, data, nil)
	
	return &EncryptedConfig{
		Data: ciphertext,
		IV:   iv,
	}, nil
}

func decrypt(ciphertext, iv, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

func Delete() error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete config file: %w", err)
	}
	
	return nil
}

func GetConfigLocation() (string, error) {
	return getConfigPath()
}
