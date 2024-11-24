// internal/ssh/ssh_transfer.go

package ssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"sshManager/internal/crypto"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FileTransfer struct {
	client     *SSHClient
	sftpClient *sftp.Client
	connected  bool
	cipher     *crypto.Cipher // Dodajemy pole cipher
}

// TransferProgress reprezentuje postęp transferu pliku
type TransferProgress struct {
	FileName         string
	TotalBytes       int64
	TransferredBytes int64
	StartTime        time.Time
}

// NewFileTransfer tworzy nową instancję FileTransfer
func NewFileTransfer(client *SSHClient, cipher *crypto.Cipher) *FileTransfer {
	return &FileTransfer{
		client:     client,
		sftpClient: nil,
		connected:  false,
		cipher:     cipher,
	}
}

// Connect nawiązuje połączenie SFTP
func (ft *FileTransfer) Connect() error {
	if ft.connected {
		return nil
	}

	host := ft.client.GetCurrentHost()
	if host == nil {
		return fmt.Errorf("no host selected")
	}

	password, err := ft.getPassword()
	if err != nil {
		return fmt.Errorf("failed to get password: %v", err)
	}

	// Konfiguracja połączenia SSH dla SFTP
	sshConfig := &ssh.ClientConfig{
		User: host.Login,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Nawiązanie połączenia SSH
	addr := fmt.Sprintf("%s:%s", host.IP, host.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}

	// Utworzenie klienta SFTP
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}

	ft.sftpClient = sftpClient
	ft.connected = true
	return nil
}

// Disconnect zamyka połączenie SFTP
func (ft *FileTransfer) Disconnect() error {
	if ft.sftpClient != nil {
		if err := ft.sftpClient.Close(); err != nil {
			return fmt.Errorf("error closing SFTP client: %v", err)
		}
		ft.sftpClient = nil
	}
	ft.connected = false
	return nil
}

// ListLocalFiles zwraca listę plików w lokalnym katalogu
func (ft *FileTransfer) ListLocalFiles(path string) ([]os.FileInfo, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	return dir.Readdir(-1)
}

// ListRemoteFiles zwraca listę plików w zdalnym katalogu
func (ft *FileTransfer) ListRemoteFiles(path string) ([]os.FileInfo, error) {
	if !ft.connected {
		return nil, fmt.Errorf("not connected")
	}

	return ft.sftpClient.ReadDir(path)
}

// UploadFile wysyła plik na serwer
func (ft *FileTransfer) UploadFile(localPath, remotePath string, progressChan chan<- TransferProgress) error {
	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	// Otwórz lokalny plik
	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer srcFile.Close()

	// Utwórz zdalny plik
	dstFile, err := ft.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer dstFile.Close()

	// Przygotuj informacje o transferze
	fileInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	progress := &TransferProgress{
		FileName:   filepath.Base(localPath),
		TotalBytes: fileInfo.Size(),
		StartTime:  time.Now(),
	}

	// Kopiuj z monitorowaniem postępu
	buf := make([]byte, 32*1024)
	for {
		n, err := srcFile.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading local file: %v", err)
		}

		if _, err := dstFile.Write(buf[:n]); err != nil {
			return fmt.Errorf("error writing remote file: %v", err)
		}

		progress.TransferredBytes += int64(n)
		if progressChan != nil {
			progressChan <- *progress
		}
	}

	return nil
}

// DownloadFile pobiera plik z serwera
func (ft *FileTransfer) DownloadFile(remotePath, localPath string, progressChan chan<- TransferProgress) error {
	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	// Otwórz zdalny plik
	srcFile, err := ft.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer srcFile.Close()

	// Utwórz lokalny plik
	dstFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer dstFile.Close()

	// Przygotuj informacje o transferze
	fileInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	progress := &TransferProgress{
		FileName:   filepath.Base(remotePath),
		TotalBytes: fileInfo.Size(),
		StartTime:  time.Now(),
	}

	// Kopiuj z monitorowaniem postępu
	buf := make([]byte, 32*1024)
	for {
		n, err := srcFile.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading remote file: %v", err)
		}

		if _, err := dstFile.Write(buf[:n]); err != nil {
			return fmt.Errorf("error writing local file: %v", err)
		}

		progress.TransferredBytes += int64(n)
		if progressChan != nil {
			progressChan <- *progress
		}
	}

	return nil
}

// GetRemoteFileInfo zwraca informacje o zdalnym pliku
func (ft *FileTransfer) GetRemoteFileInfo(path string) (os.FileInfo, error) {
	if !ft.connected {
		return nil, fmt.Errorf("not connected")
	}

	return ft.sftpClient.Stat(path)
}

// CreateRemoteDirectory tworzy katalog na zdalnym serwerze
func (ft *FileTransfer) CreateRemoteDirectory(path string) error {
	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	return ft.sftpClient.MkdirAll(path)
}

// RemoveRemoteFile usuwa plik lub katalog na zdalnym serwerze
func (ft *FileTransfer) RemoveRemoteFile(path string) error {
	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	return ft.sftpClient.Remove(path)
}

// RenameRemoteFile zmienia nazwę pliku na zdalnym serwerze
func (ft *FileTransfer) RenameRemoteFile(oldPath, newPath string) error {
	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	return ft.sftpClient.Rename(oldPath, newPath)
}

func (ft *FileTransfer) getPassword() (string, error) {
	host := ft.client.GetCurrentHost()
	if host == nil {
		return "", fmt.Errorf("no host selected")
	}

	passwords := ft.client.GetPasswords()
	if host.PasswordID >= len(passwords) {
		return "", fmt.Errorf("invalid password ID")
	}

	return passwords[host.PasswordID].GetDecrypted(ft.cipher)
}
