// internal/ssh/transfer.go

package ssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FileTransfer obsługuje operacje transferu plików
type FileTransfer struct {
	conn *Connection
}

// NewFileTransfer tworzy nową instancję FileTransfer
func NewFileTransfer(conn *Connection) *FileTransfer {
	return &FileTransfer{
		conn: conn,
	}
}

// TransferProgress reprezentuje postęp transferu
type TransferProgress struct {
	FileName         string
	TotalBytes       int64
	TransferredBytes int64
	StartTime        time.Time
}

// UploadFile wysyła plik na serwer
func (ft *FileTransfer) UploadFile(localPath, remotePath string, progressChan chan<- TransferProgress) error {
	// Sprawdzenie czy połączenie jest aktywne
	if !ft.conn.IsConnected() {
		return fmt.Errorf("ssh connection is not active")
	}

	// Otwarcie lokalnego pliku
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close()

	// Pobranie informacji o pliku
	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	// Utworzenie nowej sesji
	session, err := ft.conn.GetClient().NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	// Przygotowanie pipe'a do przesyłania danych
	writer, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	// Przygotowanie komendy SCP
	remoteFileName := filepath.Base(remotePath)
	scpCommand := fmt.Sprintf("scp -t %s", remotePath)

	// Uruchomienie komendy SCP
	if err := session.Start(scpCommand); err != nil {
		return fmt.Errorf("failed to start scp command: %v", err)
	}

	// Wysłanie nagłówka pliku
	fileHeader := fmt.Sprintf("C0644 %d %s\n", fileInfo.Size(), remoteFileName)
	if _, err := writer.Write([]byte(fileHeader)); err != nil {
		return fmt.Errorf("failed to send file header: %v", err)
	}

	// Inicjalizacja postępu
	progress := TransferProgress{
		FileName:   remoteFileName,
		TotalBytes: fileInfo.Size(),
		StartTime:  time.Now(),
	}

	// Kopiowanie danych z wykorzystaniem progress reader
	if progressChan != nil {
		reader := &ProgressReader{
			Reader:       localFile,
			Progress:     &progress,
			ProgressChan: progressChan,
		}
		_, err = io.Copy(writer, reader)
	} else {
		_, err = io.Copy(writer, localFile)
	}

	if err != nil {
		return fmt.Errorf("failed to copy file data: %v", err)
	}

	// Zakończenie transferu
	writer.Close()

	// Oczekiwanie na zakończenie sesji
	return session.Wait()
}

// DownloadFile pobiera plik z serwera
func (ft *FileTransfer) DownloadFile(remotePath, localPath string, progressChan chan<- TransferProgress) error {
	// Sprawdzenie czy połączenie jest aktywne
	if !ft.conn.IsConnected() {
		return fmt.Errorf("ssh connection is not active")
	}

	// Utworzenie nowej sesji
	session, err := ft.conn.GetClient().NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	// Przygotowanie pipe'a do odczytu danych
	reader, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	// Utworzenie lokalnego pliku
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close()

	// Przygotowanie komendy SCP
	scpCommand := fmt.Sprintf("scp -f %s", remotePath)

	// Uruchomienie komendy SCP
	if err := session.Start(scpCommand); err != nil {
		return fmt.Errorf("failed to start scp command: %v", err)
	}

	// Inicjalizacja postępu
	progress := TransferProgress{
		FileName:  filepath.Base(localPath),
		StartTime: time.Now(),
	}

	// Kopiowanie danych z wykorzystaniem progress reader
	if progressChan != nil {
		writer := &ProgressReader{
			Reader:       reader,
			Progress:     &progress,
			ProgressChan: progressChan,
		}
		_, err = io.Copy(localFile, writer)
	} else {
		_, err = io.Copy(localFile, reader)
	}

	if err != nil {
		return fmt.Errorf("failed to copy file data: %v", err)
	}

	// Oczekiwanie na zakończenie sesji
	return session.Wait()
}

