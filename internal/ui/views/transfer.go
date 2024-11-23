// internal/ui/views/transfer.go - Część 1

package views

import (
	"fmt"
	"path/filepath"
	"sshManager/internal/ssh"
	"sshManager/internal/ui"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Tryby pracy widoku transferu
type transferMode int

const (
	modeSelect   transferMode = iota // Wybór operacji
	modeUpload                       // Wysyłanie pliku
	modeDownload                     // Pobieranie pliku
	modeBrowse                       // Przeglądanie plików
)

// Typ operacji transferu
type transferOperation int

const (
	opNone transferOperation = iota
	opUpload
	opDownload
)

// Stan transferu
type transferState struct {
	fileName    string
	total       int64
	transferred int64
	startTime   time.Time
	operation   transferOperation
}

// Główna struktura widoku
type transferView struct {
	model        *ui.Model
	mode         transferMode
	state        transferState
	pathInput    textinput.Model
	remoteFiles  []string
	selectedFile int
	scrollOffset int
	errMsg       string
	progressChan chan ssh.TransferProgress
}

// Konstruktor widoku
func NewTransferView(model *ui.Model) *transferView {
	pi := textinput.New()
	pi.Placeholder = "Wprowadź ścieżkę"
	pi.CharLimit = 255

	return &transferView{
		model:        model,
		mode:         modeSelect,
		pathInput:    pi,
		progressChan: make(chan ssh.TransferProgress),
	}
}

// Inicjalizacja widoku
func (v *transferView) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		v.listenForProgress,
	)
}

// Nasłuchiwanie postępu transferu
func (v *transferView) listenForProgress() tea.Msg {
	return <-v.progressChan
}

// Aktualizacja listy plików zdalnych
func (v *transferView) updateRemoteFiles(path string) error {
	transfer := v.model.GetTransfer()
	if transfer == nil {
		return fmt.Errorf("brak połączenia")
	}

	files, err := transfer.ListFiles(path)
	if err != nil {
		return fmt.Errorf("nie udało się pobrać listy plików: %v", err)
	}

	v.remoteFiles = files
	v.selectedFile = 0
	v.scrollOffset = 0
	return nil
}

// Formatowanie postępu transferu
func (v *transferView) formatProgress() string {
	if v.state.total == 0 {
		return ""
	}

	percentage := float64(v.state.transferred) / float64(v.state.total) * 100
	return fmt.Sprintf("%.1f%% (%d/%d bajtów)",
		percentage,
		v.state.transferred,
		v.state.total,
	)
}

// Formatowanie czasu trwania operacji
func (v *transferView) formatDuration() string {
	if v.state.startTime.IsZero() {
		return ""
	}

	duration := time.Since(v.state.startTime)
	return fmt.Sprintf("Czas: %.1f s", duration.Seconds())
}

// Reset stanu transferu
func (v *transferView) resetState() {
	v.state = transferState{}
	v.errMsg = ""
	v.pathInput.Reset()
	v.remoteFiles = nil
	v.selectedFile = 0
	v.scrollOffset = 0
}

// internal/ui/views/transfer.go - Część 2

// Dodaj tę sekcję po poprzednim kodzie

