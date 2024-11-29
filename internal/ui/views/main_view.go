package views

import (
	"fmt"
	"sshManager/internal/models"
	"sshManager/internal/ui"
	"strings"
	"time"

	"sshManager/internal/ssh"

	"sshManager/internal/ui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

type mainView struct {
	model                     *ui.Model
	hosts                     []models.Host
	selectedIndex             int
	currentDir                string
	showHostList              bool
	errMsg                    string
	status                    string
	connecting                bool
	width                     int
	height                    int
	escPressed                bool
	escTimeout                *time.Timer
	waitingForKeyConfirmation bool
	hostKeyFingerprint        string
	pendingConnection         struct {
		host     *models.Host
		password string
	}
	popup *components.Popup // Dodane nowe pole

}

type connectError string

type connectFinishedMsg struct {
	err error
}

func (e connectError) Error() string {
	return string(e)
}

func NewMainView(model *ui.Model) *mainView {
	return &mainView{
		model:        model,
		showHostList: true,
		hosts:        model.GetHosts(),
		currentDir:   getHomeDir(),
		width:        model.GetTerminalWidth(),  // Dodane
		height:       model.GetTerminalHeight(), // Dodane

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

	case tea.KeyMsg:
		// Obsługa klawiszy dla popupu
		if v.popup != nil {
			switch msg.String() {
			case "esc", "enter":
				if v.popup.Type == components.PopupMessage {
					v.popup = nil
					return v, nil
				}
			case "y", "Y":
				if v.popup.Type == components.PopupHostKey && v.waitingForKeyConfirmation {
					v.waitingForKeyConfirmation = false
					cmd, err := ssh.CreateSSHCommand(v.pendingConnection.host, v.pendingConnection.password, true)
					if err != nil {
						v.popup = components.NewPopup(
							components.PopupMessage,
							"Błąd połączenia",
							fmt.Sprintf("Failed to create SSH command: %v", err),
							50,
							7,
							v.width,
							v.height,
						)
						return v, nil
					}
					v.connecting = true
					v.popup = components.NewPopup(
						components.PopupMessage,
						"SSH",
						"Connecting...",
						50,
						7,
						v.width,
						v.height,
					)
					return v, tea.ExecProcess(
						cmd,
						func(err error) tea.Msg {
							return connectFinishedMsg{err: err}
						},
					)
				}
			case "n", "N":
				if v.popup.Type == components.PopupHostKey && v.waitingForKeyConfirmation {
					v.waitingForKeyConfirmation = false
					v.popup = components.NewPopup(
						components.PopupMessage,
						"SSH",
						"Connection cancelled",
						50,
						7,
						v.width,
						v.height,
					)
					return v, nil
				}
			}
			return v, nil
		}

		// Standardowa obsługa klawiszy nawigacji
		switch msg.String() {
		case "q", "ctrl+c":
			v.model.SetQuitting(true)
			return v, tea.Quit

		case "up", "w":
			if len(v.hosts) > 0 && !v.connecting {
				v.selectedIndex--
				if v.selectedIndex < 0 {
					v.selectedIndex = len(v.hosts) - 1
				}
				v.errMsg = ""
			}

		case "down", "s":
			if len(v.hosts) > 0 && !v.connecting {
				v.selectedIndex++
				if v.selectedIndex >= len(v.hosts) {
					v.selectedIndex = 0
				}
				v.errMsg = ""
			}
		case "enter", "c":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			return v.handleConnect()
		case "k":
			if !v.connecting {
				editView := NewEditView(v.model)
				editView.mode = modeKeyList // Zmieniamy na modeKeyList zamiast modeKeyEdit
				editView.editing = true
				editView.keys = v.model.GetKeys() // Pobieramy listę kluczy
				editView.selectedItemIndex = 0    // Ustawiamy początkowy indeks zaznaczenia
				return editView, nil
			}
		case "e", "f4":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			editView := NewEditView(v.model)
			editView.currentHost = &v.hosts[v.selectedIndex]
			editView.editingHost = true
			editView.editing = true
			editView.mode = modeNormal
			editView.initializeHostInputs()
			return editView, nil

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

		case "t":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			return v.handleTransfer()

		case "d", "f8":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			return v.handleDelete()
		case " ": // spacja
			if !v.connecting && len(v.hosts) > 0 {
				ui.SwitchTheme()
				return v, nil
			}

		case "esc":
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

		// Obsługa sekwencji ESC
		if v.escPressed {
			switch msg.String() {
			case "4":
				if len(v.hosts) > 0 && !v.connecting {
					editView := NewEditView(v.model)
					editView.currentHost = &v.hosts[v.selectedIndex]
					editView.editingHost = true
					editView.editing = true
					editView.mode = modeNormal
					editView.initializeHostInputs()
					return editView, nil
				}
				v.escPressed = false
				return v, nil
			case "8":
				if len(v.hosts) > 0 && !v.connecting {
					return v.handleDelete()
				}
				v.escPressed = false
				return v, nil
			}
			v.escPressed = false
			if v.escTimeout != nil {
				v.escTimeout.Stop()
			}
			return v, nil
		}

	case connectFinishedMsg:
		v.connecting = false
		if msg.err != nil {
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Błąd połączenia",
				fmt.Sprintf("SSH connection failed: %v", msg.err),
				50,
				7,
				v.width,
				v.height,
			)
		} else {
			v.popup = nil
		}
		return v, nil

	case connectError:
		v.popup = components.NewPopup(
			components.PopupMessage,
			"Błąd połączenia",
			msg.Error(),
			50,
			7,
			v.width,
			v.height,
		)
		v.connecting = false
		return v, nil
	}

	return v, nil
}

func (v *mainView) handleConnect() (tea.Model, tea.Cmd) {
	host := v.hosts[v.selectedIndex]
	v.model.SetSelectedHost(&host)

	passwords := v.model.GetPasswords()
	if host.PasswordID >= len(passwords) {
		v.popup = components.NewPopup(
			components.PopupMessage,
			"Connection Error",
			"Invalid password ID",
			50,
			7,
			v.width,
			v.height,
		)
		return v, nil
	}

	password := passwords[host.PasswordID]
	decryptedPass, err := password.GetDecrypted(v.model.GetCipher())
	if err != nil {
		v.popup = components.NewPopup(
			components.PopupMessage,
			"Connection Error",
			fmt.Sprintf("Failed to decrypt password: %v", err),
			50,
			7,
			v.width,
			v.height,
		)
		return v, nil
	}

	cmd, err := ssh.CreateSSHCommand(&host, decryptedPass, false)
	if err != nil {
		if verificationRequired, ok := err.(*ssh.HostKeyVerificationRequired); ok {
			fingerprint, err := ssh.GetHostKeyFingerprint(&host)
			if err != nil {
				v.popup = components.NewPopup(
					components.PopupMessage,
					"Key Verification Error",
					fmt.Sprintf("Cannot retrieve key fingerprint: %v", err),
					50,
					7,
					v.width,
					v.height,
				)
				return v, nil
			}

			v.waitingForKeyConfirmation = true
			v.hostKeyFingerprint = fingerprint
			v.pendingConnection.host = &host
			v.pendingConnection.password = decryptedPass

			v.popup = components.NewPopup(
				components.PopupHostKey,
				"Host Key Verification",
				fmt.Sprintf("New host key for %s:%s\n\nKey fingerprint:\n%s\n",
					verificationRequired.IP, verificationRequired.Port, fingerprint),
				70,
				12,
				v.width,
				v.height,
			)
			return v, nil
		}
		v.popup = components.NewPopup(
			components.PopupMessage,
			"Connection Error",
			fmt.Sprintf("Failed to create SSH command: %v", err),
			50,
			7,
			v.width,
			v.height,
		)
		return v, nil
	}

	v.connecting = true
	v.popup = components.NewPopup(
		components.PopupMessage,
		"SSH",
		"Connecting...",
		50,
		7,
		v.width,
		v.height,
	)

	return v, tea.ExecProcess(
		cmd,
		func(err error) tea.Msg {
			return connectFinishedMsg{err: err}
		},
	)
}

func (v *mainView) handleDelete() (tea.Model, tea.Cmd) {
	host := v.hosts[v.selectedIndex]
	if err := v.model.DeleteHost(host.Name); err != nil {
		v.errMsg = fmt.Sprintf("Failed to delete host: %v", err)
	} else {
		if err := v.model.SaveConfig(); err != nil {
			v.errMsg = fmt.Sprintf("Failed to save configuration: %v", err)
			return v, nil
		}
		v.hosts = v.model.GetHosts()
		if v.selectedIndex >= len(v.hosts) {
			v.selectedIndex = len(v.hosts) - 1
		}
		v.status = "Host deleted successfully"
	}
	return v, nil
}

// W pliku internal/ui/views/main.go
func (v *mainView) handleTransfer() (tea.Model, tea.Cmd) {
	host := v.hosts[v.selectedIndex]
	v.model.SetSelectedHost(&host)

	passwords := v.model.GetPasswords()
	if host.PasswordID >= len(passwords) {
		v.errMsg = "Invalid password ID"
		return v, nil
	}

	password := passwords[host.PasswordID]
	decryptedPass, err := password.GetDecrypted(v.model.GetCipher())
	if err != nil {
		v.errMsg = fmt.Sprintf("Failed to decrypt password: %v", err)
		return v, nil
	}

	transfer := v.model.GetTransfer()
	if err := transfer.Connect(&host, decryptedPass); err != nil {
		v.errMsg = fmt.Sprintf("Failed to establish SFTP connection: %v", err)
		return v, nil
	}

	v.model.SetActiveView(ui.ViewTransfer)

	// Dodajemy sequence komend
	return v, tea.Sequence(
		tea.ClearScreen,
		func() tea.Msg {
			return tea.WindowSizeMsg{
				Width:  v.width,
				Height: v.height,
			}
		},
	)
}

func (v *mainView) View() string {
	// Przygotuj główną zawartość
	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("SSH Manager") + "\n\n")

	// Główny layout w stylu MC z dwoma panelami
	leftPanel := v.renderHostPanel()
	rightPanel := v.renderDetailsPanel()

	// Połącz panele horyzontalnie
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Left,
		leftPanel,
		"  +  ", // separator
		rightPanel,
	)

	content.WriteString(mainContent + "\n\n")

	// Status bar i Command bar
	statusAndCmdBar := v.renderStatusBar()
	content.WriteString(statusAndCmdBar + "\n")

	// Zastosuj styl ramki do całej zawartości
	framedContent := ui.WindowStyle.Render(content.String())

	// Podstawowy widok
	baseView := lipgloss.Place(
		v.width,
		v.height,
		lipgloss.Center,
		lipgloss.Center,
		framedContent,
		lipgloss.WithWhitespaceChars(""),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)

	// Jeśli jest aktywny popup, renderuj go na wierzchu
	if v.popup != nil {
		return lipgloss.Place(
			v.width,
			v.height,
			lipgloss.Center,
			lipgloss.Center,
			baseView+"\n"+v.popup.Render(),
			lipgloss.WithWhitespaceChars(""),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
		)
	}

	return baseView
}
func (v *mainView) renderHostPanel() string {
	style := ui.PanelStyle.Width(45)
	title := "Available Hosts"

	var content strings.Builder
	if len(v.hosts) == 0 {
		content.WriteString(ui.DescriptionStyle.Render("\n  No hosts available\n  Press 'n' to add new host"))
	} else {
		for i, host := range v.hosts {
			prefix := "  "
			var line string

			// Renderujemy nazwę hosta z użyciem HostStyle
			hostName := ui.HostStyle.Render(host.Name)

			if i == v.selectedIndex {
				// Ustawiamy prefix dla zaznaczonego hosta
				prefix = ui.SuccessStyle.Render("❯ ")
				// Budujemy linię z użyciem SelectedItemStyle i HostStyle
				line = ui.SelectedItemStyle.Render(
					fmt.Sprintf("\n%s%s", prefix, hostName),
				)
			} else {
				// Budujemy linię dla niezaznaczonego hosta z HostStyle
				line = fmt.Sprintf("\n%s%s", prefix, hostName)
			}
			// Dodajemy linię do zawartości
			content.WriteString(line)
		}
	}

	return style.Render(title + "\n" + content.String())
}

