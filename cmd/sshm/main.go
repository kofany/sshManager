package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sshManager/internal/config"
	"sshManager/internal/crypto"
	"sshManager/internal/sync"
	"sshManager/internal/ui"
	"sshManager/internal/ui/messages"
	"sshManager/internal/ui/views"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type programModel struct {
	quitting    bool
	uiModel     *ui.Model
	currentView tea.Model
	cipher      *crypto.Cipher
	restarting  bool
}

func initialModel() *programModel {
	uiModel := ui.NewModel()

	// Pobranie ścieżki do pliku konfiguracyjnego
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		fmt.Printf("Warning: Could not determine config path: %v\n", err)
		configPath = config.DefaultConfigFileName
	}

	// Inicjalizacja widoku początkowego
	initialPrompt := views.NewInitialPromptModel(configPath)

	// Ustaw domyślny rozmiar terminala
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		uiModel.SetTerminalSize(w, h)
	}

	return &programModel{
		uiModel:     uiModel,
		currentView: initialPrompt,
	}
}

func (m *programModel) IsRestarting() bool {
	return m.restarting
}

func (m *programModel) Init() tea.Cmd {
	return m.currentView.Init()
}

func (m *programModel) SetProgram(p *tea.Program) {
	if m.uiModel != nil {
		m.uiModel.SetProgram(p)
	}
}

func (m *programModel) updateCurrentView() {
	if m.cipher == nil {
		// Wciąż jesteśmy w widoku początkowym
		return
	}

	switch m.uiModel.GetActiveView() {
	case ui.ViewMain:
		m.currentView = views.NewMainView(m.uiModel)
	case ui.ViewEdit:
		m.currentView = views.NewEditView(m.uiModel)
	case ui.ViewTransfer:
		m.currentView = views.NewTransferView(m.uiModel)
	default:
		// Domyślnie ustaw widok główny
		m.currentView = views.NewMainView(m.uiModel)
		m.uiModel.SetActiveView(ui.ViewMain)
	}
}
func (m *programModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Sprawdź czy użytkownik chce zakończyć program
	if m.uiModel.IsQuitting() || m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case messages.PasswordEnteredMsg:
		// Inicjalizacja szyfru
		key := crypto.GenerateKeyFromPassword(string(msg))
		m.cipher = crypto.NewCipher(string(key))
		m.uiModel.SetCipher(m.cipher)
		m.uiModel.GetConfig().SetCipher(m.cipher) // Dodane

		// Po zainicjowaniu szyfru sprawdzamy czy mamy zapisany klucz API
		apiKey, err := m.uiModel.GetConfig().LoadApiKey(m.cipher)
		if err != nil {
			// Jeśli nie ma klucza API, pokazujemy prompt do jego wprowadzenia
			m.currentView = views.NewApiKeyPromptModel(m.uiModel.GetConfig().GetConfigPath(), m.cipher)
			return m, m.currentView.Init()
		}

		// Jeśli mamy klucz API, wykonujemy synchronizację
		return m, m.handleApiKeyAndSync(apiKey, false)

	case messages.ApiKeyEnteredMsg:
		if msg.LocalMode {
			// Użytkownik wybrał tryb lokalny (nacisnął ESC)
			m.uiModel.SetLocalMode(true)
			m.updateCurrentView()
			return m, m.currentView.Init()
		}

		// Zapisz nowy klucz API
		if err := m.uiModel.GetConfig().SaveApiKey(msg.Key, m.cipher); err != nil {
			fmt.Printf("Warning: Could not save API key: %v\n", err)
			m.uiModel.SetLocalMode(true)
		}

		return m, m.handleApiKeyAndSync(msg.Key, false)

	case messages.ReloadAppMsg:
		m.restarting = true
		m.quitting = true
		return m, tea.Quit

	default:
		// Zapisz aktualny widok
		currentActiveView := m.uiModel.GetActiveView()

		// Aktualizuj obecny widok
		var cmd tea.Cmd
		m.currentView, cmd = m.currentView.Update(msg)

		// Sprawdź czy zmienił się aktywny widok
		if currentActiveView != m.uiModel.GetActiveView() {
			m.updateCurrentView()
		}

		return m, cmd
	}
}

func (m *programModel) handleApiKeyAndSync(apiKey string, isLocalMode bool) tea.Cmd {
	if isLocalMode {
		m.uiModel.SetLocalMode(true)
		m.updateCurrentView()
		return m.currentView.Init()
	}

	// Pobranie ścieżek
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		fmt.Printf("Warning: Could not determine config path: %v\n", err)
		configPath = config.DefaultConfigFileName
	}
	keysDir := filepath.Join(filepath.Dir(configPath), config.DefaultKeysDir)

	// Tworzenie kopii zapasowych
	if err := sync.BackupConfigFile(configPath); err != nil {
		fmt.Printf("Warning: Could not create config backup: %v\n", err)
	}
	if err := sync.BackupKeys(keysDir); err != nil {
		fmt.Printf("Warning: Could not create keys backup: %v\n", err)
	}

	// Synchronizacja z API
	syncResp, err := sync.SyncWithAPI(apiKey)
	if err != nil {
		fmt.Printf("Warning: Could not sync with API: %v\n", err)
		m.uiModel.SetLocalMode(true)
	} else {
		// Zapisz dane z API - dodajemy przekazanie cipher
		if err := sync.SaveAPIData(configPath, keysDir, syncResp.Data, m.cipher); err != nil {
			fmt.Printf("Warning: Could not save API data: %v\n", err)
			if err := sync.RestoreFromBackup(configPath, keysDir); err != nil {
				fmt.Printf("Error: Could not restore from backup: %v\n", err)
				os.Exit(1)
			}
			m.uiModel.SetLocalMode(true)
		}
	}

	// Przejście do głównego widoku
	m.updateCurrentView()
	return m.currentView.Init()
}

func (m *programModel) View() string {
	if m.quitting || m.uiModel.IsQuitting() {
		return "Goodbye!\n"
	}
	return m.currentView.View()
}

func main() {
	for {
		m := initialModel()
		p := tea.NewProgram(m, tea.WithAltScreen())
		m.SetProgram(p)

		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running program: %v", err)
			os.Exit(1)
		}

		// Jeśli nie jest to restart, wyjdź z pętli
		if !m.restarting {
			break
		}
	}
}