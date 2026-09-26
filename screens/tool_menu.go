package screens

import (
	"go-zero/internal/buildinfo"
	"go-zero/internal/i18n"

	"embed"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

//go:embed assets/icons/*.png
var toolIconFiles embed.FS

type toolMenuItem struct {
	id, label, icon string
	row             int
}

var toolMenuItems = []toolMenuItem{
	{id: "DISTANCE_MAP", label: i18n.Source("text.3bd35969d3d1"), row: 2},
	{id: omniRigToolID, label: "OMNIRIG CAT", icon: "menu-omnirig.png", row: 2},
	{id: i18n.Source("text.e52a5d80bf71"), label: i18n.Source("text.f035065747e2"), icon: "menu-scope.png", row: 0},
	{id: i18n.Source("text.9b6bb9932898"), label: i18n.Source("text.e3bbc3565223"), icon: "menu-waterfall-adjust.png", row: 0},
	{id: i18n.Source("text.94fa3fe96dde"), label: i18n.Source("text.618262ce4370"), icon: "menu-fft-display.png", row: 0},
	{id: i18n.Source("text.a42c60257b01"), label: i18n.Source("text.3758217ace1a"), icon: "menu-twin-pbt.png", row: 0},
	{id: i18n.Source("text.93239b223632"), label: i18n.Source("text.ade0cbd42252"), icon: "menu-dmr-if.png", row: 1},
	{id: i18n.Source("text.a8cbb160caa6"), label: i18n.Source("text.3ae4feb8250d"), icon: "menu-digital-auto.png", row: 1},
	{id: i18n.Source("text.820d4685bc9d"), label: i18n.Source("text.820d4685bc9d"), icon: "menu-sstv.png", row: 1},
	{id: i18n.Source("text.4c4310fd27fd"), label: i18n.Source("text.4c4310fd27fd"), icon: "menu-aprs.png", row: 1},
	{id: i18n.Source("text.8be70e7cb2c4"), label: i18n.Source("text.8be70e7cb2c4"), icon: "menu-rtl433.png", row: 1},
	{id: i18n.Source("text.7dd172035702"), label: i18n.Source("text.493b20493ea1"), icon: "menu-radiosonde.png", row: 1},
	{id: i18n.Source("text.208a2a3f8f27"), label: i18n.Source("text.208a2a3f8f27"), icon: "menu-ais.png", row: 1},
	{id: i18n.Source("text.a3201958b4e6"), label: i18n.Source("text.7866f9f32e66"), icon: "menu-aircraft.png", row: 1},
	{id: i18n.Source("text.f69d86a86926"), label: i18n.Source("text.f69d86a86926"), icon: "menu-tetra.png", row: 1},
	{id: tetrapolToolID, label: "TETRAPOL", icon: "menu-tetrapol.png", row: 1},
	{id: i18n.Source("text.bcdc9d50f2be"), label: i18n.Source("text.3fc45e92c2be"), row: 2},
	{id: i18n.Source("text.918191dc299c"), label: i18n.Source("text.db6a87d580b1"), row: 2},
	{id: "ABOUT", label: "ACERCA DE", row: 2},
	{id: i18n.Source("text.03c820ee5c6c"), label: i18n.Source("text.ace1d287d60c"), row: 2},
	{id: "CHECK_UPDATES", label: "ACTUALIZACIONES", row: 2},
}

func toolMenuModalBounds() rl.Rectangle {
	width, height := float32(1360), float32(830)
	return rl.Rectangle{X: (designWidth - width) / 2, Y: (designHeight - height) / 2, Width: width, Height: height}
}

func contactDialogBounds() rl.Rectangle {
	return rl.Rectangle{X: (designWidth - 590) / 2, Y: (designHeight - 235) / 2, Width: 590, Height: 235}
}

func contactActionBounds(index int) rl.Rectangle {
	box := contactDialogBounds()
	return rl.Rectangle{X: box.X + 35 + float32(index)*275, Y: box.Y + 167, Width: 245, Height: 48}
}

func aboutDialogBounds() rl.Rectangle {
	return rl.Rectangle{X: (designWidth - 720) / 2, Y: (designHeight - 430) / 2, Width: 720, Height: 430}
}

func aboutActionBounds(index int) rl.Rectangle {
	box := aboutDialogBounds()
	return rl.Rectangle{X: box.X + 45 + float32(index)*330, Y: box.Y + 343, Width: 300, Height: 52}
}

type ToolMenu struct {
	simpleui.BaseElement
	open            bool
	selected        string
	pressedItem     int
	pressedBack     bool
	contactOpen     bool
	contactPressed  int
	contactCopied   bool
	aboutOpen       bool
	aboutPressed    int
	aboutCopied     bool
	languageOffset  int
	languageOpen    bool
	languagePressed bool
	icons           map[string]rl.Texture2D
	onSelect        func(string)
	onSelectSound   func()
	onCheckUpdates  func()
}

func NewToolMenu(selected string, onSelect func(string)) *ToolMenu {
	return &ToolMenu{
		BaseElement: simpleui.NewBaseElement("toolMenuOverlay", 0, 0, designWidth, designHeight),
		selected:    selected, pressedItem: -1, icons: make(map[string]rl.Texture2D), onSelect: onSelect,
	}
}

func (menu *ToolMenu) Open() { menu.open = true }
func (menu *ToolMenu) Close() {
	menu.open = false
	menu.languageOpen = false
	menu.languagePressed = false
	menu.pressedItem = -1
	menu.pressedBack = false
	menu.contactOpen = false
	menu.contactCopied = false
	menu.aboutOpen = false
	menu.aboutCopied = false
}
func (menu *ToolMenu) OverlayOpen() bool          { return menu.open }
func (menu *ToolMenu) Update(simpleui.Input) bool { return false }
func (menu *ToolMenu) Draw()                      {}

func (menu *ToolMenu) UpdateOverlay(input simpleui.Input) bool {
	if !menu.open {
		return false
	}
	if !menu.contactOpen && !menu.aboutOpen && menu.languageInput(input) {
		return true
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		if menu.contactOpen {
			menu.contactOpen = false
			return true
		}
		if menu.aboutOpen {
			menu.aboutOpen = false
			return true
		}
		menu.Close()
		return true
	}
	if input.Pressed {
		if menu.contactOpen {
			menu.contactPressed = 0
			if rl.CheckCollisionPointRec(input.Pointer, contactActionBounds(0)) {
				menu.contactPressed = 1
			}
			if rl.CheckCollisionPointRec(input.Pointer, contactActionBounds(1)) {
				menu.contactPressed = 2
			}
			return true
		}
		if menu.aboutOpen {
			menu.aboutPressed = 0
			if rl.CheckCollisionPointRec(input.Pointer, aboutActionBounds(0)) {
				menu.aboutPressed = 1
			}
			if rl.CheckCollisionPointRec(input.Pointer, aboutActionBounds(1)) {
				menu.aboutPressed = 2
			}
			return true
		}
		menu.pressedItem = menu.itemAt(input.Pointer)
		menu.pressedBack = rl.CheckCollisionPointRec(input.Pointer, menu.backBounds())
	}
	if input.Released {
		if menu.contactOpen {
			if menu.contactPressed == 1 && rl.CheckCollisionPointRec(input.Pointer, contactActionBounds(0)) {
				rl.SetClipboardText("luislopezmartinez1979@gmail.com")
				menu.contactCopied = rl.GetClipboardText() == "luislopezmartinez1979@gmail.com"
			}
			if menu.contactPressed == 2 && rl.CheckCollisionPointRec(input.Pointer, contactActionBounds(1)) {
				menu.contactOpen = false
			}
			menu.contactPressed = 0
			return true
		}
		if menu.aboutOpen {
			if menu.aboutPressed == 1 && rl.CheckCollisionPointRec(input.Pointer, aboutActionBounds(0)) {
				rl.SetClipboardText(buildinfo.Details())
				menu.aboutCopied = rl.GetClipboardText() == buildinfo.Details()
			}
			if menu.aboutPressed == 2 && rl.CheckCollisionPointRec(input.Pointer, aboutActionBounds(1)) {
				menu.aboutOpen = false
			}
			menu.aboutPressed = 0
			return true
		}
		item := menu.itemAt(input.Pointer)
		if menu.pressedBack && rl.CheckCollisionPointRec(input.Pointer, menu.backBounds()) {
			simpleui.PlayActivationFeedback()
			menu.Close()
		} else if item >= 0 && item == menu.pressedItem {
			if toolMenuItems[item].id == "ABOUT" {
				menu.aboutOpen = true
				menu.aboutCopied = false
				menu.pressedItem = -1
				return true
			}
			if toolMenuItems[item].id == i18n.Source("text.03c820ee5c6c") {
				menu.contactOpen = true
				menu.contactCopied = false
				menu.pressedItem = -1
				return true
			}
			if toolMenuItems[item].id == "CHECK_UPDATES" {
				menu.Close()
				if menu.onCheckUpdates != nil {
					menu.onCheckUpdates()
				}
				return true
			}
			menu.selected = toolMenuItems[item].id
			menu.Close()
			if menu.onSelectSound != nil {
				menu.onSelectSound()
			}
			if menu.onSelect != nil {
				menu.onSelect(menu.selected)
			}
		}
		menu.pressedItem = -1
		menu.pressedBack = false
	}
	return true
}

func (menu *ToolMenu) SetSelectSound(play func())   { menu.onSelectSound = play }
func (menu *ToolMenu) SetCheckUpdates(check func()) { menu.onCheckUpdates = check }

func (menu *ToolMenu) DrawOverlay() {
	if !menu.open {
		return
	}
	menu.loadIcons()
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 218})
	modal := toolMenuModalBounds()
	rl.DrawRectangleRounded(modal, .018, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(modal, .018, 8, 2, colors.border)
	header := rl.Rectangle{X: modal.X + 2, Y: modal.Y + 2, Width: modal.Width - 4, Height: 55}
	rl.DrawRectangleRec(header, colors.panelAlt)
	drawCenteredStyled(i18n.Source("text.11364a4c8f17"), header, 27, simpleui.FontSemiBold, colors.text)
	version := buildinfo.DisplayVersion()
	versionWidth := simpleui.MeasureTextStyled(version, 14, simpleui.FontSemiBold).X
	simpleui.DrawTextStyled(version, header.X+header.Width-versionWidth-22, header.Y+19, 14, simpleui.FontSemiBold, colors.cyan)
	rl.DrawLine(int32(modal.X+20), int32(modal.Y+57), int32(modal.X+modal.Width-20), int32(modal.Y+57), rl.Color{R: 75, G: 75, B: 75, A: 255})

	menu.drawCategory(0, i18n.Source("text.4230649e4327"), colors.cyan)
	menu.drawCategory(1, i18n.Source("text.e3abcf4d403c"), rl.Color{R: 155, G: 115, B: 225, A: 255})
	menu.drawCategory(2, i18n.Source("text.3950e970cb71"), rl.Color{R: 235, G: 165, B: 45, A: 255})
	for index := range toolMenuItems {
		menu.drawItem(index)
	}
	menu.drawLanguages()
	if menu.aboutOpen {
		menu.drawAboutDialog()
		return
	}
	if menu.contactOpen {
		rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 170})
		box := contactDialogBounds()
		rl.DrawRectangleRounded(box, .04, 8, colors.panel)
		rl.DrawRectangleRoundedLinesEx(box, .04, 8, 2, colors.border)
		simpleui.DrawTextStyled(i18n.Source("text.ace1d287d60c"), box.X+35, box.Y+27, 24, simpleui.FontSemiBold, colors.cyan)
		simpleui.DrawTextStyled(i18n.Source("text.568715d95c93"), box.X+35, box.Y+75, 18, simpleui.FontRegular, colors.text)
		simpleui.DrawTextStyled("luislopezmartinez1979@gmail.com", box.X+35, box.Y+109, 19, simpleui.FontSemiBold, colors.text)
		if menu.contactCopied {
			simpleui.DrawTextStyled(i18n.Source("text.532a979f11b3"), box.X+35, box.Y+139, 14, simpleui.FontRegular, colors.green)
		}
		for _, action := range []struct {
			x     float32
			label string
		}{{contactActionBounds(0).X, i18n.Source("text.99b154d65f2b")}, {contactActionBounds(1).X, i18n.Source("text.b908be5df2f1")}} {
			rect := rl.Rectangle{X: action.x, Y: contactActionBounds(0).Y, Width: 245, Height: 48}
			rl.DrawRectangleRounded(rect, .15, 6, colors.panelAlt)
			drawCenteredStyled(action.label, rect, 17, simpleui.FontSemiBold, colors.text)
		}
		return
	}
	back := menu.backBounds()
	backColor := colors.panelAlt
	if menu.pressedBack {
		backColor = mixColor(colors.panelAlt, colors.cyan, .18)
	}
	rl.DrawRectangleRounded(back, .2, 8, backColor)
	rl.DrawRectangleRoundedLinesEx(back, .2, 8, 2, rl.Color{R: 130, G: 145, B: 160, A: 255})
	drawCenteredStyled(i18n.Source("text.bd62e781ec4e"), back, 17, simpleui.FontSemiBold, simpleui.EnsureTextContrast(colors.text, backColor))
}

