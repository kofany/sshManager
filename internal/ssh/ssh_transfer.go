// internal/ssh/ssh_transfer.go

package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"sshManager/internal/crypto"
	"sshManager/internal/models"

	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// FileTransfer represents a file transfer session
type FileTransfer struct {
	sshClient   *ssh.Client
	scpClient   scp.Client
	sftpClient  *sftp.Client
	currentHost *models.Host
	cipher      *crypto.Cipher
	connected   bool
	mutex       sync.Mutex
}

// TransferProgress represents the progress of a file transfer
type TransferProgress struct {
	FileName         string
	TotalBytes       int64
	TransferredBytes int64
	StartTime        time.Time
}

// NewFileTransfer creates a new instance of FileTransfer
func NewFileTransfer(cipher *crypto.Cipher) *FileTransfer {
	return &FileTransfer{
		cipher:    cipher,
		connected: false,
	}
}

// Connect establishes an SSH, SCP, and SFTP connection
func (ft *FileTransfer) Connect(host *models.Host, authData string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if ft.connected {
		return nil
	}

	var authMethod ssh.AuthMethod
	if host.PasswordID < 0 {
		// Using SSH key authentication
		key, err := os.ReadFile(authData)
		if err != nil {
			return fmt.Errorf("failed to read SSH key: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %v", err)
		}
		authMethod = ssh.PublicKeys(signer)
	} else {
		// Using password authentication
		authMethod = ssh.Password(authData)
	}

	config := &ssh.ClientConfig{
		User:            host.Login,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", host.IP, host.Port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	// Create SCP client using existing SSH connection
	scpClient, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("failed to create SCP client: %v", err)
	}

	// Create SFTP client for directory operations
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		scpClient.Close()
		sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}

	ft.sshClient = sshClient
	ft.scpClient = scpClient
	ft.sftpClient = sftpClient
	ft.currentHost = host
	ft.connected = true

	return nil
}

// Disconnect closes the SCP, SFTP, and SSH connections
func (ft *FileTransfer) Disconnect() error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	var errors []string

	// Close the SCP client
	ft.scpClient.Close()

	// Close the SFTP client
	if ft.sftpClient != nil {
		if err := ft.sftpClient.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("SFTP client close error: %v", err))
		}
		ft.sftpClient = nil
	}

	// Close the SSH client
	if ft.sshClient != nil {
		if err := ft.sshClient.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("SSH client close error: %v", err))
		}
		ft.sshClient = nil
	}

	ft.connected = false
	ft.currentHost = nil

	if len(errors) > 0 {
		return fmt.Errorf("disconnect errors: %s", strings.Join(errors, "; "))
	}
	return nil
}

// IsConnected checks if the SSH connection is active
func (ft *FileTransfer) IsConnected() bool {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	return ft.connected
}

// ListLocalFiles returns a list of files in the local directory
func (ft *FileTransfer) ListLocalFiles(path string) ([]os.FileInfo, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open local directory: %v", err)
	}
	defer dir.Close()

	return dir.Readdir(-1)
}

// ListRemoteFiles returns a list of files in the remote directory
func (ft *FileTransfer) ListRemoteFiles(path string) ([]os.FileInfo, error) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return nil, fmt.Errorf("not connected")
	}

	return ft.sftpClient.ReadDir(path)
}

// GetRemoteFileInfo returns information about a remote file
func (ft *FileTransfer) GetRemoteFileInfo(path string) (os.FileInfo, error) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return nil, fmt.Errorf("not connected")
	}

	return ft.sftpClient.Stat(path)
}

// CreateRemoteDirectory creates a directory on the remote server
func (ft *FileTransfer) CreateRemoteDirectory(path string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	return ft.sftpClient.MkdirAll(path)
}

// RemoveRemoteFile removes a file or directory on the remote server
func (ft *FileTransfer) RemoveRemoteFile(path string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	// First, try to remove as a file
	err := ft.sftpClient.Remove(path)
	if err == nil {
		return nil
	}

	// If it fails, check if it's a directory
	info, err := ft.sftpClient.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	if info.IsDir() {
		return ft.RemoveRemoteDirectoryRecursive(path)
	}

	return fmt.Errorf("failed to remove file: %v", err)
}

// RenameRemoteFile renames a file on the remote server
func (ft *FileTransfer) RenameRemoteFile(oldPath, newPath string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	return ft.sftpClient.Rename(oldPath, newPath)
}

