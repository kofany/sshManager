package views

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"sshManager/internal/ssh"
	"sshManager/internal/ui"
	"sshManager/internal/ui/components"

	"sshManager/internal/utils"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"
)

// Dodaj na początku pliku po importach
func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// Stałe określające tryby i stany
const (
	localPanelActive  = true
	remotePanelActive = false
	maxVisibleItems   = 20
	headerHeight      = 3
	footerHeight      = 4
)

// FileEntry reprezentuje pojedynczy plik lub katalog
type FileEntry struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
	mode    os.FileMode // Dodane pole

}

// Panel reprezentuje panel plików (lokalny lub zdalny)
type Panel struct {
	path          string
	entries       []FileEntry
	selectedIndex int
	scrollOffset  int
	active        bool
}

type transferProgressMsg ssh.TransferProgress

type transferFinishedMsg struct {
	err error
}

// transferView implementuje główny widok transferu plików
type transferView struct {
	model         *ui.Model
	localPanel    Panel
	remotePanel   Panel
	statusMessage string
	errorMessage  string
	connecting    bool
	connected     bool
	transferring  bool
	progress      ssh.TransferProgress
	showHelp      bool
	input         textinput.Model
	mutex         sync.Mutex
	width         int               // Dodane
	height        int               // Dodane
	escPressed    bool              // flaga wskazująca czy ESC został wciśnięty
	escTimeout    *time.Timer       // timer do resetowania stanu ESC
	popup         *components.Popup // Zmieniamy typ na nowy komponent

}
type connectionStatusMsg struct {
	connected bool
	err       error
}

func NewTransferView(model *ui.Model) *transferView {
	input := textinput.New()
	input.Placeholder = "Enter command..."
	input.CharLimit = 255

	v := &transferView{
		model: model,
		localPanel: Panel{
			path:   getHomeDir(),
			active: true,
			entries: []FileEntry{
				{name: "..", isDir: true},
			},
		},
		remotePanel: Panel{
			path:   "~/", // Tymczasowa wartość
			active: false,
			entries: []FileEntry{
				{name: "..", isDir: true},
			},
		},
		input:  input,
		width:  model.GetTerminalWidth(),
		height: model.GetTerminalHeight(),
	}

	// Inicjalizujemy panel lokalny
	if err := v.updateLocalPanel(); err != nil {
		v.errorMessage = fmt.Sprintf("Failed to load local directory: %v", err)
		return v
	}

	// Inicjujemy połączenie SFTP w tle
	if v.model.GetSelectedHost() != nil {
		go func() {
			// Attempt to establish connection
			err := v.ensureConnected()
			if err != nil {
				v.model.Program.Send(connectionStatusMsg{
					connected: false,
					err:       err,
				})
				return
			}

			// Pobierz katalog domowy i zaktualizuj ścieżkę
			transfer := v.model.GetTransfer()
			if homeDir, err := transfer.GetRemoteHomeDir(); err == nil {
				v.remotePanel.path = homeDir
			}

			// Update remote panel
			err = v.updateRemotePanel()
			if err != nil {
				v.model.Program.Send(connectionStatusMsg{
					connected: false,
					err:       err,
				})
				return
			}

			// Send success message
			v.model.Program.Send(connectionStatusMsg{
				connected: true,
				err:       nil,
			})
		}()
	}

	return v
}

// updateLocalPanel odświeża zawartość lokalnego panelu
func (v *transferView) updateLocalPanel() error {
	entries, err := v.readLocalDirectory(v.localPanel.path)
	if err != nil {
		return err
	}
	v.localPanel.entries = entries
	return nil
}

