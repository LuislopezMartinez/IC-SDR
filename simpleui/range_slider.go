package simpleui

import (
	"go-zero/internal/i18n"

	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type rangeDrag uint8

const (
	dragNone rangeDrag = iota
	dragLow
	dragHigh
	dragInterval
)

type RangeSlider struct {
	BaseElement
	minimum, maximum float32
	step, low, high  float32
	minimumGap       float32
	orientation      Orientation
	hovered          bool
	dragging         rangeDrag
	dragStartValue   float32
	dragStartLow     float32
	dragStartHigh    float32
	dragWholeRange   bool
	disabledText     string
	onChange         func(float32, float32)
	onRelease        func(float32, float32)
}

func NewRangeSlider(id string, x, y, width, height, minimum, maximum, low, high float32) *RangeSlider {
	minimum, maximum, step := normalizeRange(minimum, maximum, 1)
	slider := &RangeSlider{
		BaseElement:    NewBaseElement(id, x, y, width, height),
		minimum:        minimum,
		maximum:        maximum,
		step:           step,
		dragWholeRange: true,
	}
	slider.SetValues(low, high)
	return slider
}

func (slider *RangeSlider) Values() (float32, float32) { return slider.low, slider.high }

func (slider *RangeSlider) SetValues(low, high float32) {
	slider.setValues(low, high, false)
}

func (slider *RangeSlider) SetStep(step float32) {
	_, _, slider.step = normalizeRange(slider.minimum, slider.maximum, step)
	slider.SetValues(slider.low, slider.high)
}

func (slider *RangeSlider) SetMinimumGap(gap float32) {
	if gap < 0 || gap > slider.maximum-slider.minimum {
		panic(i18n.Source("text.a70ec4fb4218"))
	}
	slider.minimumGap = quantize(gap, 0, slider.maximum-slider.minimum, slider.step)
	slider.SetValues(slider.low, slider.high)
}

func (slider *RangeSlider) SetOrientation(orientation Orientation)   { slider.orientation = orientation }
func (slider *RangeSlider) SetRangeDragging(enabled bool)            { slider.dragWholeRange = enabled }
func (slider *RangeSlider) SetDisabledText(text string)              { slider.disabledText = text }
func (slider *RangeSlider) OnChange(handler func(float32, float32))  { slider.onChange = handler }
func (slider *RangeSlider) OnRelease(handler func(float32, float32)) { slider.onRelease = handler }
func (slider *RangeSlider) CapturingPointer() bool                   { return slider.dragging != dragNone }

func (slider *RangeSlider) SetEnabled(enabled bool) {
	slider.BaseElement.SetEnabled(enabled)
	if !enabled {
		slider.dragging = dragNone
		slider.hovered = false
	}
}

func (slider *RangeSlider) Update(input Input) bool {
	if !slider.Enabled() {
		slider.dragging = dragNone
		slider.hovered = false
		return false
	}
	slider.hovered = input.Over(slider.Bounds())
	if input.Pressed && slider.hovered {
		slider.beginDrag(input.Pointer)
	}
	if slider.dragging != dragNone && input.Down {
		slider.continueDrag(input.Pointer)
	}
	if slider.dragging != dragNone && input.Released {
		slider.continueDrag(input.Pointer)
		slider.dragging = dragNone
		if slider.onRelease != nil {
			slider.onRelease(slider.low, slider.high)
		}
	}
	if slider.hovered && input.Wheel != 0 {
		delta := input.Wheel * slider.step
		slider.moveInterval(delta, true)
	}
	return slider.hovered || slider.dragging != dragNone
}

func (slider *RangeSlider) Draw() {
	bounds := slider.Bounds()
	_, _, radius := sliderGeometry(bounds, slider.orientation)
	lowPosition := sliderPosition(bounds, slider.orientation, slider.low, slider.minimum, slider.maximum)
	highPosition := sliderPosition(bounds, slider.orientation, slider.high, slider.minimum, slider.maximum)
	drawSliderTrack(bounds, slider.orientation, lowPosition, highPosition, slider.Enabled())
	drawSliderHandle(lowPosition, radius, slider.dragging == dragLow || slider.dragging == dragInterval, slider.Enabled())
	drawSliderHandle(highPosition, radius, slider.dragging == dragHigh || slider.dragging == dragInterval, slider.Enabled())
	if !slider.Enabled() && slider.disabledText != "" {
		DrawText(slider.disabledText, bounds.X, bounds.Y-18, 14, currentTheme.TextMuted)
	}
}

func (slider *RangeSlider) beginDrag(pointer rl.Vector2) {
	bounds := slider.Bounds()
	pointerValue := sliderValue(bounds, slider.orientation, pointer, slider.minimum, slider.maximum)
	lowPosition := sliderPosition(bounds, slider.orientation, slider.low, slider.minimum, slider.maximum)
	highPosition := sliderPosition(bounds, slider.orientation, slider.high, slider.minimum, slider.maximum)
	axis := pointer.X
	lowAxis, highAxis := lowPosition.X, highPosition.X
	if slider.orientation == Vertical {
		axis, lowAxis, highAxis = pointer.Y, lowPosition.Y, highPosition.Y
	}
	left, right := min(lowAxis, highAxis), max(lowAxis, highAxis)
	_, _, radius := sliderGeometry(bounds, slider.orientation)

	if slider.dragWholeRange && axis > left+radius && axis < right-radius {
		slider.dragging = dragInterval
		slider.dragStartValue = pointerValue
		slider.dragStartLow, slider.dragStartHigh = slider.low, slider.high
		return
	}
	if math.Abs(float64(axis-lowAxis)) <= math.Abs(float64(axis-highAxis)) {
		slider.dragging = dragLow
		slider.setValues(pointerValue, slider.high, true)
	} else {
		slider.dragging = dragHigh
		slider.setValues(slider.low, pointerValue, true)
	}
}

func (slider *RangeSlider) continueDrag(pointer rl.Vector2) {
	value := sliderValue(slider.Bounds(), slider.orientation, pointer, slider.minimum, slider.maximum)
	switch slider.dragging {
	case dragLow:
		slider.setValues(value, slider.high, true)
	case dragHigh:
		slider.setValues(slider.low, value, true)
	case dragInterval:
		delta := quantize(value-slider.dragStartValue, -(slider.maximum - slider.minimum), slider.maximum-slider.minimum, slider.step)
		slider.setIntervalFromStart(delta, true)
	}
}

func (slider *RangeSlider) moveInterval(delta float32, notify bool) {
	slider.dragStartLow, slider.dragStartHigh = slider.low, slider.high
	slider.setIntervalFromStart(delta, notify)
}

func (slider *RangeSlider) setIntervalFromStart(delta float32, notify bool) {
	if slider.dragStartLow+delta < slider.minimum {
		delta = slider.minimum - slider.dragStartLow
	}
	if slider.dragStartHigh+delta > slider.maximum {
		delta = slider.maximum - slider.dragStartHigh
	}
	slider.setValues(slider.dragStartLow+delta, slider.dragStartHigh+delta, notify)
}

func (slider *RangeSlider) setValues(low, high float32, notify bool) {
	low = quantize(low, slider.minimum, slider.maximum, slider.step)
	high = quantize(high, slider.minimum, slider.maximum, slider.step)
	gap := max(slider.minimumGap, slider.step)
	if low > high-gap {
		low = high - gap
	}
	if low < slider.minimum {
		low = slider.minimum
		high = max(high, low+gap)
	}
	if high > slider.maximum {
		high = slider.maximum
		low = min(low, high-gap)
	}
	low = quantize(low, slider.minimum, slider.maximum, slider.step)
	high = quantize(high, slider.minimum, slider.maximum, slider.step)
	if low == slider.low && high == slider.high {
		return
	}
	slider.low, slider.high = low, high
	if notify && slider.onChange != nil {
		slider.onChange(low, high)
	}
}
