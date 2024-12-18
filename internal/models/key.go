package models

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sshManager/internal/crypto"
	"strings"
	"unicode"
)

type Key struct {
	Description string `json:"description"`
	Path        string `json:"path,omitempty"`     // Ścieżka do klucza (jeśli używamy zewnętrznego)
	KeyData     string `json:"key_data,omitempty"` // Zawartość klucza (jeśli przechowujemy lokalnie)
	RawKeyData  string `json:"-"`                  // Niezaszyfrowane dane klucza - nie zapisywane do JSON
}

const (
	LocalKeysDir = "keys"
	KeyPrefix    = "K" // Dodane
)

// NewKey tworzy nową instancję Key
// NewKey tworzy nową instancję Key
func NewKey(description string, path string, keyData string, cipher *crypto.Cipher) (*Key, error) {
	if description == "" {
		return nil, errors.New("description cannot be empty")
	}

	// Sprawdzenie czy nie podano jednocześnie path i keyData
	if path != "" && keyData != "" {
		return nil, errors.New("cannot specify both path and key data")
	}

	// Sprawdzenie czy podano przynajmniej jedno: path lub keyData
	if path == "" && keyData == "" {
		return nil, errors.New("either path or key data must be provided")
	}

	key := &Key{
		Description: description,
		Path:        path,
		RawKeyData:  keyData, // Zachowujemy oryginalne dane
	}

	// Jeśli podano dane klucza, szyfrujemy je dla KeyData (do zapisu w konfiguracji/API)
	if keyData != "" {
		encryptedKey, err := cipher.Encrypt(keyData)
		if err != nil {
			return nil, err
		}
		key.KeyData = encryptedKey
	}

	// Od razu zapisz plik klucza jeśli mamy RawKeyData
	if key.RawKeyData != "" {
		if err := key.SaveKeyToFile(); err != nil {
			return nil, fmt.Errorf("failed to save key file: %v", err)
		}
	}

	return key, nil
}

// Validate sprawdza poprawność danych Key
func (k *Key) Validate() error {
	if k.Description == "" {
		return errors.New("description cannot be empty")
	}

	if k.Path == "" && k.KeyData == "" {
		return errors.New("either path or key data must be provided")
	}

	if k.Path != "" && k.KeyData != "" {
		return errors.New("cannot have both path and key data")
	}

	return nil
}

// GetKeyData zwraca odszyfrowane dane klucza
func (k *Key) GetKeyData(cipher *crypto.Cipher) (string, error) {
	if k.KeyData == "" {
		return "", errors.New("no key data stored")
	}
	return cipher.Decrypt(k.KeyData)
}

// IsLocal sprawdza czy klucz jest przechowywany lokalnie
// IsLocal sprawdza czy klucz jest przechowywany lokalnie i zwraca jego ścieżkę
func (k *Key) IsLocal() bool {
	return k.KeyData != ""
}

// GetKeyPath zwraca ścieżkę do klucza
func (k *Key) GetKeyPath() (string, error) {
	if k.Path != "" {
		return k.Path, nil
	}

	// Dla lokalnie przechowywanego klucza
	if k.KeyData != "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %v", err)
		}

		// Tworzymy bezpieczną nazwę pliku z opisu klucza
		safeFileName := strings.Map(func(r rune) rune {
			// Pozostawiamy tylko litery, cyfry i podkreślenia
			if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
				return r
			}
			return '_'
		}, k.Description)

		return filepath.Join(homeDir, ".config", "sshmen", LocalKeysDir, safeFileName+".key"), nil
	}

	return "", errors.New("no key path or data available")
}

// Clone tworzy kopię klucza
func (k *Key) Clone() *Key {
	return &Key{
		Description: k.Description,
		Path:        k.Path,
		KeyData:     k.KeyData,
	}
}

// SaveKeyToFile zapisuje niezaszyfrowaną zawartość klucza do pliku
func (k *Key) SaveKeyToFile() error {
	if k.RawKeyData == "" {
		return errors.New("no raw key data to save")
	}

	keyPath, err := k.GetKeyPath()
	if err != nil {
		return fmt.Errorf("failed to get key path: %v", err)
	}

	// Upewnij się, że katalog istnieje
	keyDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %v", err)
	}

	// Zapisz niezaszyfrowane dane
	if err := os.WriteFile(keyPath, []byte(k.RawKeyData), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %v", err)
	}

	return nil
}