// readLocalDirectory reads and returns the content of a local directory
// with proper path normalization and sorting
func (v *transferView) readLocalDirectory(path string) ([]FileEntry, error) {
	// Normalize path for local system
	normalizedPath := utils.NormalizePath(path, false)

	dir, err := os.Open(normalizedPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	// Always start with ".." for navigation up
	entries := []FileEntry{{
		name:    "..",
		isDir:   true,
		modTime: time.Now(),
	}}

	for _, fi := range fileInfos {
		// Skip hidden files starting with "." (optional)
		// but always include ".." for navigation
		if !strings.HasPrefix(fi.Name(), ".") || fi.Name() == ".." {
			entries = append(entries, FileEntry{
				name:    fi.Name(), // Keep original name for display
				size:    fi.Size(),
				modTime: fi.ModTime(),
				isDir:   fi.IsDir(),
				mode:    fi.Mode(),
			})
		}
	}

	// Sort: directories first, then files, alphabetically
	sort.Slice(entries[1:], func(i, j int) bool {
		// Adjust indices to skip ".."
		i, j = i+1, j+1
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	return entries, nil
}

// Init initializes the transfer view and starts connection if needed
func (v *transferView) Init() tea.Cmd {
	if !v.connected && !v.connecting && v.model.GetSelectedHost() != nil {
		v.connecting = true
		return v.sendConnectionUpdate()
	}
	return nil
}

// updateRemotePanel refreshes the content of the remote panel
func (v *transferView) updateRemotePanel() error {
	if err := v.ensureConnected(); err != nil {
		return err
	}

	// Normalize remote panel path
	normalizedPath := utils.NormalizePath(v.remotePanel.path, true)

	entries, err := v.readRemoteDirectory(normalizedPath)
	if err != nil {
		v.setConnected(false) // Mark as disconnected in case of error
		return err
	}

	v.remotePanel.entries = entries
	return nil
}

// readRemoteDirectory reads and returns the content of a remote directory
// with proper path normalization and sorting
func (v *transferView) readRemoteDirectory(path string) ([]FileEntry, error) {
	if err := v.ensureConnected(); err != nil {
		return nil, err
	}

	// Normalize path for remote system
	normalizedPath := utils.NormalizePath(path, true)

	transfer := v.model.GetTransfer()
	fileInfos, err := transfer.ListRemoteFiles(normalizedPath)
	if err != nil {
		v.setConnected(false)
		return nil, fmt.Errorf("failed to list remote directory: %v", err)
	}

	// Always start with ".." for navigation up
	entries := []FileEntry{{
		name:    "..",
		isDir:   true,
		modTime: time.Now(),
	}}

	for _, fi := range fileInfos {
		if !strings.HasPrefix(fi.Name(), ".") || fi.Name() == ".." {
			entries = append(entries, FileEntry{
				name:    fi.Name(), // Keep original name for display
				size:    fi.Size(),
				modTime: fi.ModTime(),
				isDir:   fi.IsDir(),
				mode:    fi.Mode(),
			})
		}
	}

	// Sort: directories first, then files, alphabetically
	sort.Slice(entries[1:], func(i, j int) bool {
		i, j = i+1, j+1
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	return entries, nil
}

// getActivePanel returns the currently active panel (local or remote)
// Used for determining the source panel in file operations
func (v *transferView) getActivePanel() *Panel {
	if v.localPanel.active {
		return &v.localPanel
	}
	return &v.remotePanel
}

// getInactivePanel returns the currently inactive panel (local or remote)
// Used for determining the destination panel in file operations
func (v *transferView) getInactivePanel() *Panel {
	if v.localPanel.active {
		return &v.remotePanel
	}
	return &v.localPanel
}

// switchActivePanel toggles the active state between local and remote panels
// This method ensures that exactly one panel is active at any time
func (v *transferView) switchActivePanel() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.localPanel.active = !v.localPanel.active
	v.remotePanel.active = !v.remotePanel.active
}

// renderPanel renders a single panel (either local or remote) with proper formatting
// It handles path display, file listing, and scroll information
func (v *transferView) renderPanel(p *Panel) string {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	var content strings.Builder

	// Calculate panel width based on terminal size
	panelWidth := (min(v.width-40, 160) - 3) / 2

	// Prepare panel content with proper styling
	var panelContent strings.Builder

	// Format and shorten the path for display
	// For remote panel, ensure the path is displayed in Unix style
	displayPath := utils.NormalizePath(p.path, p == &v.remotePanel)
	pathText := formatPath(displayPath, min(40, panelWidth-5))

	// Apply appropriate path style based on panel state
	pathStyle := inactivePathStyle
	if p.active {
		pathStyle = activePathStyle
	}
	panelContent.WriteString(pathStyle.Render(pathText))
	panelContent.WriteString("\n")

	// Render file list with proper scrolling
	filesList := v.renderFileList(
		p.entries[p.scrollOffset:min(p.scrollOffset+maxVisibleItems, len(p.entries))],
		p.selectedIndex-p.scrollOffset,
		p.active,
		panelWidth-2,
	)
	panelContent.WriteString(filesList)

	// Add scroll information if needed
	if len(p.entries) > maxVisibleItems {
		panelContent.WriteString(fmt.Sprintf("\nShowing %d-%d of %d items",
			p.scrollOffset+1,
			min(p.scrollOffset+maxVisibleItems, len(p.entries)),
			len(p.entries)))
	}

	// Apply final panel styling with border
	content.WriteString(panelStyle.
		Width(panelWidth).
		BorderForeground(ui.Subtle).
		Render(panelContent.String()))

	return content.String()
}

// View renders the complete transfer view including panels, status information,
// progress bars, and popups. This is the main rendering function called by the TUI framework.
func (v *transferView) View() string {
	var content strings.Builder

	// Render title and connection status
	titleContent := ui.TitleStyle.Render("File Transfer")
	if v.connected {
		if host := v.model.GetSelectedHost(); host != nil {
			titleContent += ui.SuccessStyle.Render(
				fmt.Sprintf(" - Connected to %s (%s)", host.Name, host.IP),
			)
		}
	} else if host := v.model.GetSelectedHost(); host != nil {
		if v.connecting {
			titleContent += ui.DescriptionStyle.Render(" - Establishing connection...")
		} else {
			titleContent += ui.ErrorStyle.Render(
				fmt.Sprintf(" - Not connected to %s (%s)", host.Name, host.IP),
			)
		}
	}
	content.WriteString(titleContent + "\n\n")

	// Show connecting status screen
	if v.connecting {
		connectingContent := ui.DescriptionStyle.Render("Establishing SFTP connection...")
		return lipgloss.Place(
			v.width,
			v.height,
			lipgloss.Center,
			lipgloss.Center,
			ui.WindowStyle.Render(connectingContent),
		)
	}

	// Show help screen if requested
	if v.showHelp {
		helpContent := ui.DescriptionStyle.Render(helpText)
		return lipgloss.Place(
			v.width,
			v.height,
			lipgloss.Center,
			lipgloss.Center,
			ui.WindowStyle.Render(helpContent),
		)
	}

	// Calculate panel dimensions
	totalWidth := min(v.width-40, 160) // Reduce width for margins
	panelWidth := (totalWidth - 3) / 2 // Account for separator width

	// Render panels
	leftPanel := v.renderPanel(&v.localPanel)
	rightPanel := ""
	if !v.connected {
		rightPanel = ui.ErrorStyle.Render("\n  No SFTP Connection\n  Press 'q' to return and connect to a host first.")
	} else {
		rightPanel = v.renderPanel(&v.remotePanel)
	}

	// Align panels by padding with spaces
	leftLines := strings.Split(leftPanel, "\n")
	rightLines := strings.Split(rightPanel, "\n")
	maxLines := max(len(leftLines), len(rightLines))

	// Ensure both panels have the same number of lines
	for i := len(leftLines); i < maxLines; i++ {
		leftLines = append(leftLines, strings.Repeat(" ", panelWidth))
	}
	for i := len(rightLines); i < maxLines; i++ {
		rightLines = append(rightLines, strings.Repeat(" ", panelWidth))
	}

	// Join panels with separator
	for i := 0; i < maxLines; i++ {
		content.WriteString(leftLines[i])
		content.WriteString(" │ ")
		content.WriteString(rightLines[i])
		content.WriteString("\n")
	}

	// Add progress bar during transfer
	if v.transferring {
		content.WriteString("\n")
		progressBar := v.formatProgressBar(totalWidth)
		content.WriteString(ui.DescriptionStyle.Render(progressBar))
	}

	// Add input field if waiting for user input
	if v.isWaitingForInput() {
		content.WriteString("\n" + v.input.View())
	}

	// Add footer
	footer := v.renderFooter()
	content.WriteString("\n")
	content.WriteString(footer)

	// Wrap content in window style
	finalContent := ui.WindowStyle.Render(content.String())

	// If popup is active, render it on top
	if v.popup != nil {
		return lipgloss.Place(
			v.width,
			v.height,
			lipgloss.Center,
			lipgloss.Center,
			finalContent+"\n"+v.popup.Render(),
			lipgloss.WithWhitespaceChars(""),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
		)
	}

	// Render main view aligned to top-left
	return lipgloss.Place(
		v.width,
		v.height,
		lipgloss.Left,
		lipgloss.Top,
		finalContent,
		lipgloss.WithWhitespaceChars(""),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatSize formats a file size in bytes to a human-readable string
// using appropriate units (B, KB, MB, GB, TB, PB, EB)
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(size)/float64(div), "KMGTPE"[exp])
}

// navigatePanel handles panel navigation and scroll adjustment
// It ensures proper wrapping and scroll position
func (v *transferView) navigatePanel(p *Panel, direction int) {
	if len(p.entries) == 0 {
		p.selectedIndex = 0
		p.scrollOffset = 0
		return
	}

	// Calculate new index with wrapping
	newIndex := p.selectedIndex + direction
	if newIndex < 0 {
		newIndex = len(p.entries) - 1
	} else if newIndex >= len(p.entries) {
		newIndex = 0
	}
	p.selectedIndex = newIndex

	// Adjust scroll position
	if p.selectedIndex < p.scrollOffset {
		p.scrollOffset = p.selectedIndex
	} else if p.selectedIndex >= p.scrollOffset+maxVisibleItems {
		p.scrollOffset = p.selectedIndex - maxVisibleItems + 1
	}

	// Ensure scrollOffset is never negative
	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}
}

// enterDirectory changes the current directory in the specified panel
// It handles both local and remote directory navigation
func (v *transferView) enterDirectory(p *Panel) error {
	if len(p.entries) == 0 || p.selectedIndex >= len(p.entries) {
		return nil
	}

	entry := p.entries[p.selectedIndex]
	if !entry.isDir {
		return nil
	}

	var newPath string
	isRemote := p == &v.remotePanel

	if entry.name == ".." {
		// Handle parent directory navigation
		currentPath := utils.NormalizePath(p.path, isRemote)
		if isRemote {
			newPath = filepath.ToSlash(filepath.Dir(currentPath))
		} else {
			newPath = filepath.Dir(currentPath)
			// Special handling for Windows root directories
			if runtime.GOOS == "windows" && filepath.Dir(newPath) == newPath {
				newPath = filepath.VolumeName(newPath) + string(filepath.Separator)
			}
		}
	} else {
		// Handle entering a subdirectory
		newPath = filepath.Join(p.path, entry.name)
		newPath = utils.NormalizePath(newPath, isRemote)
	}

	// Save current path for rollback
	oldPath := p.path
	p.path = newPath

	// Try to refresh directory content
	var err error
	if isRemote {
		err = v.updateRemotePanel()
	} else {
		err = v.updateLocalPanel()
	}

	// Rollback on error
	if err != nil {
		p.path = oldPath
		return fmt.Errorf("failed to enter directory: %v", err)
	}

	// Reset selection and scroll position
	p.selectedIndex = 0
	p.scrollOffset = 0
	return nil
}

// hasSelectedItems checks if any items are currently selected in the file transfer view.
// Returns true if at least one item is selected, false otherwise.
func (v *transferView) hasSelectedItems() bool {
	for _, isSelected := range v.getSelectedItems() {
		if isSelected {
			return true
		}
	}
	return false
}

// getSelectedItems returns a map of selected file paths.
// The map keys are file paths and values are always true for selected items.
// This method relies on the model's GetSelectedPaths method to get the actual selection data.
func (v *transferView) getSelectedItems() map[string]bool {
	selected := make(map[string]bool)
	paths := v.model.GetSelectedPaths()
	for _, path := range paths {
		selected[path] = true
	}
	return selected
}

// copyFile handles the file transfer operation between local and remote systems.
// It supports both single file/directory transfer and multiple selected items transfer.
// The function uses proper path normalization for both Windows and Unix systems.
// It handles:
// - Single file/directory transfer when nothing is selected
// - Multiple selected items transfer
// - Different path formats for local and remote systems
// - Progress monitoring and error handling
func (v *transferView) copyFile() tea.Cmd {
	// Lock to safely access panels
	v.mutex.Lock()
	srcPanel := v.getActivePanel()
	dstPanel := v.getInactivePanel()
	v.mutex.Unlock()

	// Structure to hold items that need to be copied
	var itemsToCopy []struct {
		srcPath string
		dstPath string
		isDir   bool
	}

	// Single item copy logic
	if !v.hasSelectedItems() {
		v.mutex.Lock()
		if len(srcPanel.entries) == 0 || srcPanel.selectedIndex >= len(srcPanel.entries) {
			v.mutex.Unlock()
			v.handleError(fmt.Errorf("no file selected"))
			return nil
		}

		entry := srcPanel.entries[srcPanel.selectedIndex]
		fileName := entry.name
		isUpload := srcPanel == &v.localPanel
		v.mutex.Unlock()

		// Prevent copying parent directory reference
		if fileName == ".." {
			v.handleError(fmt.Errorf("cannot copy parent directory reference"))
			return nil
		}

		srcPath, dstPath := v.buildPaths(srcPanel, dstPanel, fileName, isUpload)
		itemsToCopy = append(itemsToCopy, struct {
			srcPath string
			dstPath string
			isDir   bool
		}{srcPath, dstPath, entry.isDir})
	} else {
		// Multiple items copy logic
		for path, isSelected := range v.getSelectedItems() {
			if !isSelected {
				continue
			}

			baseName := filepath.Base(path)
			if baseName == ".." {
				continue
			}

			isUpload := srcPanel == &v.localPanel
			srcPath, dstPath := v.buildPaths(srcPanel, dstPanel, baseName, isUpload)

			isDir := v.determineItemType(srcPanel, path, baseName)
			if isDir == nil {
				continue // Error already handled
			}

			itemsToCopy = append(itemsToCopy, struct {
				srcPath string
				dstPath string
				isDir   bool
			}{srcPath, dstPath, *isDir})
		}
	}

	// Verify we have items to copy
	if len(itemsToCopy) == 0 {
		v.handleError(fmt.Errorf("no items to copy"))
		return nil
	}

	// Update transfer status
	v.mutex.Lock()
	v.transferring = true
	v.statusMessage = "Copying files..."
	v.mutex.Unlock()

	transfer := v.model.GetTransfer()

	return v.createTransferCommand(srcPanel, transfer, itemsToCopy)
}

// buildPaths constructs source and destination paths for file transfer.
// It handles path normalization for both local and remote systems,
// including special handling for UNC paths in Windows.
// Parameters:
//   - srcPanel: source panel (local or remote)
//   - dstPanel: destination panel (local or remote)
//   - fileName: name of the file to transfer
//   - isUpload: true if transferring from local to remote, false otherwise
func (v *transferView) buildPaths(srcPanel, dstPanel *Panel, fileName string, isUpload bool) (string, string) {
	if isUpload {
		// Local to remote transfer
		srcPath := filepath.Join(srcPanel.path, fileName)
		if utils.IsUNCPath(srcPath) {
			srcPath = utils.PreserveUNCPath(srcPath)
		}
		dstPath := utils.NormalizePath(filepath.Join(dstPanel.path, fileName), true)
		return srcPath, dstPath
	}
	// Remote to local transfer
	srcPath := utils.NormalizePath(filepath.Join(srcPanel.path, fileName), true)
	dstPath := filepath.Join(dstPanel.path, fileName)
	if utils.IsUNCPath(dstPath) {
		dstPath = utils.PreserveUNCPath(dstPath)
	} else {
		dstPath = utils.NormalizePath(dstPath, false)
	}
	return srcPath, dstPath
}

// determineItemType checks if the given path represents a directory.
// For local items, it uses os.Stat.
// For remote items, it searches through the panel entries.
// Returns a pointer to bool (true for directory, false for file) or nil if an error occurs.
func (v *transferView) determineItemType(srcPanel *Panel, path, baseName string) *bool {
	var isDir bool
	if srcPanel == &v.localPanel {
		info, err := os.Stat(path)
		if err != nil {
			v.handleError(fmt.Errorf("cannot access %s: %v", path, err))
			return nil
		}
		isDir = info.IsDir()
	} else {
		found := false
		for _, entry := range srcPanel.entries {
			if entry.name == baseName {
				isDir = entry.isDir
				found = true
				break
			}
		}
		if !found {
			v.handleError(fmt.Errorf("cannot find %s in remote directory", baseName))
			return nil
		}
	}
	return &isDir
}

// createTransferCommand creates a tea.Cmd that handles the file transfer process.
// It sets up the progress monitoring channels and starts the transfer goroutines.
func (v *transferView) createTransferCommand(srcPanel *Panel, transfer *ssh.FileTransfer, itemsToCopy []struct {
	srcPath string
	dstPath string
	isDir   bool
}) tea.Cmd {
	return func() tea.Msg {
		progressChan := make(chan ssh.TransferProgress)
		doneChan := make(chan error, 1)

		go v.handleTransferProcess(srcPanel, transfer, itemsToCopy, progressChan, doneChan)
		go v.monitorTransferProgress(progressChan, doneChan)

		return nil
	}
}

// handleTransferProcess manages the actual file transfer operation.
// It processes each item in the transfer queue and reports progress.
// This function runs in its own goroutine.
func (v *transferView) handleTransferProcess(srcPanel *Panel, transfer *ssh.FileTransfer, itemsToCopy []struct {
	srcPath string
	dstPath string
	isDir   bool
}, progressChan chan<- ssh.TransferProgress, doneChan chan<- error) {
	var totalErr error
	for _, item := range itemsToCopy {
		var err error
		if item.isDir {
			if srcPanel == &v.localPanel {
				err = v.copyDirectoryToRemote(item.srcPath, item.dstPath, transfer, progressChan)
			} else {
				err = v.copyDirectoryFromRemote(item.srcPath, item.dstPath, transfer, progressChan)
			}
		} else {
			if srcPanel == &v.localPanel {
				err = transfer.UploadFile(item.srcPath, item.dstPath, progressChan)
			} else {
				err = transfer.DownloadFile(item.srcPath, item.dstPath, progressChan)
			}
		}
		if err != nil {
			totalErr = fmt.Errorf("error copying %s: %v", item.srcPath, err)
			break
		}
	}
	doneChan <- totalErr
	close(progressChan)
}

// monitorTransferProgress handles progress updates and completion status
// of the file transfer operation. This function runs in its own goroutine.
func (v *transferView) monitorTransferProgress(progressChan <-chan ssh.TransferProgress, doneChan <-chan error) {
	for progress := range progressChan {
		v.model.Program.Send(transferProgressMsg(progress))
	}
	err := <-doneChan
	v.model.Program.Send(transferFinishedMsg{err: err})
	v.model.ClearSelection()
}

// copyDirectoryToRemote recursively copies a local directory to the remote system.
// It handles path normalization and maintains the directory structure.
// Parameters:
//   - localPath: source path on local system
//   - remotePath: destination path on remote system
//   - transfer: SFTP transfer client
//   - progressChan: channel for reporting transfer progress
func (v *transferView) copyDirectoryToRemote(localPath, remotePath string, transfer *ssh.FileTransfer, progressChan chan<- ssh.TransferProgress) error {
	// Normalize remote path for SFTP
	remotePath = utils.NormalizePath(remotePath, true)
	if err := transfer.CreateRemoteDirectory(remotePath); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	return filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get path relative to source directory
		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}

		// Build and normalize remote path
		remotePathFull := utils.NormalizePath(filepath.Join(remotePath, relPath), true)

		if info.IsDir() {
			return transfer.CreateRemoteDirectory(remotePathFull)
		}

		return transfer.UploadFile(path, remotePathFull, progressChan)
	})
}

