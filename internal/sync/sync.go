package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sshManager/internal/crypto"
	"sshManager/internal/ui/messages"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	ApiBaseURL = "https://sshm.io/api/v1"
)

type SyncResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Data    SyncData `json:"data"`
}

type SyncData struct {
	Hosts     []interface{} `json:"hosts"`
	Passwords []interface{} `json:"passwords"`
	Keys      []interface{} `json:"keys"`
	LastSync  string        `json:"last_sync"`
}

// BackupConfigFile tworzy kopię pliku konfiguracyjnego
func BackupConfigFile(configPath string) error {
	// Czytamy oryginalny plik
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	// Tworzymy nazwę pliku backup
	backupPath := configPath + ".old"

	// Zapisujemy backup
	if err := os.WriteFile(backupPath, content, 0600); err != nil {
		return fmt.Errorf("error creating backup file: %v", err)
	}

	return nil
}

// BackupKeys tworzy kopie wszystkich kluczy
func BackupKeys(keysDir string) error {
	// Otwórz plik logów
	logFile, err := os.OpenFile("backup_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot create log file: %v", err)
	}
	defer logFile.Close()

	// Funkcja pomocnicza do logowania
	logDebug := func(message string) {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(logFile, "[%s] %s\n", timestamp, message)
	}

	logDebug("=== Starting backup operation ===")
	logDebug(fmt.Sprintf("Keys directory: %s", keysDir))

	// Sprawdź czy katalog kluczy istnieje
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		logDebug("Keys directory does not exist")
		return nil
	}

	entries, err := os.ReadDir(keysDir)
	if err != nil {
		logDebug(fmt.Sprintf("Error reading keys directory: %v", err))
		return fmt.Errorf("error reading keys directory: %v", err)
	}

	logDebug(fmt.Sprintf("Found %d entries in keys directory", len(entries)))

	// Najpierw usuń stare backupy (.old)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".old" {
			oldPath := filepath.Join(keysDir, entry.Name())
			logDebug(fmt.Sprintf("Removing old backup: %s", oldPath))
			if err := os.Remove(oldPath); err != nil {
				logDebug(fmt.Sprintf("Error removing old backup %s: %v", oldPath, err))
				return fmt.Errorf("error removing old backup %s: %v", entry.Name(), err)
			}
		}
	}

	// Teraz twórz nowe backupy
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) == ".old" {
			logDebug(fmt.Sprintf("Skipping entry: %s", entry.Name()))
			continue
		}

		originalPath := filepath.Join(keysDir, entry.Name())
		backupPath := originalPath + ".old"

		logDebug(fmt.Sprintf("Creating backup: %s -> %s", originalPath, backupPath))

		content, err := os.ReadFile(originalPath)
		if err != nil {
			logDebug(fmt.Sprintf("Error reading file %s: %v", originalPath, err))
			return fmt.Errorf("error reading key file %s: %v", entry.Name(), err)
		}

		if err := os.WriteFile(backupPath, content, 0600); err != nil {
			logDebug(fmt.Sprintf("Error creating backup file %s: %v", backupPath, err))
			return fmt.Errorf("error creating key backup %s: %v", entry.Name(), err)
		}
		logDebug(fmt.Sprintf("Successfully created backup for: %s", entry.Name()))
	}

	logDebug("=== Backup operation completed ===\n")
	return nil
}

// SyncWithAPI synchronizuje dane z API
func SyncWithAPI(apiKey string) (*SyncResponse, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", ApiBaseURL+"/sync", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code %d: %s", resp.StatusCode, body)
	}

	var syncResp SyncResponse
	if err := json.Unmarshal(body, &syncResp); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	return &syncResp, nil
}

