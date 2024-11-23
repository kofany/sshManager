// internal/ui/styles.go

package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Kolory
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	error     = lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF0000"}

	// Style podstawowe
	BaseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle)

	// Tytuł
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginLeft(2)

	// Menu items
	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)

	ItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	// Opisy i informacje
	DescriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				MarginLeft(2)

	// Pola wejściowe
	InputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(highlight).
			Padding(0, 1)

	// Przyciski
	ButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 3).
			MarginRight(2)

	// Statusy
	SuccessStyle = lipgloss.NewStyle().
			Foreground(special).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(error).
			Bold(true)

	// Kontenery
	WindowStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(1, 2)

	// Tabele
	HeaderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle).
			Bold(true).
			Padding(0, 1)

	CellStyle = lipgloss.NewStyle().
			Padding(0, 1)
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