// copyDirectoryFromRemote recursively copies a remote directory to the local system.
// It creates the necessary local directory structure and copies all files.
// Parameters:
//   - remotePath: source path on remote system
//   - localPath: destination path on local system
//   - transfer: SFTP transfer client
//   - progressChan: channel for reporting transfer progress
func (v *transferView) copyDirectoryFromRemote(remotePath, localPath string, transfer *ssh.FileTransfer, progressChan chan<- ssh.TransferProgress) error {
	// Create local directory structure
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Normalize remote path for SFTP
	remotePath = utils.NormalizePath(remotePath, true)
	entries, err := transfer.ListRemoteFiles(remotePath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	for _, entry := range entries {
		// Skip special directories
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}

		remoteSrcPath := utils.NormalizePath(filepath.Join(remotePath, entry.Name()), true)
		localDstPath := utils.NormalizePath(filepath.Join(localPath, entry.Name()), false)

		if entry.IsDir() {
			if err := v.copyDirectoryFromRemote(remoteSrcPath, localDstPath, transfer, progressChan); err != nil {
				return fmt.Errorf("failed to copy remote directory %s: %v", entry.Name(), err)
			}
		} else {
			if err := transfer.DownloadFile(remoteSrcPath, localDstPath, progressChan); err != nil {
				return fmt.Errorf("failed to download file %s: %v", entry.Name(), err)
			}
		}
	}

	return nil
}

