// internal/ssh/ssh_transfer.go

package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sshManager/internal/crypto"
	"sshManager/internal/models"
	"sshManager/internal/utils"

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

// Connect establishes SSH, SCP, and SFTP connections to the remote host.
// It handles both password and SSH key authentication methods.
// The function is thread-safe and ensures only one active connection at a time.
// Parameters:
//   - host: The host configuration containing connection details
//   - authData: Either the password or the path to the SSH key file
//
// Returns an error if any part of the connection process fails.
func (ft *FileTransfer) Connect(host *models.Host, authData string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if ft.connected {
		return nil
	}

	// Setup authentication method based on host configuration
	var authMethod ssh.AuthMethod
	if host.PasswordID < 0 {
		// SSH key authentication
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
		// Password authentication
		authMethod = ssh.Password(authData)
	}

	// Configure SSH client
	config := &ssh.ClientConfig{
		User:            host.Login,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Establish SSH connection
	addr := fmt.Sprintf("%s:%s", host.IP, host.Port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	// Initialize SCP client
	scpClient, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("failed to create SCP client: %v", err)
	}

	// Initialize SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		scpClient.Close()
		sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}

	// Store client instances
	ft.sshClient = sshClient
	ft.scpClient = scpClient
	ft.sftpClient = sftpClient
	ft.currentHost = host
	ft.connected = true

	return nil
}

// Disconnect closes all active connections (SCP, SFTP, and SSH) in a safe manner.
// The function is thread-safe and collects all errors that occur during disconnection.
// It ensures all resources are properly released even if some operations fail.
// Returns a combined error if any close operations fail.
func (ft *FileTransfer) Disconnect() error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	var errors []string

	// Close clients in reverse order of creation
	ft.scpClient.Close()

	if ft.sftpClient != nil {
		if err := ft.sftpClient.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("SFTP client close error: %v", err))
		}
		ft.sftpClient = nil
	}

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
// Thread-safe method that returns the current connection state
func (ft *FileTransfer) IsConnected() bool {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	return ft.connected
}

// ListLocalFiles returns a list of files in the local directory
// The path should be provided in the local system format
func (ft *FileTransfer) ListLocalFiles(path string) ([]os.FileInfo, error) {
	// Normalize path for local system
	normalizedPath := utils.NormalizePath(path, false)

	dir, err := os.Open(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local directory: %v", err)
	}
	defer dir.Close()
	return dir.Readdir(-1)
}

// ListRemoteFiles returns a list of files in the remote directory
// The path will be automatically converted to UNIX format
func (ft *FileTransfer) ListRemoteFiles(path string) ([]os.FileInfo, error) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Normalize path for remote system
	normalizedPath := utils.NormalizePath(path, true)
	return ft.sftpClient.ReadDir(normalizedPath)
}

// GetRemoteFileInfo returns information about a remote file
// The path will be automatically converted to UNIX format
func (ft *FileTransfer) GetRemoteFileInfo(path string) (os.FileInfo, error) {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return nil, fmt.Errorf("not connected")
	}

	normalizedPath := utils.NormalizePath(path, true)
	return ft.sftpClient.Stat(normalizedPath)
}

// CreateRemoteDirectory creates a directory on the remote server
// The path will be automatically converted to UNIX format
func (ft *FileTransfer) CreateRemoteDirectory(path string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	normalizedPath := utils.NormalizePath(path, true)
	return ft.sftpClient.MkdirAll(normalizedPath)
}

// RemoveRemoteFile removes a file or directory on the remote server
// The path will be automatically converted to UNIX format
func (ft *FileTransfer) RemoveRemoteFile(path string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	normalizedPath := utils.NormalizePath(path, true)

	// First, try to remove as a file
	err := ft.sftpClient.Remove(normalizedPath)
	if err == nil {
		return nil
	}

	// If it fails, check if it's a directory
	info, err := ft.sftpClient.Stat(normalizedPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	if info.IsDir() {
		return ft.RemoveRemoteDirectoryRecursive(normalizedPath)
	}

	return fmt.Errorf("failed to remove file: %v", err)
}

// RenameRemoteFile renames a file on the remote server
// Both paths will be automatically converted to UNIX format
func (ft *FileTransfer) RenameRemoteFile(oldPath, newPath string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	normalizedOldPath := utils.NormalizePath(oldPath, true)
	normalizedNewPath := utils.NormalizePath(newPath, true)

	return ft.sftpClient.Rename(normalizedOldPath, normalizedNewPath)
}

// GetRemoteHomeDir returns the home directory on the remote server
// The returned path will be in UNIX format
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

	homePath := strings.TrimSpace(string(output))
	return utils.NormalizePath(homePath, true), nil
}

