package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

const (
	KEY_SIZE = 32 // 32 bytes for AES-256
)

type Cipher struct {
	key []byte
}

// Data struktura do przechowywania zaszyfrowanych danych
type Data struct {
	CipherText string
	Nonce      string
}

// NewCipher tworzy nowy obiekt szyfru z podanym hasłem
func NewCipher(password string) *Cipher {
	// Sprawdzamy czy hasło ma odpowiednią długość
	if len(password) < KEY_SIZE {
		// Rozszerzamy hasło do 32 bajtów jeśli za krótkie
		key := make([]byte, KEY_SIZE)
		copy(key, []byte(password))
		return &Cipher{key: key}
	}
	// Jeśli hasło ma 32 lub więcej bajtów, bierzemy pierwsze 32
	return &Cipher{key: []byte(password)[:KEY_SIZE]}
}

// Encrypt szyfruje tekst używając AES-256-GCM
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	// Tworzymy block cipher
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Tworzymy GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Generujemy nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Szyfrujemy
	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)

	// Łączymy nonce + ciphertext i konwertujemy do hex
	combined := make([]byte, len(nonce)+len(ciphertext))
	copy(combined, nonce)
	copy(combined[len(nonce):], ciphertext)

	return hex.EncodeToString(combined), nil
}

// Decrypt deszyfruje tekst używając AES-256-GCM
func (c *Cipher) Decrypt(encryptedHex string) (string, error) {
	// Dekodujemy z hex
	combined, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex: %v", err)
	}

	// Tworzymy block cipher
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Tworzymy GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Sprawdzamy minimalną długość
	nonceSize := aesGCM.NonceSize()
	if len(combined) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Wyodrębniamy nonce i ciphertext
	nonce := combined[:nonceSize]
	ciphertext := combined[nonceSize:]

	// Deszyfrujemy
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %v", err)
	}

	return string(plaintext), nil
}

// Helper function dla zachowania kompatybilności wstecznej
func GenerateKeyFromPassword(password string) []byte {
	cipher := NewCipher(password)
	return cipher.key
}
