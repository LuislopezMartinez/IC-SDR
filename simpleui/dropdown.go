package simpleui

import "go-zero/internal/i18n"

import rl "github.com/gen2brain/raylib-go/raylib"

// Dropdown displays a selectable list in an overlay.
type Dropdown struct {
	BaseElement
	items        []string
	placeholder  string
	fontSize     int32
	font         FontStyle
	selected     int
	highlighted  int
	scroll       int
	maxVisible   int
	itemHeight   float32
	open         bool
	focused      bool
	hovered      bool
	pressed      bool
	disabledText string
	onChange     func(int, string)
}

func NewDropdown(id string, x, y, width, height float32, placeholder string, items []string, fontSize int32) *Dropdown {
	dropdown := &Dropdown{
		BaseElement: NewBaseElement(id, x, y, width, height),
		placeholder: placeholder,
		fontSize:    fontSize,
		selected:    -1,
		highlighted: -1,
		maxVisible:  6,
		itemHeight:  max(32, height),
	}
	dropdown.SetItems(items)
	return dropdown
}

func (dropdown *Dropdown) Items() []string             { return append([]string(nil), dropdown.items...) }
func (dropdown *Dropdown) SelectedIndex() int          { return dropdown.selected }
func (dropdown *Dropdown) Open() bool                  { return dropdown.open }
func (dropdown *Dropdown) Focused() bool               { return dropdown.focused }
func (dropdown *Dropdown) CapturingPointer() bool      { return dropdown.pressed }
func (dropdown *Dropdown) OverlayOpen() bool           { return dropdown.open }
func (dropdown *Dropdown) SetFont(style FontStyle)     { dropdown.font = style }
func (dropdown *Dropdown) SetDisabledText(text string) { dropdown.disabledText = text }
func (dropdown *Dropdown) SetMaxVisibleItems(count int) {
	dropdown.maxVisible = max(1, count)
	dropdown.clampScroll()
}
func (dropdown *Dropdown) OnChange(handler func(int, string)) { dropdown.onChange = handler }

func (dropdown *Dropdown) SelectedText() string {
	if dropdown.selected < 0 || dropdown.selected >= len(dropdown.items) {
		return ""
	}
	return dropdown.items[dropdown.selected]
}

func (dropdown *Dropdown) SetItems(items []string) {
	dropdown.items = append(dropdown.items[:0], items...)
	if dropdown.selected >= len(dropdown.items) {
		dropdown.selected = -1
	}
	if dropdown.highlighted >= len(dropdown.items) {
		dropdown.highlighted = dropdown.selected
	}
	dropdown.clampScroll()
}

// SetSelected changes the selection without emitting OnChange. Use -1 to clear it.
func (dropdown *Dropdown) SetSelected(index int) {
	if index < -1 || index >= len(dropdown.items) {
		panic(i18n.Source("text.ac0ed6c5b521"))
	}
	dropdown.selected = index
	dropdown.highlighted = index
	dropdown.ensureHighlightedVisible()
}

func (dropdown *Dropdown) SetFocused(focused bool) {
	if focused && !dropdown.Enabled() {
		return
	}
	dropdown.focused = focused
	if !focused {
		dropdown.open = false
		dropdown.pressed = false
	}
}

func (dropdown *Dropdown) SetEnabled(enabled bool) {
	if !enabled {
		dropdown.SetFocused(false)
		dropdown.hovered = false
	}
	dropdown.BaseElement.SetEnabled(enabled)
}

func (dropdown *Dropdown) Update(input Input) bool {
	if !dropdown.Enabled() {
		return false
	}
	dropdown.hovered = input.Over(dropdown.Bounds())
	if input.Pressed && dropdown.hovered {
		dropdown.pressed = true
		dropdown.SetFocused(true)
	}
	if dropdown.pressed && input.Released {
		clicked := dropdown.hovered
		dropdown.pressed = false
		if clicked {
			dropdown.setOpen(!dropdown.open)
		}
	}
	if dropdown.focused && !dropdown.open && (rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeySpace)) {
		dropdown.setOpen(true)
	}
	return dropdown.hovered || dropdown.pressed || dropdown.focused
}

