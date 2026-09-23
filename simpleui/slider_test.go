package simpleui

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestSliderQuantizesAndClamps(t *testing.T) {
	slider := NewSlider("gain", 0, 0, 200, 30, 0, 100, 50)
	slider.SetStep(5)
	slider.SetValue(53)
	closeTo(t, slider.Value(), 55)
	slider.SetValue(500)
	closeTo(t, slider.Value(), 100)
}

func TestDisabledSliderIgnoresInput(t *testing.T) {
	slider := NewSlider("gain", 0, 0, 200, 30, 0, 100, 50)
	slider.SetEnabled(false)
	slider.Update(Input{Pointer: rl.Vector2{X: 190, Y: 15}, PointerInCanvas: true, Pressed: true, Down: true})
	closeTo(t, slider.Value(), 50)
}

func TestVerticalSliderMapsTopToMaximum(t *testing.T) {
	slider := NewSlider("vertical", 0, 0, 30, 200, 0, 10, 0)
	slider.SetOrientation(Vertical)
	slider.Update(Input{Pointer: rl.Vector2{X: 15, Y: 10}, PointerInCanvas: true, Pressed: true, Down: true})
	closeTo(t, slider.Value(), 10)
}

func TestSliderWheelChangesAndCommitsValue(t *testing.T) {
	slider := NewSlider("rf-gain", 0, 0, 200, 30, 0, 9, 4)
	changed, committed := float32(-1), float32(-1)
	slider.OnChange(func(value float32) { changed = value })
	slider.OnRelease(func(value float32) { committed = value })

	slider.Update(Input{Pointer: rl.Vector2{X: 100, Y: 15}, PointerInCanvas: true, Wheel: 1})

	closeTo(t, slider.Value(), 5)
	closeTo(t, changed, 5)
	closeTo(t, committed, 5)
}

func TestSliderWheelDoesNotCommitAtRangeLimit(t *testing.T) {
	slider := NewSlider("rf-gain", 0, 0, 200, 30, 0, 9, 9)
	commits := 0
	slider.OnRelease(func(float32) { commits++ })

	slider.Update(Input{Pointer: rl.Vector2{X: 100, Y: 15}, PointerInCanvas: true, Wheel: 1})

	if commits != 0 {
		t.Fatalf("wheel committed %d unchanged values, want 0", commits)
	}
}

func TestRangeSliderEnforcesMinimumGap(t *testing.T) {
	slider := NewRangeSlider("range", 0, 0, 300, 30, -140, 0, -100, -20)
	slider.SetMinimumGap(10)
	slider.SetValues(-25, -20)
	low, high := slider.Values()
	if high-low < 10 {
		t.Fatalf("range gap is %.2f, want at least 10", high-low)
	}
}

func TestRangeSliderMovesWholeInterval(t *testing.T) {
	slider := NewRangeSlider("range", 0, 0, 300, 30, 0, 100, 20, 60)
	slider.SetStep(5)
	slider.moveInterval(30, false)
	low, high := slider.Values()
	closeTo(t, low, 50)
	closeTo(t, high, 90)
}
