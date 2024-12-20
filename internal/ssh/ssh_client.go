package ssh

import (
	"bufio"
	"context"
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
	IP        string
	Port      string
	PublicKey ssh.PublicKey // Dodane pole
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

// internal/ssh/ssh_client.go

func saveHostKey(host *models.Host, hostKey ssh.PublicKey) error {
	knownHostsPath, err := getAppKnownHostsPath()
	if err != nil {
		return err
	}

	// Format zapisu hosta
	hostFormat := fmt.Sprintf("[%s]:%s", host.IP, host.Port)

	// Przygotuj linię z kluczem w formacie known_hosts
	keyLine := knownhosts.Line([]string{hostFormat}, hostKey)

	// Wczytaj istniejące klucze
	var existingContent string
	if _, err := os.Stat(knownHostsPath); err == nil {
		content, err := os.ReadFile(knownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to read known_hosts: %v", err)
		}
		existingContent = string(content)
	}

	// Usuń stare wpisy dla tego hosta
	var finalLines []string
	if existingContent != "" {
		scanner := bufio.NewScanner(strings.NewReader(existingContent))
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if !strings.Contains(line, hostFormat) {
				finalLines = append(finalLines, line)
			}
		}
	}

	// Dodaj nowy klucz
	finalLines = append(finalLines, keyLine)

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

	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			result = ssh.FingerprintSHA256(key)
			return nil
		},
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil && result != "" {
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
	// Tworzymy tymczasową konfigurację do pobrania klucza hosta
	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Zapisz klucz hosta
			if err := saveHostKey(host, key); err != nil {
				return fmt.Errorf("failed to save host key: %v", err)
			}
			return nil
		},
		Timeout: 10 * time.Second,
	}

	// Wykonaj próbne połączenie aby pobrać klucz
	tmpConn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil {
		if !strings.Contains(err.Error(), "ssh: must specify HostKeyCallback") {
			return fmt.Errorf("failed to get host key: %v", err)
		}
	}
	if tmpConn != nil {
		tmpConn.Close()
	}

	// Teraz wykonaj właściwe połączenie
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
