// internal/ssh/ssh_client.go
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
	"golang.org/x/term"
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

	// Użyj jawnie filepath.Join dla poprawnej obsługi ścieżek
	sshDir := filepath.Join(filepath.Dir(configDir), "ssh")
	knownHostsPath := filepath.Join(sshDir, knownHostsFileName)

	// Wydrukuj ścieżkę dla celów diagnostycznych
	fmt.Printf("Known hosts path: %s\n", knownHostsPath)

	return knownHostsPath, nil
}

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

// HostKeyVerificationRequired reprezentuje błąd weryfikacji klucza hosta
type HostKeyVerificationRequired struct {
	IP   string
	Port string
}

func (e *HostKeyVerificationRequired) Error() string {
	return "host key verification required"
}

// fetchAndSaveHostKey łączy się z hostem w celu pobrania klucza hosta bez użycia ssh-keyscan.
// Następnie zapisuje go do pliku known_hosts.
func fetchAndSaveHostKey(host *models.Host) error {
	knownHostsPath, err := getAppKnownHostsPath()
	if err != nil {
		return fmt.Errorf("failed to get known_hosts path: %v", err)
	}

	// Utworzenie katalogu dla known_hosts
	knownHostsDir := filepath.Dir(knownHostsPath)
	if err := os.MkdirAll(knownHostsDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", knownHostsDir, err)
	}

	// Kanał do przechwycenia klucza hosta
	hostKeyChan := make(chan ssh.PublicKey, 1)

	// Ustawiamy HostKeyCallback, który zapisze klucz do kanału.
	hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		hostKeyChan <- key
		return nil
	}

	// Próba połączenia bez metod autoryzacji (aby wymusić jedynie uzyskanie klucza)
	config := &ssh.ClientConfig{
		User:            host.Login,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	// Ignorujemy błąd, ponieważ oczekujemy błędu autoryzacji
	ssh.Dial("tcp", fmt.Sprintf("%s:%s", host.IP, host.Port), config)

	// Oczekujemy tu błędu autoryzacji, ale klucz hosta powinien być już przechwycony.
	// Jeśli połączenie się uda (co jest mało prawdopodobne bez autoryzacji), też mamy klucz.
	close(hostKeyChan)
	var hostKey ssh.PublicKey
	select {
	case hostKey = <-hostKeyChan:
		// Mamy klucz
	default:
		// Nie udało się pobrać klucza
		return fmt.Errorf("could not retrieve host key from server")
	}

	// Teraz zapisujemy klucz do known_hosts
	hostFormat := fmt.Sprintf("[%s]:%s", host.IP, host.Port)
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostKey)))
	newKeyLine := fmt.Sprintf("%s %s", hostFormat, authorizedKey)

	// Wczytanie istniejących kluczy i usunięcie poprzednich wpisów dla tego hosta
	var existingKeys []string
	if content, err := os.ReadFile(knownHostsPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if !strings.Contains(line, hostFormat) {
				existingKeys = append(existingKeys, line)
			}
		}
	}

	// Dodajemy nowy klucz
	allKeys := append(existingKeys, newKeyLine)
	content := strings.Join(allKeys, "\n") + "\n"

	if err := os.WriteFile(knownHostsPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write known_hosts file %s: %v", knownHostsPath, err)
	}

	return nil
}

func NewSSHClient(passwords []models.Password) *SSHClient {
	return &SSHClient{
		passwords: passwords,
	}
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
	// Teraz używamy naszej funkcji, aby pobrać i zapisać klucz hosta
	if err := fetchAndSaveHostKey(host); err != nil {
		return fmt.Errorf("failed to fetch and save host key: %v", err)
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
		// Przed zamknięciem sesji upewnij się, że terminal jest przywrócony
		if s.session.GetOriginalTermState() != nil {
			if err := term.Restore(int(os.Stdin.Fd()), s.session.GetOriginalTermState()); err != nil {
				fmt.Fprintf(os.Stderr, "failed to restore terminal state: %v\n", err)
			}
		}
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
