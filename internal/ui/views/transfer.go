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

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	mode    os.FileMode
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
	model                 *ui.Model
	localPanel            Panel
	remotePanel           Panel
	statusMessage         string
	errorMessage          string
	connecting            bool
	connected             bool
	transferring          bool
	progress              ssh.TransferProgress
	showHelp              bool
	input                 textinput.Model
	mutex                 sync.Mutex
	width                 int
	height                int
	escPressed            bool
	escTimeout            *time.Timer
	popup                 *components.Popup
	lastClickTime         time.Time
	lastClickPanelIsLocal bool
	lastClickIndex        int
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
			path:   "~/",
			active: false,
			entries: []FileEntry{
				{name: "..", isDir: true},
			},
		},
		input:          input,
		width:          model.GetTerminalWidth(),
		height:         model.GetTerminalHeight(),
		lastClickIndex: -1,
	}

	// Inicjalizujemy panel lokalny
	if err := v.updateLocalPanel(); err != nil {
		v.errorMessage = fmt.Sprintf("Failed to load local directory: %v", err)
		return v
	}

	// Inicjujemy połączenie SFTP w tle
	if v.model.GetSelectedHost() != nil {
		go func() {
			err := v.ensureConnected()
			if err != nil {
				v.model.Program.Send(connectionStatusMsg{
					connected: false,
					err:       err,
				})
				return
			}

			transfer := v.model.GetTransfer()
			if homeDir, err := transfer.GetRemoteHomeDir(); err == nil {
				v.remotePanel.path = homeDir
			}

			err = v.updateRemotePanel()
			if err != nil {
				v.model.Program.Send(connectionStatusMsg{
					connected: false,
					err:       err,
				})
				return
			}

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

func (v *transferView) readLocalDirectory(path string) ([]FileEntry, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	entries := []FileEntry{{
		name:    "..",
		isDir:   true,
		modTime: time.Now(),
	}}

	for _, fi := range fileInfos {
		if !strings.HasPrefix(fi.Name(), ".") || fi.Name() == ".." {
			entries = append(entries, FileEntry{
				name:    fi.Name(),
				size:    fi.Size(),
				modTime: fi.ModTime(),
				isDir:   fi.IsDir(),
				mode:    fi.Mode(),
			})
		}
	}

	sort.Slice(entries[1:], func(i, j int) bool {
		i, j = i+1, j+1
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	return entries, nil
}

func (v *transferView) Init() tea.Cmd {
	if !v.connected && !v.connecting && v.model.GetSelectedHost() != nil {
		v.connecting = true
		return v.sendConnectionUpdate()
	}
	return nil
}

func (v *transferView) updateRemotePanel() error {
	if err := v.ensureConnected(); err != nil {
		return err
	}

	entries, err := v.readRemoteDirectory(v.remotePanel.path)
	if err != nil {
		v.setConnected(false)
		return err
	}
	v.remotePanel.entries = entries
	return nil
}

func (v *transferView) readRemoteDirectory(path string) ([]FileEntry, error) {
	if err := v.ensureConnected(); err != nil {
		return nil, err
	}

	transfer := v.model.GetTransfer()
	fileInfos, err := transfer.ListRemoteFiles(path)
	if err != nil {
		v.setConnected(false)
		return nil, fmt.Errorf("failed to list remote directory: %v", err)
	}

	entries := []FileEntry{{
		name:    "..",
		isDir:   true,
		modTime: time.Now(),
	}}

	for _, fi := range fileInfos {
		if !strings.HasPrefix(fi.Name(), ".") || fi.Name() == ".." {
			entries = append(entries, FileEntry{
				name:    fi.Name(),
				size:    fi.Size(),
				modTime: fi.ModTime(),
				isDir:   fi.IsDir(),
				mode:    fi.Mode(),
			})
		}
	}

	sort.Slice(entries[1:], func(i, j int) bool {
		i, j = i+1, j+1
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	return entries, nil
}

// getActivePanel zwraca aktywny panel
func (v *transferView) getActivePanel() *Panel {
	if v.localPanel.active {
		return &v.localPanel
	}
	return &v.remotePanel
}

// getInactivePanel zwraca nieaktywny panel
func (v *transferView) getInactivePanel() *Panel {
	if v.localPanel.active {
		return &v.remotePanel
	}
	return &v.localPanel
}

// switchActivePanel przełącza aktywny panel
func (v *transferView) switchActivePanel() {
	v.localPanel.active = !v.localPanel.active
	v.remotePanel.active = !v.remotePanel.active
}

// Modern panel rendering using lipgloss
func (v *transferView) renderPanel(p *Panel, title string, width int) string {
	// Panel header with breadcrumb-style path
	pathDisplay := v.formatBreadcrumb(p.path, width-4)

	var headerStyle lipgloss.Style
	if p.active {
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.GetCurrentTheme().Highlight).
			Background(ui.GetCurrentTheme().StatusBar).
			Width(width-2).
			Padding(0, 1).
			MarginBottom(0)
	} else {
		headerStyle = lipgloss.NewStyle().
			Foreground(ui.GetCurrentTheme().Subtle).
			Width(width-2).
			Padding(0, 1).
			MarginBottom(0)
	}

	header := headerStyle.Render(fmt.Sprintf(" %s  %s", title, pathDisplay))

	// File list
	fileList := v.renderModernFileList(p, width-4)

	// Panel info bar
	var infoText string
	if len(p.entries) > 0 {
		selectedCount := 0
		for _, entry := range p.entries {
			fullPath := filepath.Join(p.path, entry.name)
			if v.model.IsSelected(fullPath) {
				selectedCount++
			}
		}

		if selectedCount > 0 {
			infoText = fmt.Sprintf(" %d/%d items (%d selected)",
				p.selectedIndex+1, len(p.entries), selectedCount)
		} else {
			infoText = fmt.Sprintf(" %d/%d items", p.selectedIndex+1, len(p.entries))
		}
	}

	infoBarStyle := lipgloss.NewStyle().
		Foreground(ui.GetCurrentTheme().Subtle).
		Width(width-2).
		Padding(0, 1).
		MarginTop(0)

	infoBar := infoBarStyle.Render(infoText)

	// Combine all parts
	panelContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		fileList,
		infoBar,
	)

	// Apply panel border
	var borderStyle lipgloss.Style
	if p.active {
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.GetCurrentTheme().Highlight).
			Width(width).
			Height(maxVisibleItems + 4)
	} else {
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.GetCurrentTheme().Border).
			Width(width).
			Height(maxVisibleItems + 4)
	}

	return borderStyle.Render(panelContent)
}

// Modern file list rendering
func (v *transferView) renderModernFileList(p *Panel, width int) string {
	var lines []string

	// Calculate visible range
	start := p.scrollOffset
	end := min(start+maxVisibleItems, len(p.entries))

	for i := start; i < end; i++ {
		entry := p.entries[i]
		fullPath := filepath.Join(p.path, entry.name)
		isSelected := v.model.IsSelected(fullPath)
		isActive := i == p.selectedIndex

		line := v.renderFileEntry(entry, isSelected, isActive, width)
		lines = append(lines, line)
	}

	// Fill remaining lines
	for i := len(lines); i < maxVisibleItems; i++ {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

// Render individual file entry with modern styling
func (v *transferView) renderFileEntry(entry FileEntry, marked bool, active bool, width int) string {
	theme := ui.GetCurrentTheme()

	// Selection indicator
	indicator := " "
	if marked {
		indicator = "●"
	}

	// File icon
	icon := v.getFileIcon(entry)

	// File name with proper truncation
	maxNameWidth := width - 25 // Reserve space for size, date, and indicators
	displayName := entry.name
	if entry.isDir && entry.name != ".." {
		displayName = entry.name + "/"
	}
	if len(displayName) > maxNameWidth {
		displayName = displayName[:maxNameWidth-3] + "..."
	}

	// Size formatting
	sizeStr := formatSize(entry.size)
	if entry.isDir {
		sizeStr = "<DIR>"
	}

	// Date formatting
	dateStr := entry.modTime.Format("02 Jan 15:04")

	// Build the line
	var lineStyle lipgloss.Style
	var nameColor lipgloss.Color

	if active {
		// Active selection style
		lineStyle = lipgloss.NewStyle().
			Background(theme.Highlight).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Width(width)

		line := fmt.Sprintf(" %s %s %-*s %8s  %s",
			indicator, icon, maxNameWidth, displayName, sizeStr, dateStr)
		return lineStyle.Render(line)
	}

	// Determine color based on file type
	if entry.name == ".." {
		nameColor = theme.DirectoryColor
	} else if entry.isDir {
		nameColor = theme.DirectoryColor
	} else {
		nameColor = v.getFileColor(entry)
	}

	// Build line parts with individual styling
	indicatorStyle := lipgloss.NewStyle().Foreground(theme.Special)
	iconStyle := lipgloss.NewStyle().Foreground(nameColor)
	nameStyle := lipgloss.NewStyle().Foreground(nameColor)
	sizeStyle := lipgloss.NewStyle().Foreground(theme.Subtle)
	dateStyle := lipgloss.NewStyle().Foreground(theme.Subtle)

	parts := []string{
		" ",
		indicatorStyle.Render(indicator),
		" ",
		iconStyle.Render(icon),
		" ",
		nameStyle.Render(fmt.Sprintf("%-*s", maxNameWidth, displayName)),
		" ",
		sizeStyle.Render(fmt.Sprintf("%8s", sizeStr)),
		"  ",
		dateStyle.Render(dateStr),
	}

	line := strings.Join(parts, "")

	// Ensure line fits width
	lineRunes := []rune(line)
	if len(lineRunes) > width {
		line = string(lineRunes[:width])
	} else if len(lineRunes) < width {
		line = line + strings.Repeat(" ", width-len(lineRunes))
	}

	return line
}

// Get file icon based on type
func (v *transferView) getFileIcon(entry FileEntry) string {
	if entry.name == ".." {
		return "↩"
	}
	if entry.isDir {
		return "📁"
	}

	ext := strings.ToLower(filepath.Ext(entry.name))
	switch ext {
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return "📦"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
		return "🖼"
	case ".txt", ".doc", ".docx", ".pdf", ".md":
		return "📄"
	case ".go":
		return "🐹"
	case ".py":
		return "🐍"
	case ".js", ".ts":
		return "📜"
	case ".json", ".yaml", ".yml", ".toml":
		return "⚙"
	case ".exe", ".sh", ".bat", ".cmd":
		return "⚡"
	case ".c", ".h", ".cpp", ".hpp":
		return "©"
	default:
		if entry.mode&0111 != 0 {
			return "⚙"
		}
		return "📃"
	}
}

// Get file color based on type
func (v *transferView) getFileColor(entry FileEntry) lipgloss.Color {
	theme := ui.GetCurrentTheme()

	if entry.isDir {
		return theme.DirectoryColor
	}

	ext := strings.ToLower(filepath.Ext(entry.name))
	switch ext {
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return theme.ArchiveColor
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
		return theme.ImageColor
	case ".txt", ".doc", ".docx", ".pdf", ".md", ".csv", ".xlsx", ".odt":
		return theme.DocumentColor
	case ".c":
		return theme.CodeCColor
	case ".h":
		return theme.CodeHColor
	case ".go":
		return theme.CodeGoColor
	case ".py":
		return theme.CodePyColor
	case ".js", ".ts":
		return theme.CodeJsColor
	case ".json", ".yaml", ".yml":
		return theme.CodeJsonColor
	case ".exe", ".sh", ".bat", ".cmd", ".com", ".app":
		return theme.ExecutableColor
	default:
		if entry.mode&0111 != 0 {
			return theme.ExecutableColor
		}
		return theme.DefaultFileColor
	}
}

// Format breadcrumb-style path
func (v *transferView) formatBreadcrumb(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}

	// Split path and show last few segments
	parts := strings.Split(path, string(filepath.Separator))
	result := ""

	for i := len(parts) - 1; i >= 0; i-- {
		segment := parts[i]
		if len(result)+len(segment)+3 > maxWidth {
			result = "…/" + result
			break
		}
		if result == "" {
			result = segment
		} else {
			result = segment + "/" + result
		}
	}

	return result
}

func (v *transferView) View() string {
	var content strings.Builder

	// Modern title bar
	titleContent := v.renderTitleBar()
	content.WriteString(titleContent + "\n")

	// Handle connecting state
	if v.connecting {
		connectingView := v.renderConnectingScreen()
		return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, connectingView)
	}

	// Handle help view
	if v.showHelp {
		helpView := v.renderHelpScreen()
		return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, helpView)
	}

	// Calculate panel width
	availableWidth := min(v.width-10, 160)
	panelWidth := (availableWidth - 6) / 2

	// Render panels side by side
	leftPanel := v.renderPanel(&v.localPanel, "📁 Local", panelWidth)

	var rightPanel string
	if !v.connected {
		rightPanel = v.renderDisconnectedPanel(panelWidth)
	} else {
		rightPanel = v.renderPanel(&v.remotePanel, "🌐 Remote", panelWidth)
	}

	panelsView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
	content.WriteString(panelsView + "\n")

	// Progress bar
	if v.transferring {
		progressBar := v.renderModernProgressBar(availableWidth)
		content.WriteString("\n" + progressBar + "\n")
	}

	// Command input (if active)
	if v.isWaitingForInput() {
		content.WriteString("\n" + v.input.View())
	}

	// Modern footer with shortcuts
	footer := v.renderModernFooter()
	content.WriteString("\n" + footer)

	// Wrap in window style
	finalContent := ui.WindowStyle.Render(content.String())

	// Handle popup overlay
	if v.popup != nil {
		overlay := lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center,
			finalContent+"\n"+v.popup.Render())
		return overlay
	}

	return lipgloss.Place(v.width, v.height, lipgloss.Left, lipgloss.Top, finalContent)
}

