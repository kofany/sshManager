// internal/ui/models.go

package ui

import (
	"fmt"
	"sshManager/internal/config"
	"sshManager/internal/crypto"
	"sshManager/internal/models"
	"sshManager/internal/ssh"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// KeyMap definiuje skróty klawiszowe
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
	Edit     key.Binding
	Delete   key.Binding
	Transfer key.Binding
	Connect  key.Binding
}

// DefaultKeyMap zwraca domyślne ustawienia klawiszy
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "góra"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "dół"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "wybierz"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "wróć"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "wyjdź"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edytuj"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "usuń"),
		),
		Transfer: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "transfer"),
		),
		Connect: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "połącz"),
		),
	}
}

// Status reprezentuje stan aplikacji
type Status struct {
	Message string
	IsError bool
}

type View int

const (
	ViewMain View = iota
	ViewConnect
	ViewEdit
	ViewTransfer
	ViewHostList
	ViewPasswordList
	ViewHostEdit
	ViewPasswordEdit
)

// Model reprezentuje główny model aplikacji
type Model struct {
	keys         KeyMap
	status       Status
	activeView   View
	connection   *ssh.Connection
	transfer     *ssh.FileTransfer
	hosts        []models.Host
	passwords    []models.Password
	selectedHost *models.Host
	hostList     list.Model
	passwordList list.Model
	input        textinput.Model
	width        int
	height       int
	quitting     bool
	config       *config.Manager
	cipher       *crypto.Cipher
}

// Init implementuje tea.Model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.DisconnectHost() // Zamknij połączenie przed wyjściem
			m.quitting = true
			return m, tea.Quit
		case "c":
			if m.activeView == ViewMain {
				m.activeView = ViewHostList
				return m, nil
			}
		case "esc":
			if m.activeView != ViewMain {
				m.activeView = ViewMain
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.hostList.SetWidth(msg.Width)
		m.hostList.SetHeight(msg.Height - 4)
		m.passwordList.SetWidth(msg.Width)
		m.passwordList.SetHeight(msg.Height - 4)
	}

	// Aktualizacja aktywnego widoku
	switch m.activeView {
	case ViewHostList:
		newListModel, cmd := m.hostList.Update(msg)
		m.hostList = newListModel
		if item, ok := m.hostList.SelectedItem().(HostItem); ok {
			m.selectedHost = &item.host
		}
		return m, cmd
	case ViewPasswordList:
		newListModel, cmd := m.passwordList.Update(msg)
		m.passwordList = newListModel
		return m, cmd
	}

	return m, cmd
}

// View implementuje tea.Model
func (m Model) View() string {
	if m.quitting {
		return "Do widzenia!\n"
	}

	var view string
	switch m.activeView {
	case ViewMain:
		view = m.viewMain()
	case ViewHostList:
		view = m.hostList.View()
	case ViewPasswordList:
		view = m.passwordList.View()
	}

	// Dodaj status jeśli istnieje
	if m.status.Message != "" {
		style := SuccessStyle
		if m.status.IsError {
			style = ErrorStyle
		}
		view += "\n" + style.Render(m.status.Message)
	}

	return view
}

// viewMain renderuje główny widok
func (m Model) viewMain() string {
	return WindowStyle.Render(
		TitleStyle.Render("SSH Manager") + "\n\n" +
			"c - Połącz\n" +
			"e - Edytuj\n" +
			"t - Transfer plików\n" +
			"q - Wyjście",
	)
}

// NewModel tworzy nowy model aplikacji
func NewModel() Model {
	configManager := config.NewManager("") // pusty string użyje domyślnej ścieżki

	m := Model{
		keys:         DefaultKeyMap(),
		activeView:   ViewMain,
		input:        textinput.New(),
		hostList:     initializeList("Hosty"),
		passwordList: initializeList("Hasła"),
		config:       configManager,
	}

	// Wczytaj zapisaną konfigurację
	if err := configManager.Load(); err != nil {
		// Można dodać obsługę błędu
	}

	// Załaduj dane do modelu
	m.hosts = configManager.GetHosts()
	m.passwords = configManager.GetPasswords()
	m.UpdateLists()

	return m
}

// SaveConfig zapisuje konfigurację
// SaveConfig zapisuje konfigurację
func (m *Model) SaveConfig() interface{} {
	if err := m.config.Save(); err != nil {
		return fmt.Errorf("nie udało się zapisać konfiguracji: %v", err)
	}
	return nil
}

// initializeList inicjalizuje nową listę
func initializeList(title string) list.Model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = title
	l.SetShowHelp(true)
	l.SetFilteringEnabled(false)
	return l
}

// UpdateLists aktualizuje listy hostów i haseł
// internal/ui/models.go

// UpdateLists aktualizuje listy hostów i haseł
func (m *Model) UpdateLists() {
	// Pobierz aktualne dane z konfiguracji
	m.hosts = m.config.GetHosts()
	m.passwords = m.config.GetPasswords()

	// Aktualizacja listy hostów
	var hostItems []list.Item
	for _, h := range m.hosts {
		hostItems = append(hostItems, HostItem{host: h})
	}
	m.hostList.SetItems(hostItems)

	// Aktualizacja listy haseł
	var passwordItems []list.Item
	for _, p := range m.passwords {
		passwordItems = append(passwordItems, PasswordItem{password: p})
	}
	m.passwordList.SetItems(passwordItems)
}

