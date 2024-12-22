// internal/config/config.go
//
// This package provides configuration management for the SSH Manager application.
// It handles loading, saving, and managing SSH host configurations, passwords, and keys.
// Additionally, it manages API key encryption and synchronization with external services.

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sshManager/internal/crypto"
	"sshManager/internal/models"
	"sshManager/internal/sync"
	"strings"
)

const (
	// DefaultConfigFileName specifies the default name of the configuration file.
	DefaultConfigFileName = "ssh_hosts.json"

	// DefaultConfigDir specifies the default directory for configuration files.
	DefaultConfigDir = ".config/sshm"

	// DefaultFilePerms defines the default file permissions for config files.
	DefaultFilePerms = 0600

	// DefaultKeysDir specifies the default directory name for storing SSH keys.
	DefaultKeysDir = "keys" // New constant added for keys directory
)

const (
	// ApiKeyFileName specifies the filename for storing the API key.
	ApiKeyFileName = "api_key.txt"
)

// Manager manages the configuration state, including hosts, passwords, and keys.
type Manager struct {
	configPath string         // Path to the configuration file.
	config     *models.Config // In-memory representation of the configuration.
	cipher     *crypto.Cipher // Cipher for encrypting and decrypting sensitive data.
}

// NewManager creates a new configuration manager.
// It initializes the manager with the provided configPath or uses the default path if none is provided.
func NewManager(configPath string) *Manager {
	if configPath == "" {
		// Use GetDefaultConfigPath() to obtain the default configuration path.
		defaultPath, err := GetDefaultConfigPath()
		if err == nil {
			configPath = defaultPath
		} else {
			// Fallback to the default config file name if the home directory cannot be determined.
			configPath = DefaultConfigFileName
		}
	}

	return &Manager{
		configPath: configPath,
		config:     &models.Config{},
	}
}

// Load loads the configuration from the config file.
// It ensures that necessary directories exist and initializes an empty configuration if the file does not exist.
func (m *Manager) Load() error {
	// Ensure the configuration directory exists.
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Ensure the keys directory exists.
	keysDir := filepath.Join(configDir, DefaultKeysDir)
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %v", err)
	}

	// Read the configuration file.
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// If the config file does not exist, initialize an empty configuration.
			m.config = &models.Config{
				Hosts:     make([]models.Host, 0),
				Passwords: make([]models.Password, 0),
				Keys:      make([]models.Key, 0), // New keys slice initialized
			}
			return m.Save() // Save the empty configuration to create the file.
		}
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse the JSON configuration data.
	if err := json.Unmarshal(data, m.config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	return nil
}

