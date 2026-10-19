package screens

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"time"

	"go-zero/internal/distance"
	"go-zero/internal/qrz"
	"go-zero/internal/resources"
	"go-zero/internal/voacap"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const qrzToolID = "QRZ_CALLBOOK"

type QRZPanel struct {
	simpleui.BaseElement
	screen                  *MainScreen
	controls                []simpleui.Element
	callsign                *simpleui.TextField
	search, open, showRoute *simpleui.Button
	showProfile             *simpleui.Button
	client                  *qrz.Client
	result                  qrz.Result
	status                  string
	statusColor             rl.Color
	lastLookup              time.Time
	photo                   rl.Texture2D
	photoPath               string
	flag                    rl.Texture2D
	flagPath                string
	authenticated           bool
	shortPathKM, longPathKM float64
	pathsAvailable          bool
	profileVisible          bool
	photoViewerOpen         bool
	photoViewerClosePressed bool
}

func NewQRZPanel(screen *MainScreen) *QRZPanel {
	p := &QRZPanel{
		BaseElement: simpleui.NewBaseElement("qrzPhotoViewer", 0, 0, designWidth, designHeight),
		screen:      screen,
		client:      qrz.New(qrz.UserDataPath("WebView2Profile"), qrz.UserDataPath("Cache")),
		status:      "ACCESO A QRZ NO COMPROBADO", statusColor: colors.muted,
	}
	p.callsign = simpleui.NewTextField("qrzCallsign", 370, toolY+75, 225, 42, "INDICATIVO", 20)
	p.callsign.SetMaxLength(16)
	p.callsign.OnSubmit(func(string) { p.lookup() })
	p.search = simpleui.NewButton("qrzSearch", 370, toolY+129, 108, 42, "BUSCAR", uiControlFontSize)
	p.search.SetColors(colors.blue, colors.border, colors.text)
	p.search.OnClick(p.lookup)
	p.open = simpleui.NewButton("qrzOpen", 487, toolY+129, 108, 42, "INICIAR SESIÓN", uiCaptionFontSize)
	p.open.SetColors(colors.panelAlt, colors.border, colors.text)
	p.open.OnClick(func() {
		if p.authenticated {
			p.logout()
			return
		}
		p.status, p.statusColor = "COMPLETE EL LOGIN EN LA VENTANA QRZ", colors.orange
		p.client.Open()
	})
	p.showProfile = simpleui.NewButton("qrzProfile", 1275, toolY+174, 132, 30, "VER PERFIL QRZ", uiCaptionFontSize)
	p.showProfile.SetColors(colors.panelAlt, colors.border, colors.text)
	p.showProfile.OnClick(p.openProfile)
	p.showRoute = simpleui.NewButton("qrzDistance", 1275, toolY+212, 132, 30, "MOSTRAR MAPA", uiCaptionFontSize)
	p.showRoute.SetColors(colors.panelAlt, colors.blue, colors.blue)
	p.showRoute.OnClick(p.openDistance)
	p.controls = []simpleui.Element{p.callsign, p.search, p.open, p.showProfile, p.showRoute}
	p.SetVisible(false)
	return p
}

func (p *QRZPanel) lookup() {
	call := strings.ToUpper(strings.TrimSpace(p.callsign.Text()))
	p.callsign.SetText(call)
	// A lookup belongs to a different station. Do not leave identity media from
	// the previous result visible while QRZ is resolving the new profile.
	p.clearFlag()
	p.status, p.statusColor = "CONSULTANDO QRZ · COMPLETE EL LOGIN SI SE SOLICITA", colors.orange
	p.search.SetEnabled(false)
	p.client.Lookup(call)
}

