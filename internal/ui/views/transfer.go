package views

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"

    "sshManager/internal/ssh"
    "sshManager/internal/ui"
    "sshManager/internal/ui/components"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// FileEntry represents a single file or directory entry
// This mirrors the core information we need for rendering and navigation
// and keeps track of the file type for styling.
type FileEntry struct {
    name    string
    size    int64
    modTime time.Time
    isDir   bool
    mode    os.FileMode
}

// panelSide identifies which pane is active: local or remote
type panelSide int

const (
    localPanelSide panelSide = iota
    remotePanelSide
)

// itemArea tracks the screen area for a single row in a panel
// so we can map mouse clicks back to the underlying file entry.
type itemArea struct {
    index  int
    startY int
    endY   int
    startX int
    endX   int
}

// filePanel contains state for either the local or remote pane of the transfer view
type filePanel struct {
    path          string
    entries       []FileEntry
    selectedIndex int
    scrollOffset  int
    maxVisible    int
    side          panelSide
    itemAreas     []itemArea
}

// transferView is the redesigned transfer UI with consistent styling and mouse support
type transferView struct {
    model         *ui.Model
    localPanel    *filePanel
    remotePanel   *filePanel
    activePanel   panelSide
    statusMessage string
    errorMessage  string
    popup         *components.Popup
    connecting    bool
    connected     bool
    transferring  bool
    progress      ssh.TransferProgress
    mutex         sync.Mutex
    width         int
    height        int

    // Layout bookkeeping
    panelTop    int
    panelWidth  int
    panelHeight int
    panelGap    int

    // Mouse tracking for double-click behaviour
    localMouseTracker  *components.MouseTracker
    remoteMouseTracker *components.MouseTracker
}

// transferStatusMsg communicates connection state between goroutines
// We keep it minimal: either we're connected, connecting, or we hit an error.
type transferStatusMsg struct {
    connecting bool
    connected  bool
    err        error
}

// transferSuccessMsg notifies the UI that a background transfer completed successfully
type transferSuccessMsg struct {
    filename string
}

// transferErrorMsg notifies the UI that a transfer failed
type transferErrorMsg struct {
    err error
}

// NewTransferView constructs the redesigned transfer view
func NewTransferView(model *ui.Model) *transferView {
    v := &transferView{
        model:              model,
        activePanel:        localPanelSide,
        width:              model.GetTerminalWidth(),
        height:             model.GetTerminalHeight(),
        panelGap:           2,
        localMouseTracker:  components.NewMouseTracker(),
        remoteMouseTracker: components.NewMouseTracker(),
    }

    homeDir := getHomeDir()
    v.localPanel = &filePanel{
        path:          homeDir,
        entries:       []FileEntry{{name: "..", isDir: true}},
        selectedIndex: 0,
        scrollOffset:  0,
        maxVisible:    20,
        side:          localPanelSide,
    }

    v.remotePanel = &filePanel{
        path:          "~/",
        entries:       []FileEntry{{name: "..", isDir: true}},
        selectedIndex: 0,
        scrollOffset:  0,
        maxVisible:    20,
        side:          remotePanelSide,
    }

    if err := v.updateLocalPanel(); err != nil {
        v.errorMessage = fmt.Sprintf("Failed to load local directory: %v", err)
    }

    if v.model.GetSelectedHost() != nil {
        go v.initializeRemotePanel()
    }

    return v
}

func getHomeDir() string {
    if home, err := os.UserHomeDir(); err == nil {
        return home
    }
    return "."
}

// Init satisfies tea.Model. No async commands required here.
func (v *transferView) Init() tea.Cmd {
    return nil
}

// initializeRemotePanel establishes the remote SFTP connection asynchronously
func (v *transferView) initializeRemotePanel() {
    v.connecting = true
    v.model.Program.Send(transferStatusMsg{connecting: true})

    if err := v.ensureConnected(); err != nil {
        v.model.Program.Send(transferStatusMsg{connected: false, err: err})
        return
    }

    transfer := v.model.GetTransfer()
    if homeDir, err := transfer.GetRemoteHomeDir(); err == nil {
        v.remotePanel.path = homeDir
    }

    if err := v.updateRemotePanel(); err != nil {
        v.model.Program.Send(transferStatusMsg{connected: false, err: err})
        return
    }

    v.model.Program.Send(transferStatusMsg{connected: true})
}