// executeDelete handles file or directory deletion on both local and remote systems.
// It supports recursive deletion for directories and handles path normalization.
// After successful deletion, it refreshes the panel view.
func (v *transferView) executeDelete() error {
	v.mutex.Lock()
	panel := v.getActivePanel()
	entry := panel.entries[panel.selectedIndex]
	v.mutex.Unlock()

	// Build and normalize path based on system type
	isRemote := panel == &v.remotePanel
	path := utils.NormalizePath(filepath.Join(panel.path, entry.name), isRemote)

	itemType := "file"
	if entry.isDir {
		itemType = "directory"
	}

	var err error
	if panel == &v.localPanel {
		// Local deletion
		if entry.isDir {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
	} else {
		// Remote deletion
		transfer := v.model.GetTransfer()
		if entry.isDir {
			err = v.removeRemoteDirectory(path, transfer)
		} else {
			err = transfer.RemoveRemoteFile(path)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to delete %s '%s': %v", itemType, entry.name, err)
	}

	// Refresh panel after deletion
	if panel == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	if err != nil {
		return fmt.Errorf("failed to refresh panel: %v", err)
	}

	v.statusMessage = fmt.Sprintf("Deleted %s '%s'", itemType, entry.name)
	return nil
}

// removeRemoteDirectory recursively removes a directory on the remote system.
// It first removes all files and subdirectories before removing the directory itself.
// Parameters:
//   - path: path to the remote directory to remove
//   - transfer: SFTP transfer client for remote operations
func (v *transferView) removeRemoteDirectory(path string, transfer *ssh.FileTransfer) error {
	// Normalize remote path and get directory contents
	normalizedPath := utils.NormalizePath(path, true)
	entries, err := transfer.ListRemoteFiles(normalizedPath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	// Recursively remove directory contents
	for _, entry := range entries {
		// Skip special directory entries
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}

		// Build and normalize full path for each entry
		fullPath := utils.NormalizePath(filepath.Join(normalizedPath, entry.Name()), true)

		if entry.IsDir() {
			// Recursively remove subdirectory
			if err := v.removeRemoteDirectory(fullPath, transfer); err != nil {
				return err
			}
		} else {
			// Remove regular file
			if err := transfer.RemoveRemoteFile(fullPath); err != nil {
				return err
			}
		}
	}

	// Finally, remove the empty directory itself
	return transfer.RemoveRemoteFile(normalizedPath)
}

// createDirectory creates a new directory in either local or remote system.
// It validates the directory name, creates the directory, and refreshes the panel view.
// Parameters:
//   - name: name of the directory to create (without path)
func (v *transferView) createDirectory(name string) error {
	// Validate directory name
	if name == "" {
		return fmt.Errorf("directory name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("directory name cannot contain path separators")
	}

	// Get active panel and build path
	panel := v.getActivePanel()
	isRemote := panel == &v.remotePanel
	path := utils.NormalizePath(filepath.Join(panel.path, name), isRemote)

	var err error
	if panel == &v.localPanel {
		// Create local directory
		err = os.MkdirAll(path, 0755)
	} else {
		// Create remote directory
		if !v.connected {
			return fmt.Errorf("not connected to remote host")
		}
		transfer := v.model.GetTransfer()
		err = transfer.CreateRemoteDirectory(path)
	}

	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Refresh panel to show new directory
	if panel == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	if err != nil {
		return fmt.Errorf("failed to refresh panel: %v", err)
	}

	v.statusMessage = fmt.Sprintf("Created directory '%s'", name)
	return nil
}

// renameFile renames a file or directory in either local or remote system.
// It validates the new name, performs the rename operation, and refreshes the panel view.
// Parameters:
//   - newName: new name for the file/directory (without path)
func (v *transferView) renameFile(newName string) error {
	// Validate new name
	if newName == "" {
		return fmt.Errorf("new name cannot be empty")
	}

	// Get active panel and current selection
	panel := v.getActivePanel()
	if len(panel.entries) == 0 || panel.selectedIndex >= len(panel.entries) {
		return fmt.Errorf("no file selected")
	}

	entry := panel.entries[panel.selectedIndex]
	if entry.name == ".." {
		return fmt.Errorf("cannot rename parent directory reference")
	}

	// Build and normalize paths
	isRemote := panel == &v.remotePanel
	oldPath := utils.NormalizePath(filepath.Join(panel.path, entry.name), isRemote)
	newPath := utils.NormalizePath(filepath.Join(panel.path, newName), isRemote)

	var err error
	if panel == &v.localPanel {
		// Rename local file/directory
		err = os.Rename(oldPath, newPath)
	} else {
		// Rename remote file/directory
		transfer := v.model.GetTransfer()
		err = transfer.RenameRemoteFile(oldPath, newPath)
	}

	if err != nil {
		return fmt.Errorf("failed to rename file: %v", err)
	}

	// Refresh panel to show the renamed file/directory
	if panel == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	if err != nil {
		return fmt.Errorf("failed to refresh panel: %v", err)
	}

	v.statusMessage = fmt.Sprintf("Renamed %s to %s", entry.name, newName)
	return nil
}

// handleError error handling function
func (v *transferView) handleError(err error) {
	if err != nil {
		v.errorMessage = err.Error()
	}
}

// Update handles all UI messages and user interactions in the transfer view.
// It manages file operations, navigation, and view state changes.
// Returns updated model and any commands to be executed.
func (v *transferView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle window size changes
	case tea.WindowSizeMsg:
		v.mutex.Lock()
		v.width = msg.Width
		v.height = msg.Height
		v.model.UpdateWindowSize(msg.Width, msg.Height)
		v.mutex.Unlock()
		return v, nil

	// Handle file transfer progress updates
	case transferProgressMsg:
		v.mutex.Lock()
		v.progress = ssh.TransferProgress(msg)
		v.mutex.Unlock()
		return v, nil

	// Handle transfer completion status
	case transferFinishedMsg:
		v.mutex.Lock()
		v.transferring = false
		if msg.err != nil {
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Transfer Error",
				fmt.Sprintf("Transfer error: %v", msg.err),
				50, 7, v.width, v.height,
			)
		} else {
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Success",
				"Transfer completed successfully",
				50, 7, v.width, v.height,
			)
			// Refresh destination panel after successful transfer
			dstPanel := v.getInactivePanel()
			if dstPanel == &v.localPanel {
				v.updateLocalPanel()
			} else {
				v.updateRemotePanel()
			}
		}
		v.mutex.Unlock()
		return v, nil

	// Handle connection status updates
	case connectionStatusMsg:
		v.mutex.Lock()
		v.connecting = false
		if msg.err != nil {
			v.connected = false
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Connection Error",
				fmt.Sprintf("Connection error: %v", msg.err),
				50, 7, v.width, v.height,
			)
		} else {
			v.connected = msg.connected
		}
		v.mutex.Unlock()
		return v, nil

	// Handle keyboard input
	case tea.KeyMsg:
		// Handle popup interactions
		if v.popup != nil {
			return v.handlePopupInput(msg)
		}

		// Handle help mode
		if v.showHelp {
			return v.handleHelpModeInput(msg)
		}

		// Handle ESC sequence
		if v.escPressed {
			return v.handleEscSequence(msg)
		}

		// Handle single ESC press
		if msg.String() == "esc" {
			return v.handleSingleEsc()
		}
		// Handle standard function keys and navigation
		switch msg.String() {
		case " ": // Theme switching
			if !v.transferring {
				ui.SwitchTheme()
				return v, nil
			}

		case "f1": // Help toggle
			v.showHelp = !v.showHelp
			return v, nil

		case "f5", "c": // Copy operation
			if !v.transferring {
				cmd := v.copyFile()
				return v, cmd
			}
			return v, nil

		case "f6", "r": // Rename operation
			if !v.transferring {
				v.popup = components.NewPopup(
					components.PopupRename,
					"Rename",
					"Enter new name:",
					50, 7, v.width, v.height,
				)
				v.popup.Input.SetValue("")
				v.popup.Input.Focus()
			}
			return v, nil

		case "f7", "m": // Create directory operation
			if !v.transferring {
				v.popup = components.NewPopup(
					components.PopupMkdir,
					"Create Directory",
					"Enter directory name:",
					50, 7, v.width, v.height,
				)
				v.popup.Input.SetValue("")
				v.popup.Input.Focus()
			}
			return v, nil

		case "f8", "d": // Delete operation
			if !v.transferring {
				return v.handleDeleteRequest()
			}
			return v, nil

		// Navigation and control keys
		case "q": // Quit/return to main view
			if v.transferring {
				return v, nil
			}
			if v.connected {
				transfer := v.model.GetTransfer()
				if transfer != nil {
					transfer.Disconnect()
				}
			}
			v.model.SetActiveView(ui.ViewMain)
			return v, nil

		case "tab": // Switch active panel
			if v.connected {
				v.switchActivePanel()
				v.errorMessage = ""
			}
			return v, nil

		case "up", "w": // Navigate up
			panel := v.getActivePanel()
			v.navigatePanel(panel, -1)
			v.errorMessage = ""
			return v, nil

		case "down", "s": // Navigate down
			panel := v.getActivePanel()
			v.navigatePanel(panel, 1)
			v.errorMessage = ""
			return v, nil

		case "enter": // Enter directory
			panel := v.getActivePanel()
			if err := v.enterDirectory(panel); err != nil {
				v.popup = components.NewPopup(
					components.PopupMessage,
					"Error",
					err.Error(),
					50, 7, v.width, v.height,
				)
			}
			return v, nil

		case "x": // Toggle selection
			if !v.transferring {
				panel := v.getActivePanel()
				if len(panel.entries) > 0 && panel.selectedIndex < len(panel.entries) {
					entry := panel.entries[panel.selectedIndex]
					if entry.name != ".." {
						path := filepath.Join(panel.path, entry.name)
						v.model.ToggleSelection(path)
					}
				}
			}
			return v, nil
		}

	// Handle direct transfer progress updates
	case ssh.TransferProgress:
		v.progress = msg
		return v, nil
	}

	return v, nil
}

