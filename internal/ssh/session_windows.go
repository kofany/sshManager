// internal/ssh/session_windows.go
//go:build windows
// +build windows

package ssh

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// SessionState reprezentuje stan sesji SSH
type SessionState int

const (
	StateDisconnected SessionState = iota
	StateConnecting
	StateConnected
	StateError
)

// SSHSession reprezentuje aktywną sesję SSH
type SSHSession struct {
	client            *ssh.Client
	session           *ssh.Session
	state             SessionState
	lastError         error
	stdin             *os.File
	stdout            *os.File
	stderr            *os.File
	termWidth         int
	termHeight        int
	keepAlive         time.Duration
	stopChan          chan struct{}
	stateMutex        sync.RWMutex
	originalTermState *term.State // Dodane pole także dla Windows
}

// NewSSHSession tworzy nową sesję SSH
func NewSSHSession(client *ssh.Client) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	// Pobierz aktualny rozmiar terminala
	fd := int(os.Stdout.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24 // Wartości domyślne
	}

	s := &SSHSession{
		client:     client,
		session:    session,
		state:      StateConnecting,
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		termWidth:  width,
		termHeight: height,
		keepAlive:  30 * time.Second,
		stopChan:   make(chan struct{}),
	}

	return s, nil
}

func (s *SSHSession) ConfigureTerminal(termType string) error {
	// Dla Windows wymuszamy konkretne ustawienia terminala
	if runtime.GOOS == "windows" {
		// Używamy "vt100" zamiast "xterm-256color" dla lepszej kompatybilności z Windows
		termType = "vt100"
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 115200, // Zwiększona prędkość
		ssh.TTY_OP_OSPEED: 115200,
		ssh.VINTR:         3,
		ssh.VQUIT:         28,
		ssh.VERASE:        127,
		ssh.VKILL:         21,
		ssh.VEOF:          4,
		ssh.VWERASE:       23,
		ssh.VLNEXT:        22,
		ssh.VSUSP:         26,
		ssh.OCRNL:         0, // Wyłącz konwersję CR -> NL
		ssh.ONLCR:         0, // Wyłącz konwersję NL -> CR-NL
		ssh.INLCR:         0, // Wyłącz konwersję NL -> CR
		ssh.ICRNL:         0, // Wyłącz konwersję CR -> NL dla wejścia
		ssh.OPOST:         0, // Wyłącz przetwarzanie wyjścia
		ssh.ISIG:          0, // Wyłącz generowanie sygnałów
		ssh.IEXTEN:        0, // Wyłącz rozszerzone przetwarzanie wejścia
		ssh.ECHOE:         0, // Wyłącz echo
		ssh.ICANON:        0, // Wyłącz tryb kanoniczny
	}

	// Ustaw mniejszy rozmiar terminala dla lepszej kontroli nad buforowaniem
	width, height := s.termWidth, s.termHeight
	if width > 160 {
		width = 160 // Ograniczamy szerokość dla lepszej kontroli zawijania
	}

	if err := s.session.RequestPty(termType, height, width, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %v", err)
	}

	return nil
}

func (s *SSHSession) StartShell() error {
	// Konfiguracja strumieni we/wy
	s.session.Stdin = s.stdin
	s.session.Stdout = s.stdout
	s.session.Stderr = s.stderr

	// Zapisujemy oryginalny stan terminala
	var err error
	s.originalTermState, err = term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to get terminal state: %v", err)
	}

	// Uruchomienie obsługi sygnałów
	go s.handleSignals()

	// Uruchomienie keepalive jeśli włączone
	if s.keepAlive > 0 {
		go s.keepAliveLoop()
	}

	// Specjalne ustawienia dla Windows
	rawState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}

	cleanup := func() {
		// Zatrzymujemy keepalive i sygnały
		close(s.stopChan)

		// Resetujemy stan sesji
		s.setState(StateDisconnected)

		// Czyszczenie i przywracanie stanu terminala
		s.clearTerminalBuffer()

		// Dłuższe opóźnienie dla Windows przed przywróceniem stanu
		time.Sleep(200 * time.Millisecond)

		// Przywracamy stan terminala
		if err := term.Restore(int(os.Stdin.Fd()), rawState); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restore terminal state: %v\n", err)
		}

		// Ostateczne czyszczenie bufora
		s.clearTerminalBuffer()

		// Dodatkowe opóźnienie dla Windows po przywróceniu stanu
		time.Sleep(100 * time.Millisecond)
	}
	defer cleanup()

	// Inicjalizacja terminala
	fmt.Fprint(s.stdout, "\x1b[?1049h") // Przełącz na alternatywny bufor
	fmt.Fprint(s.stdout, "\x1b[?25h")   // Pokaż kursor

	// Uruchomienie powłoki
	if err := s.session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %v", err)
	}

	s.setState(StateConnected)

	// Czekanie na zakończenie sesji
	if err := s.session.Wait(); err != nil {
		errStr := err.Error()
		if errStr != "Process exited with status 1" &&
			!strings.Contains(errStr, "exit status") &&
			!strings.Contains(errStr, "signal: terminated") &&
			!strings.Contains(errStr, "signal: interrupt") {
			return fmt.Errorf("session ended with error: %v", err)
		}
	}

	// Dodatkowe opóźnienie przed zakończeniem
	time.Sleep(200 * time.Millisecond)

	return nil
}