func (menu *ToolMenu) drawAboutDialog() {
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 175})
	box := aboutDialogBounds()
	rl.DrawRectangleRounded(box, .035, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(box, .035, 8, 2, colors.cyan)
	drawCenteredStyled("IC-SDR", rl.Rectangle{X: box.X, Y: box.Y + 27, Width: box.Width, Height: 42}, 30, simpleui.FontSemiBold, colors.cyan)
	drawCenteredStyled(buildinfo.DisplayVersion(), rl.Rectangle{X: box.X, Y: box.Y + 72, Width: box.Width, Height: 30}, 19, simpleui.FontSemiBold, colors.text)

	details := []struct {
		label, value string
	}{
		{"COMPILACIÓN", buildinfo.BuildDate},
		{"REVISIÓN GIT", buildinfo.Revision},
		{"PLATAFORMA", buildinfo.Platform()},
	}
	for index, detail := range details {
		y := box.Y + 135 + float32(index)*58
		simpleui.DrawTextStyled(detail.label, box.X+65, y, 12, simpleui.FontSemiBold, colors.muted)
		simpleui.DrawTextStyled(detail.value, box.X+245, y-2, 15, simpleui.FontMono, colors.text)
	}
	if menu.aboutCopied {
		drawCenteredStyled("INFORMACIÓN COPIADA", rl.Rectangle{X: box.X, Y: box.Y + 304, Width: box.Width, Height: 24}, 13, simpleui.FontSemiBold, colors.green)
	}
	for index, label := range []string{"COPIAR INFORMACIÓN", "CERRAR"} {
		bounds := aboutActionBounds(index)
		fill := colors.panelAlt
		if menu.aboutPressed == index+1 {
			fill = mixColor(fill, colors.cyan, .18)
		}
		rl.DrawRectangleRounded(bounds, .15, 7, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .15, 7, 1.5, colors.border)
		drawCenteredStyled(label, bounds, 15, simpleui.FontSemiBold, simpleui.EnsureTextContrast(colors.text, fill))
	}
}

