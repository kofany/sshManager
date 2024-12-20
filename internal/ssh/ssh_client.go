package ssh

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
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

type HostKeyVerificationRequired struct {
	IP          string
	Port        string
	Fingerprint string
	PublicKey   ssh.PublicKey
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

func saveHostKey(host *models.Host, publicKey ssh.PublicKey) error {
	knownHostsPath, err := getAppKnownHostsPath()
	if err != nil {
		return err
	}

	// Format hosta
	hostStr := fmt.Sprintf("[%s]:%s", host.IP, host.Port)

	// Format linii w known_hosts
	line := knownhosts.Line([]string{hostStr}, publicKey)

	// Wczytaj istniejące wpisy
	existingContent := ""
	if _, err := os.Stat(knownHostsPath); err == nil {
		content, err := os.ReadFile(knownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to read known_hosts: %v", err)
		}
		existingContent = string(content)
	}

	// Usuń stare wpisy dla tego hosta
	var finalLines []string
	scanner := bufio.NewScanner(strings.NewReader(existingContent))
	for scanner.Scan() {
		lineText := scanner.Text()
		if !strings.Contains(lineText, hostStr) {
			finalLines = append(finalLines, lineText)
		}
	}

	// Dodaj nowy wpis
	finalLines = append(finalLines, line)

	// Zapisz plik
	content := strings.Join(finalLines, "\n") + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write known_hosts file: %v", err)
	}

	return nil
}

func NewSSHClient(passwords []models.Password) *SSHClient {
	return &SSHClient{
		passwords: passwords,
		// Usuwamy isNative - zawsze będziemy używać natywnej implementacji
	}
}

func (e *HostKeyVerificationRequired) Error() string {
	return "host key verification required"
}

// internal/ssh/ssh_client.go

func GetHostKeyFingerprint(host *models.Host) (string, error) {
	var result string

	// Konfiguracja do tymczasowego połączenia
	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			result = ssh.FingerprintSHA256(key)
			return nil
		},
		User: host.Login,
		Auth: []ssh.AuthMethod{
			ssh.Password("dummy"), // Używamy dowolnego hasła, bo interesuje nas tylko handshake
		},
		Timeout: 10 * time.Second,
	}

	// Próba połączenia - potrzebujemy tylko handshake'a
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil {
		// Jeśli mamy fingerprint, ignorujemy błąd auth
		if result != "" {
			return result, nil
		}
		return "", fmt.Errorf("failed to get host key: %v", err)
	}
	if conn != nil {
		conn.Close()
	}

	if result == "" {
		return "", fmt.Errorf("no host key received")
	}

	return result, nil
}

// Zmodyfikowana funkcja Connect
func (s *SSHClient) Connect(host *models.Host, authData string) error {
	// Konfiguracja autoryzacji
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

	// Pobranie ścieżki do known_hosts
	knownHostsPath, err := getAppKnownHostsPath()
	if err != nil {
		return fmt.Errorf("failed to get known_hosts path: %v", err)
	}

	var fingerprint string

	// Konfiguracja klienta SSH z callbackiem zbierającym klucz hosta
	config := &ssh.ClientConfig{
		User: host.Login,
		Auth: []ssh.AuthMethod{authMethod},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fingerprint = ssh.FingerprintSHA256(key)

			// Sprawdź czy klucz jest już znany
			knownHostsCallback, err := knownhosts.New(knownHostsPath)
			if err == nil {
				err = knownHostsCallback(hostname, remote, key)
				if err == nil {
					return nil // Klucz jest znany i poprawny
				}
			}

			// Klucz nie jest znany - zwróć błąd weryfikacji
			return &HostKeyVerificationRequired{
				IP:          host.IP,
				Port:        host.Port,
				Fingerprint: fingerprint,
				PublicKey:   key,
			}
		},
		Timeout: 10 * time.Second,
	}

	// Próba połączenia
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil {
		if verificationErr, ok := err.(*HostKeyVerificationRequired); ok {
			return verificationErr
		}
		return fmt.Errorf("failed to connect: %v", err)
	}

	// Utworzenie sesji SSH
	session, err := NewSSHSession(client)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to create session: %v", err)
	}

	s.session = session
	s.currentHost = host
	return nil
}

func (s *SSHClient) ConnectWithAcceptedKey(host *models.Host, authData string) error {
	// Najpierw próbujemy połączenia, aby uzyskać klucz publiczny
	err := s.Connect(host, authData)
	if verificationErr, ok := err.(*HostKeyVerificationRequired); ok {
		// Zapisujemy nowy klucz hosta do known_hosts
		if err := saveHostKey(host, verificationErr.PublicKey); err != nil {
			return fmt.Errorf("failed to save host key: %v", err)
		}
		// Ponowna próba połączenia
		return s.Connect(host, authData)
	}
	return err
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