// Update processes keyboard, mouse, and async messages
func (v *transferView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        v.width = msg.Width
        v.height = msg.Height
        v.model.UpdateWindowSize(msg.Width, msg.Height)
        v.recalculatePanelMetrics()
        return v, nil

    case transferStatusMsg:
        v.connecting = msg.connecting
        v.connected = msg.connected
        if msg.err != nil {
            v.errorMessage = fmt.Sprintf("Connection failed: %v", msg.err)
        } else {
            v.errorMessage = ""
        }
        return v, nil

    case transferSuccessMsg:
        v.transferring = false
        v.statusMessage = fmt.Sprintf("Transfer completed: %s", msg.filename)
        v.errorMessage = ""
        return v, nil

    case transferErrorMsg:
        v.transferring = false
        v.statusMessage = ""
        v.errorMessage = fmt.Sprintf("Transfer failed: %v", msg.err)
        return v, nil

    case tea.MouseMsg:
        if msg.Type == tea.MouseLeft {
            return v.handleMouse(msg)
        }

    case tea.KeyMsg:
        if v.popup != nil {
            return v.handlePopupKey(msg)
        }
        return v.handleKey(msg)
    }

    return v, nil
}

func (v *transferView) handlePopupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "esc", "enter":
        v.popup = nil
    case "y", "Y":
        if v.popup != nil && v.popup.Type == components.PopupDelete {
            v.popup = nil
            return v.executeDelete()
        }
    case "n", "N":
        if v.popup != nil && v.popup.Type == components.PopupDelete {
            v.popup = nil
        }
    }
    return v, nil
}

func (v *transferView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "q", "esc":
        v.model.SetActiveView(ui.ViewMain)
        return NewMainView(v.model), nil

    case "tab":
        v.switchActivePanel()
        return v, nil

    case "up", "w":
        v.getActivePanel().selectPrev()
        return v, nil

    case "down", "s":
        v.getActivePanel().selectNext()
        return v, nil

    case "enter":
        return v.handleEnter()

    case "f5", "c":
        return v.handleCopy()

    case "f8", "d":
        return v.promptDelete()

    case " ":
        // Placeholder for multi-selection toggle
        return v, nil
    }
    return v, nil
}

func (v *transferView) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
    // Determine which panel the click occurred in
    if area, ok := v.hitTest(v.localPanel, msg.X, msg.Y); ok {
        v.activePanel = localPanelSide
        v.localPanel.selectedIndex = area.index
        _, activated := v.localMouseTracker.HandleClick(msg.X, msg.Y, fmt.Sprintf("local-%d", area.index))
        if activated {
            return v.handleEnter()
        }
        return v, nil
    }

    if area, ok := v.hitTest(v.remotePanel, msg.X, msg.Y); ok {
        v.activePanel = remotePanelSide
        v.remotePanel.selectedIndex = area.index
        _, activated := v.remoteMouseTracker.HandleClick(msg.X, msg.Y, fmt.Sprintf("remote-%d", area.index))
        if activated {
            return v.handleEnter()
        }
        return v, nil
    }

    return v, nil
}

func (v *transferView) handleEnter() (tea.Model, tea.Cmd) {
    panel := v.getActivePanel()
    if panel.selectedIndex < 0 || panel.selectedIndex >= len(panel.entries) {
        return v, nil
    }

    entry := panel.entries[panel.selectedIndex]
    if !entry.isDir {
        return v, nil
    }

    if entry.name == ".." {
        panel.path = filepath.Dir(panel.path)
    } else {
        panel.path = filepath.Join(panel.path, entry.name)
    }

    if panel.side == localPanelSide {
        if err := v.updateLocalPanel(); err != nil {
            v.errorMessage = fmt.Sprintf("Failed to open directory: %v", err)
        }
    } else {
        if err := v.updateRemotePanel(); err != nil {
            v.errorMessage = fmt.Sprintf("Failed to open directory: %v", err)
        }
    }

    return v, nil
}

