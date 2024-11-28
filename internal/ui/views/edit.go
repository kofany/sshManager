// internal/ui/views/edit.go

package views

import (
	"fmt"
	"sshManager/internal/models"
	"sshManager/internal/ui"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type editMode int

const (
	modeNormal editMode = iota
	modeSelectPassword
	modeHostList
	modePasswordList
)

type editView struct {
	model                 *ui.Model
	activeField           int
	editing               bool
	editingHost           bool
	inputs                []textinput.Model
	currentHost           *models.Host
	currentPassword       *models.Password
	errorMsg              string
	mode                  editMode
	passwordList          []models.Password
	selectedPasswordIndex int
	tmpHost               *models.Host
	hosts                 []models.Host
	passwords             []models.Password
	selectedItemIndex     int
	deleteConfirmation    bool
	width                 int // Dodane
	height                int // Dodane
}

func NewEditView(model *ui.Model) *editView {
	v := &editView{
		model:                 model,
		inputs:                make([]textinput.Model, 6), // Name, Description, Login, IP, Port, Password
		width:                 model.GetTerminalWidth(),
		height:                model.GetTerminalHeight(),
		mode:                  modeNormal,
		activeField:           0,
		editing:               false,
		editingHost:           false,
		errorMsg:              "",
		selectedPasswordIndex: 0,
		selectedItemIndex:     0,
		deleteConfirmation:    false,
		hosts:                 make([]models.Host, 0),
		passwords:             make([]models.Password, 0),
		passwordList:          make([]models.Password, 0),
	}

	// Initialize text inputs
	for i := range v.inputs {
		t := textinput.New()
		t.CharLimit = 64

		switch i {
		case 0:
			t.Placeholder = "Name"
			t.Focus()
		case 1:
			t.Placeholder = "Description"
			t.EchoMode = textinput.EchoNormal // Ensure it's normal text
		case 2:
			t.Placeholder = "Login"
		case 3:
			t.Placeholder = "IP/Host"
		case 4:
			t.Placeholder = "Port"
		case 5:
			t.Placeholder = "Password"
			t.EchoMode = textinput.EchoPassword
		}
		v.inputs[i] = t
	}

	return v
}

func (v *editView) Init() tea.Cmd {
	// aktualizacja list przy inicjalizacji
	v.model.UpdateLists()
	return textinput.Blink
}

func (v *editView) View() string {
	if v.width == 0 || v.height == 0 {
		// Zabezpieczenie przed zerowymi wymiarami
		v.width = v.model.GetTerminalWidth()
		v.height = v.model.GetTerminalHeight()
	}

	var content string
	contentWidth := min(v.width-40, 160) // Maksymalna szerokość z marginesami

	switch v.mode {
	case modePasswordList:
		content = v.renderPasswordList(contentWidth)
	case modeSelectPassword:
		content = v.renderPasswordSelection(contentWidth)
	default:
		if v.editing {
			if v.editingHost {
				content = v.renderHostEdit(contentWidth)
			} else {
				content = v.renderPasswordEdit(contentWidth)
			}
		}
	}

	if v.errorMsg != "" {
		content += "\n" + ui.ErrorStyle.Render(v.errorMsg)
	}

	finalContent := ui.WindowStyle.
		Width(contentWidth).
		Render(content)

	return lipgloss.Place(
		v.width,
		v.height,
		lipgloss.Center,
		lipgloss.Center,
		finalContent,
		lipgloss.WithWhitespaceChars(""),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

func (v *editView) resetState() {
	// Reset basic state
	v.activeField = 0
	v.errorMsg = ""
	v.currentHost = nil
	v.currentPassword = nil
	v.tmpHost = nil
	v.editing = false
	v.mode = modeNormal
	v.deleteConfirmation = false

	// Reset lists
	v.hosts = make([]models.Host, 0)
	v.passwords = make([]models.Password, 0)
	v.passwordList = make([]models.Password, 0)
	v.selectedItemIndex = 0
	v.selectedPasswordIndex = 0

	// Reset all inputs
	for i := range v.inputs {
		v.inputs[i].Reset()
		v.inputs[i].Blur()
	}

	// Refresh lists to ensure state consistency in main_view.go
	v.model.UpdateLists()
}

func (v *editView) renderPasswordSelection(width int) string {
	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("Select Password for Host") + "\n\n")

	if len(v.passwordList) == 0 {
		content.WriteString(ui.ErrorStyle.Render("No passwords available. Please add a password first.") + "\n")
	} else {
		listWidth := width - 4 // Margines wewnętrzny
		for i, pwd := range v.passwordList {
			prefix := "  "
			if i == v.selectedPasswordIndex {
				prefix = "> "
				line := fmt.Sprintf("%-*s", listWidth-1, prefix+pwd.Description)
				content.WriteString(ui.SelectedItemStyle.Render(line) + "\n")
			} else {
				line := fmt.Sprintf("%-*s", listWidth-1, prefix+pwd.Description)
				content.WriteString(line + "\n")
			}
		}
	}

	content.WriteString("\n" + v.renderControls(
		Control{"ENTER", "Select"},
		Control{"ESC", "Cancel"},
	))

	return content.String()
}

func (v *editView) renderPasswordList(width int) string {
	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("Password List") + "\n\n")

	if len(v.passwords) == 0 {
		content.WriteString(ui.DescriptionStyle.Render("No passwords available. Press 'p' to add a new password.") + "\n")
	} else {
		listWidth := width - 4
		for i, pass := range v.passwords {
			prefix := "  "
			if i == v.selectedItemIndex {
				prefix = "> "
				line := fmt.Sprintf("%-*s", listWidth-1, prefix+pass.Description)
				content.WriteString(ui.SelectedItemStyle.Render(line) + "\n")
			} else {
				line := fmt.Sprintf("%-*s", listWidth-1, prefix+pass.Description)
				content.WriteString(line + "\n")
			}
		}
	}

	content.WriteString("\n" + v.renderControls(
		Control{"e", "Edit"},
		Control{"d", "Delete"},
		Control{"ESC", "Back"},
	))

	return content.String()
}

// Helper struct for rendering controls
type Control struct {
	key         string
	description string
}

func (v *editView) renderControls(controls ...Control) string {
	var content strings.Builder
	for i, ctrl := range controls {
		if i > 0 {
			content.WriteString("    ") // Zwiększony odstęp między kontrolkami
		}
		content.WriteString(ui.ButtonStyle.Render(ctrl.key) + " - " + ctrl.description)
	}
	return content.String()
}

func (v *editView) renderPasswordEdit(width int) string {
	var content strings.Builder

	// Tytuł
	title := "Add New Password"
	if v.currentPassword != nil {
		title = "Edit Password"
	}
	content.WriteString(ui.TitleStyle.Render(title) + "\n\n")

	// Dopasowanie szerokości pól wejściowych
	inputWidth := width - 8 // Marginesy i ramki

	// Etykiety dla pól
	labels := []string{
		"Description:",
		"Password:",
	}

	// Renderowanie pól wejściowych
	for i, input := range v.inputs[:2] {
		content.WriteString(ui.LabelStyle.Render(labels[i]) + "\n")

		inputStyle := ui.InputStyle.Width(inputWidth)
		if i == v.activeField {
			inputStyle = ui.SelectedItemStyle.Width(inputWidth)
		}
		content.WriteString(inputStyle.Render(input.View()) + "\n\n")
	}

	// Dodanie kontroli na dole widoku
	content.WriteString(v.renderControls(
		Control{"ENTER", "Save"},
		Control{"ESC", "Cancel"},
		Control{"↑/↓", "Navigate"},
	))

	return content.String()
}

func (v *editView) renderHostEdit(width int) string {
	var content strings.Builder

	// Tytuł
	title := "Add New Host"
	if v.currentHost != nil {
		title = "Edit Host"
	}
	content.WriteString(ui.TitleStyle.Render(title) + "\n\n")

	// Dopasowanie szerokości pól wejściowych
	inputWidth := width - 8 // Marginesy i ramki

	// Etykiety dla pól
	labels := []string{
		"Host Name:",
		"Description:",
		"Login:",
		"IP/Host:",
		"Port:",
	}

	// Renderowanie pól wejściowych
	for i, input := range v.inputs[:5] {
		content.WriteString(ui.LabelStyle.Render(labels[i]) + "\n")

		inputStyle := ui.InputStyle.Width(inputWidth)
		if i == v.activeField {
			inputStyle = ui.SelectedItemStyle.Width(inputWidth)
		}
		content.WriteString(inputStyle.Render(input.View()) + "\n\n")
	}

	// Dodanie kontroli na dole widoku
	content.WriteString(v.renderControls(
		Control{"ENTER", "Save"},
		Control{"ESC", "Cancel"},
		Control{"↑/↓", "Navigate"},
	))

	return content.String()
}

func (v *editView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Aktualizuj rozmiar okna
		v.width = msg.Width
		v.height = msg.Height
		v.model.UpdateWindowSize(msg.Width, msg.Height)
		return v, nil

	case tea.KeyMsg:
		// Sprawdź, czy jesteśmy w trybie edycji
		if v.editing && v.mode != modeSelectPassword &&
			v.mode != modeHostList && v.mode != modePasswordList {
			// Obsługuj specjalne klawisze w trybie edycji
			switch msg.String() {
			case "esc":
				model, cmd := v.handleEscapeKey()
				if _, ok := model.(*editView); !ok {
					// Jeśli zwrócony model nie jest editView, zwróć go
					return model, cmd
				}
				// W przeciwnym razie pozostań w obecnym widoku
				return v, cmd

			case "enter":
				model, cmd := v.handleEnterKey()
				if _, ok := model.(*editView); !ok {
					// Jeśli zwrócony model nie jest editView, zwróć go
					return model, cmd
				}
				return v, cmd

			case "tab", "shift+tab", "up", "down":
				return v.handleNavigationKey(msg.String())

			default:
				// Przekazanie innych klawiszy do aktywnego pola tekstowego
				v.inputs[v.activeField], cmd = v.inputs[v.activeField].Update(msg)
				return v, cmd
			}
		}

		// Obsługuj klawisze w normalnym trybie
		switch msg.String() {
		case "esc":
			model, cmd := v.handleEscapeKey()
			if _, ok := model.(*editView); !ok {
				// Jeśli zwrócony model nie jest editView, zwróć go
				return model, cmd
			}
			return v, cmd

		case "tab", "shift+tab", "up", "down":
			return v.handleNavigationKey(msg.String())

		case "enter":
			model, cmd := v.handleEnterKey()
			if _, ok := model.(*editView); !ok {
				// Jeśli zwrócony model nie jest editView, zwróć go
				return model, cmd
			}
			return v, cmd

		case "e", "d":
			model, cmd := v.handleActionKey(msg.String())
			if _, ok := model.(*editView); !ok {
				// Jeśli zwrócony model nie jest editView, zwróć go
				return model, cmd
			}
			return v, cmd
		}
	}

	return v, cmd
}

