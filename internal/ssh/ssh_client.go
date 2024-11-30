// internal/ssh/ssh_client.go

package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sshManager/internal/models"
)

type SSHClient struct {
	currentHost *models.Host
	passwords   []models.Password
}

func (s *SSHClient) GetPasswords() []models.Password {
	return s.passwords
}

func NewSSHClient(passwords []models.Password) *SSHClient {
	return &SSHClient{
		passwords: passwords,
	}
}

func GetHostKeyFingerprint(host *models.Host) (string, error) {
	checkCmd := fmt.Sprintf("ssh-keyscan -H -p %s %s 2>/dev/null | ssh-keygen -lf -",
		host.Port, host.IP)

	cmd := exec.Command("sh", "-c", checkCmd)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("nie można pobrać odcisku klucza: %v", err)
	}
	return string(output), nil
}

func CreateSSHCommand(host *models.Host, authData string, acceptKey bool) (*exec.Cmd, error) {
	knownHostsPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	searchCmd := fmt.Sprintf("ssh-keygen -F '[%s]:%s' -f '%s'",
		host.IP, host.Port, knownHostsPath)

	cmd := exec.Command("sh", "-c", searchCmd)
	_, err := cmd.Output()
	isKnown := err == nil

	if !isKnown && !acceptKey {
		return nil, &HostKeyVerificationRequired{
			IP:   host.IP,
			Port: host.Port,
		}
	}

	if !isKnown && acceptKey {
		if err := addHostKey(host); err != nil {
			return nil, fmt.Errorf("nie można dodać klucza hosta: %v", err)
		}
	}

	// Sprawdzamy czy używamy klucza czy hasła na podstawie prefiksu ID
	isKey := host.PasswordID < 0

	var sshCommand string
	if isKey {
		sshCommand = fmt.Sprintf(
			"clear; ssh -i '%s' %s@%s -p %s; clear",
			authData, host.Login, host.IP, host.Port,
		)
	} else {
		sshCommand = fmt.Sprintf(
			"clear; sshpass -p '%s' ssh %s@%s -p %s; clear",
			authData, host.Login, host.IP, host.Port,
		)
	}

	cmd = exec.Command("sh", "-c", sshCommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

// Nowy typ błędu
type HostKeyVerificationRequired struct {
	IP   string
	Port string
}

func (e *HostKeyVerificationRequired) Error() string {
	return "wymagana weryfikacja klucza hosta"
}

func (s *SSHClient) Connect(host *models.Host, authData string) error {
	cmd, err := CreateSSHCommand(host, authData, false)
	if err != nil {
		if _, ok := err.(*HostKeyVerificationRequired); ok {
			// Przekazujemy ten błąd wyżej, aby UI mógł go obsłużyć
			return err
		}
		return fmt.Errorf("błąd tworzenia komendy SSH: %v", err)
	}

	if cmd == nil {
		return fmt.Errorf("nie utworzono komendy SSH")
	}

	s.currentHost = host
	return cmd.Run()
}

// Dodajmy też pomocniczą metodę do ponowienia połączenia po zaakceptowaniu klucza
func (s *SSHClient) ConnectWithAcceptedKey(host *models.Host, authData string) error {
	cmd, err := CreateSSHCommand(host, authData, true)
	if err != nil {
		return fmt.Errorf("błąd tworzenia komendy SSH z zaakceptowanym kluczem: %v", err)
	}

	if cmd == nil {
		return fmt.Errorf("nie utworzono komendy SSH")
	}

	s.currentHost = host
	return cmd.Run()
}

func (s *SSHClient) IsConnected() bool {
	return s.currentHost != nil
}

func (s *SSHClient) Disconnect() {
	s.currentHost = nil
}

func (s *SSHClient) GetCurrentHost() *models.Host {
	return s.currentHost
}

func addHostKey(host *models.Host) error {
	// Pobierz klucz hosta
	scanCmd := fmt.Sprintf("ssh-keyscan -H -p %s %s 2>/dev/null", host.Port, host.IP)
	cmd := exec.Command("sh", "-c", scanCmd)
	hostKey, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("nie można pobrać klucza hosta: %v", err)
	}

	// Dodaj klucz do known_hosts
	knownHostsPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	err = os.MkdirAll(filepath.Dir(knownHostsPath), 0700)
	if err != nil {
		return fmt.Errorf("nie można utworzyć katalogu .ssh: %v", err)
	}

	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("nie można otworzyć pliku known_hosts: %v", err)
	}
	defer f.Close()

	_, err = f.Write(hostKey)
	if err != nil {
		return fmt.Errorf("nie można zapisać klucza hosta: %v", err)
	}

	return nil
}