// Modern title bar
func (v *transferView) renderTitleBar() string {
	theme := ui.GetCurrentTheme()

	title := "📁 File Transfer Manager"

	var status string
	var statusStyle lipgloss.Style

	if v.connected {
		if host := v.model.GetSelectedHost(); host != nil {
			status = fmt.Sprintf("✓ Connected to %s (%s)", host.Name, host.IP)
			statusStyle = lipgloss.NewStyle().
				Foreground(theme.Special).
				Bold(true)
		}
	} else if host := v.model.GetSelectedHost(); host != nil {
		if v.connecting {
			status = "⟳ Establishing connection..."
			statusStyle = lipgloss.NewStyle().
				Foreground(theme.Highlight)
		} else {
			status = fmt.Sprintf("✗ Not connected to %s", host.Name)
			statusStyle = lipgloss.NewStyle().
				Foreground(theme.Error)
		}
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Highlight).
		Bold(true)

	return titleStyle.Render(title) + "  " + statusStyle.Render(status)
}

// Modern progress bar
func (v *transferView) renderModernProgressBar(width int) string {
	if !v.transferring || v.progress.TotalBytes == 0 {
		return ""
	}

	theme := ui.GetCurrentTheme()
	percentage := float64(v.progress.TransferredBytes) / float64(v.progress.TotalBytes)

	// Progress bar
	barWidth := width - 40
	completedWidth := int(float64(barWidth) * percentage)

	barStyle := lipgloss.NewStyle().
		Foreground(theme.Special).
		Background(theme.Subtle)

	bar := barStyle.Render(strings.Repeat("█", completedWidth)) +
		lipgloss.NewStyle().Foreground(theme.Subtle).Render(strings.Repeat("░", barWidth-completedWidth))

	// Calculate speed
	elapsed := time.Since(v.progress.StartTime).Seconds()
	if elapsed == 0 {
		elapsed = 1
	}
	speed := float64(v.progress.TransferredBytes) / elapsed

	progressText := fmt.Sprintf("📤 %s  %s %3.0f%%  %s/s",
		v.progress.FileName,
		bar,
		percentage*100,
		formatSize(int64(speed)))

	return lipgloss.NewStyle().
		Foreground(theme.Highlight).
		Render(progressText)
}

