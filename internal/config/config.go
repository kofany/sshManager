// internal/config/config.go - zaktualizuj początek pliku

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
	DefaultConfigFileName = "ssh_hosts.json"
	DefaultConfigDir      = ".config/sshm"
	DefaultFilePerms      = 0600
	DefaultKeysDir        = "keys" // Nowa stała
)

const ApiKeyFileName = "api_key.txt"

type Manager struct {
	configPath string
	config     *models.Config
	cipher     *crypto.Cipher // Dodane

}

// NewManager tworzy nowego menedżera konfiguracji
func NewManager(configPath string) *Manager {
	if configPath == "" {
		// Użyj GetDefaultConfigPath() do uzyskania ścieżki
		defaultPath, err := GetDefaultConfigPath()
		if err == nil {
			configPath = defaultPath
		} else {
			// Fallback do bieżącego katalogu jeśli nie można uzyskać ścieżki domowej
			configPath = DefaultConfigFileName
		}
	}

	return &Manager{
		configPath: configPath,
		config:     &models.Config{},
	}
}

// Load wczytuje konfigurację z pliku
// Load wczytuje konfigurację z pliku
func (m *Manager) Load() error {
	// Upewnij się, że katalog konfiguracyjny istnieje
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Upewnij się, że katalog na klucze istnieje
	keysDir := filepath.Join(configDir, DefaultKeysDir)
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %v", err)
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Jeśli plik nie istnieje, tworzymy nową pustą konfigurację
			m.config = &models.Config{
				Hosts:     make([]models.Host, 0),
				Passwords: make([]models.Password, 0),
				Keys:      make([]models.Key, 0), // Dodane
			}
			return m.Save() // Zapisujemy pustą konfigurację
		}
		return fmt.Errorf("failed to read config file: %v", err)
	}

	if err := json.Unmarshal(data, m.config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	return nil
}

func (m *Manager) Save() error {
	// Najpierw zapisujemy lokalnie
	data, err := json.MarshalIndent(m.config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(m.configPath, data, DefaultFilePerms); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	// Jeśli mamy klucz API i nie jesteśmy w trybie lokalnym, synchronizujemy
	if apiKey, err := m.LoadApiKey(m.cipher); err == nil {
		keysDir := filepath.Join(filepath.Dir(m.configPath), DefaultKeysDir)

		// Wypychamy dane do API z przekazaniem cipher do szyfrowania
		if err := sync.PushToAPI(apiKey, m.configPath, keysDir, m.cipher); err != nil {
			return fmt.Errorf("failed to sync with API: %v", err)
		}
	}

	return nil
}

// GetHosts zwraca listę wszystkich hostów
func (m *Manager) GetHosts() []models.Host {
	return m.config.Hosts
}

// AddHost dodaje nowego hosta
func (m *Manager) AddHost(host models.Host) {
	m.config.Hosts = append(m.config.Hosts, host)
}

// UpdateHost aktualizuje istniejącego hosta
func (m *Manager) UpdateHost(index int, host models.Host) error {
	if index < 0 || index >= len(m.config.Hosts) {
		return errors.New("invalid host index")
	}
	m.config.Hosts[index] = host
	return nil
}

// DeleteHost usuwa hosta
func (m *Manager) DeleteHost(index int) error {
	if index < 0 || index >= len(m.config.Hosts) {
		return errors.New("invalid host index")
	}
	m.config.Hosts = append(m.config.Hosts[:index], m.config.Hosts[index+1:]...)
	return nil
}

// GetPasswords zwraca listę wszystkich haseł
func (m *Manager) GetPasswords() []models.Password {
	return m.config.Passwords
}

// AddPassword dodaje nowe hasło
func (m *Manager) AddPassword(password models.Password) {
	m.config.Passwords = append(m.config.Passwords, password)
}

// UpdatePassword aktualizuje istniejące hasło
func (m *Manager) UpdatePassword(index int, password models.Password) error {
	if index < 0 || index >= len(m.config.Passwords) {
		return errors.New("invalid password index")
	}
	m.config.Passwords[index] = password
	return nil
}

// DeletePassword usuwa hasło
func (m *Manager) DeletePassword(index int) error {
	if index < 0 || index >= len(m.config.Passwords) {
		return errors.New("invalid password index")
	}
	// Sprawdzamy czy hasło nie jest używane przez żadnego hosta
	for _, host := range m.config.Hosts {
		if host.PasswordID == index {
			return errors.New("password is in use by a host")
		}
	}
	m.config.Passwords = append(m.config.Passwords[:index], m.config.Passwords[index+1:]...)
	return nil
}

// GetPassword zwraca hasło o danym indeksie
func (m *Manager) GetPassword(index int) (models.Password, error) {
	if index < 0 || index >= len(m.config.Passwords) {
		return models.Password{}, errors.New("invalid password index")
	}
	return m.config.Passwords[index], nil
}

// FindHostByName szuka hosta po nazwie
func (m *Manager) FindHostByName(name string) (models.Host, int, error) {
	for i, host := range m.config.Hosts {
		if host.Name == name {
			return host, i, nil
		}
	}
	return models.Host{}, -1, errors.New("host not found")
}

func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %v", err)
	}

	// Utwórz katalog konfiguracyjny jeśli nie istnieje
	configDir := filepath.Join(homeDir, DefaultConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("could not create config directory: %v", err)
	}

	return filepath.Join(configDir, DefaultConfigFileName), nil
}

