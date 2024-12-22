// internal/error/error.go
//
// This package provides a structured approach to error handling within the SSH Manager application.
// It defines custom error types, categorizes errors, and implements the error interface for consistent error reporting.

package error

import "fmt"

// AppError represents a custom application error.
// It includes the type of error, a descriptive message, and an underlying error if applicable.
type AppError struct {
	Type    ErrorType // The category of the error.
	Message string    // A descriptive message explaining the error.
	Err     error     // The underlying error that caused this error, if any.
}

// ErrorType defines the category of an application error.
// It helps in identifying the nature of the error for better error handling and reporting.
type ErrorType int

const (
	// ConfigError indicates an error related to configuration issues.
	ConfigError ErrorType = iota

	// ConnectionError indicates an error related to connection failures.
	ConnectionError

	// CryptoError indicates an error related to cryptographic operations.
	CryptoError

	// FileError indicates an error related to file operations.
	FileError

	// ValidationError indicates an error related to data validation.
	ValidationError
)

// Error implements the error interface for AppError.
// It returns the error message, including the underlying error if present.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// New creates a new instance of AppError.
// It takes the error type, a descriptive message, and an underlying error as parameters.
// This function standardizes error creation across the application.
func New(errType ErrorType, message string, err error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}
