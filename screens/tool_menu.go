package screens

import (
	"go-zero/internal/buildinfo"
	"go-zero/internal/i18n"

	"embed"
	"math"
	"strconv"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const feedbackToolID = "FEEDBACK"

//go:embed assets/icons/*.png assets/menu/*.png
var toolIconFiles embed.FS

type toolMenuItem struct {
	id, label, icon, art string
	row                  int
}

var toolMenuItems = []toolMenuItem{
	{id: "DISTANCE_MAP", label: i18n.Source("text.3bd35969d3d1"), art: "menu-card-distance.png", row: 2},
	{id: qrzToolID, label: "QRZ CALLBOOK", art: "menu-card-qrz.png", row: 2},
	{id: omniRigToolID, label: "OMNIRIG CAT", icon: "menu-omnirig.png", art: "menu-card-omnirig.png", row: 2},
	{id: contestLogToolID, label: i18n.Source("text.log0037"), art: "menu-card-contest.png", row: 2},
	{id: voacapToolID, label: "PROPAGACIÓN DX", icon: "menu-scope.png", art: "menu-card-propagation.png", row: 0},
	{id: i18n.Source("text.9b6bb9932898"), label: i18n.Source("text.e3bbc3565223"), icon: "menu-waterfall-adjust.png", art: "menu-card-waterfall.png", row: 0},
	{id: i18n.Source("text.94fa3fe96dde"), label: i18n.Source("text.618262ce4370"), icon: "menu-fft-display.png", art: "menu-card-fft.png", row: 0},
	{id: i18n.Source("text.a42c60257b01"), label: i18n.Source("text.3758217ace1a"), icon: "menu-twin-pbt.png", art: "menu-card-pbt-audio.png", row: 0},
	{id: i18n.Source("text.93239b223632"), label: i18n.Source("text.ade0cbd42252"), icon: "menu-dmr-if.png", art: "menu-card-dmr.png", row: 1},
	{id: i18n.Source("text.a8cbb160caa6"), label: i18n.Source("text.3ae4feb8250d"), icon: "menu-digital-auto.png", art: "menu-card-digital-auto.png", row: 1},
	{id: dabToolID, label: "DAB / DAB+", art: "menu-card-dab.png", row: 1},
	{id: datvToolID, label: "DATV", art: "menu-card-datv.png", row: 1},
	{id: i18n.Source("text.820d4685bc9d"), label: i18n.Source("text.820d4685bc9d"), icon: "menu-sstv.png", art: "menu-card-sstv.png", row: 1},
	{id: ft8ToolID, label: "FT8", icon: "menu-ft8.png", art: "menu-card-ft8.png", row: 1},
	{id: i18n.Source("text.4c4310fd27fd"), label: i18n.Source("text.4c4310fd27fd"), icon: "menu-aprs.png", art: "menu-card-aprs.png", row: 1},
	{id: i18n.Source("text.8be70e7cb2c4"), label: i18n.Source("text.8be70e7cb2c4"), icon: "menu-rtl433.png", art: "menu-card-rtl433.png", row: 1},
	{id: i18n.Source("text.7dd172035702"), label: i18n.Source("text.493b20493ea1"), icon: "menu-radiosonde.png", art: "menu-card-radiosonde.png", row: 1},
	{id: i18n.Source("text.208a2a3f8f27"), label: i18n.Source("text.208a2a3f8f27"), icon: "menu-ais.png", art: "menu-card-ais.png", row: 1},
	{id: i18n.Source("text.a3201958b4e6"), label: i18n.Source("text.7866f9f32e66"), icon: "menu-aircraft.png", art: "menu-card-aircraft.png", row: 1},
	{id: i18n.Source("text.f69d86a86926"), label: i18n.Source("text.f69d86a86926"), icon: "menu-tetra.png", art: "menu-card-tetra.png", row: 1},
	{id: tetrapolToolID, label: "TETRAPOL", icon: "menu-tetrapol.png", art: "menu-card-tetrapol.png", row: 1},
	{id: i18n.Source("text.bcdc9d50f2be"), label: i18n.Source("text.3fc45e92c2be"), art: "menu-card-satellites.png", row: 2},
	{id: i18n.Source("text.918191dc299c"), label: i18n.Source("text.db6a87d580b1"), art: "menu-card-web.png", row: 2},
	{id: "ABOUT", label: "ACERCA DE", art: "menu-card-about.png", row: 2},
	{id: i18n.Source("text.03c820ee5c6c"), label: i18n.Source("text.ace1d287d60c"), art: "menu-card-contact.png", row: 2},
	{id: "CHECK_UPDATES", label: "ACTUALIZACIONES", art: "menu-card-updates.png", row: 2},
	{id: feedbackToolID, label: "REPORTES Y SUGERENCIAS", art: "menu-card-feedback.png", row: 2},
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
	open              bool
	selected          string
	category          int
	focusIndex        int
	carousel          float32
	openedAt          float64
	hoverItem         int
	hoverCategory     int
	pressedCategory   int
	pressedItem       int
	pressedThumbArrow int
	pressedBack       bool
	contactOpen       bool
	contactPressed    int
	contactCopied     bool
	aboutOpen         bool
	aboutPressed      int
	aboutCopied       bool
	languageOffset    int
	languageOpen      bool
	languagePressed   bool
	icons             map[string]rl.Texture2D
	onSelect          func(string)
	onSelectSound     func()
	onCheckUpdates    func()
	onFeedback        func()
}

func NewToolMenu(selected string, onSelect func(string)) *ToolMenu {
	menu := &ToolMenu{
		BaseElement: simpleui.NewBaseElement("toolMenuOverlay", 0, 0, designWidth, designHeight),
		selected:    selected, pressedItem: -1, pressedCategory: -1, pressedThumbArrow: -1,
		hoverItem: -1, hoverCategory: -1,
		icons: make(map[string]rl.Texture2D), onSelect: onSelect,
	}
	menu.syncFocusToSelected()
	return menu
}

func (menu *ToolMenu) Open() {
	menu.open = true
	menu.openedAt = rl.GetTime()
	menu.syncFocusToSelected()
}
func (menu *ToolMenu) Close() {
	menu.open = false
	menu.languageOpen = false
	menu.languagePressed = false
	menu.pressedItem = -1
	menu.pressedCategory = -1
	menu.pressedThumbArrow = -1
	menu.hoverItem = -1
	menu.hoverCategory = -1
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
	menu.updateAnimation()
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
	if !menu.contactOpen && !menu.aboutOpen && !menu.languageOpen {
		if rl.IsKeyPressed(rl.KeyLeft) {
			menu.moveFocus(-1)
			return true
		}
		if rl.IsKeyPressed(rl.KeyRight) {
			menu.moveFocus(1)
			return true
		}
		if rl.IsKeyPressed(rl.KeyUp) {
			menu.setCategory((menu.category + 2) % 3)
			return true
		}
		if rl.IsKeyPressed(rl.KeyDown) {
			menu.setCategory((menu.category + 1) % 3)
			return true
		}
		if rl.IsKeyPressed(rl.KeyEnter) && menu.focusIndex >= 0 {
			menu.activateItem(menu.focusIndex)
			return true
		}
	}
	menu.hoverItem = menu.itemAt(input.Pointer)
	menu.hoverCategory = menu.categoryAt(input.Pointer)
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
		menu.pressedCategory = menu.categoryAt(input.Pointer)
		menu.pressedItem = menu.itemAt(input.Pointer)
		menu.pressedThumbArrow = menu.thumbnailArrowAt(input.Pointer)
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
		category := menu.categoryAt(input.Pointer)
		thumbArrow := menu.thumbnailArrowAt(input.Pointer)
		if menu.pressedBack && rl.CheckCollisionPointRec(input.Pointer, menu.backBounds()) {
			simpleui.PlayActivationFeedback()
			menu.Close()
		} else if category >= 0 && category == menu.pressedCategory {
			menu.setCategory(category)
		} else if item >= 0 && item == menu.pressedItem {
			if item != menu.focusIndex {
				menu.focusIndex = item
			} else {
				menu.activateItem(item)
			}
		} else if thumbArrow >= 0 && thumbArrow == menu.pressedThumbArrow {
			menu.moveThumbnailPage(thumbArrow)
		}
		menu.pressedItem = -1
		menu.pressedCategory = -1
		menu.pressedThumbArrow = -1
		menu.pressedBack = false
	}
	return true
}