// internal/ui/views/edit.go
func (v *editView) handleEscapeKey() (tea.Model, tea.Cmd) {
	switch v.mode {
	case modeSelectPassword:
		v.mode = modeNormal
		v.editing = false
		v.resetState()
		v.model.SetActiveView(ui.ViewMain)
		return NewMainView(v.model), nil

	case modeHostList, modePasswordList:
		v.mode = modeNormal
		v.editing = false
		v.resetState()
		v.model.SetActiveView(ui.ViewMain)
		return NewMainView(v.model), nil

	default:
		if !v.editing {
			v.model.SetStatus("", false)
			v.model.SetActiveView(ui.ViewMain)
			v.resetState()
			return NewMainView(v.model), nil
		}

		v.editing = false
		v.resetState()
		v.model.SetActiveView(ui.ViewMain)
		return NewMainView(v.model), nil
	}
}

func (v *editView) handleNavigationKey(key string) (tea.Model, tea.Cmd) {
	switch v.mode {
	case modeSelectPassword:
		v.navigatePasswordSelection(key)
		return v, nil

	case modeHostList, modePasswordList:
		v.navigateList(key)
		return v, nil

	default:
		if v.editing {
			v.navigateFields(key)
		}
	}

	return v, nil
}

func (v *editView) navigatePasswordSelection(key string) {
	if key == "up" || key == "shift+tab" {
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
}

func (v *editView) navigateList(key string) {
	maxItems := len(v.hosts)
	if v.mode == modePasswordList {
		maxItems = len(v.passwords)
	}

	if key == "up" || key == "shift+tab" {
		v.selectedItemIndex--
		if v.selectedItemIndex < 0 {
			v.selectedItemIndex = maxItems - 1
		}
	} else {
		v.selectedItemIndex++
		if v.selectedItemIndex >= maxItems {
			v.selectedItemIndex = 0
		}
	}
}

func (v *editView) navigateFields(key string) {
	if key == "up" || key == "shift+tab" {
		v.activeField--
	} else {
		v.activeField++
	}

	maxFields := 5 // For host editing
	if !v.editingHost {
		maxFields = 2 // For password editing
	}

	// Wrap around navigation
	if v.activeField >= maxFields {
		v.activeField = 0
	} else if v.activeField < 0 {
		v.activeField = maxFields - 1
	}

	// Update focus
	for i := range v.inputs {
		if i == v.activeField {
			v.inputs[i].Focus()
		} else {
			v.inputs[i].Blur()
		}
	}
}

func (v *editView) handleActionKey(key string) (tea.Model, tea.Cmd) {
	switch v.mode {
	case modePasswordList:
		if len(v.passwords) == 0 {
			return v, nil
		}
		return v.handlePasswordListAction(key)
	}
	return v, nil
}

func (v *editView) handlePasswordListAction(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "e":
		// Edytuj wybrane hasło
		v.currentPassword = &v.passwords[v.selectedItemIndex]
		v.editingHost = false
		v.mode = modeNormal
		v.initializePasswordInputs()
		return v, nil

	case "d":
		if !v.deleteConfirmation {
			v.errorMsg = "Press 'd' again to confirm deletion"
			v.deleteConfirmation = true
			return v, nil
		}

		// Usuń wybrane hasło
		password := v.passwords[v.selectedItemIndex]
		if err := v.model.DeletePassword(password.Description); err != nil {
			v.errorMsg = fmt.Sprint(err)
		} else {
			// Zapisz konfigurację po usunięciu hasła
			if err := v.model.SaveConfig(); err != nil {
				v.errorMsg = fmt.Sprintf("Failed to save configuration: %v", err)
				return v, nil
			}
			v.model.UpdateLists()
			v.passwords = v.model.GetPasswords()
			if v.selectedItemIndex >= len(v.passwords) {
				v.selectedItemIndex = len(v.passwords) - 1
			}
			v.model.SetStatus("Password deleted successfully", false)
		}
		v.deleteConfirmation = false

		// Przekierowanie na widok główny po usunięciu
		v.model.SetActiveView(ui.ViewMain)
		return v, nil
	}
	return v, nil
}

