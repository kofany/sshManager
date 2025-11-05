package views

import (
    "fmt"
    "path/filepath"
    "sshManager/internal/config"
    "sshManager/internal/models"
    "sshManager/internal/ssh"
    "sshManager/internal/sync"
    "sshManager/internal/ui"
    "sshManager/internal/ui/components"
    "sshManager/internal/ui/messages"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// Message types
type hostKeyVerificationMsg struct {
    IP          string
    Port        string
    Fingerprint string
}

type connectSuccessMsg struct{}

type errMsg string

func (e errMsg) Error() string {
    return string(e)
}

// mainView is the redesigned main view with mouse support and consistent styling
type mainView struct {
    model          *ui.Model
    hosts          []models.Host
    selectedIndex  int
    width          int
    height         int
    regionManager  *components.ClickableRegionManager
    popup          *components.Popup
    statusMessage  string
    errorMessage   string
    connecting     bool
    escPressed     bool
    escTimeout     *time.Timer
    waitingForKey  bool
    pendingConn    struct {
        host     *models.Host
        password string
    }
}

// NewMainView creates a new redesigned main view
func NewMainView(model *ui.Model) *mainView {
    return &mainView{
        model:         model,
        hosts:         model.GetHosts(),
        width:         model.GetTerminalWidth(),
        height:        model.GetTerminalHeight(),
        regionManager: components.NewClickableRegionManager(),
    }
}

func (v *mainView) Init() tea.Cmd {
    return tea.Sequence(
        tea.EnterAltScreen,
        tea.ClearScreen,
    )
}

func (v *mainView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        v.width = msg.Width
        v.height = msg.Height
        v.model.UpdateWindowSize(msg.Width, msg.Height)
        return v, nil

    case tea.MouseMsg:
        if msg.Type == tea.MouseLeft {
            return v.handleMouseClick(msg)
        }

    case hostKeyVerificationMsg:
        v.popup = components.NewPopup(
            components.PopupHostKey,
            "Host Key Verification",
            fmt.Sprintf("New host key for %s:%s\n\nKey fingerprint:\n%s\n",
                msg.IP, msg.Port, msg.Fingerprint),
            70, 12, v.width, v.height,
        )
        return v, nil

    case connectSuccessMsg:
        v.connecting = true
        v.popup = components.NewPopup(
            components.PopupMessage,
            "SSH Connection",
            "Connecting to host...",
            50, 7, v.width, v.height,
        )
        return v, tea.Quit

    case errMsg:
        v.popup = components.NewPopup(
            components.PopupMessage,
            "Error",
            string(msg),
            50, 7, v.width, v.height,
        )
        return v, nil

    case messages.ReloadAppMsg:
        v.model.SetQuitting(true)
        return v, tea.Quit

    case tea.KeyMsg:
        // Handle popup keys
        if v.popup != nil {
            switch msg.String() {
            case "esc", "enter":
                if v.popup.Type == components.PopupMessage || v.popup.Type == components.PopupSessionEnded {
                    v.popup = nil
                    return v, nil
                }
            case "y", "Y":
                if v.popup.Type == components.PopupHostKey && v.waitingForKey {
                    return v.handleHostKeyAcceptance()
                }
            case "n", "N":
                if v.popup.Type == components.PopupHostKey && v.waitingForKey {
                    v.waitingForKey = false
                    v.popup = components.NewPopup(
                        components.PopupMessage,
                        "Connection Cancelled",
                        "Host key was not accepted",
                        50, 7, v.width, v.height,
                    )
                    return v, nil
                }
            }
            return v, nil
        }

        // Handle main view keys
        switch msg.String() {
        case "q", "ctrl+c":
            if !v.connecting {
                v.model.SetQuitting(true)
                return v, tea.Quit
            }

        case "up", "w":
            if len(v.hosts) > 0 && !v.connecting {
                v.selectedIndex--
                if v.selectedIndex < 0 {
                    v.selectedIndex = len(v.hosts) - 1
                }
            }

        case "down", "s":
            if len(v.hosts) > 0 && !v.connecting {
                v.selectedIndex++
                if v.selectedIndex >= len(v.hosts) {
                    v.selectedIndex = 0
                }
            }

        case "enter", "c":
            if !v.connecting && len(v.hosts) > 0 {
                return v.handleConnect()
            }

        case "e", "f4":
            if !v.connecting && len(v.hosts) > 0 {
                editView := NewEditView(v.model)
                editView.currentHost = &v.hosts[v.selectedIndex]
                editView.editingHost = true
                editView.editing = true
                editView.mode = modeNormal
                editView.initializeHostInputs()
                return editView, nil
            }

        case "h":
            if !v.connecting {
                editView := NewEditView(v.model)
                editView.editingHost = true
                editView.editing = true
                editView.mode = modeNormal
                editView.initializeHostInputs()
                return editView, nil
            }

        case "p":
            if !v.connecting {
                editView := NewEditView(v.model)
                editView.mode = modePasswordList
                editView.editing = true
                editView.passwords = v.model.GetPasswords()
                editView.selectedItemIndex = 0
                return editView, nil
            }

        case "k":
            if !v.connecting {
                editView := NewEditView(v.model)
                editView.mode = modeKeyList
                editView.editing = true
                editView.keys = v.model.GetKeys()
                editView.selectedItemIndex = 0
                return editView, nil
            }

        case "t":
            if !v.connecting && len(v.hosts) > 0 {
                return v.handleTransfer()
            }

        case "d", "f8":
            if !v.connecting && len(v.hosts) > 0 {
                return v.handleDelete()
            }

        case " ":
            if !v.connecting {
                ui.SwitchTheme()
                ui.ApplyCurrentTheme()
                return v, nil
            }

        case "ctrl+r":
            return v.handleRestoreBackup()
        }
    }

    return v, nil
}

