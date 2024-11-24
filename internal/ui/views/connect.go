// internal/ui/views/connect.go

package views

import (
	"fmt"
	"sshManager/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

type connectView struct {
	model  *ui.Model
	errMsg string
}

func NewConnectView(model *ui.Model) *connectView {
	return &connectView{
		model: model,
	}
}

func (v *connectView) Init() tea.Cmd {
	return nil
}

func (v *connectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			v.model.SetActiveView(ui.ViewMain)
			return v, nil

		case "enter", "c":
			host := v.model.GetSelectedHost()
			if host == nil {
				v.errMsg = "No host selected"
				return v, nil
			}

			// Pobierz i zdekoduj hasło
			password := v.model.GetPasswords()[host.PasswordID]
			decryptedPass, err := password.GetDecrypted(v.model.GetCipher())
			if err != nil {
				v.errMsg = fmt.Sprintf("Failed to decrypt password: %v", err)
				return v, nil
			}

			// Wyłącz tryb alternatywny terminala
			fmt.Printf("\x1b[?1049l")

			// Wykonaj połączenie przez sshpass i zakończ program
			return v, tea.Sequence(
				tea.Quit,
				func() tea.Msg {
					client := v.model.GetSSHClient()
					if err := client.Connect(host, decryptedPass); err != nil {
						fmt.Printf("\nSSH connection failed: %v\n", err)
					}
					return nil
				},
			)
		}
	}
	return v, nil
}

func (v *connectView) View() string {
	var content string
	host := v.model.GetSelectedHost()

	content = ui.TitleStyle.Render("Connect to host") + "\n\n"
	if host != nil {
		content += fmt.Sprintf("Selected host: %s\n", host.Name) +
			fmt.Sprintf("IP: %s\n", host.IP) +
			fmt.Sprintf("Port: %s\n", host.Port) +
			fmt.Sprintf("Login: %s\n", host.Login) +
			fmt.Sprintf("Description: %s\n\n", host.Description)

		content += ui.ButtonStyle.Render("c") + " - Connect SSH    " +
			ui.ButtonStyle.Render("ESC") + " - Back\n"
	} else {
		content += "No host selected\n\n" +
			ui.ButtonStyle.Render("ESC") + " - Back"
	}

	if v.errMsg != "" {
		content += "\n\n" + ui.ErrorStyle.Render(v.errMsg)
	}

	return ui.WindowStyle.Render(content)
}