func SaveAPIData(configPath, keysDir string, data SyncData, cipher *crypto.Cipher) error {
	// Przygotuj strukturę danych do zapisania
	configData := struct {
		Hosts     []interface{} `json:"hosts"`
		Passwords []interface{} `json:"passwords"`
		Keys      []interface{} `json:"keys"`
	}{
		Hosts:     data.Hosts,
		Passwords: data.Passwords,
		Keys:      data.Keys,
	}

	// Konwertuj do JSON
	jsonData, err := json.MarshalIndent(configData, "", "    ")
	if err != nil {
		return fmt.Errorf("error marshaling config data: %v", err)
	}

	// Zapisz plik konfiguracyjny
	if err := os.WriteFile(configPath, jsonData, 0600); err != nil {
		return fmt.Errorf("error saving config file: %v", err)
	}

	// Usuń tylko oryginalne pliki kluczy (bez .old)
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return fmt.Errorf("error reading keys directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".old" {
			continue // Nie usuwaj plików backupów
		}
		keyPath := filepath.Join(keysDir, entry.Name())
		logDebug := func(message string) {
			// Zakładam, że masz dostęp do funkcji logowania, podobnie jak w BackupKeys
			logFile, err := os.OpenFile("backup_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Printf("Cannot open log file: %v\n", err)
				return
			}
			defer logFile.Close()
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			fmt.Fprintf(logFile, "[%s] %s\n", timestamp, message)
		}

		logDebug(fmt.Sprintf("Removing key file: %s", keyPath))
		if err := os.Remove(keyPath); err != nil {
			logDebug(fmt.Sprintf("Error removing key file %s: %v", keyPath, err))
			return fmt.Errorf("error removing key file %s: %v", entry.Name(), err)
		}
	}

	// Odtwórz katalog kluczy (nie usuwaj plików backupów)
	// Upewnij się, że katalog istnieje
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("error creating keys directory: %v", err)
	}

	// Zapisz nowe klucze
	for _, key := range data.Keys {
		keyData, ok := key.(map[string]interface{})
		if !ok {
			continue
		}

		description, ok := keyData["description"].(string)
		if !ok || description == "" {
			continue
		}

		// Sprawdź czy klucz ma dane do zapisania
		keyContent, exists := keyData["key_data"].(string)
		if !exists || keyContent == "" {
			continue
		}

		// Odszyfruj zawartość klucza przed zapisem do pliku
		if cipher != nil {
			decryptedContent, err := cipher.Decrypt(keyContent)
			if err != nil {
				return fmt.Errorf("error decrypting key %s: %v", description, err)
			}
			keyContent = decryptedContent
		}

		keyPath := filepath.Join(keysDir, description+".key")
		if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
			return fmt.Errorf("error saving key file %s: %v", keyPath, err)
		}
	}

	return nil
}

// RestoreFromBackup przywraca pliki z kopii zapasowych
func RestoreFromBackup(configPath, keysDir string) error {
	// Przywróć plik konfiguracyjny
	backupConfigPath := configPath + ".old"
	if _, err := os.Stat(backupConfigPath); err == nil {
		if err := os.Rename(backupConfigPath, configPath); err != nil {
			return fmt.Errorf("error restoring config from backup: %v", err)
		}
	}

	// Przywróć klucze
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return fmt.Errorf("error reading keys directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".old" {
			oldPath := filepath.Join(keysDir, entry.Name())
			newPath := oldPath[:len(oldPath)-4] // usuń '.old'
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("error restoring key %s from backup: %v", entry.Name(), err)
			}
		}
	}

	return nil
}

// PushToAPI wysyła dane do API
func PushToAPI(apiKey string, configPath, keysDir string) error {
	// Wczytaj dane z lokalnej konfiguracji
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	var localData struct {
		Hosts     []interface{} `json:"hosts"`
		Passwords []interface{} `json:"passwords"`
		Keys      []interface{} `json:"keys"`
	}

	if err := json.Unmarshal(configData, &localData); err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}

	// Przygotuj dane do wysłania
	payload := struct {
		Data struct {
			Hosts     []interface{} `json:"hosts"`
			Passwords []interface{} `json:"passwords"`
			Keys      []interface{} `json:"keys"`
		} `json:"data"`
	}{
		Data: localData,
	}

	// Konwertuj do JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error preparing data: %v", err)
	}

	// Wyślij do API
	client := &http.Client{}
	req, err := http.NewRequest("POST", ApiBaseURL+"/sync", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error status %d: %s", resp.StatusCode, body)
	}

	return nil
}

// RestoreAndSync przywraca dane z backupu i synchronizuje je z API
func RestoreAndSync(configPath, keysDir string, apiKey string, program *tea.Program) error {
	// Najpierw przywróć z backupu
	if err := RestoreFromBackup(configPath, keysDir); err != nil {
		return fmt.Errorf("error restoring from backup: %v", err)
	}

	// Wypchnij przywrócone dane do API
	if err := PushToAPI(apiKey, configPath, keysDir); err != nil {
		return fmt.Errorf("error pushing restored data to API: %v", err)
	}

	// Wyślij komendę do przeładowania aplikacji
	program.Send(messages.ReloadAppMsg{})

	return nil
}
