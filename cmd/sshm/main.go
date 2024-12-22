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
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// programModel represents the main application model
type programModel struct {
	quitting    bool           // Indicates if the program is quitting
	uiModel     *ui.Model      // Holds the UI state and configuration
	currentView tea.Model      // Represents the current active view
	cipher      *crypto.Cipher // Handles encryption/decryption
	restarting  bool           // Indicates if the program is restarting
}

// Initializes the initial program model
func initialModel() *programModel {
	uiModel := ui.NewModel()

	// Retrieve the path to the configuration file
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		fmt.Printf("Warning: Could not determine config path: %v\n", err)
		configPath = config.DefaultConfigFileName
	}

	// Initialize the initial view
	initialPrompt := views.NewInitialPromptModel(configPath)

	// Set the default terminal size
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		uiModel.SetTerminalSize(w, h)
	}

	return &programModel{
		uiModel:     uiModel,
		currentView: initialPrompt,
	}
}

// Checks if the program is restarting
func (m *programModel) IsRestarting() bool {
	return m.restarting
}

// Initialize the program's initial view
func (m *programModel) Init() tea.Cmd {
	return m.currentView.Init()
}

// Sets the tea.Program instance for the UI model
func (m *programModel) SetProgram(p *tea.Program) {
	if m.uiModel != nil {
		m.uiModel.SetProgram(p)
	}
}

// Updates the current view based on the active view in the UI model
func (m *programModel) updateCurrentView() {
	if m.cipher == nil {
		// Still in the initial view
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
		// Default to the main view
		m.currentView = views.NewMainView(m.uiModel)
		m.uiModel.SetActiveView(ui.ViewMain)
	}
}

// Handles updating the program state based on incoming messages
func (m *programModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check if the user wants to quit the program
	if m.uiModel.IsQuitting() {
		m.quitting = true
		return m, tea.Quit
	}
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case messages.PasswordEnteredMsg:
		// Initialize the encryption cipher
		key := crypto.GenerateKeyFromPassword(string(msg))
		m.cipher = crypto.NewCipher(string(key))
		m.uiModel.SetCipher(m.cipher)
		m.uiModel.GetConfig().SetCipher(m.cipher) // Set the cipher in the config

		// Check if an API key is stored
		apiKey, err := m.uiModel.GetConfig().LoadApiKey(m.cipher)
		if err != nil {
			// If no API key, prompt for input
			m.currentView = views.NewApiKeyPromptModel(m.uiModel.GetConfig().GetConfigPath(), m.cipher)
			return m, m.currentView.Init()
		}

		// If API key exists, perform synchronization
		return m, m.handleApiKeyAndSync(apiKey, false)

	case messages.ApiKeyEnteredMsg:
		if msg.LocalMode {
			// User selected local mode (pressed ESC)
			m.uiModel.SetLocalMode(true)
			m.updateCurrentView()
			return m, m.currentView.Init()
		}

		// Save the new API key
		if err := m.uiModel.GetConfig().SaveApiKey(msg.Key, m.cipher); err != nil {
			fmt.Printf("Warning: Could not save API key: %v\n", err)
			m.uiModel.SetLocalMode(true)
		}

		return m, m.handleApiKeyAndSync(msg.Key, false)

	case messages.ReloadAppMsg:
		// Handle application reload
		m.restarting = true
		m.quitting = true
		return m, tea.Quit

	default:
		// Store the currently active view
		currentActiveView := m.uiModel.GetActiveView()

		// Update the current view
		var cmd tea.Cmd
		m.currentView, cmd = m.currentView.Update(msg)

		// Check if the active view has changed
		if currentActiveView != m.uiModel.GetActiveView() {
			m.updateCurrentView()
		}

		return m, cmd
	}
}

