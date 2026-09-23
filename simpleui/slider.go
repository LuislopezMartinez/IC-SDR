package simpleui

type Slider struct {
	BaseElement
	minimum, maximum  float32
	step, value       float32
	orientation       Orientation
	hovered, dragging bool
	disabledText      string
	onChange          func(float32)
	onRelease         func(float32)
}

func NewSlider(id string, x, y, width, height, minimum, maximum, value float32) *Slider {
	minimum, maximum, step := normalizeRange(minimum, maximum, 1)
	return &Slider{
		BaseElement: NewBaseElement(id, x, y, width, height),
		minimum:     minimum, maximum: maximum, step: step,
		value: quantize(value, minimum, maximum, step),
	}
}

func (slider *Slider) Value() float32 { return slider.value }

func (slider *Slider) SetValue(value float32) {
	slider.setValue(value, false)
}

func (slider *Slider) SetStep(step float32) {
	_, _, slider.step = normalizeRange(slider.minimum, slider.maximum, step)
	slider.SetValue(slider.value)
}

// SetRange changes the selectable interval and clamps the current value.
// It does not emit OnChange; callers can update dependent state atomically.
func (slider *Slider) SetRange(minimum, maximum float32) {
	minimum, maximum, step := normalizeRange(minimum, maximum, slider.step)
	slider.minimum, slider.maximum, slider.step = minimum, maximum, step
	slider.SetValue(slider.value)
}

func (slider *Slider) SetOrientation(orientation Orientation) { slider.orientation = orientation }
func (slider *Slider) SetDisabledText(text string)            { slider.disabledText = text }
func (slider *Slider) OnChange(handler func(float32))         { slider.onChange = handler }
func (slider *Slider) OnRelease(handler func(float32))        { slider.onRelease = handler }
func (slider *Slider) CapturingPointer() bool                 { return slider.dragging }

func (slider *Slider) SetEnabled(enabled bool) {
	slider.BaseElement.SetEnabled(enabled)
	if !enabled {
		slider.dragging = false
		slider.hovered = false
	}
}

func (slider *Slider) Update(input Input) bool {
	if !slider.Enabled() {
		slider.dragging = false
		slider.hovered = false
		return false
	}
	slider.hovered = input.Over(slider.Bounds())
	if input.Pressed && slider.hovered {
		slider.dragging = true
		slider.setValue(sliderValue(slider.Bounds(), slider.orientation, input.Pointer, slider.minimum, slider.maximum), true)
	}
	if slider.dragging && input.Down {
		slider.setValue(sliderValue(slider.Bounds(), slider.orientation, input.Pointer, slider.minimum, slider.maximum), true)
	}
	if slider.dragging && input.Released {
		slider.setValue(sliderValue(slider.Bounds(), slider.orientation, input.Pointer, slider.minimum, slider.maximum), true)
		slider.dragging = false
		if slider.onRelease != nil {
			slider.onRelease(slider.value)
		}
	}
	if slider.hovered && input.Wheel != 0 {
		previous := slider.value
		slider.setValue(slider.value+input.Wheel*slider.step, true)
		// A wheel step is a complete, discrete edit. Unlike pointer dragging it
		// has no later mouse-button release, so commit it immediately for
		// controls that defer hardware updates until OnRelease.
		if slider.value != previous && slider.onRelease != nil {
			slider.onRelease(slider.value)
		}
	}
	return slider.hovered || slider.dragging
}

func (slider *Slider) Draw() {
	bounds := slider.Bounds()
	start, _, radius := sliderGeometry(bounds, slider.orientation)
	handle := sliderPosition(bounds, slider.orientation, slider.value, slider.minimum, slider.maximum)
	drawSliderTrack(bounds, slider.orientation, start, handle, slider.Enabled())
	drawSliderHandle(handle, radius, slider.dragging, slider.Enabled())
	if !slider.Enabled() && slider.disabledText != "" {
		DrawText(slider.disabledText, bounds.X, bounds.Y-18, 14, currentTheme.TextMuted)
	}
}

func (slider *Slider) setValue(value float32, notify bool) {
	next := quantize(value, slider.minimum, slider.maximum, slider.step)
	if next == slider.value {
		return
	}
	slider.value = next
	if notify && slider.onChange != nil {
		slider.onChange(next)
	}
}
