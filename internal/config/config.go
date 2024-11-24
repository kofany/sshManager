// internal/config/config.go - zaktualizuj początek pliku

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sshManager/internal/models"
)

const (
	DefaultConfigFileName = "ssh_hosts.json" // Zmiana na wielką literę
	DefaultConfigDir      = ".config/sshmen" // Zmiana na wielką literę
	DefaultFilePerms      = 0600
)

type Manager struct {
	configPath string
	config     *models.Config
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
func (m *Manager) Load() error {
	// Upewnij się, że katalog konfiguracyjny istnieje
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Jeśli plik nie istnieje, tworzymy nową pustą konfigurację
			m.config = &models.Config{
				Hosts:     make([]models.Host, 0),
				Passwords: make([]models.Password, 0),
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

// Save zapisuje konfigurację do pliku
func (m *Manager) Save() error {
	// Upewnij się, że katalog konfiguracyjny istnieje
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := json.MarshalIndent(m.config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(m.configPath, data, DefaultFilePerms); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
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
