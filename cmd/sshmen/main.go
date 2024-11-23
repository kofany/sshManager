// cmd/sshmen/main.go

package main

import (
	"flag"
	"fmt"
	"os"
	"sshManager/internal/crypto"
	"sshManager/internal/ui"
	"sshManager/internal/ui/views"

	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/crypto/ssh/terminal"
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
	// Używamy & aby uzyskać wskaźnik do modelu UI
	uiModel := &ui.Model{}
	*uiModel = ui.NewModel()
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

func (m *programModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.currentView, cmd = m.currentView.Update(msg)
	return m, cmd
}

func (m *programModel) View() string {
	if m.quitting {
		return "Do widzenia!\n"
	}
	return m.currentView.View()
}

func main() {
	editMode := flag.Bool("edit", false, "Edit mode")
	transferMode := flag.Bool("file-transfer", false, "File transfer mode")
	flag.Parse()

	m := initialModel()

	// Pytanie o klucz szyfrowania
	fmt.Print("Wprowadź klucz szyfrowania: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("Błąd podczas odczytu klucza szyfrowania: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	cipher := crypto.NewCipher(string(bytePassword))
	m.uiModel.SetCipher(cipher)

	// Ustawienie początkowego widoku na podstawie flag
	if *editMode {
		m.mode = modeEdit
		m.uiModel.SetActiveView(ui.ViewEdit)
		m.currentView = views.NewEditView(m.uiModel)
	} else if *transferMode {
		m.mode = modeTransfer
		m.uiModel.SetActiveView(ui.ViewTransfer)
		m.currentView = views.NewTransferView(m.uiModel)
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
