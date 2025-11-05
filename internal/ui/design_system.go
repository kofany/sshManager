package ui

import (
    "github.com/charmbracelet/lipgloss"
)

// Design System - Comprehensive styling constants and helpers
// This provides a consistent look and feel across all views

// Color Palette - Modern and consistent
var (
    // Primary Colors
    ColorPrimary   = lipgloss.AdaptiveColor{Light: "#5E81AC", Dark: "#7DC4E4"}
    ColorSecondary = lipgloss.AdaptiveColor{Light: "#81A1C1", Dark: "#89DCEB"}
    ColorAccent    = lipgloss.AdaptiveColor{Light: "#BF616A", Dark: "#FF9E64"}
    ColorSuccess   = lipgloss.AdaptiveColor{Light: "#A3BE8C", Dark: "#50FA7B"}
    ColorWarning   = lipgloss.AdaptiveColor{Light: "#EBCB8B", Dark: "#FFB86C"}
    ColorError     = lipgloss.AdaptiveColor{Light: "#BF616A", Dark: "#F38BA8"}
    ColorInfo      = lipgloss.AdaptiveColor{Light: "#88C0D0", Dark: "#8BE9FD"}

    // Neutral Colors
    ColorBg       = lipgloss.AdaptiveColor{Light: "#ECEFF4", Dark: "#1E1E2E"}
    ColorBgAlt    = lipgloss.AdaptiveColor{Light: "#E5E9F0", Dark: "#282A36"}
    ColorFg       = lipgloss.AdaptiveColor{Light: "#2E3440", Dark: "#CDD6F4"}
    ColorFgSubtle = lipgloss.AdaptiveColor{Light: "#4C566A", Dark: "#6C7086"}
    ColorBorder   = lipgloss.AdaptiveColor{Light: "#D8DEE9", Dark: "#33B2FF"}
    ColorBorderActive = lipgloss.AdaptiveColor{Light: "#5E81AC", Dark: "#89DCEB"}

    // Selection Colors
    ColorSelected       = lipgloss.AdaptiveColor{Light: "#88C0D0", Dark: "#FF79C6"}
    ColorSelectedBg     = lipgloss.AdaptiveColor{Light: "#D8DEE9", Dark: "#44475A"}
    ColorHover          = lipgloss.AdaptiveColor{Light: "#ECEFF4", Dark: "#313244"}
)

// Spacing constants for consistent layouts
const (
    SpacingNone   = 0
    SpacingXS     = 1
    SpacingS      = 2
    SpacingM      = 3
    SpacingL      = 4
    SpacingXL     = 6
)

// Border styles for consistent borders
var (
    BorderNormal  = lipgloss.NormalBorder()
    BorderRounded = lipgloss.RoundedBorder()
    BorderDouble  = lipgloss.DoubleBorder()
    BorderThick   = lipgloss.ThickBorder()
)

// Base Styles - Foundation for all UI components
var (
    // Container styles
    StyleContainer = lipgloss.NewStyle().
        Border(BorderRounded).
        BorderForeground(ColorBorder).
        Padding(SpacingS, SpacingM)

    StyleContainerActive = lipgloss.NewStyle().
        Border(BorderRounded).
        BorderForeground(ColorBorderActive).
        Padding(SpacingS, SpacingM)

    // Card style for host items, etc.
    StyleCard = lipgloss.NewStyle().
        Border(BorderNormal).
        BorderForeground(ColorBorder).
        Padding(SpacingXS, SpacingS).
        MarginBottom(SpacingXS)

    StyleCardSelected = lipgloss.NewStyle().
        Border(BorderNormal).
        BorderForeground(ColorSelected).
        Background(ColorSelectedBg).
        Padding(SpacingXS, SpacingS).
        MarginBottom(SpacingXS).
        Bold(true)

    StyleCardHover = lipgloss.NewStyle().
        Border(BorderNormal).
        BorderForeground(ColorBorderActive).
        Background(ColorHover).
        Padding(SpacingXS, SpacingS).
        MarginBottom(SpacingXS)
)

