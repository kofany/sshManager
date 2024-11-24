package views

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"sshManager/internal/ssh"
	"sshManager/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Dodaj na początku pliku po importach
func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// Stałe określające tryby i stany
const (
	localPanelActive  = true
	remotePanelActive = false
	maxVisibleItems   = 15
	headerHeight      = 3
	footerHeight      = 4
)

// FileEntry reprezentuje pojedynczy plik lub katalog
type FileEntry struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

// Panel reprezentuje panel plików (lokalny lub zdalny)
type Panel struct {
	path          string
	entries       []FileEntry
	selectedIndex int
	scrollOffset  int
	active        bool
}

// transferView implementuje główny widok transferu plików
type transferView struct {
	model         *ui.Model
	localPanel    Panel
	remotePanel   Panel
	statusMessage string
	errorMessage  string
	connecting    bool
	connected     bool
	transferring  bool
	progress      ssh.TransferProgress
	showHelp      bool
	input         textinput.Model
	mutex         sync.Mutex // Dodajemy mutex do bezpiecznej aktualizacji stanu
}

func NewTransferView(model *ui.Model) *transferView {
	input := textinput.New()
	input.Placeholder = "Enter command..."
	input.CharLimit = 255

	v := &transferView{
		model: model,
		localPanel: Panel{
			path:   getHomeDir(), // Zaczynamy od katalogu domowego
			active: true,
			entries: []FileEntry{
				{name: "..", isDir: true}, // Dodajemy ".." do nawigacji w górę
			},
		},
		remotePanel: Panel{
			path:   "/",
			active: false,
			entries: []FileEntry{
				{name: "..", isDir: true},
			},
		},
		input: input,
	}

	// Inicjalizujemy panel lokalny
	if err := v.updateLocalPanel(); err != nil {
		v.errorMessage = fmt.Sprintf("Failed to load local directory: %v", err)
		return v
	}

	// Inicjujemy połączenie SFTP w tle
	if v.model.GetSelectedHost() != nil {
		go func() {
			v.mutex.Lock()
			v.connecting = true
			v.mutex.Unlock()

			if err := v.ensureConnected(); err != nil {
				v.mutex.Lock()
				v.errorMessage = fmt.Sprintf("Connection error: %v", err)
				v.connecting = false
				v.mutex.Unlock()
				return
			}

			// Po połączeniu aktualizujemy panel zdalny
			if err := v.updateRemotePanel(); err != nil {
				v.mutex.Lock()
				v.errorMessage = fmt.Sprintf("Failed to load remote directory: %v", err)
				v.connecting = false
				v.mutex.Unlock()
				return
			}

			v.mutex.Lock()
			v.connected = true
			v.connecting = false
			v.mutex.Unlock()
		}()
	}

	return v
}

// updateLocalPanel odświeża zawartość lokalnego panelu
func (v *transferView) updateLocalPanel() error {
	entries, err := v.readLocalDirectory(v.localPanel.path)
	if err != nil {
		return err
	}
	v.localPanel.entries = entries
	return nil
}

