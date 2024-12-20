// internal/ssh/ssh_client.go

package ssh

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sshManager/internal/config"
	"sshManager/internal/models"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHClient struct {
	currentHost *models.Host
	passwords   []models.Password
	session     *SSHSession
}

const (
	knownHostsFileName = "known_hosts"
)

// getAppKnownHostsPath zwraca ścieżkę do naszego pliku known_hosts
func getAppKnownHostsPath() (string, error) {
	configDir, err := config.GetDefaultConfigPath()
	if err != nil {
		return "", fmt.Errorf("could not get config directory: %v", err)
	}

	sshDir := filepath.Join(filepath.Dir(configDir), "ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", fmt.Errorf("could not create ssh directory: %v", err)
	}

	return filepath.Join(sshDir, knownHostsFileName), nil
}

// internal/ssh/ssh_client.go

// checkKnownHost sprawdza czy klucz hosta jest znany
func checkKnownHost(host *models.Host) error {
	knownHostsPath, err := getAppKnownHostsPath()
	if err != nil {
		return err
	}

	// Sprawdź czy plik istnieje
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return &HostKeyVerificationRequired{IP: host.IP, Port: host.Port}
	}

	// Format hosta do sprawdzenia - używamy dokładnej nazwy hosta bez hasha
	hostToCheck := fmt.Sprintf("[%s]:%s", host.IP, host.Port)

	// Wczytaj zawartość pliku
	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return err
	}

	// Sprawdź każdą linię
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.Contains(line, hostToCheck) {
			return nil // Znaleziono hosta
		}
	}

	return &HostKeyVerificationRequired{IP: host.IP, Port: host.Port}
}

func saveHostKey(host *models.Host) error {
	knownHostsPath, err := getAppKnownHostsPath()
	if err != nil {
		return err
	}

	if !checkSSHKeyscanAvailable() {
		return fmt.Errorf("ssh-keyscan not found - please install OpenSSH")
	}

	scanCmd := fmt.Sprintf("ssh-keyscan -p %s %s 2>/dev/null", host.Port, host.IP)
	cmd := exec.Command("sh", "-c", scanCmd)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", scanCmd)
	}

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to scan host key: %v", err)
	}

	existingContent := ""
	if _, err := os.Stat(knownHostsPath); err == nil {
		content, err := os.ReadFile(knownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to read known_hosts: %v", err)
		}
		existingContent = string(content)
	}

	hostFormat := fmt.Sprintf("[%s]:%s", host.IP, host.Port)

	var newKeys []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			newKeys = append(newKeys, fmt.Sprintf("%s %s %s", hostFormat, parts[1], parts[2]))
		}
	}

	var finalKeys []string
	if existingContent != "" {
		scanner := bufio.NewScanner(strings.NewReader(existingContent))
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if !strings.Contains(line, hostFormat) {
				finalKeys = append(finalKeys, line)
			}
		}
	}

	finalKeys = append(finalKeys, newKeys...)

	content := strings.Join(finalKeys, "\n") + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write known_hosts: %v", err)
	}

	return nil
}

func NewSSHClient(passwords []models.Password) *SSHClient {
	return &SSHClient{
		passwords: passwords,
		// Usuwamy isNative - zawsze będziemy używać natywnej implementacji
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

func GetHostKeyFingerprint(host *models.Host) (string, error) {
	var result string
	// Konfiguracja do tymczasowego połączenia
	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			result = ssh.FingerprintSHA256(key)
			return nil
		},
	}

	// Próba połączenia - potrzebujemy tylko handshake'a żeby pobrać klucz
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil && result != "" {
		// Jeśli mamy fingerprint, ignorujemy błąd auth
		return result, nil
	} else if err != nil {
		return "", fmt.Errorf("failed to get host key: %v", err)
	}
	defer conn.Close()

	if result == "" {
		return "", fmt.Errorf("no host key received")
	}

	return result, nil
}

func (s *SSHClient) Connect(host *models.Host, authData string) error {
	// Sprawdzenie klucza hosta z użyciem naszego mechanizmu
	if err := checkKnownHost(host); err != nil {
		return err
	}

	// Tworzymy kontekst z timeoutem
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Kanał do komunikacji błędów
	errChan := make(chan error, 1)

	go func() {
		// Konfiguracja autoryzacji
		var authMethod ssh.AuthMethod
		if host.PasswordID < 0 {
			key, err := os.ReadFile(authData)
			if err != nil {
				errChan <- fmt.Errorf("failed to read SSH key: %v", err)
				return
			}
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				errChan <- fmt.Errorf("failed to parse SSH key: %v", err)
				return
			}
			authMethod = ssh.PublicKeys(signer)
		} else {
			authMethod = ssh.Password(authData)
		}

		// Pobranie ścieżki do known_hosts
		knownHostsPath, err := getAppKnownHostsPath()
		if err != nil {
			errChan <- fmt.Errorf("failed to get known_hosts path: %v", err)
			return
		}

		// Utworzenie callbacka weryfikującego klucz hosta
		hostKeyCallback, err := knownhosts.New(knownHostsPath)
		if err != nil {
			errChan <- fmt.Errorf("failed to create hostKeyCallback: %v", err)
			return
		}

		// Konfiguracja klienta SSH
		config := &ssh.ClientConfig{
			User:            host.Login,
			Auth:            []ssh.AuthMethod{authMethod},
			HostKeyCallback: hostKeyCallback,
			Timeout:         10 * time.Second,
		}

		// Nawiązanie połączenia
		client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
		if err != nil {
			errChan <- fmt.Errorf("failed to connect: %v", err)
			return
		}

		// Utworzenie sesji SSH
		session, err := NewSSHSession(client)
		if err != nil {
			client.Close()
			errChan <- fmt.Errorf("failed to create session: %v", err)
			return
		}

		s.session = session
		s.currentHost = host
		errChan <- nil
	}()

	// Oczekiwanie na wynik lub timeout
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("connection failed: %v", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("connection timeout: host is unreachable")
	}
}

func (s *SSHClient) ConnectWithAcceptedKey(host *models.Host, authData string) error {
	// Zapisujemy nowy klucz hosta do known_hosts
	if err := saveHostKey(host); err != nil {
		return fmt.Errorf("failed to save host key: %v", err)
	}
	// Po zapisaniu klucza host jest teraz znany,
	// więc ponowna próba połączenia powinna się udać
	return s.Connect(host, authData)
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

func (c *SSHClient) Session() *SSHSession {
	return c.session
}

func checkSSHKeyscanAvailable() bool {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("where", "ssh-keyscan")
		if err := cmd.Run(); err != nil {
			return false
		}
	}
	return true
}
