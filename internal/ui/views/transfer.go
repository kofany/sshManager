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

	"github.com/charmbracelet/bubbles/table"
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
	mode    os.FileMode // Dodane pole

}

// Panel reprezentuje panel plików (lokalny lub zdalny)
type Panel struct {
	path          string
	entries       []FileEntry
	selectedIndex int
	scrollOffset  int
	active        bool
}

type transferProgressMsg ssh.TransferProgress

type transferFinishedMsg struct {
	err error
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
	mutex         sync.Mutex
	width         int         // Dodane
	height        int         // Dodane
	escPressed    bool        // flaga wskazująca czy ESC został wciśnięty
	escTimeout    *time.Timer // timer do resetowania stanu ESC
}
type connectionStatusMsg struct {
	connected bool
	err       error
}

func NewTransferView(model *ui.Model) *transferView {
	input := textinput.New()
	input.Placeholder = "Enter command..."
	input.CharLimit = 255

	v := &transferView{
		model: model,
		localPanel: Panel{
			path:   getHomeDir(),
			active: true,
			entries: []FileEntry{
				{name: "..", isDir: true},
			},
		},
		remotePanel: Panel{
			path:   "~/", // Tymczasowa wartość
			active: false,
			entries: []FileEntry{
				{name: "..", isDir: true},
			},
		},
		input:  input,
		width:  model.GetTerminalWidth(),
		height: model.GetTerminalHeight(),
	}

	// Inicjalizujemy panel lokalny
	if err := v.updateLocalPanel(); err != nil {
		v.errorMessage = fmt.Sprintf("Failed to load local directory: %v", err)
		return v
	}

	// Inicjujemy połączenie SFTP w tle
	if v.model.GetSelectedHost() != nil {
		go func() {
			// Attempt to establish connection
			err := v.ensureConnected()
			if err != nil {
				v.model.Program.Send(connectionStatusMsg{
					connected: false,
					err:       err,
				})
				return
			}

			// Pobierz katalog domowy i zaktualizuj ścieżkę
			transfer := v.model.GetTransfer()
			if homeDir, err := transfer.GetRemoteHomeDir(); err == nil {
				v.remotePanel.path = homeDir
			}

			// Update remote panel
			err = v.updateRemotePanel()
			if err != nil {
				v.model.Program.Send(connectionStatusMsg{
					connected: false,
					err:       err,
				})
				return
			}

			// Send success message
			v.model.Program.Send(connectionStatusMsg{
				connected: true,
				err:       nil,
			})
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
				mode:    fi.Mode(), // Dodane

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
	if !v.connected && !v.connecting && v.model.GetSelectedHost() != nil {
		v.connecting = true
		return v.sendConnectionUpdate() // Usuń argument program
	}
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
				mode:    fi.Mode(), // Dodane
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

	// Oblicz szerokość panelu na podstawie szerokości ekranu
	panelWidth := (min(v.width-40, 160) - 3) / 2 // Dostosowujemy do nowej całkowitej szerokości

	// Zastosuj styl panelu z ramką
	var panelContent strings.Builder

	// Formatowanie i skracanie ścieżki
	pathText := formatPath(p.path, min(40, panelWidth-5))

	// Użycie stylów ścieżki
	pathStyle := inactivePathStyle
	if p.active {
		pathStyle = activePathStyle
	}
	panelContent.WriteString(pathStyle.Render(pathText))
	panelContent.WriteString("\n")

	// Dostosuj szerokości kolumn do dostępnej przestrzeni
	nameWidth := min(30, panelWidth-35) // Zostaw miejsce na size i modified
	sizeWidth := 10
	modifiedWidth := 20

	// Nagłówki kolumn
	panelContent.WriteString(ui.HeaderStyle.Render(
		fmt.Sprintf("%-*s %*s %*s",
			nameWidth, "Name",
			sizeWidth, "Size",
			modifiedWidth, "Modified"),
	))
	panelContent.WriteString("\n")

	// Sprawdź czy entries nie jest nil i czy ma elementy
	if len(p.entries) > 0 {
		// Lista plików
		filesList := v.renderFileList(
			p.entries[p.scrollOffset:min(p.scrollOffset+maxVisibleItems, len(p.entries))],
			p.selectedIndex-p.scrollOffset,
			p.active,
			panelWidth-2, // szerokość listy plików z uwzględnieniem marginesów
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
		Width(panelWidth).
		BorderForeground(ui.Subtle).
		Render(panelContent.String()))

	return content.String()
}

func (v *transferView) View() string {
	var content strings.Builder

	// Tytuł i status połączenia
	titleContent := ui.TitleStyle.Render("File Transfer")
	if v.connected {
		if host := v.model.GetSelectedHost(); host != nil {
			titleContent += ui.SuccessStyle.Render(
				fmt.Sprintf(" - Connected to %s (%s)", host.Name, host.IP),
			)
		}
	} else if host := v.model.GetSelectedHost(); host != nil {
		if v.connecting {
			titleContent += ui.DescriptionStyle.Render(" - Establishing connection...")
		} else {
			titleContent += ui.ErrorStyle.Render(
				fmt.Sprintf(" - Not connected to %s (%s)", host.Name, host.IP),
			)
		}
	}
	content.WriteString(titleContent + "\n\n")

	// Obsługa stanu łączenia
	if v.connecting {
		connectingContent := ui.DescriptionStyle.Render("Establishing SFTP connection...")
		return lipgloss.Place(
			v.width,
			v.height,
			lipgloss.Center,
			lipgloss.Center,
			ui.WindowStyle.Render(connectingContent),
		)
	}

	// Obsługa widoku pomocy
	if v.showHelp {
		helpContent := ui.DescriptionStyle.Render(helpText)
		return lipgloss.Place(
			v.width,
			v.height,
			lipgloss.Center,
			lipgloss.Center,
			ui.WindowStyle.Render(helpContent),
		)
	}

	// Oblicz szerokość paneli na podstawie szerokości ekranu
	totalWidth := min(v.width-40, 160) // Zmniejszamy szerokość o marginesy (20 z każdej strony)
	panelWidth := (totalWidth - 3) / 2 // 3 to szerokość separatora

	// Renderuj panele
	leftPanel := v.renderPanel(&v.localPanel)
	rightPanel := ""

	if !v.connected {
		rightPanel = ui.ErrorStyle.Render("\n  No SFTP Connection\n  Press 'q' to return and connect to a host first.")
	} else {
		rightPanel = v.renderPanel(&v.remotePanel)
	}

	// Wyrównaj panele
	leftLines := strings.Split(leftPanel, "\n")
	rightLines := strings.Split(rightPanel, "\n")

	maxLines := max(len(leftLines), len(rightLines))

	// Wyrównaj liczbe linii w panelach
	for i := len(leftLines); i < maxLines; i++ {
		leftLines = append(leftLines, strings.Repeat(" ", panelWidth))
	}
	for i := len(rightLines); i < maxLines; i++ {
		rightLines = append(rightLines, strings.Repeat(" ", panelWidth))
	}

	// Połącz panele
	for i := 0; i < maxLines; i++ {
		content.WriteString(leftLines[i])
		content.WriteString(" │ ")
		content.WriteString(rightLines[i])
		content.WriteString("\n")
	}

	// Pasek postępu
	if v.transferring {
		content.WriteString("\n")
		progressBar := v.formatProgressBar(totalWidth)
		content.WriteString(ui.DescriptionStyle.Render(progressBar))
	}
	if v.isWaitingForInput() {
		content.WriteString("\n" + v.input.View())
	}
	footer := v.renderFooter()
	content.WriteString("\n")
	content.WriteString(footer)

	// Renderuj całość w oknie i wycentruj
	finalContent := ui.WindowStyle.Render(content.String())

	return lipgloss.Place(
		v.width,
		v.height,
		lipgloss.Center,
		lipgloss.Center,
		finalContent,
		lipgloss.WithWhitespaceChars(""),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

// Pomocnicza funkcja do określania maksimum
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func (v *transferView) hasSelectedItems() bool {
	for _, isSelected := range v.getSelectedItems() {
		if isSelected {
			return true
		}
	}
	return false
}

func (v *transferView) getSelectedItems() map[string]bool {
	selected := make(map[string]bool)
	paths := v.model.GetSelectedPaths() // zakładając, że taka metoda istnieje w Model
	for _, path := range paths {
		selected[path] = true
	}
	return selected
}

func (v *transferView) copyFile() tea.Cmd {
	srcPanel := v.getActivePanel()
	dstPanel := v.getInactivePanel()

	// Zbierz wszystkie zaznaczone pliki i foldery
	var itemsToCopy []struct {
		srcPath string
		dstPath string
		isDir   bool
	}

	// Najpierw sprawdź aktualnie wybrany element, jeśli nie ma zaznaczonych
	if !v.hasSelectedItems() {
		if len(srcPanel.entries) == 0 || srcPanel.selectedIndex >= len(srcPanel.entries) {
			v.handleError(fmt.Errorf("no file selected"))
			return nil
		}
		entry := srcPanel.entries[srcPanel.selectedIndex]
		srcPath := filepath.Join(srcPanel.path, entry.name)
		dstPath := filepath.Join(dstPanel.path, entry.name)
		itemsToCopy = append(itemsToCopy, struct {
			srcPath string
			dstPath string
			isDir   bool
		}{srcPath, dstPath, entry.isDir})
	} else {
		// Dodaj wszystkie zaznaczone elementy
		for path, isSelected := range v.getSelectedItems() {
			if isSelected {
				baseName := filepath.Base(path)
				dstPath := filepath.Join(dstPanel.path, baseName)
				// Sprawdź czy to folder czy plik
				info, err := os.Stat(path)
				if err != nil {
					v.handleError(fmt.Errorf("cannot access %s: %v", path, err))
					continue
				}
				itemsToCopy = append(itemsToCopy, struct {
					srcPath string
					dstPath string
					isDir   bool
				}{path, dstPath, info.IsDir()})
			}
		}
	}

	if len(itemsToCopy) == 0 {
		v.handleError(fmt.Errorf("no items to copy"))
		return nil
	}

	v.mutex.Lock()
	v.transferring = true
	v.statusMessage = "Copying files..."
	v.mutex.Unlock()

	transfer := v.model.GetTransfer()

	// Zwróć komendę rozpoczynającą transfer
	return func() tea.Msg {
		progressChan := make(chan ssh.TransferProgress)
		doneChan := make(chan error, 1)

		// Uruchom transfer w goroutine
		go func() {
			var totalErr error
			for _, item := range itemsToCopy {
				var err error
				if item.isDir {
					if srcPanel == &v.localPanel {
						err = v.copyDirectoryToRemote(item.srcPath, item.dstPath, transfer, progressChan)
					} else {
						err = v.copyDirectoryFromRemote(item.srcPath, item.dstPath, transfer, progressChan)
					}
				} else {
					if srcPanel == &v.localPanel {
						err = transfer.UploadFile(item.srcPath, item.dstPath, progressChan)
					} else {
						err = transfer.DownloadFile(item.srcPath, item.dstPath, progressChan)
					}
				}
				if err != nil {
					totalErr = fmt.Errorf("error copying %s: %v", item.srcPath, err)
					break
				}
			}
			doneChan <- totalErr
			close(progressChan)
		}()

		// Goroutine do czytania postępu i wysyłania wiadomości
		go func() {
			for progress := range progressChan {
				v.model.Program.Send(transferProgressMsg(progress))
			}
			err := <-doneChan
			v.model.Program.Send(transferFinishedMsg{err: err})
			// Wyczyść zaznaczenie po zakończeniu
			v.model.ClearSelection()
		}()

		return nil
	}
}

// Dodaj nowe funkcje do obsługi kopiowania folderów
func (v *transferView) copyDirectoryToRemote(localPath, remotePath string, transfer *ssh.FileTransfer, progressChan chan<- ssh.TransferProgress) error {
	// Utwórz katalog na zdalnym serwerze
	if err := transfer.CreateRemoteDirectory(remotePath); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	// Przejdź przez wszystkie pliki w lokalnym katalogu
	return filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Oblicz względną ścieżkę
		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		// Utwórz pełną ścieżkę zdalną
		remotePathFull := filepath.Join(remotePath, relPath)

		if info.IsDir() {
			// Utwórz katalog na zdalnym serwerze
			return transfer.CreateRemoteDirectory(remotePathFull)
		} else {
			// Prześlij plik
			return transfer.UploadFile(path, remotePathFull, progressChan)
		}
	})
}

func (v *transferView) copyDirectoryFromRemote(remotePath, localPath string, transfer *ssh.FileTransfer, progressChan chan<- ssh.TransferProgress) error {
	// Utwórz lokalny katalog
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Pobierz listę plików z katalogu zdalnego
	entries, err := transfer.ListRemoteFiles(remotePath)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	// Rekurencyjnie kopiuj zawartość
	for _, entry := range entries {
		remoteSrcPath := filepath.Join(remotePath, entry.Name())
		localDstPath := filepath.Join(localPath, entry.Name())

		if entry.IsDir() {
			if err := v.copyDirectoryFromRemote(remoteSrcPath, localDstPath, transfer, progressChan); err != nil {
				return err
			}
		} else {
			if err := transfer.DownloadFile(remoteSrcPath, localDstPath, progressChan); err != nil {
				return err
			}
		}
	}

	return nil
}

// deleteFile usuwa wybrany plik
// deleteFile usuwa wybrany plik lub katalog
func (v *transferView) deleteFile() error {
	panel := v.getActivePanel()
	if len(panel.entries) == 0 || panel.selectedIndex >= len(panel.entries) {
		return fmt.Errorf("no file selected")
	}

	entry := panel.entries[panel.selectedIndex]
	if entry.name == ".." {
		return fmt.Errorf("cannot delete parent directory reference")
	}

	// Dostosuj komunikat w zależności od typu
	itemType := "file"
	if entry.isDir {
		itemType = "directory"
	}

	// Potwierdź usunięcie z odpowiednim komunikatem
	v.statusMessage = fmt.Sprintf("Delete %s '%s'? (y/n)", itemType, entry.name)

	return nil
}

// executeDelete wykonuje faktyczne usuwanie pliku
func (v *transferView) executeDelete() error {
	panel := v.getActivePanel()
	entry := panel.entries[panel.selectedIndex]
	path := filepath.Join(panel.path, entry.name)

	var err error
	itemType := "file"
	if entry.isDir {
		itemType = "directory"
	}

	if panel == &v.localPanel {
		if entry.isDir {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
	} else {
		transfer := v.model.GetTransfer()
		if entry.isDir {
			// Rekursywne usuwanie katalogu na zdalnym serwerze
			err = v.removeRemoteDirectory(path, transfer)
		} else {
			err = transfer.RemoveRemoteFile(path)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to delete %s '%s': %v", itemType, entry.name, err)
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

	v.statusMessage = fmt.Sprintf("Deleted %s '%s'", itemType, entry.name)
	return nil
}

func (v *transferView) removeRemoteDirectory(path string, transfer *ssh.FileTransfer) error {
	// Pobierz listę plików w katalogu
	entries, err := transfer.ListRemoteFiles(path)
	if err != nil {
		return fmt.Errorf("failed to list remote directory: %v", err)
	}

	// Rekurencyjnie usuń zawartość katalogu
	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			// Rekurencyjnie usuń podkatalog
			if err := v.removeRemoteDirectory(fullPath, transfer); err != nil {
				return err
			}
		} else {
			// Usuń plik
			if err := transfer.RemoveRemoteFile(fullPath); err != nil {
				return err
			}
		}
	}

	// Na końcu usuń sam katalog
	return transfer.RemoveRemoteFile(path)
}

// createDirectory tworzy nowy katalog
func (v *transferView) createDirectory(name string) error {
	if name == "" {
		return fmt.Errorf("directory name cannot be empty")
	}

	// Sprawdź czy nazwa nie zawiera niedozwolonych znaków
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("directory name cannot contain path separators")
	}

	panel := v.getActivePanel()
	path := filepath.Join(panel.path, name)

	var err error
	if panel == &v.localPanel {
		err = os.MkdirAll(path, 0755)
	} else {
		if !v.connected {
			return fmt.Errorf("not connected to remote host")
		}
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

	v.statusMessage = fmt.Sprintf("Created directory '%s'", name)
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

func (v *transferView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.mutex.Lock()
		v.width = msg.Width
		v.height = msg.Height
		v.model.UpdateWindowSize(msg.Width, msg.Height)
		v.mutex.Unlock()
		return v, nil

	case transferProgressMsg:
		v.mutex.Lock()
		v.progress = ssh.TransferProgress(msg)
		v.mutex.Unlock()
		return v, nil

	case transferFinishedMsg:
		v.mutex.Lock()
		v.transferring = false
		if msg.err != nil {
			v.errorMessage = fmt.Sprintf("Transfer error: %v", msg.err)
		} else {
			v.statusMessage = "Transfer completed successfully"
			dstPanel := v.getInactivePanel()
			if dstPanel == &v.localPanel {
				v.updateLocalPanel()
			} else {
				v.updateRemotePanel()
			}
		}
		v.mutex.Unlock()
		return v, nil

	case connectionStatusMsg:
		v.mutex.Lock()
		defer v.mutex.Unlock()
		v.connecting = false
		if msg.err != nil {
			v.connected = false
			v.errorMessage = fmt.Sprintf("Connection error: %v", msg.err)
			v.statusMessage = ""
		} else {
			v.connected = msg.connected
			v.statusMessage = "Connection established"
			v.errorMessage = ""
		}
		return v, nil

	case tea.KeyMsg:
		// Najpierw obsłużmy wyjście z pomocy jeśli jest aktywna
		if v.showHelp {
			switch msg.String() {
			case "esc", "q", "f1":
				v.showHelp = false
				return v, nil
			default:
				return v, nil // Ignoruj inne klawisze w trybie pomocy
			}
		}

		// Obsługa wejścia użytkownika, jeśli czekamy na input
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
		// Obsługa sekwencji ESC
		if v.escPressed {
			switch msg.String() {
			case "0", "q":
				if v.transferring {
					return v, nil
				}
				if v.connected {
					transfer := v.model.GetTransfer()
					if transfer != nil {
						transfer.Disconnect()
					}
				}
				v.model.SetActiveView(ui.ViewMain)
				return v, nil

			case "5":
				if !v.transferring {
					cmd := v.copyFile()
					v.escPressed = false
					if v.escTimeout != nil {
						v.escTimeout.Stop()
					}
					return v, cmd
				}

			case "6":
				if !v.transferring {
					v.statusMessage = "Enter new name:"
					v.input.SetValue("")
					v.input.Focus()
				}

			case "7":
				if !v.transferring {
					v.statusMessage = "Enter directory name:"
					v.input.SetValue("")
					v.input.Focus()
				}

			case "8":
				if !v.transferring {
					if err := v.deleteFile(); err != nil {
						v.handleError(err)
					}
				}
			}
			// Reset stanu ESC
			v.escPressed = false
			if v.escTimeout != nil {
				v.escTimeout.Stop()
			}
			return v, nil
		}

		// Standardowa obsługa klawiszy
		switch msg.String() {
		case "esc":
			// Aktywuj tryb ESC
			v.escPressed = true
			if v.escTimeout != nil {
				v.escTimeout.Stop()
			}
			v.escTimeout = time.NewTimer(500 * time.Millisecond)
			go func() {
				<-v.escTimeout.C
				v.escPressed = false
			}()
			return v, nil

		case "q":
			if v.transferring {
				return v, nil
			}
			if v.connected {
				transfer := v.model.GetTransfer()
				if transfer != nil {
					transfer.Disconnect()
				}
			}
			v.model.SetActiveView(ui.ViewMain)
			return v, nil

		case "f5", "c":
			if !v.transferring {
				cmd := v.copyFile()
				return v, cmd
			}
			return v, nil

		case "f6", "r":
			if !v.transferring {
				v.statusMessage = "Enter new name:"
				v.input.SetValue("")
				v.input.Focus()
			}
			return v, nil

		case "f7", "m":
			if !v.transferring {
				v.statusMessage = "Enter directory name:"
				v.input.SetValue("")
				v.input.Focus()
			}
			return v, nil

		case "f8", "d":
			if !v.transferring {
				if err := v.deleteFile(); err != nil {
					v.handleError(err)
				}
			}
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

		case "s":
			if !v.transferring {
				panel := v.getActivePanel()
				if len(panel.entries) > 0 && panel.selectedIndex < len(panel.entries) {
					entry := panel.entries[panel.selectedIndex]
					path := filepath.Join(panel.path, entry.name)
					if entry.name != ".." {
						v.model.ToggleSelection(path)
					}
				}
			}
			return v, nil

		case "y":
			if strings.HasPrefix(v.statusMessage, "Delete ") {
				if err := v.executeDelete(); err != nil {
					v.handleError(err)
				}
				v.statusMessage = ""
			}
			return v, nil

		case "n":
			if strings.HasPrefix(v.statusMessage, "Delete ") {
				v.statusMessage = "Delete cancelled"
			}
			return v, nil

		case "f1":
			v.showHelp = !v.showHelp
			return v, nil

		case "ctrl+r":
			if err := v.updateLocalPanel(); err != nil {
				v.handleError(err)
			}
			if v.connected {
				if err := v.updateRemotePanel(); err != nil {
					v.handleError(err)
				}
			}
			return v, nil
		}

	case ssh.TransferProgress:
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

// internal/ui/views/transfer.go

func (v *transferView) formatProgressBar(width int) string {
	if !v.transferring || v.progress.TotalBytes == 0 {
		return ""
	}

	percentage := float64(v.progress.TransferredBytes) / float64(v.progress.TotalBytes)
	barWidth := width - 30 // Zostaw miejsce na procenty i prędkość
	completedWidth := int(float64(barWidth) * percentage)

	bar := fmt.Sprintf("[%s%s] %3.0f%%",
		strings.Repeat("=", completedWidth),
		strings.Repeat(" ", barWidth-completedWidth),
		percentage*100)

	elapsed := time.Since(v.progress.StartTime).Seconds()
	if elapsed == 0 {
		elapsed = 1 // Zapobieganie dzieleniu przez zero
	}
	speed := float64(v.progress.TransferredBytes) / elapsed

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
 Tab          - Switch panel
 Enter        - Enter directory
 F5/ESC+5/c   - Copy file
 F6/ESC+6/r   - Rename
 F7/ESC+7/m   - Create directory
 F8/ESC+8/d   - Delete
 F1           - Toggle help
 Ctrl+r       - Refresh
 q/ESC+0      - Exit
 s            - Select/Unselect file

 Navigation
 ----------
 Up/k         - Move up
 Down/j       - Move down
`

func (v *transferView) renderShortcuts() string {
	t := table.New()

	columns := []table.Column{
		{Title: "Switch panel", Width: 12},
		{Title: "Select", Width: 8},
		{Title: "Copy", Width: 14},
		{Title: "Rename", Width: 14},
		{Title: "MkDir", Width: 14},
		{Title: "Delete", Width: 14},
		{Title: "Help", Width: 6},
		{Title: "Exit", Width: 10},
	}
	t.SetColumns(columns)

	rows := []table.Row{
		{
			"[Tab]",
			"[s]",
			"[F5|ESC+5|c]",
			"[F6|ESC+6|r]",
			"[F7|ESC+7|m]",
			"[F8|ESC+8|d]",
			"[F1]",
			"[q|ESC+0]",
		},
	}
	t.SetRows(rows)

	// Ustawiamy style
	s := table.DefaultStyles()
	t.SetStyles(s)

	// Ustawiamy wysokość tabeli na 2 (nagłówek + jeden wiersz)
	t.SetHeight(2)

	return t.View()
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

func getFileType(entry FileEntry) string {
	if entry.isDir {
		return "directory"
	}

	// Określenie typu na podstawie rozszerzenia
	ext := strings.ToLower(filepath.Ext(entry.name))

	// Archiwa
	switch ext {
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return "archive"
	}

	// Obrazy
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
		return "image"
	}

	// Dokumenty
	switch ext {
	case ".txt", ".doc", ".docx", ".pdf", ".md", ".csv", ".xlsx", ".odt":
		return "document"
	}

	// Pliki wykonywalne
	switch ext {
	case ".exe", ".sh", ".bat", ".cmd", ".com", ".app":
		return "executable"
	}

	// Pliki kodu
	switch ext {
	case ".c":
		return "code_c"
	case ".h":
		return "code_h"
	case ".go":
		return "code_go"
	case ".py":
		return "code_py"
	case ".js":
		return "code_js"
	case ".json":
		return "code_json"
		// Możesz dodać więcej rozszerzeń dla innych języków programowania tutaj
	}

	// Jeśli plik ma ustawione prawa wykonywania
	if entry.mode&0111 != 0 {
		return "executable"
	}

	return "default"
}

func (v *transferView) renderFileList(entries []FileEntry, selected int, active bool, width int) string {
	var content strings.Builder

	if len(entries) == 0 {
		return ""
	}

	nameWidth := width - 35 // Szerokość kolumny z nazwą
	sizeWidth := 10         // Stała szerokość kolumny rozmiaru

	for i, entry := range entries {
		isSelected := i == selected && selected >= 0 && selected < len(entries)
		path := filepath.Join(v.getActivePanel().path, entry.name)
		isMarked := v.model.IsSelected(path)

		// Przygotowanie nazwy pliku
		baseName := entry.name
		if entry.isDir {
			baseName = "[" + baseName + "]"
		}

		// Dodanie prefiksu dla zaznaczonych elementów
		prefix := "  "
		if isMarked {
			prefix = "* "
		}
		displayName := prefix + baseName

		// Utworzenie stylu
		mainStyle := lipgloss.NewStyle()
		if isSelected && active {
			mainStyle = mainStyle.Bold(true).Background(ui.Highlight).Foreground(lipgloss.Color("0"))
		} else if isSelected {
			mainStyle = mainStyle.Underline(false)
		} else {
			if entry.isDir {
				if active {
					mainStyle = mainStyle.Inherit(ui.DirectoryStyle)
				} else {
					mainStyle = mainStyle.Foreground(ui.Subtle)
				}
			} else {
				switch getFileType(entry) {
				case "executable":
					mainStyle = mainStyle.Inherit(ui.ExecutableStyle)
				case "archive":
					mainStyle = mainStyle.Inherit(ui.ArchiveStyle)
				case "image":
					mainStyle = mainStyle.Inherit(ui.ImageStyle)
				case "document":
					mainStyle = mainStyle.Inherit(ui.DocumentStyle)
				case "code_c":
					mainStyle = mainStyle.Inherit(ui.CodeCStyle)
				case "code_h":
					mainStyle = mainStyle.Inherit(ui.CodeHStyle)
				case "code_go":
					mainStyle = mainStyle.Inherit(ui.CodeGoStyle)
				case "code_py":
					mainStyle = mainStyle.Inherit(ui.CodePyStyle)
				case "code_js":
					mainStyle = mainStyle.Inherit(ui.CodeJsStyle)
				case "code_json":
					mainStyle = mainStyle.Inherit(ui.CodeJsonStyle)
				default:
					if strings.HasPrefix(getFileType(entry), "code_") {
						mainStyle = mainStyle.Inherit(ui.CodeDefaultStyle)
					} else {
						mainStyle = mainStyle.Inherit(ui.DefaultFileStyle)
					}
				}
			}
		}

		// Renderowanie nazwy z użyciem stylu
		styledName := mainStyle.Render(fmt.Sprintf("%-*s", nameWidth, displayName))
		sizeStr := fmt.Sprintf("%*s", sizeWidth, formatSize(entry.size))
		dateStr := entry.modTime.Format("2006-01-02 15:04")

		// Złożenie całej linii
		line := fmt.Sprintf("%s %s %19s", styledName, sizeStr, dateStr)
		content.WriteString(line)
		content.WriteString("\n")
	}

	return content.String()
}

func (v *transferView) ensureConnected() error {
	transfer := v.model.GetTransfer()
	if transfer == nil {
		return fmt.Errorf("no transfer client available")
	}

	host := v.model.GetSelectedHost()
	if host == nil {
		return fmt.Errorf("no host selected")
	}

	passwords := v.model.GetPasswords()
	if host.PasswordID >= len(passwords) {
		return fmt.Errorf("invalid password ID")
	}
	password := passwords[host.PasswordID]
	decryptedPass, err := password.GetDecrypted(v.model.GetCipher())
	if err != nil {
		return fmt.Errorf("failed to decrypt password: %v", err)
	}

	if err := transfer.Connect(host, decryptedPass); err != nil {
		return fmt.Errorf("failed to establish SFTP connection: %v", err)
	}

	return nil
}

func (v *transferView) setConnected(connected bool) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.connected = connected
}

func (v *transferView) sendConnectionUpdate() tea.Cmd {
	return func() tea.Msg {
		return connectionStatusMsg{
			connected: v.connected,
			err:       nil,
		}
	}
}

func (v *transferView) renderFooter() string {
	var footerContent strings.Builder

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

	// Komunikat o braku połączenia
	if !v.connected && v.errorMessage == "" {
		footerContent.WriteString(ui.ErrorStyle.Render(
			"SFTP connection not established. Press 'q' to return to main menu and connect first."))
		footerContent.WriteString("\n")
	}

	// Skróty klawiszowe
	if v.connected {
		footerContent.WriteString(v.renderShortcuts())
	} else {
		footerContent.WriteString(ui.ButtonStyle.Render("q") + " - Return to main menu")
	}

	return footerContent.String()
}

// createDirectory tworzy nowy katalog