// Text Styles
var (
    StyleTitle = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Bold(true).
        MarginBottom(SpacingS)

    StyleSubtitle = lipgloss.NewStyle().
        Foreground(ColorSecondary).
        Bold(true).
        MarginBottom(SpacingXS)

    StyleLabel = lipgloss.NewStyle().
        Foreground(ColorFgSubtle).
        MarginRight(SpacingS)

    StyleValue = lipgloss.NewStyle().
        Foreground(ColorFg)

    StyleMuted = lipgloss.NewStyle().
        Foreground(ColorFgSubtle).
        Italic(true)

    StyleHighlight = lipgloss.NewStyle().
        Foreground(ColorAccent).
        Bold(true)

    StyleSuccess = lipgloss.NewStyle().
        Foreground(ColorSuccess).
        Bold(true)

    StyleError = lipgloss.NewStyle().
        Foreground(ColorError).
        Bold(true)

    StyleWarning = lipgloss.NewStyle().
        Foreground(ColorWarning).
        Bold(true)

    StyleInfo = lipgloss.NewStyle().
        Foreground(ColorInfo)
)

// Header and Footer Styles
var (
    StyleHeader = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Background(ColorBgAlt).
        Bold(true).
        Padding(SpacingXS, SpacingS).
        BorderBottom(true).
        BorderStyle(BorderNormal).
        BorderForeground(ColorBorder)

    StyleFooter = lipgloss.NewStyle().
        Foreground(ColorFgSubtle).
        Background(ColorBgAlt).
        Padding(SpacingXS, SpacingS).
        BorderTop(true).
        BorderStyle(BorderNormal).
        BorderForeground(ColorBorder)

    StyleStatusBar = lipgloss.NewStyle().
        Foreground(ColorFg).
        Background(ColorBgAlt).
        Padding(SpacingXS, SpacingS)
)

// Panel Styles for dual-pane views
var (
    StylePanel = lipgloss.NewStyle().
        Border(BorderNormal).
        BorderForeground(ColorBorder).
        Padding(SpacingS)

    StylePanelActive = lipgloss.NewStyle().
        Border(BorderDouble).
        BorderForeground(ColorBorderActive).
        Padding(SpacingS)

    StylePanelTitle = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Bold(true).
        Padding(SpacingXS, SpacingS).
        Background(ColorBgAlt)

    StylePanelTitleActive = lipgloss.NewStyle().
        Foreground(ColorBorderActive).
        Bold(true).
        Padding(SpacingXS, SpacingS).
        Background(ColorSelectedBg)
)

// List Item Styles
var (
    StyleListItem = lipgloss.NewStyle().
        Padding(SpacingXS, SpacingS).
        MarginBottom(0)

    StyleListItemSelected = lipgloss.NewStyle().
        Foreground(ColorSelected).
        Background(ColorSelectedBg).
        Padding(SpacingXS, SpacingS).
        MarginBottom(0).
        Bold(true)

    StyleListItemHover = lipgloss.NewStyle().
        Background(ColorHover).
        Padding(SpacingXS, SpacingS).
        MarginBottom(0)
)

// Button Styles
var (
    StyleButton = lipgloss.NewStyle().
        Foreground(ColorFg).
        Background(ColorBgAlt).
        Padding(SpacingXS, SpacingM).
        Border(BorderRounded).
        BorderForeground(ColorBorder)

    StyleButtonActive = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Background(ColorBgAlt).
        Padding(SpacingXS, SpacingM).
        Border(BorderRounded).
        BorderForeground(ColorBorderActive).
        Bold(true)

    StyleButtonDisabled = lipgloss.NewStyle().
        Foreground(ColorFgSubtle).
        Background(ColorBgAlt).
        Padding(SpacingXS, SpacingM).
        Border(BorderRounded).
        BorderForeground(ColorBorder)
)

// Input Styles
var (
    StyleInput = lipgloss.NewStyle().
        Foreground(ColorFg).
        Background(ColorBg).
        Border(BorderNormal).
        BorderForeground(ColorBorder).
        Padding(SpacingXS, SpacingS)

    StyleInputFocused = lipgloss.NewStyle().
        Foreground(ColorFg).
        Background(ColorBg).
        Border(BorderNormal).
        BorderForeground(ColorBorderActive).
        Padding(SpacingXS, SpacingS)
)

// Dialog/Popup Styles
var (
    StyleDialog = lipgloss.NewStyle().
        Border(BorderRounded).
        BorderForeground(ColorBorderActive).
        Padding(SpacingM).
        Background(ColorBg)

    StyleDialogTitle = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Bold(true).
        Align(lipgloss.Center).
        MarginBottom(SpacingS)

    StyleDialogContent = lipgloss.NewStyle().
        Foreground(ColorFg).
        MarginBottom(SpacingS)

    StyleDialogFooter = lipgloss.NewStyle().
        Foreground(ColorFgSubtle).
        Italic(true).
        Align(lipgloss.Center)
)

