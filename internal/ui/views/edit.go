// internal/ui/views/edit.go

package views

import (
	"fmt"
	"sshManager/internal/models"
	"sshManager/internal/ui"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type editMode int

const (
	modeNormal editMode = iota
	modeSelectPassword
)

type editView struct {
	model                 *ui.Model
	activeField           int
	editing               bool
	editingHost           bool
	inputs                []textinput.Model
	currentHost           *models.Host
	currentPassword       *models.Password
	errMsg                string
	mode                  editMode
	passwordList          []models.Password
	selectedPasswordIndex int
	tmpHost               *models.Host // tymczasowy host podczas wyboru hasła

}

func NewEditView(model *ui.Model) *editView {
	v := &editView{
		model:  model,
		inputs: make([]textinput.Model, 6), // Nazwa, Opis, Login, IP, Port, Hasło
	}

	// Inicjalizacja pól tekstowych
	for i := range v.inputs {
		t := textinput.New()
		t.CharLimit = 64

		switch i {
		case 0:
			t.Placeholder = "Nazwa"
			t.Focus()
		case 1:
			t.Placeholder = "Opis"
		case 2:
			t.Placeholder = "Login"
		case 3:
			t.Placeholder = "IP/Host"
		case 4:
			t.Placeholder = "Port"
		case 5:
			t.Placeholder = "Hasło"
			t.EchoMode = textinput.EchoPassword
		}

		v.inputs[i] = t
	}

	return v
}

func (v *editView) Init() tea.Cmd {
	return textinput.Blink
}

func (v *editView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if v.mode == modeSelectPassword {
				v.mode = modeNormal
				v.editing = false
				v.resetState()
				return v, nil
			}
			if !v.editing {
				v.model.SetStatus("", false)
				v.model.SetActiveView(ui.ViewMain)
				v.resetState()
				return v, nil
			}
			v.editing = false
			v.resetState()
			return v, nil

		case "tab", "shift+tab", "up", "down":
			if v.mode == modeSelectPassword {
				if msg.String() == "up" || msg.String() == "shift+tab" {
					v.selectedPasswordIndex--
					if v.selectedPasswordIndex < 0 {
						v.selectedPasswordIndex = len(v.passwordList) - 1
					}
				} else {
					v.selectedPasswordIndex++
					if v.selectedPasswordIndex >= len(v.passwordList) {
						v.selectedPasswordIndex = 0
					}
				}
				return v, nil
			}

			if v.editing {
				s := msg.String()
				if s == "up" || s == "shift+tab" {
					v.activeField--
				} else {
					v.activeField++
				}

				if v.editingHost {
					if v.activeField > 4 {
						v.activeField = 0
					} else if v.activeField < 0 {
						v.activeField = 4
					}
				} else {
					if v.activeField > 1 {
						v.activeField = 0
					} else if v.activeField < 0 {
						v.activeField = 1
					}
				}

				for i := range v.inputs {
					if i == v.activeField {
						v.inputs[i].Focus()
					} else {
						v.inputs[i].Blur()
					}
				}
			}

		case "enter":
			if v.mode == modeSelectPassword {
				return v.saveHostWithPassword()
			}

			if !v.editing {
				v.editing = true
				v.editingHost = true
				v.initializeHostInputs()
				return v, nil
			}
			return v.handleSave()

		case "h":
			if !v.editing {
				v.editing = true
				v.editingHost = true
				v.initializeHostInputs()
				return v, nil
			}

		case "p":
			if !v.editing {
				v.editing = true
				v.editingHost = false
				v.initializePasswordInputs()
				return v, nil
			}
		}
	}

	// Aktualizacja aktywnego pola
	if v.editing && v.mode != modeSelectPassword {
		var cmd tea.Cmd
		v.inputs[v.activeField], cmd = v.inputs[v.activeField].Update(msg)
		return v, cmd
	}

	return v, cmd
}

func (v *editView) View() string {
	var content string

	if v.editing {
		if v.editingHost {
			content = ui.TitleStyle.Render("Edycja hosta") + "\n\n"

			labels := []string{"Nazwa hosta:", "Opis:", "Login:", "IP/Host:", "Port:"}
			for i, input := range v.inputs[:5] {
				content += labels[i] + "\n"
				if i == v.activeField {
					content += ui.SelectedItemStyle.Render(input.View()) + "\n\n"
				} else {
					content += input.View() + "\n\n"
				}
			}
		} else {
			content = ui.TitleStyle.Render("Edycja hasła") + "\n\n"

			labels := []string{"Opis hasła:", "Hasło:"}
			for i, input := range v.inputs[:2] {
				content += labels[i] + "\n"
				if i == v.activeField {
					content += ui.SelectedItemStyle.Render(input.View()) + "\n\n"
				} else {
					content += input.View() + "\n\n"
				}
			}
		}

		content += ui.ButtonStyle.Render("ENTER") + " - Zapisz    " +
			ui.ButtonStyle.Render("ESC") + " - Anuluj    " +
			ui.ButtonStyle.Render("↑/↓") + " - Nawigacja"
	} else {
		content = ui.TitleStyle.Render("Tryb edycji") + "\n\n" +
			"h - Dodaj/Edytuj host\n" +
			"p - Dodaj/Edytuj hasło\n" +
			"ESC - Powrót\n"
	}

	if v.errMsg != "" {
		content += "\n" + ui.ErrorStyle.Render(v.errMsg)
	}

	if v.mode == modeSelectPassword {
		content = ui.TitleStyle.Render("Wybierz hasło dla hosta") + "\n\n"

		for i, pwd := range v.passwordList {
			if i == v.selectedPasswordIndex {
				content += ui.SelectedItemStyle.Render("> " + pwd.Description + "\n")
			} else {
				content += "  " + pwd.Description + "\n"
			}
		}

		content += "\n" + ui.ButtonStyle.Render("ENTER") + " - Wybierz    " +
			ui.ButtonStyle.Render("ESC") + " - Anuluj"

		return ui.WindowStyle.Render(content)
	}

	return ui.WindowStyle.Render(content)
}