// GetKeys zwraca listę wszystkich kluczy
func (m *Manager) GetKeys() []models.Key {
	return m.config.Keys
}

// AddKey dodaje nowy klucz
// W pliku internal/config/config.go
func (m *Manager) AddKey(key models.Key) error {
	// Sprawdź czy klucz o takiej nazwie już istnieje
	for _, k := range m.config.Keys {
		if k.Description == key.Description {
			return fmt.Errorf("key with description '%s' already exists", key.Description)
		}
	}

	// Jeśli klucz ma być przechowywany lokalnie
	if key.KeyData != "" {
		// Utwórz katalog na klucze jeśli nie istnieje
		keyPath, err := key.GetKeyPath()
		if err != nil {
			return fmt.Errorf("failed to get key path: %v", err)
		}

		keyDir := filepath.Dir(keyPath)
		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return fmt.Errorf("failed to create key directory: %v", err)
		}

		// Zapisz niezaszyfrowany klucz do pliku
		keyContent := strings.TrimSpace(key.RawKeyData) // usuwamy białe znaki z końca
		if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
			return fmt.Errorf("failed to write key file: %v", err)
		}
	}

	m.config.Keys = append(m.config.Keys, key)
	return nil
}

// UpdateKey aktualizuje istniejący klucz
func (m *Manager) UpdateKey(index int, key models.Key) error {
	if index < 0 || index >= len(m.config.Keys) {
		return errors.New("invalid key index")
	}

	// Jeśli zmieniamy klucz lokalny na zewnętrzny, usuń stary plik
	oldKey := m.config.Keys[index]
	if oldKey.IsLocal() {
		oldPath, err := oldKey.GetKeyPath()
		if err == nil {
			os.Remove(oldPath) // Ignorujemy błąd jeśli plik nie istnieje
		}
	}

	// Jeśli nowy klucz ma być przechowywany lokalnie
	if key.IsLocal() {
		keyPath, err := key.GetKeyPath()
		if err != nil {
			return fmt.Errorf("failed to get key path: %v", err)
		}

		keyDir := filepath.Dir(keyPath)
		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return fmt.Errorf("failed to create key directory: %v", err)
		}

		keyContent := strings.TrimSpace(key.RawKeyData) // usuwamy białe znaki z końca
		if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
			return fmt.Errorf("failed to write key file: %v", err)
		}
	}

	m.config.Keys[index] = key
	return nil
}

// DeleteKey usuwa klucz
func (m *Manager) DeleteKey(index int) error {
	if index < 0 || index >= len(m.config.Keys) {
		return fmt.Errorf("invalid key index: %d", index)
	}

	key := m.config.Keys[index]
	actualIndex := -(index + 1) // Konwertujemy na ujemny indeks używany w PasswordID

	// Sprawdź czy klucz nie jest używany przez żadnego hosta
	for _, host := range m.config.Hosts {
		if host.PasswordID == actualIndex {
			return fmt.Errorf("key '%s' is in use by host '%s'", key.Description, host.Name)
		}
	}

	// Usuń plik klucza jeśli był przechowywany lokalnie
	if key.IsLocal() {
		if keyPath, err := key.GetKeyPath(); err == nil {
			_ = os.Remove(keyPath)
		}
	}

	// Usuń klucz z konfiguracji
	m.config.Keys = append(m.config.Keys[:index], m.config.Keys[index+1:]...)
	return nil
}

// GetApiKeyPath zwraca ścieżkę do pliku z kluczem API
func (m *Manager) GetApiKeyPath() (string, error) {
	configDir := filepath.Dir(m.configPath)
	return filepath.Join(configDir, ApiKeyFileName), nil
}

// SaveApiKey zapisuje zaszyfrowany klucz API do pliku
func (m *Manager) SaveApiKey(apiKey string, cipher *crypto.Cipher) error {
	apiKeyPath, err := m.GetApiKeyPath()
	if err != nil {
		return err
	}

	// Szyfrowanie klucza
	encryptedKey, err := cipher.Encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt API key: %v", err)
	}

	// Zapisanie do pliku
	return os.WriteFile(apiKeyPath, []byte(encryptedKey), 0600)
}

// LoadApiKey wczytuje i deszyfruje klucz API
func (m *Manager) LoadApiKey(cipher *crypto.Cipher) (string, error) {
	apiKeyPath, err := m.GetApiKeyPath()
	if err != nil {
		return "", err
	}

	// Sprawdzenie czy plik istnieje
	encryptedKey, err := os.ReadFile(apiKeyPath)
	if err != nil {
		return "", fmt.Errorf("api key file not found")
	}

	// Deszyfrowanie
	apiKey, err := cipher.Decrypt(string(encryptedKey))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %v", err)
	}

	return apiKey, nil
}

// RemoveApiKey usuwa plik z kluczem API
func (m *Manager) RemoveApiKey() error {
	apiKeyPath, err := m.GetApiKeyPath()
	if err != nil {
		return err
	}
	return os.Remove(apiKeyPath)
}

// SetCipher ustawia obiekt szyfru
func (m *Manager) SetCipher(cipher *crypto.Cipher) {
	m.cipher = cipher
}

// GetConfigPath zwraca ścieżkę do pliku konfiguracyjnego
func (m *Manager) GetConfigPath() string {
	return m.configPath
}
