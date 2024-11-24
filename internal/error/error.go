// internal/error/error.go

package error

import "fmt"

type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

type ErrorType int

const (
	ConfigError ErrorType = iota
	ConnectionError
	CryptoError
	FileError
	ValidationError
)

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func New(errType ErrorType, message string, err error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}
