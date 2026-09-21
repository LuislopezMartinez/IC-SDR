package simpleui

import rl "github.com/gen2brain/raylib-go/raylib"

// Switch is a labelled boolean control.
type Switch struct {
	BaseElement
	label        string
	fontSize     int32
	active       bool
	hovered      bool
	pressed      bool
	visual       float32
	disabledText string
	font         FontStyle
	onChange     func(bool)
	customColors bool
	offColor     rl.Color
	onColor      rl.Color
}

func NewSwitch(id string, x, y, width, height float32, label string, active bool, fontSize int32) *Switch {
	visual := float32(0)
	if active {
		visual = 1
	}
	return &Switch{
		BaseElement: NewBaseElement(id, x, y, width, height),
		label:       label,
		fontSize:    fontSize,
		active:      active,
		visual:      visual,
	}
}

func (toggle *Switch) Active() bool                { return toggle.active }
func (toggle *Switch) Label() string               { return toggle.label }
func (toggle *Switch) SetLabel(label string)       { toggle.label = label }
func (toggle *Switch) SetDisabledText(text string) { toggle.disabledText = text }
func (toggle *Switch) OnChange(handler func(bool)) { toggle.onChange = handler }
func (toggle *Switch) CapturingPointer() bool      { return toggle.pressed }

// SetTrackColors customizes this switch without changing the global theme.
func (toggle *Switch) SetTrackColors(off, on rl.Color) {
	toggle.customColors = true
	toggle.offColor = off
	toggle.onColor = on
}

func (toggle *Switch) ClearTrackColors() { toggle.customColors = false }

// SetActive changes the state without emitting OnChange.
func (toggle *Switch) SetActive(active bool) {
	toggle.active = active
}

func (toggle *Switch) SetEnabled(enabled bool) {
	toggle.BaseElement.SetEnabled(enabled)
	if !enabled {
		toggle.hovered = false
		toggle.pressed = false
	}
}

func (toggle *Switch) Update(input Input) bool {
	if !toggle.Enabled() {
		toggle.hovered = false
		toggle.pressed = false
		return false
	}

	toggle.hovered = input.Over(toggle.Bounds())
	if input.Pressed && toggle.hovered {
		toggle.pressed = true
	}
	if toggle.pressed && input.Released {
		clicked := toggle.hovered
		toggle.pressed = false
		if clicked {
			toggle.active = !toggle.active
			if toggle.onChange != nil {
				toggle.onChange(toggle.active)
			}
		}
	}
	return toggle.hovered || toggle.pressed
}

func (toggle *Switch) Draw() {
	toggle.updateAnimation()
	bounds := toggle.Bounds()
	track := toggle.trackBounds()
	theme := currentTheme

	trackColor := theme.SwitchOff
	if toggle.active {
		trackColor = theme.SwitchOn
	}
	if toggle.customColors {
		trackColor = themedColor(toggle.offColor)
		if toggle.active {
			trackColor = themedColor(toggle.onColor)
		}
	}
	if !toggle.Enabled() {
		trackColor = theme.DisabledTrack
	} else if toggle.hovered {
		trackColor = brighten(trackColor, 18)
	}

	rl.DrawRectangleRounded(track, 1, 12, trackColor)
	if !toggle.Enabled() {
		drawSwitchDisabledPattern(track)
	}

	radius := track.Height*0.5 - 3
	left := track.X + track.Height*0.5
	right := track.X + track.Width - track.Height*0.5
	knob := rl.Vector2{X: left + (right-left)*toggle.visual, Y: track.Y + track.Height*0.5}
	handleColor := theme.SliderHandle
	if !toggle.Enabled() {
		handleColor = theme.DisabledHandle
	} else if toggle.pressed {
		handleColor = theme.Accent
	}
	rl.DrawCircleV(knob, radius, handleColor)
	rl.DrawCircleLines(int32(knob.X), int32(knob.Y), radius, theme.Border)

	textColor := theme.Text
	if !toggle.Enabled() {
		textColor = theme.TextMuted
	}
	textY := bounds.Y + (bounds.Height-MeasureTextStyled("Ag", toggle.fontSize, toggle.font).Y)*0.5
	DrawTextStyled(toggle.label, bounds.X, textY, toggle.fontSize, toggle.font, textColor)
	if !toggle.Enabled() && toggle.disabledText != "" {
		textWidth := MeasureText(toggle.disabledText, 12).X
		DrawText(toggle.disabledText, track.X+track.Width-textWidth, bounds.Y-15, 12, theme.TextMuted)
	}
}

func (toggle *Switch) trackBounds() rl.Rectangle {
	bounds := toggle.Bounds()
	height := min(bounds.Height*0.68, 26)
	width := min(max(height*1.8, 42), bounds.Width*0.45)
	return rl.Rectangle{
		X:      bounds.X + bounds.Width - width,
		Y:      bounds.Y + (bounds.Height-height)*0.5,
		Width:  width,
		Height: height,
	}
}

func (toggle *Switch) updateAnimation() {
	target := float32(0)
	if toggle.active {
		target = 1
	}
	speed := float32(14) * rl.GetFrameTime()
	if speed <= 0 || speed > 1 {
		speed = 1
	}
	toggle.visual += (target - toggle.visual) * speed
	if abs32(target-toggle.visual) < 0.001 {
		toggle.visual = target
	}
}

func drawSwitchDisabledPattern(track rl.Rectangle) {
	phase := float32(rl.GetTime()*12) - float32(int(rl.GetTime()*12/8))*8
	for x := track.X - track.Height - phase; x < track.X+track.Width; x += 8 {
		startX := max(track.X, x)
		endX := min(track.X+track.Width, x+track.Height)
		if endX > startX {
			rl.DrawLineEx(
				rl.Vector2{X: startX, Y: track.Y + track.Height - (startX - x)},
				rl.Vector2{X: endX, Y: track.Y + track.Height - (endX - x)},
				2,
				currentTheme.DisabledPattern,
			)
		}
	}
}

func brighten(color rl.Color, amount uint8) rl.Color {
	color.R = uint8(min(int(color.R)+int(amount), 255))
	color.G = uint8(min(int(color.G)+int(amount), 255))
	color.B = uint8(min(int(color.B)+int(amount), 255))
	return color
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
