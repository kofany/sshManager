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
	"sshManager/internal/ui/components"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"
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
	maxVisibleItems   = 20
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
	width         int               // Dodane
	height        int               // Dodane
	escPressed    bool              // flaga wskazująca czy ESC został wciśnięty
	escTimeout    *time.Timer       // timer do resetowania stanu ESC
	popup         *components.Popup // Zmieniamy typ na nowy komponent

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

	// Oblicz szerokość panelu
	panelWidth := (min(v.width-40, 160) - 3) / 2

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

	// Renderowanie listy plików
	filesList := v.renderFileList(
		p.entries[p.scrollOffset:min(p.scrollOffset+maxVisibleItems, len(p.entries))],
		p.selectedIndex-p.scrollOffset,
		p.active,
		panelWidth-2,
	)
	panelContent.WriteString(filesList)

	// Informacja o przewijaniu
	if len(p.entries) > maxVisibleItems {
		panelContent.WriteString(fmt.Sprintf("\nShowing %d-%d of %d items",
			p.scrollOffset+1,
			min(p.scrollOffset+maxVisibleItems, len(p.entries)),
			len(p.entries)))
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

	// Wyrównaj liczbę linii w panelach
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

	// Renderuj całość w oknie
	finalContent := ui.WindowStyle.Render(content.String())

	// Jeśli jest aktywny popup, renderuj go na wierzchu (wycentrowany)
	if v.popup != nil {
		return lipgloss.Place(
			v.width,
			v.height,
			lipgloss.Center,
			lipgloss.Center,
			finalContent+"\n"+v.popup.Render(),
			lipgloss.WithWhitespaceChars(""),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
		)
	}

	// Główny widok wyrównany do lewego górnego rogu
	return lipgloss.Place(
		v.width,
		v.height,
		lipgloss.Left, // Zmiana z Center na Left
		lipgloss.Top,  // Zmiana z Center na Top
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

// update

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
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Transfer Error",
				fmt.Sprintf("Transfer error: %v", msg.err),
				50,
				7,
				v.width,
				v.height,
			)
		} else {
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Success",
				"Transfer completed successfully",
				50,
				7,
				v.width,
				v.height,
			)
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
		v.connecting = false
		if msg.err != nil {
			v.connected = false
			v.popup = components.NewPopup(
				components.PopupMessage,
				"Connection Error",
				fmt.Sprintf("Connection error: %v", msg.err),
				50,
				7,
				v.width,
				v.height,
			)
		} else {
			v.connected = msg.connected
		}
		v.mutex.Unlock()
		return v, nil

	case tea.KeyMsg:
		// Obsługa popupu
		if v.popup != nil {
			switch msg.String() {
			case "esc":
				v.popup = nil
				return v, nil
			case "enter":
				if v.popup.Type != components.PopupDelete {
					// Użyj v.popup.Input zamiast v.input
					if err := v.handleCommand(v.popup.Input.Value()); err != nil {
						v.handleError(err)
					}
					v.popup = nil
					return v, nil
				}
			case "y":
				if v.popup.Type == components.PopupDelete {
					if err := v.executeDelete(); err != nil {
						v.handleError(err)
					}
					v.popup = nil
					return v, nil
				}
			case "n":
				if v.popup.Type == components.PopupDelete {
					v.popup = nil
					return v, nil
				}
			default:
				if v.popup.Type != components.PopupDelete {
					var cmd tea.Cmd
					// Aktualizuj v.popup.Input zamiast v.input
					v.popup.Input, cmd = v.popup.Input.Update(msg)
					return v, cmd
				}
			}
			return v, nil
		}
		// Obsługa trybu pomocy
		if v.showHelp {
			switch msg.String() {
			case "esc", "q", "f1":
				v.showHelp = false
				return v, nil
			default:
				return v, nil // Ignoruj inne klawisze w trybie pomocy
			}
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
					v.popup = components.NewPopup(
						components.PopupRename,
						"Rename",
						"Enter new name:",
						50,
						7,
						v.width,
						v.height,
					)
					v.popup.Input.SetValue("")
					v.popup.Input.Focus()
				}
				return v, nil

			case "7":
				if !v.transferring {
					v.popup = components.NewPopup(
						components.PopupMkdir,
						"Create Directory",
						"Enter directory name:",
						50,
						7,
						v.width,
						v.height,
					)
					v.popup.Input.SetValue("")
					v.popup.Input.Focus()
				}
				return v, nil

			case "8":
				if !v.transferring {
					panel := v.getActivePanel()
					if len(panel.entries) == 0 || panel.selectedIndex >= len(panel.entries) {
						return v, nil
					}
					entry := panel.entries[panel.selectedIndex]
					if entry.name == ".." {
						return v, nil
					}
					v.popup = components.NewPopup(
						components.PopupDelete,
						"Delete",
						fmt.Sprintf("Delete %s '%s'? (y/n)",
							map[bool]string{true: "directory", false: "file"}[entry.isDir],
							entry.name),
						50,
						7,
						v.width,
						v.height,
					)
				}
				return v, nil
			}
			// Reset stan}u ESC
			v.escPressed = false
			if v.escTimeout != nil {
				v.escTimeout.Stop()
			}
			return v, nil
		}

		// Pojedyncze naciśnięcie ESC
		if msg.String() == "esc" {
			if v.popup != nil {
				v.popup = nil
				return v, nil
			}
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
		}

		// Standardowe klawisze funkcyjne
		switch msg.String() {
		case " ": // dodajemy jako pierwszy case
			if !v.transferring {
				ui.SwitchTheme()
				return v, nil
			}
		case "f1":
			v.showHelp = !v.showHelp
			return v, nil

		case "f5", "c":
			if !v.transferring {
				cmd := v.copyFile()
				return v, cmd
			}
			return v, nil

		case "f6", "r":
			if !v.transferring {
				v.popup = components.NewPopup(
					components.PopupRename,
					"Rename",
					"Enter new name:",
					50,
					7,
					v.width,
					v.height,
				)
				v.popup.Input.SetValue("")
				v.popup.Input.Focus()
			}
			return v, nil

		case "f7", "m":
			if !v.transferring {
				v.popup = components.NewPopup(
					components.PopupMkdir,
					"Create Directory",
					"Enter directory name:",
					50,
					7,
					v.width,
					v.height,
				)
				v.popup.Input.SetValue("")
				v.popup.Input.Focus()
			}
			return v, nil

		case "f8", "d":
			if !v.transferring {
				panel := v.getActivePanel()
				if len(panel.entries) == 0 || panel.selectedIndex >= len(panel.entries) {
					return v, nil
				}
				entry := panel.entries[panel.selectedIndex]
				if entry.name == ".." {
					return v, nil
				}
				v.popup = components.NewPopup(
					components.PopupDelete,
					"Delete",
					fmt.Sprintf("Delete %s '%s'? (y/n)",
						map[bool]string{true: "directory", false: "file"}[entry.isDir],
						entry.name),
					50,
					7,
					v.width,
					v.height,
				)
			}
			return v, nil

		// Standardowe klawisze nawigacji i kontroli
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

		case "tab":
			if v.connected {
				v.switchActivePanel()
				v.errorMessage = ""
			}
			return v, nil

		case "up", "w":
			panel := v.getActivePanel()
			v.navigatePanel(panel, -1)
			v.errorMessage = ""
			return v, nil

		case "down", "s":
			panel := v.getActivePanel()
			v.navigatePanel(panel, 1)
			v.errorMessage = ""
			return v, nil

		case "enter":
			panel := v.getActivePanel()
			if err := v.enterDirectory(panel); err != nil {
				v.popup = components.NewPopup(
					components.PopupMessage,
					"Error",
					err.Error(),
					50,
					7,
					v.width,
					v.height,
				)
			}
			return v, nil

		case "x":
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

		}

	case ssh.TransferProgress:
		v.progress = msg
		return v, nil
	}

	return v, nil
}