func (menu *ToolMenu) drawCategory(row int, label string, accent rl.Color) {
	y := []float32{124, 262, 540}[row]
	modal := toolMenuModalBounds()
	simpleui.DrawTextStyled(label, modal.X+30, y-8, 13, simpleui.FontSemiBold, accent)
	labelWidth := simpleui.MeasureTextStyled(label, 13, simpleui.FontSemiBold).X
	rl.DrawLine(int32(modal.X+42+labelWidth), int32(y), int32(modal.X+modal.Width-30), int32(y), rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 110})
}

func (menu *ToolMenu) drawItem(index int) {
	item := toolMenuItems[index]
	bounds := menu.itemBounds(index)
	active := item.id == menu.selected
	accent := menu.rowAccent(item.row)
	fill := colors.panelAlt
	if active {
		fill = blendRGBA(fill, accent, .20)
	}
	border := rl.Color{R: 80, G: 86, B: 94, A: 255}
	borderWidth := float32(1.5)
	if active {
		border, borderWidth = accent, 2.5
	}
	rl.DrawRectangleRounded(bounds, .08, 8, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .08, 8, borderWidth, border)
	inner := rl.Rectangle{X: bounds.X + 5, Y: bounds.Y + 5, Width: bounds.Width - 10, Height: bounds.Height - 10}
	rl.DrawRectangleRoundedLinesEx(inner, .08, 8, 1, rl.Color{R: 120, G: 125, B: 130, A: 255})

	center := rl.Vector2{X: bounds.X + bounds.Width/2, Y: bounds.Y + 42}
	if texture, exists := menu.icons[item.icon]; exists {
		source := rl.Rectangle{Width: float32(texture.Width), Height: float32(texture.Height)}
		destination := rl.Rectangle{X: center.X, Y: center.Y, Width: 76, Height: 68}
		rl.DrawTexturePro(texture, source, destination, rl.Vector2{X: 38, Y: 34}, 0, rl.White)
	} else {
		menu.drawCustomIcon(item.id, center, active, item.row)
	}
	textColor := simpleui.EnsureTextContrast(colors.text, fill)
	if active {
		textColor = accent
	}
	labelSize := int32(17)
	for labelSize > 12 && simpleui.MeasureTextStyled(item.label, labelSize, simpleui.FontSemiBold).X > bounds.Width-14 {
		labelSize--
	}
	drawCenteredStyled(item.label, rl.Rectangle{X: bounds.X, Y: bounds.Y + 75, Width: bounds.Width, Height: 27}, labelSize, simpleui.FontSemiBold, textColor)
	if active {
		rl.DrawRectangleRounded(rl.Rectangle{X: bounds.X + 25, Y: bounds.Y + bounds.Height - 6, Width: bounds.Width - 50, Height: 3}, 1, 4, accent)
	}
}