// Save writes the current configuration to the config file.
// It also synchronizes the configuration with an external API if an API key is available.
func (m *Manager) Save() error {
	// Marshal the configuration into JSON with indentation for readability.
	data, err := json.MarshalIndent(m.config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write the JSON data to the configuration file with appropriate permissions.
	if err := os.WriteFile(m.configPath, data, DefaultFilePerms); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	// If an API key is available and not in local mode, synchronize the configuration with the API.
	if apiKey, err := m.LoadApiKey(m.cipher); err == nil {
		keysDir := filepath.Join(filepath.Dir(m.configPath), DefaultKeysDir)

		// Push data to the API, encrypting sensitive information using the cipher.
		if err := sync.PushToAPI(apiKey, m.configPath, keysDir, m.cipher); err != nil {
			return fmt.Errorf("failed to sync with API: %v", err)
		}
	}

	return nil
}

// GetHosts returns a slice of all configured SSH hosts.
func (m *Manager) GetHosts() []models.Host {
	return m.config.Hosts
}

// AddHost adds a new SSH host to the configuration.
func (m *Manager) AddHost(host models.Host) {
	m.config.Hosts = append(m.config.Hosts, host)
}

// UpdateHost updates an existing SSH host at the specified index.
// It returns an error if the index is out of bounds.
func (m *Manager) UpdateHost(index int, host models.Host) error {
	if index < 0 || index >= len(m.config.Hosts) {
		return errors.New("invalid host index")
	}
	m.config.Hosts[index] = host
	return nil
}

// DeleteHost removes an SSH host from the configuration at the specified index.
// It returns an error if the index is out of bounds.
func (m *Manager) DeleteHost(index int) error {
	if index < 0 || index >= len(m.config.Hosts) {
		return errors.New("invalid host index")
	}
	m.config.Hosts = append(m.config.Hosts[:index], m.config.Hosts[index+1:]...)
	return nil
}

// GetPasswords returns a slice of all stored passwords.
func (m *Manager) GetPasswords() []models.Password {
	return m.config.Passwords
}

// AddPassword adds a new password to the configuration.
func (m *Manager) AddPassword(password models.Password) {
	m.config.Passwords = append(m.config.Passwords, password)
}

// UpdatePassword updates an existing password at the specified index.
// It returns an error if the index is out of bounds.
func (m *Manager) UpdatePassword(index int, password models.Password) error {
	if index < 0 || index >= len(m.config.Passwords) {
		return errors.New("invalid password index")
	}
	m.config.Passwords[index] = password
	return nil
}

// DeletePassword removes a password from the configuration at the specified index.
// It ensures that the password is not in use by any host before deletion.
// Returns an error if the index is invalid or the password is in use.
func (m *Manager) DeletePassword(index int) error {
	if index < 0 || index >= len(m.config.Passwords) {
		return errors.New("invalid password index")
	}
	// Check if the password is used by any host.
	for _, host := range m.config.Hosts {
		if host.PasswordID == index {
			return errors.New("password is in use by a host")
		}
	}
	m.config.Passwords = append(m.config.Passwords[:index], m.config.Passwords[index+1:]...)
	return nil
}

// GetPassword retrieves a password by its index.
// Returns an error if the index is out of bounds.
func (m *Manager) GetPassword(index int) (models.Password, error) {
	if index < 0 || index >= len(m.config.Passwords) {
		return models.Password{}, errors.New("invalid password index")
	}
	return m.config.Passwords[index], nil
}

// FindHostByName searches for an SSH host by its name.
// Returns the host, its index, or an error if not found.
func (m *Manager) FindHostByName(name string) (models.Host, int, error) {
	for i, host := range m.config.Hosts {
		if host.Name == name {
			return host, i, nil
		}
	}
	return models.Host{}, -1, errors.New("host not found")
}

// GetDefaultConfigPath returns the default path for the configuration file.
// It ensures that the configuration directory exists.
func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %v", err)
	}

	// Create the configuration directory if it does not exist.
	configDir := filepath.Join(homeDir, DefaultConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("could not create config directory: %v", err)
	}

	return filepath.Join(configDir, DefaultConfigFileName), nil
}

// GetKeys returns a slice of all stored SSH keys.
func (m *Manager) GetKeys() []models.Key {
	return m.config.Keys
}

// AddKey adds a new SSH key to the configuration.
// It ensures that the key description is unique and handles local key storage if required.
func (m *Manager) AddKey(key models.Key) error {
	// Check if a key with the same description already exists.
	for _, k := range m.config.Keys {
		if k.Description == key.Description {
			return fmt.Errorf("key with description '%s' already exists", key.Description)
		}
	}

	// If the key is to be stored locally, handle file operations.
	if key.KeyData != "" {
		// Obtain the path where the key should be stored.
		keyPath, err := key.GetKeyPath()
		if err != nil {
			return fmt.Errorf("failed to get key path: %v", err)
		}

		// Ensure the directory for the key exists.
		keyDir := filepath.Dir(keyPath)
		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return fmt.Errorf("failed to create key directory: %v", err)
		}

		// Write the raw key data to the file, trimming any whitespace.
		keyContent := strings.TrimSpace(key.RawKeyData)
		if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
			return fmt.Errorf("failed to write key file: %v", err)
		}
	}

	// Append the new key to the configuration.
	m.config.Keys = append(m.config.Keys, key)
	return nil
}