// ProgressReader to wrapper do śledzenia postępu transferu
type ProgressReader struct {
	io.Reader
	Progress     *TransferProgress
	ProgressChan chan<- TransferProgress
}

// Read implementuje interfejs io.Reader i aktualizuje postęp
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Progress.TransferredBytes += int64(n)
		if pr.ProgressChan != nil {
			pr.ProgressChan <- *pr.Progress
		}
	}
	return n, err
}

// ListFiles zwraca listę plików w zdalnym katalogu
func (ft *FileTransfer) ListFiles(remotePath string) ([]string, error) {
	output, err := ft.conn.ExecuteCommand(fmt.Sprintf("ls -1 %s", remotePath))
	if err != nil {
		return nil, fmt.Errorf("failed to list remote files: %v", err)
	}

	// Konwersja output na slice stringów
	files := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Filtrowanie pustych linii
	var result []string
	for _, file := range files {
		if file != "" {
			result = append(result, file)
		}
	}

	return result, nil
}

// RemoteFileInfo reprezentuje informacje o zdalnym pliku
type RemoteFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

// Implementacja interfejsu os.FileInfo
func (fi *RemoteFileInfo) Name() string       { return fi.name }
func (fi *RemoteFileInfo) Size() int64        { return fi.size }
func (fi *RemoteFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *RemoteFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *RemoteFileInfo) IsDir() bool        { return fi.isDir }
func (fi *RemoteFileInfo) Sys() interface{}   { return nil }

// GetFileInfo zwraca informacje o zdalnym pliku
func (ft *FileTransfer) GetFileInfo(remotePath string) (os.FileInfo, error) {
	// Wykonujemy komendę stat aby uzyskać informacje o pliku
	cmd := fmt.Sprintf(`stat -c "%%n|%%s|%%f|%%Y|%%F" %s`, remotePath)
	output, err := ft.conn.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	// Parsowanie outputu
	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid stat output format")
	}

	// Parsowanie wielkości pliku
	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file size: %v", err)
	}

	// Parsowanie trybu pliku (format szesnastkowy)
	modeInt, err := strconv.ParseInt(parts[2], 16, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file mode: %v", err)
	}

	// Parsowanie czasu modyfikacji (unix timestamp)
	modTimeUnix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse modification time: %v", err)
	}

	// Sprawdzenie czy to katalog
	isDir := strings.Contains(parts[4], "directory")

	// Tworzenie obiektu FileInfo
	fileInfo := &RemoteFileInfo{
		name:    filepath.Base(parts[0]),
		size:    size,
		mode:    os.FileMode(modeInt),
		modTime: time.Unix(modTimeUnix, 0),
		isDir:   isDir,
	}

	return fileInfo, nil
}

// ValidateRemotePath sprawdza czy ścieżka zdalna istnieje i jest dostępna
func (ft *FileTransfer) ValidateRemotePath(remotePath string) error {
	// Sprawdzamy czy ścieżka istnieje
	_, err := ft.GetFileInfo(remotePath)
	if err != nil {
		return fmt.Errorf("remote path is not accessible: %v", err)
	}
	return nil
}

// CreateRemoteDirectory tworzy katalog zdalny
func (ft *FileTransfer) CreateRemoteDirectory(remotePath string) error {
	_, err := ft.conn.ExecuteCommand(fmt.Sprintf("mkdir -p %s", remotePath))
	if err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}
	return nil
}

// RemoveRemoteFile usuwa plik lub katalog zdalny
func (ft *FileTransfer) RemoveRemoteFile(remotePath string) error {
	// Sprawdzamy czy to katalog
	fileInfo, err := ft.GetFileInfo(remotePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	var cmd string
	if fileInfo.IsDir() {
		cmd = fmt.Sprintf("rm -rf %s", remotePath)
	} else {
		cmd = fmt.Sprintf("rm -f %s", remotePath)
	}

	_, err = ft.conn.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to remove remote file: %v", err)
	}
	return nil
}