func (v *transferView) readLocalDirectory(path string) ([]FileEntry, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	// Zawsze zaczynamy od ".." do nawigacji w górę
	entries := []FileEntry{{
		name:    "..",
		isDir:   true,
		modTime: time.Now(),
	}}

	for _, fi := range fileInfos {
		// Pomijamy ukryte pliki zaczynające się od "." (opcjonalnie)
		if !strings.HasPrefix(fi.Name(), ".") || fi.Name() == ".." {
			entries = append(entries, FileEntry{
				name:    fi.Name(),
				size:    fi.Size(),
				modTime: fi.ModTime(),
				isDir:   fi.IsDir(),
			})
		}
	}

	// Sortowanie: najpierw katalogi, potem pliki, alfabetycznie
	sort.Slice(entries[1:], func(i, j int) bool {
		// Przesuwamy indeksy o 1, bo pomijamy ".."
		i, j = i+1, j+1
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	return entries, nil
}

func (v *transferView) Init() tea.Cmd {
	return nil
}

func (v *transferView) updateRemotePanel() error {
	if err := v.ensureConnected(); err != nil {
		return err
	}

	entries, err := v.readRemoteDirectory(v.remotePanel.path)
	if err != nil {
		v.setConnected(false) // Oznacz jako rozłączony w przypadku błędu
		return err
	}
	v.remotePanel.entries = entries
	return nil
}

// readRemoteDirectory czyta zawartość zdalnego katalogu
func (v *transferView) readRemoteDirectory(path string) ([]FileEntry, error) {
	if err := v.ensureConnected(); err != nil {
		return nil, err
	}

	transfer := v.model.GetTransfer()
	fileInfos, err := transfer.ListRemoteFiles(path)
	if err != nil {
		v.setConnected(false)
		return nil, fmt.Errorf("failed to list remote directory: %v", err)
	}

	// Zawsze zaczynamy od ".." do nawigacji w górę
	entries := []FileEntry{{
		name:    "..",
		isDir:   true,
		modTime: time.Now(),
	}}

	for _, fi := range fileInfos {
		if !strings.HasPrefix(fi.Name(), ".") || fi.Name() == ".." {
			entries = append(entries, FileEntry{
				name:    fi.Name(),
				size:    fi.Size(),
				modTime: fi.ModTime(),
				isDir:   fi.IsDir(),
			})
		}
	}

	// Sortowanie: najpierw katalogi, potem pliki, alfabetycznie
	sort.Slice(entries[1:], func(i, j int) bool {
		i, j = i+1, j+1
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})

	return entries, nil
}

// getActivePanel zwraca aktywny panel
func (v *transferView) getActivePanel() *Panel {
	if v.localPanel.active {
		return &v.localPanel
	}
	return &v.remotePanel
}

// getInactivePanel zwraca nieaktywny panel
func (v *transferView) getInactivePanel() *Panel {
	if v.localPanel.active {
		return &v.remotePanel
	}
	return &v.localPanel
}

// switchActivePanel przełącza aktywny panel
func (v *transferView) switchActivePanel() {
	v.localPanel.active = !v.localPanel.active
	v.remotePanel.active = !v.remotePanel.active
}

func (v *transferView) renderPanel(p *Panel) string {
	var content strings.Builder

	// Zastosuj styl panelu z ramką
	var panelContent strings.Builder

	// Formatowanie i skracanie ścieżki
	pathText := formatPath(p.path, 40)

	// Użycie stylów ścieżki
	pathStyle := inactivePathStyle
	if p.active {
		pathStyle = activePathStyle
	}
	panelContent.WriteString(pathStyle.Render(pathText))
	panelContent.WriteString("\n")

	// Nagłówki kolumn
	panelContent.WriteString(ui.HeaderStyle.Render(
		fmt.Sprintf("%-30s %10s %20s", "Name", "Size", "Modified"),
	))
	panelContent.WriteString("\n")

	// Sprawdź czy entries nie jest nil i czy ma elementy
	if len(p.entries) > 0 {
		// Lista plików
		filesList := renderFileList(
			p.entries[p.scrollOffset:min(p.scrollOffset+maxVisibleItems, len(p.entries))],
			p.selectedIndex-p.scrollOffset,
			p.active,
			60, // szerokość listy plików
		)
		panelContent.WriteString(filesList)

		// Informacja o przewijaniu
		if len(p.entries) > maxVisibleItems {
			panelContent.WriteString(fmt.Sprintf(" Showing %d-%d of %d items",
				p.scrollOffset+1,
				min(p.scrollOffset+maxVisibleItems, len(p.entries)),
				len(p.entries)))
		}
	} else {
		// Dodaj informację gdy katalog jest pusty
		panelContent.WriteString("\n Directory is empty")
	}

	// Zastosuj styl całego panelu
	content.WriteString(panelStyle.
		BorderForeground(ui.Subtle).
		Render(panelContent.String()))

	return content.String()
}

// Pomocnicza funkcja min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatSize formatuje rozmiar pliku
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(size)/float64(div), "KMGTPE"[exp])
}

