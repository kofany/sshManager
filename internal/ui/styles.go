package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Kolory
	Subtle    = lipgloss.Color("#6C7086")
	Highlight = lipgloss.Color("#7DC4E4")
	Special   = lipgloss.Color("#FF9E64")
	Error     = lipgloss.Color("#F38BA8")
	StatusBar = lipgloss.Color("#E7E7E7")
	Border    = lipgloss.Color("#33B2FF")
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

	Infotext = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3A99"))

	HostStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2DAFFF"))

	LabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6ADC8"))

	InputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(Highlight).
			Padding(0, 1)

	StatusConnectingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7DC4E4")).
				Bold(true)

	StatusConnectedStyle = lipgloss.NewStyle().
				Foreground(Special).
				Bold(true)

	StatusDefaultStyle = lipgloss.NewStyle().
				Foreground(Subtle)

	StatusStyle = lipgloss.NewStyle().
			Foreground(StatusBar)

	// Style dla paneli
	PanelTitleStyle = lipgloss.NewStyle().
			Foreground(Highlight).
			Bold(true).
			Padding(0, 1)

	// Style dla wyłączonych elementów
	ButtonDisabledStyle = lipgloss.NewStyle().
				Foreground(Subtle).
				Bold(true)

	DescriptionDisabledStyle = lipgloss.NewStyle().
					Foreground(Subtle).
					MarginLeft(2)

	// Zmiana nazwy Infotext na InfotextStyle dla spójności
	InfotextStyle = Infotext
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

	// Style dla różnych typów plików
	DirectoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1E90FF")).
			Bold(true)

	ExecutableStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#32CD32"))

	ArchiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BA55D3"))

	ImageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF8C00"))

	DocumentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	CodeCStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CED1"))

	CodeHStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4682B4"))

	CodeGoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2E8B57"))

	CodePyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6A5ACD"))

	CodeJsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DAA520"))
	CodeJsonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7FFF00"))
	CodeDefaultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#708090"))

	DefaultFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A9A9A9"))
	SelectedFileStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF1493"))
)