// Modern footer with shortcuts
func (v *transferView) renderModernFooter() string {
	theme := ui.GetCurrentTheme()

	// Error message
	if v.errorMessage != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(theme.Error).
			Bold(true)
		return errorStyle.Render("✗ " + v.errorMessage)
	}

	// Status message
	if v.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(theme.Highlight)
		return statusStyle.Render("ℹ " + v.statusMessage)
	}

	if !v.connected {
		disconnectedStyle := lipgloss.NewStyle().
			Foreground(theme.Error)
		return disconnectedStyle.Render("⚠ Not connected. Press 'q' to return to main menu.")
	}

	// Shortcuts bar
	shortcuts := []string{
		"Tab·Switch",
		"x·Select",
		"F5·Copy",
		"F6·Rename",
		"F7·MkDir",
		"F8·Del",
		"F1·Help",
		"Space·Theme",
		"q·Exit",
	}

	shortcutStyle := lipgloss.NewStyle().
		Foreground(theme.Special).
		Bold(false)

	separatorStyle := lipgloss.NewStyle().
		Foreground(theme.Subtle)

	var parts []string
	for i, shortcut := range shortcuts {
		parts = append(parts, shortcutStyle.Render(shortcut))
		if i < len(shortcuts)-1 {
			parts = append(parts, separatorStyle.Render(" │ "))
		}
	}

	return strings.Join(parts, "")
}