func (v *editView) handleEnterKey() (tea.Model, tea.Cmd) {
	switch {
	case v.mode == modeSelectPassword:
		model, cmd := v.saveHostWithPassword()
		if _, ok := model.(*editView); ok {
			// Jeśli wystąpił błąd, pozostań w widoku edycji
			return model, cmd
		}
		v.model.SetActiveView(ui.ViewMain)
		v.model.UpdateLists()
		return model, cmd

	case v.mode == modeHostList, v.mode == modePasswordList:
		return v, nil

	case !v.editing:
		v.editing = true
		v.editingHost = true
		v.initializeHostInputs()
		return v, nil

	default:
		model, cmd := v.handleSave()
		if _, ok := model.(*editView); ok {
			// Jeśli wystąpił błąd, pozostań w widoku edycji
			return model, cmd
		}
		v.model.UpdateLists()
		return model, cmd
	}
}

func (v *editView) handleSave() (tea.Model, tea.Cmd) {
	if v.editingHost {
		// Zapisz host i przejdź do widoku głównego
		model, cmd := v.validateAndSaveHost()
		if _, ok := model.(*editView); ok {
			// Jeśli wystąpił błąd, pozostań w widoku edycji
			return model, cmd
		}
		// Jeśli walidacja przeszła pomyślnie, zostaniemy w tym samym widoku
		// aby wybrać hasło
		return model, cmd
	}

	// Zapisz hasło i przejdź do widoku głównego
	model, cmd := v.validateAndSavePassword()
	if _, ok := model.(*editView); ok {
		// Jeśli wystąpił błąd, pozostań w widoku edycji
		return model, cmd
	}
	v.model.SetActiveView(ui.ViewMain)
	return model, cmd
}

