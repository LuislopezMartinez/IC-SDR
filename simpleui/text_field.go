package simpleui

import (
	"strings"
	"unicode"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TextField is a single-line Unicode text editor.
type TextField struct {
	BaseElement
	text          []rune
	placeholder   string
	fontSize      int32
	font          FontStyle
	cursor        int
	anchor        int
	focused       bool
	hovered       bool
	dragging      bool
	scrollX       float32
	maxLength     int
	password      bool
	filter        func(rune) bool
	originalText  string
	lastBlinkTime float64
	onChange      func(string)
	onSubmit      func(string)
	onFocusLost   func(string)
}

func NewTextField(id string, x, y, width, height float32, placeholder string, fontSize int32) *TextField {
	return &TextField{
		BaseElement: NewBaseElement(id, x, y, width, height),
		placeholder: placeholder,
		fontSize:    fontSize,
	}
}

func (field *TextField) Text() string                     { return string(field.text) }
func (field *TextField) Placeholder() string              { return field.placeholder }
func (field *TextField) SetPlaceholder(value string)      { field.placeholder = value }
func (field *TextField) SetFont(style FontStyle)          { field.font = style }
func (field *TextField) SetMaxLength(length int)          { field.maxLength = max(0, length) }
func (field *TextField) SetPassword(password bool)        { field.password = password }
func (field *TextField) SetFilter(filter func(rune) bool) { field.filter = filter }
func (field *TextField) OnChange(handler func(string))    { field.onChange = handler }
func (field *TextField) OnSubmit(handler func(string))    { field.onSubmit = handler }
func (field *TextField) OnFocusLost(handler func(string)) { field.onFocusLost = handler }
func (field *TextField) Focused() bool                    { return field.focused }
func (field *TextField) CapturingPointer() bool           { return field.dragging }

func (field *TextField) SetText(value string) {
	field.text = field.acceptedRunes(value)
	field.cursor = len(field.text)
	field.anchor = field.cursor
	field.ensureCursorVisible()
}

func (field *TextField) SetFocused(focused bool) {
	if focused == field.focused || (!field.Enabled() && focused) {
		return
	}
	field.focused = focused
	field.dragging = false
	field.lastBlinkTime = rl.GetTime()
	if focused {
		field.originalText = field.Text()
		return
	}
	field.anchor = field.cursor
	if field.onFocusLost != nil {
		field.onFocusLost(field.Text())
	}
}

func (field *TextField) SetEnabled(enabled bool) {
	if !enabled {
		field.SetFocused(false)
		field.hovered = false
		field.dragging = false
	}
	field.BaseElement.SetEnabled(enabled)
}

func (field *TextField) Update(input Input) bool {
	if !field.Enabled() {
		return false
	}
	field.hovered = input.Over(field.Bounds())
	if input.Pressed && field.hovered {
		field.SetFocused(true)
		field.cursor = field.indexAt(input.Pointer.X)
		field.anchor = field.cursor
		field.dragging = true
		field.lastBlinkTime = rl.GetTime()
	}
	if field.dragging && input.Down {
		field.cursor = field.indexAt(input.Pointer.X)
		field.ensureCursorVisible()
	}
	if field.dragging && input.Released {
		field.cursor = field.indexAt(input.Pointer.X)
		field.dragging = false
		field.ensureCursorVisible()
	}
	if field.focused {
		field.handleKeyboard()
	}
	return field.hovered || field.dragging || field.focused
}

func (field *TextField) Draw() {
	bounds := field.Bounds()
	theme := currentTheme
	background := theme.InputBackground
	border := theme.Border
	if !field.Enabled() {
		background = theme.ControlDisabled
	} else if field.focused {
		border = theme.Accent
	} else if field.hovered {
		border = brighten(border, 20)
	}
	rl.DrawRectangleRounded(bounds, theme.CornerRadius, 8, background)
	rl.DrawRectangleRoundedLinesEx(bounds, theme.CornerRadius, 8, max(theme.BorderWidth, float32(1)), border)
	if !field.Enabled() {
		drawFieldDisabledPattern(bounds)
	}

	padding := field.padding()
	clip := rl.Rectangle{X: bounds.X + padding, Y: bounds.Y + 2, Width: bounds.Width - padding*2, Height: bounds.Height - 4}
	rl.BeginScissorMode(int32(clip.X), int32(clip.Y), int32(max(0, clip.Width)), int32(max(0, clip.Height)))
	defer rl.EndScissorMode()

	textY := bounds.Y + (bounds.Height-MeasureTextStyled("Ag", field.fontSize, field.font).Y)*0.5
	textX := bounds.X + padding - field.scrollX
	if len(field.text) == 0 && !field.focused {
		DrawTextStyled(field.placeholder, textX, textY, field.fontSize, field.font, theme.TextMuted)
		return
	}

	field.drawSelection(textX, textY)
	textColor := theme.Text
	if !field.Enabled() {
		textColor = theme.TextMuted
	}
	DrawTextStyled(field.displayText(), textX, textY, field.fontSize, field.font, textColor)
	field.drawCaret(textX, textY)
}

func (field *TextField) handleKeyboard() {
	control := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
	shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)

	if control && rl.IsKeyPressed(rl.KeyA) {
		field.anchor, field.cursor = 0, len(field.text)
		field.ensureCursorVisible()
		return
	}
	if control && rl.IsKeyPressed(rl.KeyC) {
		if field.password {
			return
		}
		if selected := field.selectedText(); selected != "" {
			rl.SetClipboardText(selected)
		}
		return
	}
	if control && rl.IsKeyPressed(rl.KeyX) {
		if field.password {
			field.deleteSelection(true)
			return
		}
		if selected := field.selectedText(); selected != "" {
			rl.SetClipboardText(selected)
			field.deleteSelection(true)
		}
		return
	}
	if control && rl.IsKeyPressed(rl.KeyV) {
		field.replaceSelection(field.acceptedRunes(rl.GetClipboardText()), true)
		return
	}

	if keyPressedOrRepeated(rl.KeyLeft) {
		field.moveCursor(-1, shift)
	}
	if keyPressedOrRepeated(rl.KeyRight) {
		field.moveCursor(1, shift)
	}
	if rl.IsKeyPressed(rl.KeyHome) {
		field.moveCursorTo(0, shift)
	}
	if rl.IsKeyPressed(rl.KeyEnd) {
		field.moveCursorTo(len(field.text), shift)
	}
	if keyPressedOrRepeated(rl.KeyBackspace) {
		field.backspace()
	}
	if keyPressedOrRepeated(rl.KeyDelete) {
		field.deleteForward()
	}
	if rl.IsKeyPressed(rl.KeyEnter) {
		if field.onSubmit != nil {
			field.onSubmit(field.Text())
		}
		field.SetFocused(false)
		return
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		if field.Text() != field.originalText {
			field.text = []rune(field.originalText)
			field.cursor, field.anchor = len(field.text), len(field.text)
			field.changed()
		}
		field.SetFocused(false)
		return
	}

	if !control {
		for codepoint := rl.GetCharPressed(); codepoint > 0; codepoint = rl.GetCharPressed() {
			character := rune(codepoint)
			if unicode.IsControl(character) || (field.filter != nil && !field.filter(character)) {
				continue
			}
			field.replaceSelection([]rune{character}, true)
		}
	}
}