func (menu *ToolMenu) drawCustomIcon(id string, center rl.Vector2, active bool, row int) {
	accent := colors.cyan
	if active {
		accent = menu.rowAccent(row)
	}
	switch id {
	case "DIGITAL_AUTO":
		rl.DrawCircleLines(int32(center.X), int32(center.Y), 31, accent)
		for ring := float32(12); ring <= 24; ring += 12 {
			rl.DrawCircleLines(int32(center.X), int32(center.Y), ring, colors.muted)
		}
		for i, h := range []float32{13, 28, 43, 24, 36} {
			x := center.X - 31 + float32(i)*15
			rl.DrawRectangleRounded(rl.Rectangle{X: x, Y: center.Y + 24 - h, Width: 8, Height: h}, .3, 4, accent)
		}
	case "RADIOSONDE":
		rl.DrawEllipse(int32(center.X), int32(center.Y-13), 20, 24, accent)
		rl.DrawLineEx(rl.Vector2{X: center.X - 12, Y: center.Y + 6}, rl.Vector2{X: center.X, Y: center.Y + 23}, 2, colors.muted)
		rl.DrawLineEx(rl.Vector2{X: center.X + 12, Y: center.Y + 6}, rl.Vector2{X: center.X, Y: center.Y + 23}, 2, colors.muted)
		rl.DrawRectangle(int32(center.X-6), int32(center.Y+23), 12, 10, accent)
	case "APRS":
		rl.DrawRectangleRounded(rl.Rectangle{X: center.X - 38, Y: center.Y - 27, Width: 76, Height: 54}, .12, 6, rl.Color{R: 22, G: 34, B: 42, A: 255})
		rl.DrawRectangleRoundedLinesEx(rl.Rectangle{X: center.X - 38, Y: center.Y - 27, Width: 76, Height: 54}, .12, 6, 2, accent)
		rl.DrawLineEx(rl.Vector2{X: center.X - 27, Y: center.Y + 12}, rl.Vector2{X: center.X - 8, Y: center.Y - 1}, 2, accent)
		rl.DrawLineEx(rl.Vector2{X: center.X - 8, Y: center.Y - 1}, rl.Vector2{X: center.X + 24, Y: center.Y + 7}, 2, accent)
		rl.DrawCircle(int32(center.X+5), int32(center.Y-8), 6, accent)
	case "RTL_433":
		rl.DrawCircleLines(int32(center.X), int32(center.Y-7), 5, accent)
		rl.DrawLine(int32(center.X), int32(center.Y-2), int32(center.X), int32(center.Y+20), accent)
		rl.DrawRectangleRounded(rl.Rectangle{X: center.X - 35, Y: center.Y + 5, Width: 70, Height: 28}, .18, 6, rl.Color{R: 35, G: 43, B: 50, A: 255})
		drawCentered("433", rl.Rectangle{X: center.X - 35, Y: center.Y + 5, Width: 70, Height: 28}, 12, accent)
	case "RECORDER":
		rl.DrawRectangleRounded(rl.Rectangle{X: center.X - 40, Y: center.Y - 28, Width: 80, Height: 58}, .12, 6, rl.Color{R: 29, G: 35, B: 42, A: 255})
		rl.DrawCircleLines(int32(center.X-19), int32(center.Y-7), 13, colors.muted)
		rl.DrawCircleLines(int32(center.X+19), int32(center.Y-7), 13, colors.muted)
		rl.DrawLine(int32(center.X-19), int32(center.Y+6), int32(center.X+19), int32(center.Y+6), colors.orange)
		rl.DrawCircle(int32(center.X+29), int32(center.Y+20), 6, colors.red)
	case "SATELLITES":
		// A compact orbital mark that remains crisp in every palette and avoids
		// adding another bitmap asset before the satellite tool is implemented.
		rl.DrawEllipseLines(int32(center.X), int32(center.Y), 36, 17, accent)
		rl.DrawEllipseLines(int32(center.X), int32(center.Y), 17, 36, colors.muted)
		rl.DrawCircle(int32(center.X), int32(center.Y), 10, accent)
		rl.DrawRectangle(int32(center.X-25), int32(center.Y-5), 12, 10, colors.blue)
		rl.DrawRectangle(int32(center.X+13), int32(center.Y-5), 12, 10, colors.blue)
		rl.DrawCircle(int32(center.X+31), int32(center.Y-17), 4, colors.orange)
	case "WEB_SERVER":
		body := rl.Rectangle{X: center.X - 30, Y: center.Y - 28, Width: 60, Height: 56}
		rl.DrawRectangleRounded(body, .12, 6, accent)
		for _, offset := range []float32{-18, -1, 16} {
			row := rl.Rectangle{X: center.X - 22, Y: center.Y + offset, Width: 44, Height: 11}
			rl.DrawRectangleRounded(row, .2, 4, colors.panelAlt)
			rl.DrawCircle(int32(center.X-14), int32(center.Y+offset+5), 3, colors.green)
			rl.DrawLineEx(rl.Vector2{X: center.X - 4, Y: center.Y + offset + 5}, rl.Vector2{X: center.X + 14, Y: center.Y + offset + 5}, 2, colors.muted)
		}
	case "OMNIRIG":
		rl.DrawRectangleRounded(rl.Rectangle{X: center.X - 39, Y: center.Y - 24, Width: 78, Height: 48}, .15, 6, colors.panelAlt)
		rl.DrawRectangleRoundedLinesEx(rl.Rectangle{X: center.X - 39, Y: center.Y - 24, Width: 78, Height: 48}, .15, 6, 2, accent)
		drawCentered("CAT", rl.Rectangle{X: center.X - 35, Y: center.Y - 20, Width: 70, Height: 40}, 18, accent)
		rl.DrawLineEx(rl.Vector2{X: center.X - 28, Y: center.Y + 30}, rl.Vector2{X: center.X + 28, Y: center.Y + 30}, 3, colors.muted)
	case "DISTANCE_MAP":
		rl.DrawLineEx(rl.Vector2{X: center.X - 25, Y: center.Y + 15}, rl.Vector2{X: center.X + 25, Y: center.Y - 15}, 3, accent)
		rl.DrawCircleV(rl.Vector2{X: center.X - 25, Y: center.Y + 15}, 9, colors.cyan)
		rl.DrawCircleV(rl.Vector2{X: center.X + 25, Y: center.Y - 15}, 9, colors.orange)
	case "CONTACT_AUTHOR":
		envelope := rl.Rectangle{X: center.X - 34, Y: center.Y - 22, Width: 68, Height: 47}
		rl.DrawRectangleRounded(envelope, .12, 6, colors.panelAlt)
		rl.DrawRectangleRoundedLinesEx(envelope, .12, 6, 3, accent)
		rl.DrawLineEx(rl.Vector2{X: center.X - 30, Y: center.Y - 17}, rl.Vector2{X: center.X, Y: center.Y + 3}, 3, accent)
		rl.DrawLineEx(rl.Vector2{X: center.X, Y: center.Y + 3}, rl.Vector2{X: center.X + 30, Y: center.Y - 17}, 3, accent)
		rl.DrawCircle(int32(center.X+34), int32(center.Y-23), 7, colors.green)
	case "ABOUT":
		rl.DrawCircleLines(int32(center.X), int32(center.Y), 32, accent)
		drawCenteredStyled("i", rl.Rectangle{X: center.X - 25, Y: center.Y - 29, Width: 50, Height: 58}, 32, simpleui.FontSemiBold, accent)
	default:
		drawCentered("?", rl.Rectangle{X: center.X - 30, Y: center.Y - 25, Width: 60, Height: 50}, 24, accent)
	}
}