func (p *QRZPanel) poll() {
	for {
		select {
		case result := <-p.client.Results():
			if result.Err != nil {
				p.search.SetEnabled(true)
				p.status, p.statusColor = strings.ToUpper(result.Err.Error()), colors.orange
				continue
			}
			if result.AuthenticationRequired {
				p.authenticated = false
				p.updateSessionButton()
				p.status, p.statusColor = "INICIE SESIÓN EN LA VENTANA QRZ", colors.orange
				continue
			}
			p.search.SetEnabled(true)
			if result.Authenticated {
				p.authenticated = true
				p.updateSessionButton()
			}
			if result.Callsign == "" {
				p.status, p.statusColor = "SESIÓN QRZ ACTIVA", colors.green
				continue
			}
			p.result = result
			p.profileVisible = false
			p.updateProfileButton()
			p.updatePathDistances()
			p.lastLookup = time.Now()
			p.status, p.statusColor = "SESIÓN ACTIVA · "+result.Callsign, colors.green
			if result.ImagePath != "" && result.ImagePath != p.photoPath {
				if p.photo.ID != 0 {
					rl.UnloadTexture(p.photo)
				}
				p.photo = loadQRZTexture(result.ImagePath)
				if rl.IsTextureValid(p.photo) {
					p.photoPath = result.ImagePath
				} else {
					p.status, p.statusColor = "DATOS RECIBIDOS · NO SE PUDO CARGAR LA FOTO", colors.orange
				}
			}
			p.setFlag(result.FlagPath)
		default:
			p.showRoute.SetEnabled(p.result.Coordinates)
			p.showProfile.SetEnabled(p.authenticated && p.result.Callsign != "")
			return
		}
	}
}

func (p *QRZPanel) updateSessionButton() {
	if p.authenticated {
		p.open.SetLabel("CERRAR SESIÓN")
		p.open.SetColors(actionStopFill, colors.red, colors.text)
		return
	}
	p.open.SetLabel("INICIAR SESIÓN")
	p.open.SetColors(colors.panelAlt, colors.border, colors.text)
}

func (p *QRZPanel) clearFlag() {
	if rl.IsTextureValid(p.flag) {
		rl.UnloadTexture(p.flag)
	}
	p.flag = rl.Texture2D{}
	p.flagPath = ""
}

func (p *QRZPanel) setFlag(path string) {
	if path == p.flagPath && path != "" && rl.IsTextureValid(p.flag) {
		return
	}
	p.clearFlag()
	if path == "" {
		return
	}
	flag := loadQRZTexture(path)
	if !rl.IsTextureValid(flag) {
		return
	}
	p.flag = flag
	p.flagPath = path
}

func (p *QRZPanel) updateProfileButton() {
	if p.profileVisible {
		p.showProfile.SetLabel("OCULTAR PERFIL QRZ")
		p.showProfile.SetColors(colors.panelAlt, colors.orange, colors.orange)
		return
	}
	p.showProfile.SetLabel("VER PERFIL QRZ")
	p.showProfile.SetColors(colors.panelAlt, colors.border, colors.text)
}

func (p *QRZPanel) logout() {
	if err := p.client.Logout(); err != nil {
		p.status, p.statusColor = "NO SE PUDO CERRAR LA SESIÓN · "+strings.ToUpper(err.Error()), colors.red
		return
	}
	p.authenticated = false
	p.result = qrz.Result{}
	p.lastLookup = time.Time{}
	p.callsign.SetText("")
	if rl.IsTextureValid(p.photo) {
		rl.UnloadTexture(p.photo)
	}
	p.clearFlag()
	p.photo = rl.Texture2D{}
	p.photoPath = ""
	p.shortPathKM, p.longPathKM, p.pathsAvailable = 0, 0, false
	p.profileVisible = false
	p.photoViewerOpen = false
	p.updateProfileButton()
	p.status, p.statusColor = "SESIÓN QRZ CERRADA", colors.muted
	p.updateSessionButton()
}

