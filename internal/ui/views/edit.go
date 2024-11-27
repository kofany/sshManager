// internal/ui/views/edit.go

package views

import (
	"fmt"
	"sshManager/internal/models"
	"sshManager/internal/ui"
	"strconv"

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
		model:  model,
		inputs: make([]textinput.Model, 6), // Name, Description, Login, IP, Port, Password
		width:  model.GetTerminalWidth(),   // Dodane
		height: model.GetTerminalHeight(),  // Dodane
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
	return textinput.Blink
}

func (v *editView) View() string {
	var content string

	contentWidth := min(v.width-40, 160) // Maksymalna szerokość z marginesami

	switch v.mode {
	case modeHostList:
		content = v.renderHostList(contentWidth)
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
		} else {
			content = v.renderMainMenu(contentWidth)
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

func (v *editView) renderPasswordSelection(width int) string {
	content := ui.TitleStyle.Render("Select Password for Host") + "\n\n"

	if len(v.passwordList) == 0 {
		content += ui.ErrorStyle.Render("No passwords available. Please add a password first.") + "\n"
	} else {
		listWidth := width - 4 // Margines wewnętrzny
		for i, pwd := range v.passwordList {
			prefix := "  "
			if i == v.selectedPasswordIndex {
				prefix = "> "
				line := fmt.Sprintf("%s%-*s",
					prefix,
					listWidth-len(prefix),
					pwd.Description)
				content += ui.SelectedItemStyle.Render(line) + "\n"
			} else {
				line := fmt.Sprintf("%s%-*s",
					prefix,
					listWidth-len(prefix),
					pwd.Description)
				content += line + "\n"
			}
		}
	}

	content += "\n" + v.renderControls(
		Control{"ENTER", "Select"},
		Control{"ESC", "Cancel"},
	)

	return content
}

func (v *editView) renderHostList(width int) string {
	content := ui.TitleStyle.Render("Host List") + "\n\n"

	if len(v.hosts) == 0 {
		content += ui.DescriptionStyle.Render("No hosts available. Press 'h' to add a new host.") + "\n"
	} else {
		listWidth := width - 4     // Margines wewnętrzny
		nameWidth := listWidth / 2 // Połowa szerokości na nazwę

		for i, host := range v.hosts {
			prefix := "  "
			if i == v.selectedItemIndex {
				prefix = "> "
			}

			// Formatuj linię z nazwą i opisem obok siebie
			line := fmt.Sprintf("%s%-*s %-*s",
				prefix,
				nameWidth,
				host.Name,
				listWidth-nameWidth-len(prefix),
				"("+host.Description+")")

			if i == v.selectedItemIndex {
				content += ui.SelectedItemStyle.Render(line) + "\n"
			} else {
				content += line + "\n"
			}
		}
	}

	content += "\n" + v.renderControls(
		Control{"e", "Edit"},
		Control{"d", "Delete"},
		Control{"ESC", "Back"},
	)

	return content
}

func (v *editView) renderPasswordList(width int) string {
	content := ui.TitleStyle.Render("Password List") + "\n\n"

	if len(v.passwords) == 0 {
		content += ui.DescriptionStyle.Render("No passwords available. Press 'p' to add a new password.") + "\n"
	} else {
		listWidth := width - 4
		for i, pass := range v.passwords {
			prefix := "  "
			if i == v.selectedItemIndex {
				prefix = "> "
				line := fmt.Sprintf("%s%-*s",
					prefix,
					listWidth-len(prefix),
					pass.Description)
				content += ui.SelectedItemStyle.Render(line) + "\n"
			} else {
				line := fmt.Sprintf("%s%-*s",
					prefix,
					listWidth-len(prefix),
					pass.Description)
				content += line + "\n"
			}
		}
	}

	content += "\n" + v.renderControls(
		Control{"e", "Edit"},
		Control{"d", "Delete"},
		Control{"ESC", "Back"},
	)

	return content
}

func (v *editView) renderPasswordEdit(width int) string {
	title := "Add New Password"
	if v.currentPassword != nil {
		title = "Edit Password"
	}

	content := ui.TitleStyle.Render(title) + "\n\n"

	// Zmniejszamy szerokość pola wejściowego, aby pasowało do ramki
	inputWidth := width - 8 // Odjęcie marginesów i ramki

	// Ustawienia dla pól wejściowych
	labels := []string{
		"Description:",
		"Password:",
	}

	for i, input := range v.inputs[:2] {
		content += labels[i] + "\n"
		inputStyle := ui.InputStyle.Width(inputWidth)
		if i == v.activeField {
			inputStyle = ui.SelectedItemStyle.Width(inputWidth)
		}
		content += inputStyle.Render(input.View()) + "\n\n"
	}

	content += v.renderControls(
		Control{"ENTER", "Save"},
		Control{"ESC", "Cancel"},
		Control{"↑/↓", "Navigate"},
	)

	return content
}

func (v *editView) renderHostEdit(width int) string {
	title := "Add New Host"
	if v.currentHost != nil {
		title = "Edit Host"
	}

	content := ui.TitleStyle.Render(title) + "\n\n"

	// Zmniejszamy szerokość pola wejściowego
	inputWidth := width - 8 // Odjęcie marginesów i ramki

	labels := []string{
		"Host Name:",
		"Description:",
		"Login:",
		"IP/Host:",
		"Port:",
	}

	for i, input := range v.inputs[:5] {
		content += labels[i] + "\n"
		inputStyle := ui.InputStyle.Width(inputWidth)
		if i == v.activeField {
			inputStyle = ui.SelectedItemStyle.Width(inputWidth)
		}
		content += inputStyle.Render(input.View()) + "\n\n"
	}

	content += v.renderControls(
		Control{"ENTER", "Save"},
		Control{"ESC", "Cancel"},
		Control{"↑/↓", "Navigate"},
	)

	return content
}

func (v *editView) renderMainMenu(width int) string {
	content := ui.TitleStyle.Render("Edit Mode") + "\n\n"

	menuItems := []struct {
		key, description string
	}{
		{"h", "Add new host"},
		{"H", "Host list"},
		{"p", "Add new password"},
		{"P", "Password list"},
		{"ESC", "Back"},
	}

	menuWidth := width - 4
	for _, item := range menuItems {
		line := fmt.Sprintf("%-*s", menuWidth,
			fmt.Sprintf("%s - %s",
				ui.ButtonStyle.Render(item.key),
				item.description))
		content += line + "\n"
	}

	return content
}

// Helper struct for rendering controls
type Control struct {
	key, description string
}

func (v *editView) renderControls(controls ...Control) string {
	var content string
	for i, ctrl := range controls {
		if i > 0 {
			content += "    "
		}
		content += ui.ButtonStyle.Render(ctrl.key) + " - " + ctrl.description
	}
	return content
}

// internal/ui/views/edit.go - część 3

// internal/ui/views/edit.go

func (v *editView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.model.UpdateWindowSize(msg.Width, msg.Height)
		return v, nil
	case tea.KeyMsg:
		// Najpierw sprawdzamy czy jesteśmy w trybie wprowadzania tekstu
		if v.editing && v.mode != modeSelectPassword &&
			v.mode != modeHostList && v.mode != modePasswordList {
			// Obsługujemy tylko klawisze specjalne w trybie edycji
			switch msg.String() {
			case "esc":
				return v.handleEscapeKey()
			case "enter":
				return v.handleEnterKey()
			case "tab", "shift+tab", "up", "down":
				return v.handleNavigationKey(msg.String())
			default:
				// Przekazujemy wszystkie inne klawisze do aktywnego pola tekstowego
				v.inputs[v.activeField], cmd = v.inputs[v.activeField].Update(msg)
				return v, cmd
			}
		}

		// Jeśli nie jesteśmy w trybie edycji, obsługujemy wszystkie klawisze normalnie
		switch msg.String() {
		case "esc":
			return v.handleEscapeKey()
		case "tab", "shift+tab", "up", "down":
			return v.handleNavigationKey(msg.String())
		case "enter":
			return v.handleEnterKey()
		case "h", "H", "p", "P":
			return v.handleModeKey(msg.String())
		case "e", "d":
			return v.handleActionKey(msg.String())
		}
	}

	return v, cmd
}

func (v *editView) handleEscapeKey() (tea.Model, tea.Cmd) {
	switch v.mode {
	case modeSelectPassword:
		v.mode = modeNormal
		v.editing = false
		v.resetState()
		return v, nil

	case modeHostList, modePasswordList:
		v.mode = modeNormal
		v.editing = false
		v.resetState()
		return v, nil

	default:
		if !v.editing {
			v.model.SetStatus("", false)
			v.model.SetActiveView(ui.ViewMain)
			v.resetState()
			return v, nil
		}
		v.editing = false
		v.resetState()
		return v, nil
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

func (v *editView) handleModeKey(key string) (tea.Model, tea.Cmd) {
	if v.editing {
		return v, nil
	}

	switch key {
	case "h":
		v.editing = true
		v.editingHost = true
		v.initializeHostInputs()

	case "H":
		v.mode = modeHostList
		v.editing = true
		v.hosts = v.model.GetHosts()
		v.selectedItemIndex = 0
		v.deleteConfirmation = false

	case "p":
		v.editing = true
		v.editingHost = false
		v.initializePasswordInputs()

	case "P":
		v.mode = modePasswordList
		v.editing = true
		v.passwords = v.model.GetPasswords()
		v.selectedItemIndex = 0
		v.deleteConfirmation = false
	}

	return v, nil
}

// internal/ui/views/edit.go - część 4

func (v *editView) handleActionKey(key string) (tea.Model, tea.Cmd) {
	switch v.mode {
	case modeHostList:
		if len(v.hosts) == 0 {
			return v, nil
		}
		return v.handleHostListAction(key)

	case modePasswordList:
		if len(v.passwords) == 0 {
			return v, nil
		}
		return v.handlePasswordListAction(key)
	}
	return v, nil
}

func (v *editView) handleHostListAction(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "e":
		v.currentHost = &v.hosts[v.selectedItemIndex]
		v.editingHost = true
		v.mode = modeNormal
		v.initializeHostInputs()
		return v, nil

	case "d":
		if !v.deleteConfirmation {
			v.errorMsg = "Press 'd' again to confirm deletion"
			v.deleteConfirmation = true
			return v, nil
		}

		host := v.hosts[v.selectedItemIndex]
		if err := v.model.DeleteHost(host.Name); err != nil {
			v.errorMsg = fmt.Sprint(err)
		} else {
			if err := v.model.SaveConfig(); err != nil {
				v.errorMsg = fmt.Sprintf("Failed to save configuration: %v", err)
				return v, nil
			}
			v.model.UpdateLists()
			v.hosts = v.model.GetHosts()
			if v.selectedItemIndex >= len(v.hosts) {
				v.selectedItemIndex = len(v.hosts) - 1
			}
			v.model.SetStatus("Host deleted successfully", false)
		}
		v.deleteConfirmation = false
		return v, nil
	}
	return v, nil
}

func (v *editView) handlePasswordListAction(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "e":
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

		password := v.passwords[v.selectedItemIndex]
		if err := v.model.DeletePassword(password.Description); err != nil {
			v.errorMsg = fmt.Sprint(err)
		} else {
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
		return v, nil
	}
	return v, nil
}

func (v *editView) handleEnterKey() (tea.Model, tea.Cmd) {
	switch {
	case v.mode == modeSelectPassword:
		return v.saveHostWithPassword()

	case v.mode == modeHostList, v.mode == modePasswordList:
		return v, nil

	case !v.editing:
		v.editing = true
		v.editingHost = true
		v.initializeHostInputs()
		return v, nil

	default:
		return v.handleSave()
	}
}

func (v *editView) handleSave() (tea.Model, tea.Cmd) {
	if v.editingHost {
		return v.validateAndSaveHost()
	}
	return v.validateAndSavePassword()
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

	// Initialize temporary host
	v.tmpHost = &models.Host{
		Name:        v.inputs[0].Value(),
		Description: v.inputs[1].Value(),
		Login:       v.inputs[2].Value(),
		IP:          v.inputs[3].Value(),
		Port:        v.inputs[4].Value(),
	}

	// Switch to password selection mode
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

	// Create new password with encryption
	password, err := models.NewPassword(v.inputs[0].Value(), v.inputs[1].Value(), v.model.GetCipher())
	if err != nil {
		v.errorMsg = fmt.Sprintf("Failed to create password: %v", err)
		return v, nil
	}

	// Update or add password
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

	// Save configuration
	if err := v.model.SaveConfig(); err != nil {
		v.errorMsg = fmt.Sprintf("Failed to save configuration: %v", err)
		return v, nil
	}

	// Update UI state
	v.model.UpdateLists()
	v.model.SetStatus("Password saved successfully", false)
	v.editing = false
	v.resetState()
	return v, nil
}

// internal/ui/views/edit.go - część 5

func (v *editView) saveHostWithPassword() (tea.Model, tea.Cmd) {
	v.tmpHost.PasswordID = v.selectedPasswordIndex

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

	if err := v.model.SaveConfig(); err != nil {
		v.errorMsg = fmt.Sprintf("Failed to save configuration: %v", err)
		return v, nil
	}

	v.mode = modeNormal
	v.model.UpdateLists()
	v.model.SetStatus("Host saved successfully", false)
	v.editing = false
	v.resetState()
	return v, nil
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
	v.hosts = nil
	v.passwords = nil
	v.passwordList = nil
	v.selectedItemIndex = 0
	v.selectedPasswordIndex = 0

	// Reset all inputs
	for i := range v.inputs {
		v.inputs[i].Reset()
		v.inputs[i].Blur()
	}
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