func (menu *ToolMenu) activateItem(index int) {
	if index < 0 || index >= len(toolMenuItems) {
		return
	}
	item := toolMenuItems[index]
	if item.id == i18n.Source("text.03c820ee5c6c") {
		menu.contactOpen, menu.contactCopied = true, false
		return
	}
	switch item.id {
	case "ABOUT":
		menu.aboutOpen, menu.aboutCopied = true, false
	case "CHECK_UPDATES":
		menu.Close()
		if menu.onCheckUpdates != nil {
			menu.onCheckUpdates()
		}
	case feedbackToolID:
		menu.Close()
		if menu.onFeedback != nil {
			menu.onFeedback()
		}
	default:
		menu.selected = item.id
		menu.Close()
		if menu.onSelectSound != nil {
			menu.onSelectSound()
		}
		if menu.onSelect != nil {
			menu.onSelect(menu.selected)
		}
	}
}

func (menu *ToolMenu) SetSelectSound(play func())   { menu.onSelectSound = play }
func (menu *ToolMenu) SetCheckUpdates(check func()) { menu.onCheckUpdates = check }
func (menu *ToolMenu) SetFeedback(open func())      { menu.onFeedback = open }

func (menu *ToolMenu) DrawOverlay() {
	if !menu.open {
		return
	}
	menu.loadIcons()
	menu.updateAnimation()
	openAmount := float32(min((rl.GetTime()-menu.openedAt)*7.5, 1))
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{R: 1, G: 6, B: 13, A: uint8(205 * openAmount)})
	modal := toolMenuModalBounds()
	panel := rl.Color{R: 4, G: 12, B: 23, A: uint8(246 * openAmount)}
	rl.DrawRectangleRounded(modal, .018, 10, panel)
	menu.drawAtmosphere(modal)
	rl.DrawRectangleRoundedLinesEx(modal, .018, 10, 2, rl.Color{R: 32, G: 111, B: 154, A: 210})
	header := rl.Rectangle{X: modal.X + 2, Y: modal.Y + 2, Width: modal.Width - 4, Height: 68}
	rl.DrawRectangleGradientV(int32(header.X), int32(header.Y), int32(header.Width), int32(header.Height), rl.Color{R: 8, G: 26, B: 44, A: 250}, rl.Color{R: 3, G: 13, B: 24, A: 235})
	simpleui.DrawTextStyled("IC-SDR", header.X+183, header.Y+19, 19, simpleui.FontSemiBold, colors.cyan)
	drawCenteredStyled(menuText("CENTRO DE CONTROL", "CONTROL CENTER"), header, 28, simpleui.FontSemiBold, colors.text)
	version := buildinfo.DisplayVersion()
	simpleui.DrawTextStyled(version, header.X+265, header.Y+23, 13, simpleui.FontMono, colors.muted)
	rl.DrawLine(int32(modal.X+18), int32(modal.Y+70), int32(modal.X+modal.Width-18), int32(modal.Y+70), rl.Color{R: 25, G: 112, B: 155, A: 150})

	menu.drawCategory(0, menuText("VISTAS", "VIEWS"), colors.cyan)
	menu.drawCategory(1, menuText("DECODIFICADORES", "DECODERS"), rl.Color{R: 180, G: 105, B: 255, A: 255})
	menu.drawCategory(2, menuText("UTILIDADES", "TOOLS"), rl.Color{R: 255, G: 171, B: 35, A: 255})
	menu.drawCarousel()
	menu.drawThumbnailRail()
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
	drawCenteredStyled("←  "+i18n.Display(i18n.Source("text.bd62e781ec4e")), back, 17, simpleui.FontSemiBold, simpleui.EnsureTextContrast(colors.text, backColor))
}

