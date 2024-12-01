// internal/ssh/ssh_client.go

package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sshManager/internal/models"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	currentHost *models.Host
	passwords   []models.Password
	session     *SSHSession
	isNative    bool // true dla golang/crypto/ssh, false dla external command
}

func NewSSHClient(passwords []models.Password) *SSHClient {
	return &SSHClient{
		passwords: passwords,
		isNative:  runtime.GOOS != "windows", // Używamy natywnego SSH tylko na non-Windows
	}
}

// HostKeyVerificationRequired reprezentuje błąd weryfikacji klucza hosta
type HostKeyVerificationRequired struct {
	IP   string
	Port string
}

func (e *HostKeyVerificationRequired) Error() string {
	return "host key verification required"
}

// Zachowujemy kompatybilność z UI
func CreateSSHCommand(host *models.Host, authData string, acceptKey bool) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		return createWindowsCommand(host, authData)
	}
	return createUnixCommand(host, authData, acceptKey)
}

func createWindowsCommand(host *models.Host, authData string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	if host.PasswordID < 0 {
		// Używamy klucza SSH
		cmd = exec.Command("powershell",
			fmt.Sprintf("ssh -i '%s' %s@%s -p %s",
				authData, host.Login, host.IP, host.Port))
	} else {
		// Używamy hasła
		cmd = exec.Command("powershell",
			fmt.Sprintf("ssh %s@%s -p %s",
				host.Login, host.IP, host.Port))
		cmd.Env = append(os.Environ(), fmt.Sprintf("SSHPASS=%s", authData))
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func createUnixCommand(host *models.Host, authData string, acceptKey bool) (*exec.Cmd, error) {
	knownHostsPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	if !acceptKey {
		// Sprawdź czy host jest już znany
		searchCmd := fmt.Sprintf("ssh-keygen -F '[%s]:%s' -f '%s'",
			host.IP, host.Port, knownHostsPath)
		cmd := exec.Command("sh", "-c", searchCmd)
		_, err := cmd.Output()
		if err != nil {
			return nil, &HostKeyVerificationRequired{IP: host.IP, Port: host.Port}
		}
	}

	var sshCommand string
	if host.PasswordID < 0 {
		sshCommand = fmt.Sprintf(
			"ssh -i '%s' %s@%s -p %s",
			authData, host.Login, host.IP, host.Port,
		)
	} else {
		sshCommand = fmt.Sprintf(
			"sshpass -p '%s' ssh %s@%s -p %s",
			authData, host.Login, host.IP, host.Port,
		)
	}

	cmd := exec.Command("sh", "-c", sshCommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func GetHostKeyFingerprint(host *models.Host) (string, error) {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell",
			fmt.Sprintf("ssh-keyscan -H -p %s %s 2>$null | ssh-keygen -lf -",
				host.Port, host.IP))
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get host key fingerprint: %v", err)
		}
		return string(output), nil
	}

	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("ssh-keyscan -H -p %s %s 2>/dev/null | ssh-keygen -lf -",
			host.Port, host.IP))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get host key fingerprint: %v", err)
	}
	return string(output), nil
}

func (s *SSHClient) Connect(host *models.Host, authData string) error {
	if !s.isNative {
		// Używamy external command dla Windows lub gdy wymuszono
		cmd, err := CreateSSHCommand(host, authData, false)
		if err != nil {
			if _, ok := err.(*HostKeyVerificationRequired); ok {
				return err
			}
			return fmt.Errorf("failed to create SSH command: %v", err)
		}
		s.currentHost = host
		return cmd.Run()
	}

	// Używamy natywnej implementacji SSH
	var authMethod ssh.AuthMethod
	if host.PasswordID < 0 {
		key, err := os.ReadFile(authData)
		if err != nil {
			return fmt.Errorf("failed to read SSH key: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %v", err)
		}
		authMethod = ssh.PublicKeys(signer)
	} else {
		authMethod = ssh.Password(authData)
	}

	config := &ssh.ClientConfig{
		User:            host.Login,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: proper host key verification
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	session, err := NewSSHSession(client)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to create session: %v", err)
	}

	if err := session.ConfigureTerminal("xterm-256color"); err != nil {
		session.Close()
		return fmt.Errorf("failed to configure terminal: %v", err)
	}

	s.session = session
	s.currentHost = host

	// Uruchamiamy sesję w osobnej goroutynie
	go func() {
		if err := session.StartShell(); err != nil {
			fmt.Fprintf(os.Stderr, "Session error: %v\n", err)
		}
	}()

	return nil
}

func (s *SSHClient) ConnectWithAcceptedKey(host *models.Host, authData string) error {
	if !s.isNative {
		cmd, err := CreateSSHCommand(host, authData, true)
		if err != nil {
			return fmt.Errorf("failed to create SSH command: %v", err)
		}
		s.currentHost = host
		return cmd.Run()
	}
	return s.Connect(host, authData) // Dla natywnej implementacji nie ma różnicy
}

func (s *SSHClient) IsConnected() bool {
	if s.session != nil {
		return s.session.GetState() == StateConnected
	}
	return s.currentHost != nil
}

func (s *SSHClient) Disconnect() {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	s.currentHost = nil
}

func (s *SSHClient) GetCurrentHost() *models.Host {
	return s.currentHost
}

func (s *SSHClient) GetPasswords() []models.Password {
	return s.passwords
}
