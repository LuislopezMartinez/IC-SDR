package simpleui

import rl "github.com/gen2brain/raylib-go/raylib"

type Button struct {
	BaseElement
	label                         string
	fontSize                      int32
	font                          FontStyle
	hovered                       bool
	pressed                       bool
	onClick                       func()
	customColors                  bool
	background, border, textColor rl.Color
	menuIcon                      bool
}

func NewButton(id string, x, y, width, height float32, label string, fontSize int32) *Button {
	return &Button{
		BaseElement: NewBaseElement(id, x, y, width, height),
		label:       label,
		fontSize:    fontSize,
		font:        FontSemiBold,
	}
}

func (button *Button) Label() string            { return button.label }
func (button *Button) SetLabel(label string)    { button.label = label }
func (button *Button) OnClick(handler func())   { button.onClick = handler }
func (button *Button) Hovered() bool            { return button.hovered }
func (button *Button) SetFont(style FontStyle)  { button.font = style }
func (button *Button) SetMenuIcon(enabled bool) { button.menuIcon = enabled }
func (button *Button) SetColors(background, border, text rl.Color) {
	button.customColors = true
	button.background, button.border, button.textColor = background, border, text
}
func (button *Button) ClearColors() { button.customColors = false }

func (button *Button) Update(input Input) bool {
	if !button.Enabled() {
		button.hovered = false
		button.pressed = false
		return false
	}
	button.hovered = input.Over(button.Bounds())
	if input.Pressed && button.hovered {
		button.pressed = true
	}
	if input.Released {
		clicked := button.pressed && button.hovered
		button.pressed = false
		if clicked && button.onClick != nil {
			PlayActivationFeedback()
			button.onClick()
		}
	}
	return button.hovered || button.pressed
}

func (button *Button) CapturingPointer() bool { return button.pressed }

func (button *Button) Draw() {
	theme := currentTheme
	color := theme.Control
	borderColor := theme.Border
	textColor := theme.Text
	if button.customColors {
		color, borderColor, textColor = themedColor(button.background), themedColor(button.border), themedColor(button.textColor)
	}
	if !button.Enabled() {
		color = theme.ControlDisabled
	} else if button.pressed {
		if button.customColors {
			color = brighten(color, 30)
		} else {
			color = theme.ControlPressed
		}
	} else if button.hovered {
		if button.customColors {
			color = brighten(color, 16)
		} else {
			color = theme.ControlHover
		}
	}
	if button.customColors && button.Enabled() {
		textColor = EnsureTextContrast(textColor, color)
	}

	bounds := button.Bounds()
	rl.DrawRectangleRounded(bounds, theme.CornerRadius, 8, color)
	rl.DrawRectangleRoundedLinesEx(bounds, theme.CornerRadius, 8, theme.BorderWidth, borderColor)
	textSize := MeasureTextStyled(button.label, button.fontSize, button.font)
	contentWidth := textSize.X
	if button.menuIcon {
		contentWidth += 24
	}
	x := bounds.X + (bounds.Width-contentWidth)*0.5
	y := bounds.Y + (bounds.Height-textSize.Y)*0.5
	if !button.Enabled() {
		textColor = theme.TextMuted
	}
	if button.menuIcon {
		iconY := bounds.Y + bounds.Height*.5 - 7
		for row := float32(0); row < 3; row++ {
			lineY := iconY + row*7
			rl.DrawLineEx(rl.Vector2{X: x, Y: lineY}, rl.Vector2{X: x + 15, Y: lineY}, 2.5, textColor)
		}
		x += 24
	}
	DrawTextStyled(button.label, x, y, button.fontSize, button.font, textColor)
}