func (v *editView) validateAndSaveHost() (tea.Model, tea.Cmd) {
	// Sprawdź poprawność pól
	if err := v.validateHostFields(); err != nil {
		v.errorMsg = err.Error()
		return v, nil
	}

	// Sprawdź dostępne hasła
	passwords := v.model.GetPasswords()
	if len(passwords) == 0 {
		v.errorMsg = "Please add a password first"
		return v, nil
	}

	// Zainicjalizuj tymczasowego hosta
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
	v.selectedPasswordIndex = 0
	return v, nil
}

func (v *editView) validateAndSavePassword() (tea.Model, tea.Cmd) {
	// Walidacja pól hasła
	if err := v.validatePasswordFields(); err != nil {
		v.errorMsg = err.Error()
		return v, nil
	}

	// Utworzenie nowego hasła z szyfrowaniem
	password, err := models.NewPassword(v.inputs[0].Value(), v.inputs[1].Value(), v.model.GetCipher())
	if err != nil {
		v.errorMsg = fmt.Sprintf("Failed to create password: %v", err)
		return v, nil
	}

	// Aktualizacja lub dodanie hasła
	var opErr interface{}
	if v.currentPassword != nil {
		opErr = v.model.UpdatePassword(v.currentPassword.Description, password)
	} else {
		opErr = v.model.AddPassword(password)
	}

	if opErr != nil {
		v.errorMsg = fmt.Sprint(opErr)
		return v, nil
	}

	// Zapis konfiguracji
	if err := v.model.SaveConfig(); err != nil {
		v.errorMsg = fmt.Sprintf("Failed to save configuration: %v", err)
		return v, nil
	}

	// Aktualizacja stanu UI
	v.model.UpdateLists()
	v.model.SetStatus("Password saved successfully", false)
	v.editing = false
	v.resetState()

	// Przekierowanie na widok główny
	v.model.SetActiveView(ui.ViewMain)
	return NewMainView(v.model), nil
}

