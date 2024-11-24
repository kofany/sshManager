// internal/ssh/ssh_client.go

package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"sshManager/internal/models"
)

type SSHClient struct {
	currentHost *models.Host
	passwords   []models.Password
}

func (s *SSHClient) GetPasswords() []models.Password {
	return s.passwords
}

func NewSSHClient(passwords []models.Password) *SSHClient {
	return &SSHClient{
		passwords: passwords,
	}
}

func (s *SSHClient) Connect(host *models.Host, password string) error {
	sshCommand := fmt.Sprintf("sshpass -p '%s' ssh -o stricthostkeychecking=no %s@%s -p %s",
		password, host.Login, host.IP, host.Port)

	// Debug info (bezpieczne - nie pokazuje hasła)
	fmt.Printf("\nConnecting to %s@%s...\n", host.Login, host.IP)

	cmd := exec.Command("sh", "-c", sshCommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	s.currentHost = host
	return cmd.Run()
}

func (s *SSHClient) IsConnected() bool {
	return s.currentHost != nil
}

func (s *SSHClient) Disconnect() {
	s.currentHost = nil
}

func (s *SSHClient) GetCurrentHost() *models.Host {
	return s.currentHost
}
