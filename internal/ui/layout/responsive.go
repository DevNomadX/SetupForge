package layout

// BreakpointClass represents the four responsive tier categories.
type BreakpointClass string

const (
	ClassS  BreakpointClass = "S"  // Small: 80×24 minimum usable
	ClassM  BreakpointClass = "M"  // Medium: 100×30
	ClassL  BreakpointClass = "L"  // Large: 120×35 baseline standard
	ClassXL BreakpointClass = "XL" // Extra Large: 160×45+
)

// Metrics holds computed dimensions and layout modes based on terminal geometry.
type Metrics struct {
	TotalWidth  int
	TotalHeight int

	Class BreakpointClass

	// Region allocations
	HeaderHeight int
	DockHeight   int
	RailWidth    int
	StageWidth   int
	StageHeight  int

	// Macro M-unit grid (1M = 1/4 of stage dimension)
	ModuleW int
	ModuleH int

	// Mode flags
	IsSmall      bool
	IsMedium     bool
	IsLarge      bool
	IsExtraLarge bool

	RailOverlay bool // In small mode, rail is toggled as an overlay rather than consuming stage columns
}

// Calculate computes responsive layout allocations according to docs/UI-UX-DESIGN.md §9.
func Calculate(width, height int, railOverlayToggled bool) Metrics {
	m := Metrics{
		TotalWidth:   width,
		TotalHeight:  height,
		HeaderHeight: 1,
		DockHeight:   1,
	}

	// Classify breakpoint
	if width < 100 || height < 30 {
		m.Class = ClassS
		m.IsSmall = true
		m.RailWidth = 0
		m.RailOverlay = railOverlayToggled
	} else if width < 120 {
		m.Class = ClassM
		m.IsMedium = true
		m.RailWidth = 14
	} else if width < 160 || height < 45 {
		m.Class = ClassL
		m.IsLarge = true
		m.RailWidth = 18
	} else {
		m.Class = ClassXL
		m.IsExtraLarge = true
		m.RailWidth = 18
	}

	// Calculate stage dimensions
	m.StageHeight = m.TotalHeight - m.HeaderHeight - m.DockHeight
	if m.StageHeight < 0 {
		m.StageHeight = 0
	}

	m.StageWidth = m.TotalWidth - m.RailWidth
	if m.StageWidth < 0 {
		m.StageWidth = 0
	}

	// Calculate 1M units
	m.ModuleW = m.StageWidth / 4
	m.ModuleH = m.StageHeight / 4

	return m
}

// WholePaneWidth returns 4M width (the entire stage).
func (m Metrics) WholePaneWidth() int {
	return m.StageWidth
}

// HalfPaneWidth returns 2M width.
func (m Metrics) HalfPaneWidth() int {
	if m.IsMedium {
		// In M breakpoint, 55/45 ratio is used for master/inspector
		return (m.StageWidth * 55) / 100
	}
	return m.StageWidth / 2
}

// InspectorPaneWidth returns the complementary width to HalfPaneWidth.
func (m Metrics) InspectorPaneWidth() int {
	return m.StageWidth - m.HalfPaneWidth()
}

// QuadTileWidth returns 2M width.
func (m Metrics) QuadTileWidth() int {
	return m.StageWidth / 2
}

// QuadTileHeight returns 2M height.
func (m Metrics) QuadTileHeight() int {
	return m.StageHeight / 2
}
