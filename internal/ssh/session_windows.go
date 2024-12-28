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
	originalTermState *term.State
}

// NewSSHSession tworzy nową sesję SSH
func NewSSHSession(client *ssh.Client) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	fd := int(os.Stdout.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24
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
	// Dla Windows zawsze używamy xterm-256color
	if runtime.GOOS == "windows" {
		termType = "xterm-256color"
	}

	// Typowe ustawienia zdalnego terminala dla interaktywnego shell-a:
	// - ECHO włączone (1), bo chcemy widzieć wpisywany tekst
	// - ICANON włączone (1), czyli tryb kanoniczny
	// - ONLCR = 1 -> konwersja \n -> \r\n
	// - ISIG = 1 -> obsługa sygnałów (Ctrl+C, Ctrl+Z)
	// - IEXTEN = 1 -> rozszerzone przetwarzanie
	// - OPOST = 1 -> podstawowe przetwarzanie wyjścia
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.ICANON:        1,
		ssh.ISIG:          1,
		ssh.IEXTEN:        1,
		ssh.OPOST:         1,
		ssh.ONLCR:         1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}

	fd := int(os.Stdout.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24
	}

	if err := s.session.RequestPty(termType, height, width, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %v", err)
	}

	return nil
}

func (s *SSHSession) StartShell() error {
	s.session.Stdin = s.stdin
	s.session.Stdout = s.stdout
	s.session.Stderr = s.stderr

	// Zapisujemy oryginalny stan terminala (Windows)
	var err error
	s.originalTermState, err = term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to get terminal state: %v", err)
	}

	// Obsługa sygnałów (SIGTERM, SIGINT, zmiana rozmiaru okna)
	go s.handleSignals()

	// Uruchomienie keepalive (opcjonalne)
	if s.keepAlive > 0 {
		go s.keepAliveLoop()
	}

	// Lokalnie ustawiamy raw, żeby specjalne klawisze były
	// przekazywane prosto do zdalnego terminala.
	rawState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}

	// (Opcjonalne) przejście w alternatywny bufor:
	// Jeśli weechat z tym koliduje, możesz to wyłączyć.
	//
	// fmt.Fprint(s.stdout, "\x1b[?1049h") // ALTERNATYWNY bufor
	// fmt.Fprint(s.stdout, "\x1b[?25h")   // Pokaż kursor

	if err := s.session.Shell(); err != nil {
		// Przy błędzie natychmiast przywracamy terminal
		term.Restore(int(os.Stdin.Fd()), rawState)
		return fmt.Errorf("failed to start shell: %v", err)
	}

	s.setState(StateConnected)

	// Czekamy na zakończenie pracy powłoki
	if err := s.session.Wait(); err != nil {
		errStr := err.Error()
		if errStr != "Process exited with status 1" &&
			!strings.Contains(errStr, "exit status") &&
			!strings.Contains(errStr, "signal: terminated") &&
			!strings.Contains(errStr, "signal: interrupt") {
			return fmt.Errorf("session ended with error: %v", err)
		}
	}

	// Zakończenie sesji: przywracamy stan terminala itp.
	s.cleanup(rawState)
	return nil
}

// cleanup – przywrócenie stanu terminala i ewentualne wyjście z bufora alternatywnego
func (s *SSHSession) cleanup(rawState *term.State) {
	close(s.stopChan)
	s.setState(StateDisconnected)

	// Przywracamy stan terminala
	if err := term.Restore(int(os.Stdin.Fd()), rawState); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restore terminal state: %v\n", err)
	}

	// (Opcjonalne) wyjście z alternatywnego bufora:
	// fmt.Fprint(s.stdout, "\x1b[?1049l")

	// Możesz łagodnie wyczyścić ekran tylko przy wyjściu (jeśli potrzebujesz):
	// fmt.Fprint(s.stdout, "\x1b[2J\x1b[H")

	// Niewielkie opóźnienie, aby terminal zdążył przetworzyć sekwencje
	time.Sleep(50 * time.Millisecond)
}

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

						// Nie czyścimy ekranu w tym miejscu!
						// Pozwalamy aplikacji (np. weechat) samodzielnie
						// odświeżyć widok po zmianie rozmiaru.

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

func (s *SSHSession) Close() error {
	// Spróbuj zamknąć kanał stopChan (jeśli jeszcze żyje)
	select {
	case <-s.stopChan:
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

func (s *SSHSession) setState(state SessionState) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.state = state
}

func (s *SSHSession) setError(err error) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.lastError = err
	s.state = StateError
}

func (s *SSHSession) GetState() SessionState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.state
}

func (s *SSHSession) GetLastError() error {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.lastError
}

func (s *SSHSession) SetKeepAlive(duration time.Duration) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.keepAlive = duration
}

// clearTerminalBuffer – jeżeli chcesz użyć, to raczej tylko przy wyjściu z aplikacji
func (s *SSHSession) clearTerminalBuffer() {
	sequences := []string{
		"\x1b[?25l", // Ukryj kursor
		"\x1b[2J",   // Wyczyść cały ekran
		"\x1b[H",    // Kursor na początek
		"\x1b[?25h", // Pokaż kursor
	}

	for _, seq := range sequences {
		fmt.Fprint(s.stdout, seq)
		time.Sleep(10 * time.Millisecond)
	}
}