func (v *mainView) handleMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
    // Handle popup clicks
    if v.popup != nil {
        // Popup handles its own clicks
        return v, nil
    }

    // Handle list item clicks
    regionID, activated := v.regionManager.HandleMouseClick(msg)
    if regionID != "" {
        // Find the host by ID and select it
        for i := range v.hosts {
            if fmt.Sprintf("host-%d", i) == regionID {
                v.selectedIndex = i
                if activated {
                    // Double-click - connect
                    return v.handleConnect()
                }
                break
            }
        }
    }

    return v, nil
}

func (v *mainView) handleConnect() (tea.Model, tea.Cmd) {
    if v.selectedIndex < 0 || v.selectedIndex >= len(v.hosts) {
        return v, nil
    }

    host := v.hosts[v.selectedIndex]
    v.model.SetSelectedHost(&host)

    return v, func() tea.Msg {
        var authData string

        // Handle authentication
        if host.PasswordID < 0 {
            // SSH Key
            keyIndex := -(host.PasswordID + 1)
            keys := v.model.GetKeys()
            if keyIndex >= len(keys) {
                return errMsg("Invalid key ID")
            }
            key := keys[keyIndex]
            keyPath, err := key.GetKeyPath()
            if err != nil {
                return errMsg(fmt.Sprintf("Failed to get key path: %v", err))
            }
            authData = keyPath
        } else {
            // Password
            passwords := v.model.GetPasswords()
            if host.PasswordID >= len(passwords) {
                return errMsg("Invalid password ID")
            }
            password := passwords[host.PasswordID]
            decryptedPass, err := password.GetDecrypted(v.model.GetCipher())
            if err != nil {
                return errMsg(fmt.Sprintf("Failed to decrypt password: %v", err))
            }
            authData = decryptedPass
        }

        // Create SSH client
        sshClient := ssh.NewSSHClient(v.model.GetPasswords())
        connectionDone := make(chan error, 1)
        
        go func() {
            connectionDone <- sshClient.Connect(&host, authData)
        }()

        // Wait for connection with timeout
        select {
        case err := <-connectionDone:
            if err != nil {
                // Check for host key verification
                if verificationRequired, ok := err.(*ssh.HostKeyVerificationRequired); ok {
                    fingerprint, err := ssh.GetHostKeyFingerprint(&host)
                    if err != nil {
                        return errMsg(fmt.Sprintf("Cannot retrieve key fingerprint: %v", err))
                    }

                    v.waitingForKey = true
                    v.pendingConn.host = &host
                    v.pendingConn.password = authData

                    return hostKeyVerificationMsg{
                        IP:          verificationRequired.IP,
                        Port:        verificationRequired.Port,
                        Fingerprint: fingerprint,
                    }
                }
                return errMsg(fmt.Sprintf("Failed to connect: %v", err))
            }

            v.model.SetSSHClient(sshClient)
            return connectSuccessMsg{}

        case <-time.After(7 * time.Second):
            return errMsg("Connection timed out")
        }
    }
}

func (v *mainView) handleHostKeyAcceptance() (tea.Model, tea.Cmd) {
    v.waitingForKey = false

    sshClient := ssh.NewSSHClient(v.model.GetPasswords())
    err := sshClient.ConnectWithAcceptedKey(
        v.pendingConn.host,
        v.pendingConn.password,
    )

    if err != nil {
        v.popup = components.NewPopup(
            components.PopupMessage,
            "Connection Error",
            fmt.Sprintf("Failed to connect: %v", err),
            50, 7, v.width, v.height,
        )
        return v, nil
    }

    v.model.SetSSHClient(sshClient)
    v.connecting = true
    v.popup = components.NewPopup(
        components.PopupMessage,
        "SSH Connection",
        "Connecting...",
        50, 7, v.width, v.height,
    )

    return v, tea.Quit
}

