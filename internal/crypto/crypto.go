// internal/crypto/crypto.go

package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	keySize   = 32
	nonceSize = 24
)

type Cipher struct {
	key [keySize]byte
}

// NewCipher tworzy nowy obiekt szyfru z podanego hasła
func NewCipher(password string) *Cipher {
	// Generujemy klucz z hasła używając SHA-256
	h := sha256.New()
	h.Write([]byte(password))
	var key [keySize]byte
	copy(key[:], h.Sum(nil))

	return &Cipher{key: key}
}

// Encrypt szyfruje dane
func (c *Cipher) Encrypt(data string) (string, error) {
	// Generujemy nonce
	var nonce [nonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}

	// Szyfrujemy
	encrypted := secretbox.Seal(nonce[:], []byte(data), &nonce, &c.key)

	// Kodujemy do base64
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// Decrypt deszyfruje dane
func (c *Cipher) Decrypt(encryptedStr string) (string, error) {
	// Dekodujemy z base64
	encrypted, err := base64.StdEncoding.DecodeString(encryptedStr)
	if err != nil {
		return "", err
	}

	// Sprawdzamy czy dane są wystarczająco długie
	if len(encrypted) < nonceSize {
		return "", errors.New("encrypted data too short")
	}

	// Wyodrębniamy nonce
	var nonce [nonceSize]byte
	copy(nonce[:], encrypted[:nonceSize])

	// Deszyfrujemy
	decrypted, ok := secretbox.Open(nil, encrypted[nonceSize:], &nonce, &c.key)
	if !ok {
		return "", errors.New("decryption failed")
	}

	return string(decrypted), nil
}

// ValidateKey sprawdza czy klucz jest poprawny próbując odszyfrować przykładowe dane
func ValidateKey(cipher *Cipher, testData string) bool {
	encrypted, err := cipher.Encrypt("test")
	if err != nil {
		return false
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		return false
	}

	return decrypted == "test"
}

func GenerateKeyFromPassword(password string) []byte {
	// Dopełnij hasło do 32 bajtów
	paddedPass := make([]byte, 32)
	copy(paddedPass, []byte(password))

	// Zakoduj using base64
	return []byte(base64.URLEncoding.EncodeToString(paddedPass)[:32])
}
