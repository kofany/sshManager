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

	"github.com/containerd/console"
	"golang.org/x/crypto/ssh"
)

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
	winConsole console.Console
}

func NewSSHSession(client *ssh.Client) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Inicjalizacja konsoli Windows
	con := console.Current()
	if err := con.SetRaw(); err != nil {
		return nil, fmt.Errorf("failed to set raw console mode: %w", err)
	}

	// Pobierz rozmiar
	size, err := con.Size()
	if err != nil {
		size.Width = 80
		size.Height = 24
	}

	s := &SSHSession{
		client:     client,
		session:    session,
		state:      StateConnecting,
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		termWidth:  int(size.Width),
		termHeight: int(size.Height),
		keepAlive:  30 * time.Second,
		stopChan:   make(chan struct{}),
		winConsole: con,
	}

	return s, nil
}

func (s *SSHSession) ConfigureTerminal(termType string) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
		ssh.VINTR:         3,   // Ctrl+C
		ssh.VQUIT:         28,  // Ctrl+\
		ssh.VERASE:        127, // Backspace
		ssh.VKILL:         21,  // Ctrl+U
		ssh.VEOF:          4,   // Ctrl+D
		ssh.VWERASE:       23,  // Ctrl+W
		ssh.VLNEXT:        22,  // Ctrl+V
		ssh.VSUSP:         26,  // Ctrl+Z
		ssh.OCRNL:         0,   // Disable CR to NL
		ssh.ONLCR:         1,   // Map NL to CR-NL
		ssh.ICRNL:         1,   // Map CR to NL on input
		ssh.IEXTEN:        1,   // Extended input processing
	}

	if termType == "" {
		termType = "xterm-256color"
	}

	if err := s.session.RequestPty(termType, s.termHeight, s.termWidth, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	return nil
}

func (s *SSHSession) StartShell() error {
	s.session.Stdin = s.stdin
	s.session.Stdout = s.stdout
	s.session.Stderr = s.stderr

	// Zachowaj oryginalny stan konsoli
	if err := s.winConsole.SetRaw(); err != nil {
		return fmt.Errorf("failed to set raw console mode: %w", err)
	}

	go s.handleSignals()

	if s.keepAlive > 0 {
		go s.keepAliveLoop()
	}

	cleanup := func() {
		close(s.stopChan)
		s.setState(StateDisconnected)

		// Przywróć oryginalny stan konsoli
		if err := s.winConsole.Reset(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to reset console: %v\n", err)
		}

		if s.winConsole != nil {
			s.winConsole.Close()
		}
	}
	defer cleanup()

	if err := s.session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	s.setState(StateConnected)

	if err := s.session.Wait(); err != nil {
		errStr := err.Error()
		if errStr != "Process exited with status 1" &&
			!strings.Contains(errStr, "exit status") &&
			!strings.Contains(errStr, "signal: terminated") &&
			!strings.Contains(errStr, "signal: interrupt") {
			return fmt.Errorf("session ended with error: %w", err)
		}
	}

	return nil
}

func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	// Monitorowanie rozmiaru
	go func() {
		resizeTicker := time.NewTicker(250 * time.Millisecond)
		defer resizeTicker.Stop()

		var lastWidth, lastHeight = s.termWidth, s.termHeight

		for {
			select {
			case <-resizeTicker.C:
				size, err := s.winConsole.Size()
				if err != nil {
					continue
				}

				width, height := int(size.Width), int(size.Height)
				if width != lastWidth || height != lastHeight {
					s.stateMutex.Lock()
					if err := s.session.WindowChange(height, width); err == nil {
						s.termWidth = width
						s.termHeight = height
						lastWidth, lastHeight = width, height
					}
					s.stateMutex.Unlock()
				}
			case <-s.stopChan:
				return
			}
		}
	}()

	for {
		select {
		case sig := <-sigChan:
			if sig == syscall.SIGTERM || sig == syscall.SIGINT {
				s.Close()
				return
			}
		case <-s.stopChan:
			return
		}
	}
}

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

func (s *SSHSession) Close() error {
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

	if s.winConsole != nil {
		if err := s.winConsole.Reset(); err != nil {
			errors = append(errors, fmt.Sprintf("console reset error: %v", err))
		}
		s.winConsole.Close()
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