func (field *TextField) moveCursor(delta int, selecting bool) {
	if !selecting && field.hasSelection() {
		start, end := field.selection()
		if delta < 0 {
			field.cursor = start
		} else {
			field.cursor = end
		}
		field.anchor = field.cursor
		field.ensureCursorVisible()
		return
	}
	field.moveCursorTo(max(0, min(len(field.text), field.cursor+delta)), selecting)
}

func (field *TextField) moveCursorTo(position int, selecting bool) {
	if !selecting {
		field.anchor = position
	}
	field.cursor = position
	field.lastBlinkTime = rl.GetTime()
	field.ensureCursorVisible()
}

func (field *TextField) backspace() {
	if field.deleteSelection(true) || field.cursor == 0 {
		return
	}
	field.text = append(field.text[:field.cursor-1], field.text[field.cursor:]...)
	field.cursor--
	field.anchor = field.cursor
	field.changed()
}

func (field *TextField) deleteForward() {
	if field.deleteSelection(true) || field.cursor >= len(field.text) {
		return
	}
	field.text = append(field.text[:field.cursor], field.text[field.cursor+1:]...)
	field.anchor = field.cursor
	field.changed()
}

func (field *TextField) replaceSelection(value []rune, notify bool) {
	start, end := field.selection()
	available := len(value)
	if field.maxLength > 0 {
		available = min(available, field.maxLength-(len(field.text)-(end-start)))
	}
	if available < 0 {
		available = 0
	}
	value = value[:available]
	next := make([]rune, 0, len(field.text)-(end-start)+len(value))
	next = append(next, field.text[:start]...)
	next = append(next, value...)
	next = append(next, field.text[end:]...)
	field.text = next
	field.cursor = start + len(value)
	field.anchor = field.cursor
	if notify {
		field.changed()
	}
}