func (v *editView) handleSave() (tea.Model, tea.Cmd) {
	if v.editingHost {
		// Walidacja pól hosta
		if v.inputs[0].Value() == "" || v.inputs[2].Value() == "" ||
			v.inputs[3].Value() == "" || v.inputs[4].Value() == "" {
			v.errMsg = "Wszystkie pola oprócz opisu są wymagane"
			return v, nil
		}

		// Sprawdzenie poprawności portu
		if _, err := strconv.Atoi(v.inputs[4].Value()); err != nil {
			v.errMsg = "Port musi być liczbą"
			return v, nil
		}

		// Sprawdzenie czy jest dostępne hasło
		passwords := v.model.GetPasswords()
		if len(passwords) == 0 {
			v.errMsg = "Najpierw dodaj hasło"
			return v, nil
		}

		// Inicjalizacja tymczasowego hosta
		v.tmpHost = &models.Host{
			Name:        v.inputs[0].Value(),
			Description: v.inputs[1].Value(),
			Login:       v.inputs[2].Value(),
			IP:          v.inputs[3].Value(),
			Port:        v.inputs[4].Value(),
		}

		// Przejdź do trybu wyboru hasła
		v.mode = modeSelectPassword
		v.passwordList = passwords
		v.selectedPasswordIndex = 0 // Ustaw początkowy indeks wybranego hasła
		return v, nil
	} else {
		// Walidacja pól hasła
		if v.inputs[0].Value() == "" || v.inputs[1].Value() == "" {
			v.errMsg = "Opis i hasło są wymagane"
			return v, nil
		}

		// Szyfrowanie hasła przy użyciu obiektu Cipher
		password, err := models.NewPassword(v.inputs[0].Value(), v.inputs[1].Value(), v.model.GetCipher())
		if err != nil {
			v.errMsg = fmt.Sprintf("Błąd podczas tworzenia hasła: %v", err)
			return v, nil
		}

		var opErr interface{}
		if v.currentPassword != nil {
			opErr = v.model.UpdatePassword(v.currentPassword.Description, password)
		} else {
			opErr = v.model.AddPassword(password)
		}

		if opErr != nil {
			v.errMsg = fmt.Sprint(opErr)
			return v, nil
		}

		// Zapisz konfigurację
		if err := v.model.SaveConfig(); err != nil {
			v.errMsg = fmt.Sprintf("Błąd podczas zapisywania konfiguracji: %v", err)
			return v, nil
		}
	}

	// Aktualizacja list po zapisie
	v.model.UpdateLists()
	v.model.SetStatus("Zapisano pomyślnie", false)
	v.editing = false
	v.resetState()
	return v, nil
}

func (v *editView) initializeHostInputs() {
	if v.currentHost != nil {
		v.inputs[0].SetValue(v.currentHost.Name)
		v.inputs[1].SetValue(v.currentHost.Description)
		v.inputs[2].SetValue(v.currentHost.Login)
		v.inputs[3].SetValue(v.currentHost.IP)
		v.inputs[4].SetValue(v.currentHost.Port)
	} else {
		for i := range v.inputs {
			v.inputs[i].Reset()
		}
	}
	v.inputs[0].Focus()
}

func (v *editView) initializePasswordInputs() {
	if v.currentPassword != nil {
		v.inputs[0].SetValue(v.currentPassword.Description)
		// Nie ustawiamy wartości hasła ze względów bezpieczeństwa
	} else {
		for i := range v.inputs[:2] { // tylko pierwsze dwa pola
			v.inputs[i].Reset()
		}
	}

	// Ustawmy odpowiednie placeholdery dla pól hasła
	v.inputs[0].Placeholder = "Opis hasła"
	v.inputs[1].Placeholder = "Hasło"
	v.inputs[1].EchoMode = textinput.EchoPassword

	v.inputs[0].Focus()
}

func (v *editView) resetState() {
	v.activeField = 0
	v.errMsg = ""
	v.currentHost = nil
	v.currentPassword = nil
	for i := range v.inputs {
		v.inputs[i].Reset()
	}
}

func (v *editView) saveHostWithPassword() (tea.Model, tea.Cmd) {
	v.tmpHost.PasswordID = v.selectedPasswordIndex

	var err interface{}
	if v.currentHost != nil {
		err = v.model.UpdateHost(v.currentHost.Name, v.tmpHost)
	} else {
		err = v.model.AddHost(v.tmpHost)
	}

	if err != nil {
		v.errMsg = fmt.Sprint(err)
		return v, nil
	}

	// Zapisz konfigurację
	if err := v.model.SaveConfig(); err != nil {
		v.errMsg = fmt.Sprintf("Błąd podczas zapisywania konfiguracji: %v", err)
		return v, nil
	}

	v.mode = modeNormal
	v.model.UpdateLists()
	v.model.SetStatus("Zapisano pomyślnie", false)
	v.editing = false
	v.resetState()
	return v, nil
}
