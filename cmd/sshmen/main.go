package main

import (
	"flag"
	"fmt"
	"os"
	"sshManager/internal/config"
	"sshManager/internal/crypto"
	"sshManager/internal/ui"
	"sshManager/internal/ui/messages"
	"sshManager/internal/ui/views"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type mode int

const (
	modeConnect mode = iota
	modeEdit
	modeTransfer
)

type programModel struct {
	mode        mode
	quitting    bool
	uiModel     *ui.Model
	currentView tea.Model
	cipher      *crypto.Cipher
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
		mode:        modeConnect,
		uiModel:     uiModel,
		currentView: initialPrompt,
	}
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

		// Przełączenie na główny widok
		m.updateCurrentView()

		// Inicjalizacja nowego widoku
		initCmd := m.currentView.Init()

		// Zwracamy model i komendę inicjalizującą
		return m, initCmd

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

func (m *programModel) View() string {
	if m.quitting || m.uiModel.IsQuitting() {
		return "Goodbye!\n"
	}
	return m.currentView.View()
}

func main() {
	// Parsowanie flag linii komend
	editMode := flag.Bool("edit", false, "Edit mode")
	transferMode := flag.Bool("file-transfer", false, "File transfer mode")
	flag.Parse()

	// Inicjalizacja modelu programu
	m := initialModel()

	// Ustawienie początkowego widoku na podstawie flag
	if *editMode {
		m.mode = modeEdit
		m.uiModel.SetActiveView(ui.ViewEdit)
	} else if *transferMode {
		m.mode = modeTransfer
		m.uiModel.SetActiveView(ui.ViewTransfer)
	}

	// Uruchomienie programu
	p := tea.NewProgram(m, tea.WithMouseCellMotion(), tea.WithAltScreen())
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
