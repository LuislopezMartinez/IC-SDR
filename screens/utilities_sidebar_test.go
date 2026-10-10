package screens

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestCollapsedToolBoundsUseReleasedSidebarSpace(t *testing.T) {
	expanded := rl.Rectangle{X: toolContentX, Y: toolY, Width: toolContentRight - toolContentX, Height: 40}
	collapsed := collapsedToolBounds(expanded)
	if collapsed.X != collapsedToolContentX {
		t.Fatalf("collapsed workspace starts at %.2f, want %.2f", collapsed.X, collapsedToolContentX)
	}
	if math.Abs(float64(collapsed.X+collapsed.Width-toolContentRight)) > .01 {
		t.Fatalf("collapsed workspace ends at %.2f, want %.2f", collapsed.X+collapsed.Width, toolContentRight)
	}
	restored := expandedToolBounds(collapsed)
	if math.Abs(float64(restored.X-expanded.X)) > .01 || math.Abs(float64(restored.Width-expanded.Width)) > .01 {
		t.Fatalf("workspace bounds did not round-trip: got %+v want %+v", restored, expanded)
	}
}

func TestCollapsedSidebarLeavesCompactOperationalRail(t *testing.T) {
	if utilitiesCollapsedRight >= utilitiesRight {
		t.Fatal("collapsed sidebar must release horizontal workspace")
	}
	if collapsedToolContentX != utilitiesCollapsedRight+10 {
		t.Fatal("collapsed tool content does not preserve the sidebar gutter")
	}
}