func (dropdown *Dropdown) UpdateOverlay(input Input) bool {
	if !dropdown.open || !dropdown.Enabled() {
		dropdown.open = false
		return false
	}

	dropdown.handleOverlayKeyboard()
	if !dropdown.open {
		return true
	}

	popup := dropdown.popupBounds()
	if input.Wheel != 0 && input.Over(popup) {
		dropdown.scroll -= int(input.Wheel)
		dropdown.clampScroll()
	}
	dropdown.highlighted = dropdown.itemAt(input.Pointer)
	if input.Pressed {
		switch {
		case input.Over(dropdown.Bounds()):
			dropdown.setOpen(false)
		case input.Over(popup):
			if dropdown.highlighted >= 0 {
				dropdown.choose(dropdown.highlighted, true)
			}
		default:
			dropdown.SetFocused(false)
		}
		return true
	}
	return input.Over(popup) || input.Over(dropdown.Bounds())
}

func (dropdown *Dropdown) Draw() {
	bounds := dropdown.Bounds()
	theme := currentTheme
	background := theme.InputBackground
	border := theme.Border
	if !dropdown.Enabled() {
		background = theme.ControlDisabled
	} else if dropdown.open || dropdown.focused {
		border = theme.Accent
	} else if dropdown.hovered {
		border = brighten(border, 20)
	}
	rl.DrawRectangleRounded(bounds, theme.CornerRadius, 8, background)
	rl.DrawRectangleRoundedLinesEx(bounds, theme.CornerRadius, 8, theme.BorderWidth, border)
	if !dropdown.Enabled() {
		drawFieldDisabledPattern(bounds)
	}

	text := dropdown.SelectedText()
	color := theme.Text
	if text == "" {
		text = dropdown.placeholder
		color = theme.TextMuted
	}
	if !dropdown.Enabled() {
		color = theme.TextMuted
	}
	textY := bounds.Y + (bounds.Height-MeasureTextStyled("Ag", dropdown.fontSize, dropdown.font).Y)*0.5
	DrawTextStyled(dropdown.fitText(text, bounds.Width-48), bounds.X+12, textY, dropdown.fontSize, dropdown.font, color)
	dropdown.drawArrow()
	if !dropdown.Enabled() && dropdown.disabledText != "" {
		width := MeasureText(dropdown.disabledText, 12).X
		DrawText(dropdown.disabledText, bounds.X+bounds.Width-width, bounds.Y-15, 12, theme.TextMuted)
	}
}

func (dropdown *Dropdown) DrawOverlay() {
	popup := dropdown.popupBounds()
	theme := currentTheme
	rl.DrawRectangleRec(popup, theme.PopupBackground)
	rl.DrawRectangleLinesEx(popup, max(theme.BorderWidth, float32(1)), theme.Accent)

	visible := dropdown.visibleCount()
	for row := 0; row < visible; row++ {
		index := dropdown.scroll + row
		item := rl.Rectangle{X: popup.X + 1, Y: popup.Y + 1 + float32(row)*dropdown.itemHeight, Width: popup.Width - 2, Height: dropdown.itemHeight}
		if index == dropdown.highlighted {
			rl.DrawRectangleRec(item, theme.PopupHover)
		} else if index == dropdown.selected {
			rl.DrawRectangleRec(item, theme.InputSelection)
		}
		textY := item.Y + (item.Height-MeasureTextStyled("Ag", dropdown.fontSize, dropdown.font).Y)*0.5
		DrawTextStyled(dropdown.fitText(dropdown.items[index], item.Width-24), item.X+11, textY, dropdown.fontSize, dropdown.font, theme.Text)
	}
	dropdown.drawScrollbar(popup)
}

func (dropdown *Dropdown) fitText(value string, width float32) string {
	if width <= 0 {
		return ""
	}
	if MeasureTextStyled(value, dropdown.fontSize, dropdown.font).X <= width {
		return value
	}
	const ellipsis = "…"
	ellipsisWidth := MeasureTextStyled(ellipsis, dropdown.fontSize, dropdown.font).X
	if ellipsisWidth > width {
		return ""
	}
	runes := []rune(value)
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high + 1) / 2
		if MeasureTextStyled(string(runes[:mid]), dropdown.fontSize, dropdown.font).X+ellipsisWidth <= width {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return string(runes[:low]) + ellipsis
}

func (dropdown *Dropdown) handleOverlayKeyboard() {
	if rl.IsKeyPressed(rl.KeyEscape) {
		dropdown.setOpen(false)
		return
	}
	if keyPressedOrRepeated(rl.KeyDown) {
		if len(dropdown.items) > 0 {
			dropdown.highlighted = min(len(dropdown.items)-1, max(0, dropdown.highlighted+1))
			dropdown.ensureHighlightedVisible()
		}
	}
	if keyPressedOrRepeated(rl.KeyUp) {
		if len(dropdown.items) > 0 {
			if dropdown.highlighted < 0 {
				dropdown.highlighted = len(dropdown.items) - 1
			} else {
				dropdown.highlighted = max(0, dropdown.highlighted-1)
			}
			dropdown.ensureHighlightedVisible()
		}
	}
	if rl.IsKeyPressed(rl.KeyEnter) && dropdown.highlighted >= 0 {
		dropdown.choose(dropdown.highlighted, true)
	}
}

