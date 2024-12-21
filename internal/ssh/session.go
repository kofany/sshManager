// internal/ssh/session.go
//go:build !windows
// +build !windows

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

// ConfigureTerminal konfiguruje terminal dla sesji
func (s *SSHSession) ConfigureTerminal(termType string) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
		ssh.VINTR:         3,  // Ctrl+C
		ssh.VQUIT:         28, // Ctrl+\
		ssh.VERASE:        127,
		ssh.VKILL:         21, // Ctrl+U
		ssh.VEOF:          4,  // Ctrl+D
		ssh.VWERASE:       23, // Ctrl+W
		ssh.VLNEXT:        22, // Ctrl+V
		ssh.VSUSP:         26, // Ctrl+Z
	}

	if err := s.session.RequestPty(termType, s.termHeight, s.termWidth, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %v", err)
	}

	return nil
}

// StartShell uruchamia powłokę interaktywną
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

	// Przejście w tryb raw dla terminala
	rawState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}

	// Upewniamy się, że terminal zostanie przywrócony
	defer func(raw *term.State) {
		// Najpierw resetujemy stan sesji
		s.setState(StateDisconnected)

		// Przywracamy stan terminala
		if err := term.Restore(int(os.Stdin.Fd()), raw); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restore terminal state: %v\n", err)
		}
	}(rawState)

	// Uruchomienie powłoki
	if err := s.session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %v", err)
	}

	s.setState(StateConnected)

	// Czekanie na zakończenie sesji
	if err := s.session.Wait(); err != nil {
		// Ignorujemy typowe kody wyjścia
		errStr := err.Error()
		if errStr != "Process exited with status 1" &&
			!strings.Contains(errStr, "exit status") &&
			!strings.Contains(errStr, "signal: terminated") &&
			!strings.Contains(errStr, "signal: interrupt") {
			return fmt.Errorf("session ended with error: %v", err)
		}
	}

	return nil
}

// handleSignals obsługuje sygnały systemowe
func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	// macOS/Linux obsługuje więcej sygnałów, więc dodajemy SIGWINCH
	signal.Notify(sigChan, syscall.SIGWINCH, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGWINCH:
				if err := s.updateTerminalSize(); err != nil {
					s.setError(fmt.Errorf("failed to update terminal size: %v", err))
				}
			case syscall.SIGTERM, syscall.SIGINT:
				s.Close()
				return
			}
		case <-s.stopChan:
			return
		}
	}
}

func (s *SSHSession) updateTerminalSize() error {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("failed to get terminal size: %v", err)
	}

	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	// Sprawdź czy rozmiar rzeczywiście się zmienił
	if width == s.termWidth && height == s.termHeight {
		return nil
	}

	if err := s.session.WindowChange(height, width); err != nil {
		return fmt.Errorf("failed to update window size: %v", err)
	}

	s.termWidth = width
	s.termHeight = height

	return nil
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