// navigatePanel obsługuje nawigację w panelu
func (v *transferView) navigatePanel(p *Panel, direction int) {
	if len(p.entries) == 0 {
		p.selectedIndex = 0
		p.scrollOffset = 0
		return
	}

	newIndex := p.selectedIndex + direction

	if newIndex < 0 {
		newIndex = len(p.entries) - 1
	} else if newIndex >= len(p.entries) {
		newIndex = 0
	}

	p.selectedIndex = newIndex

	// Dostosuj przewijanie
	if p.selectedIndex < p.scrollOffset {
		p.scrollOffset = p.selectedIndex
	} else if p.selectedIndex >= p.scrollOffset+maxVisibleItems {
		p.scrollOffset = p.selectedIndex - maxVisibleItems + 1
	}

	// Upewnij się, że scrollOffset nie jest ujemny
	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}
}

// enterDirectory wchodzi do wybranego katalogu
func (v *transferView) enterDirectory(p *Panel) error {
	if len(p.entries) == 0 || p.selectedIndex >= len(p.entries) {
		return nil
	}

	entry := p.entries[p.selectedIndex]
	if !entry.isDir {
		return nil
	}

	var newPath string
	if entry.name == ".." {
		// Nawigacja do góry
		newPath = filepath.Dir(p.path)
		// Dla Windows możemy potrzebować dodatkowej obsługi ścieżki głównej
		if runtime.GOOS == "windows" && filepath.Dir(newPath) == newPath {
			newPath = filepath.VolumeName(newPath) + "\\"
		}
	} else {
		newPath = filepath.Join(p.path, entry.name)
	}

	// Zapisz poprzednią ścieżkę
	oldPath := p.path
	p.path = newPath

	// Spróbuj odświeżyć zawartość
	var err error
	if p == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	// W przypadku błędu, przywróć poprzednią ścieżkę
	if err != nil {
		p.path = oldPath
		return err
	}

	// Resetuj wybór i przewijanie
	p.selectedIndex = 0
	p.scrollOffset = 0
	return nil
}

func (v *transferView) copyFile() error {
	srcPanel := v.getActivePanel()
	dstPanel := v.getInactivePanel()

	if len(srcPanel.entries) == 0 || srcPanel.selectedIndex >= len(srcPanel.entries) {
		return fmt.Errorf("no file selected")
	}

	entry := srcPanel.entries[srcPanel.selectedIndex]
	if entry.isDir {
		return fmt.Errorf("directory copying not supported yet")
	}

	// Przygotuj ścieżki
	srcPath := filepath.Join(srcPanel.path, entry.name)
	dstPath := filepath.Join(dstPanel.path, entry.name)

	v.mutex.Lock()
	v.transferring = true
	v.statusMessage = fmt.Sprintf("Copying %s...", entry.name)
	v.mutex.Unlock()

	// Utwórz kanały
	progressChan := make(chan ssh.TransferProgress, 100) // Buforowany kanał
	doneChan := make(chan error, 1)
	updateChan := make(chan tea.Msg)

	// Goroutine do transferu
	go func() {
		var err error
		if srcPanel == &v.localPanel {
			err = v.model.GetTransfer().UploadFile(srcPath, dstPath, progressChan)
		} else {
			err = v.model.GetTransfer().DownloadFile(srcPath, dstPath, progressChan)
		}
		doneChan <- err
		close(progressChan)
	}()

	// Goroutine do aktualizacji UI
	go func() {
		for {
			select {
			case progress, ok := <-progressChan:
				if !ok {
					return
				}
				v.mutex.Lock()
				v.progress = progress
				v.mutex.Unlock()
				updateChan <- ssh.TransferProgress(progress)
			case err := <-doneChan:
				v.mutex.Lock()
				v.transferring = false
				if err != nil {
					v.errorMessage = fmt.Sprintf("Transfer error: %v", err)
				} else {
					v.statusMessage = "Transfer completed successfully"
					// Odśwież panel docelowy
					if dstPanel == &v.localPanel {
						v.updateLocalPanel()
					} else {
						v.updateRemotePanel()
					}
				}
				v.mutex.Unlock()
				return
			}
		}
	}()

	return nil
}

// deleteFile usuwa wybrany plik
func (v *transferView) deleteFile() error {
	panel := v.getActivePanel()
	if len(panel.entries) == 0 || panel.selectedIndex >= len(panel.entries) {
		return fmt.Errorf("no file selected")
	}

	entry := panel.entries[panel.selectedIndex]
	if entry.name == ".." {
		return fmt.Errorf("cannot delete parent directory reference")
	}

	// Potwierdź usunięcie
	v.statusMessage = fmt.Sprintf("Delete %s? (y/n)", entry.name)

	return nil
}