// Handles the API key and performs synchronization
func (m *programModel) handleApiKeyAndSync(apiKey string, isLocalMode bool) tea.Cmd {
	if isLocalMode {
		m.uiModel.SetLocalMode(true)
		m.updateCurrentView()
		return m.currentView.Init()
	}

	// Retrieve paths
	configPath, err := config.GetDefaultConfigPath()
	if err != nil {
		fmt.Printf("Warning: Could not determine config path: %v\n", err)
		configPath = config.DefaultConfigFileName
	}
	keysDir := filepath.Join(filepath.Dir(configPath), config.DefaultKeysDir)

	// Create backups
	if err := sync.BackupConfigFile(configPath); err != nil {
		fmt.Printf("Warning: Could not create config backup: %v\n", err)
	}
	if err := sync.BackupKeys(keysDir); err != nil {
		fmt.Printf("Warning: Could not create keys backup: %v\n", err)
	}

	// Synchronize with the API
	syncResp, err := sync.SyncWithAPI(apiKey)
	if err != nil {
		fmt.Printf("Warning: Could not sync with API: %v\n", err)
		m.uiModel.SetLocalMode(true)
	} else {
		// Save data from the API
		if err := sync.SaveAPIData(configPath, keysDir, syncResp.Data, m.cipher); err != nil {
			fmt.Printf("Warning: Could not save API data: %v\n", err)
			if err := sync.RestoreFromBackup(configPath, keysDir); err != nil {
				fmt.Printf("Error: Could not restore from backup: %v\n", err)
				os.Exit(1)
			}
			m.uiModel.SetLocalMode(true)
		} else {
			// Load the saved configuration into the UI model
			if err := m.uiModel.GetConfig().Load(); err != nil {
				fmt.Printf("Warning: Could not load saved configuration: %v\n", err)
			}
			// Refresh lists in the UI model
			m.uiModel.UpdateLists()
		}
	}

	// Switch to the main view
	m.updateCurrentView()
	return m.currentView.Init()
}

// Renders the current view or a goodbye message if quitting
func (m *programModel) View() string {
	if m.quitting || m.uiModel.IsQuitting() {
		return "Goodbye!\n"
	}
	return m.currentView.View()
}

// Main entry point of the application
func main() {
	m := initialModel()
	var p *tea.Program
	var savedProgram *tea.Program // Variable for storing the program instance

	for {
		// Use the saved program if available, otherwise create a new one
		if savedProgram != nil {
			p = savedProgram
			savedProgram = nil
		} else {
			p = tea.NewProgram(m, tea.WithAltScreen())
			m.SetProgram(p)
		}

		// Run the program
		model, err := p.Run()
		if err != nil {
			if !strings.Contains(err.Error(), "program was killed") &&
				!strings.Contains(err.Error(), "context canceled") {
				fmt.Printf("Error running program: %v\n", err)
				os.Exit(1)
			}
		}

		m = model.(*programModel)
		if m.quitting {
			break
		}

		if sshClient := m.uiModel.GetSSHClient(); sshClient != nil {
			if session := sshClient.Session(); session != nil {
				savedProgram = p

				if err := p.ReleaseTerminal(); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to release terminal: %v\n", err)
					continue
				}

				// Handle SSH session
				sessionDone := make(chan error)
				go func() {
					if err := session.ConfigureTerminal("xterm-256color"); err != nil {
						sessionDone <- fmt.Errorf("failed to configure terminal: %v", err)
						return
					}
					sessionDone <- session.StartShell()
				}()

				if err := <-sessionDone; err != nil {
					fmt.Fprintf(os.Stderr, "Session error: %v\n", err)
				}

				// Close the session
				sshClient.Disconnect()
				m.uiModel.SetSSHClient(nil)
				m.uiModel.SetActiveView(ui.ViewMain)

				// Create a new main view with a popup
				mainView := views.NewMainView(m.uiModel)
				mainView.ShowSessionEndedPopup()
				m.currentView = mainView

				continue
			}
		}
	}
}