func (v *transferView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	// Obsługa postępu transferu
	// W metodzie Update
	case ssh.TransferProgress:
		v.state.fileName = msg.FileName
		v.state.total = msg.TotalBytes
		v.state.transferred = msg.TransferredBytes
		if v.state.startTime.IsZero() {
			v.state.startTime = time.Now()
		}
		return v, v.listenForProgress

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if v.mode == modeSelect {
				v.model.SetActiveView(ui.ViewMain)
				v.resetState()
				return v, nil
			}
			// Powrót do wyboru operacji
			v.mode = modeSelect
			v.resetState()
			return v, nil

		case "up", "k":
			if v.mode == modeBrowse && len(v.remoteFiles) > 0 {
				v.selectedFile--
				if v.selectedFile < 0 {
					v.selectedFile = len(v.remoteFiles) - 1
				}
				// Dostosowanie przewijania
				if v.selectedFile < v.scrollOffset {
					v.scrollOffset = v.selectedFile
				}
			}

		case "down", "j":
			if v.mode == modeBrowse && len(v.remoteFiles) > 0 {
				v.selectedFile++
				if v.selectedFile >= len(v.remoteFiles) {
					v.selectedFile = 0
				}
				// Dostosowanie przewijania
				if v.selectedFile >= v.scrollOffset+10 { // 10 to liczba widocznych linii
					v.scrollOffset = v.selectedFile - 9
				}
			}

		case "enter":
			switch v.mode {
			case modeSelect:
				switch v.selectedFile {
				case 0: // Upload
					v.mode = modeUpload
					v.pathInput.Focus()
					v.state.operation = opUpload
				case 1: // Download
					v.mode = modeDownload
					if err := v.updateRemoteFiles("/"); err != nil {
						v.errMsg = err.Error()
					}
					v.mode = modeBrowse
					v.state.operation = opDownload
				}

			case modeUpload:
				if v.pathInput.Value() == "" {
					v.errMsg = "Wprowadź ścieżkę pliku"
					return v, nil
				}
				return v.handleUpload()

			case modeBrowse:
				if len(v.remoteFiles) == 0 {
					return v, nil
				}
				selectedFile := v.remoteFiles[v.selectedFile]

				// Sprawdzenie czy to katalog
				fileInfo, err := v.model.GetTransfer().GetFileInfo(selectedFile)
				if err != nil {
					v.errMsg = err.Error()
					return v, nil
				}

				if fileInfo.IsDir() {
					if err := v.updateRemoteFiles(selectedFile); err != nil {
						v.errMsg = err.Error()
					}
					return v, nil
				}

				// Jeśli to plik, rozpocznij pobieranie
				v.pathInput.SetValue(selectedFile)
				return v.handleDownload()
			}
		}
	}

	// Obsługa wprowadzania tekstu w polu ścieżki
	if v.mode == modeUpload {
		var cmd tea.Cmd
		v.pathInput, cmd = v.pathInput.Update(msg)
		return v, cmd
	}

	return v, cmd
}

func (v *transferView) handleUpload() (tea.Model, tea.Cmd) {
	localPath := v.pathInput.Value()
	// Pobierz nazwę pliku z ścieżki
	fileName := filepath.Base(localPath)
	remotePath := filepath.Join("/", fileName)

	go func() {
		err := v.model.GetTransfer().UploadFile(localPath, remotePath, v.progressChan)
		if err != nil {
			v.errMsg = fmt.Sprintf("Błąd podczas wysyłania: %v", err)
		} else {
			v.model.SetStatus("Plik wysłany pomyślnie", false)
			v.mode = modeSelect
			v.resetState()
		}
	}()

	return v, v.listenForProgress
}

func (v *transferView) handleDownload() (tea.Model, tea.Cmd) {
	remotePath := v.pathInput.Value()
	// Pobierz nazwę pliku z ścieżki
	fileName := filepath.Base(remotePath)
	localPath := filepath.Join(".", fileName)

	go func() {
		err := v.model.GetTransfer().DownloadFile(remotePath, localPath, v.progressChan)
		if err != nil {
			v.errMsg = fmt.Sprintf("Błąd podczas pobierania: %v", err)
		} else {
			v.model.SetStatus("Plik pobrany pomyślnie", false)
			v.mode = modeSelect
			v.resetState()
		}
	}()

	return v, v.listenForProgress
}

// internal/ui/views/transfer.go - Część 3