func (v *transferView) handleCopy() (tea.Model, tea.Cmd) {
    if !v.connected {
        v.popup = components.NewPopup(
            components.PopupMessage,
            "Not Connected",
            "Connect to a host before transferring files.",
            48, 7, v.width, v.height,
        )
        return v, nil
    }

    panel := v.getActivePanel()
    if panel.selectedIndex < 0 || panel.selectedIndex >= len(panel.entries) {
        return v, nil
    }

    entry := panel.entries[panel.selectedIndex]
    if entry.name == ".." {
        return v, nil
    }

    go v.performTransfer(entry, panel.side)
    v.transferring = true
    v.statusMessage = fmt.Sprintf("Transferring %s...", entry.name)
    v.errorMessage = ""

    return v, nil
}

func (v *transferView) promptDelete() (tea.Model, tea.Cmd) {
    panel := v.getActivePanel()
    if panel.selectedIndex < 0 || panel.selectedIndex >= len(panel.entries) {
        return v, nil
    }

    entry := panel.entries[panel.selectedIndex]
    if entry.name == ".." {
        return v, nil
    }

    v.popup = components.NewPopup(
        components.PopupDelete,
        "Delete",
        fmt.Sprintf("Delete %s?", entry.name),
        46, 7, v.width, v.height,
    )

    return v, nil
}

func (v *transferView) executeDelete() (tea.Model, tea.Cmd) {
    panel := v.getActivePanel()
    entry := panel.entries[panel.selectedIndex]
    if panel.side == localPanelSide {
        localPath := filepath.Join(panel.path, entry.name)
        if entry.isDir {
            if err := os.RemoveAll(localPath); err != nil {
                v.errorMessage = fmt.Sprintf("Failed to delete: %v", err)
            }
        } else {
            if err := os.Remove(localPath); err != nil {
                v.errorMessage = fmt.Sprintf("Failed to delete: %v", err)
            }
        }
        _ = v.updateLocalPanel()
    } else {
        if !v.connected {
            v.errorMessage = "Not connected"
            return v, nil
        }
        transfer := v.model.GetTransfer()
        remotePath := filepath.Join(panel.path, entry.name)
        if err := transfer.RemoveRemoteFile(remotePath); err != nil {
            v.errorMessage = fmt.Sprintf("Failed to delete: %v", err)
        }
        _ = v.updateRemotePanel()
    }

    v.statusMessage = fmt.Sprintf("Deleted %s", entry.name)
    return v, nil
}

func (v *transferView) performTransfer(entry FileEntry, fromSide panelSide) {
    transfer := v.model.GetTransfer()
    var err error

    progressChan := make(chan ssh.TransferProgress, 10)
    defer close(progressChan)

    if fromSide == localPanelSide {
        localPath := filepath.Join(v.localPanel.path, entry.name)
        remotePath := filepath.Join(v.remotePanel.path, entry.name)
        err = transfer.UploadFile(localPath, remotePath, progressChan)
        _ = v.updateRemotePanel()
    } else {
        remotePath := filepath.Join(v.remotePanel.path, entry.name)
        localPath := filepath.Join(v.localPanel.path, entry.name)
        err = transfer.DownloadFile(remotePath, localPath, progressChan)
        _ = v.updateLocalPanel()
    }

    if err != nil {
        v.model.Program.Send(transferErrorMsg{err: err})
        return
    }
    v.model.Program.Send(transferSuccessMsg{filename: entry.name})
}