// Helper method to handle delete request
func (v *transferView) handleDeleteRequest() (tea.Model, tea.Cmd) {
	panel := v.getActivePanel()
	if len(panel.entries) == 0 || panel.selectedIndex >= len(panel.entries) {
		return v, nil
	}
	entry := panel.entries[panel.selectedIndex]
	if entry.name == ".." {
		return v, nil
	}
	v.popup = components.NewPopup(
		components.PopupDelete,
		"Delete",
		fmt.Sprintf("Delete %s '%s'? (y/n)",
			map[bool]string{true: "directory", false: "file"}[entry.isDir],
			entry.name),
		50, 7, v.width, v.height,
	)
	return v, nil
}

// Helper method to handle popup input
func (v *transferView) handlePopupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.popup = nil
		return v, nil
	case "enter":
		if v.popup.Type != components.PopupDelete {
			if err := v.handleCommand(v.popup.Input.Value()); err != nil {
				v.handleError(err)
			}
			v.popup = nil
			return v, nil
		}
	case "y":
		if v.popup.Type == components.PopupDelete {
			if err := v.executeDelete(); err != nil {
				v.handleError(err)
			}
			v.popup = nil
			return v, nil
		}
	case "n":
		if v.popup.Type == components.PopupDelete {
			v.popup = nil
			return v, nil
		}
	default:
		if v.popup.Type != components.PopupDelete {
			var cmd tea.Cmd
			v.popup.Input, cmd = v.popup.Input.Update(msg)
			return v, cmd
		}
	}
	return v, nil
}