// executeDelete wykonuje faktyczne usuwanie pliku
func (v *transferView) executeDelete() error {
	panel := v.getActivePanel()
	entry := panel.entries[panel.selectedIndex]
	path := filepath.Join(panel.path, entry.name)

	var err error
	if panel == &v.localPanel {
		if entry.isDir {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
	} else {
		transfer := v.model.GetTransfer()
		err = transfer.RemoveRemoteFile(path)
	}

	if err != nil {
		return fmt.Errorf("failed to delete %s: %v", entry.name, err)
	}

	// Odśwież panel po usunięciu
	if panel == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	if err != nil {
		return fmt.Errorf("failed to refresh panel: %v", err)
	}

	v.statusMessage = fmt.Sprintf("Deleted %s", entry.name)
	return nil
}

// createDirectory tworzy nowy katalog
func (v *transferView) createDirectory(name string) error {
	if name == "" {
		return fmt.Errorf("directory name cannot be empty")
	}

	panel := v.getActivePanel()
	path := filepath.Join(panel.path, name)

	var err error
	if panel == &v.localPanel {
		err = os.MkdirAll(path, 0755)
	} else {
		transfer := v.model.GetTransfer()
		err = transfer.CreateRemoteDirectory(path)
	}

	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Odśwież panel
	if panel == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	if err != nil {
		return fmt.Errorf("failed to refresh panel: %v", err)
	}

	v.statusMessage = fmt.Sprintf("Created directory %s", name)
	return nil
}

// renameFile zmienia nazwę pliku
func (v *transferView) renameFile(newName string) error {
	if newName == "" {
		return fmt.Errorf("new name cannot be empty")
	}

	panel := v.getActivePanel()
	if len(panel.entries) == 0 || panel.selectedIndex >= len(panel.entries) {
		return fmt.Errorf("no file selected")
	}

	entry := panel.entries[panel.selectedIndex]
	if entry.name == ".." {
		return fmt.Errorf("cannot rename parent directory reference")
	}

	oldPath := filepath.Join(panel.path, entry.name)
	newPath := filepath.Join(panel.path, newName)

	var err error
	if panel == &v.localPanel {
		err = os.Rename(oldPath, newPath)
	} else {
		transfer := v.model.GetTransfer()
		err = transfer.RenameRemoteFile(oldPath, newPath)
	}

	if err != nil {
		return fmt.Errorf("failed to rename file: %v", err)
	}

	// Odśwież panel
	if panel == &v.localPanel {
		err = v.updateLocalPanel()
	} else {
		err = v.updateRemotePanel()
	}

	if err != nil {
		return fmt.Errorf("failed to refresh panel: %v", err)
	}

	v.statusMessage = fmt.Sprintf("Renamed %s to %s", entry.name, newName)
	return nil
}

// handleError obsługuje błędy i wyświetla komunikat
func (v *transferView) handleError(err error) {
	if err != nil {
		v.errorMessage = err.Error()
	}
}

// internal/ui/views/transfer.go - Part 4

// Update obsługuje zdarzenia i aktualizuje stan
func (v *transferView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.isWaitingForInput() {
			if msg.Type == tea.KeyEnter {
				if err := v.handleCommand(v.input.Value()); err != nil {
					v.handleError(err)
				}
				v.input.Reset()
				return v, nil
			}
			var cmd tea.Cmd
			v.input, cmd = v.input.Update(msg)
			return v, cmd
		}
		switch msg.String() {
		case "q", "esc":
			if v.transferring {
				return v, nil // Zablokuj wyjście podczas transferu
			}
			// Zamknij połączenie SFTP przed wyjściem
			if v.connected {
				transfer := v.model.GetTransfer()
				if transfer != nil {
					transfer.Disconnect()
				}
			}
			v.model.SetActiveView(ui.ViewMain)
			return v, nil

		case "tab":
			if v.connected {
				v.switchActivePanel()
				v.errorMessage = ""
			}
			return v, nil

		case "up", "k":
			panel := v.getActivePanel()
			v.navigatePanel(panel, -1)
			v.errorMessage = ""
			return v, nil

		case "down", "j":
			panel := v.getActivePanel()
			v.navigatePanel(panel, 1)
			v.errorMessage = ""
			return v, nil

		case "enter":
			panel := v.getActivePanel()
			if err := v.enterDirectory(panel); err != nil {
				v.handleError(err)
			}
			return v, nil

		case "f5", "c":
			if !v.transferring {
				if err := v.copyFile(); err != nil {
					v.handleError(err)
				}
			}
			return v, nil

		case "f8", "d":
			if !v.transferring {
				if err := v.deleteFile(); err != nil {
					v.handleError(err)
				}
			}
			return v, nil

		case "f7", "m":
			if !v.transferring {
				v.statusMessage = "Enter directory name:"
				// Obsługa wprowadzania nazwy będzie w następnym Update
			}
			return v, nil

		case "f6", "r":
			if !v.transferring {
				newName := "New Name" // To powinno być pobierane z inputu
				if err := v.renameFile(newName); err != nil {
					v.handleError(err)
				}
			}
			return v, nil

		case "y":
			// Potwierdzenie usunięcia
			if strings.HasPrefix(v.statusMessage, "Delete ") {
				if err := v.executeDelete(); err != nil {
					v.handleError(err)
				}
				v.statusMessage = ""
			}
			return v, nil

		case "n":
			// Anulowanie usunięcia
			if strings.HasPrefix(v.statusMessage, "Delete ") {
				v.statusMessage = "Delete cancelled"
			}
			return v, nil

		case "f1":
			v.showHelp = !v.showHelp
			return v, nil

		case "ctrl+r":
			// Odśwież oba panele
			if err := v.updateLocalPanel(); err != nil {
				v.handleError(err)
			}
			if v.connected {
				if err := v.updateRemotePanel(); err != nil {
					v.handleError(err)
				}
			}
			return v, nil
		case "ctrl+n":
			if !v.transferring {
				name := "New Directory" // To powinno być pobierane z inputu
				if err := v.createDirectory(name); err != nil {
					v.handleError(err)
				}
			}
			return v, nil
		}

	case ssh.TransferProgress:
		// Aktualizacja postępu transferu
		v.progress = msg
		return v, nil
	}

	return v, nil
}

