// internal/ui/views/connect.go

package views

import (
	"fmt"
	"sshManager/internal/models"
	"sshManager/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type connectView struct {
	model           *ui.Model
	connecting      bool
	errMsg          string // Zmiana z err error na errMsg string
	passwordInput   textinput.Model
	selectedPass    *models.Password
	passwordEntered bool
}

func NewConnectView(model *ui.Model) *connectView {
	pi := textinput.New()
	pi.Placeholder = "Wprowadź hasło"
	pi.EchoMode = textinput.EchoPassword
	pi.CharLimit = 64

	return &connectView{
		model:         model,
		passwordInput: pi,
	}
}

func (v *connectView) Init() tea.Cmd {
	return textinput.Blink
}

func (v *connectView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if v.connecting {
				return v, nil
			}
			v.model.SetStatus("", false)
			v.model.DisconnectHost()
			v.resetState()
			v.model.SetActiveView(ui.ViewMain)
			return v, nil

		case "enter":
			if v.model.GetSelectedHost() == nil {
				v.model.SetStatus("Najpierw wybierz hosta", true)
				return v, nil
			}

			if !v.passwordEntered {
				if v.passwordInput.Value() == "" {
					v.model.SetStatus("Wprowadź hasło", true)
					return v, nil
				}
				return v.handleConnect()
			}
		}

		if !v.connecting && !v.passwordEntered {
			v.passwordInput, cmd = v.passwordInput.Update(msg)
			return v, cmd
		}
	}

	return v, nil
}

func (v *connectView) handleConnect() (tea.Model, tea.Cmd) {
	host := v.model.GetSelectedHost()
	if host == nil {
		v.model.SetStatus("Nie wybrano hosta", true)
		return v, nil
	}

	v.connecting = true
	v.passwordEntered = true
	v.model.SetStatus("Łączenie...", false)

	// Próba połączenia
	if err := v.model.ConnectToHost(host, v.passwordInput.Value()); err != nil {
		v.connecting = false
		v.errMsg = fmt.Sprint(err) // Konwersja interface{} na string
		v.model.SetStatus(fmt.Sprintf("Błąd połączenia: %v", err), true)
		v.resetState()
		return v, nil
	}

	v.connecting = false
	v.model.SetStatus(fmt.Sprintf("Połączono z %s", host.Name), false)
	return v, nil
}

func (v *connectView) View() string {
	var content string

	host := v.model.GetSelectedHost()

	if v.connecting {
		content = ui.TitleStyle.Render("Łączenie...") + "\n\n"
		if host != nil {
			content += fmt.Sprintf("Host: %s (%s)\n", host.Name, host.Description)
		}
	} else if v.model.IsConnected() {
		content = ui.TitleStyle.Render(fmt.Sprintf("Połączono z %s", host.Name)) + "\n\n" +
			fmt.Sprintf("IP: %s\n", host.IP) +
			fmt.Sprintf("Port: %s\n", host.Port) +
			fmt.Sprintf("Login: %s\n", host.Login) +
			fmt.Sprintf("Opis: %s\n\n", host.Description) +
			ui.ButtonStyle.Render("ESC") + " - Rozłącz"
	} else {
		content = ui.TitleStyle.Render("Połącz z hostem") + "\n\n"

		if host != nil {
			content += fmt.Sprintf("Wybrany host: %s\n", host.Name) +
				fmt.Sprintf("IP: %s\n", host.IP) +
				fmt.Sprintf("Port: %s\n", host.Port) +
				fmt.Sprintf("Login: %s\n", host.Login) +
				fmt.Sprintf("Opis: %s\n\n", host.Description)

			if !v.passwordEntered {
				content += ui.InputStyle.Render(v.passwordInput.View()) + "\n\n"
				content += ui.ButtonStyle.Render("ENTER") + " - Połącz    " +
					ui.ButtonStyle.Render("ESC") + " - Powrót\n"
			}
		} else {
			content += "Nie wybrano hosta\n\n" +
				ui.ButtonStyle.Render("ESC") + " - Powrót"
		}

		if v.errMsg != "" {
			content += "\n\n" + ui.ErrorStyle.Render(v.errMsg)
		}
	}

	return ui.WindowStyle.Render(content)
}

func (v *connectView) resetState() {
	v.connecting = false
	v.passwordEntered = false
	v.errMsg = ""
	v.passwordInput.Reset()
	v.selectedPass = nil
}