// handleHelpModeInput handles keyboard input while in help mode
func (v *transferView) handleHelpModeInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "f1":
		v.showHelp = false
		return v, nil
	default:
		return v, nil // Ignore other keys in help mode
	}
}

// handleEscSequence handles ESC key sequence operations
func (v *transferView) handleEscSequence(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "0", "q":
		if v.transferring {
			return v, nil
		}
		if v.connected {
			transfer := v.model.GetTransfer()
			if transfer != nil {
				transfer.Disconnect()
			}
		}
		v.model.SetActiveView(ui.ViewMain)
		return v, nil

	case "5": // Copy
		if !v.transferring {
			cmd := v.copyFile()
			v.escPressed = false
			if v.escTimeout != nil {
				v.escTimeout.Stop()
			}
			return v, cmd
		}

	case "6": // Rename
		if !v.transferring {
			v.popup = components.NewPopup(
				components.PopupRename,
				"Rename",
				"Enter new name:",
				50,
				7,
				v.width,
				v.height,
			)
			v.popup.Input.SetValue("")
			v.popup.Input.Focus()
		}
		return v, nil

	case "7": // Create Directory
		if !v.transferring {
			v.popup = components.NewPopup(
				components.PopupMkdir,
				"Create Directory",
				"Enter directory name:",
				50,
				7,
				v.width,
				v.height,
			)
			v.popup.Input.SetValue("")
			v.popup.Input.Focus()
		}
		return v, nil

	case "8": // Delete
		if !v.transferring {
			return v.handleDeleteRequest()
		}
	}

	// Reset ESC state
	v.escPressed = false
	if v.escTimeout != nil {
		v.escTimeout.Stop()
	}
	return v, nil
}

