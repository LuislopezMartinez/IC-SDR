package simpleui

import rl "github.com/gen2brain/raylib-go/raylib"

type TextAlignment uint8

const (
	AlignLeft TextAlignment = iota
	AlignCenter
	AlignRight
)

type Label struct {
	BaseElement
	text        string
	fontSize    int32
	color       rl.Color
	customColor bool
	alignment   TextAlignment
	font        FontStyle
}

func NewLabel(id string, x, y, width, height float32, text string, fontSize int32) *Label {
	return &Label{
		BaseElement: NewBaseElement(id, x, y, width, height),
		text:        text,
		fontSize:    fontSize,
		alignment:   AlignLeft,
	}
}

func (label *Label) Update(Input) bool   { return false }
func (label *Label) Text() string        { return label.text }
func (label *Label) SetText(text string) { label.text = text }
func (label *Label) SetColor(color rl.Color) {
	label.color = color
	label.customColor = true
}
func (label *Label) SetAlignment(value TextAlignment) { label.alignment = value }
func (label *Label) SetFont(style FontStyle)          { label.font = style }

func (label *Label) Draw() {
	bounds := label.Bounds()
	textSize := MeasureTextStyled(label.text, label.fontSize, label.font)
	x := bounds.X
	switch label.alignment {
	case AlignCenter:
		x += (bounds.Width - textSize.X) * 0.5
	case AlignRight:
		x += bounds.Width - textSize.X
	}
	y := bounds.Y + (bounds.Height-textSize.Y)*0.5
	color := currentTheme.Text
	if label.customColor {
		color = label.color
	}
	if !label.Enabled() {
		color = currentTheme.TextMuted
	}
	DrawTextStyled(label.text, x, y, label.fontSize, label.font, color)
}
