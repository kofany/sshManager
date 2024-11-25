// internal/ui/views/main.go
package views

import (
	"fmt"
	"sshManager/internal/models"
	"sshManager/internal/ui"
	"strings"

	"sshManager/internal/ssh"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mainView struct {
	model         *ui.Model
	hosts         []models.Host
	selectedIndex int
	currentDir    string
	showHostList  bool
	errMsg        string
	status        string
	connecting    bool
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
	}
}

func (v *mainView) Init() tea.Cmd {
	return nil
}

// internal/ui/views/main.go - Part 2
func (v *mainView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			v.model.SetQuitting(true)
			return v, tea.Quit

		case "up", "k":
			if len(v.hosts) > 0 && !v.connecting {
				v.selectedIndex--
				if v.selectedIndex < 0 {
					v.selectedIndex = len(v.hosts) - 1
				}
				v.errMsg = ""
			}

		case "down", "j":
			if len(v.hosts) > 0 && !v.connecting {
				v.selectedIndex++
				if v.selectedIndex >= len(v.hosts) {
					v.selectedIndex = 0
				}
				v.errMsg = ""
			}
		case "enter":
			// Jeśli trwa łączenie lub nie ma hostów, ignorujemy
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			// Używamy tej samej logiki co dla klawisza "c"
			return v.handleConnect()

		case "c":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			return v.handleConnect()

		case "t":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			return v.handleTransfer()

		case "e":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			host := v.hosts[v.selectedIndex]
			v.model.SetSelectedHost(&host)
			v.model.SetActiveView(ui.ViewEdit)
			return v, nil

		case "d":
			if v.connecting || len(v.hosts) == 0 {
				return v, nil
			}
			return v.handleDelete()

		case "n":
			if !v.connecting {
				v.model.SetActiveView(ui.ViewEdit)
				return v, nil
			}

		case "r":
			if !v.connecting {
				v.hosts = v.model.GetHosts()
				v.errMsg = ""
				if v.selectedIndex >= len(v.hosts) {
					v.selectedIndex = len(v.hosts) - 1
				}
				v.status = "Host list refreshed"
			}
		}

	case connectFinishedMsg:
		v.connecting = false
		v.status = ""
		if msg.err != nil {
			v.errMsg = fmt.Sprintf("SSH connection failed: %v", msg.err)
		} else {
			v.errMsg = ""
		}
		// Dodaj komendę clear screen
		return v, tea.ClearScreen

	case connectError:
		v.errMsg = msg.Error()
		v.connecting = false
		v.status = ""
		return v, nil
	}

	return v, nil
}

func (v *mainView) handleConnect() (tea.Model, tea.Cmd) {
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

	v.connecting = true
	v.status = "Connecting..."

	cmd := ssh.CreateSSHCommand(&host, decryptedPass) // Używamy funkcji z pakietu ssh
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
	return v, nil
}

func (v *mainView) View() string {
	content := ui.TitleStyle.Render("SSH Manager") + "\n\n"

	// Główny layout w stylu MC z dwoma panelami
	leftPanel := v.renderHostPanel()
	rightPanel := v.renderDetailsPanel()

	// Połącz panele horyzontalnie
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Left,
		leftPanel,
		"  │  ", // separator
		rightPanel,
	)

	content += mainContent + "\n\n"

	// Status bar
	statusBar := v.renderStatusBar()
	content += statusBar + "\n"

	// Command bar
	cmdBar := v.renderCommandBar()
	content += cmdBar

	return ui.WindowStyle.Render(content)
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
			if i == v.selectedIndex {
				prefix = ui.SuccessStyle.Render("❯ ")
				content.WriteString(ui.SelectedItemStyle.Render(
					fmt.Sprintf("\n%s%s", prefix, host.Name),
				))
			} else {
				content.WriteString(fmt.Sprintf("\n%s%s", prefix, host.Name))
			}
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

	return ui.StatusBarStyle.Render(status)
}

func (v *mainView) renderCommandBar() string {
	commands := []struct {
		key  string
		desc string
	}{
		{"Enter/c", "Connect"},
		{"t", "File Transfer"},
		{"e", "Edit"},
		{"d", "Delete"},
		{"n", "New Host"},
		{"q", "Quit"},
	}

	var cmdBar strings.Builder
	for i, cmd := range commands {
		if i > 0 {
			cmdBar.WriteString(" ∥")
		}
		// Odwróć kolejność: najpierw opis, potem klawisz
		cmdBar.WriteString(ui.DescriptionStyle.Render(cmd.desc))
		cmdBar.WriteString(" ― ")
		cmdBar.WriteString(ui.ButtonStyle.Render(cmd.key))
	}

	return ui.CommandBarStyle.
		Align(lipgloss.Left).
		Render(cmdBar.String())
}
