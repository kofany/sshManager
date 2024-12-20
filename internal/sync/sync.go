package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sshManager/internal/crypto"
	"sshManager/internal/models"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	ApiBaseURL = "https://sshm.io/api/v1/"
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

func BackupKeys(keysDir string) error {
	// Sprawdź czy katalog kluczy istnieje
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return fmt.Errorf("error reading keys directory: %v", err)
	}

	// Najpierw usuń stare backupy (.old)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".old" {
			oldPath := filepath.Join(keysDir, entry.Name())
			if err := os.Remove(oldPath); err != nil {
				return fmt.Errorf("error removing old backup %s: %v", entry.Name(), err)
			}
		}
	}

	// Teraz twórz nowe backupy
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) == ".old" {
			continue
		}

		originalPath := filepath.Join(keysDir, entry.Name())
		backupPath := originalPath + ".old"

		content, err := os.ReadFile(originalPath)
		if err != nil {
			return fmt.Errorf("error reading key file %s: %v", entry.Name(), err)
		}

		if err := os.WriteFile(backupPath, content, 0600); err != nil {
			return fmt.Errorf("error creating key backup %s: %v", entry.Name(), err)
		}
	}

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
	fmt.Printf("Starting SaveAPIData - config: %s, keys dir: %s\n", configPath, keysDir)

	// Przygotuj strukturę danych do lokalnego zapisu
	config := struct {
		Hosts     []models.Host     `json:"hosts"`
		Passwords []models.Password `json:"passwords"`
		Keys      []models.Key      `json:"keys"`
	}{
		Hosts:     make([]models.Host, 0),
		Passwords: make([]models.Password, 0),
		Keys:      make([]models.Key, 0),
	}

	// Przetwarzanie hostów
	for _, h := range data.Hosts {
		hostMap, ok := h.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid host data format")
		}

		// Odszyfrowanie danych
		name, err := cipher.Decrypt(getStringValue(hostMap, "name"))
		if err != nil {
			return fmt.Errorf("failed to decrypt name: %v", err)
		}

		description, err := cipher.Decrypt(getStringValue(hostMap, "description"))
		if err != nil {
			return fmt.Errorf("failed to decrypt description: %v", err)
		}

		login, err := cipher.Decrypt(getStringValue(hostMap, "login"))
		if err != nil {
			return fmt.Errorf("failed to decrypt login: %v", err)
		}

		ip, err := cipher.Decrypt(getStringValue(hostMap, "ip"))
		if err != nil {
			return fmt.Errorf("failed to decrypt ip: %v", err)
		}

		port, err := cipher.Decrypt(getStringValue(hostMap, "port"))
		if err != nil {
			return fmt.Errorf("failed to decrypt port: %v", err)
		}

		// Tworzenie obiektu hosta z odszyfrowanymi danymi
		host := models.Host{
			Name:        name,
			Description: description,
			Login:       login,
			IP:          ip,
			Port:        port,
			PasswordID:  getIntValue(hostMap, "password_id"),
		}
		config.Hosts = append(config.Hosts, host)
	}

	// Przetwarzanie haseł
	for _, p := range data.Passwords {
		passMap, ok := p.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid password data format")
		}

		pass := models.Password{
			Description: getStringValue(passMap, "description"),
			Password:    getStringValue(passMap, "password"),
		}
		config.Passwords = append(config.Passwords, pass)
	}

	// Konwersja kluczy z API do lokalnej struktury
	for _, k := range data.Keys {
		keyMap, ok := k.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid key data format")
		}

		key := models.Key{
			Description: getStringValue(keyMap, "description"),
			Path:        getStringValue(keyMap, "path"),
			KeyData:     getStringValue(keyMap, "key_data"),
		}
		config.Keys = append(config.Keys, key)
	}

	// Zapisz konfigurację
	jsonData, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("error marshaling config data: %v", err)
	}

	if err := os.WriteFile(configPath, jsonData, 0600); err != nil {
		return fmt.Errorf("error saving config file: %v", err)
	}

	// Upewnij się, że katalog kluczy istnieje
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("error creating keys directory: %v", err)
	}

	// Usuń istniejące pliki kluczy
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return fmt.Errorf("error reading keys directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) == ".old" {
			continue
		}
		keyPath := filepath.Join(keysDir, entry.Name())
		if err := os.Remove(keyPath); err != nil {
			return fmt.Errorf("error removing key file %s: %v", entry.Name(), err)
		}
	}

	// Zapisz nowe klucze
	for _, key := range config.Keys {
		if key.KeyData == "" || key.Description == "" {
			continue
		}

		// Sanityzacja nazwy pliku
		safeDesc := sanitizeFilename(key.Description)
		keyContent := key.KeyData

		if cipher != nil {
			decrypted, err := cipher.Decrypt(keyContent)
			if err != nil {
				return fmt.Errorf("error decrypting key %s: %v", key.Description, err)
			}
			keyContent = decrypted
		}

		// Normalizacja końców linii
		if runtime.GOOS == "windows" {
			keyContent = strings.ReplaceAll(keyContent, "\n", "\r\n")
		}
		keyContent = strings.TrimSpace(keyContent) + "\n"

		// Konstruowanie bezpiecznej ścieżki
		keyPath := filepath.Clean(filepath.Join(keysDir, safeDesc+".key"))
		if !strings.HasPrefix(keyPath, filepath.Clean(keysDir)) {
			return fmt.Errorf("invalid key path detected: %s", keyPath)
		}

		fmt.Printf("Saving key: %s to %s\n", safeDesc, keyPath)

		// Próba zapisu z powtórzeniami
		var writeErr error
		for attempts := 0; attempts < 3; attempts++ {
			if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
				writeErr = err
				time.Sleep(100 * time.Millisecond)
				continue
			}
			writeErr = nil
			break
		}

		if writeErr != nil {
			return fmt.Errorf("error saving key file %s after multiple attempts: %v", keyPath, writeErr)
		}

		// Weryfikacja zapisanego pliku
		content, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("error verifying key file %s: %v", keyPath, err)
		}
		normalizedContent := strings.ReplaceAll(string(content), "\r\n", "\n")
		normalizedKeyContent := strings.ReplaceAll(keyContent, "\r\n", "\n")
		if normalizedContent != normalizedKeyContent {
			return fmt.Errorf("key file verification failed for %s: content mismatch", keyPath)
		}
	}

	fmt.Println("SaveAPIData completed successfully")
	return nil
}