// UploadFile uploads a file to the server using SCP
// Both paths will be normalized according to their respective systems
// Progress will be reported through the provided channel
func (ft *FileTransfer) UploadFile(localPath, remotePath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Normalize paths for respective systems
	normalizedLocalPath := utils.NormalizePath(localPath, false)
	normalizedRemotePath := utils.NormalizePath(remotePath, true)

	// Open local file
	localFile, err := os.Open(normalizedLocalPath)
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
	passThru := func(r io.Reader, total int64) io.Reader {
		return &ProgressReader{
			Reader:    r,
			Total:     total,
			FileName:  filepath.Base(localPath), // Use original name for display
			StartTime: startTime,
			Progress:  progressChan,
		}
	}

	// Copy file to remote server
	err = ft.scpClient.CopyFilePassThru(ctx, localFile, normalizedRemotePath, perm, passThru)
	if err != nil {
		return fmt.Errorf("error while uploading file: %v", err)
	}

	return nil
}

// DownloadFile downloads a file from the server using SCP
// Both paths will be normalized according to their respective systems
// Progress will be reported through the provided channel
func (ft *FileTransfer) DownloadFile(remotePath, localPath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Normalize paths for respective systems
	normalizedRemotePath := utils.NormalizePath(remotePath, true)
	normalizedLocalPath := utils.NormalizePath(localPath, false)

	// Create directory structure if it doesn't exist
	if dir := filepath.Dir(normalizedLocalPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory structure: %v", err)
		}
	}

	// Open local file for writing
	localFile, err := os.OpenFile(normalizedLocalPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close()

	// Start time for progress
	startTime := time.Now()

	// Prepare context
	ctx := context.Background()

	// Define PassThru function for progress reporting
	passThru := func(r io.Reader, total int64) io.Reader {
		return &ProgressReader{
			Reader:    r,
			Total:     total,
			FileName:  filepath.Base(remotePath), // Use original name for display
			StartTime: startTime,
			Progress:  progressChan,
		}
	}

	// Copy file from remote server
	err = ft.scpClient.CopyFromRemotePassThru(ctx, localFile, normalizedRemotePath, passThru)
	if err != nil {
		return fmt.Errorf("error while downloading file: %v", err)
	}

	return nil
}

// RemoveRemoteDirectoryRecursive removes a directory recursively on the remote server
// All paths will be normalized to UNIX format
func (ft *FileTransfer) RemoveRemoteDirectoryRecursive(path string) error {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if !ft.connected {
		return fmt.Errorf("not connected")
	}

	// Normalize input path
	normalizedPath := utils.NormalizePath(path, true)

	entries, err := ft.sftpClient.ReadDir(normalizedPath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	for _, entry := range entries {
		// Use forward slashes for remote paths
		fullPath := utils.NormalizePath(filepath.Join(normalizedPath, entry.Name()), true)
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

	return ft.sftpClient.RemoveDirectory(normalizedPath)
}

// UploadDirectory uploads an entire directory to the server
// All paths will be normalized according to their respective systems
// Progress is reported through the provided channel
func (ft *FileTransfer) UploadDirectory(localPath, remotePath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Normalize the initial paths
	normalizedLocalPath := utils.NormalizePath(localPath, false)
	normalizedRemotePath := utils.NormalizePath(remotePath, true)

	// Create the destination directory
	if err := ft.CreateRemoteDirectory(normalizedRemotePath); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	// Walk through the directory and transfer files
	err := filepath.Walk(normalizedLocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source directory
		relPath, err := filepath.Rel(normalizedLocalPath, path)
		if err != nil {
			return err
		}

		// Create remote path using normalized path joining
		remotePathFull := utils.NormalizePath(filepath.Join(normalizedRemotePath, relPath), true)

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
// All paths will be normalized according to their respective systems
// Progress is reported through the provided channel
func (ft *FileTransfer) DownloadDirectory(remotePath, localPath string, progressChan chan<- TransferProgress) error {
	ft.mutex.Lock()
	if !ft.connected {
		ft.mutex.Unlock()
		return fmt.Errorf("not connected")
	}
	ft.mutex.Unlock()

	// Normalize paths
	normalizedRemotePath := utils.NormalizePath(remotePath, true)
	normalizedLocalPath := utils.NormalizePath(localPath, false)

	// Create local directory
	if err := os.MkdirAll(normalizedLocalPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Get list of files
	entries, err := ft.sftpClient.ReadDir(normalizedRemotePath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	// Process each file/directory
	for _, entry := range entries {
		remoteSrcPath := utils.NormalizePath(filepath.Join(normalizedRemotePath, entry.Name()), true)
		localDstPath := utils.NormalizePath(filepath.Join(normalizedLocalPath, entry.Name()), false)

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
// This structure remains unchanged as it doesn't deal with paths
type ProgressReader struct {
	Reader         io.Reader
	Total          int64
	Transferred    int64
	FileName       string
	StartTime      time.Time
	Progress       chan<- TransferProgress
	LastReportTime time.Time
}

// Read implements io.Reader interface and reports transfer progress
// This method remains unchanged as it doesn't deal with paths
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