func (dropdown *Dropdown) setOpen(open bool) {
	if open && (!dropdown.Enabled() || len(dropdown.items) == 0) {
		return
	}
	dropdown.open = open
	if open {
		dropdown.focused = true
		dropdown.highlighted = dropdown.selected
		if dropdown.highlighted < 0 {
			dropdown.highlighted = 0
		}
		dropdown.ensureHighlightedVisible()
	}
}

func (dropdown *Dropdown) choose(index int, notify bool) {
	if index < 0 || index >= len(dropdown.items) {
		return
	}
	changed := index != dropdown.selected
	dropdown.selected = index
	dropdown.highlighted = index
	dropdown.open = false
	if changed && notify && dropdown.onChange != nil {
		dropdown.onChange(index, dropdown.items[index])
	}
}

func (dropdown *Dropdown) popupBounds() rl.Rectangle {
	bounds := dropdown.Bounds()
	height := float32(dropdown.visibleCount())*dropdown.itemHeight + 2
	y := bounds.Y + bounds.Height + 4
	if y+height > float32(runtime.height) {
		y = bounds.Y - height - 4
	}
	return rl.Rectangle{X: bounds.X, Y: y, Width: bounds.Width, Height: height}
}

func (dropdown *Dropdown) itemAt(pointer rl.Vector2) int {
	popup := dropdown.popupBounds()
	if !rl.CheckCollisionPointRec(pointer, popup) {
		return -1
	}
	row := int((pointer.Y - popup.Y - 1) / dropdown.itemHeight)
	index := dropdown.scroll + row
	if row < 0 || row >= dropdown.visibleCount() || index >= len(dropdown.items) {
		return -1
	}
	return index
}

func (dropdown *Dropdown) visibleCount() int { return min(len(dropdown.items), dropdown.maxVisible) }

func (dropdown *Dropdown) clampScroll() {
	dropdown.scroll = max(0, min(dropdown.scroll, max(0, len(dropdown.items)-dropdown.visibleCount())))
}

func (dropdown *Dropdown) ensureHighlightedVisible() {
	if dropdown.highlighted < 0 {
		return
	}
	if dropdown.highlighted < dropdown.scroll {
		dropdown.scroll = dropdown.highlighted
	}
	if dropdown.highlighted >= dropdown.scroll+dropdown.visibleCount() {
		dropdown.scroll = dropdown.highlighted - dropdown.visibleCount() + 1
	}
	dropdown.clampScroll()
}

func (dropdown *Dropdown) drawArrow() {
	bounds := dropdown.Bounds()
	centerX := bounds.X + bounds.Width - 19
	centerY := bounds.Y + bounds.Height*0.5
	color := currentTheme.TextMuted
	if dropdown.Enabled() {
		color = currentTheme.Text
	}
	if dropdown.open {
		rl.DrawTriangle(rl.Vector2{X: centerX - 6, Y: centerY + 3}, rl.Vector2{X: centerX + 6, Y: centerY + 3}, rl.Vector2{X: centerX, Y: centerY - 4}, color)
	} else {
		rl.DrawTriangle(rl.Vector2{X: centerX, Y: centerY + 4}, rl.Vector2{X: centerX + 6, Y: centerY - 3}, rl.Vector2{X: centerX - 6, Y: centerY - 3}, color)
	}
}

func (dropdown *Dropdown) drawScrollbar(popup rl.Rectangle) {
	if len(dropdown.items) <= dropdown.visibleCount() {
		return
	}
	rail := rl.Rectangle{X: popup.X + popup.Width - 5, Y: popup.Y + 3, Width: 2, Height: popup.Height - 6}
	rl.DrawRectangleRec(rail, currentTheme.SliderTrack)
	ratio := float32(dropdown.visibleCount()) / float32(len(dropdown.items))
	thumbHeight := max(12, rail.Height*ratio)
	travel := rail.Height - thumbHeight
	maxScroll := max(1, len(dropdown.items)-dropdown.visibleCount())
	thumbY := rail.Y + travel*float32(dropdown.scroll)/float32(maxScroll)
	rl.DrawRectangleRec(rl.Rectangle{X: rail.X - 1, Y: thumbY, Width: 4, Height: thumbHeight}, currentTheme.Accent)
}