// handleSignals obsługuje sygnały systemowe
func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	go func() {
		resizeTicker := time.NewTicker(100 * time.Millisecond)
		defer resizeTicker.Stop()

		var lastWidth, lastHeight = s.termWidth, s.termHeight
		var resizeDebouncer *time.Timer
		var consecutiveErrors int

		for {
			select {
			case <-resizeTicker.C:
				width, height, err := term.GetSize(int(os.Stdout.Fd()))
				if err != nil {
					consecutiveErrors++
					if consecutiveErrors > 5 {
						s.setError(fmt.Errorf("terminal size monitoring error: %v", err))
					}
					continue
				}
				consecutiveErrors = 0

				if width != lastWidth || height != lastHeight {
					if resizeDebouncer != nil {
						resizeDebouncer.Stop()
					}
					resizeDebouncer = time.AfterFunc(50*time.Millisecond, func() {
						s.stateMutex.Lock()
						defer s.stateMutex.Unlock()

						// Używamy nowej funkcji zamiast bezpośredniego wysyłania sekwencji
						s.clearTerminalBuffer()

						if err := s.session.WindowChange(height, width); err != nil {
							s.setError(fmt.Errorf("failed to update window size: %v", err))
						} else {
							s.termWidth = width
							s.termHeight = height
							lastWidth, lastHeight = width, height
						}
					})
				}
			case sig := <-sigChan:
				if sig == syscall.SIGTERM || sig == syscall.SIGINT {
					s.Close()
					return
				}
			case <-s.stopChan:
				return
			}
		}
	}()
}

// keepAliveLoop wysyła pakiety keepalive
func (s *SSHSession) keepAliveLoop() {
	ticker := time.NewTicker(s.keepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				s.setError(fmt.Errorf("keepalive failed: %v", err))
				s.Close()
				return
			}
		case <-s.stopChan:
			return
		}
	}
}

// Close zamyka sesję
func (s *SSHSession) Close() error {
	// Zamknięcie kanału stopChan
	select {
	case <-s.stopChan:
		// Kanał już zamknięty
	default:
		close(s.stopChan)
	}

	var errors []string

	if s.session != nil {
		if err := s.session.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("session close error: %v", err))
		}
		s.session = nil
	}

	if s.client != nil {
		if err := s.client.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("client close error: %v", err))
		}
		s.client = nil
	}

	s.setState(StateDisconnected)

	if len(errors) > 0 {
		return fmt.Errorf("close errors: %s", strings.Join(errors, "; "))
	}
	return nil
}

// setState ustawia stan sesji
func (s *SSHSession) setState(state SessionState) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.state = state
}

// setError ustawia błąd sesji
func (s *SSHSession) setError(err error) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.lastError = err
	s.state = StateError
}

// GetState zwraca aktualny stan sesji
func (s *SSHSession) GetState() SessionState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.state
}

// GetLastError zwraca ostatni błąd
func (s *SSHSession) GetLastError() error {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.lastError
}

// SetKeepAlive ustawia interwał keepalive
func (s *SSHSession) SetKeepAlive(duration time.Duration) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.keepAlive = duration
}

// clearTerminalBuffer czyści bufor terminala w bezpieczny sposób
func (s *SSHSession) clearTerminalBuffer() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	// Sekwencja czyszcząca dla Windows
	sequences := []string{
		"\x1b[?25l", // Ukryj kursor
		"\x1b[2J",   // Wyczyść cały ekran
		"\x1b[H",    // Przesuń kursor na początek
		"\x1b[3J",   // Wyczyść przewinięty bufor
		"\x1b[?25h", // Pokaż kursor
	}

	for _, seq := range sequences {
		fmt.Fprint(s.stdout, seq)
		time.Sleep(10 * time.Millisecond) // Małe opóźnienie między sekwencjami
	}
}