// Render disconnected panel
func (v *transferView) renderDisconnectedPanel(width int) string {
	theme := ui.GetCurrentTheme()

	message := lipgloss.NewStyle().
		Foreground(theme.Error).
		Bold(true).
		Align(lipgloss.Center).
		Width(width - 4).
		Render("\n\n⚠ No SFTP Connection\n\nPress 'q' to return to main menu\nand connect to a host first")

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Width(width).
		Height(maxVisibleItems + 4)

	return borderStyle.Render(message)
}

// Render connecting screen
func (v *transferView) renderConnectingScreen() string {
	theme := ui.GetCurrentTheme()

	spinner := "⟳"
	message := lipgloss.NewStyle().
		Foreground(theme.Highlight).
		Bold(true).
		Render(fmt.Sprintf("%s Establishing SFTP connection...", spinner))

	return ui.WindowStyle.Render(message)
}

// Render help screen
func (v *transferView) renderHelpScreen() string {
	theme := ui.GetCurrentTheme()

	helpText := `
╔═══════════════════════════════════════════════╗
║        FILE TRANSFER HELP                     ║
╚═══════════════════════════════════════════════╝

Navigation:
  ↑/↓, w/s     Move cursor up/down
  Enter        Enter directory / Open file
  Backspace    Go to parent directory
  Tab          Switch between panels

File Operations:
  x            Select/unselect current file
  F5 / c       Copy file(s) to other panel
  F6 / r       Rename file/directory
  F7 / m       Create new directory
  F8 / d       Delete file/directory
  
View & Settings:
  F1           Toggle this help
  Space        Change theme
  Ctrl+R       Refresh current panel

Transfer:
  ESC+5        Alternative copy
  ESC+0        Exit transfer mode

General:
  q            Exit to main menu
  ESC          Cancel current operation

Tips:
  • Use 'x' to select multiple files before copying
  • Double-click on items to enter directories
  • Selected items are marked with ●
  • Active panel has highlighted border
`

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.LabelColor).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Highlight).
		Padding(1, 2).
		Width(60)

	return helpStyle.Render(helpText)
}

