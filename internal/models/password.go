// internal/models/password.go

package models

import (
	"errors"
	"sshManager/internal/crypto"
)

type Password struct {
	Description string `json:"description"`
	Password    string `json:"password"` // zaszyfrowane hasło
}

// NewPassword tworzy nową instancję Password
func NewPassword(description string, plainPassword string, cipher *crypto.Cipher) (*Password, error) {
	if description == "" {
		return nil, errors.New("description cannot be empty")
	}
	if plainPassword == "" {
		return nil, errors.New("password cannot be empty")
	}

	// Szyfrowanie hasła
	encryptedPass, err := cipher.Encrypt(plainPassword)
	if err != nil {
		return nil, err
	}

	return &Password{
		Description: description,
		Password:    encryptedPass,
	}, nil
}

// Validate sprawdza poprawność danych Password
func (p *Password) Validate() error {
	if p.Description == "" {
		return errors.New("description cannot be empty")
	}
	if p.Password == "" {
		return errors.New("password cannot be empty")
	}
	return nil
}

// GetDecrypted zwraca odszyfrowane hasło
func (p *Password) GetDecrypted(cipher *crypto.Cipher) (string, error) {
	return cipher.Decrypt(p.Password)
}

// UpdatePassword aktualizuje zaszyfrowane hasło
func (p *Password) UpdatePassword(newPlainPassword string, cipher *crypto.Cipher) error {
	if newPlainPassword == "" {
		return errors.New("new password cannot be empty")
	}

	encryptedPass, err := cipher.Encrypt(newPlainPassword)
	if err != nil {
		return err
	}

	p.Password = encryptedPass
	return nil
}

// UpdateDescription aktualizuje opis hasła
func (p *Password) UpdateDescription(newDescription string) error {
	if newDescription == "" {
		return errors.New("new description cannot be empty")
	}
	p.Description = newDescription
	return nil
}

// Clone tworzy kopię hasła
func (p *Password) Clone() *Password {
	return &Password{
		Description: p.Description,
		Password:    p.Password,
	}
}