func (menu *ToolMenu) loadIcons() {
	if len(menu.icons) > 0 {
		return
	}
	for _, item := range toolMenuItems {
		if item.icon == "" {
			continue
		}
		data, err := toolIconFiles.ReadFile("assets/icons/" + item.icon)
		if err != nil {
			continue
		}
		image := rl.LoadImageFromMemory(".png", data, int32(len(data)))
		if image == nil || image.Data == nil {
			continue
		}
		rl.ImageResize(image, 80, 80)
		texture := rl.LoadTextureFromImage(image)
		rl.UnloadImage(image)
		rl.SetTextureFilter(texture, rl.FilterBilinear)
		menu.icons[item.icon] = texture
	}
}

func (menu *ToolMenu) itemAt(point rl.Vector2) int {
	for index := range toolMenuItems {
		if rl.CheckCollisionPointRec(point, menu.itemBounds(index)) {
			return index
		}
	}
	return -1
}

func (menu *ToolMenu) itemBounds(index int) rl.Rectangle {
	item := toolMenuItems[index]
	column, count := 0, 0
	for i, candidate := range toolMenuItems {
		if candidate.row == item.row {
			count++
			if i < index {
				column++
			}
		}
	}
	step, width := float32(168), float32(160)
	y := float32(135 + item.row*138)
	if item.row == 1 {
		lineCount, lineColumn := 5, column
		y = 273
		if column >= 5 {
			lineCount, lineColumn, y = count-5, column-5, 390
		}
		rowX := designWidth/2 - float32(lineCount)*step/2
		return rl.Rectangle{X: rowX + float32(lineColumn)*step, Y: y, Width: width, Height: 108}
	}
	if item.row == 2 {
		y = 551
		step, width = 150, 140
	}
	rowX := designWidth/2 - float32(count)*step/2
	return rl.Rectangle{X: rowX + float32(column)*step, Y: y, Width: width, Height: 108}
}