func (menu *ToolMenu) drawAtmosphere(modal rl.Rectangle) {
	// A quiet live-instrument backdrop: grid, spectrum and travelling scan light.
	grid := rl.Color{R: 20, G: 78, B: 110, A: 42}
	for x := modal.X + 20; x < modal.X+modal.Width; x += 58 {
		rl.DrawLine(int32(x), int32(modal.Y+72), int32(x), int32(modal.Y+modal.Height-58), grid)
	}
	for y := modal.Y + 88; y < modal.Y+modal.Height-55; y += 48 {
		rl.DrawLine(int32(modal.X+12), int32(y), int32(modal.X+modal.Width-12), int32(y), grid)
	}
	baseY := modal.Y + 116
	previous := rl.Vector2{X: modal.X + 210, Y: baseY}
	for x := float32(0); x <= modal.Width-235; x += 8 {
		wave := float32(math.Sin(float64(x)*.045+rl.GetTime()*.8)) * 8
		wave += float32(math.Sin(float64(x)*.137)) * 4
		current := rl.Vector2{X: modal.X + 210 + x, Y: baseY - 18 - wave}
		rl.DrawLineEx(previous, current, 1.2, rl.Color{R: 23, G: 181, B: 232, A: 80})
		previous = current
	}
	scanX := modal.X + 220 + float32(math.Mod(rl.GetTime()*95, float64(modal.Width-250)))
	rl.DrawRectangleGradientH(int32(scanX-32), int32(modal.Y+72), 64, int32(modal.Height-130), rl.Color{}, rl.Color{R: 30, G: 202, B: 255, A: 22})
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
	modal := toolMenuModalBounds()
	b := menu.categoryBounds(row)
	active := row == menu.category
	hovered := menu.hoverCategory == row
	fill := rl.Color{R: 7, G: 18, B: 31, A: 235}
	if active {
		fill = blendRGBA(fill, accent, .22)
	}
	if hovered {
		fill = blendRGBA(fill, accent, .10)
	}
	for glow := float32(8); active && glow >= 2; glow -= 2 {
		g := rl.Rectangle{X: b.X - glow/2, Y: b.Y - glow/2, Width: b.Width + glow, Height: b.Height + glow}
		c := accent
		c.A = uint8(13 * (10 - glow))
		rl.DrawRectangleRoundedLinesEx(g, .09, 7, 1, c)
	}
	rl.DrawRectangleRounded(b, .09, 7, fill)
	border := rl.Color{R: 55, G: 82, B: 102, A: 230}
	width := float32(1.2)
	if active {
		border, width = accent, 2.6
	}
	rl.DrawRectangleRoundedLinesEx(b, .09, 7, width, border)
	iconCenter := rl.Vector2{X: b.X + 35, Y: b.Y + b.Height/2}
	menu.drawCategoryGlyph(row, iconCenter, accent)
	simpleui.DrawTextStyled(label, b.X+64, b.Y+28, 16, simpleui.FontSemiBold, func() rl.Color {
		if active {
			return accent
		}
		return colors.text
	}())
	count := len(menu.indicesForRow(row))
	simpleui.DrawTextStyled(menuText("MÓDULOS", "MODULES")+"  "+strconv.Itoa(count), b.X+64, b.Y+57, 11, simpleui.FontMono, colors.muted)
	if active {
		rl.DrawRectangleRounded(rl.Rectangle{X: b.X, Y: b.Y + 18, Width: 4, Height: b.Height - 36}, 1, 3, accent)
	}
	_ = modal
}

