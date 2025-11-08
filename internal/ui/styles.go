package ui

import (
    "github.com/charmbracelet/lipgloss"
)

var (
    // Base colors
    Subtle    = lipgloss.Color("#6C7086")
    Highlight = lipgloss.Color("#7DC4E4")
    Special   = lipgloss.Color("#FF9E64")
    Error     = lipgloss.Color("#F38BA8")
    StatusBar = lipgloss.Color("#E7E7E7")
    Border    = lipgloss.Color("#33B2FF")

    // Base styles
    BaseStyle = lipgloss.NewStyle().
            Foreground(Subtle).
            BorderStyle(lipgloss.NormalBorder()).
            BorderForeground(Border)

    // Title styles
    TitleStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(Highlight).
            MarginLeft(2)

    // Menu item styles
    SelectedItemStyle = lipgloss.NewStyle().
                Foreground(Highlight).
                Bold(true)

    ItemStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FF3A99"))

    // Description and information styles
    DescriptionStyle = lipgloss.NewStyle().
                Foreground(Subtle).
                MarginLeft(2)

    Infotext = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FF3A99"))

    InfotextStyle = Infotext

    HostStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#2DAFFF"))

    LabelStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#A6ADC8"))

    InputStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FFFFFF")).
            BorderStyle(lipgloss.NormalBorder()).
            BorderForeground(Highlight).
            Padding(0, 1)

    // Status styles
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

    // Panel styles
    PanelTitleStyle = lipgloss.NewStyle().
            Foreground(Highlight).
            Bold(true).
            Padding(0, 1)

    PanelStyle = lipgloss.NewStyle().
            Border(lipgloss.NormalBorder()).
            BorderForeground(Border).
            Padding(0, 1)

    // Disabled element styles
    ButtonDisabledStyle = lipgloss.NewStyle().
                Foreground(Subtle).
                Bold(true)

    DescriptionDisabledStyle = lipgloss.NewStyle().
                    Foreground(Subtle).
                    MarginLeft(2)

    // Button styles
    ButtonStyle = lipgloss.NewStyle().
            Foreground(Special).
            Bold(true)

    // Success and error styles
    SuccessStyle = lipgloss.NewStyle().
            Foreground(Special).
            Bold(true)

    ErrorStyle = lipgloss.NewStyle().
            Foreground(Error).
            Bold(true)

    // Container styles
    WindowStyle = lipgloss.NewStyle().
            BorderStyle(lipgloss.DoubleBorder()).
            BorderForeground(Border).
            Padding(1, 2)

    // Table styles
    HeaderStyle = lipgloss.NewStyle().
            Foreground(Highlight).
            Bold(true).
            Underline(true).
            Padding(0, 1)

    CellStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FFFFFF")).
            Padding(0, 1)

    // Dialog styles
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

    // Status bar styles
    StatusBarStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FFFFFF")).
            Background(StatusBar).
            Bold(true).
            Padding(0, 1).
            Width(103)

    // Command bar styles
    CommandBarStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FFFFFF")).
            Padding(0, 0).
            Width(103).
            BorderStyle(lipgloss.NormalBorder()).
            BorderTop(true).
            BorderForeground(Border)

    // File type styles
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

    DefaultFileStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("#A9A9A9"))

    SelectedFileStyle = lipgloss.NewStyle().
                Foreground(lipgloss.Color("#FF1493"))

    // Code file styles
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
)