func (v *transferView) ensureConnected() error {
    transfer := v.model.GetTransfer()
    if transfer.IsConnected() {
        v.connected = true
        return nil
    }

    host := v.model.GetSelectedHost()
    if host == nil {
        return fmt.Errorf("no host selected")
    }

    var authData string
    if host.PasswordID < 0 {
        keyIndex := -(host.PasswordID + 1)
        keys := v.model.GetKeys()
        if keyIndex < 0 || keyIndex >= len(keys) {
            return fmt.Errorf("invalid key index")
        }
        keyPath, err := keys[keyIndex].GetKeyPath()
        if err != nil {
            return fmt.Errorf("failed to get key path: %v", err)
        }
        authData = keyPath
    } else {
        passwords := v.model.GetPasswords()
        if host.PasswordID < 0 || host.PasswordID >= len(passwords) {
            return fmt.Errorf("invalid password index")
        }
        decrypted, err := passwords[host.PasswordID].GetDecrypted(v.model.GetCipher())
        if err != nil {
            return fmt.Errorf("failed to decrypt password: %v", err)
        }
        authData = decrypted
    }

    if err := transfer.Connect(host, authData); err != nil {
        return err
    }

    v.connected = true
    return nil
}

func (v *transferView) getActivePanel() *filePanel {
    if v.activePanel == localPanelSide {
        return v.localPanel
    }
    return v.remotePanel
}

func (v *transferView) switchActivePanel() {
    if v.activePanel == localPanelSide {
        v.activePanel = remotePanelSide
    } else {
        v.activePanel = localPanelSide
    }
}

func (v *transferView) updateLocalPanel() error {
    entries, err := readLocalDirectory(v.localPanel.path)
    if err != nil {
        return err
    }
    v.localPanel.entries = entries
    v.localPanel.selectedIndex = min(v.localPanel.selectedIndex, len(entries)-1)
    if v.localPanel.selectedIndex < 0 {
        v.localPanel.selectedIndex = 0
    }
    v.localPanel.updateScroll()
    return nil
}

func (v *transferView) updateRemotePanel() error {
    if err := v.ensureConnected(); err != nil {
        return err
    }

    transfer := v.model.GetTransfer()
    fileInfos, err := transfer.ListRemoteFiles(v.remotePanel.path)
    if err != nil {
        return err
    }

    entries := []FileEntry{{name: "..", isDir: true, modTime: time.Now()}}
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

    v.remotePanel.entries = entries
    v.remotePanel.selectedIndex = min(v.remotePanel.selectedIndex, len(entries)-1)
    if v.remotePanel.selectedIndex < 0 {
        v.remotePanel.selectedIndex = 0
    }
    v.remotePanel.updateScroll()
    return nil
}

func readLocalDirectory(path string) ([]FileEntry, error) {
    dir, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer dir.Close()

    fileInfos, err := dir.Readdir(-1)
    if err != nil {
        return nil, err
    }

    entries := []FileEntry{{name: "..", isDir: true, modTime: time.Now()}}
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

func (panel *filePanel) selectNext() {
    if len(panel.entries) == 0 {
        return
    }
    panel.selectedIndex = (panel.selectedIndex + 1) % len(panel.entries)
    panel.updateScroll()
}

func (panel *filePanel) selectPrev() {
    if len(panel.entries) == 0 {
        return
    }
    panel.selectedIndex--
    if panel.selectedIndex < 0 {
        panel.selectedIndex = len(panel.entries) - 1
    }
    panel.updateScroll()
}

func (panel *filePanel) updateScroll() {
    if panel.selectedIndex < panel.scrollOffset {
        panel.scrollOffset = panel.selectedIndex
    } else if panel.selectedIndex >= panel.scrollOffset+panel.maxVisible {
        panel.scrollOffset = panel.selectedIndex - panel.maxVisible + 1
    }
}

func (v *transferView) View() string {
    v.recalculatePanelMetrics()

    header := v.renderHeader()
    mainContent := v.renderPanels()
    footer := v.renderFooter()

    content := lipgloss.JoinVertical(
        lipgloss.Left,
        header,
        "",
        mainContent,
        "",
        footer,
    )

    view := lipgloss.Place(
        v.width,
        v.height,
        lipgloss.Left,
        lipgloss.Top,
        content,
    )

    if v.popup != nil {
        return lipgloss.Place(
            v.width,
            v.height,
            lipgloss.Center,
            lipgloss.Center,
            view+"\n"+v.popup.Render(),
        )
    }

    return view
}

func (v *transferView) renderHeader() string {
    title := "File Transfer"
    if host := v.model.GetSelectedHost(); host != nil {
        switch {
        case v.transferring:
            title += fmt.Sprintf(" - Transferring to %s", host.Name)
        case v.connecting:
            title += fmt.Sprintf(" - Connecting to %s", host.Name)
        case v.connected:
            title += fmt.Sprintf(" - Connected to %s (%s)", host.Name, host.IP)
        default:
            title += fmt.Sprintf(" - Not connected to %s", host.Name)
        }
    }

    return ui.StyleHeader.Width(v.width).Render(title)
}

func (v *transferView) renderPanels() string {
    if v.connecting {
        return v.renderConnecting()
    }
    if !v.connected {
        return v.renderNotConnected()
    }

    left, leftAreas := v.renderFilePanel(v.localPanel, v.panelWidth, v.panelHeight, v.activePanel == localPanelSide, 0)
    right, rightAreas := v.renderFilePanel(v.remotePanel, v.panelWidth, v.panelHeight, v.activePanel == remotePanelSide, v.panelWidth+v.panelGap)

    v.localPanel.itemAreas = leftAreas
    v.remotePanel.itemAreas = rightAreas

    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        left,
        strings.Repeat(" ", v.panelGap),
        right,
    )
}

