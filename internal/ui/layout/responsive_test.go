package layout_test

import (
	"testing"

	"github.com/setupforge/setupforge/internal/ui/layout"
)

func TestCalculate_Breakpoints(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		overlay      bool
		expectedTier layout.BreakpointClass
		expectedRail int
	}{
		{
			name:         "Small 80x24",
			width:        80,
			height:       24,
			overlay:      false,
			expectedTier: layout.ClassS,
			expectedRail: 0,
		},
		{
			name:         "Medium 100x30",
			width:        100,
			height:       30,
			overlay:      false,
			expectedTier: layout.ClassM,
			expectedRail: 14,
		},
		{
			name:         "Large 120x35",
			width:        120,
			height:       35,
			overlay:      false,
			expectedTier: layout.ClassL,
			expectedRail: 18,
		},
		{
			name:         "Extra Large 160x45",
			width:        160,
			height:       45,
			overlay:      false,
			expectedTier: layout.ClassXL,
			expectedRail: 18,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := layout.Calculate(tc.width, tc.height, tc.overlay)
			if m.Class != tc.expectedTier {
				t.Errorf("expected tier %s, got %s", tc.expectedTier, m.Class)
			}
			if m.RailWidth != tc.expectedRail {
				t.Errorf("expected rail width %d, got %d", tc.expectedRail, m.RailWidth)
			}
			if m.HeaderHeight != 1 {
				t.Errorf("expected header height 1, got %d", m.HeaderHeight)
			}
			if m.DockHeight != 1 {
				t.Errorf("expected dock height 1, got %d", m.DockHeight)
			}
			if m.StageHeight != tc.height-2 {
				t.Errorf("expected stage height %d, got %d", tc.height-2, m.StageHeight)
			}
			if m.StageWidth != tc.width-tc.expectedRail {
				t.Errorf("expected stage width %d, got %d", tc.width-tc.expectedRail, m.StageWidth)
			}
		})
	}
}

func TestCalculate_StageMath(t *testing.T) {
	m := layout.Calculate(120, 35, false)

	// In Large mode, rail is 18 cols, stage is 120 - 18 = 102 cols
	if m.StageWidth != 102 {
		t.Errorf("expected stage width 102, got %d", m.StageWidth)
	}

	// Half pane width should be 51 cols
	if m.HalfPaneWidth() != 51 {
		t.Errorf("expected half pane width 51, got %d", m.HalfPaneWidth())
	}

	// Inspector pane width should be complementary: 102 - 51 = 51 cols
	if m.InspectorPaneWidth() != 51 {
		t.Errorf("expected inspector pane width 51, got %d", m.InspectorPaneWidth())
	}

	if m.HalfPaneWidth()+m.InspectorPaneWidth() != m.StageWidth {
		t.Errorf("sum of half + inspector (%d) does not match stage width (%d)",
			m.HalfPaneWidth()+m.InspectorPaneWidth(), m.StageWidth)
	}
}