// handleCommand obsługuje wprowadzanie komend
func (v *transferView) handleCommand(cmd string) error {
	if strings.HasPrefix(v.statusMessage, "Enter directory name:") {
		return v.createDirectory(cmd)
	}
	if strings.HasPrefix(v.statusMessage, "Enter new name:") {
		return v.renameFile(cmd)
	}
	return fmt.Errorf("unknown command")
}

// formatProgressBar tworzy pasek postępu
func (v *transferView) formatProgressBar(width int) string {
	if !v.transferring || v.progress.TotalBytes == 0 {
		return ""
	}

	percentage := float64(v.progress.TransferredBytes) / float64(v.progress.TotalBytes)
	barWidth := width - 10 // Zostaw miejsce na procenty
	completedWidth := int(float64(barWidth) * percentage)

	bar := fmt.Sprintf("[%s%s] %3.0f%%",
		strings.Repeat("=", completedWidth),
		strings.Repeat(" ", barWidth-completedWidth),
		percentage*100)

	speed := float64(v.progress.TransferredBytes) / time.Since(v.progress.StartTime).Seconds()
	return fmt.Sprintf("%s %s %s/s",
		v.progress.FileName,
		bar,
		formatSize(int64(speed)))
}

// shouldShowDeleteConfirm sprawdza czy wyświetlić potwierdzenie usunięcia
func (v *transferView) shouldShowDeleteConfirm() bool {
	return strings.HasPrefix(v.statusMessage, "Delete ")
}

// isWaitingForInput sprawdza czy oczekuje na wprowadzenie tekstu
func (v *transferView) isWaitingForInput() bool {
	return strings.HasPrefix(v.statusMessage, "Enter ")
}

var helpText = `
 File Transfer Help
 -----------------
 Tab       - Switch panel
 Enter     - Enter directory
 F5/c      - Copy file
 F6/r      - Rename
 F7/m      - Create directory
 F8/d      - Delete
 F1        - Toggle help
 Ctrl+r    - Refresh
 Esc/q     - Exit
 
 Navigation
 ----------
 Up/k      - Move up
 Down/j    - Move down
 `

