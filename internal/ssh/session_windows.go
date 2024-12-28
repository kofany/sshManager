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
	// Dla Windows zawsze używamy xterm-256color
	if runtime.GOOS == "windows" {
		termType = "xterm-256color"
	}

	// Ustawienia trybu zbliżonego do raw dla zdalnego terminala
	// (możesz to dostosować w zależności od potrzeb)
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,      // Wyłącz echo (weechat zwykle sam wyświetla to, co potrzeba)
		ssh.TTY_OP_ISPEED: 115200, // Szybkość wejścia
		ssh.TTY_OP_OSPEED: 115200, // Szybkość wyjścia
		ssh.ONLCR:         1,      // Map NL na CR-NL (zostawione włączone by uniknąć problemów z CR/LF)
		ssh.ICANON:        0,      // Wyłącz tryb kanoniczny
		ssh.ISIG:          1,      // Pozostaw włączone sygnały (Ctrl+C, itp.)
		ssh.IEXTEN:        1,      // Włącz rozszerzone przetwarzanie wejścia
		ssh.OPOST:         1,      // Pozostaw podstawowe przetwarzanie wyjścia
	}

	// Pobierz aktualny rozmiar terminala
	fd := int(os.Stdout.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 24 // Domyślne wartości jeśli nie można pobrać
	}

	// Żądanie PTY z ustalonymi parametrami
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

	// Ustawiamy lokalnie tryb raw
	rawState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}

	// Możesz włączyć alternatywny bufor TYLKO jeśli masz pewność,
	// że nie będzie to kolidować z aplikacjami curses.
	// Jeśli weechat ma z tym problemy, po prostu wyłącz:
	// fmt.Fprint(s.stdout, "\x1b[?1049h") // ALTERNATYWNY bufor ekranu
	// fmt.Fprint(s.stdout, "\x1b[?25h")   // Pokaż kursor

	// Uruchomienie powłoki
	if err := s.session.Shell(); err != nil {
		// Przywracamy stan terminala, jeśli Shell() się nie uruchomi
		term.Restore(int(os.Stdin.Fd()), rawState)
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

	// Zakończenie – przywracamy stan i czyścimy
	s.cleanup(rawState)
	return nil
}

// cleanup przywraca stan terminala i wykonuje niezbędne czyszczenia
func (s *SSHSession) cleanup(rawState *term.State) {
	// Zatrzymujemy keepalive i sygnały
	close(s.stopChan)

	// Resetujemy stan sesji
	s.setState(StateDisconnected)

	// Przywracamy stan terminala
	if err := term.Restore(int(os.Stdin.Fd()), rawState); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restore terminal state: %v\n", err)
	}

	// Jeśli użyłeś alternatywnego bufora, wyjdź z niego:
	// fmt.Fprint(s.stdout, "\x1b[?1049l") // Powrót z alternatywnego bufora

	// Opcjonalnie możesz wykonać łagodne czyszczenie ekranu,
	// np. tylko "\x1b[2J\x1b[H" – bez scrollback:
	// fmt.Fprint(s.stdout, "\x1b[2J\x1b[H")

	// Krótkie opóźnienie, by terminal zdążył się zaktualizować
	time.Sleep(50 * time.Millisecond)
}

// handleSignals obsługuje sygnały systemowe
func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	go func() {
		// Odpytywanie o rozmiar terminala co 100 ms
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

						// Usuwamy tu wywołanie s.clearTerminalBuffer()
						// Pozwalamy aplikacjom curses (np. weechat) samodzielnie
						// zareagować na zmianę rozmiaru.

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
	// Zamknięcie kanału stopChan (o ile nie jest już zamknięty)
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

// clearTerminalBuffer – jeśli jednak chcesz używać, np. tylko przy wyjściu
// i naprawdę musisz wyczyścić cały ekran:
func (s *SSHSession) clearTerminalBuffer() {
	// Używaj tej funkcji oszczędnie.
	// Standardowe sekwencje czyszczące (bez scrollback 3J):
	sequences := []string{
		"\x1b[?25l", // Ukryj kursor
		"\x1b[2J",   // Wyczyść cały ekran
		"\x1b[H",    // Kursor na początek
		"\x1b[?25h", // Pokaż kursor
	}

	for _, seq := range sequences {
		fmt.Fprint(s.stdout, seq)
		time.Sleep(10 * time.Millisecond) // krótkie opóźnienie
	}
}