// UpdateKey updates an existing SSH key at the specified index.
// It handles the transition between local and external storage and ensures file integrity.
// Returns an error if the index is invalid.
func (m *Manager) UpdateKey(index int, key models.Key) error {
	if index < 0 || index >= len(m.config.Keys) {
		return errors.New("invalid key index")
	}

	// Retrieve the existing key.
	oldKey := m.config.Keys[index]

	// If the old key was stored locally and is being changed to external, remove the old key file.
	if oldKey.IsLocal() {
		oldPath, err := oldKey.GetKeyPath()
		if err == nil {
			os.Remove(oldPath) // Ignore error if the file does not exist.
		}
	}

	// If the new key is to be stored locally, handle file operations.
	if key.IsLocal() {
		keyPath, err := key.GetKeyPath()
		if err != nil {
			return fmt.Errorf("failed to get key path: %v", err)
		}

		// Ensure the directory for the key exists.
		keyDir := filepath.Dir(keyPath)
		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return fmt.Errorf("failed to create key directory: %v", err)
		}

		// Write the raw key data to the file, trimming any whitespace.
		keyContent := strings.TrimSpace(key.RawKeyData)
		if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
			return fmt.Errorf("failed to write key file: %v", err)
		}
	}

	// Update the key in the configuration.
	m.config.Keys[index] = key
	return nil
}

// DeleteKey removes an SSH key from the configuration at the specified index.
// It ensures that the key is not in use by any host before deletion and handles file removal if stored locally.
// Returns an error if the index is invalid or the key is in use.
func (m *Manager) DeleteKey(index int) error {
	if index < 0 || index >= len(m.config.Keys) {
		return fmt.Errorf("invalid key index: %d", index)
	}

	key := m.config.Keys[index]
	actualIndex := -(index + 1) // Convert to negative index used in PasswordID

	// Check if the key is used by any host.
	for _, host := range m.config.Hosts {
		if host.PasswordID == actualIndex {
			return fmt.Errorf("key '%s' is in use by host '%s'", key.Description, host.Name)
		}
	}

	// If the key is stored locally, remove the key file.
	if key.IsLocal() {
		if keyPath, err := key.GetKeyPath(); err == nil {
			_ = os.Remove(keyPath) // Ignore error if the file does not exist.
		}
	}

	// Remove the key from the configuration.
	m.config.Keys = append(m.config.Keys[:index], m.config.Keys[index+1:]...)
	return nil
}

// GetApiKeyPath returns the file path where the API key is stored.
func (m *Manager) GetApiKeyPath() (string, error) {
	configDir := filepath.Dir(m.configPath)
	return filepath.Join(configDir, ApiKeyFileName), nil
}

// SaveApiKey encrypts and saves the API key to the designated file.
// It uses the provided cipher for encryption and ensures the file is securely written.
func (m *Manager) SaveApiKey(apiKey string, cipher *crypto.Cipher) error {
	apiKeyPath, err := m.GetApiKeyPath()
	if err != nil {
		return err
	}

	// Encrypt the API key using the cipher.
	encryptedKey, err := cipher.Encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt API key: %v", err)
	}

	// Write the encrypted API key to the file with secure permissions.
	return os.WriteFile(apiKeyPath, []byte(encryptedKey), 0600)
}

// LoadApiKey reads and decrypts the API key from the designated file.
// It uses the provided cipher for decryption and returns the plaintext API key.
func (m *Manager) LoadApiKey(cipher *crypto.Cipher) (string, error) {
	apiKeyPath, err := m.GetApiKeyPath()
	if err != nil {
		return "", err
	}

	// Read the encrypted API key from the file.
	encryptedKey, err := os.ReadFile(apiKeyPath)
	if err != nil {
		return "", fmt.Errorf("api key file not found")
	}

	// Decrypt the API key using the cipher.
	apiKey, err := cipher.Decrypt(string(encryptedKey))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %v", err)
	}

	return apiKey, nil
}

// RemoveApiKey deletes the API key file from the filesystem.
// It returns an error if the removal fails.
func (m *Manager) RemoveApiKey() error {
	apiKeyPath, err := m.GetApiKeyPath()
	if err != nil {
		return err
	}
	return os.Remove(apiKeyPath)
}

// SetCipher assigns a cipher to the Manager for encrypting and decrypting sensitive data.
func (m *Manager) SetCipher(cipher *crypto.Cipher) {
	m.cipher = cipher
}

// GetConfigPath returns the file path of the current configuration.
func (m *Manager) GetConfigPath() string {
	return m.configPath
}
