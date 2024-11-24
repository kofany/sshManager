// internal/ui/views/main.go

package views

import (
	"fmt"
	"sshManager/internal/models"
	"sshManager/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

type mainView struct {
	model         *ui.Model
	hosts         []models.Host
	selectedIndex int
	showHostList  bool
	errMsg        string
}

func NewMainView(model *ui.Model) *mainView {
	return &mainView{
		model:        model,
		showHostList: true, // Start with host list visible
		hosts:        model.GetHosts(),
	}
}

func (v *mainView) Init() tea.Cmd {
	return nil
}

func (v *mainView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			v.model.SetStatus("", false)
			return v, tea.Quit

		case "up", "k":
			if v.showHostList && len(v.hosts) > 0 {
				v.selectedIndex--
				if v.selectedIndex < 0 {
					v.selectedIndex = len(v.hosts) - 1
				}
			}

		case "down", "j":
			if v.showHostList && len(v.hosts) > 0 {
				v.selectedIndex++
				if v.selectedIndex >= len(v.hosts) {
					v.selectedIndex = 0
				}
			}

		case "c":
			if v.showHostList && len(v.hosts) > 0 {
				host := v.hosts[v.selectedIndex]
				v.model.SetSelectedHost(&host)
				v.model.SetActiveView(ui.ViewConnect)
				return v, nil
			}

		case "e":
			v.model.SetActiveView(ui.ViewEdit)
			return v, nil

		case "t", "u":
			if !v.model.IsConnected() {
				v.errMsg = "Connection required for file transfer"
				return v, nil
			}
			v.model.SetActiveView(ui.ViewTransfer)
			return v, nil

		case "r":
			// Refresh host list
			v.hosts = v.model.GetHosts()
			v.errMsg = ""
			if v.selectedIndex >= len(v.hosts) {
				v.selectedIndex = len(v.hosts) - 1
			}
		}
	}
	return v, nil
}

func (v *mainView) View() string {
	var content string

	// Title and connection status
	content = ui.TitleStyle.Render("SSH Manager") + "\n\n"

	if v.model.IsConnected() {
		if host := v.model.GetSelectedHost(); host != nil {
			content += ui.SuccessStyle.Render(fmt.Sprintf("Connected to: %s", host.Name)) + "\n\n"
		}
	} else {
		content += ui.DescriptionStyle.Render("No active connection") + "\n\n"
	}

	// Host list
	if v.showHostList {
		if len(v.hosts) == 0 {
			content += ui.DescriptionStyle.Render("No hosts available. Press 'e' to add hosts.") + "\n\n"
		} else {
			content += ui.TitleStyle.Render("Available hosts:") + "\n\n"
			for i, host := range v.hosts {
				prefix := "  "
				if i == v.selectedIndex {
					prefix = "> "
					content += ui.SelectedItemStyle.Render(fmt.Sprintf("%s%s (%s)\n", prefix, host.Name, host.Description))
				} else {
					content += fmt.Sprintf("%s%s (%s)\n", prefix, host.Name, host.Description)
				}
			}
			content += "\n"
		}
	}

	// Controls
	content += "Available actions:\n\n"

	if len(v.hosts) > 0 {
		content += ui.ButtonStyle.Render("c") + " - Connect    "
		if v.model.IsConnected() {
			content += ui.ButtonStyle.Render("t") + " - Transfer files\n"
		} else {
			content += ui.ButtonStyle.Render("t") + " - Transfer files (requires connection)\n"
		}
	}

	content += ui.ButtonStyle.Render("e") + " - Edit hosts/passwords    " +
		ui.ButtonStyle.Render("r") + " - Refresh list    " +
		ui.ButtonStyle.Render("q") + " - Quit\n"

	// Navigation help
	if len(v.hosts) > 0 {
		content += ui.DescriptionStyle.Render("\nUse ↑/↓ arrows to navigate") + "\n"
	}

	// Statistics
	content += "\nStatistics:\n" +
		ui.DescriptionStyle.Render(
			fmt.Sprintf("Saved hosts: %s\n",
				ui.SuccessStyle.Render(fmt.Sprintf("%d", len(v.hosts))),
			)+
				fmt.Sprintf("Saved passwords: %s\n",
					ui.SuccessStyle.Render(fmt.Sprintf("%d", len(v.model.GetPasswords()))),
				),
		)

	// Error message
	if v.errMsg != "" {
		content += "\n" + ui.ErrorStyle.Render(v.errMsg)
	}

	return ui.WindowStyle.Render(content)
}