func loadQRZTexture(path string) rl.Texture2D {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return rl.Texture2D{}
	}
	width, height, pixels, decoded := decodeQRZImage(data)
	if decoded {
		canvas := rl.GenImageColor(width, height, rl.Blank)
		texture := rl.LoadTextureFromImage(canvas)
		rl.UnloadImage(canvas)
		if rl.IsTextureValid(texture) {
			rl.UpdateTexture(texture, pixels)
			rl.SetTextureFilter(texture, rl.FilterBilinear)
			return texture
		}
	}
	// Keep WebP and future formats on Raylib's native fallback path.
	extension := ".jpg"
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		extension = ".png"
	} else if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		extension = ".webp"
	}
	image := rl.LoadImageFromMemory(extension, data, int32(len(data)))
	if !rl.IsImageValid(image) {
		return rl.Texture2D{}
	}
	texture := rl.LoadTextureFromImage(image)
	rl.UnloadImage(image)
	if rl.IsTextureValid(texture) {
		rl.SetTextureFilter(texture, rl.FilterBilinear)
	}
	return texture
}

func decodeQRZImage(data []byte) (int, int, []color.RGBA, bool) {
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, false
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return 0, 0, nil, false
	}
	pixels := make([]color.RGBA, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := decoded.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			pixels[y*width+x] = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
		}
	}
	return width, height, pixels, true
}

func (p *QRZPanel) openDistance() {
	if !p.result.Coordinates {
		p.status, p.statusColor = "LA FICHA NO CONTIENE COORDENADAS", colors.orange
		return
	}
	settings := voacap.Load(resources.WritablePath("config", "voacap.json"))
	localName := strings.TrimSpace(settings.QTH)
	if localName == "" {
		localName = "MI ESTACIÓN"
	}
	local := distance.Point{Lat: settings.Latitude, Lon: settings.Longitude}
	remote := distance.Point{Lat: p.result.Latitude, Lon: p.result.Longitude}
	if err := OpenQRZDistanceMap(localName, p.result.Callsign, local, remote); err != nil {
		p.status, p.statusColor = strings.ToUpper(err.Error()), colors.red
	}
}

func (p *QRZPanel) updatePathDistances() {
	p.shortPathKM, p.longPathKM, p.pathsAvailable = 0, 0, false
	if !p.result.Coordinates {
		return
	}
	settings := voacap.Load(resources.WritablePath("config", "voacap.json"))
	local := distance.Point{Lat: settings.Latitude, Lon: settings.Longitude}
	remote := distance.Point{Lat: p.result.Latitude, Lon: p.result.Longitude}
	p.shortPathKM = distance.Distance(local, remote) / 1000
	p.longPathKM = distance.LongDistance(local, remote) / 1000
	p.pathsAvailable = true
}

func (p *QRZPanel) openProfile() {
	if p.profileVisible {
		p.client.HideProfile()
		p.profileVisible = false
		p.updateProfileButton()
		p.status, p.statusColor = "PERFIL QRZ OCULTO", colors.green
		return
	}
	if p.result.Callsign == "" {
		p.status, p.statusColor = "CONSULTE PRIMERO UN INDICATIVO", colors.orange
		return
	}
	if err := p.client.ShowProfile(p.result.Callsign); err != nil {
		p.status, p.statusColor = "NO SE PUDO ABRIR EL PERFIL · "+strings.ToUpper(err.Error()), colors.red
		return
	}
	p.profileVisible = true
	p.updateProfileButton()
	p.status, p.statusColor = "PERFIL COMPLETO ABIERTO EN QRZ", colors.green
}

func (p *QRZPanel) SetVisible(visible bool) {
	p.BaseElement.SetVisible(visible)
	if !visible {
		p.photoViewerOpen = false
		p.photoViewerClosePressed = false
	}
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
}

func (p *QRZPanel) Update(input simpleui.Input) bool {
	if p.photoViewerOpen {
		return false
	}
	if input.Released && rl.IsTextureValid(p.photo) && input.Over(p.photoBounds()) {
		simpleui.PlayActivationFeedback()
		p.photoViewerOpen = true
		return true
	}
	return false
}

