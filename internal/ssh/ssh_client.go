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

func CreateSSHCommand(host *models.Host, password string) *exec.Cmd {
	sshCommand := fmt.Sprintf(
		"clear; sshpass -p '%s' ssh -o StrictHostKeyChecking=no %s@%s -p %s; clear",
		password, host.Login, host.IP, host.Port,
	)
	cmd := exec.Command("sh", "-c", sshCommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (s *SSHClient) Connect(host *models.Host, password string) error {
	fmt.Printf("\nConnecting to %s@%s...\n", host.Login, host.IP)
	cmd := CreateSSHCommand(host, password)
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
