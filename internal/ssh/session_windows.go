// internal/ssh/session_windows.go
//go:build windows
// +build windows

package ssh

import (
	"fmt"
	"os"
	"os/signal"
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
	originalTermState *term.State
}

// NewSSHSession tworzy nową sesję SSH
func NewSSHSession(client *ssh.Client) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Pobieramy rozmiar z stdin (częściej rzeczywisty TTY niż stdout)
	fd := int(os.Stdin.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24 // Domyślne w razie błędu
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

// ConfigureTerminal konfiguruje zdalny PTY (np. "xterm-256color" albo "tmux-256color")
func (s *SSHSession) ConfigureTerminal(termType string) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
		// Można dodać inne, np. VINTR, VQUIT, itp. w razie potrzeby
	}

	if err := s.session.RequestPty(termType, s.termHeight, s.termWidth, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}
	return nil
}

// StartShell uruchamia zdalną powłokę (shell) w trybie interaktywnym
func (s *SSHSession) StartShell() error {
	// Przypięcie strumieni
	s.session.Stdin = s.stdin
	s.session.Stdout = s.stdout
	s.session.Stderr = s.stderr

	// Zapisanie oryginalnego stanu terminala lokalnie (Windows)
	var err error
	s.originalTermState, err = term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to get terminal state: %w", err)
	}

	// Obsługa sygnałów (SIGINT, SIGTERM)
	go s.handleSignals()

	// KeepAlive (jeżeli włączone)
	if s.keepAlive > 0 {
		go s.keepAliveLoop()
	}

	// Ustawienie trybu raw lokalnego terminala,
	// co ułatwia poprawne przekazywanie klawiszy specjalnych.
	rawState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %w", err)
	}

	// Funkcja sprzątająca, wywoływana po zakończeniu
	cleanup := func() {
		close(s.stopChan)
		s.setState(StateDisconnected)

		// Przywrócenie pierwotnego stanu terminala
		if restoreErr := term.Restore(int(os.Stdin.Fd()), rawState); restoreErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to restore terminal state: %v\n", restoreErr)
		}
	}
	defer cleanup()

	// Uruchamiamy zdalną powłokę
	if err := s.session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	s.setState(StateConnected)

	// Czekamy na zakończenie (wyjście z powłoki)
	if err := s.session.Wait(); err != nil {
		errStr := err.Error()
		// Jeśli nie jest to "zwykłe" wyjście, logujemy
		if errStr != "Process exited with status 1" &&
			!strings.Contains(errStr, "exit status") &&
			!strings.Contains(errStr, "signal: terminated") &&
			!strings.Contains(errStr, "signal: interrupt") {
			return fmt.Errorf("session ended with error: %w", err)
		}
	}

	return nil
}

// handleSignals obsługuje podstawowe sygnały: SIGINT, SIGTERM
func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	for {
		select {
		case sig := <-sigChan:
			if sig == syscall.SIGTERM || sig == syscall.SIGINT {
				fmt.Fprintf(os.Stdout, "\nReceived signal %v, closing session\n", sig)
				s.Close()
				return
			}
		case <-s.stopChan:
			return
		}
	}
}

// keepAliveLoop wysyła pakiety keepalive, by zapobiec rozłączeniom
func (s *SSHSession) keepAliveLoop() {
	ticker := time.NewTicker(s.keepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				s.setError(fmt.Errorf("keepalive failed: %w", err))
				s.Close()
				return
			}
		case <-s.stopChan:
			return
		}
	}
}

// Close zamyka kanały i klienta SSH
func (s *SSHSession) Close() error {
	// Zamknięcie stopChan (jeśli jeszcze nie był)
	select {
	case <-s.stopChan:
	default:
		close(s.stopChan)
	}

	var errs []string

	if s.session != nil {
		if err := s.session.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			errs = append(errs, fmt.Sprintf("session close error: %v", err))
		}
		s.session = nil
	}
	if s.client != nil {
		if err := s.client.Close(); err != nil && !strings.Contains(err.Error(), "EOF") {
			errs = append(errs, fmt.Sprintf("client close error: %v", err))
		}
		s.client = nil
	}

	s.setState(StateDisconnected)

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// setState ustawia stan sesji w sposób bezpieczny współbieżnie
func (s *SSHSession) setState(state SessionState) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.state = state
}

// setError ustawia ostatni błąd sesji i przechodzi w stan StateError
func (s *SSHSession) setError(err error) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.lastError = err
	s.state = StateError
}

// GetState zwraca aktualny stan sesji (wątkowo-bezpiecznie)
func (s *SSHSession) GetState() SessionState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.state
}

// GetLastError zwraca ostatni błąd (wątkowo-bezpiecznie)
func (s *SSHSession) GetLastError() error {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.lastError
}

// SetKeepAlive zmienia interwał keepalive
func (s *SSHSession) SetKeepAlive(duration time.Duration) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.keepAlive = duration
}
