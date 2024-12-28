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

// SessionState represents SSH session state
type SessionState int

const (
	StateDisconnected SessionState = iota
	StateConnecting
	StateConnected
	StateError
)

// SSHSession represents an active SSH session
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

// NewSSHSession creates a new SSH session
func NewSSHSession(client *ssh.Client) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	fd := os.Stdout.Fd()
	size, err := term.GetWinsize(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to get terminal size: %v", err)
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
	}

	return s, nil
}

// ConfigureTerminal configures the terminal for the session
func (s *SSHSession) ConfigureTerminal(termType string) error {
	if runtime.GOOS == "windows" {
		termType = "xterm-256color"
	}

	fd := os.Stdout.Fd()
	size, err := term.GetWinsize(fd)
	if err != nil {
		return fmt.Errorf("failed to get terminal size: %v", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}

	if err := s.session.RequestPty(termType, int(size.Height), int(size.Width), modes); err != nil {
		return fmt.Errorf("failed to request PTY: %v", err)
	}

	state, err := term.SetRawTerminal(fd)
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}

	s.originalTermState = state
	return nil
}

// StartShell starts the interactive shell session
func (s *SSHSession) StartShell() error {
	s.session.Stdin = s.stdin
	s.session.Stdout = s.stdout
	s.session.Stderr = s.stderr

	go s.handleSignals()

	if s.keepAlive > 0 {
		go s.keepAliveLoop()
	}

	cleanup := func() {
		close(s.stopChan)
		s.setState(StateDisconnected)

		time.Sleep(150 * time.Millisecond)

		if s.originalTermState != nil {
			term.RestoreTerminal(os.Stdout.Fd(), s.originalTermState)
		}

		time.Sleep(50 * time.Millisecond)
	}
	defer cleanup()

	if err := s.session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %v", err)
	}

	s.setState(StateConnected)

	if err := s.session.Wait(); err != nil {
		errStr := err.Error()
		if errStr != "Process exited with status 1" &&
			!strings.Contains(errStr, "exit status") &&
			!strings.Contains(errStr, "signal: terminated") &&
			!strings.Contains(errStr, "signal: interrupt") {
			return fmt.Errorf("session ended with error: %v", err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	return nil
}

// handleSignals handles system signals and terminal resizing
func (s *SSHSession) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	go func() {
		resizeTicker := time.NewTicker(250 * time.Millisecond)
		defer resizeTicker.Stop()

		var lastWidth, lastHeight = s.termWidth, s.termHeight
		var consecutiveErrors int

		for {
			select {
			case <-resizeTicker.C:
				size, err := term.GetWinsize(os.Stdout.Fd())
				if err != nil {
					consecutiveErrors++
					if consecutiveErrors > 5 {
						s.setError(fmt.Errorf("terminal size monitoring error: %v", err))
					}
					continue
				}
				consecutiveErrors = 0

				width, height := int(size.Width), int(size.Height)
				if width != lastWidth || height != lastHeight {
					s.stateMutex.Lock()
					if err := s.session.WindowChange(height, width); err != nil {
						s.setError(fmt.Errorf("failed to update window size: %v", err))
					} else {
						s.termWidth = width
						s.termHeight = height
						lastWidth, lastHeight = width, height
					}
					s.stateMutex.Unlock()
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

// keepAliveLoop sends keepalive messages
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

// Close closes the session
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

	s.setState(StateDisconnected)

	if len(errors) > 0 {
		return fmt.Errorf("close errors: %s", strings.Join(errors, "; "))
	}
	return nil
}

// setState sets the session state
func (s *SSHSession) setState(state SessionState) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.state = state
}

// setError sets the session error
func (s *SSHSession) setError(err error) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.lastError = err
	s.state = StateError
}

// GetState returns the current session state
func (s *SSHSession) GetState() SessionState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.state
}

// GetLastError returns the last error
func (s *SSHSession) GetLastError() error {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.lastError
}

// SetKeepAlive sets the keepalive interval
func (s *SSHSession) SetKeepAlive(duration time.Duration) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()
	s.keepAlive = duration
}
