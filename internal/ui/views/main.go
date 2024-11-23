// internal/ui/views/main.go

package views

import (
	"fmt"
	"sshManager/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

type mainView struct {
	model  *ui.Model
	errMsg string
}

func NewMainView(model *ui.Model) *mainView {
	return &mainView{
		model: model,
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

		case "c":
			v.model.SetActiveView(ui.ViewConnect)
			return v, nil

		case "e":
			v.model.SetActiveView(ui.ViewEdit)
			return v, nil

		case "t":
			if !v.model.IsConnected() {
				v.model.SetStatus("Najpierw połącz się z hostem", true)
				return v, nil
			}
			v.model.SetActiveView(ui.ViewTransfer)
			return v, nil
		}
	}
	return v, nil
}

func (v *mainView) View() string {
	var content string

	// Logo/Tytuł
	content = ui.TitleStyle.Render("SSH Manager") + "\n\n"

	// Status połączenia
	if v.model.IsConnected() {
		if host := v.model.GetSelectedHost(); host != nil {
			content += ui.SuccessStyle.Render("Połączono z: "+host.Name) + "\n\n"
		}
	} else {
		content += ui.DescriptionStyle.Render("Brak aktywnego połączenia") + "\n\n"
	}

	// Menu główne
	content += "Dostępne opcje:\n\n"

	// Połączenie
	content += ui.ButtonStyle.Render("c") + " - "
	if v.model.IsConnected() {
		content += "Zmień połączenie\n"
	} else {
		content += "Połącz z hostem\n"
	}

	// Edycja
	content += ui.ButtonStyle.Render("e") + " - Edytuj hosty/hasła\n"

	// Transfer plików
	content += ui.ButtonStyle.Render("t") + " - "
	if v.model.IsConnected() {
		content += "Transfer plików\n"
	} else {
		content += "Transfer plików (wymaga połączenia)\n"
	}

	// Wyjście
	content += ui.ButtonStyle.Render("q") + " - Wyjście\n"

	// Statystyki
	content += "\nStatystyki:\n"
	content += ui.DescriptionStyle.Render(
		"Zapisane hosty: " +
			ui.SuccessStyle.Render(
				fmt.Sprintf("%d", len(v.model.GetHosts())),
			) + "\n" +
			"Zapisane hasła: " +
			ui.SuccessStyle.Render(
				fmt.Sprintf("%d", len(v.model.GetPasswords())),
			) + "\n",
	)

	// Komunikaty o błędach
	if v.errMsg != "" {
		content += "\n" + ui.ErrorStyle.Render(v.errMsg)
	}

	return ui.WindowStyle.Render(content)
}