// HostItem implementuje list.Item dla hosta
type HostItem struct {
	host models.Host
}

func (i HostItem) Title() string       { return i.host.Name }
func (i HostItem) Description() string { return i.host.Description }
func (i HostItem) FilterValue() string { return i.host.Name }

// PasswordItem implementuje list.Item dla hasła
type PasswordItem struct {
	password models.Password
}

func (i PasswordItem) Title() string       { return i.password.Description }
func (i PasswordItem) Description() string { return "********" }
func (i PasswordItem) FilterValue() string { return i.password.Description }

// SetStatus ustawia status aplikacji
func (m *Model) SetStatus(msg string, isError bool) {
	m.status = Status{
		Message: msg,
		IsError: isError,
	}
}

// ClearStatus czyści status
func (m *Model) ClearStatus() {
	m.status = Status{}
}

// ConnectToHost nawiązuje połączenie z wybranym hostem
func (m *Model) ConnectToHost(host *models.Host, password string) interface{} {
	// Zamknij poprzednie połączenie jeśli istnieje
	if m.connection != nil {
		m.connection.Close()
	}

	config := &ssh.ConnectionConfig{
		Host:     host,
		Password: password,
	}

	conn, err := ssh.NewConnection(config)
	if err != nil {
		return fmt.Errorf("nie udało się nawiązać połączenia: %v", err)
	}

	m.connection = conn
	m.selectedHost = host
	m.transfer = ssh.NewFileTransfer(conn)
	return nil
}

// DisconnectHost zamyka połączenie
func (m *Model) DisconnectHost() interface{} {
	if m.connection != nil {
		err := m.connection.Close()
		m.connection = nil
		m.transfer = nil
		m.selectedHost = nil
		if err != nil {
			return fmt.Errorf("błąd podczas zamykania połączenia: %v", err)
		}
	}
	return nil
}

// GetSelectedHost zwraca aktualnie wybrany host
func (m *Model) GetSelectedHost() *models.Host {
	return m.selectedHost
}

// SetSelectedHost ustawia wybrany host
func (m *Model) SetSelectedHost(host *models.Host) {
	m.selectedHost = host
}

// IsConnected sprawdza czy jest aktywne połączenie
func (m *Model) IsConnected() bool {
	return m.connection != nil && m.connection.IsConnected()
}

// GetConnection zwraca aktywne połączenie
func (m *Model) GetConnection() *ssh.Connection {
	return m.connection
}

// GetTransfer zwraca obiekt do transferu plików
func (m *Model) GetTransfer() *ssh.FileTransfer {
	return m.transfer
}

func (m *Model) SetActiveView(view View) {
	m.activeView = view
}

// Dodaj te metody w internal/ui/models.go

// AddHost dodaje nowego hosta
func (m *Model) AddHost(host *models.Host) interface{} {
	// Sprawdzenie czy host o takiej nazwie już istnieje
	for _, h := range m.config.GetHosts() {
		if h.Name == host.Name {
			return fmt.Errorf("host o nazwie %s już istnieje", host.Name)
		}
	}

	// Dodaj hosta do konfiguracji
	m.config.AddHost(*host)

	// Zaktualizuj lokalną listę hostów
	m.hosts = m.config.GetHosts()
	return nil
}

// UpdateHost aktualizuje istniejącego hosta
func (m *Model) UpdateHost(oldName string, host *models.Host) interface{} {
	for i, h := range m.hosts {
		if h.Name == oldName {
			m.hosts[i] = *host
			return nil
		}
	}
	return fmt.Errorf("nie znaleziono hosta %s", oldName)
}

// AddPassword dodaje nowe hasło

func (m *Model) AddPassword(password *models.Password) interface{} {
	// Sprawdzenie czy hasło o takim opisie już istnieje
	for _, p := range m.config.GetPasswords() {
		if p.Description == password.Description {
			return fmt.Errorf("hasło o opisie %s już istnieje", password.Description)
		}
	}

	// Dodaj hasło do konfiguracji
	m.config.AddPassword(*password)

	// Zapisz konfigurację
	if err := m.config.Save(); err != nil {
		return fmt.Errorf("nie udało się zapisać konfiguracji: %v", err)
	}

	// Aktualizuj lokalną listę haseł
	m.passwords = m.config.GetPasswords()
	return nil
}

// UpdatePassword aktualizuje istniejące hasło
func (m *Model) UpdatePassword(oldDesc string, password *models.Password) interface{} {
	for i, p := range m.passwords {
		if p.Description == oldDesc {
			m.passwords[i] = *password
			return nil
		}
	}
	return fmt.Errorf("nie znaleziono hasła %s", oldDesc)
}

// GetHosts zwraca listę hostów
func (m *Model) GetHosts() []models.Host {
	return m.hosts
}

// GetPasswords zwraca listę haseł
func (m *Model) GetPasswords() []models.Password {
	return m.passwords
}

// Dodaj w internal/ui/models.go

// GetPasswordByIndex zwraca hasło o danym indeksie
func (m *Model) GetPasswordByIndex(index int) *models.Password {
	if index >= 0 && index < len(m.passwords) {
		return &m.passwords[index]
	}
	return nil
}

func (m *Model) SetCipher(cipher *crypto.Cipher) {
	m.cipher = cipher
}

func (m *Model) GetCipher() *crypto.Cipher {
	return m.cipher
}
