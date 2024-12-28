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

	// Tutaj będziemy przechowywać stan konsoli zwracany przez moby/term
	oldState *term.State
}

// NewSSHSession tworzy nową sesję SSH
func NewSSHSession(client *ssh.Client) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	// Pobierz bieżące wymiary konsoli przez moby/term
	ws, err := term.GetWinsize(os.Stdout.Fd())
	width, height := 80, 24 // wartości domyślne, jeśli się nie uda pobrać
	if err == nil {
		width = int(ws.Width)
		height = int(ws.Height)
		if width < 1 {
			width = 80
		}
		if height < 1 {
			height = 24
		}
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

// ConfigureTerminal ustawia parametry zdalnego PTY (zdalnej powłoki)
func (s *SSHSession) ConfigureTerminal(termType string) error {
	// Na Windows zwykle używamy xterm-256color (lub cokolwiek innego, co wspiera 256 kolorów)
	if runtime.GOOS == "windows" {
		termType = "xterm-256color"
	}

	// Typowe ustawienia dla zdalnego terminala, jeśli chcemy widzieć wpisywany tekst:
	modes := ssh.TerminalModes{
		ssh.ECHO:          1, // włączone echo po stronie serwera
		ssh.ICANON:        1, // włączony tryb kanoniczny
		ssh.ISIG:          1, // obsługa sygnałów (Ctrl+C, itp.)
		ssh.IEXTEN:        1, // rozszerzone przetwarzanie
		ssh.OPOST:         1, // podstawowe przetwarzanie wyjścia
		ssh.ONLCR:         1, // konwersja \n na \r\n
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}

	// Pobierz aktualne wymiary
	ws, err := term.GetWinsize(os.Stdout.Fd())
	width, height := s.termWidth, s.termHeight
	if err == nil {
		w := int(ws.Width)
		h := int(ws.Height)
		if w > 0 {
			width = w
		}
		if h > 0 {
			height = h
		}
	}

	if err := s.session.RequestPty(termType, height, width, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %v", err)
	}

	return nil
}

// StartShell uruchamia powłokę zdalną w bieżącej sesji
func (s *SSHSession) StartShell() error {
	s.session.Stdin = s.stdin
	s.session.Stdout = s.stdout
	s.session.Stderr = s.stderr

	// Obsługa sygnałów (SIGINT, SIGTERM, zmiana rozmiaru okna)
	go s.handleSignals()

	// Uruchomienie keepalive, jeśli ustawione
	if s.keepAlive > 0 {
		go s.keepAliveLoop()
	}

	// Ustawiamy lokalnie tryb "raw" przez moby/term
	oldState, err := term.SetRawTerminal(s.stdin.Fd())
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}
	// Zapisujemy stary stan, żeby przywrócić go w cleanup
	s.oldState = oldState

	// (Opcjonalnie) alternatywny bufor ekranu:
	// fmt.Fprint(s.stdout, "\x1b[?1049h")
	// fmt.Fprint(s.stdout, "\x1b[?25h") // pokaż kursor

	if err := s.session.Shell(); err != nil {
		// Jeśli Shell() się nie uruchomi, natychmiast przywracamy terminal
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

	// Normalne wyjście – cleanup
	s.cleanup()
	return nil
}

// cleanup przywraca oryginalny stan terminala, wychodzi z alternatywnego bufora itp.
func (s *SSHSession) cleanup() {
	close(s.stopChan)
	s.setState(StateDisconnected)

	// Przywrócenie stanu konsoli Windows
	if s.oldState != nil {
		_ = term.RestoreTerminal(s.stdin.Fd(), s.oldState)
		s.oldState = nil
	}

	// Jeśli użyliśmy alternatywnego bufora, wyjdźmy z niego:
	// fmt.Fprint(s.stdout, "\x1b[?1049l")

	// Opcjonalnie można wyczyścić ekran:
	// fmt.Fprint(s.stdout, "\x1b[2J\x1b[H")

	time.Sleep(50 * time.Millisecond)
}

// handleSignals – obsługa sygnałów i zmiany rozmiaru
func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	// W osobnej goroutine monitorujemy rozmiar okna
	go func() {
		resizeTicker := time.NewTicker(100 * time.Millisecond)
		defer resizeTicker.Stop()

		lastW, lastH := s.termWidth, s.termHeight
		var resizeDebouncer *time.Timer
		var consecutiveErrors int

		for {
			select {
			case <-resizeTicker.C:
				ws, err := term.GetWinsize(os.Stdout.Fd())
				if err != nil {
					consecutiveErrors++
					if consecutiveErrors > 5 {
						s.setError(fmt.Errorf("terminal size monitoring error: %v", err))
					}
					continue
				}
				consecutiveErrors = 0

				w := int(ws.Width)
				h := int(ws.Height)
				if w == 0 {
					w = 80
				}
				if h == 0 {
					h = 24
				}

				if w != lastW || h != lastH {
					// Debouncer, żeby nie wysyłać WindowChange zbyt często
					if resizeDebouncer != nil {
						resizeDebouncer.Stop()
					}
					resizeDebouncer = time.AfterFunc(50*time.Millisecond, func() {
						s.stateMutex.Lock()
						defer s.stateMutex.Unlock()

						// Nie czyścimy ekranu tutaj – pozwalamy aplikacjom
						// curses (np. weechat) same go odświeżyć.
						if err := s.session.WindowChange(h, w); err != nil {
							s.setError(fmt.Errorf("failed to update window size: %v", err))
						} else {
							s.termWidth = w
							s.termHeight = h
							lastW = w
							lastH = h
						}
					})
				}

			case sig := <-sigChan:
				if sig == syscall.SIGTERM || sig == syscall.SIGINT {
					// Jeśli chcemy zareagować na SIGTERM / SIGINT lokalny
					s.Close()
					return
				}

			case <-s.stopChan:
				return
			}
		}
	}()
}

// keepAliveLoop wysyła pakiety keepalive do serwera
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
	// Zamknięcie kanału stopChan (jeśli jeszcze jest otwarty)
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

// clearTerminalBuffer – ewentualne ręczne czyszczenie ekranu
// Zaleca się używać tego oszczędnie, np. tylko przy wyjściu, jeśli w ogóle.
func (s *SSHSession) clearTerminalBuffer() {
	sequences := []string{
		"\x1b[?25l", // Ukryj kursor
		"\x1b[2J",   // Wyczyść cały ekran
		"\x1b[H",    // Przenieś kursor na początek
		"\x1b[?25h", // Pokaż kursor
	}
	for _, seq := range sequences {
		fmt.Fprint(s.stdout, seq)
		time.Sleep(10 * time.Millisecond)
	}
}