func (field *TextField) deleteSelection(notify bool) bool {
	if !field.hasSelection() {
		return false
	}
	field.replaceSelection(nil, notify)
	return true
}

func (field *TextField) selection() (int, int) {
	return min(field.anchor, field.cursor), max(field.anchor, field.cursor)
}

func (field *TextField) hasSelection() bool { return field.anchor != field.cursor }

func (field *TextField) selectedText() string {
	start, end := field.selection()
	return string(field.text[start:end])
}

func (field *TextField) displayText() string {
	if field.password {
		return strings.Repeat("•", len(field.text))
	}
	return field.Text()
}

func (field *TextField) displayPrefix(length int) string {
	if field.password {
		return strings.Repeat("•", length)
	}
	return string(field.text[:length])
}

func (field *TextField) acceptedRunes(value string) []rune {
	result := make([]rune, 0, len(value))
	for _, character := range []rune(strings.ReplaceAll(value, "\n", "")) {
		if character == '\r' || unicode.IsControl(character) || (field.filter != nil && !field.filter(character)) {
			continue
		}
		result = append(result, character)
		if field.maxLength > 0 && len(result) == field.maxLength {
			break
		}
	}
	return result
}

func (field *TextField) changed() {
	field.lastBlinkTime = rl.GetTime()
	field.ensureCursorVisible()
	if field.onChange != nil {
		field.onChange(field.Text())
	}
}

func (field *TextField) indexAt(pointerX float32) int {
	localX := pointerX - field.Bounds().X - field.padding() + field.scrollX
	if localX <= 0 {
		return 0
	}
	for index := 1; index <= len(field.text); index++ {
		previous := MeasureTextStyled(field.displayPrefix(index-1), field.fontSize, field.font).X
		current := MeasureTextStyled(field.displayPrefix(index), field.fontSize, field.font).X
		if localX < (previous+current)*0.5 {
			return index - 1
		}
	}
	return len(field.text)
}

func (field *TextField) ensureCursorVisible() {
	if runtime.canvas == nil {
		return
	}
	visibleWidth := max(1, field.Bounds().Width-field.padding()*2)
	cursorX := MeasureTextStyled(field.displayPrefix(field.cursor), field.fontSize, field.font).X
	if cursorX-field.scrollX > visibleWidth {
		field.scrollX = cursorX - visibleWidth
	}
	if cursorX-field.scrollX < 0 {
		field.scrollX = cursorX
	}
	textWidth := MeasureTextStyled(field.displayText(), field.fontSize, field.font).X
	field.scrollX = max(0, min(field.scrollX, max(0, textWidth-visibleWidth)))
}

func (field *TextField) drawSelection(textX, textY float32) {
	if !field.focused || !field.hasSelection() {
		return
	}
	start, end := field.selection()
	left := MeasureTextStyled(field.displayPrefix(start), field.fontSize, field.font).X
	right := MeasureTextStyled(field.displayPrefix(end), field.fontSize, field.font).X
	height := MeasureTextStyled("Ag", field.fontSize, field.font).Y
	rl.DrawRectangleRec(rl.Rectangle{X: textX + left, Y: textY, Width: right - left, Height: height}, currentTheme.InputSelection)
}

func (field *TextField) drawCaret(textX, textY float32) {
	if !field.focused || field.hasSelection() || int((rl.GetTime()-field.lastBlinkTime)*2)%2 != 0 {
		return
	}
	x := textX + MeasureTextStyled(field.displayPrefix(field.cursor), field.fontSize, field.font).X
	height := MeasureTextStyled("Ag", field.fontSize, field.font).Y
	rl.DrawLineEx(rl.Vector2{X: x, Y: textY}, rl.Vector2{X: x, Y: textY + height}, 2, currentTheme.InputCaret)
}

func (field *TextField) padding() float32 { return max(8, field.Bounds().Height*0.22) }

func keyPressedOrRepeated(key int32) bool {
	return rl.IsKeyPressed(key) || rl.IsKeyPressedRepeat(key)
}

func drawFieldDisabledPattern(bounds rl.Rectangle) {
	phase := float32(rl.GetTime()*12) - float32(int(rl.GetTime()*12/10))*10
	for x := bounds.X - bounds.Height - phase; x < bounds.X+bounds.Width; x += 10 {
		startX := max(bounds.X, x)
		endX := min(bounds.X+bounds.Width, x+bounds.Height)
		if endX > startX {
			rl.DrawLineEx(
				rl.Vector2{X: startX, Y: bounds.Y + bounds.Height - (startX - x)},
				rl.Vector2{X: endX, Y: bounds.Y + bounds.Height - (endX - x)},
				1,
				currentTheme.DisabledPattern,
			)
		}
	}
}
