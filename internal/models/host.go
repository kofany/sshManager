// internal/models/host.go

package models

type Host struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Login        string `json:"login"`
	IP           string `json:"ip"`
	Port         string `json:"port"`
	PasswordID   int    `json:"password_id"`
	TerminalType string `json:"terminal_type"`
	KeepAlive    bool   `json:"keep_alive"`
	Compression  bool   `json:"compression"`
}

type Config struct {
	Hosts     []Host     `json:"hosts"`
	Passwords []Password `json:"passwords"`
	Keys      []Key      `json:"keys"`
}
