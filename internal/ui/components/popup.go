package components

import (
	"sshManager/internal/ui"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type PopupType int

const (
	PopupNone PopupType = iota
	PopupRename
	PopupMkdir
	PopupDelete
	PopupHostKey
	PopupMessage
)

type Popup struct {
	Type         PopupType
	Title        string
	Message      string
	Input        textinput.Model
	Width        int
	Height       int
	ScreenWidth  int // Dodane
	ScreenHeight int // Dodane
}

func NewPopup(popupType PopupType, title, message string, width, height, screenWidth, screenHeight int) *Popup {
	input := textinput.New()
	input.Placeholder = "Enter value..."
	input.Focus()

	return &Popup{
		Type:         popupType,
		Title:        title,
		Message:      message,
		Input:        input,
		Width:        width,
		Height:       height,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
	}
}

func (p *Popup) Render() string {
	// Style dla popupu
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Border).
		Padding(1, 2).
		Width(p.Width).
		Height(p.Height)

	// Style dla tytułu
	titleStyle := ui.TitleStyle.
		Align(lipgloss.Center).
		Width(p.Width - 4)

	// Budowanie zawartości popupu
	var content strings.Builder
	content.WriteString(titleStyle.Render(p.Title) + "\n\n")
	content.WriteString(p.Message + "\n")

	// Dodaj pole input dla promptów wymagających wprowadzenia tekstu
	if p.Type == PopupRename || p.Type == PopupMkdir {
		content.WriteString("\n" + p.Input.View())
	}

	// Dodaj informację o klawiszach
	var keys string
	switch p.Type {
	case PopupDelete, PopupHostKey:
		keys = "y - Yes, n - No" // Translated from "y - Potwierdź, n - Anuluj, ESC - Anuluj"
	case PopupMessage:
		keys = "ESC/ENTER - Close"
	default:
		keys = "ENTER - Confirm, ESC - Cancel"
	}
	content.WriteString("\n" + ui.DescriptionStyle.Render(keys))

	// Renderowanie popupu
	popupContent := popupStyle.Render(content.String())

	// Wyśrodkowanie popupu na ekranie
	return lipgloss.Place(
		p.ScreenWidth,
		p.ScreenHeight,
		lipgloss.Center,
		lipgloss.Center,
		popupContent,
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}