func (v *mainView) handleTransfer() (tea.Model, tea.Cmd) {
    if v.selectedIndex < 0 || v.selectedIndex >= len(v.hosts) {
        return v, nil
    }

    host := v.hosts[v.selectedIndex]
    v.model.SetSelectedHost(&host)
    v.model.SetActiveView(ui.ViewTransfer)

    return NewTransferView(v.model), nil
}

func (v *mainView) handleDelete() (tea.Model, tea.Cmd) {
    host := v.hosts[v.selectedIndex]
    if err := v.model.DeleteHost(host.Name); err != nil {
        v.errorMessage = fmt.Sprintf("Failed to delete host: %v", err)
    } else {
        if err := v.model.SaveConfig(); err != nil {
            v.errorMessage = fmt.Sprintf("Failed to save configuration: %v", err)
            return v, nil
        }
        v.hosts = v.model.GetHosts()
        if v.selectedIndex >= len(v.hosts) {
            v.selectedIndex = len(v.hosts) - 1
        }
        if v.selectedIndex < 0 {
            v.selectedIndex = 0
        }
        v.statusMessage = "Host deleted successfully"
    }
    return v, nil
}

func (v *mainView) handleRestoreBackup() (tea.Model, tea.Cmd) {
    configPath, err := config.GetDefaultConfigPath()
    if err != nil {
        v.popup = components.NewPopup(
            components.PopupMessage,
            "Error",
            fmt.Sprintf("Could not determine config path: %v", err),
            50, 7, v.width, v.height,
        )
        return v, nil
    }

    keysDir := filepath.Join(filepath.Dir(configPath), config.DefaultKeysDir)
    if err := sync.RestoreFromBackup(configPath, keysDir); err != nil {
        v.popup = components.NewPopup(
            components.PopupMessage,
            "Restore Failed",
            fmt.Sprintf("Could not restore from backup: %v", err),
            50, 7, v.width, v.height,
        )
        return v, nil
    }

    v.model.GetConfig().Load()
    v.model.UpdateLists()
    v.hosts = v.model.GetHosts()

    v.popup = components.NewPopup(
        components.PopupMessage,
        "Restore Success",
        "Configuration restored from backup",
        50, 7, v.width, v.height,
    )

    return v, nil
}

func (v *mainView) View() string {
    // Header
    header := v.renderHeader()
    
    // Main content area
    mainContent := v.renderMainContent()
    
    // Footer with keybindings
    footer := v.renderFooter()
    
    // Combine all sections
    content := lipgloss.JoinVertical(
        lipgloss.Left,
        header,
        "",
        mainContent,
        "",
        footer,
    )
    
    // Place in viewport
    view := lipgloss.Place(
        v.width,
        v.height,
        lipgloss.Left,
        lipgloss.Top,
        content,
    )
    
    // Render popup on top if active
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

func (v *mainView) renderHeader() string {
    title := ui.StyleHeader.
        Width(v.width).
        Render("SSH Manager - https://sshm.io")
    
    return title
}

func (v *mainView) renderMainContent() string {
    if len(v.hosts) == 0 {
        emptyMsg := lipgloss.JoinVertical(
            lipgloss.Center,
            ui.StyleMuted.Render("No SSH hosts configured"),
            "",
            ui.StyleInfo.Render("Press 'h' to add a new host"),
        )
        
        container := ui.StyleContainer.
            Width(v.width - 10).
            Height(v.height - 15).
            Align(lipgloss.Center, lipgloss.Center)
        
        return container.Render(emptyMsg)
    }
    
    // Calculate layout
    panelWidth := (v.width - 20) / 2
    panelHeight := v.height - 15
    
    // Left panel: Host list
    leftPanel := v.renderHostList(panelWidth, panelHeight)
    
    // Right panel: Host details
    rightPanel := v.renderHostDetails(panelWidth, panelHeight)
    
    // Join panels horizontally
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        leftPanel,
        "  ", // Separator
        rightPanel,
    )
}