func (v *transferView) View() string {
	var content string

	// Nagłówek
	content = ui.TitleStyle.Render("Transfer plików") + "\n\n"

	// Stan połączenia
	if host := v.model.GetSelectedHost(); host != nil {
		content += ui.SuccessStyle.Render(fmt.Sprintf("Host: %s (%s)", host.Name, host.IP)) + "\n\n"
	}

	// Główna zawartość zależna od trybu
	switch v.mode {
	case modeSelect:
		content += "Wybierz operację:\n\n"
		options := []string{"Wyślij plik (upload)", "Pobierz plik (download)"}
		for i, opt := range options {
			if i == v.selectedFile {
				content += ui.SelectedItemStyle.Render("> " + opt + "\n")
			} else {
				content += "  " + opt + "\n"
			}
		}
		content += "\n" + ui.ButtonStyle.Render("ENTER") + " - Wybierz    " +
			ui.ButtonStyle.Render("ESC") + " - Powrót"

	case modeUpload:
		content += ui.TitleStyle.Render("Wysyłanie pliku") + "\n\n"
		content += "Wprowadź ścieżkę lokalnego pliku:\n"
		content += ui.InputStyle.Render(v.pathInput.View()) + "\n\n"
		content += ui.ButtonStyle.Render("ENTER") + " - Wyślij    " +
			ui.ButtonStyle.Render("ESC") + " - Powrót"

		// Wyświetlanie postępu
		if v.state.fileName != "" {
			content += "\n\nWysyłanie " + v.state.fileName + "\n"
			content += v.renderProgressBar() + "\n"
			content += v.formatProgress() + "\n"
			content += v.formatDuration()
		}

	case modeBrowse:
		content += ui.TitleStyle.Render("Przeglądanie plików zdalnych") + "\n\n"

		if len(v.remoteFiles) == 0 {
			content += ui.DescriptionStyle.Render("Brak plików") + "\n"
		} else {
			// Wyświetlanie listy plików z przewijaniem
			visibleFiles := v.remoteFiles[v.scrollOffset:]
			if len(visibleFiles) > 10 {
				visibleFiles = visibleFiles[:10]
			}

			for i, file := range visibleFiles {
				fileIndex := v.scrollOffset + i
				prefix := "  "
				if fileIndex == v.selectedFile {
					prefix = "> "
					content += ui.SelectedItemStyle.Render(prefix + file + "\n")
				} else {
					content += prefix + file + "\n"
				}
			}

			// Informacja o przewijaniu
			if len(v.remoteFiles) > 10 {
				content += "\n" + ui.DescriptionStyle.Render(
					fmt.Sprintf("Pokazano %d/%d plików",
						len(visibleFiles),
						len(v.remoteFiles)),
				)
			}
		}

		content += "\n" + ui.ButtonStyle.Render("↑/↓") + " - Nawigacja    " +
			ui.ButtonStyle.Render("ENTER") + " - Wybierz    " +
			ui.ButtonStyle.Render("ESC") + " - Powrót"

	case modeDownload:
		content += ui.TitleStyle.Render("Pobieranie pliku") + "\n\n"

		// Wyświetlanie postępu
		if v.state.fileName != "" {
			content += "Pobieranie " + v.state.fileName + "\n"
			content += v.renderProgressBar() + "\n"
			content += v.formatProgress() + "\n"
			content += v.formatDuration()
		}
	}

	// Wyświetlanie błędów
	if v.errMsg != "" {
		content += "\n\n" + ui.ErrorStyle.Render(v.errMsg)
	}

	return ui.WindowStyle.Render(content)
}

// renderProgressBar generuje pasek postępu
func (v *transferView) renderProgressBar() string {
	width := 50 // szerokość paska postępu
	completed := int(float64(v.state.transferred) / float64(v.state.total) * float64(width))

	if completed > width {
		completed = width
	}

	bar := "["
	for i := 0; i < width; i++ {
		if i < completed {
			bar += "="
		} else {
			bar += " "
		}
	}
	bar += "]"

	return ui.DescriptionStyle.Render(bar)
}

// Upewnij się, że transferView implementuje tea.Model
var _ tea.Model = (*transferView)(nil)