// navigatePanel obsługuje nawigację w panelu
func (v *transferView) navigatePanel(p *Panel, direction int) {
	if len(p.entries) == 0 {
		p.selectedIndex = 0
		p.scrollOffset = 0
		return
	}

	newIndex := p.selectedIndex + direction

	if newIndex < 0 {
		newIndex = len(p.entries) - 1
	} else if newIndex >= len(p.entries) {
		newIndex = 0
	}

	p.selectedIndex = newIndex

	// Adjust scrolling
	if p.selectedIndex < p.scrollOffset {
		p.scrollOffset = p.selectedIndex
	} else if p.selectedIndex >= p.scrollOffset+maxVisibleItems {
		p.scrollOffset = p.selectedIndex - maxVisibleItems + 1
	}

	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}
}

// enterDirectory wchodzi do wybranego katalogu
func (v *transferView) enterDirectory(p *Panel) error {
	if len(p.entries) == 0 || p.selectedIndex >= len(p.entries) {
		return nil
	}

	entry := p.entries[p.selectedIndex]
	if !entry.isDir {
		return nil
	}

	var newPath string
	if entry.name == ".." {
		newPath = filepath.Dir(p.path)
		if runtime.GOOS == "windows" && filepath.Dir(newPath) == newPath {
			newPath = filepath.VolumeName(newPath) + "\\"
		}
	} else {
		newPath = filepath.Join(p.path, entry.name)
	}

	oldPath := p.path
	p.path = newPath

	var err error
	if p == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	if err != nil {
		p.path = oldPath
		return err
	}

	p.selectedIndex = 0
	p.scrollOffset = 0
	return nil
}

func (v *transferView) tryEnterDirectory(panel *Panel) {
	if err := v.enterDirectory(panel); err != nil {
		v.popup = components.NewPopup(
			components.PopupMessage,
			"Error",
			err.Error(),
			50,
			7,
			v.width,
			v.height,
		)
	}
}

func (v *transferView) hasSelectedItems() bool {
	for _, isSelected := range v.getSelectedItems() {
		if isSelected {
			return true
		}
	}
	return false
}

func (v *transferView) getSelectedItems() map[string]bool {
	selected := make(map[string]bool)
	paths := v.model.GetSelectedPaths()
	for _, path := range paths {
		selected[path] = true
	}
	return selected
}

func (v *transferView) copyFile() tea.Cmd {
	srcPanel := v.getActivePanel()
	dstPanel := v.getInactivePanel()

	var itemsToCopy []struct {
		srcPath string
		dstPath string
		isDir   bool
	}

	if !v.hasSelectedItems() {
		if len(srcPanel.entries) == 0 || srcPanel.selectedIndex >= len(srcPanel.entries) {
			v.handleError(fmt.Errorf("no file selected"))
			return nil
		}
		entry := srcPanel.entries[srcPanel.selectedIndex]

		isLocal := srcPanel == &v.localPanel
		srcName := filepath.Base(entry.name)
		dstName := srcName

		var srcPath, dstPath string
		if isLocal {
			srcPath = filepath.Join(srcPanel.path, srcName)
			dstPath = utils.ToSFTPPath(filepath.Join(dstPanel.path, dstName))
		} else {
			srcPath = utils.ToSFTPPath(filepath.Join(srcPanel.path, srcName))
			dstPath = utils.ToLocalPath(filepath.Join(dstPanel.path, dstName))
		}

		itemsToCopy = append(itemsToCopy, struct {
			srcPath string
			dstPath string
			isDir   bool
		}{srcPath, dstPath, entry.isDir})
	} else {
		for path, isSelected := range v.getSelectedItems() {
			if !isSelected {
				continue
			}

			isLocal := srcPanel == &v.localPanel
			srcName := filepath.Base(path)
			dstName := srcName

			var srcPath, dstPath string
			if isLocal {
				srcPath = filepath.Join(srcPanel.path, srcName)
				dstPath = utils.ToSFTPPath(filepath.Join(dstPanel.path, dstName))
			} else {
				srcPath = utils.ToSFTPPath(filepath.Join(srcPanel.path, srcName))
				dstPath = utils.ToLocalPath(filepath.Join(dstPanel.path, dstName))
			}

			info, err := os.Stat(path)
			if err != nil {
				v.handleError(fmt.Errorf("cannot access %s: %v", path, err))
				continue
			}

			itemsToCopy = append(itemsToCopy, struct {
				srcPath string
				dstPath string
				isDir   bool
			}{srcPath, dstPath, info.IsDir()})
		}
	}

	if len(itemsToCopy) == 0 {
		v.handleError(fmt.Errorf("no items to copy"))
		return nil
	}

	v.mutex.Lock()
	v.transferring = true
	v.statusMessage = "Copying files..."
	v.mutex.Unlock()

	transfer := v.model.GetTransfer()

	return func() tea.Msg {
		progressChan := make(chan ssh.TransferProgress)
		doneChan := make(chan error, 1)

		go func() {
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
		}()

		go func() {
			for progress := range progressChan {
				v.model.Program.Send(transferProgressMsg(progress))
			}
			err := <-doneChan
			v.model.Program.Send(transferFinishedMsg{err: err})
			v.model.ClearSelection()
		}()

		return nil
	}
}