// Funkcja pomocnicza do sanityzacji nazw plików
func sanitizeFilename(filename string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, filename)
}

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

// Uproszczona funkcja do pobierania wartości string z mapy
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case *string:
			if v != nil {
				return *v
			}
		case nil:
			return ""
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

// Uproszczona funkcja do pobierania wartości int z mapy
func getIntValue(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		case nil:
			return 0
		}
	}
	return 0
}

func PushToAPI(apiKey string, configPath, keysDir string, cipher *crypto.Cipher) error {
	// Odczytaj plik konfiguracyjny
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	// Struktura do przechowywania danych lokalnych
	var localData struct {
		Hosts     []models.Host     `json:"hosts"`
		Passwords []models.Password `json:"passwords"`
		Keys      []models.Key      `json:"keys"`
	}

	// Parsowanie danych konfiguracyjnych
	if err := json.Unmarshal(configData, &localData); err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}

	// Struktura do wysłania do API
	payload := struct {
		Data struct {
			Hosts     []map[string]interface{} `json:"hosts"`
			Passwords []map[string]interface{} `json:"passwords"`
			Keys      []map[string]interface{} `json:"keys"`
		} `json:"data"`
	}{}

	// Przygotowanie hostów do wysyłki
	for _, host := range localData.Hosts {
		// Szyfrowanie wrażliwych danych
		encryptedName, err := cipher.Encrypt(host.Name)
		if err != nil {
			return fmt.Errorf("error encrypting name: %v", err)
		}

		encryptedDescription, err := cipher.Encrypt(host.Description)
		if err != nil {
			return fmt.Errorf("error encrypting description: %v", err)
		}

		encryptedLogin, err := cipher.Encrypt(host.Login)
		if err != nil {
			return fmt.Errorf("error encrypting login: %v", err)
		}

		encryptedIP, err := cipher.Encrypt(host.IP)
		if err != nil {
			return fmt.Errorf("error encrypting IP: %v", err)
		}

		encryptedPort, err := cipher.Encrypt(host.Port)
		if err != nil {
			return fmt.Errorf("error encrypting port: %v", err)
		}

		// Przygotowanie mapy z zaszyfrowanymi danymi
		hostData := map[string]interface{}{
			"name":          encryptedName,
			"description":   encryptedDescription,
			"login":         encryptedLogin,
			"ip":            encryptedIP,
			"port":          encryptedPort,
			"password_id":   host.PasswordID,
			"terminal_type": host.TerminalType,
			"keep_alive":    host.KeepAlive,
			"compression":   host.Compression,
		}
		payload.Data.Hosts = append(payload.Data.Hosts, hostData)
	}

	// Przygotowanie haseł do wysyłki
	for _, pass := range localData.Passwords {
		passData := map[string]interface{}{
			"description": pass.Description,
			"password":    pass.Password,
		}
		payload.Data.Passwords = append(payload.Data.Passwords, passData)
	}

	// Przygotowanie kluczy do wysyłki
	for _, key := range localData.Keys {
		keyData := map[string]interface{}{
			"description": key.Description,
			"key_data":    key.KeyData,
			"path":        key.Path,
		}
		payload.Data.Keys = append(payload.Data.Keys, keyData)
	}

	// Konwersja na JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error preparing data for API: %v", err)
	}

	// Przygotowanie i wykonanie requestu HTTP
	client := &http.Client{}
	req, err := http.NewRequest("POST", ApiBaseURL+"sync", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Wykonanie zapytania
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Sprawdzenie odpowiedzi
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error status %d: %s", resp.StatusCode, body)
	}

	return nil
}
