package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Kolory
	Subtle    = lipgloss.Color("#6C7086") // Subtelny szary dla mniej istotnego tekstu
	Highlight = lipgloss.Color("#7DC4E4") // Jasny niebieski dla tytułów i podświetleń
	Special   = lipgloss.Color("#FF9E64") // Pomarańczowy dla przycisków i elementów specjalnych
	Error     = lipgloss.Color("#F38BA8") // Jasny czerwony dla błędów
	StatusBar = lipgloss.Color("#33B2FF") // Neonowy róż dla paska statusu
	Border    = lipgloss.Color("#33B2FF") // Kolor obramowań

	// Style podstawowe
	BaseStyle = lipgloss.NewStyle().
			Foreground(Subtle).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(Border)

	// Tytuł
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Highlight).
			MarginLeft(2)

	// Elementy menu
	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(Highlight).
				Bold(true)

	ItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3A99"))

	// Opisy i informacje
	DescriptionStyle = lipgloss.NewStyle().
				Foreground(Subtle).
				MarginLeft(2)
		// W pakiecie styles lub ui

	Infotext = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3A99"))

	LabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6ADC8")) // Subtelny szary dla etykiet
	// Pola wejściowe
	InputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(Highlight).
			Padding(0, 1)

	// Przyciski
	ButtonStyle = lipgloss.NewStyle().
			Foreground(Special).
			Bold(true)

	// Statusy
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Special).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	// Kontenery
	WindowStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(Border).
			Padding(1, 2)

	// Tabele
	HeaderStyle = lipgloss.NewStyle().
			Foreground(Highlight).
			Bold(true).
			Underline(true).
			Padding(0, 1)

	CellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	// Style dialogów
	DialogStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(1, 2)

	DialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Highlight).
				Padding(0, 1)

	DialogButtonStyle = lipgloss.NewStyle().
				Foreground(Special).
				Bold(true).
				Padding(0, 2)

	// Panele
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(Border).
			Padding(0, 1)

	// Pasek statusu
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(StatusBar).
			Bold(true).
			Padding(0, 1).
			Width(103)

	// Pasek poleceń
	CommandBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 0).
			Width(103).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(Border)
)

// GetMaxWidth zwraca maksymalną szerokość tekstu w slice'u
func GetMaxWidth(items []string) int {
	maxWidth := 0
	for _, item := range items {
		if w := lipgloss.Width(item); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

// CenterText centruje tekst w danej szerokości
func CenterText(text string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, text)
}