func (v *transferView) View() string {
	var content strings.Builder

	// Nagłówek
	content.WriteString(ui.TitleStyle.Render("File Transfer"))
	if v.connected {
		if host := v.model.GetSelectedHost(); host != nil {
			content.WriteString(ui.SuccessStyle.Render(
				fmt.Sprintf(" - Connected to %s (%s)", host.Name, host.IP),
			))
		}
	} else if host := v.model.GetSelectedHost(); host != nil {
		content.WriteString(ui.ErrorStyle.Render(
			fmt.Sprintf(" - Not connected to %s (%s)", host.Name, host.IP),
		))
	}
	content.WriteString("\n\n")

	// Obsługa stanu łączenia
	if v.connecting {
		content.WriteString(ui.DescriptionStyle.Render("Establishing SFTP connection..."))
		return ui.WindowStyle.Render(content.String())
	}

	// Pomoc (jeśli włączona)
	if v.showHelp {
		content.WriteString(ui.DescriptionStyle.Render(helpText))
		return ui.WindowStyle.Render(content.String())
	}
	// Oblicz szerokość paneli
	totalWidth := 120                  // Zwiększamy całkowitą szerokość
	panelWidth := (totalWidth - 3) / 2 // 3 to szerokość separatora

	// Renderuj panele w jednej linii
	leftPanel := v.renderPanel(&v.localPanel)
	rightPanel := ""

	if !v.connected {
		rightPanel = ui.ErrorStyle.Render("\n  No SFTP Connection\n  Press 'q' to return and connect to a host first.")
	} else {
		rightPanel = v.renderPanel(&v.remotePanel)
	}

	// Użyj strings.Split aby podzielić panele na linie
	leftLines := strings.Split(leftPanel, "\n")
	rightLines := strings.Split(rightPanel, "\n")

	// Wyrównaj liczbę linii w obu panelach
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	for i := len(leftLines); i < maxLines; i++ {
		leftLines = append(leftLines, strings.Repeat(" ", panelWidth))
	}
	for i := len(rightLines); i < maxLines; i++ {
		rightLines = append(rightLines, strings.Repeat(" ", panelWidth))
	}

	// Połącz linie paneli ze sobą
	for i := 0; i < maxLines; i++ {
		content.WriteString(leftLines[i])
		content.WriteString("   ") // Separator
		content.WriteString(rightLines[i])
		content.WriteString("\n")
	}

	// Pasek postępu (jeśli trwa transfer)
	if v.transferring {
		content.WriteString("\n")
		progressBar := v.formatProgressBar(totalWidth)
		content.WriteString(ui.DescriptionStyle.Render(progressBar))
	}

	// Status i komunikaty
	footerContent := strings.Builder{}

	// Komunikat o błędzie
	if v.errorMessage != "" {
		footerContent.WriteString(ui.ErrorStyle.Render("Error: " + v.errorMessage))
		footerContent.WriteString("\n")
	}

	// Status
	if v.statusMessage != "" {
		style := ui.DescriptionStyle
		if v.shouldShowDeleteConfirm() {
			style = ui.ErrorStyle
		} else if v.isWaitingForInput() {
			style = ui.InputStyle
		}
		footerContent.WriteString(style.Render(v.statusMessage))
		footerContent.WriteString("\n")
	}

	// Komunikat o braku połączenia (jeśli nie połączono)
	if !v.connected && v.errorMessage == "" {
		footerContent.WriteString(ui.ErrorStyle.Render("SFTP connection not established. Press 'q' to return to main menu and connect first."))
		footerContent.WriteString("\n")
	}

	// Skróty klawiszowe (pokazuj tylko aktywne w zależności od stanu połączenia)
	if v.connected {
		footerContent.WriteString(v.renderShortcuts())
	} else {
		footerContent.WriteString(ui.ButtonStyle.Render("q") + " - Return to main menu")
	}

	// Dodaj stopkę do głównej zawartości
	content.WriteString("\n")
	content.WriteString(footerContent.String())

	// Renderuj całość w oknie
	return ui.WindowStyle.Render(content.String())
}

