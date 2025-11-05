package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MouseTracker tracks mouse clicks for two-click behavior
// First click selects, second click activates
type MouseTracker struct {
	lastClickX      int
	lastClickY      int
	lastClickTime   time.Time
	lastClickItemID string
	doubleClickTime time.Duration
}

// NewMouseTracker creates a new mouse tracker
func NewMouseTracker() *MouseTracker {
	return &MouseTracker{
		doubleClickTime: 500 * time.Millisecond,
	}
}

// HandleClick processes a mouse click and returns whether it's a selection or activation
// Returns: (isSelection, isActivation, itemID)
func (m *MouseTracker) HandleClick(x, y int, itemID string) (isSelection bool, isActivation bool) {
	now := time.Now()
	
	// Check if this is a double-click on the same item
	if itemID == m.lastClickItemID && 
	   x == m.lastClickX && 
	   y == m.lastClickY &&
	   now.Sub(m.lastClickTime) < m.doubleClickTime {
		// Second click - activation
		m.lastClickItemID = ""
		return false, true
	}
	
	// First click - selection
	m.lastClickX = x
	m.lastClickY = y
	m.lastClickTime = now
	m.lastClickItemID = itemID
	return true, false
}

// Reset resets the click tracking
func (m *MouseTracker) Reset() {
	m.lastClickItemID = ""
	m.lastClickTime = time.Time{}
}

// ClickableRegion represents a clickable area in the UI
type ClickableRegion struct {
	X      int
	Y      int
	Width  int
	Height int
	ID     string
	Action func()
}

// Contains checks if a point is within the region
func (r *ClickableRegion) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width &&
	       y >= r.Y && y < r.Y+r.Height
}

// ClickableRegionManager manages multiple clickable regions
type ClickableRegionManager struct {
	regions      []ClickableRegion
	mouseTracker *MouseTracker
}

// NewClickableRegionManager creates a new region manager
func NewClickableRegionManager() *ClickableRegionManager {
	return &ClickableRegionManager{
		regions:      make([]ClickableRegion, 0),
		mouseTracker: NewMouseTracker(),
	}
}

// AddRegion adds a clickable region
func (m *ClickableRegionManager) AddRegion(region ClickableRegion) {
	m.regions = append(m.regions, region)
}

// ClearRegions removes all regions
func (m *ClickableRegionManager) ClearRegions() {
	m.regions = make([]ClickableRegion, 0)
}

// HandleMouseClick processes a mouse click event
// Returns the ID of the clicked region and whether it was activated
func (m *ClickableRegionManager) HandleMouseClick(msg tea.MouseMsg) (regionID string, activated bool) {
	x, y := msg.X, msg.Y
	
	// Find the region that was clicked
	for _, region := range m.regions {
		if region.Contains(x, y) {
			isSelection, isActivation := m.mouseTracker.HandleClick(x, y, region.ID)
			
			if isActivation {
				// Execute the action if provided
				if region.Action != nil {
					region.Action()
				}
				return region.ID, true
			} else if isSelection {
				return region.ID, false
			}
		}
	}
	
	return "", false
}

// GetRegionByID returns a region by its ID
func (m *ClickableRegionManager) GetRegionByID(id string) *ClickableRegion {
	for i := range m.regions {
		if m.regions[i].ID == id {
			return &m.regions[i]
		}
	}
	return nil
}