func (v *transferView) renderConnecting() string {
    message := lipgloss.JoinVertical(
        lipgloss.Center,
        ui.StyleInfo.Render("Establishing SFTP connection..."),
        "",
        ui.StyleMuted.Render("Please wait"),
    )

    return ui.StyleContainer.
        Width(v.width - 10).
        Height(v.height - 8).
        Align(lipgloss.Center, lipgloss.Center).
        Render(message)
}

func (v *transferView) renderNotConnected() string {
    message := lipgloss.JoinVertical(
        lipgloss.Center,
        ui.StyleError.Render("Not connected"),
        "",
        ui.StyleMuted.Render("Connect to a host from the main view"),
    )

    return ui.StyleContainer.
        Width(v.width - 10).
        Height(v.height - 8).
        Align(lipgloss.Center, lipgloss.Center).
        Render(message)
}

func (v *transferView) renderFilePanel(panel *filePanel, width, height int, isActive bool, startX int) (string, []itemArea) {
    panelStyle := ui.StylePanel
    titleStyle := ui.StylePanelTitle
    if isActive {
        panelStyle = ui.StylePanelActive
        titleStyle = ui.StylePanelTitleActive
    }

    var builder strings.Builder
    builder.WriteString(titleStyle.Width(width - 4).Render(v.panelTitle(panel.side)) + "\n\n")
    builder.WriteString(ui.StyleMuted.Render("📁 " + ui.TruncateText(panel.path, width-6)) + "\n\n")

    areas := make([]itemArea, 0, len(panel.entries))
    currentY := v.panelTop + 4 // account for header + padding

    endIndex := min(panel.scrollOffset+panel.maxVisible, len(panel.entries))
    visibleEntries := panel.entries[panel.scrollOffset:endIndex]

    for i, entry := range visibleEntries {
        actualIndex := panel.scrollOffset + i
        isSelected := actualIndex == panel.selectedIndex

        item := v.renderFileItem(entry, isSelected, width-6)
        heightLines := lipgloss.Height(item)
        if heightLines == 0 {
            heightLines = 1
        }

        areas = append(areas, itemArea{
            index:  actualIndex,
            startY: currentY,
            endY:   currentY + heightLines,
            startX: startX,
            endX:   startX + width,
        })

        builder.WriteString(item + "\n")
        currentY += heightLines
    }

    if len(panel.entries) > panel.maxVisible {
        scrollInfo := fmt.Sprintf("  %d-%d of %d",
            panel.scrollOffset+1,
            min(panel.scrollOffset+panel.maxVisible, len(panel.entries)),
            len(panel.entries))
        builder.WriteString("\n" + ui.StyleMuted.Render(scrollInfo))
    }

    return panelStyle.Width(width).Height(height).Render(builder.String()), areas
}