// GetRemoteHomeDir returns the home directory on the remote server
func (ft *FileTransfer) GetRemoteHomeDir() (string, error) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return "", fmt.Errorf("not connected")
	}

	session, err := ft.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	output, err := session.Output("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (ft *FileTransfer) UploadFile(localPath, remotePath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Convert remote path to SFTP format (ensure forward slashes)
	remotePath = toSFTPPath(remotePath)

	// Use local path as is since it's already in correct format for the OS
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close()

	// Get file info for permissions
	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %v", err)
	}

	// Set permissions (convert to string in octal)
	perm := fmt.Sprintf("%#o", fileInfo.Mode().Perm())

	// Start time for progress
	startTime := time.Now()

	// Prepare context
	ctx := context.Background()

	// Define PassThru function for progress reporting
	// Use filepath.Base for the local path to get proper filename
	passThru := func(r io.Reader, total int64) io.Reader {
		return &ProgressReader{
			Reader:    r,
			Total:     total,
			FileName:  filepath.Base(localPath),
			StartTime: startTime,
			Progress:  progressChan,
		}
	}

	// Copy file to remote server using the converted path
	err = ft.scpClient.CopyFilePassThru(ctx, localFile, remotePath, perm, passThru)
	if err != nil {
		return fmt.Errorf("error while uploading file: %v", err)
	}

	return nil
}

func (ft *FileTransfer) DownloadFile(remotePath, localPath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Convert paths appropriately
	remotePath = toSFTPPath(remotePath)
	localPath = toLocalPath(localPath)

	// Ensure the target directory exists
	targetDir := filepath.Dir(localPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %v", err)
	}

	// Open local file for writing with proper permissions
	localFile, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close()

	// Start time for progress
	startTime := time.Now()

	// Prepare context
	ctx := context.Background()

	// Define PassThru function for progress reporting
	// Use filepath.Base with the remote path to get proper filename
	passThru := func(r io.Reader, total int64) io.Reader {
		return &ProgressReader{
			Reader:    r,
			Total:     total,
			FileName:  filepath.Base(remotePath),
			StartTime: startTime,
			Progress:  progressChan,
		}
	}

	// Copy file from remote server using the converted paths
	err = ft.scpClient.CopyFromRemotePassThru(ctx, localFile, remotePath, passThru)
	if err != nil {
		return fmt.Errorf("error while downloading file: %v", err)
	}

	return nil
}

// RemoveRemoteDirectoryRecursive removes a directory recursively on the remote server
func (ft *FileTransfer) RemoveRemoteDirectoryRecursive(path string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	entries, err := ft.sftpClient.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if err := ft.RemoveRemoteDirectoryRecursive(fullPath); err != nil {
				return err
			}
		} else {
			if err := ft.sftpClient.Remove(fullPath); err != nil {
				return err
			}
		}
	}

	return ft.sftpClient.RemoveDirectory(path)
}

// UploadDirectory uploads an entire directory to the server
func (ft *FileTransfer) UploadDirectory(localPath, remotePath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Create the destination directory
	if err := ft.CreateRemoteDirectory(remotePath); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	// Walk through the directory and transfer files
	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		remotePathFull := filepath.Join(remotePath, relPath)

		if info.IsDir() {
			return ft.CreateRemoteDirectory(remotePathFull)
		}

		return ft.UploadFile(path, remotePathFull, progressChan)
	})

	if err != nil {
		return fmt.Errorf("failed to upload directory: %v", err)
	}

	return nil
}

// DownloadDirectory downloads a directory from the server
func (ft *FileTransfer) DownloadDirectory(remotePath, localPath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Create local directory
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Get list of files
	entries, err := ft.sftpClient.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	// Process each file/directory
	for _, entry := range entries {
		remoteSrcPath := filepath.Join(remotePath, entry.Name())
		localDstPath := filepath.Join(localPath, entry.Name())

		if entry.IsDir() {
			if err := ft.DownloadDirectory(remoteSrcPath, localDstPath, progressChan); err != nil {
				return err
			}
		} else {
			if err := ft.DownloadFile(remoteSrcPath, localDstPath, progressChan); err != nil {
				return err
			}
		}
	}

	return nil
}

// ProgressReader wraps an io.Reader to report progress
type ProgressReader struct {
	Reader         io.Reader
	Total          int64
	Transferred    int64
	FileName       string
	StartTime      time.Time
	Progress       chan<- TransferProgress
	LastReportTime time.Time
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Transferred += int64(n)

	// Report progress every second or when done
	now := time.Now()
	if now.Sub(pr.LastReportTime) >= time.Second || err == io.EOF {
		pr.LastReportTime = now
		progress := TransferProgress{
			FileName:         pr.FileName,
			TotalBytes:       pr.Total,
			TransferredBytes: pr.Transferred,
			StartTime:        pr.StartTime,
		}
		if pr.Progress != nil {
			select {
			case pr.Progress <- progress:
			default:
			}
		}
	}

	return n, err
}

// toSFTPPath converts local path to SFTP path format
func toSFTPPath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "\\", "/")
	}
	return path
}

// toLocalPath converts SFTP path to local path format
func toLocalPath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "/", "\\")
	}
	return path
}