func (menu *ToolMenu) drawCategoryGlyph(row int, center rl.Vector2, accent rl.Color) {
	switch row {
	case 0:
		for i, h := range []float32{17, 30, 23, 39, 18} {
			x := center.X - 24 + float32(i)*11
			rl.DrawRectangleRounded(rl.Rectangle{X: x, Y: center.Y + h/2 - h, Width: 7, Height: h}, .35, 3, accent)
		}
	case 1:
		rl.DrawCircleLines(int32(center.X), int32(center.Y), 7, accent)
		for r := float32(15); r <= 25; r += 10 {
			rl.DrawCircleLines(int32(center.X), int32(center.Y), r, rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 155})
		}
		rl.DrawLineEx(rl.Vector2{X: center.X, Y: center.Y + 7}, rl.Vector2{X: center.X, Y: center.Y + 28}, 3, accent)
	case 2:
		rl.DrawCircleLines(int32(center.X), int32(center.Y), 19, accent)
		for a := 0; a < 4; a++ {
			angle := float64(a) * math.Pi / 2
			p1 := rl.Vector2{X: center.X + float32(math.Cos(angle))*22, Y: center.Y + float32(math.Sin(angle))*22}
			p2 := rl.Vector2{X: center.X + float32(math.Cos(angle))*29, Y: center.Y + float32(math.Sin(angle))*29}
			rl.DrawLineEx(p1, p2, 5, accent)
		}
		rl.DrawCircle(int32(center.X), int32(center.Y), 7, accent)
	}
}

func (menu *ToolMenu) drawCarousel() {
	indices := menu.indicesForRow(menu.category)
	for distance := len(indices); distance >= 0; distance-- {
		for _, index := range indices {
			delta := float32(menu.localIndex(index)) - menu.carousel
			if int(math.Abs(float64(delta))+.5) != distance || math.Abs(float64(delta)) > 2.65 {
				continue
			}
			menu.drawItem(index, delta)
		}
	}
}

func (menu *ToolMenu) drawItem(index int, delta float32) {
	item := toolMenuItems[index]
	bounds := menu.itemBounds(index)
	if bounds.Width <= 0 {
		return
	}
	selected := index == menu.focusIndex
	hovered := index == menu.hoverItem
	accent := menu.rowAccent(item.row)
	fill := rl.Color{R: 5, G: 14, B: 27, A: 250}
	if selected {
		fill = blendRGBA(fill, accent, .12)
	}
	if hovered {
		fill = blendRGBA(fill, accent, .08)
	}
	if selected {
		for glow := float32(13); glow >= 3; glow -= 2.5 {
			g := rl.Rectangle{X: bounds.X - glow/2, Y: bounds.Y - glow/2, Width: bounds.Width + glow, Height: bounds.Height + glow}
			c := accent
			c.A = uint8(8 * (16 - glow))
			rl.DrawRectangleRoundedLinesEx(g, .055, 8, 1.2, c)
		}
	}
	rl.DrawRectangleRounded(bounds, .055, 8, fill)
	border := rl.Color{R: 65, G: 100, B: 125, A: 220}
	borderWidth := float32(1.2)
	if selected {
		border, borderWidth = accent, 3
	}
	if hovered && !selected {
		border = mixColor(border, accent, .55)
		borderWidth = 2
	}
	rl.DrawRectangleRoundedLinesEx(bounds, .055, 8, borderWidth, border)
	imageHeight := bounds.Height - 76
	if selected {
		imageHeight = bounds.Height - 105
	}
	imageBounds := rl.Rectangle{X: bounds.X + 7, Y: bounds.Y + 7, Width: bounds.Width - 14, Height: imageHeight}
	menu.drawItemArt(item, imageBounds, selected)
	shadeY := imageBounds.Y + imageBounds.Height - 52
	rl.DrawRectangleGradientV(int32(imageBounds.X), int32(shadeY), int32(imageBounds.Width), 52, rl.Color{R: 3, G: 10, B: 20, A: 0}, rl.Color{R: 3, G: 10, B: 20, A: 245})
	labelY := bounds.Y + imageHeight + 16
	labelSize := int32(15)
	if selected {
		labelSize = 23
	}
	for labelSize > 11 && simpleui.MeasureTextStyled(i18n.Display(item.label), labelSize, simpleui.FontSemiBold).X > bounds.Width-18 {
		labelSize--
	}
	drawCenteredStyled(i18n.Display(item.label), rl.Rectangle{X: bounds.X + 7, Y: labelY, Width: bounds.Width - 14, Height: 30}, labelSize, simpleui.FontSemiBold, func() rl.Color {
		if selected {
			return accent
		}
		return colors.text
	}())
	if selected {
		drawCenteredStyled(menu.toolDescription(item), rl.Rectangle{X: bounds.X + 12, Y: bounds.Y + bounds.Height - 48, Width: bounds.Width - 24, Height: 23}, 12, simpleui.FontRegular, colors.muted)
		state := menuText("ABRIR", "OPEN")
		if item.id == menu.selected {
			state = menuText("ACTIVO", "ACTIVE")
		}
		pill := rl.Rectangle{X: bounds.X + bounds.Width - 82, Y: bounds.Y + 15, Width: 66, Height: 24}
		rl.DrawRectangleRounded(pill, .45, 8, rl.Color{R: 3, G: 18, B: 25, A: 225})
		drawCenteredStyled(state, pill, 10, simpleui.FontSemiBold, func() rl.Color {
			if item.id == menu.selected {
				return colors.green
			}
			return accent
		}())
	}
	_ = delta
}

