package views

import (
	"sshManager/internal/ui"
	"sshManager/internal/ui/messages"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type initialPromptModel struct {
	password      []rune
	configPath    string
	errorMessage  string
	width, height int
}

func NewInitialPromptModel(configPath string) *initialPromptModel {
	return &initialPromptModel{
		password:   []rune{},
		configPath: configPath,
	}
}

func (m *initialPromptModel) Init() tea.Cmd {
	return nil
}

func (m *initialPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRunes:
			m.password = append(m.password, msg.Runes...)
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.password) > 0 {
				m.password = m.password[:len(m.password)-1]
			}
		case tea.KeyEnter:
			if len(m.password) == 0 {
				m.errorMessage = "Password cannot be empty"
				return m, nil
			}
			// Najpierw wyczyść ekran, potem wyślij hasło
			return m, tea.Sequence(
				tea.ClearScreen,
				tea.ClearScrollArea,
				func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				},
				func() tea.Msg {
					return messages.PasswordEnteredMsg(string(m.password))
				},
			)
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m *initialPromptModel) View() string {
	// Definicja stylów
	asciiArtStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7DC4E4")).
		Bold(true)

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A6ADC8")).
		Italic(true)

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	errorStyle := ui.ErrorStyle

	// ASCII Art
	asciiArt := `
         _     __  __                                   
 ___ ___| |__ |  \/  | __ _ _ __   __ _  __ _  ___ _ __ 
/ __/ __| '_ \| |\/| |/ _' | '_ \ / _' |/ _' |/ _ \ '__|
\__ \__ \ | | | |  | | (_| | | | | (_| | (_| |  __/ |   
|___/___/_| |_|_|  |_|\__,_|_| |_|\__,_|\__, |\___|_|   
                                        |___/`

	asciiArtRendered := asciiArtStyle.Render(asciiArt)

	// Informacja o pliku konfiguracyjnym
	configInfo := infoStyle.Render("Using config file: " + m.configPath)

	// Pytanie o hasło
	passwordPrompt := promptStyle.Render("Enter encryption key: ")
	maskedPassword := strings.Repeat("*", len(m.password))

	// Połączenie wszystkich elementów
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		asciiArtRendered,
		"",
		configInfo,
		"",
		passwordPrompt+maskedPassword,
	)

	// Dodanie komunikatu o błędzie, jeśli istnieje
	if m.errorMessage != "" {
		content += "\n" + errorStyle.Render(m.errorMessage)
	}

	// Ramka wokół zawartości
	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7DC4E4")).
		Padding(1, 2)

	framedContent := frameStyle.Render(content)

	// Wyśrodkowanie zawartości
	finalContent := lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		framedContent,
	)

	return finalContent
}
