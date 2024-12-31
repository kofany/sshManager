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
	RawKey      []byte // Dodane - surowe dane klucza
	KeyType     string // Dodane - typ klucza
}

const (
	knownHostsFileName = "known_hosts"
	KeyAlgoRSA         = "ssh-rsa"
	KeyAlgoRSASHA2256  = "rsa-sha2-256"
	KeyAlgoRSASHA2512  = "rsa-sha2-512"
	KeyAlgoECDSA256    = "ecdsa-sha2-nistp256"
	KeyAlgoECDSA384    = "ecdsa-sha2-nistp384"
	KeyAlgoECDSA521    = "ecdsa-sha2-nistp521"
	KeyAlgoED25519     = "ssh-ed25519"
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

	// Upewnij się, że katalog istnieje
	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Format hosta
	hostPatterns := []string{
		fmt.Sprintf("[%s]:%s", host.IP, host.Port),
		host.IP,
	}

	// Generuj linię w known_hosts
	line := knownhosts.Line(hostPatterns, publicKey)

	// Jeśli plik nie istnieje, po prostu zapisz nową linię
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return os.WriteFile(knownHostsPath, []byte(line+"\n"), 0600)
	}

	// Wczytaj istniejącą zawartość
	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to read known_hosts: %v", err)
	}

	// Usuń stare wpisy dla tego hosta
	var finalLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		lineText := scanner.Text()
		shouldKeep := true
		for _, pattern := range hostPatterns {
			if strings.Contains(lineText, pattern) {
				shouldKeep = false
				break
			}
		}
		if shouldKeep {
			finalLines = append(finalLines, lineText)
		}
	}

	// Dodaj nowy wpis
	finalLines = append(finalLines, line)

	// Zapisz plik
	content = []byte(strings.Join(finalLines, "\n") + "\n")
	return os.WriteFile(knownHostsPath, content, 0600)
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
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			result = ssh.FingerprintSHA256(key)
			return nil
		},
		User:    host.Login,
		Auth:    []ssh.AuthMethod{},
		Timeout: 2 * time.Second,
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil && result != "" {
		return result, nil
	}
	if conn != nil {
		conn.Close()
	}

	if result == "" {
		return "", fmt.Errorf("no host key received")
	}

	return result, nil
}

func (s *SSHClient) Connect(host *models.Host, authData string) error {
	// Konfiguracja autoryzacji
	var authMethod ssh.AuthMethod
	if host.PasswordID < 0 {
		// Obsługa klucza SSH
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
		// Obsługa hasła
		authMethod = ssh.Password(authData)
	}

	// Pobranie ścieżki do known_hosts
	knownHostsPath, err := getAppKnownHostsPath()
	if err != nil {
		return fmt.Errorf("failed to get known_hosts path: %v", err)
	}

	var verificationRequired *HostKeyVerificationRequired

	config := &ssh.ClientConfig{
		User: host.Login,
		Auth: []ssh.AuthMethod{authMethod},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Próbuj standardowej weryfikacji najpierw
			hostKeyCallback, err := knownhosts.New(knownHostsPath)
			if err == nil {
				err = hostKeyCallback(hostname, remote, key)
				if err == nil {
					return nil // Klucz jest znany i poprawny
				}
			}

			// Jeśli klucz nie jest znany lub wystąpił błąd, zapisz informacje do weryfikacji
			verificationRequired = &HostKeyVerificationRequired{
				IP:          host.IP,
				Port:        host.Port,
				Fingerprint: ssh.FingerprintSHA256(key),
				PublicKey:   key,
				RawKey:      key.Marshal(),
				KeyType:     key.Type(),
			}
			return verificationRequired
		},
		Timeout: 3 * time.Second,
		// Kompletna lista obsługiwanych algorytmów
		HostKeyAlgorithms: []string{
			KeyAlgoECDSA256,
			KeyAlgoECDSA384,
			KeyAlgoECDSA521,
			KeyAlgoED25519,
			KeyAlgoRSA,
			KeyAlgoRSASHA2256,
			KeyAlgoRSASHA2512,
		},
		// Dodajemy konfigurację cipherów i KEX
		Config: ssh.Config{
			Ciphers: []string{
				"aes128-gcm@openssh.com",
				"aes256-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"aes128-ctr",
				"aes192-ctr",
				"aes256-ctr",
			},
			KeyExchanges: []string{
				"curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256",
				"diffie-hellman-group16-sha512",
			},
		},
	}

	// Próba nawiązania połączenia
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)
	if err != nil {
		// Jeśli wymagana jest weryfikacja klucza hosta
		if verificationRequired != nil {
			return verificationRequired
		}

		// Szczegółowa diagnostyka błędów
		switch {
		case strings.Contains(err.Error(), "no common algorithm"):
			return fmt.Errorf("SSH handshake failed: no compatible algorithms found.\n"+
				"Server offered different algorithms than what we support.\n"+
				"Original error: %v", err)
		case strings.Contains(err.Error(), "connection refused"):
			return fmt.Errorf("connection refused: the SSH server is not accepting connections on %s:%s", host.IP, host.Port)
		case strings.Contains(err.Error(), "i/o timeout"):
			return fmt.Errorf("connection timed out: could not reach %s:%s within %v", host.IP, host.Port, config.Timeout)
		case strings.Contains(err.Error(), "unable to authenticate"):
			return fmt.Errorf("authentication failed: invalid credentials for user %s", host.Login)
		case strings.Contains(err.Error(), "handshake failed"):
			return fmt.Errorf("SSH handshake failed: %v\nPlease check if the server supports modern SSH protocols", err)
		default:
			return fmt.Errorf("failed to establish SSH connection: %v", err)
		}
	}

	// Utworzenie nowej sesji
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