func (v *mainView) renderHostList(width, height int) string {
    // Clear and rebuild clickable regions
    v.regionManager.ClearRegions()
    
    var content strings.Builder
    
    // Panel title
    titleStyle := ui.StylePanelTitleActive
    if v.connecting {
        titleStyle = ui.StylePanelTitle
    }
    content.WriteString(titleStyle.Width(width - 4).Render("📋 Hosts") + "\n\n")
    
    // Render host items
    currentY := 4 // Start after title
    maxItems := (height - 6) / 3 // Rough estimate: 3 lines per item
    
    for i, host := range v.hosts {
        if i >= maxItems {
            break
        }
        
        itemContent := v.renderHostItem(host, i == v.selectedIndex, width-6)
        content.WriteString(itemContent + "\n")
        
        // Register clickable region
        itemHeight := strings.Count(itemContent, "\n") + 1
        v.regionManager.AddRegion(components.ClickableRegion{
            X:      5,
            Y:      currentY,
            Width:  width - 6,
            Height: itemHeight,
            ID:     fmt.Sprintf("host-%d", i),
        })
        
        currentY += itemHeight + 1
    }
    
    // Wrap in panel
    panel := ui.StylePanelActive.
        Width(width).
        Height(height)
    
    return panel.Render(content.String())
}

func (v *mainView) renderHostItem(host models.Host, isSelected bool, width int) string {
    var style lipgloss.Style
    if isSelected {
        style = ui.StyleCardSelected.Width(width)
    } else {
        style = ui.StyleCard.Width(width)
    }
    
    // Build item content
    icon := "🖥️ "
    if isSelected {
        icon = "▶️ "
    }
    
    title := ui.StyleValue.Bold(true).Render(icon + host.Name)
    addr := ui.StyleMuted.Render(fmt.Sprintf("  %s@%s:%s", host.Login, host.IP, host.Port))
    desc := ""
    if host.Description != "" {
        desc = "\n" + ui.StyleMuted.Render("  "+ui.TruncateText(host.Description, width-6))
    }
    
    itemContent := lipgloss.JoinVertical(
        lipgloss.Left,
        title,
        addr+desc,
    )
    
    return style.Render(itemContent)
}

func (v *mainView) renderHostDetails(width, height int) string {
    var content strings.Builder
    
    // Panel title
    content.WriteString(ui.StylePanelTitle.Width(width - 4).Render("ℹ️  Details") + "\n\n")
    
    if v.selectedIndex >= 0 && v.selectedIndex < len(v.hosts) {
        host := v.hosts[v.selectedIndex]
        
        // Render details
        details := []struct {
            label string
            value string
        }{
            {"Name", host.Name},
            {"Description", host.Description},
            {"Login", host.Login},
            {"Address", host.IP},
            {"Port", host.Port},
        }
        
        for _, detail := range details {
            label := ui.StyleLabel.Render(detail.label + ":")
            value := ui.StyleValue.Render(detail.value)
            content.WriteString(fmt.Sprintf("%s %s\n", label, value))
        }
        
        // Authentication info
        content.WriteString("\n")
        if host.PasswordID < 0 {
            authType := ui.StyleSuccess.Render("🔑 SSH Key Authentication")
            content.WriteString(authType + "\n")
        } else {
            authType := ui.StyleInfo.Render("🔐 Password Authentication")
            content.WriteString(authType + "\n")
        }
    }
    
    // Status messages
    if v.statusMessage != "" {
        content.WriteString("\n" + ui.StyleSuccess.Render("✓ " + v.statusMessage))
    }
    if v.errorMessage != "" {
        content.WriteString("\n" + ui.StyleError.Render("✗ " + v.errorMessage))
    }
    
    // Wrap in panel
    panel := ui.StylePanel.
        Width(width).
        Height(height)
    
    return panel.Render(content.String())
}

func (v *mainView) renderFooter() string {
    // Keybindings
    bindings := []struct {
        key  string
        desc string
    }{
        {"↑↓/w/s", "Navigate"},
        {"Enter/c", "Connect"},
        {"e/F4", "Edit"},
        {"h", "New Host"},
        {"p", "Passwords"},
        {"k", "Keys"},
        {"t", "Transfer"},
        {"d/F8", "Delete"},
        {"Space", "Theme"},
        {"Ctrl+R", "Restore"},
        {"q", "Quit"},
    }
    
    var keys []string
    for _, b := range bindings {
        key := ui.StyleKeybinding.Render(b.key)
        desc := ui.StyleMuted.Render(b.desc)
        keys = append(keys, fmt.Sprintf("%s %s", key, desc))
    }
    
    footer := ui.StyleFooter.
        Width(v.width).
        Render(strings.Join(keys, " │ "))
    
    return footer
}

// ShowSessionEndedPopup shows a popup when SSH session ends
func (v *mainView) ShowSessionEndedPopup() {
    v.popup = components.NewPopup(
        components.PopupSessionEnded,
        "SSH Session Ended",
        "The SSH session has been closed.\nPress ESC or Enter to continue.",
        50, 7, v.width, v.height,
    )
}

// PostInitialize reinitializes the view after returning from SSH session
func (v *mainView) PostInitialize() tea.Cmd {
    return tea.Sequence(
        tea.EnterAltScreen,
        tea.ClearScreen,
    )
}
