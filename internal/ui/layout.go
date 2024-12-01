// internal/ui/layout.go

package ui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"
)

// BaseLayout zawiera podstawowe wymiary i style dla layoutu
type BaseLayout struct {
	Width         int
	Height        int
	HeaderHeight  int
	FooterHeight  int
	ContentHeight int
}

// NewBaseLayout tworzy nowy podstawowy layout
func NewBaseLayout(width, height int) BaseLayout {
	const (
		headerHeight = 3 // Wysokość nagłówka
		footerHeight = 4 // Wysokość stopki
	)

	return BaseLayout{
		Width:         width,
		Height:        height,
		HeaderHeight:  headerHeight,
		FooterHeight:  footerHeight,
		ContentHeight: height - headerHeight - footerHeight,
	}
}

// MainContainer tworzy główny kontener aplikacji
func (l BaseLayout) MainContainer() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(l.Width).
		Height(l.Height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Border)
}

// Header tworzy styl dla nagłówka
func (l BaseLayout) Header() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(l.Width-2). // -2 na ramkę
		Height(l.HeaderHeight).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(Border)
}

// Footer tworzy styl dla stopki
func (l BaseLayout) Footer() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(l.Width-2). // -2 na ramkę
		Height(l.FooterHeight).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(Border)
}

// ContentArea tworzy styl dla głównej zawartości
func (l BaseLayout) ContentArea() lipgloss.Style {
	return lipgloss.NewStyle().
		Width(l.Width - 2). // -2 na ramkę
		Height(l.ContentHeight)
}

// CreateBubbleTable tworzy tabelę bubble tea z odpowiednimi stylami
func CreateBubbleTable(columns []table.Column, rows []table.Row, width int) table.Model {
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(15),
		table.WithWidth(width),
	)

	style := table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(Highlight).
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(Highlight).
			Bold(true).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Padding(0, 1),
	}

	t.SetStyles(style)
	return t
}

// CreateLipglossTable tworzy tabelę lipgloss z odpowiednimi stylami
func CreateLipglossTable(headers []string, rows [][]string) string {
	tableStyle := func(row, col int) lipgloss.Style {
		switch {
		case row == -1: // Nagłówki
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(Highlight).
				Bold(true)
		default:
			return lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(Special)
		}
	}

	return ltable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(Border)).
		StyleFunc(tableStyle).
		Headers(headers...).
		Rows(rows...).
		Render()
}

// SplitView tworzy styl dla widoku podzielonego (np. dla transfer view)
func (l BaseLayout) SplitView() (left, right lipgloss.Style) {
	panelWidth := (l.Width - 5) / 2 // 5 to szerokość separatora i ramek

	baseStyle := lipgloss.NewStyle().
		Height(l.ContentHeight).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Border)

	left = baseStyle.Width(panelWidth)
	right = baseStyle.Width(panelWidth)

	return left, right
}
