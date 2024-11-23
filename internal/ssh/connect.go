// internal/ssh/connect.go

package ssh

import (
	"fmt"
	"io"
	"net"
	"sshManager/internal/models"

	"golang.org/x/crypto/ssh"
)

// Connection reprezentuje połączenie SSH
type Connection struct {
	client  *ssh.Client
	session *ssh.Session
}

// ConnectionConfig zawiera konfigurację połączenia
type ConnectionConfig struct {
	Host     *models.Host
	Password string
}

// NewConnection tworzy nowe połączenie SSH
func NewConnection(config *ConnectionConfig) (*Connection, error) {
	if config.Host == nil {
		return nil, fmt.Errorf("host configuration is required")
	}

	// Konfiguracja klienta SSH
	sshConfig := &ssh.ClientConfig{
		User: config.Host.Login,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.Password),
		},
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// TODO: W przyszłości można dodać weryfikację known_hosts
			return nil
		}),
	}

	// Adres do połączenia
	addr := fmt.Sprintf("%s:%s", config.Host.IP, config.Host.Port)

	// Nawiązanie połączenia
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}

	return &Connection{
		client: client,
	}, nil
}

// Close zamyka połączenie
func (c *Connection) Close() error {
	if c.session != nil {
		c.session.Close()
	}
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// StartShell rozpoczyna interaktywną sesję SSH
func (c *Connection) StartShell(stdin io.Reader, stdout, stderr io.Writer) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	c.session = session

	// Ustawienie wejścia/wyjścia
	c.session.Stdin = stdin
	c.session.Stdout = stdout
	c.session.Stderr = stderr

	// Ustawienie trybu terminala
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Żądanie pseudoterminala
	if err := c.session.RequestPty("xterm", 40, 80, modes); err != nil {
		return fmt.Errorf("failed to request pty: %v", err)
	}

	// Uruchomienie powłoki
	if err := c.session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %v", err)
	}

	// Oczekiwanie na zakończenie sesji
	return c.session.Wait()
}

// ExecuteCommand wykonuje pojedyncze polecenie
func (c *Connection) ExecuteCommand(command string) ([]byte, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	return session.CombinedOutput(command)
}

// IsConnected sprawdza czy połączenie jest aktywne
func (c *Connection) IsConnected() bool {
	if c.client == nil {
		return false
	}

	// Próba wykonania prostego polecenia
	_, err := c.ExecuteCommand("echo 1")
	return err == nil
}

// GetClient zwraca klienta SSH (potrzebne dla operacji transferu plików)
func (c *Connection) GetClient() *ssh.Client {
	return c.client
}
