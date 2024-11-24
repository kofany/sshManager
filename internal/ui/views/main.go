// internal/ui/views/main.go

package views

import (
	"fmt"
	"os"
	"os/exec"
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
	status        string
	connecting    bool
}

type connectError string

func (e connectError) Error() string {
	return string(e)
}

func NewMainView(model *ui.Model) *mainView {
	return &mainView{
		model:        model,
		showHostList: true,
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
			if v.showHostList && len(v.hosts) > 0 && !v.connecting {
				v.selectedIndex--
				if v.selectedIndex < 0 {
					v.selectedIndex = len(v.hosts) - 1
				}
				v.errMsg = ""
			}

		case "down", "j":
			if v.showHostList && len(v.hosts) > 0 && !v.connecting {
				v.selectedIndex++
				if v.selectedIndex >= len(v.hosts) {
					v.selectedIndex = 0
				}
				v.errMsg = ""
			}

		case "c":
			if v.connecting {
				return v, nil
			}
			if v.showHostList && len(v.hosts) > 0 {
				host := v.hosts[v.selectedIndex]
				v.model.SetSelectedHost(&host)

				// Pobierz i zdekoduj hasło
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
				return v, tea.Batch(
					tea.ExecProcess(
						createSSHCommand(&host, decryptedPass),
						func(err error) tea.Msg {
							if err != nil {
								return connectError(fmt.Sprintf("SSH connection failed: %v", err))
							}
							return nil
						},
					),
				)
			}

		case "t":
			if v.connecting {
				return v, nil
			}
			if v.showHostList && len(v.hosts) > 0 {
				host := v.hosts[v.selectedIndex]
				v.model.SetSelectedHost(&host)

				// NIE łączymy się przez SSH - tylko przechodzimy do widoku transferu
				// Transfer zostanie zainicjowany w tle w TransferView
				v.model.SetActiveView(ui.ViewTransfer)
				return v, nil
			}

		case "e":
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

	case connectError:
		v.errMsg = msg.Error()
		v.connecting = false
		v.status = ""
		return v, nil
	}

	return v, nil
}

func (v *mainView) View() string {
	var content string

	// Tytuł i status połączenia
	content = ui.TitleStyle.Render("SSH Manager") + "\n\n"

	if v.model.IsConnected() {
		if host := v.model.GetSelectedHost(); host != nil {
			content += ui.SuccessStyle.Render(fmt.Sprintf("Connected to: %s", host.Name))
			if transfer := v.model.GetTransfer(); transfer != nil {
				content += ui.SuccessStyle.Render(" (SFTP available)")
			}
			content += "\n\n"
		}
	} else {
		content += ui.DescriptionStyle.Render("No active connection") + "\n\n"
	}

	// Lista hostów
	if v.showHostList {
		if len(v.hosts) == 0 {
			content += ui.DescriptionStyle.Render("No hosts available. Press 'e' to add hosts.") + "\n\n"
		} else {
			content += ui.TitleStyle.Render("Available hosts:") + "\n\n"
			for i, host := range v.hosts {
				prefix := "  "
				if i == v.selectedIndex {
					prefix = "> "
					content += ui.SelectedItemStyle.Render(
						fmt.Sprintf("%s%s (%s)\n", prefix, host.Name, host.Description),
					)
				} else {
					content += fmt.Sprintf("%s%s (%s)\n", prefix, host.Name, host.Description)
				}
			}
			content += "\n"
		}
	}

	// Dostępne akcje
	content += "Available actions:\n\n"

	if len(v.hosts) > 0 {
		content += ui.ButtonStyle.Render("c") + " - Connect SSH    "
		content += ui.ButtonStyle.Render("t") + " - Transfer files"
		if !v.model.IsConnected() {
			content += " (requires connection)"
		}
		content += "\n"
	}

	content += ui.ButtonStyle.Render("e") + " - Edit hosts/passwords    " +
		ui.ButtonStyle.Render("r") + " - Refresh list    " +
		ui.ButtonStyle.Render("q") + " - Quit\n"

	// Pomoc nawigacji
	if len(v.hosts) > 0 && !v.connecting {
		content += ui.DescriptionStyle.Render("\nUse ↑/↓ arrows to navigate") + "\n"
	}

	// Statystyki
	content += "\nStatistics:\n" +
		ui.DescriptionStyle.Render(
			fmt.Sprintf("Saved hosts: %s\n",
				ui.SuccessStyle.Render(fmt.Sprintf("%d", len(v.hosts))),
			)+
				fmt.Sprintf("Saved passwords: %s\n",
					ui.SuccessStyle.Render(fmt.Sprintf("%d", len(v.model.GetPasswords()))),
				),
		)

	// Status i błędy
	if v.errMsg != "" {
		content += "\n" + ui.ErrorStyle.Render(v.errMsg)
	}
	if v.status != "" {
		content += "\n" + ui.SuccessStyle.Render(v.status)
	}

	return ui.WindowStyle.Render(content)
}

func createSSHCommand(host *models.Host, decryptedPass string) *exec.Cmd {
	sshCommand := fmt.Sprintf("sshpass -p '%s' ssh -o stricthostkeychecking=no %s@%s -p %s",
		decryptedPass, host.Login, host.IP, host.Port)

	cmd := exec.Command("sh", "-c", sshCommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}
