// internal/crypto/crypto.go
//
// This package provides cryptographic functionalities for the SSH Manager application.
// It handles encryption and decryption of sensitive data using AES-256-GCM.
// The package ensures secure handling of keys and encrypted data storage.

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
	// KEY_SIZE defines the size of the encryption key in bytes.
	// 32 bytes are used for AES-256 encryption.
	KEY_SIZE = 32 // 32 bytes for AES-256
)

// Cipher represents an AES-256-GCM cipher with a specific key.
type Cipher struct {
	key []byte // Encryption key used for AES-256-GCM
}

// Data represents the structure for storing encrypted data.
// It includes the ciphertext and the nonce used during encryption.
type Data struct {
	CipherText string // Hex-encoded ciphertext
	Nonce      string // Hex-encoded nonce
}

// NewCipher creates a new Cipher instance using the provided password.
// It ensures the password is exactly KEY_SIZE bytes long by padding or truncating.
func NewCipher(password string) *Cipher {
	// Check if the password meets the required key size.
	if len(password) < KEY_SIZE {
		// If the password is too short, pad it with zeros to reach KEY_SIZE.
		key := make([]byte, KEY_SIZE)
		copy(key, []byte(password))
		return &Cipher{key: key}
	}
	// If the password is long enough, truncate it to KEY_SIZE.
	return &Cipher{key: []byte(password)[:KEY_SIZE]}
}

// Encrypt encrypts the given plaintext using AES-256-GCM.
// It returns the encrypted data as a hex-encoded string.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	// Create a new AES cipher block using the key.
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Wrap the cipher block in Galois/Counter Mode (GCM) for authenticated encryption.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Generate a random nonce of the appropriate size.
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Encrypt the plaintext using Seal, which appends the ciphertext to the nonce.
	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)

	// Combine the nonce and ciphertext for storage or transmission.
	combined := make([]byte, len(nonce)+len(ciphertext))
	copy(combined, nonce)
	copy(combined[len(nonce):], ciphertext)

	// Encode the combined nonce and ciphertext to a hex string for easy handling.
	return hex.EncodeToString(combined), nil
}

// Decrypt decrypts the given hex-encoded ciphertext using AES-256-GCM.
// It returns the decrypted plaintext as a string.
func (c *Cipher) Decrypt(encryptedHex string) (string, error) {
	// Decode the hex-encoded ciphertext.
	combined, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex: %v", err)
	}

	// Create a new AES cipher block using the key.
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Wrap the cipher block in Galois/Counter Mode (GCM) for authenticated decryption.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %v", err)
	}

	// Retrieve the nonce size from the GCM.
	nonceSize := aesGCM.NonceSize()
	if len(combined) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract the nonce and ciphertext from the combined data.
	nonce := combined[:nonceSize]
	ciphertext := combined[nonceSize:]

	// Decrypt the ciphertext using Open, which verifies the authentication tag.
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %v", err)
	}

	return string(plaintext), nil
}

// GenerateKeyFromPassword generates a 32-byte key from the given password.
// It is a helper function to maintain backward compatibility.
func GenerateKeyFromPassword(password string) []byte {
	cipher := NewCipher(password)
	return cipher.key
}