// handleSingleEsc handles single ESC key press operations
func (v *transferView) handleSingleEsc() (tea.Model, tea.Cmd) {
	if v.popup != nil {
		v.popup = nil
		return v, nil
	}

	v.escPressed = true
	if v.escTimeout != nil {
		v.escTimeout.Stop()
	}

	v.escTimeout = time.NewTimer(500 * time.Millisecond)
	go func() {
		<-v.escTimeout.C
		v.escPressed = false
	}()

	return v, nil
}

// ----------------------
// handleCommand obsługuje wprowadzanie komend
func (v *transferView) handleCommand(cmd string) error {
	if v.popup == nil {
		return fmt.Errorf("no active popup")
	}

	switch v.popup.Type { // użycie Type zamiast promptType
	case components.PopupRename: // użycie components.PopupRename zamiast promptRename
		err := v.renameFile(cmd)
		v.popup = nil
		return err
	case components.PopupMkdir: // użycie components.PopupMkdir zamiast promptMkdir
		err := v.createDirectory(cmd)
		v.popup = nil
		return err
	default:
		v.popup = nil
		return fmt.Errorf("unknown command")
	}
}

// internal/ui/views/transfer.go

func (v *transferView) formatProgressBar(width int) string {
	if !v.transferring || v.progress.TotalBytes == 0 {
		return ""
	}

	percentage := float64(v.progress.TransferredBytes) / float64(v.progress.TotalBytes)
	barWidth := width - 30 // Zostaw miejsce na procenty i prędkość
	completedWidth := int(float64(barWidth) * percentage)

	bar := fmt.Sprintf("[%s%s] %3.0f%%",
		strings.Repeat("=", completedWidth),
		strings.Repeat(" ", barWidth-completedWidth),
		percentage*100)

	elapsed := time.Since(v.progress.StartTime).Seconds()
	if elapsed == 0 {
		elapsed = 1 // Zapobieganie dzieleniu przez zero
	}
	speed := float64(v.progress.TransferredBytes) / elapsed

	return fmt.Sprintf("%s %s %s/s",
		v.progress.FileName,
		bar,
		formatSize(int64(speed)))
}

// shouldShowDeleteConfirm sprawdza czy wyświetlić potwierdzenie usunięcia
func (v *transferView) shouldShowDeleteConfirm() bool {
	return strings.HasPrefix(v.statusMessage, "Delete ")
}

// isWaitingForInput sprawdza czy oczekuje na wprowadzenie tekstu
func (v *transferView) isWaitingForInput() bool {
	return strings.HasPrefix(v.statusMessage, "Enter ")
}

var helpText = `
 File Transfer Help
 -----------------
 Tab          - Switch panel
 Enter        - Enter directory
 F5/ESC+5/c   - Copy file
 F6/ESC+6/r   - Rename
 F7/ESC+7/m   - Create directory
 F8/ESC+8/d   - Delete
 F1           - Toggle help
 Ctrl+r       - Refresh
 q/ESC+0      - Exit
 x            - Select/Unselect file

 Navigation
 ----------
 Up/w         - Move up
 Down/s       - Move down
`

func (v *transferView) renderShortcuts() string {
	// Nagłówki tabeli i skróty
	headers := []string{"Switch Panel", "Select", "Copy", "Rename", "MkDir", "Delete", "Help", "Theme", "Exit"}
	shortcuts := []string{"[Tab]", "[x]", "[F5|ESC+5|c]", "[F6|ESC+6|r]", "[F7|ESC+7|m]", "[F8|ESC+8|d]", "[F1]", "[space]", "[q|ESC+0]"}

	// Funkcja stylizująca kolumny
	var TableStyle = func(row, col int) lipgloss.Style {
		switch {
		case row == 0: // Nagłówki
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(ui.Subtle).
				Align(lipgloss.Center)
		default: // Skróty
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(ui.Special).
				Align(lipgloss.Center)
		}
	}

	// Tworzenie tabeli
	cmdTable := ltable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.StatusBar)).
		StyleFunc(TableStyle).
		Headers(headers...).
		Row(shortcuts...)

	// Renderowanie tabeli
	return cmdTable.Render()
}

// Funkcja pomocnicza do budowania wierszy

// Pomocnicze stałe dla kolorów i stylów
var (
	panelBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
	}

	panelStyle = lipgloss.NewStyle().
			Border(panelBorder).
			BorderForeground(ui.Subtle).
			Padding(0, 1).
			Height(20) // Dodaj stałą wysokość

	activePathStyle = lipgloss.NewStyle().
			Bold(true).
			Background(ui.Highlight).
			Foreground(lipgloss.Color("0"))

	inactivePathStyle = lipgloss.NewStyle().
				Foreground(ui.Subtle)
)

// formatPath formatuje ścieżkę do wyświetlenia
func formatPath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}

	// Dodaj "..." na początku jeśli ścieżka jest za długa
	return "..." + path[len(path)-(maxWidth-3):]
}

func getFileType(entry FileEntry) string {
	if entry.isDir {
		return "directory"
	}

	// Określenie typu na podstawie rozszerzenia
	ext := strings.ToLower(filepath.Ext(entry.name))

	// Archiwa
	switch ext {
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return "archive"
	}

	// Obrazy
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
		return "image"
	}

	// Dokumenty
	switch ext {
	case ".txt", ".doc", ".docx", ".pdf", ".md", ".csv", ".xlsx", ".odt":
		return "document"
	}

	// Pliki wykonywalne
	switch ext {
	case ".exe", ".sh", ".bat", ".cmd", ".com", ".app":
		return "executable"
	}

	// Pliki kodu
	switch ext {
	case ".c":
		return "code_c"
	case ".h":
		return "code_h"
	case ".go":
		return "code_go"
	case ".py":
		return "code_py"
	case ".js":
		return "code_js"
	case ".json":
		return "code_json"
		// Możesz dodać więcej rozszerzeń dla innych języków programowania tutaj
	}

	// Jeśli plik ma ustawione prawa wykonywania
	if entry.mode&0111 != 0 {
		return "executable"
	}

	return "default"
}

