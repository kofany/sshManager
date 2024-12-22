// internal/models/host.go
//
// Package models defines the core data structures for the SSH Manager application.

package models

// Host represents the configuration details of an SSH host.
type Host struct {
	Name         string `json:"name"`          // Unique identifier for the host
	Description  string `json:"description"`   // Description of the host
	Login        string `json:"login"`         // Username for SSH authentication
	IP           string `json:"ip"`            // IP address or hostname of the SSH server
	Port         string `json:"port"`          // SSH server port
	PasswordID   int    `json:"password_id"`   // Reference to the associated password
	TerminalType string `json:"terminal_type"` // Type of terminal to emulate (e.g., xterm)
	KeepAlive    bool   `json:"keep_alive"`    // Enable keep-alive messages
	Compression  bool   `json:"compression"`   // Enable compression for the SSH connection
}

// Config holds the application's configuration, including hosts, passwords, and keys.
type Config struct {
	Hosts     []Host     `json:"hosts"`     // List of SSH hosts
	Passwords []Password `json:"passwords"` // List of passwords
	Keys      []Key      `json:"keys"`      // List of SSH keys
}