func (v *transferView) renderFileItem(entry FileEntry, isSelected bool, width int) string {
    style := ui.StyleListItem
    if isSelected {
        style = ui.StyleListItemSelected
    }

    icon := getFileIcon(entry)
    nameStyle := ui.StyleFile
    switch {
    case entry.name == "..":
        nameStyle = ui.StyleMuted
    case entry.isDir:
        nameStyle = ui.StyleDirectory
    case entry.mode&0111 != 0:
        nameStyle = ui.StyleExecutable
    }

    name := nameStyle.Render(entry.name)
    size := ""
    if !entry.isDir && entry.name != ".." {
        size = " " + ui.StyleMuted.Render(formatSize(entry.size))
    }

    return style.Width(width).Render(fmt.Sprintf("%s %s%s", icon, name, size))
}

func (v *transferView) renderFooter() string {
    statusLine := ""
    if v.errorMessage != "" {
        statusLine = ui.StyleError.Render("✗ " + v.errorMessage)
    } else if v.statusMessage != "" {
        statusLine = ui.StyleSuccess.Render("✓ " + v.statusMessage)
    }

    bindings := []struct {
        key  string
        desc string
    }{
        {"Tab", "Switch panel"},
        {"↑↓/w/s", "Navigate"},
        {"Enter", "Open directory"},
        {"F5/c", "Copy"},
        {"F8/d", "Delete"},
        {"Space", "Toggle select"},
        {"q/ESC", "Back to hosts"},
    }

    var parts []string
    for _, b := range bindings {
        parts = append(parts, fmt.Sprintf("%s %s", ui.StyleKeybinding.Render(b.key), ui.StyleMuted.Render(b.desc)))
    }

    footer := lipgloss.JoinVertical(
        lipgloss.Left,
        statusLine,
        ui.StyleFooter.Width(v.width).Render(strings.Join(parts, " │ ")),
    )

    return footer
}

func (v *transferView) panelTitle(side panelSide) string {
    if side == localPanelSide {
        return "Local Files"
    }
    return "Remote Files"
}

func (v *transferView) recalculatePanelMetrics() {
    usableWidth := max(v.width-6, 40)
    v.panelWidth = usableWidth / 2
    v.panelHeight = max(v.height-10, 12)
    if v.panelHeight < 8 {
        v.panelHeight = 8
    }

    if v.localPanel != nil {
        v.localPanel.maxVisible = max((v.panelHeight-6), 1)
    }
    if v.remotePanel != nil {
        v.remotePanel.maxVisible = max((v.panelHeight-6), 1)
    }

    headerHeight := lipgloss.Height(ui.StyleHeader.Render(""))
    if headerHeight == 0 {
        headerHeight = 1
    }
    v.panelTop = headerHeight + 2 // header + blank line
}

func (v *transferView) hitTest(panel *filePanel, x, y int) (itemArea, bool) {
    for _, area := range panel.itemAreas {
        if y >= area.startY && y <= area.endY && x >= area.startX && x <= area.endX {
            return area, true
        }
    }
    return itemArea{}, false
}

func getFileIcon(entry FileEntry) string {
    switch {
    case entry.name == "..":
        return "⬆"
    case entry.isDir:
        return "📁"
    case entry.mode&0111 != 0:
        return "⚙"
    }

    switch strings.ToLower(filepath.Ext(entry.name)) {
    case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z":
        return "📦"
    case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg":
        return "🖼"
    case ".mp3", ".wav", ".flac", ".ogg":
        return "🎵"
    case ".mp4", ".avi", ".mkv", ".mov":
        return "🎬"
    case ".pdf":
        return "📄"
    case ".txt", ".md", ".doc", ".docx":
        return "📝"
    case ".go":
        return "🐹"
    case ".py":
        return "🐍"
    case ".js", ".ts":
        return "📜"
    case ".json", ".yaml", ".yml", ".toml":
        return "⚙"
    default:
        return "📄"
    }
}

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
    return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

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