func (v *mainView) renderDetailsPanel() string {
	style := ui.PanelStyle.Width(45)
	title := "Host Details"

	var content strings.Builder
	if len(v.hosts) > 0 {
		host := v.hosts[v.selectedIndex]
		content.WriteString(fmt.Sprintf("\n  %s %s", ui.LabelStyle.Render("Name:"), ui.Infotext.Render(host.Name)))
		content.WriteString(fmt.Sprintf("\n  %s %s", ui.LabelStyle.Render("Description:"), ui.Infotext.Render(host.Description)))
		content.WriteString(fmt.Sprintf("\n  %s %s", ui.LabelStyle.Render("Login:"), ui.Infotext.Render(host.Login)))
		content.WriteString(fmt.Sprintf("\n  %s %s", ui.LabelStyle.Render("Address:"), ui.Infotext.Render(host.IP)))
		content.WriteString(fmt.Sprintf("\n  %s %s", ui.LabelStyle.Render("Port:"), ui.Infotext.Render(host.Port)))
	}

	return style.Render(title + "\n" + content.String())
}

func (v *mainView) renderStatusBar() string {
	// Renderowanie paska statusu
	var status string
	if v.errMsg != "" {
		status = ui.ErrorStyle.Render(v.errMsg)
	} else if v.status != "" {
		status = ui.SuccessStyle.Render(v.status)
	} else if v.model.IsConnected() {
		if host := v.model.GetSelectedHost(); host != nil {
			status = ui.SuccessStyle.Render(fmt.Sprintf("Connected to: %s", host.Name))
		}
	} else {
		status = ui.DescriptionStyle.Render("No active connection, Press:")
	}

	// Renderowanie tabeli poleceń
	headers := []string{
		"Connect", "Navigate", "Edit Host", "Add Host", "Pass",
		"Transfer", "Delete Host", "List Keys", "Theme", "Quit",
	}
	shortcuts := []string{
		"enter/c", "↑↓/w/s", "e/f4/ESC+4", "h", "p",
		"t", "d/f8/ESC+8", "k", "space", "q/^c",
	}

	// Renderowanie wierszy tabeli
	var TableStyle = func(row, col int) lipgloss.Style {
		switch {
		case row == -1: // Nagłówki
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(ui.Subtle).
				Align(lipgloss.Center)
		default: // Skróty
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(ui.Special)
		}
	}

	cmdTable := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.StatusBar)).
		StyleFunc(TableStyle).
		Headers(headers...).
		Row(shortcuts...)

	// Połączenie statusu i tabeli w jedną ramkę
	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		status,            // Pasek statusu
		cmdTable.Render(), // Tabela poleceń
	)

	// Dodanie ramki wokół wszystkiego
	framed := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(fullContent)

	return framed
}
