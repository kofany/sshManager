package views

import (
	"sshManager/internal/crypto"
	"sshManager/internal/ui"
	"sshManager/internal/ui/messages"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type initialPromptModel struct {
	password      []rune
	configPath    string
	errorMessage  string
	width, height int
}

type ApiKeyPromptModel struct {
	input        textinput.Model
	configPath   string
	errorMessage string
	width        int
	height       int
	cipher       *crypto.Cipher
}

func NewApiKeyPromptModel(configPath string, cipher *crypto.Cipher) *ApiKeyPromptModel {
	input := textinput.New()
	input.Placeholder = "Enter API key (or press ESC for local mode)"
	input.Focus()

	return &ApiKeyPromptModel{
		input:      input,
		configPath: configPath,
		cipher:     cipher,
	}
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
			// Wyczyść ekran i wyślij hasło
			return m, tea.Sequence(
				tea.ClearScreen,
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
                        https://sshm.io |___/`

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

func (m *ApiKeyPromptModel) Init() tea.Cmd {
	return textinput.Blink // Dodane dla migającego kursora w polu input
}

func (m *ApiKeyPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			// Użytkownik wybrał tryb lokalny
			return m, tea.Sequence(
				tea.ClearScreen,
				func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				},
				func() tea.Msg {
					return messages.ApiKeyEnteredMsg{
						Key:       "",
						LocalMode: true,
					}
				},
			)

		case tea.KeyEnter:
			apiKey := m.input.Value()
			if len(apiKey) == 0 {
				m.errorMessage = "API key cannot be empty. Press ESC for local mode."
				return m, nil
			}

			// Sprawdź podstawową walidację klucza API (np. minimalna długość)
			if len(apiKey) < 32 {
				m.errorMessage = "Invalid API key format"
				return m, nil
			}

			return m, tea.Sequence(
				tea.ClearScreen,
				func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				},
				func() tea.Msg {
					return messages.ApiKeyEnteredMsg{
						Key:       apiKey,
						LocalMode: false,
					}
				},
			)

		case tea.KeyCtrlC:
			return m, tea.Quit
		}

		// Obsługa wprowadzania tekstu
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *ApiKeyPromptModel) View() string {
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

	// ASCII Art - ten sam co w pierwotnym pliku
	asciiArt := `
         _     __  __                                   
 ___ ___| |__ |  \/  | __ _ _ __   __ _  __ _  ___ _ __ 
/ __/ __| '_ \| |\/| |/ _' | '_ \ / _' |/ _' |/ _ \ '__|
\__ \__ \ | | | |  | | (_| | | | | (_| | (_| |  __/ |   
|___/___/_| |_|_|  |_|\__,_|_| |_|\__,_|\__, |\___|_|   
                        https://sshm.io |___/`

	asciiArtRendered := asciiArtStyle.Render(asciiArt)

	// Informacje
	configInfo := infoStyle.Render("Using config file: " + m.configPath)

	apiInfo := infoStyle.Render("Press ESC to work in local mode without synchronization\n" +
		"If you don't have an API key, please register at https://sshm.io")

	// Prompt dla API key
	apiKeyPrompt := promptStyle.Render("Enter API key: ")
	maskedApiKey := strings.Repeat("*", len(m.input.Value()))

	// Połączenie wszystkich elementów
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		asciiArtRendered,
		"",
		configInfo,
		"",
		apiInfo,
		"",
		apiKeyPrompt+maskedApiKey,
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
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		framedContent,
	)

	return finalContent
}