func (v *editView) saveHostWithPassword() (tea.Model, tea.Cmd) {
	v.tmpHost.PasswordID = v.selectedPasswordIndex

	// Aktualizacja lub dodanie hosta
	var err interface{}
	if v.currentHost != nil {
		err = v.model.UpdateHost(v.currentHost.Name, v.tmpHost)
	} else {
		err = v.model.AddHost(v.tmpHost)
	}

	if err != nil {
		v.errorMsg = fmt.Sprint(err)
		return v, nil
	}

	// Zapis konfiguracji
	if err := v.model.SaveConfig(); err != nil {
		v.errorMsg = fmt.Sprintf("Failed to save configuration: %v", err)
		return v, nil
	}

	// Aktualizacja stanu UI
	v.mode = modeNormal
	v.model.UpdateLists()
	v.model.SetStatus("Host saved successfully", false)
	v.editing = false
	v.resetState()

	// Przekierowanie na widok główny
	v.model.SetActiveView(ui.ViewMain)
	return NewMainView(v.model), nil
}

func (v *editView) initializeHostInputs() {
	// Reset all inputs first
	for i := range v.inputs {
		v.inputs[i].Reset()
		v.inputs[i].Blur()
	}

	// Set default values or current host values
	if v.currentHost != nil {
		v.inputs[0].SetValue(v.currentHost.Name)
		v.inputs[1].SetValue(v.currentHost.Description)
		v.inputs[2].SetValue(v.currentHost.Login)
		v.inputs[3].SetValue(v.currentHost.IP)
		v.inputs[4].SetValue(v.currentHost.Port)
	}

	// Configure field properties
	v.inputs[0].Placeholder = "Host name"
	v.inputs[1].Placeholder = "Description"
	v.inputs[1].EchoMode = textinput.EchoNormal
	v.inputs[2].Placeholder = "Username"
	v.inputs[3].Placeholder = "IP address or hostname"
	v.inputs[4].Placeholder = "Port number"

	// Focus the first field
	v.activeField = 0
	v.inputs[0].Focus()
}

func (v *editView) initializePasswordInputs() {
	// Reset all inputs first
	for i := range v.inputs {
		v.inputs[i].Reset()
		v.inputs[i].Blur()
	}

	// Set default values or current password values
	if v.currentPassword != nil {
		v.inputs[0].SetValue(v.currentPassword.Description)
		// Don't set the password value for security reasons
	}

	// Configure field properties
	v.inputs[0].Placeholder = "Password description"
	v.inputs[1].Placeholder = "Enter password"
	v.inputs[1].EchoMode = textinput.EchoPassword

	// Focus the first field
	v.activeField = 0
	v.inputs[0].Focus()
}

// Helper function to check if a field contains only digits
func isNumeric(s string) bool {
	num, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	// Sprawdź czy numer portu jest w prawidłowym zakresie (1-65535)
	return num > 0 && num <= 65535
}

// Helper function to validate host fields
func (v *editView) validateHostFields() error {
	if v.inputs[0].Value() == "" {
		return fmt.Errorf("host name is required")
	}
	if v.inputs[2].Value() == "" {
		return fmt.Errorf("login is required")
	}
	if v.inputs[3].Value() == "" {
		return fmt.Errorf("IP/hostname is required")
	}
	if !isNumeric(v.inputs[4].Value()) {
		return fmt.Errorf("port must be a valid number")
	}
	port, _ := strconv.Atoi(v.inputs[4].Value())
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// Helper function to validate password fields
func (v *editView) validatePasswordFields() error {
	if v.inputs[0].Value() == "" {
		return fmt.Errorf("password description is required")
	}
	if v.inputs[1].Value() == "" {
		return fmt.Errorf("password value is required")
	}
	if len(v.inputs[1].Value()) < 6 {
		return fmt.Errorf("password must be at least 6 characters long")
	}
	return nil
}
