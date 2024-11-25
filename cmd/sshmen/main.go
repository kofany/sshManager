// cmd/sshmen/main.go

package main

import (
	"flag"
	"fmt"
	"os"
	"sshManager/internal/config"
	"sshManager/internal/crypto"
	"sshManager/internal/ui"
	"sshManager/internal/ui/views"
	"syscall"

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
}

func initialModel() *programModel {
	uiModel := ui.NewModel()
	mainView := views.NewMainView(uiModel)

	return &programModel{
		mode:        modeConnect,
		uiModel:     uiModel,
		currentView: mainView,
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
	switch m.uiModel.GetActiveView() {
	case ui.ViewMain:
		m.currentView = views.NewMainView(m.uiModel)
	case ui.ViewEdit:
		m.currentView = views.NewEditView(m.uiModel)
	case ui.ViewTransfer:
		m.currentView = views.NewTransferView(m.uiModel)
	}
}

func (m *programModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Sprawdź czy użytkownik chce zakończyć program
	if m.uiModel.IsQuitting() || m.quitting {
		return m, tea.Quit
	}

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

	// Pokaż informację o lokalizacji pliku konfiguracyjnego
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		fmt.Printf("Warning: Could not determine config path: %v\n", err)
		configPath = config.DefaultConfigFileName
	}
	fmt.Printf("Using config file: %s\n", configPath)

	// Wczytanie klucza szyfrowania
	fmt.Print("Enter encryption key: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("Error reading encryption key: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Inicjalizacja szyfru
	key := crypto.GenerateKeyFromPassword(string(bytePassword))
	cipher := crypto.NewCipher(string(key))

	// Inicjalizacja modelu UI
	m := initialModel()
	m.uiModel.SetCipher(cipher)

	// Ustawienie początkowego widoku na podstawie flag
	if *editMode {
		m.mode = modeEdit
		m.uiModel.SetActiveView(ui.ViewEdit)
		m.updateCurrentView()
	} else if *transferMode {
		m.mode = modeTransfer
		m.uiModel.SetActiveView(ui.ViewTransfer)
		m.updateCurrentView()
	}

	// Uruchomienie programu
	p := tea.NewProgram(m)
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