func (v *transferView) copyDirectoryToRemote(localPath, remotePath string, transfer *ssh.FileTransfer, progressChan chan<- ssh.TransferProgress) error {
	remotePath = utils.ToSFTPPath(remotePath)
	if err := transfer.CreateRemoteDirectory(remotePath); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	return filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}

		remotePathFull := utils.ToSFTPPath(filepath.Join(remotePath, relPath))

		if info.IsDir() {
			return transfer.CreateRemoteDirectory(remotePathFull)
		}

		return transfer.UploadFile(path, remotePathFull, progressChan)
	})
}

func (v *transferView) copyDirectoryFromRemote(remotePath, localPath string, transfer *ssh.FileTransfer, progressChan chan<- ssh.TransferProgress) error {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	remotePath = utils.ToSFTPPath(remotePath)
	entries, err := transfer.ListRemoteFiles(remotePath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}

		remoteSrcPath := utils.ToSFTPPath(filepath.Join(remotePath, entry.Name()))
		localDstPath := filepath.Join(localPath, entry.Name())

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

// executeDelete wykonuje faktyczne usuwanie pliku
func (v *transferView) executeDelete() error {
	panel := v.getActivePanel()
	entry := panel.entries[panel.selectedIndex]
	path := filepath.Join(panel.path, entry.name)

	var err error
	itemType := "file"
	if entry.isDir {
		itemType = "directory"
	}

	if panel == &v.localPanel {
		if entry.isDir {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
	} else {
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

func (v *transferView) removeRemoteDirectory(path string, transfer *ssh.FileTransfer) error {
	entries, err := transfer.ListRemoteFiles(path)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if err := v.removeRemoteDirectory(fullPath, transfer); err != nil {
				return err
			}
		} else {
			if err := transfer.RemoveRemoteFile(fullPath); err != nil {
				return err
			}
		}
	}

	return transfer.RemoveRemoteFile(path)
}

// createDirectory tworzy nowy katalog
func (v *transferView) createDirectory(name string) error {
	if name == "" {
		return fmt.Errorf("directory name cannot be empty")
	}

	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("directory name cannot contain path separators")
	}

	panel := v.getActivePanel()
	newPath := filepath.Join(panel.path, name)

	var err error
	if panel == &v.localPanel {
		err = os.Mkdir(newPath, 0755)
	} else {
		transfer := v.model.GetTransfer()
		err = transfer.CreateRemoteDirectory(newPath)
	}

	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if panel == &v.localPanel {
		v.updateLocalPanel()
	} else {
		v.updateRemotePanel()
	}

	v.statusMessage = fmt.Sprintf("Created directory '%s'", name)
	return nil
}

// renameFile zmienia nazwę pliku
func (v *transferView) renameFile(newName string) error {
	if newName == "" {
		return fmt.Errorf("new name cannot be empty")
	}

	panel := v.getActivePanel()
	if panel.selectedIndex >= len(panel.entries) {
		return fmt.Errorf("no file selected")
	}

	entry := panel.entries[panel.selectedIndex]
	if entry.name == ".." {
		return fmt.Errorf("cannot rename parent directory")
	}

	oldPath := filepath.Join(panel.path, entry.name)
	newPath := filepath.Join(panel.path, newName)

	var err error
	if panel == &v.localPanel {
		err = os.Rename(oldPath, newPath)
	} else {
		transfer := v.model.GetTransfer()
		err = transfer.RenameRemoteFile(oldPath, newPath)
	}

	if err != nil {
		return fmt.Errorf("failed to rename: %v", err)
	}

	if panel == &v.localPanel {
		v.updateLocalPanel()
	} else {
		v.updateRemotePanel()
	}

	v.statusMessage = fmt.Sprintf("Renamed '%s' to '%s'", entry.name, newName)
	return nil
}

func (v *transferView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return v, nil

	case transferProgressMsg:
		v.mutex.Lock()
		v.progress = ssh.TransferProgress(msg)
		v.mutex.Unlock()
		return v, nil

	case transferFinishedMsg:
		v.mutex.Lock()
		v.transferring = false
		if msg.err != nil {
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Transfer Error",
				fmt.Sprintf("Transfer error: %v", msg.err),
				50,
				7,
				v.width,
				v.height,
			)
		} else {
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Success",
				"Transfer completed successfully",
				50,
				7,
				v.width,
				v.height,
			)
			dstPanel := v.getInactivePanel()
			if dstPanel == &v.localPanel {
				v.updateLocalPanel()
			} else {
				v.updateRemotePanel()
			}
		}
		v.mutex.Unlock()
		return v, nil

	case connectionStatusMsg:
		v.mutex.Lock()
		v.connecting = false
		if msg.err != nil {
			v.connected = false
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Connection Error",
				fmt.Sprintf("Connection error: %v", msg.err),
				50,
				7,
				v.width,
				v.height,
			)
		} else {
			v.connected = msg.connected
		}
		v.mutex.Unlock()
		return v, nil

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseLeft:
			if v.popup != nil {
				if msg.X >= v.popup.X && msg.X <= v.popup.X+v.popup.Width &&
					msg.Y >= v.popup.Y && msg.Y <= v.popup.Y+v.popup.Height {
					if v.popup.Type == components.PopupDelete {
						if msg.Y >= v.popup.Y+v.popup.Height-3 {
							if msg.X < v.popup.X+v.popup.Width/2 {
								if err := v.executeDelete(); err != nil {
									v.handleError(err)
								}
								v.popup = nil
								return v, nil
							} else {
								v.popup = nil
								return v, nil
							}
						}
					} else if v.popup.Type == components.PopupRename || v.popup.Type == components.PopupMkdir {
						if msg.Y >= v.popup.Y+v.popup.Height-3 {
							if msg.X < v.popup.X+v.popup.Width/2 {
								if err := v.handleCommand(v.popup.Input.Value()); err != nil {
									v.handleError(err)
								}
								v.popup = nil
								return v, nil
							} else {
								v.popup = nil
								return v, nil
							}
						}
						if msg.Y == v.popup.Y+2 && msg.X >= v.popup.X+1 && msg.X <= v.popup.X+v.popup.Width-2 {
							v.popup.Input.Focus()
							return v, nil
						}
					} else {
						v.popup = nil
						return v, nil
					}
				}
				return v, nil
			}

			panelWidth := (min(v.width-10, 160) - 6) / 2
			panelStartY := 7
			panelHeight := v.height - 10

			// Left panel click
			if msg.Y >= panelStartY && msg.Y < panelStartY+panelHeight && msg.X >= 1 && msg.X < panelWidth {
				v.localPanel.active = true
				v.remotePanel.active = false
				clickedIndex := msg.Y - panelStartY + v.localPanel.scrollOffset
				if clickedIndex >= 0 && clickedIndex < len(v.localPanel.entries) {
					now := time.Now()
					isDoubleClick := v.lastClickPanelIsLocal &&
						v.lastClickIndex == clickedIndex &&
						!v.lastClickTime.IsZero() &&
						now.Sub(v.lastClickTime) <= mouseDoubleClickThreshold

					v.localPanel.selectedIndex = clickedIndex
					v.errorMessage = ""
					v.lastClickTime = now
					v.lastClickPanelIsLocal = true
					v.lastClickIndex = clickedIndex

					if isDoubleClick {
						v.lastClickTime = time.Time{}
						v.lastClickIndex = -1
						v.tryEnterDirectory(&v.localPanel)
					}
					return v, nil
				}
				return v, nil
			}

			// Right panel click
			separatorPos := panelWidth + 4
			if msg.Y >= panelStartY && msg.Y < panelStartY+panelHeight &&
				msg.X >= separatorPos && msg.X < separatorPos+panelWidth {
				v.localPanel.active = false
				v.remotePanel.active = true
				clickedIndex := msg.Y - panelStartY + v.remotePanel.scrollOffset
				if clickedIndex >= 0 && clickedIndex < len(v.remotePanel.entries) {
					now := time.Now()
					isDoubleClick := !v.lastClickPanelIsLocal &&
						v.lastClickIndex == clickedIndex &&
						!v.lastClickTime.IsZero() &&
						now.Sub(v.lastClickTime) <= mouseDoubleClickThreshold

					v.remotePanel.selectedIndex = clickedIndex
					v.errorMessage = ""
					v.lastClickTime = now
					v.lastClickPanelIsLocal = false
					v.lastClickIndex = clickedIndex

					if isDoubleClick {
						v.lastClickTime = time.Time{}
						v.lastClickIndex = -1
						v.tryEnterDirectory(&v.remotePanel)
					}
					return v, nil
				}
				return v, nil
			}

		case tea.MouseWheelUp:
			panel := v.getActivePanel()
			if panel.selectedIndex > 0 {
				panel.selectedIndex--
				if panel.selectedIndex < panel.scrollOffset {
					panel.scrollOffset = panel.selectedIndex
				}
			}

		case tea.MouseWheelDown:
			panel := v.getActivePanel()
			if panel.selectedIndex < len(panel.entries)-1 {
				panel.selectedIndex++
				if panel.selectedIndex >= panel.scrollOffset+maxVisibleItems {
					panel.scrollOffset = panel.selectedIndex - maxVisibleItems + 1
				}
			}
		}

	case tea.KeyMsg:
		// Handle popup
		if v.popup != nil {
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

		// Handle help
		if v.showHelp {
			switch msg.String() {
			case "esc", "q", "f1":
				v.showHelp = false
				return v, nil
			default:
				return v, nil
			}
		}

		// Handle ESC sequences
		if v.escPressed {
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

			case "5":
				if !v.transferring {
					cmd := v.copyFile()
					v.escPressed = false
					if v.escTimeout != nil {
						v.escTimeout.Stop()
					}
					return v, cmd
				}

			case "6":
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

			case "7":
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

			case "8":
				if !v.transferring {
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
						50,
						7,
						v.width,
						v.height,
					)
				}
				return v, nil
			}

			v.escPressed = false
			if v.escTimeout != nil {
				v.escTimeout.Stop()
			}
			return v, nil
		}

		// Normal key handling
		switch msg.String() {
		case "esc":
			if !v.escPressed {
				v.escPressed = true
				v.escTimeout = time.AfterFunc(1*time.Second, func() {
					v.escPressed = false
				})
			}
			return v, nil

		case "f1":
			v.showHelp = !v.showHelp
			return v, nil

		case " ":
			ui.NextTheme()
			return v, nil

		case "ctrl+r":
			panel := v.getActivePanel()
			if panel == &v.localPanel {
				v.updateLocalPanel()
			} else {
				v.updateRemotePanel()
			}
			v.statusMessage = "Panel refreshed"
			return v, nil

		case "f5", "c":
			if !v.transferring && v.connected {
				return v, v.copyFile()
			}
			return v, nil

		case "f6", "r":
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

		case "f7", "m":
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

		case "f8", "d":
			if !v.transferring {
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
					50,
					7,
					v.width,
					v.height,
				)
			}
			return v, nil

		case "q":
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

		case "tab":
			if v.connected {
				v.switchActivePanel()
				v.errorMessage = ""
			}
			return v, nil

		case "up", "w":
			panel := v.getActivePanel()
			v.navigatePanel(panel, -1)
			v.errorMessage = ""
			return v, nil

		case "down", "s":
			panel := v.getActivePanel()
			v.navigatePanel(panel, 1)
			v.errorMessage = ""
			return v, nil

		case "enter":
			panel := v.getActivePanel()
			v.tryEnterDirectory(panel)
			return v, nil

		case "x":
			if !v.transferring {
				panel := v.getActivePanel()
				if len(panel.entries) > 0 && panel.selectedIndex < len(panel.entries) {
					entry := panel.entries[panel.selectedIndex]
					path := filepath.Join(panel.path, entry.name)
					if entry.name != ".." {
						v.model.ToggleSelection(path)
					}
				}
			}
			return v, nil
		}

	case ssh.TransferProgress:
		v.progress = msg
		return v, nil
	}

	return v, nil
}

// handleCommand obsługuje wprowadzanie komend
func (v *transferView) handleCommand(cmd string) error {
	if v.popup == nil {
		return fmt.Errorf("no active popup")
	}

	switch v.popup.Type {
	case components.PopupRename:
		err := v.renameFile(cmd)
		v.popup = nil
		return err
	case components.PopupMkdir:
		err := v.createDirectory(cmd)
		v.popup = nil
		return err
	default:
		v.popup = nil
		return fmt.Errorf("unknown command")
	}
}

// shouldShowDeleteConfirm sprawdza czy wyświetlić potwierdzenie usunięcia
func (v *transferView) shouldShowDeleteConfirm() bool {
	return strings.HasPrefix(v.statusMessage, "Delete ")
}

// isWaitingForInput sprawdza czy oczekuje na wprowadzenie tekstu
func (v *transferView) isWaitingForInput() bool {
	return strings.HasPrefix(v.statusMessage, "Enter ")
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

func (v *transferView) handleError(err error) {
	v.errorMessage = err.Error()
	time.AfterFunc(3*time.Second, func() {
		v.errorMessage = ""
	})
}

// formatSize formatuje rozmiar pliku
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

// Pomocnicze funkcje
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