// handleCommand obsługuje wprowadzanie komend
func (v *transferView) handleCommand(cmd string) error {
	if v.popup == nil {
		return fmt.Errorf("no active popup")
	}

	switch v.popup.Type { // użycie Type zamiast promptType
	case components.PopupRename: // użycie components.PopupRename zamiast promptRename
		err := v.renameFile(cmd)
		v.popup = nil
		return err
	case components.PopupMkdir: // użycie components.PopupMkdir zamiast promptMkdir
		err := v.createDirectory(cmd)
		v.popup = nil
		return err
	default:
		v.popup = nil
		return fmt.Errorf("unknown command")
	}
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
 x            - Select/Unselect file

 Navigation
 ----------
 Up/w         - Move up
 Down/s       - Move down
`

func (v *transferView) renderShortcuts() string {
	// Nagłówki tabeli i skróty
	headers := []string{"Switch Panel", "Select", "Copy", "Rename", "MkDir", "Delete", "Help", "Theme", "Exit"}
	shortcuts := []string{"[Tab]", "[x]", "[F5|ESC+5|c]", "[F6|ESC+6|r]", "[F7|ESC+7|m]", "[F8|ESC+8|d]", "[F1]", "[space]", "[q|ESC+0]"}

	// Funkcja stylizująca kolumny
	var TableStyle = func(row, col int) lipgloss.Style {
		switch {
		case row == 0: // Nagłówki
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(ui.Subtle).
				Align(lipgloss.Center)
		default: // Skróty
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(ui.Special).
				Align(lipgloss.Center)
		}
	}

	// Tworzenie tabeli
	cmdTable := ltable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.StatusBar)).
		StyleFunc(TableStyle).
		Headers(headers...).
		Row(shortcuts...)

	// Renderowanie tabeli
	return cmdTable.Render()
}

// Funkcja pomocnicza do budowania wierszy

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

// internal/ui/views/transfer.go
// internal/ui/views/transfer.go

func (v *transferView) renderFileList(entries []FileEntry, selected int, _ bool, width int) string {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: " ", Width: 2}, // Kolumna na gwiazdkę
			{Title: "Name", Width: width - 37},
			{Title: "Size", Width: 10},
			{Title: "Modified", Width: 19},
		}),
	)

	var rows []table.Row
	for _, entry := range entries {
		path := filepath.Join(v.getActivePanel().path, entry.name)
		isMarked := v.model.IsSelected(path)

		// Tworzenie wiersza
		prefix := " "
		if isMarked {
			prefix = "*"
		}

		name := entry.name
		if entry.isDir {
			name = "[" + name + "]"
		}

		row := table.Row{
			prefix,
			name,
			formatSize(entry.size),
			entry.modTime.Format("2006-01-02 15:04"),
		}
		rows = append(rows, row)
	}

	t.SetRows(rows)

	// Renderujemy tabelę
	tableOutput := t.View()

	// Teraz dodajemy kolory linijka po linijce
	var coloredOutput strings.Builder
	lines := strings.Split(tableOutput, "\n")

	for i, line := range lines {
		// Pomijamy linie nagłówka (pierwsza linia)
		if i == 0 {
			coloredOutput.WriteString(line + "\n")
			continue
		}

		// Sprawdzamy czy ta linia odpowiada jakiemuś plikowi
		entryIndex := i - 1 // odejmujemy 1 bo pierwsza linia to nagłówek
		if entryIndex >= 0 && entryIndex < len(entries) {
			entry := entries[entryIndex]
			var style lipgloss.Style

			// Specjalne traktowanie linii ".."
			if entry.name == ".." {
				if entryIndex == selected {
					// Ten sam styl dla aktywnego i nieaktywnego panelu gdy ".." jest zaznaczone
					style = lipgloss.NewStyle().
						Bold(true).
						Background(ui.Highlight).
						Foreground(lipgloss.Color("0"))
				} else {
					style = ui.DirectoryStyle
				}
			} else if entryIndex == selected {
				// Ten sam styl dla zaznaczenia w obu panelach
				style = lipgloss.NewStyle().
					Bold(true).
					Background(ui.Highlight).
					Foreground(lipgloss.Color("0"))
			} else if entry.isDir {
				// Katalogi zawsze używają DirectoryStyle
				style = ui.DirectoryStyle
			} else {
				switch getFileType(entry) {
				case "executable":
					style = ui.ExecutableStyle
				case "archive":
					style = ui.ArchiveStyle
				case "image":
					style = ui.ImageStyle
				case "document":
					style = ui.DocumentStyle
				case "code_c":
					style = ui.CodeCStyle
				case "code_h":
					style = ui.CodeHStyle
				case "code_go":
					style = ui.CodeGoStyle
				case "code_py":
					style = ui.CodePyStyle
				case "code_js":
					style = ui.CodeJsStyle
				case "code_json":
					style = ui.CodeJsonStyle
				default:
					if strings.HasPrefix(getFileType(entry), "code_") {
						style = ui.CodeDefaultStyle
					} else {
						style = ui.DefaultFileStyle
					}
				}
			}
			coloredOutput.WriteString(style.Render(line) + "\n")
		} else {
			coloredOutput.WriteString(line + "\n")
		}
	}

	return coloredOutput.String()
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

	var authData string

	if host.PasswordID < 0 {
		// Obsługa klucza SSH
		keyIndex := -(host.PasswordID + 1)
		keys := v.model.GetKeys()
		if keyIndex >= len(keys) {
			return fmt.Errorf("invalid key ID")
		}

		key := keys[keyIndex]
		keyPath, pathErr := key.GetKeyPath()
		if pathErr != nil {
			return fmt.Errorf("failed to get key path: %v", pathErr)
		}
		authData = keyPath
	} else {
		// Obsługa hasła
		passwords := v.model.GetPasswords()
		if host.PasswordID >= len(passwords) {
			return fmt.Errorf("invalid password ID")
		}

		password := passwords[host.PasswordID]
		decryptedPass, decErr := password.GetDecrypted(v.model.GetCipher())
		if decErr != nil {
			return fmt.Errorf("failed to decrypt password: %v", decErr)
		}
		authData = decryptedPass
	}

	if err := transfer.Connect(host, authData); err != nil {
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