// renderShortcuts renderuje pasek skrótów
func (v *transferView) renderShortcuts() string {
	shortcuts := []struct {
		key         string
		description string
		disabled    bool
	}{
		{"Tab", "Switch panel", !v.connected},
		{"F5/c", "Copy", v.transferring},
		{"F6/r", "Rename", v.transferring},
		{"F7/m", "MkDir", v.transferring},
		{"F8/d", "Delete", v.transferring},
		{"F1", "Help", false},
		{"Esc", "Exit", false},
	}

	var result strings.Builder
	for i, sc := range shortcuts {
		if i > 0 {
			result.WriteString(" ")
		}

		keyStyle := ui.ButtonStyle
		descStyle := ui.DescriptionStyle
		if sc.disabled {
			keyStyle = keyStyle.Foreground(ui.Subtle)
			descStyle = descStyle.Foreground(ui.Subtle)
		}

		result.WriteString(keyStyle.Render(sc.key))
		result.WriteString(descStyle.Render(fmt.Sprintf(":%s", sc.description)))
	}

	return result.String()
}

// Pomocnicze stałe dla kolorów i stylów
var (
	panelBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
	}

	panelStyle = lipgloss.NewStyle().
			Border(panelBorder).
			BorderForeground(ui.Subtle).
			Padding(0, 1).
			Height(20) // Dodaj stałą wysokość

	activePathStyle = lipgloss.NewStyle().
			Bold(true).
			Background(ui.Highlight).
			Foreground(lipgloss.Color("0"))

	inactivePathStyle = lipgloss.NewStyle().
				Foreground(ui.Subtle)
)

// formatPath formatuje ścieżkę do wyświetlenia
func formatPath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}

	// Dodaj "..." na początku jeśli ścieżka jest za długa
	return "..." + path[len(path)-(maxWidth-3):]
}

// renderFileList renderuje listę plików z odpowiednim formatowaniem
func renderFileList(entries []FileEntry, selected int, active bool, width int) string {
	var content strings.Builder

	// Zabezpieczenie przed pustą listą
	if len(entries) == 0 {
		return ""
	}

	for i, entry := range entries {
		// Sprawdź czy selected jest w prawidłowym zakresie
		isSelected := i == selected && selected >= 0 && selected < len(entries)

		// Formatowanie nazwy pliku
		name := entry.name
		if entry.isDir {
			name = "[" + name + "]"
		}

		// Skróć nazwę jeśli jest za długa
		maxNameWidth := width - 35 // miejsce na rozmiar i datę
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-3] + "..."
		}

		// Formatowanie linii
		line := fmt.Sprintf("%-*s %10s %19s",
			maxNameWidth,
			name,
			formatSize(entry.size),
			entry.modTime.Format("2006-01-02 15:04"))

		// Styl linii
		style := lipgloss.NewStyle()
		if isSelected {
			if active {
				style = style.Bold(true).Background(ui.Highlight).Foreground(lipgloss.Color("0"))
			} else {
				style = style.Underline(true)
			}
		}

		content.WriteString(style.Render(line))
		content.WriteString("\n")
	}

	return content.String()
}

func (v *transferView) ensureConnected() error {
	if !v.connected {
		transfer := v.model.GetTransfer()
		if transfer == nil {
			return fmt.Errorf("no transfer client available")
		}

		host := v.model.GetSelectedHost()
		if host == nil {
			return fmt.Errorf("no host selected")
		}

		// Pobierz i odszyfruj hasło
		passwords := v.model.GetPasswords()
		if host.PasswordID >= len(passwords) {
			return fmt.Errorf("invalid password ID")
		}
		password := passwords[host.PasswordID]
		decryptedPass, err := password.GetDecrypted(v.model.GetCipher())
		if err != nil {
			return fmt.Errorf("failed to decrypt password: %v", err)
		}

		// Nawiąż połączenie SFTP
		if err := transfer.Connect(host, decryptedPass); err != nil {
			return fmt.Errorf("failed to establish SFTP connection: %v", err)
		}

		v.setConnected(true)
	}
	return nil
}

func (v *transferView) setConnected(connected bool) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.connected = connected
}
