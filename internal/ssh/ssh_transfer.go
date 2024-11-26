// internal/ssh/ssh_transfer.go

package ssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sshManager/internal/crypto"
	"sshManager/internal/models"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FileTransfer struct {
	sftpClient  *sftp.Client
	sshClient   *ssh.Client // To jest natywny klient golang.org/x/crypto/ssh
	currentHost *models.Host
	cipher      *crypto.Cipher
	connected   bool
}

// TransferProgress reprezentuje postęp transferu pliku
type TransferProgress struct {
	FileName         string
	TotalBytes       int64
	TransferredBytes int64
	StartTime        time.Time
}

// NewFileTransfer tworzy nową instancję FileTransfer
func NewFileTransfer(cipher *crypto.Cipher) *FileTransfer {
	return &FileTransfer{
		cipher:    cipher,
		connected: false,
	}
}

// Connect nawiązuje połączenie SFTP
func (ft *FileTransfer) Connect(host *models.Host, password string) error {
	if ft.connected {
		return nil
	}

	config := &ssh.ClientConfig{
		User: host.Login,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%s", host.IP, host.Port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}

	ft.sshClient = sshClient
	ft.sftpClient = sftpClient
	ft.currentHost = host
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
	if ft.sshClient != nil {
		if err := ft.sshClient.Close(); err != nil {
			return fmt.Errorf("error closing SSH client: %v", err)
		}
		ft.sshClient = nil
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

func (ft *FileTransfer) IsConnected() bool {
	return ft.connected && ft.sftpClient != nil
}

// internal/ssh/ssh_transfer.go

func (ft *FileTransfer) UploadFile(localPath, remotePath string, progressChan chan<- TransferProgress) error {
	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := ft.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer dstFile.Close()

	fileInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	progress := TransferProgress{
		FileName:   filepath.Base(localPath),
		TotalBytes: fileInfo.Size(),
		StartTime:  time.Now(),
	}

	bufSize := 128 * 1024 // Zwiększenie rozmiaru bufora do 128 KB
	buf := make([]byte, bufSize)
	for {
		n, err := srcFile.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading local file: %v", err)
		}

		if n > 0 {
			written, writeErr := dstFile.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("error writing remote file: %v", writeErr)
			}
			if written != n {
				return fmt.Errorf("incomplete write: wrote %d bytes instead of %d", written, n)
			}

			progress.TransferredBytes += int64(n)
			if progressChan != nil {
				select {
				case progressChan <- progress:
				default:
				}
			}
		}

		if err == io.EOF {
			break
		}
	}

	// Upewnij się, że dane zostały zapisane na zdalnym dysku
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync remote file: %v", err)
	}

	// Wyślij końcową aktualizację postępu
	if progressChan != nil {
		select {
		case progressChan <- progress:
		default:
		}
	}

	return nil
}

// internal/ssh/ssh_transfer.go

func (ft *FileTransfer) DownloadFile(remotePath, localPath string, progressChan chan<- TransferProgress) error {
	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	srcFile, err := ft.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer dstFile.Close()

	fileInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	progress := TransferProgress{
		FileName:   filepath.Base(remotePath),
		TotalBytes: fileInfo.Size(),
		StartTime:  time.Now(),
	}

	bufSize := 128 * 1024 // Zwiększenie rozmiaru bufora do 128 KB
	buf := make([]byte, bufSize)
	for {
		n, err := srcFile.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading remote file: %v", err)
		}

		if n > 0 {
			written, writeErr := dstFile.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("error writing local file: %v", writeErr)
			}
			if written != n {
				return fmt.Errorf("incomplete write: wrote %d bytes instead of %d", written, n)
			}

			progress.TransferredBytes += int64(n)
			if progressChan != nil {
				select {
				case progressChan <- progress:
				default:
				}
			}
		}

		if err == io.EOF {
			break
		}
	}

	// Upewnij się, że dane zostały zapisane na lokalnym dysku
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync local file: %v", err)
	}

	// Wyślij końcową aktualizację postępu
	if progressChan != nil {
		select {
		case progressChan <- progress:
		default:
		}
	}

	return nil
}

// Dodaj na końcu pliku internal/ssh/ssh_transfer.go

func (ft *FileTransfer) GetRemoteHomeDir() (string, error) {
	if !ft.connected {
		return "", fmt.Errorf("not connected")
	}

	session, err := ft.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	output, err := session.Output("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}

	// Usuń znak nowej linii z końca
	homeDir := strings.TrimSpace(string(output))
	return homeDir, nil
}