// internal/ui/views/transfer.go
// internal/ui/views/transfer.go

func (v *transferView) renderFileList(entries []FileEntry, selected int, _ bool, width int) string {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: " ", Width: 2}, // Kolumna na gwiazdkę
			{Title: "Name", Width: width - 37},
			{Title: "Size", Width: 10},
			{Title: "Modified", Width: 19},
		}),
	)

	var rows []table.Row
	for _, entry := range entries {
		path := filepath.Join(v.getActivePanel().path, entry.name)
		isMarked := v.model.IsSelected(path)

		// Tworzenie wiersza
		prefix := " "
		if isMarked {
			prefix = "*"
		}

		name := entry.name
		if entry.isDir {
			name = "[" + name + "]"
		}

		row := table.Row{
			prefix,
			name,
			formatSize(entry.size),
			entry.modTime.Format("2006-01-02 15:04"),
		}
		rows = append(rows, row)
	}

	t.SetRows(rows)

	// Renderujemy tabelę
	tableOutput := t.View()

	// Teraz dodajemy kolory linijka po linijce
	var coloredOutput strings.Builder
	lines := strings.Split(tableOutput, "\n")

	for i, line := range lines {
		// Pomijamy linie nagłówka (pierwsza linia)
		if i == 0 {
			coloredOutput.WriteString(line + "\n")
			continue
		}

		// Sprawdzamy czy ta linia odpowiada jakiemuś plikowi
		entryIndex := i - 1 // odejmujemy 1 bo pierwsza linia to nagłówek
		if entryIndex >= 0 && entryIndex < len(entries) {
			entry := entries[entryIndex]
			var style lipgloss.Style

			// Specjalne traktowanie linii ".."
			if entry.name == ".." {
				if entryIndex == selected {
					// Ten sam styl dla aktywnego i nieaktywnego panelu gdy ".." jest zaznaczone
					style = lipgloss.NewStyle().
						Bold(true).
						Background(ui.Highlight).
						Foreground(lipgloss.Color("0"))
				} else {
					style = ui.DirectoryStyle
				}
			} else if entryIndex == selected {
				// Ten sam styl dla zaznaczenia w obu panelach
				style = lipgloss.NewStyle().
					Bold(true).
					Background(ui.Highlight).
					Foreground(lipgloss.Color("0"))
			} else if entry.isDir {
				// Katalogi zawsze używają DirectoryStyle
				style = ui.DirectoryStyle
			} else {
				switch getFileType(entry) {
				case "executable":
					style = ui.ExecutableStyle
				case "archive":
					style = ui.ArchiveStyle
				case "image":
					style = ui.ImageStyle
				case "document":
					style = ui.DocumentStyle
				case "code_c":
					style = ui.CodeCStyle
				case "code_h":
					style = ui.CodeHStyle
				case "code_go":
					style = ui.CodeGoStyle
				case "code_py":
					style = ui.CodePyStyle
				case "code_js":
					style = ui.CodeJsStyle
				case "code_json":
					style = ui.CodeJsonStyle
				default:
					if strings.HasPrefix(getFileType(entry), "code_") {
						style = ui.CodeDefaultStyle
					} else {
						style = ui.DefaultFileStyle
					}
				}
			}
			coloredOutput.WriteString(style.Render(line) + "\n")
		} else {
			coloredOutput.WriteString(line + "\n")
		}
	}

	return coloredOutput.String()
}

func (v *transferView) ensureConnected() error {
	transfer := v.model.GetTransfer()
	if transfer == nil {
		return fmt.Errorf("no transfer client available")
	}

	host := v.model.GetSelectedHost()
	if host == nil {
		return fmt.Errorf("no host selected")
	}

	var authData string

	if host.PasswordID < 0 {
		// Obsługa klucza SSH
		keyIndex := -(host.PasswordID + 1)
		keys := v.model.GetKeys()
		if keyIndex >= len(keys) {
			return fmt.Errorf("invalid key ID")
		}

		key := keys[keyIndex]
		keyPath, pathErr := key.GetKeyPath()
		if pathErr != nil {
			return fmt.Errorf("failed to get key path: %v", pathErr)
		}
		authData = keyPath
	} else {
		// Obsługa hasła
		passwords := v.model.GetPasswords()
		if host.PasswordID >= len(passwords) {
			return fmt.Errorf("invalid password ID")
		}

		password := passwords[host.PasswordID]
		decryptedPass, decErr := password.GetDecrypted(v.model.GetCipher())
		if decErr != nil {
			return fmt.Errorf("failed to decrypt password: %v", decErr)
		}
		authData = decryptedPass
	}

	if err := transfer.Connect(host, authData); err != nil {
		return fmt.Errorf("failed to establish SFTP connection: %v", err)
	}

	return nil
}

func (v *transferView) setConnected(connected bool) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.connected = connected
}

func (v *transferView) sendConnectionUpdate() tea.Cmd {
	return func() tea.Msg {
		return connectionStatusMsg{
			connected: v.connected,
			err:       nil,
		}
	}
}

func (v *transferView) renderFooter() string {
	var footerContent strings.Builder

	// Komunikat o błędzie
	if v.errorMessage != "" {
		footerContent.WriteString(ui.ErrorStyle.Render("Error: " + v.errorMessage))
		footerContent.WriteString("\n")
	}

	// Status
	if v.statusMessage != "" {
		style := ui.DescriptionStyle
		if v.shouldShowDeleteConfirm() {
			style = ui.ErrorStyle
		} else if v.isWaitingForInput() {
			style = ui.InputStyle
		}
		footerContent.WriteString(style.Render(v.statusMessage))
		footerContent.WriteString("\n")
	}

	// Komunikat o braku połączenia
	if !v.connected && v.errorMessage == "" {
		footerContent.WriteString(ui.ErrorStyle.Render(
			"SFTP connection not established. Press 'q' to return to main menu and connect first."))
		footerContent.WriteString("\n")
	}

	// Skróty klawiszowe
	if v.connected {
		footerContent.WriteString(v.renderShortcuts())
	} else {
		footerContent.WriteString(ui.ButtonStyle.Render("q") + " - Return to main menu")
	}

	return footerContent.String()
}