func (p *QRZPanel) Draw() {}

func (p *QRZPanel) OverlayOpen() bool { return p.photoViewerOpen }

func (p *QRZPanel) photoBounds() rl.Rectangle {
	// Fill the remaining right-hand column while preserving a small gap from
	// the route controls and the panel edge.
	return rl.Rectangle{X: 1413, Y: toolY + 34, Width: toolContentRight - 1421, Height: 216}
}

func (p *QRZPanel) photoViewerCloseBounds() rl.Rectangle {
	return rl.Rectangle{X: 1458, Y: 50, Width: 54, Height: 54}
}

func (p *QRZPanel) UpdateOverlay(input simpleui.Input) bool {
	if !p.photoViewerOpen {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		p.photoViewerOpen = false
		p.photoViewerClosePressed = false
		return true
	}
	closeBounds := p.photoViewerCloseBounds()
	if input.Pressed {
		p.photoViewerClosePressed = input.Over(closeBounds)
	}
	if input.Released {
		if p.photoViewerClosePressed && input.Over(closeBounds) {
			simpleui.PlayActivationFeedback()
			p.photoViewerOpen = false
		}
		p.photoViewerClosePressed = false
	}
	return true
}

func (p *QRZPanel) DrawOverlay() {
	if !p.photoViewerOpen || !rl.IsTextureValid(p.photo) {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 225})
	modal := rl.Rectangle{X: 88, Y: 38, Width: designWidth - 176, Height: designHeight - 76}
	rl.DrawRectangleRounded(modal, .015, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(modal, .015, 8, 2, colors.blue)
	title := "FOTO QRZ"
	if p.result.Callsign != "" {
		title += " · " + p.result.Callsign
	}
	simpleui.DrawTextStyled(title, modal.X+28, modal.Y+20, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	closeBounds := p.photoViewerCloseBounds()
	closeFill := colors.panelAlt
	if p.photoViewerClosePressed {
		closeFill = mixColor(closeFill, colors.red, .25)
	}
	rl.DrawRectangleRounded(closeBounds, .18, 6, closeFill)
	rl.DrawRectangleRoundedLinesEx(closeBounds, .18, 6, 1, colors.red)
	drawCentered("X", closeBounds, 21, colors.red)

	available := rl.Rectangle{X: modal.X + 28, Y: modal.Y + 78, Width: modal.Width - 56, Height: modal.Height - 106}
	scale := min(available.Width/float32(p.photo.Width), available.Height/float32(p.photo.Height))
	destination := rl.Rectangle{
		X:      available.X + (available.Width-float32(p.photo.Width)*scale)/2,
		Y:      available.Y + (available.Height-float32(p.photo.Height)*scale)/2,
		Width:  float32(p.photo.Width) * scale,
		Height: float32(p.photo.Height) * scale,
	}
	rl.DrawTexturePro(p.photo, rl.Rectangle{Width: float32(p.photo.Width), Height: float32(p.photo.Height)}, destination, rl.Vector2{}, 0, rl.White)
}

func (p *QRZPanel) DrawPanel() {
	p.poll()
	drawPanel(toolContentX, toolY, toolContentRight-toolContentX, toolH)
	leftDivider, rightDivider := float32(615), float32(1250)
	rl.DrawLineEx(rl.Vector2{X: leftDivider, Y: toolY + 15}, rl.Vector2{X: leftDivider, Y: toolY + 250}, 1, colors.border)
	rl.DrawLineEx(rl.Vector2{X: rightDivider, Y: toolY + 15}, rl.Vector2{X: rightDivider, Y: toolY + 250}, 1, colors.border)

	simpleui.DrawTextStyled("QRZ CALLBOOK", 370, toolY+10, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	rl.DrawCircle(379, int32(toolY+47), 6, p.statusColor)
	simpleui.DrawTextStyled(clipDAB(p.status, 34), 393, toolY+36, uiCaptionFontSize, simpleui.FontSemiBold, p.statusColor)
	simpleui.DrawText("INDICATIVO", 370, toolY+59, uiCaptionFontSize, colors.muted)
	last := "Sin consultas en esta sesión"
	if !p.lastLookup.IsZero() {
		last = "Última consulta: " + p.lastLookup.Format("15:04:05")
	}
	simpleui.DrawText(last, 370, toolY+195, uiCaptionFontSize, colors.muted)
	simpleui.DrawText("Login y Cloudflare en ventana integrada", 370, toolY+221, uiCaptionFontSize, colors.muted)

	simpleui.DrawTextStyled("RESULTADO", 640, toolY+10, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	if p.result.Callsign == "" {
		simpleui.DrawTextStyled("Introduzca un indicativo para consultar QRZ", 640, toolY+78, uiMinimumFontSize, simpleui.FontSemiBold, colors.muted)
		simpleui.DrawText("La sesión del navegador se conservará para próximos usos.", 640, toolY+108, uiBodyFontSize, colors.muted)
	} else {
		simpleui.DrawTextStyled(p.result.Callsign, 640, toolY+43, 31, simpleui.FontSemiBold, colors.text)
		country := p.result.Country
		if country == "" {
			country = "—"
		}
		countryX := float32(820)
		if rl.IsTextureValid(p.flag) {
			const flagHeight = float32(30)
			flagWidth := flagHeight * float32(p.flag.Width) / float32(p.flag.Height)
			flagWidth = min(flagWidth, 48)
			rl.DrawTexturePro(p.flag, rl.Rectangle{Width: float32(p.flag.Width), Height: float32(p.flag.Height)}, rl.Rectangle{X: countryX, Y: toolY + 43, Width: flagWidth, Height: flagHeight}, rl.Vector2{}, 0, rl.White)
			countryX += flagWidth + 10
		}
		countryLabel := country
		if p.result.Continent != "" {
			countryLabel += " · " + p.result.Continent
		}
		simpleui.DrawTextStyled(clipDAB(countryLabel, 35), countryX, toolY+57, uiMinimumFontSize, simpleui.FontSemiBold, colors.muted)
		simpleui.DrawTextStyled(clipDAB(p.result.Name, 48), 640, toolY+93, 17, simpleui.FontSemiBold, colors.text)
		location := strings.Trim(strings.Join([]string{p.result.City, p.result.Region}, " · "), " ·")
		simpleui.DrawText(clipDAB(location, 66), 640, toolY+124, uiMinimumFontSize, colors.text)
		if p.result.Member != "" {
			drawQRZBadge(p.result.Member, 640, toolY+153, colors.green)
		}
		if p.result.Lookups != "" {
			drawQRZBadge(p.result.Lookups+" CONSULTAS", 765, toolY+153, colors.blue)
		}
		rl.DrawLineEx(rl.Vector2{X: 640, Y: toolY + 197}, rl.Vector2{X: 1225, Y: toolY + 197}, 1, colors.border)
		locator := p.result.Locator
		if locator == "" {
			locator = "—"
		}
		simpleui.DrawTextStyled("LOCATOR", 640, toolY+214, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
		simpleui.DrawText(locator, 710, toolY+214, uiMinimumFontSize, colors.text)
		if p.result.Coordinates {
			simpleui.DrawText(fmt.Sprintf("LAT  %.6f", p.result.Latitude), 825, toolY+214, uiMinimumFontSize, colors.text)
			simpleui.DrawText(fmt.Sprintf("LON  %.6f", p.result.Longitude), 1030, toolY+214, uiMinimumFontSize, colors.text)
		}
	}

	simpleui.DrawTextStyled("UBICACIÓN Y DISTANCIA", 1275, toolY+10, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("LOCAL", 1275, toolY+52, uiCaptionFontSize, colors.muted)
	simpleui.DrawTextStyled("MI ESTACIÓN", 1328, toolY+48, uiBodyFontSize, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText("REMOTO", 1275, toolY+82, uiCaptionFontSize, colors.muted)
	remote := "—"
	if p.result.Callsign != "" {
		remote = p.result.Callsign
	}
	simpleui.DrawTextStyled(clipDAB(remote, 11), 1328, toolY+78, uiBodyFontSize, simpleui.FontSemiBold, colors.text)
	coordinateColor, coordinateText := colors.muted, "Sin coordenadas"
	if p.result.Coordinates {
		coordinateColor, coordinateText = colors.green, "Coordenadas OK"
	}
	rl.DrawCircle(1281, int32(toolY+119), 5, coordinateColor)
	simpleui.DrawText(coordinateText, 1294, toolY+109, uiBodyFontSize, coordinateColor)
	if p.pathsAvailable {
		simpleui.DrawTextStyled("RUTA CORTA", 1275, toolY+137, uiCaptionFontSize, simpleui.FontSemiBold, colors.muted)
		simpleui.DrawTextStyled(fmt.Sprintf("%.0f km", p.shortPathKM), 1353, toolY+137, uiBodyFontSize, simpleui.FontSemiBold, colors.text)
		simpleui.DrawTextStyled("RUTA LARGA", 1275, toolY+157, uiCaptionFontSize, simpleui.FontSemiBold, colors.muted)
		simpleui.DrawTextStyled(fmt.Sprintf("%.0f km", p.longPathKM), 1353, toolY+157, uiBodyFontSize, simpleui.FontSemiBold, colors.text)
	}
	photoBounds := p.photoBounds()
	p.drawPhoto(photoBounds.X, photoBounds.Y, photoBounds.Width, photoBounds.Height)
	if rl.IsTextureValid(p.photo) && rl.CheckCollisionPointRec(simpleui.MousePosition(), photoBounds) {
		rl.DrawRectangleLinesEx(photoBounds, 2, colors.cyan)
	}
}

func drawQRZBadge(label string, x, y float32, color rl.Color) {
	width := simpleui.MeasureTextStyled(label, uiCaptionFontSize, simpleui.FontSemiBold).X + 20
	bounds := rl.Rectangle{X: x, Y: y, Width: width, Height: 27}
	fill := rl.Color{R: color.R, G: color.G, B: color.B, A: 35}
	rl.DrawRectangleRounded(bounds, .15, 4, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .15, 4, 1, color)
	simpleui.DrawTextStyled(label, x+10, y+6, uiCaptionFontSize, simpleui.FontSemiBold, color)
}

func (p *QRZPanel) drawPhoto(x, y, width, height float32) {
	bounds := rl.Rectangle{X: x, Y: y, Width: width, Height: height}
	rl.DrawRectangleRec(bounds, colors.panelAlt)
	rl.DrawRectangleLinesEx(bounds, 1, colors.border)
	if p.photo.ID == 0 {
		simpleui.DrawText("FOTO QRZ", x+23, y+62, uiCaptionFontSize, colors.muted)
		return
	}
	scale := min(width/float32(p.photo.Width), height/float32(p.photo.Height))
	destination := rl.Rectangle{X: x + (width-float32(p.photo.Width)*scale)/2, Y: y + (height-float32(p.photo.Height)*scale)/2, Width: float32(p.photo.Width) * scale, Height: float32(p.photo.Height) * scale}
	rl.DrawTexturePro(p.photo, rl.Rectangle{Width: float32(p.photo.Width), Height: float32(p.photo.Height)}, destination, rl.Vector2{}, 0, rl.White)
}

func (p *QRZPanel) Close() {
	if p.photo.ID != 0 {
		rl.UnloadTexture(p.photo)
		p.photo = rl.Texture2D{}
	}
	p.clearFlag()
	p.client.Close()
}
