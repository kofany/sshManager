package components

import (
    "fmt"
    "sshManager/internal/ui"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// ListItem represents an item that can be displayed in a list
type ListItem interface {
    ID() string
    Title() string
    Description() string
    IsSelectable() bool
}

// ListStyle defines styling for a list component
type ListStyle struct {
    ItemStyle        lipgloss.Style
    SelectedStyle    lipgloss.Style
    TitleStyle       lipgloss.Style
    DescriptionStyle lipgloss.Style
}

// DefaultListStyle returns the default list styling
func DefaultListStyle() ListStyle {
    return ListStyle{
        ItemStyle:        ui.StyleListItem,
        SelectedStyle:    ui.StyleListItemSelected,
        TitleStyle:       ui.StyleValue,
        DescriptionStyle: ui.StyleMuted,
    }
}

// List is a reusable list component with mouse support
type List struct {
    items           []ListItem
    selectedIndex   int
    scrollOffset    int
    maxVisible      int
    style           ListStyle
    regionManager   *ClickableRegionManager
    width           int
    height          int
    offsetY         int // Y offset for rendering
    title           string
    showBorder      bool
}

// NewList creates a new list component
func NewList(title string, width, height int) *List {
    return &List{
        items:         make([]ListItem, 0),
        selectedIndex: 0,
        scrollOffset:  0,
        maxVisible:    height - 4, // Account for borders and title
        style:         DefaultListStyle(),
        regionManager: NewClickableRegionManager(),
        width:         width,
        height:        height,
        title:         title,
        showBorder:    true,
    }
}

// SetItems sets the items in the list
func (l *List) SetItems(items []ListItem) {
    l.items = items
    if l.selectedIndex >= len(items) {
        l.selectedIndex = max(0, len(items)-1)
    }
    l.updateScroll()
}

// GetItems returns the list items
func (l *List) GetItems() []ListItem {
    return l.items
}

// SetSelected sets the selected index
func (l *List) SetSelected(index int) {
    if index >= 0 && index < len(l.items) {
        l.selectedIndex = index
        l.updateScroll()
    }
}

// GetSelected returns the selected index
func (l *List) GetSelected() int {
    return l.selectedIndex
}

// GetSelectedItem returns the currently selected item
func (l *List) GetSelectedItem() ListItem {
    if l.selectedIndex >= 0 && l.selectedIndex < len(l.items) {
        return l.items[l.selectedIndex]
    }
    return nil
}

// SelectNext moves selection down
func (l *List) SelectNext() {
    if len(l.items) > 0 {
        l.selectedIndex = (l.selectedIndex + 1) % len(l.items)
        l.updateScroll()
    }
}

// SelectPrev moves selection up
func (l *List) SelectPrev() {
    if len(l.items) > 0 {
        l.selectedIndex--
        if l.selectedIndex < 0 {
            l.selectedIndex = len(l.items) - 1
        }
        l.updateScroll()
    }
}

// updateScroll updates scroll offset based on selected item
func (l *List) updateScroll() {
    if l.selectedIndex < l.scrollOffset {
        l.scrollOffset = l.selectedIndex
    } else if l.selectedIndex >= l.scrollOffset+l.maxVisible {
        l.scrollOffset = l.selectedIndex - l.maxVisible + 1
    }
}

// HandleMouse processes mouse events for the list
func (l *List) HandleMouse(msg tea.MouseMsg) (activated bool, cmd tea.Cmd) {
    if msg.Type != tea.MouseLeft {
        return false, nil
    }
    
    regionID, activated := l.regionManager.HandleMouseClick(msg)
    if regionID != "" {
        // Find the item by ID and select it
        for i, item := range l.items {
            if item.ID() == regionID {
                l.selectedIndex = i
                l.updateScroll()
                return activated, nil
            }
        }
    }
    
    return false, nil
}

// View renders the list
func (l *List) View() string {
    if len(l.items) == 0 {
        emptyMsg := ui.StyleMuted.Render("No items")
        if l.showBorder {
            container := ui.StyleContainer.
                Width(l.width - 4).
                Height(l.height - 2).
                Align(lipgloss.Center, lipgloss.Center)
            return container.Render(emptyMsg)
        }
        return emptyMsg
    }
    
    // Clear regions and rebuild them
    l.regionManager.ClearRegions()
    
    var content strings.Builder
    
    // Add title if present
    if l.title != "" {
        titleLine := ui.StyleSubtitle.Width(l.width - 4).Render(l.title)
        content.WriteString(titleLine + "\n\n")
    }
    
    // Calculate visible range
    endIndex := min(l.scrollOffset+l.maxVisible, len(l.items))
    visibleItems := l.items[l.scrollOffset:endIndex]
    
    // Render items
    currentY := l.offsetY + 2 // Account for border and title
    if l.title != "" {
        currentY += 2
    }
    
    for i, item := range visibleItems {
        actualIndex := l.scrollOffset + i
        isSelected := actualIndex == l.selectedIndex
        
        // Render item
        itemContent := l.renderItem(item, isSelected)
        content.WriteString(itemContent + "\n")
        
        // Register clickable region
        itemHeight := strings.Count(itemContent, "\n") + 1
        l.regionManager.AddRegion(ClickableRegion{
            X:      0,
            Y:      currentY,
            Width:  l.width,
            Height: itemHeight,
            ID:     item.ID(),
        })
        
        currentY += itemHeight
    }
    
    // Add scroll indicator if needed
    if len(l.items) > l.maxVisible {
        scrollInfo := fmt.Sprintf("  %d-%d of %d", 
            l.scrollOffset+1, 
            min(l.scrollOffset+l.maxVisible, len(l.items)), 
            len(l.items))
        content.WriteString("\n" + ui.StyleMuted.Render(scrollInfo))
    }
    
    // Apply container style if border is shown
    if l.showBorder {
        container := ui.StyleContainer.
            Width(l.width - 4).
            Height(l.height - 2)
        return container.Render(content.String())
    }
    
    return content.String()
}

// renderItem renders a single list item
func (l *List) renderItem(item ListItem, isSelected bool) string {
    var style lipgloss.Style
    if isSelected {
        style = l.style.SelectedStyle
    } else {
        style = l.style.ItemStyle
    }
    
    // Build item content
    title := l.style.TitleStyle.Render(item.Title())
    desc := ""
    if item.Description() != "" {
        desc = "\n" + l.style.DescriptionStyle.Render("  " + item.Description())
    }
    
    itemContent := title + desc
    return style.Width(l.width - 6).Render(itemContent)
}

// SetOffsetY sets the Y offset for rendering (used for positioning in layouts)
func (l *List) SetOffsetY(offset int) {
    l.offsetY = offset
}

// SetSize updates the list dimensions
func (l *List) SetSize(width, height int) {
    l.width = width
    l.height = height
    l.maxVisible = height - 4
    if l.title != "" {
        l.maxVisible -= 2
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}