func (menu *ToolMenu) drawItemArt(item toolMenuItem, bounds rl.Rectangle, selected bool) {
	if texture, ok := menu.icons[item.art]; ok {
		source := rl.Rectangle{Width: float32(texture.Width), Height: float32(texture.Height)}
		tint := rl.White
		if !selected {
			tint.A = 220
		}
		rl.DrawTexturePro(texture, source, bounds, rl.Vector2{}, 0, tint)
		return
	}
	center := rl.Vector2{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	if texture, ok := menu.icons[item.icon]; ok {
		source := rl.Rectangle{Width: float32(texture.Width), Height: float32(texture.Height)}
		rl.DrawTexturePro(texture, source, rl.Rectangle{X: center.X - 45, Y: center.Y - 45, Width: 90, Height: 90}, rl.Vector2{}, 0, rl.White)
		return
	}
	menu.drawCustomIcon(item.id, center, selected, item.row)
}

func (menu *ToolMenu) drawThumbnailRail() {
	indices := menu.indicesForRow(menu.category)
	area := menu.thumbnailRailBounds()
	rl.DrawRectangleRounded(area, .035, 8, rl.Color{R: 2, G: 9, B: 18, A: 238})
	rl.DrawRectangleRoundedLinesEx(area, .035, 8, 1.4, rl.Color{R: 35, G: 98, B: 132, A: 210})
	simpleui.DrawTextStyled(menuText("SELECCIÓN RÁPIDA", "QUICK SELECT"), area.X+18, area.Y+10, 11, simpleui.FontMono, colors.muted)

	start, end := menu.thumbnailVisibleRange()
	shown := end - start
	content := menu.thumbnailContentBounds()
	cardWidth := min(float32(151), (content.Width-float32(max(shown-1, 0))*10)/float32(max(shown, 1)))
	groupWidth := cardWidth*float32(shown) + float32(max(shown-1, 0))*10
	groupX := content.X + (content.Width-groupWidth)/2
	for slot, local := 0, start; local < end; slot, local = slot+1, local+1 {
		index := indices[local]
		b := rl.Rectangle{X: groupX + float32(slot)*(cardWidth+10), Y: content.Y, Width: cardWidth, Height: content.Height}
		item := toolMenuItems[index]
		active := index == menu.focusIndex
		hovered := index == menu.hoverItem
		accent := menu.rowAccent(item.row)
		fill := rl.Color{R: 6, G: 17, B: 29, A: 245}
		if active {
			fill = blendRGBA(fill, accent, .18)
		} else if hovered {
			fill = blendRGBA(fill, accent, .08)
		}
		rl.DrawRectangleRounded(b, .06, 7, fill)
		img := rl.Rectangle{X: b.X + 5, Y: b.Y + 5, Width: b.Width - 10, Height: b.Height - 39}
		menu.drawItemArt(item, img, active)
		border := rl.Color{R: 48, G: 79, B: 101, A: 230}
		borderWidth := float32(1)
		if active {
			border, borderWidth = accent, 2.5
		} else if hovered {
			border, borderWidth = mixColor(border, accent, .55), 1.8
		}
		rl.DrawRectangleRoundedLinesEx(b, .06, 7, borderWidth, border)
		rl.DrawRectangleGradientV(int32(img.X), int32(img.Y+img.Height-28), int32(img.Width), 28, rl.Color{}, rl.Color{R: 2, G: 9, B: 18, A: 225})
		label := i18n.Display(item.label)
		font := int32(13)
		for font > 9 && simpleui.MeasureTextStyled(label, font, simpleui.FontSemiBold).X > b.Width-10 {
			font--
		}
		drawCenteredStyled(label, rl.Rectangle{X: b.X + 3, Y: b.Y + b.Height - 31, Width: b.Width - 6, Height: 26}, font, simpleui.FontSemiBold, func() rl.Color {
			if active {
				return accent
			}
			return colors.muted
		}())
	}
	if len(indices) > thumbnailPageSize {
		for arrow := 0; arrow < 2; arrow++ {
			b := menu.thumbnailArrowBounds(arrow)
			fill := rl.Color{R: 8, G: 24, B: 39, A: 245}
			if menu.pressedThumbArrow == arrow {
				fill = blendRGBA(fill, menu.rowAccent(menu.category), .22)
			}
			rl.DrawRectangleRounded(b, .16, 6, fill)
			rl.DrawRectangleRoundedLinesEx(b, .16, 6, 1.5, menu.rowAccent(menu.category))
			center := rl.Vector2{X: b.X + b.Width/2, Y: b.Y + b.Height/2}
			tipX := center.X - 7
			outerX := center.X + 6
			if arrow == 1 {
				tipX, outerX = center.X+7, center.X-6
			}
			rl.DrawLineEx(rl.Vector2{X: outerX, Y: center.Y - 12}, rl.Vector2{X: tipX, Y: center.Y}, 4, menu.rowAccent(menu.category))
			rl.DrawLineEx(rl.Vector2{X: tipX, Y: center.Y}, rl.Vector2{X: outerX, Y: center.Y + 12}, 4, menu.rowAccent(menu.category))
		}
	}
}

func (menu *ToolMenu) drawCustomIcon(id string, center rl.Vector2, active bool, row int) {
	accent := colors.cyan
	if active {
		accent = menu.rowAccent(row)
	}
	switch id {
	case dabToolID:
		rl.DrawRectangleRounded(rl.Rectangle{X: center.X - 39, Y: center.Y - 25, Width: 78, Height: 50}, .14, 6, colors.panelAlt)
		rl.DrawRectangleRoundedLinesEx(rl.Rectangle{X: center.X - 39, Y: center.Y - 25, Width: 78, Height: 50}, .14, 6, 2, accent)
		drawCenteredStyled("DAB+", rl.Rectangle{X: center.X - 38, Y: center.Y - 22, Width: 76, Height: 44}, 16, simpleui.FontSemiBold, accent)
	case datvToolID:
		// Television screen + dipole and RF marks: this distinguishes amateur
		// digital television from a generic media-player icon.
		antennaBase := rl.Vector2{X: center.X, Y: center.Y - 19}
		rl.DrawLineEx(antennaBase, rl.Vector2{X: center.X - 20, Y: center.Y - 34}, 2.5, accent)
		rl.DrawLineEx(antennaBase, rl.Vector2{X: center.X + 20, Y: center.Y - 34}, 2.5, accent)
		rl.DrawCircleV(antennaBase, 3, colors.orange)
		for side := float32(-1); side <= 1; side += 2 {
			x := center.X + side*27
			rl.DrawLineEx(rl.Vector2{X: x, Y: center.Y - 31}, rl.Vector2{X: x + side*7, Y: center.Y - 26}, 2, colors.muted)
			rl.DrawLineEx(rl.Vector2{X: x + side*7, Y: center.Y - 26}, rl.Vector2{X: x, Y: center.Y - 21}, 2, colors.muted)
		}
		screen := rl.Rectangle{X: center.X - 36, Y: center.Y - 19, Width: 72, Height: 43}
		rl.DrawRectangleRounded(screen, .12, 6, colors.panelAlt)
		rl.DrawRectangleRoundedLinesEx(screen, .12, 6, 2.5, accent)
		// A tiny digital raster remains legible at menu size.
		for row := 0; row < 3; row++ {
			for column := 0; column < 5; column++ {
				if (row+column)%3 == 1 {
					continue
				}
				rl.DrawRectangle(int32(center.X-27+float32(column)*7), int32(center.Y-10+float32(row)*7), 4, 4, accent)
			}
		}
		rl.DrawTriangle(rl.Vector2{X: center.X + 12, Y: center.Y - 10}, rl.Vector2{X: center.X + 12, Y: center.Y + 12}, rl.Vector2{X: center.X + 30, Y: center.Y + 1}, colors.orange)
		rl.DrawLineEx(rl.Vector2{X: center.X - 18, Y: center.Y + 27}, rl.Vector2{X: center.X + 18, Y: center.Y + 27}, 3, colors.muted)
	case qrzToolID:
		rl.DrawCircleLines(int32(center.X-17), int32(center.Y), 27, accent)
		rl.DrawEllipseLines(int32(center.X-17), int32(center.Y), 11, 27, colors.muted)
		rl.DrawLineEx(rl.Vector2{X: center.X - 44, Y: center.Y}, rl.Vector2{X: center.X + 10, Y: center.Y}, 2, colors.muted)
		simpleui.DrawTextStyled("QRZ", center.X+13, center.Y-12, 17, simpleui.FontSemiBold, accent)
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
	case feedbackToolID:
		back := rl.Rectangle{X: center.X - 35, Y: center.Y - 21, Width: 59, Height: 43}
		rl.DrawRectangleRounded(back, .32, 8, rl.Color{R: 232, G: 14, B: 205, A: 255})
		front := rl.Rectangle{X: center.X - 19, Y: center.Y - 31, Width: 58, Height: 45}
		rl.DrawRectangleRounded(front, .32, 8, rl.Color{R: 255, G: 112, B: 33, A: 255})
		rl.DrawRectangleRoundedLinesEx(front, .32, 8, 4, rl.Color{R: 255, G: 211, B: 71, A: 255})
		drawCenteredStyled("!", front, 27, simpleui.FontSemiBold, rl.White)
	default:
		drawCentered("?", rl.Rectangle{X: center.X - 30, Y: center.Y - 25, Width: 60, Height: 50}, 24, accent)
	}
}

func (menu *ToolMenu) loadIcons() {
	if len(menu.icons) > 0 {
		return
	}
	for _, item := range toolMenuItems {
		for _, asset := range []struct{ key, path string }{
			{item.art, "assets/menu/" + item.art}, {item.icon, "assets/icons/" + item.icon},
		} {
			if asset.key == "" {
				continue
			}
			if _, exists := menu.icons[asset.key]; exists {
				continue
			}
			data, err := toolIconFiles.ReadFile(asset.path)
			if err != nil {
				continue
			}
			image := rl.LoadImageFromMemory(".png", data, int32(len(data)))
			if image == nil || image.Data == nil {
				continue
			}
			texture := rl.LoadTextureFromImage(image)
			rl.UnloadImage(image)
			rl.SetTextureFilter(texture, rl.FilterBilinear)
			menu.icons[asset.key] = texture
		}
	}
}

func (menu *ToolMenu) itemAt(point rl.Vector2) int {
	indices := menu.indicesForRow(menu.category)
	// The navigation rail gets priority because carousel cards may overlap it visually.
	start, end := menu.thumbnailVisibleRange()
	shown := end - start
	content := menu.thumbnailContentBounds()
	cardWidth := min(float32(151), (content.Width-float32(max(shown-1, 0))*10)/float32(max(shown, 1)))
	groupWidth := cardWidth*float32(shown) + float32(max(shown-1, 0))*10
	groupX := content.X + (content.Width-groupWidth)/2
	for slot, local := 0, start; local < end; slot, local = slot+1, local+1 {
		bounds := rl.Rectangle{X: groupX + float32(slot)*(cardWidth+10), Y: content.Y, Width: cardWidth, Height: content.Height}
		if rl.CheckCollisionPointRec(point, bounds) {
			return indices[local]
		}
	}
	best, bestDistance := -1, float32(999)
	for _, index := range indices {
		b := menu.itemBounds(index)
		if b.Width <= 0 || !rl.CheckCollisionPointRec(point, b) {
			continue
		}
		d := float32(math.Abs(float64(float32(menu.localIndex(index)) - menu.carousel)))
		if d < bestDistance {
			best, bestDistance = index, d
		}
	}
	return best
}

const thumbnailPageSize = 8

func (menu *ToolMenu) thumbnailRailBounds() rl.Rectangle {
	modal := toolMenuModalBounds()
	return rl.Rectangle{X: modal.X + 14, Y: modal.Y + 588, Width: modal.Width - 28, Height: 224}
}

func (menu *ToolMenu) thumbnailContentBounds() rl.Rectangle {
	area := menu.thumbnailRailBounds()
	return rl.Rectangle{X: area.X + 62, Y: area.Y + 31, Width: area.Width - 124, Height: area.Height - 42}
}

func (menu *ToolMenu) thumbnailVisibleRange() (int, int) {
	indices := menu.indicesForRow(menu.category)
	if len(indices) == 0 {
		return 0, 0
	}
	local := min(max(menu.localIndex(menu.focusIndex), 0), len(indices)-1)
	start := (local / thumbnailPageSize) * thumbnailPageSize
	return start, min(start+thumbnailPageSize, len(indices))
}

func (menu *ToolMenu) thumbnailArrowBounds(arrow int) rl.Rectangle {
	area := menu.thumbnailRailBounds()
	x := area.X + 10
	if arrow == 1 {
		x = area.X + area.Width - 52
	}
	return rl.Rectangle{X: x, Y: area.Y + 76, Width: 42, Height: 88}
}

func (menu *ToolMenu) thumbnailArrowAt(point rl.Vector2) int {
	if len(menu.indicesForRow(menu.category)) <= thumbnailPageSize {
		return -1
	}
	for arrow := 0; arrow < 2; arrow++ {
		if rl.CheckCollisionPointRec(point, menu.thumbnailArrowBounds(arrow)) {
			return arrow
		}
	}
	return -1
}

func (menu *ToolMenu) moveThumbnailPage(arrow int) {
	indices := menu.indicesForRow(menu.category)
	if len(indices) == 0 {
		return
	}
	local := menu.localIndex(menu.focusIndex)
	direction := -1
	if arrow == 1 {
		direction = 1
	}
	local = min(max(local+direction*thumbnailPageSize, 0), len(indices)-1)
	menu.focusIndex = indices[local]
	if menu.onSelectSound != nil {
		menu.onSelectSound()
	}
}

func (menu *ToolMenu) itemBounds(index int) rl.Rectangle {
	if index < 0 || index >= len(toolMenuItems) || toolMenuItems[index].row != menu.category {
		return rl.Rectangle{}
	}
	modal := toolMenuModalBounds()
	delta := float32(menu.localIndex(index)) - menu.carousel
	if math.Abs(float64(delta)) > 2.7 {
		return rl.Rectangle{}
	}
	centerX := modal.X + 820
	centerY := modal.Y + 365
	abs := float32(math.Abs(float64(delta)))
	selectedAmount := max(0, 1-min(abs, 1))
	width := 190 + selectedAmount*108
	height := 275 + selectedAmount*92
	x := centerX + delta*224 - width/2
	y := centerY - height/2 + abs*16
	if index == menu.hoverItem && selectedAmount < .2 {
		y -= 5
	}
	return rl.Rectangle{X: x, Y: y, Width: width, Height: height}
}

func (menu *ToolMenu) categoryBounds(row int) rl.Rectangle {
	modal := toolMenuModalBounds()
	return rl.Rectangle{X: modal.X + 18, Y: modal.Y + 105 + float32(row)*126, Width: 182, Height: 105}
}

func (menu *ToolMenu) categoryAt(point rl.Vector2) int {
	for row := 0; row < 3; row++ {
		if rl.CheckCollisionPointRec(point, menu.categoryBounds(row)) {
			return row
		}
	}
	return -1
}

func (menu *ToolMenu) indicesForRow(row int) []int {
	indices := make([]int, 0, 14)
	for index, item := range toolMenuItems {
		if item.row == row {
			indices = append(indices, index)
		}
	}
	return indices
}

func (menu *ToolMenu) localIndex(index int) int {
	for local, candidate := range menu.indicesForRow(menu.category) {
		if candidate == index {
			return local
		}
	}
	return 0
}

func (menu *ToolMenu) syncFocusToSelected() {
	menu.focusIndex = -1
	for index, item := range toolMenuItems {
		if item.id == menu.selected {
			menu.focusIndex = index
			menu.category = item.row
			break
		}
	}
	if menu.focusIndex < 0 {
		menu.setCategory(0)
		return
	}
	menu.carousel = float32(menu.localIndex(menu.focusIndex))
}

func (menu *ToolMenu) setCategory(row int) {
	menu.category = min(max(row, 0), 2)
	indices := menu.indicesForRow(menu.category)
	if len(indices) == 0 {
		menu.focusIndex = -1
		menu.carousel = 0
		return
	}
	menu.focusIndex = indices[0]
	for _, index := range indices {
		if toolMenuItems[index].id == menu.selected {
			menu.focusIndex = index
			break
		}
	}
	menu.carousel = float32(menu.localIndex(menu.focusIndex))
}

func (menu *ToolMenu) moveFocus(direction int) {
	indices := menu.indicesForRow(menu.category)
	if len(indices) == 0 {
		return
	}
	local := menu.localIndex(menu.focusIndex)
	local = (local + direction + len(indices)) % len(indices)
	menu.focusIndex = indices[local]
	if menu.onSelectSound != nil {
		menu.onSelectSound()
	}
}

func (menu *ToolMenu) updateAnimation() {
	if menu.focusIndex < 0 {
		return
	}
	target := float32(menu.localIndex(menu.focusIndex))
	speed := min(rl.GetFrameTime()*11, 1)
	menu.carousel += (target - menu.carousel) * speed
	if math.Abs(float64(target-menu.carousel)) < .002 {
		menu.carousel = target
	}
}

func menuText(spanish, english string) string {
	if i18n.Current() == "en" {
		return english
	}
	return spanish
}

func (menu *ToolMenu) toolDescription(item toolMenuItem) string {
	if item.row == 0 {
		return menuText("Visualización y control de señal", "Signal display and control")
	}
	if item.row == 1 {
		return menuText("Decodificación y análisis digital", "Digital decoding and analysis")
	}
	return menuText("Herramienta y recurso técnico", "Technical tool and resource")
}

func (menu *ToolMenu) backBounds() rl.Rectangle {
	modal := toolMenuModalBounds()
	return rl.Rectangle{X: modal.X + 14, Y: modal.Y + 13, Width: 145, Height: 43}
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
