package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"strconv"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var tuningStepsHz = []int64{1, 10, 100, 500, 1_000, 5_000, 6_250, 8_330, 10_000, 12_500, 25_000, 100_000}

// StepSelector provides direct access to every tuning raster supported by IC-SDR.
type StepSelector struct {
	simpleui.BaseElement
	open, pressedClose bool
	selected           int64
	pressed            int
	onSelect           func(int64)
	close              *simpleui.Button
}

func NewStepSelector(selected int64, onSelect func(int64)) *StepSelector {
	return &StepSelector{
		BaseElement: simpleui.NewBaseElement("stepSelectorOverlay", 0, 0, designWidth, designHeight),
		selected:    selected, pressed: -1, onSelect: onSelect,
		close: simpleui.NewButton("stepSelectorClose", 650, 515, 300, 50, i18n.Source("text.b908be5df2f1"), 17),
	}
}

func (selector *StepSelector) Open()                      { selector.open = true }
func (selector *StepSelector) SetSelected(step int64)     { selector.selected = step }
func (selector *StepSelector) OverlayOpen() bool          { return selector.open }
func (selector *StepSelector) Update(simpleui.Input) bool { return false }
func (selector *StepSelector) Draw()                      {}

func (selector *StepSelector) UpdateOverlay(input simpleui.Input) bool {
	if !selector.open {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		selector.open = false
		return true
	}
	if selector.close.Update(input) {
		if input.Released {
			selector.open = false
		}
		return true
	}
	if input.Pressed {
		selector.pressed = selector.stepAt(input.Pointer)
	}
	if input.Released {
		index := selector.stepAt(input.Pointer)
		if index >= 0 && index == selector.pressed {
			selector.selected = tuningStepsHz[index]
			selector.open = false
			if selector.onSelect != nil {
				selector.onSelect(selector.selected)
			}
		}
		selector.pressed = -1
	}
	return true
}

func (selector *StepSelector) DrawOverlay() {
	if !selector.open {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 200})
	panel := rl.Rectangle{X: 430, Y: 170, Width: 740, Height: 430}
	rl.DrawRectangleRounded(panel, .03, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(panel, .03, 8, 2, colors.blue)
	rl.DrawRectangleRounded(rl.Rectangle{X: 430, Y: 170, Width: 10, Height: 430}, .5, 8, colors.blue)
	drawCentered(i18n.Source("text.2c770106f952"), rl.Rectangle{X: 470, Y: 195, Width: 660, Height: 44}, 25, colors.text)
	for index, step := range tuningStepsHz {
		bounds := selector.stepBounds(index)
		fill := colors.panelAlt
		border := colors.border
		if step == selector.selected {
			fill = colors.blue
			border = colors.cyan
		} else if index == selector.pressed {
			fill = rl.Color{R: 70, G: 78, B: 90, A: 255}
		}
		rl.DrawRectangleRounded(bounds, .12, 8, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .12, 8, 2, border)
		drawCentered(formatStep(step), bounds, 16, simpleui.EnsureTextContrast(colors.text, fill))
	}
	selector.close.Draw()
}

func (selector *StepSelector) stepBounds(index int) rl.Rectangle {
	column, row := index%5, index/5
	return rl.Rectangle{X: 460 + float32(column)*138, Y: 280 + float32(row)*72, Width: 122, Height: 54}
}

func (selector *StepSelector) stepAt(point rl.Vector2) int {
	for index := range tuningStepsHz {
		if rl.CheckCollisionPointRec(point, selector.stepBounds(index)) {
			return index
		}
	}
	return -1
}

func formatStep(hz int64) string {
	if hz < 1_000 {
		return fmt.Sprintf(i18n.Source("text.3999a0ad05ce"), hz)
	}
	return strconv.FormatFloat(float64(hz)/1_000, 'f', -1, 64) + i18n.Source("text.acaf5a32d70a")
}
