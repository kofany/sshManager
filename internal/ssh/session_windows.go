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

	"github.com/moby/term"
	"golang.org/x/crypto/ssh"
	terminal "golang.org/x/term"
)

func init() {
	if runtime.GOOS == "windows" {
		// Spróbuj załadować ConPTY
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		if proc := kernel32.NewProc("CreatePseudoConsole"); proc != nil {
			// ConPTY jest dostępne, możemy użyć lepszej emulacji terminala
			os.Setenv("TERM", "xterm-256color")
		}
	}
}

// SessionState reprezentuje stan sesji SSH
type SessionState int

const (
	StateDisconnected SessionState = iota
	StateConnecting
	StateConnected
	StateError
)

type SSHSession struct {
	client     *ssh.Client
	session    *ssh.Session
	state      SessionState
	lastError  error
	stdin      *os.File
	stdout     *os.File
	stderr     *os.File
	termWidth  int
	termHeight int

	keepAlive  time.Duration
	stopChan   chan struct{}
	stateMutex sync.RWMutex

	oldState *term.State // stan lokalnego terminala (Windows)
}

// NewSSHSession tworzy nową sesję SSH
func NewSSHSession(client *ssh.Client) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	// Pobieramy rozmiar lokalnego terminala (Windows) przez moby/term
	fd := int(os.Stdout.Fd())
	width, height, err := terminal.GetSize(fd)

	return &SSHSession{
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
	}, nil
}

// ConfigureTerminal – żądamy zdalnego PTY i ustawiamy tryb surowy (ECHO=0, ICANON=0)
// aby strzałki i mysza mogły w pełni działać w aplikacjach curses.
func (s *SSHSession) ConfigureTerminal(termType string) error {
	if runtime.GOOS == "windows" {
		termType = "xterm-256color"
	}

	// Sprawdź czy mamy terminal
	fd := os.Stdin.Fd()
	if !term.IsTerminal(fd) {
		return fmt.Errorf("not a terminal")
	}

	// Pobierz rozmiar terminala
	size, _ := term.GetWinsize(fd)
	s.termWidth = int(size.Width)
	s.termHeight = int(size.Height)

	// Pełna konfiguracja trybów terminala
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 34400,
		ssh.TTY_OP_OSPEED: 34400,
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

// StartShell – uruchamia zdalną powłokę (shell) w trybie surowym
func (s *SSHSession) StartShell() error {
	s.session.Stdin = s.stdin
	s.session.Stdout = s.stdout
	s.session.Stderr = s.stderr

	// Uruchamiamy obsługę sygnałów (SIGINT, SIGTERM, resize)
	go s.handleSignals()

	// Opcjonalny keepalive
	if s.keepAlive > 0 {
		go s.keepAliveLoop()
	}

	// Lokalne przejście w tryb raw (Windows)
	oldState, err := term.SetRawTerminal(s.stdin.Fd())
	if err != nil {
		return fmt.Errorf("failed to set local raw terminal: %v", err)
	}
	s.oldState = oldState

	// (Opcjonalnie) alternatywny bufor ekranu (jeśli potrzebny):
	// fmt.Fprint(s.stdout, "\x1b[?1049h")

	// Start zdalnego shell-a
	if err := s.session.Shell(); err != nil {
		// Jeśli się nie uda, natychmiast przywracamy stan terminala
		_ = term.RestoreTerminal(s.stdin.Fd(), oldState)
		return fmt.Errorf("failed to start shell: %v", err)
	}

	s.setState(StateConnected)

	// Czekamy, aż zdalna powłoka się zakończy
	if err := s.session.Wait(); err != nil {
		errStr := err.Error()
		if errStr != "Process exited with status 1" &&
			!strings.Contains(errStr, "exit status") &&
			!strings.Contains(errStr, "signal: terminated") &&
			!strings.Contains(errStr, "signal: interrupt") {
			return fmt.Errorf("session ended with error: %v", err)
		}
	}

	// Normalne wyjście
	s.cleanup()
	return nil
}

// cleanup – przywrócenie stanu terminala (Windows) i ewentualnie wyjście z alternatywnego bufora
func (s *SSHSession) cleanup() {
	close(s.stopChan)
	s.setState(StateDisconnected)

	// Przywracamy terminal lokalny
	if s.oldState != nil {
		_ = term.RestoreTerminal(s.stdin.Fd(), s.oldState)
		s.oldState = nil
	}

	// Wyjście z alternatywnego bufora (jeśli włączony):
	// fmt.Fprint(s.stdout, "\x1b[?1049l")

	// Można łagodnie wyczyścić ekran:
	// fmt.Fprint(s.stdout, "\x1b[2J\x1b[H")

	time.Sleep(50 * time.Millisecond)
}

// handleSignals – obsługa sygnałów i zmiany rozmiaru
func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	go func() {
		resizeTicker := time.NewTicker(100 * time.Millisecond) // Szybsze sprawdzanie zmian
		defer resizeTicker.Stop()

		fd := os.Stdin.Fd()
		if !term.IsTerminal(fd) {
			return
		}

		var lastWidth, lastHeight = s.termWidth, s.termHeight
		var resizeDebouncer *time.Timer

		for {
			select {
			case <-resizeTicker.C:
				if size, err := term.GetWinsize(fd); err == nil {
					width, height := int(size.Width), int(size.Height)
					if width != lastWidth || height != lastHeight {
						if resizeDebouncer != nil {
							resizeDebouncer.Stop()
						}

						// Debouncing zmiany rozmiaru
						resizeDebouncer = time.AfterFunc(50*time.Millisecond, func() {
							s.stateMutex.Lock()
							defer s.stateMutex.Unlock()

							if err := s.session.WindowChange(height, width); err == nil {
								s.termWidth = width
								s.termHeight = height
								lastWidth, lastHeight = width, height
							}
						})
					}
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

// Close zamyka sesję
func (s *SSHSession) Close() error {
	select {
	case <-s.stopChan:
	default:
		close(s.stopChan)
	}

	var errs []string

	if s.session != nil {
		if err := s.session.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("session close error: %v", err))
		}
		s.session = nil
	}

	if s.client != nil {
		if err := s.client.Close(); err != nil {
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

// clearTerminalBuffer – jeśli potrzebujesz wyczyścić ekran przed wyjściem
func (s *SSHSession) clearTerminalBuffer() {
	seq := []string{
		"\x1b[?25l", // Ukryj kursor
		"\x1b[2J",   // Wyczyść cały ekran
		"\x1b[H",    // Przesuń kursor na początek
		"\x1b[?25h", // Pokaż kursor
	}
	for _, esc := range seq {
		fmt.Fprint(s.stdout, esc)
		time.Sleep(10 * time.Millisecond)
	}
}