// File/Directory Styles
var (
    StyleDirectory = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Bold(true)

    StyleFile = lipgloss.NewStyle().
        Foreground(ColorFg)

    StyleExecutable = lipgloss.NewStyle().
        Foreground(ColorSuccess)

    StyleSymlink = lipgloss.NewStyle().
        Foreground(ColorInfo)

    StyleArchive = lipgloss.NewStyle().
        Foreground(ColorWarning)

    StyleImage = lipgloss.NewStyle().
        Foreground(ColorAccent)

    StyleDocument = lipgloss.NewStyle().
        Foreground(ColorSecondary)
)

// Help and Hint Styles
var (
    StyleHelp = lipgloss.NewStyle().
        Foreground(ColorFgSubtle).
        Italic(true).
        MarginTop(SpacingS)

    StyleHint = lipgloss.NewStyle().
        Foreground(ColorInfo).
        Italic(true)

    StyleKeybinding = lipgloss.NewStyle().
        Foreground(ColorAccent).
        Bold(true)
)

// Helper functions for responsive layouts
func CalculateColumnWidth(totalWidth, columns int) int {
    if columns <= 0 {
        return totalWidth
    }
    spacing := SpacingM * (columns - 1)
    return (totalWidth - spacing) / columns
}

func MaxWidth(width, maxWidth int) int {
    if width > maxWidth {
        return maxWidth
    }
    return width
}

func MinWidth(width, minWidth int) int {
    if width < minWidth {
        return minWidth
    }
    return width
}

// Truncate text with ellipsis
func TruncateText(text string, maxLen int) string {
    if len(text) <= maxLen {
        return text
    }
    if maxLen <= 3 {
        return text[:maxLen]
    }
    return text[:maxLen-3] + "..."
}

// Center text within a width
func CenterText(text string, width int) string {
    return lipgloss.PlaceHorizontal(width, lipgloss.Center, text)
}

// Apply theme colors to the design system
func ApplyCurrentTheme() {
    theme := getCurrentTheme()
    
    // Update adaptive colors with theme
    ColorPrimary = lipgloss.AdaptiveColor{Light: string(theme.Highlight), Dark: string(theme.Highlight)}
    ColorSecondary = lipgloss.AdaptiveColor{Light: string(theme.ItemColor), Dark: string(theme.ItemColor)}
    ColorAccent = lipgloss.AdaptiveColor{Light: string(theme.Special), Dark: string(theme.Special)}
    ColorSuccess = lipgloss.AdaptiveColor{Light: "#A3BE8C", Dark: "#50FA7B"}
    ColorError = lipgloss.AdaptiveColor{Light: string(theme.Error), Dark: string(theme.Error)}
    ColorBorder = lipgloss.AdaptiveColor{Light: string(theme.Border), Dark: string(theme.Border)}
    ColorBorderActive = lipgloss.AdaptiveColor{Light: string(theme.Highlight), Dark: string(theme.Highlight)}
    ColorSelected = lipgloss.AdaptiveColor{Light: string(theme.SelectedFileColor), Dark: string(theme.SelectedFileColor)}
    
    // Rebuild styles with new colors
    rebuildStyles()
}

// getCurrentTheme gets the current theme from themes.go
func getCurrentTheme() Theme {
    if currentThemeIndex >= 0 && currentThemeIndex < len(themes) {
        return themes[currentThemeIndex]
    }
    return themes[0]
}

func rebuildStyles() {
    // Rebuild all styles with current theme colors
    StyleTitle = StyleTitle.Foreground(ColorPrimary)
    StyleSubtitle = StyleSubtitle.Foreground(ColorSecondary)
    StyleHighlight = StyleHighlight.Foreground(ColorAccent)
    StyleSuccess = StyleSuccess.Foreground(ColorSuccess)
    StyleError = StyleError.Foreground(ColorError)
    StyleCardSelected = StyleCardSelected.BorderForeground(ColorSelected).Background(ColorSelectedBg)
    StyleContainerActive = StyleContainerActive.BorderForeground(ColorBorderActive)
    StylePanelActive = StylePanelActive.BorderForeground(ColorBorderActive)
}
