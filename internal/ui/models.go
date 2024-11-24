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
	Connect  key.Binding
	Transfer key.Binding
	Refresh  key.Binding
}

// DefaultKeyMap zwraca domyślne ustawienia klawiszy
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Connect: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "connect"),
		),
		Transfer: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "transfer"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
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
	sshClient    *ssh.SSHClient // Zmiana z Connection na SSHClient
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
// internal/ui/models.go - w funkcji NewModel() zaktualizuj inicjalizację configManager

func NewModel() Model {
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		configPath = config.DefaultConfigFileName
	}

	configManager := config.NewManager(configPath)

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
		m.SetStatus(fmt.Sprintf("Warning: %v", err), true)
	}

	// Załaduj dane do modelu
	m.hosts = configManager.GetHosts()
	m.passwords = configManager.GetPasswords()
	m.UpdateLists()

	return m
}

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

// ConnectToHost establishes connection with selected host
func (m *Model) ConnectToHost(host *models.Host, password string) interface{} {
	// Jeśli istnieje poprzednie połączenie, zamknij je
	if m.sshClient != nil {
		m.DisconnectHost()
	}

	// Utwórz nowego klienta SSH
	m.sshClient = ssh.NewSSHClient(m.passwords)

	// Nawiąż połączenie
	err := m.sshClient.Connect(host, password)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	m.selectedHost = host

	// Utwórz nowy obiekt transferu plików
	m.transfer = ssh.NewFileTransfer(m.sshClient, m.cipher)

	return nil
}

func (m *Model) DisconnectHost() interface{} {
	if m.transfer != nil {
		if err := m.transfer.Disconnect(); // Używamy Disconnect zamiast Close
		err != nil {
			return fmt.Errorf("error disconnecting transfer: %v", err)
		}
		m.transfer = nil
	}
	if m.sshClient != nil {
		m.sshClient.Disconnect()
		m.sshClient = nil
	}
	m.selectedHost = nil
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
	return m.sshClient != nil && m.sshClient.IsConnected()
}

// GetConnection zwraca aktywne połączenie
func (m *Model) GetSSHClient() *ssh.SSHClient {
	return m.sshClient
}

// GetTransfer zwraca obiekt do transferu plików
func (m *Model) GetTransfer() *ssh.FileTransfer {
	if m.transfer == nil && m.IsConnected() {
		m.transfer = ssh.NewFileTransfer(m.sshClient, m.cipher)
	}
	return m.transfer
}

// SetActiveView switch view and initialize if needed
func (m *Model) SetActiveView(view View) {
	m.activeView = view
	// Resetujemy komunikaty o błędach
	m.status = Status{}

	// Inicjalizujemy odpowiedni widok
	switch view {
	case ViewConnect:
		if m.sshClient != nil { // Zmiana z connection na sshClient
			m.DisconnectHost() // Używamy istniejącej metody do rozłączenia
		}
	case ViewMain:
		m.UpdateLists() // Odświeżamy listy przy powrocie do głównego widoku
	}
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

// DeleteHost usuwa hosta
func (m *Model) DeleteHost(name string) interface{} {
	// Najpierw znajdź hosta w konfiguracji
	for i, h := range m.config.GetHosts() {
		if h.Name == name {
			// Usuń z konfiguracji
			if err := m.config.DeleteHost(i); err != nil {
				return fmt.Errorf("nie można usunąć hosta: %v", err)
			}
			// Usuń z lokalnej listy
			for j, host := range m.hosts {
				if host.Name == name {
					m.hosts = append(m.hosts[:j], m.hosts[j+1:]...)
					break
				}
			}
			return nil
		}
	}
	return fmt.Errorf("nie znaleziono hosta %s", name)
}

// DeletePassword usuwa hasło
func (m *Model) DeletePassword(description string) interface{} {
	// Najpierw znajdź indeks hasła
	var passwordIndex int = -1
	for i, p := range m.config.GetPasswords() {
		if p.Description == description {
			passwordIndex = i
			break
		}
	}

	if passwordIndex == -1 {
		return fmt.Errorf("nie znaleziono hasła %s", description)
	}

	// Sprawdź czy hasło nie jest używane przez żadnego hosta
	for _, h := range m.config.GetHosts() {
		if h.PasswordID == passwordIndex {
			return fmt.Errorf("hasło jest używane przez hosta %s", h.Name)
		}
	}

	// Usuń hasło z konfiguracji
	if err := m.config.DeletePassword(passwordIndex); err != nil {
		return fmt.Errorf("nie można usunąć hasła: %v", err)
	}

	// Usuń z lokalnej listy
	for i, p := range m.passwords {
		if p.Description == description {
			m.passwords = append(m.passwords[:i], m.passwords[i+1:]...)
			break
		}
	}

	return nil
}

func (m *Model) GetActiveView() View {
	return m.activeView
}