func (menu *ToolMenu) backBounds() rl.Rectangle {
	return rl.Rectangle{X: (designWidth - 300) / 2, Y: 820, Width: 300, Height: 38}
}

func (menu *ToolMenu) rowAccent(row int) rl.Color {
	if row == 1 {
		return rl.Color{R: 155, G: 115, B: 225, A: 255}
	}
	if row == 2 {
		return rl.Color{R: 235, G: 165, B: 45, A: 255}
	}
	return colors.cyan
}

func drawCentered(text string, bounds rl.Rectangle, size int32, color rl.Color) {
	drawCenteredStyled(text, bounds, size, simpleui.FontRegular, color)

}

func drawCenteredStyled(text string, bounds rl.Rectangle, size int32, style simpleui.FontStyle, color rl.Color) {
	measured := simpleui.MeasureTextStyled(text, size, style)
	simpleui.DrawTextStyled(text, bounds.X+(bounds.Width-measured.X)/2, bounds.Y+(bounds.Height-measured.Y)/2, size, style, color)
}

func blendRGBA(from, to rl.Color, amount float32) rl.Color {
	amount = min(max(amount, 0), 1)
	return rl.Color{
		R: uint8(float32(from.R) + (float32(to.R)-float32(from.R))*amount),
		G: uint8(float32(from.G) + (float32(to.G)-float32(from.G))*amount),
		B: uint8(float32(from.B) + (float32(to.B)-float32(from.B))*amount), A: 255,
	}
}
