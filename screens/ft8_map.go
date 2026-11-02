package screens

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go-zero/internal/dxcc"
	"go-zero/internal/ft8"
	"go-zero/internal/localtime"
	"go-zero/internal/resources"
	"go-zero/simpleui"

	"github.com/biter777/countries"
	rl "github.com/gen2brain/raylib-go/raylib"
)

//go:embed assets/flags/atlas.png
var ft8FlagAtlasPNG []byte

//go:embed assets/flags/dxcc-flags.json
var ft8DXCCFlagsJSON []byte

var (
	ft8MapModal     = rl.Rectangle{X: 7, Y: 5, Width: 1586, Height: 890}
	ft8MapBounds    = rl.Rectangle{X: 32, Y: 142, Width: 1080, Height: 718}
	ft8MapMax       = rl.Rectangle{X: 1466, Y: 22, Width: 48, Height: 48}
	ft8MapClose     = rl.Rectangle{X: 1524, Y: 22, Width: 48, Height: 48}
	ft8MapBack      = rl.Rectangle{X: 1390, Y: 586, Width: 164, Height: 26}
	ft8RegionClear  = rl.Rectangle{X: 1448, Y: 299, Width: 108, Height: 28}
	ft8DXCCOnce     sync.Once
	ft8DXCCData     *dxcc.Database
	ft8FlagOnce     sync.Once
	ft8FlagAtlas    rl.Texture2D
	ft8FlagIndex    map[string]int
	ft8DXCCFlagOnce sync.Once
	ft8DXCCFlags    map[string]string
)

type ft8MapPoint struct {
	decode      ft8.Decode
	call, grid  string
	lat, lon    float64
	hasPosition bool
}

type ft8MapStation struct {
	call, grid   string
	lat, lon     float64
	hasPosition  bool
	latest       ft8.Decode
	messageCount int
	selected     bool
	provisional  bool
}

type ft8ActivityEvent struct {
	decode                ft8.Decode
	from, to, kind, phase string
}

type ft8QSOStats struct {
	total, active, completed int
}

type ft8QSOConversation struct {
	first, last  time.Time
	callA, callB string
	phase        string
	messages     int
}

const (
	ft8QSOReply       = "reply"
	ft8QSOReport      = "report"
	ft8QSORogerReport = "roger_report"
	ft8QSOConfirmed   = "confirmed"
	ft8QSOComplete    = "complete"
	ft8MapStyleRadar  = "radar"
	ft8MapStyleQSL    = "qsl"
)

func validFT8MapStyle(style string) bool {
	return style == ft8MapStyleRadar || style == ft8MapStyleQSL
}

func (p *FT8Panel) openMap(index int) {
	if index < 0 || index >= len(p.status.Decodes) {
		return
	}
	p.openExternalViewer(index)
}

func (p *FT8Panel) refreshOpenMap() {
	selectedKey := ""
	if p.mapSelected >= 0 && p.mapSelected < len(p.mapDecodes) {
		selectedKey = ft8DecodeKey(p.mapDecodes[p.mapSelected])
	}
	p.mapDecodes = append(p.mapDecodes[:0], p.status.Decodes...)
	p.mapSelected = -1
	for index, decode := range p.mapDecodes {
		if ft8DecodeKey(decode) == selectedKey {
			p.mapSelected = index
			break
		}
	}
}

func ft8DecodeKey(decode ft8.Decode) string {
	return fmt.Sprintf("%d|%s|%.1f", decode.Slot.Unix(), decode.Message, decode.Frequency)
}

func (p *FT8Panel) focusMapSelection() {
	if p.mapSelected < 0 || p.mapSelected >= len(p.mapDecodes) {
		p.mapLat, p.mapLon, p.mapSpan = 15, 0, 300
		return
	}
	// A global view guarantees that every station remains visible. Selection
	// is expressed through marker emphasis rather than hiding distant context.
	p.mapLat, p.mapLon, p.mapSpan = 10, 0, 360
}

func (p *FT8Panel) UpdateOverlay(input simpleui.Input) bool {
	if !p.mapOpen {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		p.mapOpen = false
		return true
	}
	if !p.viewerMode && input.Pressed && input.Over(ft8MapClose) {
		p.mapOpen = false
		if p.viewerMode {
			simpleui.RequestClose()
		}
		return true
	}
	if !p.viewerMode && input.Pressed && input.Over(ft8MapMax) {
		if rl.IsWindowMaximized() {
			rl.RestoreWindow()
		} else {
			rl.MaximizeWindow()
		}
		return true
	}
	if input.Pressed && input.Over(ft8MapBack) {
		p.mapSelected = -1
		return true
	}
	regionBounds := rl.Rectangle{X: 1140, Y: 304, Width: 416, Height: 268}
	if input.Over(regionBounds) && input.Wheel != 0 {
		regionCount := len(p.ft8RegionActivity(p.ft8MapStations()))
		p.mapRegionOffset = min(max(0, p.mapRegionOffset-int(input.Wheel)*2), max(0, regionCount-8))
		return true
	}
	if input.Pressed && p.mapRegionFilter != "" && input.Over(ft8RegionClear) {
		p.mapRegionFilter = ""
		p.mapSelected = -1
		return true
	}
	if input.Pressed && input.Pointer.X >= 1140 && input.Pointer.X <= 1556 && input.Pointer.Y >= 359 && input.Pointer.Y < 559 {
		regions := p.stableFT8RegionActivity(p.ft8MapStations(), time.Now())
		row := int((input.Pointer.Y - 359) / 25)
		index := p.mapRegionOffset + row
		if row >= 0 && row < 8 && index >= 0 && index < len(regions) {
			if p.mapRegionFilter == regions[index].name {
				p.mapRegionFilter = ""
			} else {
				p.mapRegionFilter = regions[index].name
			}
			p.mapSelected = -1
			return true
		}
	}
	if input.Pressed && input.Pointer.Y >= 91 && input.Pointer.Y <= 125 {
		switch {
		case input.Pointer.X >= 32 && input.Pointer.X <= 112:
			p.mapWindow = 5 * time.Minute
		case input.Pointer.X >= 120 && input.Pointer.X <= 205:
			p.mapWindow = 15 * time.Minute
		case input.Pointer.X >= 213 && input.Pointer.X <= 288:
			p.mapWindow = time.Hour
		case input.Pointer.X >= 296 && input.Pointer.X <= 390:
			p.mapWindow = 0
		case input.Pointer.X >= 438 && input.Pointer.X <= 520:
			p.mapFilter = "all"
		case input.Pointer.X >= 528 && input.Pointer.X <= 598:
			p.mapFilter = "cq"
		case input.Pointer.X >= 606 && input.Pointer.X <= 710:
			p.mapFilter = "calls"
		case input.Pointer.X >= 718 && input.Pointer.X <= 916:
			p.mapOnlyPosition = !p.mapOnlyPosition
		case input.Pointer.X >= 932 && input.Pointer.X <= 1012:
			p.setFT8MapStyle(ft8MapStyleRadar)
		case input.Pointer.X >= 1020 && input.Pointer.X <= 1112:
			p.setFT8MapStyle(ft8MapStyleQSL)
		}
		p.mapNextRegionRank = time.Time{}
		return true
	}
	inside := input.Over(ft8MapBounds)
	if inside && input.Wheel != 0 {
		p.mapLat, p.mapLon, p.mapSpan = zoomMap(p.mapLat, p.mapLon, p.mapSpan, input.Pointer, ft8MapBounds, input.Wheel)
	}
	if inside && input.Pressed {
		p.mapDragging = true
		p.mapDragLast = input.Pointer
	}
	if p.mapDragging && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		delta := rl.Vector2Subtract(input.Pointer, p.mapDragLast)
		p.mapLat, p.mapLon = panMap(p.mapLat, p.mapLon, p.mapSpan, delta, ft8MapBounds)
		p.mapLat, p.mapLon, p.mapSpan = clampMapView(p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds)
		p.mapDragLast = input.Pointer
	}
	if p.mapDragging && input.Released {
		p.mapDragging = false
		p.selectFT8MapMarker(input.Pointer)
	}
	return true
}

func (p *FT8Panel) selectFT8MapMarker(pointer rl.Vector2) {
	stations := p.visibleFT8MapStations()
	// A grouped contact first behaves as a geographic drill-down. At regional
	// zoom, a second click selects its newest member; coincident locators can
	// therefore still be inspected instead of forming a dead visual element.
	contacts := p.ft8RadarContacts(stations, true, false)
	for _, cluster := range clusterFT8RadarContacts(contacts, 27) {
		if len(cluster.contacts) < 3 || rl.Vector2Distance(pointer, cluster.center) > 22 {
			continue
		}
		if p.mapSpan > 70 {
			lat, lon := ft8RadarClusterLocation(cluster)
			p.mapLat, p.mapLon, p.mapSpan = clampMapView(lat, lon, min(float64(70), p.mapSpan*.35), ft8MapBounds)
			p.mapSelected = -1
			return
		}
		representative := ft8RadarClusterRepresentative(cluster)
		p.selectFT8StationCall(representative.call)
		return
	}

	// Radar Vivo groups very close stations into a single visual contact.  A
	// slightly larger hit radius keeps every member of that contact selectable
	// without making the individual beacons visually oversized.
	best, bestDistance := -1, float32(28)
	for _, station := range stations {
		if !station.hasPosition {
			continue
		}
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, station.lat, station.lon, ft8MapBounds)
		distance := rl.Vector2Distance(pointer, position)
		if distance < bestDistance {
			for index, decode := range p.mapDecodes {
				if ft8MessageCall(decode.Message) == station.call {
					best, bestDistance = index, distance
					break
				}
			}
		}
	}
	if best >= 0 {
		p.mapSelected = best
	}
}

func (p *FT8Panel) selectFT8StationCall(call string) bool {
	for index, decode := range p.mapDecodes {
		if ft8MessageCall(decode.Message) == call {
			p.mapSelected = index
			return true
		}
	}
	return false
}

func (p *FT8Panel) DrawOverlay() {
	if !p.mapOpen {
		return
	}
	modal := ft8MapModal
	if p.viewerMode {
		modal = rl.Rectangle{Width: designWidth, Height: designHeight}
		rl.DrawRectangleRec(modal, colors.panel)
	} else {
		rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{R: 0, G: 0, B: 0, A: 218})
		rl.DrawRectangleRounded(modal, .022, 10, colors.panel)
		rl.DrawRectangleRoundedLinesEx(modal, .022, 10, 2, colors.cyan)
	}
	simpleui.DrawTextStyled("ACTIVIDAD FT8 GLOBAL", 32, 25, 23, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("Actividad recibida en esta estación · posición estimada mediante locator Maidenhead", 32, 57, uiCaptionFontSize, colors.muted)
	clockText, clockColor := ft8ClockPresentation(p.status)
	simpleui.DrawTextStyled("●  EN VIVO", 1175, 38, uiCaptionFontSize, simpleui.FontSemiBold, colors.green)
	simpleui.DrawText(clockText, 1270, 38, 10, clockColor)
	if !p.viewerMode {
		drawFT8WindowButton(ft8MapMax, "maximize", rl.IsWindowMaximized())
		drawFT8WindowButton(ft8MapClose, "close", false)
	}
	p.drawFT8MapFilters()

	drawGeoMap(p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds)
	drawMapGrid(p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds)
	rl.DrawRectangleLinesEx(ft8MapBounds, 1, colors.border)
	if p.ft8MapStyle() == ft8MapStyleQSL {
		p.drawFT8QSLStations()
		drawFT8QSLLegend()
	} else {
		p.drawFT8RadarStations()
		drawFT8RadarLegend()
	}
	p.drawFT8Dashboard()
}

func (p *FT8Panel) ft8MapStyle() string {
	if p.screen != nil && validFT8MapStyle(p.screen.ft8MapStyle) {
		return p.screen.ft8MapStyle
	}
	return ft8MapStyleRadar
}

func drawFT8WindowButton(bounds rl.Rectangle, kind string, restored bool) {
	background := colors.panelAlt
	rl.DrawRectangleRounded(bounds, .09, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .09, 6, 2, colors.border)
	line := simpleui.EnsureTextContrast(colors.text, background)
	if kind == "close" {
		rl.DrawLineEx(rl.Vector2{X: bounds.X + 16, Y: bounds.Y + 16}, rl.Vector2{X: bounds.X + 32, Y: bounds.Y + 32}, 2, line)
		rl.DrawLineEx(rl.Vector2{X: bounds.X + 32, Y: bounds.Y + 16}, rl.Vector2{X: bounds.X + 16, Y: bounds.Y + 32}, 2, line)
		return
	}
	if restored {
		rl.DrawRectangleLinesEx(rl.Rectangle{X: bounds.X + 19, Y: bounds.Y + 15, Width: 15, Height: 14}, 2, line)
		rl.DrawRectangleLinesEx(rl.Rectangle{X: bounds.X + 14, Y: bounds.Y + 20, Width: 15, Height: 14}, 2, line)
	} else {
		rl.DrawRectangleLinesEx(rl.Rectangle{X: bounds.X + 15, Y: bounds.Y + 15, Width: 18, Height: 18}, 2, line)
	}
}

func (p *FT8Panel) drawFT8MapFilters() {
	items := []struct {
		bounds rl.Rectangle
		label  string
		active bool
	}{
		{rl.Rectangle{X: 32, Y: 91, Width: 80, Height: 34}, "5 MIN", p.mapWindow == 5*time.Minute},
		{rl.Rectangle{X: 120, Y: 91, Width: 85, Height: 34}, "15 MIN", p.mapWindow == 15*time.Minute},
		{rl.Rectangle{X: 213, Y: 91, Width: 75, Height: 34}, "1 H", p.mapWindow == time.Hour},
		{rl.Rectangle{X: 296, Y: 91, Width: 94, Height: 34}, "SESIÓN", p.mapWindow == 0},
	}
	for _, item := range items {
		drawModalAction(item.bounds, item.label, map[bool]rl.Color{true: colors.cyan, false: colors.border}[item.active], item.active)
	}
	filters := []struct {
		bounds rl.Rectangle
		label  string
		active bool
	}{
		{rl.Rectangle{X: 438, Y: 91, Width: 82, Height: 34}, "TODOS", p.mapFilter == "all"},
		{rl.Rectangle{X: 528, Y: 91, Width: 70, Height: 34}, "CQ", p.mapFilter == "cq"},
		{rl.Rectangle{X: 606, Y: 91, Width: 104, Height: 34}, "LLAMADAS", p.mapFilter == "calls"},
		{rl.Rectangle{X: 718, Y: 91, Width: 198, Height: 34}, "SOLO CON POSICIÓN", p.mapOnlyPosition},
	}
	for _, item := range filters {
		drawModalAction(item.bounds, item.label, map[bool]rl.Color{true: colors.cyan, false: colors.border}[item.active], item.active)
	}
	style := ft8MapStyleRadar
	if p.screen != nil && validFT8MapStyle(p.screen.ft8MapStyle) {
		style = p.screen.ft8MapStyle
	}
	for _, item := range []struct {
		bounds rl.Rectangle
		label  string
		style  string
	}{
		{rl.Rectangle{X: 932, Y: 91, Width: 80, Height: 34}, "RADAR", ft8MapStyleRadar},
		{rl.Rectangle{X: 1020, Y: 91, Width: 92, Height: 34}, "QSL", ft8MapStyleQSL},
	} {
		active := style == item.style
		drawModalAction(item.bounds, item.label, map[bool]rl.Color{true: colors.orange, false: colors.border}[active], active)
	}
}

func (p *FT8Panel) setFT8MapStyle(style string) {
	if p.screen == nil || !validFT8MapStyle(style) || p.screen.ft8MapStyle == style {
		return
	}
	p.screen.ft8MapStyle = style
	if p.viewerMode {
		p.writeViewerStyleCommand(style)
		return
	}
	p.screen.markSettingsDirty()
}

func (p *FT8Panel) drawFT8RadarStations() {
	stations := p.visibleFT8MapStations()
	p.drawFT8LiveEvents()
	now := time.Now()
	selected := ft8MapStation{}
	for _, station := range stations {
		if station.selected {
			selected = station
			break
		}
	}
	homeLat, homeLon := p.ft8HomePosition()
	if selected.hasPosition {
		drawFT8Route(homeLat, homeLon, selected.lat, selected.lon, p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds, rl.Color{R: colors.cyan.R, G: colors.cyan.G, B: colors.cyan.B, A: 210})
	}

	// Draw context first. Radar Vivo collapses dense areas into compact radar
	// contacts so Europe remains readable at the global zoom level.
	contacts := p.ft8RadarContacts(stations, false, false)
	for _, cluster := range clusterFT8RadarContacts(contacts, 27) {
		if len(cluster.contacts) >= 3 {
			drawFT8RadarCluster(cluster, now)
			continue
		}
		for _, contact := range cluster.contacts {
			drawFT8StationMarker(contact.position, contact.station.latest, now, false)
		}
	}
	// One-off extreme-weak decodes remain visible, but as hollow provisional
	// contacts. They do not join clusters or influence the regional ranking.
	for _, contact := range p.ft8RadarContacts(stations, false, true) {
		if contact.station.provisional {
			drawFT8ProvisionalMarker(contact.position, contact.station.latest)
		}
	}
	home := projectMercator(p.mapLat, p.mapLon, p.mapSpan, homeLat, homeLon, ft8MapBounds)
	if rl.CheckCollisionPointRec(home, ft8MapBounds) {
		rl.DrawCircleV(home, 7, colors.cyan)
		rl.DrawCircleLines(int32(home.X), int32(home.Y), 11, colors.text)
		rl.DrawCircleLines(int32(home.X), int32(home.Y), 15, colors.cyan)
	}
	if selected.hasPosition {
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, selected.lat, selected.lon, ft8MapBounds)
		if rl.CheckCollisionPointRec(position, ft8MapBounds) {
			drawFT8StationMarker(position, selected.latest, now, true)
			drawMapCallout(selected.call+" · "+selected.grid, position.X+17, position.Y-8)
		}
	}
}

var (
	ft8QSLGold   = rl.Color{R: 202, G: 151, B: 55, A: 255}
	ft8QSLCream  = rl.Color{R: 238, G: 225, B: 190, A: 255}
	ft8QSLShadow = rl.Color{R: 4, G: 8, B: 12, A: 180}
)

func (p *FT8Panel) drawFT8QSLStations() {
	stations := p.visibleFT8MapStations()
	p.drawFT8LiveEvents()
	selected := ft8MapStation{}
	for _, station := range stations {
		if station.selected {
			selected = station
			break
		}
	}
	homeLat, homeLon := p.ft8HomePosition()
	if selected.hasPosition {
		drawFT8Route(homeLat, homeLon, selected.lat, selected.lon, p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds, withAlpha(ft8QSLGold, 190))
	}

	contacts := p.ft8RadarContacts(stations, false, false)
	for _, cluster := range clusterFT8RadarContacts(contacts, 29) {
		if len(cluster.contacts) >= 3 {
			drawFT8QSLCluster(cluster)
			continue
		}
		for _, contact := range cluster.contacts {
			drawFT8QSLBadge(contact.position, contact.station, false)
		}
	}
	for _, contact := range p.ft8RadarContacts(stations, false, true) {
		if contact.station.provisional {
			drawFT8QSLBadge(contact.position, contact.station, false)
		}
	}

	home := projectMercator(p.mapLat, p.mapLon, p.mapSpan, homeLat, homeLon, ft8MapBounds)
	if rl.CheckCollisionPointRec(home, ft8MapBounds) {
		rl.DrawCircleV(home, 8, withAlpha(colors.cyan, 45))
		rl.DrawCircleLines(int32(home.X), int32(home.Y), 9, ft8QSLGold)
		rl.DrawCircleV(home, 3, colors.cyan)
	}
	if selected.hasPosition {
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, selected.lat, selected.lon, ft8MapBounds)
		if rl.CheckCollisionPointRec(position, ft8MapBounds) {
			drawFT8QSLBadge(position, selected, true)
			drawFT8QSLCallout(position, selected)
		}
	}
}

func drawFT8QSLBadge(position rl.Vector2, station ft8MapStation, selected bool) {
	width, height := float32(28), float32(20)
	if selected {
		width, height = 36, 26
		rl.DrawCircleV(position, 25, withAlpha(ft8QSLGold, 42))
	}
	border := ft8QSLGold
	if station.provisional {
		border = withAlpha(colors.orange, 155)
	}
	card := rl.Rectangle{X: position.X - width/2, Y: position.Y - height/2 - 2, Width: width, Height: height}
	shadow := card
	shadow.X, shadow.Y = shadow.X+2, shadow.Y+3
	rl.DrawRectangleRounded(shadow, .18, 5, ft8QSLShadow)
	// The small lower pennant keeps the marker anchored to an exact map point,
	// while the rectangular face gives the flag enough pixels to stay legible.
	left := rl.Vector2{X: position.X - 5, Y: card.Y + card.Height - 1}
	right := rl.Vector2{X: position.X + 5, Y: card.Y + card.Height - 1}
	tip := rl.Vector2{X: position.X, Y: card.Y + card.Height + 8}
	rl.DrawTriangle(left, tip, right, border)
	rl.DrawTriangle(rl.Vector2{X: left.X + 2, Y: left.Y}, rl.Vector2{X: tip.X, Y: tip.Y - 3}, rl.Vector2{X: right.X - 2, Y: right.Y}, colors.panel)
	rl.DrawRectangleRounded(card, .18, 5, border)
	inner := rl.Rectangle{X: card.X + 2, Y: card.Y + 2, Width: card.Width - 4, Height: card.Height - 4}
	rl.DrawRectangleRounded(inner, .12, 4, withAlpha(colors.panel, map[bool]uint8{true: 250, false: 235}[selected]))
	flag := rl.Rectangle{X: inner.X + 2, Y: inner.Y + 1, Width: inner.Width - 4, Height: inner.Height - 2}
	drawFT8MiniFlag(ft8CallRegion(station.call), flag)
	if station.grid != "" {
		rl.DrawRectangle(int32(position.X-2), int32(tip.Y-3), 4, 2, colors.cyan)
	}
	if station.latest.SNR >= -10 {
		rl.DrawCircleV(rl.Vector2{X: card.X + 2, Y: card.Y + 2}, 2.5, ft8QSLGold)
		rl.DrawCircleLines(int32(card.X+2), int32(card.Y+2), 3.5, colors.panel)
	}
	if ft8DecodeIsCQ(station.latest) {
		x := card.X + card.Width + 2
		for offset := float32(-2); offset <= 2; offset += 4 {
			rl.DrawLineEx(rl.Vector2{X: x, Y: position.Y + offset}, rl.Vector2{X: x + 4, Y: position.Y + offset - 3}, 1.4, colors.orange)
		}
	}
	if event, ok := classifyFT8Activity(station.latest); ok && event.kind == ft8QSOComplete {
		center := rl.Vector2{X: card.X, Y: card.Y + card.Height}
		rl.DrawCircleV(center, 5, colors.panel)
		rl.DrawCircleLines(int32(center.X), int32(center.Y), 5, colors.green)
		a := rl.Vector2{X: center.X - 2.5, Y: center.Y}
		rl.DrawLineEx(a, rl.Vector2{X: a.X + 2, Y: a.Y + 2}, 1.5, colors.green)
		rl.DrawLineEx(rl.Vector2{X: a.X + 2, Y: a.Y + 2}, rl.Vector2{X: a.X + 5, Y: a.Y - 2}, 1.5, colors.green)
	}
	if station.provisional {
		center := rl.Vector2{X: card.X + card.Width, Y: card.Y}
		rl.DrawCircleV(center, 6, colors.panel)
		rl.DrawCircleLines(int32(center.X), int32(center.Y), 6, colors.orange)
		size := simpleui.MeasureTextStyled("?", 9, simpleui.FontSemiBold)
		simpleui.DrawTextStyled("?", center.X-size.X/2, center.Y-size.Y/2, 9, simpleui.FontSemiBold, colors.orange)
	}
}

func drawFT8QSLCluster(cluster ft8RadarCluster) {
	if len(cluster.contacts) == 0 {
		return
	}
	representative := ft8RadarClusterRepresentative(cluster)
	for _, offset := range []rl.Vector2{{X: -5, Y: -5}, {X: 5, Y: 2}} {
		card := rl.Rectangle{X: cluster.center.X - 14 + offset.X, Y: cluster.center.Y - 12 + offset.Y, Width: 28, Height: 20}
		rl.DrawRectangleRounded(card, .18, 5, withAlpha(ft8QSLGold, 165))
		card.X, card.Y, card.Width, card.Height = card.X+2, card.Y+2, card.Width-4, card.Height-4
		rl.DrawRectangleRounded(card, .12, 4, withAlpha(colors.panelAlt, 240))
	}
	drawFT8QSLBadge(cluster.center, representative, false)
	label := fmt.Sprintf("%d", len(cluster.contacts))
	size := simpleui.MeasureTextStyled(label, 10, simpleui.FontSemiBold)
	box := rl.Rectangle{X: cluster.center.X - size.X/2 - 4, Y: cluster.center.Y + 7, Width: size.X + 8, Height: 14}
	rl.DrawRectangleRounded(box, .4, 5, colors.panel)
	rl.DrawRectangleRoundedLinesEx(box, .4, 5, 1, ft8QSLGold)
	simpleui.DrawTextStyled(label, cluster.center.X-size.X/2, cluster.center.Y+8, 10, simpleui.FontSemiBold, ft8QSLCream)
}

func drawFT8QSLCallout(position rl.Vector2, station ft8MapStation) {
	width := float32(224)
	height := float32(48)
	x := position.X + 22
	if x+width > ft8MapBounds.X+ft8MapBounds.Width-8 {
		x = position.X - width - 22
	}
	y := min(max(position.Y-height/2, ft8MapBounds.Y+8), ft8MapBounds.Y+ft8MapBounds.Height-height-8)
	box := rl.Rectangle{X: x, Y: y, Width: width, Height: height}
	anchorX := x
	if x < position.X {
		anchorX = x + width
	}
	rl.DrawLineEx(position, rl.Vector2{X: anchorX, Y: y + height/2}, 1.5, ft8QSLGold)
	rl.DrawRectangleRounded(rl.Rectangle{X: x + 3, Y: y + 4, Width: width, Height: height}, .12, 7, ft8QSLShadow)
	rl.DrawRectangleRounded(box, .12, 7, withAlpha(colors.panel, 248))
	rl.DrawRectangleRoundedLinesEx(box, .12, 7, 2, ft8QSLGold)
	drawFT8MiniFlag(ft8CallRegion(station.call), rl.Rectangle{X: x + 10, Y: y + 10, Width: 22, Height: 14})
	simpleui.DrawTextStyled(station.call, x+40, y+8, 15, simpleui.FontSemiBold, ft8QSLCream)
	detail := fmt.Sprintf("%s  ·  SNR %+d dB", station.gridOrDash(), station.latest.SNR)
	simpleui.DrawTextStyled(detail, x+40, y+28, 10, simpleui.FontMono, map[bool]rl.Color{true: colors.orange, false: colors.cyan}[station.provisional])
}

func drawFT8MiniFlag(region string, bounds rl.Rectangle) {
	code := ft8RegionCountryCode(region)
	if code != countries.Unknown {
		loadFT8FlagAtlas()
		if index, ok := ft8FlagIndex[code.Alpha2()]; ok && ft8FlagAtlas.ID != 0 {
			source := rl.Rectangle{X: float32(index%16) * 24, Y: float32(index/16) * 18, Width: 24, Height: 18}
			rl.DrawTexturePro(ft8FlagAtlas, source, bounds, rl.Vector2{}, 0, rl.White)
			rl.DrawRectangleLinesEx(bounds, 1, withAlpha(ft8QSLCream, 150))
			return
		}
	}
	rl.DrawRectangleRec(bounds, colors.cyan)
	rl.DrawRectangleLinesEx(bounds, 1, ft8QSLCream)
}

func drawFT8QSLLegend() {
	bounds := rl.Rectangle{X: 44, Y: 154, Width: 540, Height: 32}
	rl.DrawRectangleRounded(bounds, .18, 6, withAlpha(colors.panel, 232))
	rl.DrawRectangleRoundedLinesEx(bounds, .18, 6, 1, withAlpha(ft8QSLGold, 205))
	simpleui.DrawTextStyled("QSL ATLAS", 55, 165, 9, simpleui.FontSemiBold, ft8QSLGold)
	demo := ft8MapStation{call: "EA1", grid: "IN80", latest: ft8.Decode{SNR: -8}}
	drawFT8QSLBadge(rl.Vector2{X: 130, Y: 170}, demo, false)
	simpleui.DrawText("ESTACIÓN", 145, 165, 9, colors.text)
	for index := 0; index < 2; index++ {
		card := rl.Rectangle{X: float32(203 + index*4), Y: float32(162 + index*3), Width: 20, Height: 14}
		rl.DrawRectangleRounded(card, .18, 4, ft8QSLGold)
		card.X, card.Y, card.Width, card.Height = card.X+2, card.Y+2, card.Width-4, card.Height-4
		rl.DrawRectangleRounded(card, .12, 3, colors.panel)
	}
	simpleui.DrawText("GRUPO", 232, 165, 9, colors.text)
	rl.DrawCircleV(rl.Vector2{X: 282, Y: 170}, 2, ft8QSLGold)
	simpleui.DrawText("FUERTE", 290, 165, 9, colors.text)
	simpleui.DrawTextStyled("»", 345, 162, 11, simpleui.FontSemiBold, colors.orange)
	simpleui.DrawText("CQ", 358, 165, 9, colors.text)
	simpleui.DrawTextStyled("✓", 385, 162, 11, simpleui.FontSemiBold, colors.green)
	simpleui.DrawText("QSO", 399, 165, 9, colors.text)
	simpleui.DrawTextStyled("?", 438, 163, 10, simpleui.FontSemiBold, colors.orange)
	simpleui.DrawText("PROVISIONAL", 449, 165, 9, colors.text)
}

func ft8MarkerColor(decode ft8.Decode) rl.Color {
	event, ok := classifyFT8Activity(decode)
	if ok {
		if event.kind == "complete" {
			return colors.green
		}
	}
	if decode.SNR <= -20 {
		return colors.orange
	}
	return colors.cyan
}

type ft8RadarContact struct {
	station  ft8MapStation
	position rl.Vector2
}

type ft8RadarCluster struct {
	contacts []ft8RadarContact
	center   rl.Vector2
}

func clusterFT8RadarContacts(contacts []ft8RadarContact, threshold float32) []ft8RadarCluster {
	clusters := make([]ft8RadarCluster, 0, len(contacts))
	for _, contact := range contacts {
		best, bestDistance := -1, threshold
		for index := range clusters {
			if distance := rl.Vector2Distance(contact.position, clusters[index].center); distance <= bestDistance {
				best, bestDistance = index, distance
			}
		}
		if best < 0 {
			clusters = append(clusters, ft8RadarCluster{contacts: []ft8RadarContact{contact}, center: contact.position})
			continue
		}
		cluster := &clusters[best]
		count := float32(len(cluster.contacts))
		cluster.center.X = (cluster.center.X*count + contact.position.X) / (count + 1)
		cluster.center.Y = (cluster.center.Y*count + contact.position.Y) / (count + 1)
		cluster.contacts = append(cluster.contacts, contact)
	}
	return clusters
}

func (p *FT8Panel) ft8RadarContacts(stations []ft8MapStation, includeSelected, provisionalOnly bool) []ft8RadarContact {
	contacts := make([]ft8RadarContact, 0, len(stations))
	for _, station := range stations {
		if !station.hasPosition || (!includeSelected && station.selected) {
			continue
		}
		if provisionalOnly != station.provisional {
			continue
		}
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, station.lat, station.lon, ft8MapBounds)
		if rl.CheckCollisionPointRec(position, ft8MapBounds) {
			contacts = append(contacts, ft8RadarContact{station: station, position: position})
		}
	}
	return contacts
}

func ft8RadarClusterRepresentative(cluster ft8RadarCluster) ft8MapStation {
	if len(cluster.contacts) == 0 {
		return ft8MapStation{}
	}
	representative := cluster.contacts[0].station
	for _, contact := range cluster.contacts[1:] {
		candidate := contact.station
		left, right := ft8DecodeTime(candidate.latest), ft8DecodeTime(representative.latest)
		if left.After(right) || (left.Equal(right) && candidate.latest.SNR > representative.latest.SNR) {
			representative = candidate
		}
	}
	return representative
}

func ft8RadarClusterLocation(cluster ft8RadarCluster) (float64, float64) {
	if len(cluster.contacts) == 0 {
		return 0, 0
	}
	lat := 0.0
	anchorLon := cluster.contacts[0].station.lon
	lonOffset := 0.0
	for _, contact := range cluster.contacts {
		lat += contact.station.lat
		lonOffset += wrapLongitude(contact.station.lon - anchorLon)
	}
	count := float64(len(cluster.contacts))
	return lat / count, wrapLongitude(anchorLon + lonOffset/count)
}

func ft8RadarStrength(snr int) float32 {
	return min(max(float32(snr+30)/30, 0), 1)
}

func ft8RadarAge(decode ft8.Decode, now time.Time) time.Duration {
	stamp := decode.ReceivedAt
	if stamp.IsZero() {
		stamp = decode.Slot
	}
	if stamp.IsZero() {
		// Unknown timestamps are historical imports, not live contacts. Treating
		// them as brand new would leave their pulse animation running forever.
		return time.Hour
	}
	if now.Before(stamp) {
		return 0
	}
	return now.Sub(stamp)
}

func ft8RadarDisplayColor(decode ft8.Decode, now time.Time, selected bool) rl.Color {
	color := ft8MarkerColor(decode)
	if selected {
		return color
	}
	// Contacts lose phosphor intensity gradually but remain identifiable for
	// the full selected time window.
	age := ft8RadarAge(decode, now)
	fade := min(float32(age)/float32(time.Hour), 1) * .58
	return mixColor(color, colors.border, fade)
}

func drawFT8StationMarker(position rl.Vector2, decode ft8.Decode, now time.Time, selected bool) {
	color := ft8RadarDisplayColor(decode, now, selected)
	strength := ft8RadarStrength(decode.SNR)
	coreRadius := float32(3.2) + strength*2.8
	haloRadius := float32(9) + strength*5
	if selected {
		haloRadius += 3
	}

	rl.DrawCircleV(position, haloRadius, withAlpha(color, 30))
	rl.DrawCircleV(position, coreRadius+2.4, withAlpha(colors.panel, 235))
	rl.DrawCircleV(position, coreRadius, color)
	rl.DrawCircleLines(int32(position.X), int32(position.Y), coreRadius+3, withAlpha(colors.text, 205))
	if ft8DecodeIsCQ(decode) {
		// CQ is a shape, not a color: weak CQ contacts can remain amber without
		// becoming indistinguishable from ordinary weak stations.
		rl.DrawRing(position, coreRadius+5, coreRadius+7, 205, 335, 12, withAlpha(colors.cyan, 230))
	}

	// A newly received contact emits one expanding, fading sweep. This gives
	// the map motion without a permanent neon pulse on every station.
	age := ft8RadarAge(decode, now)
	if age <= 12*time.Second {
		progress := float32(math.Mod(age.Seconds(), 3)) / 3
		pulseRadius := haloRadius + 3 + progress*16
		alpha := uint8(float32(150) * (1 - progress))
		rl.DrawCircleLines(int32(position.X), int32(position.Y), pulseRadius, withAlpha(color, alpha))
	}

	if selected {
		cross := withAlpha(color, 235)
		for _, line := range [][2]rl.Vector2{
			{{X: position.X - 23, Y: position.Y}, {X: position.X - 11, Y: position.Y}},
			{{X: position.X + 11, Y: position.Y}, {X: position.X + 23, Y: position.Y}},
			{{X: position.X, Y: position.Y - 23}, {X: position.X, Y: position.Y - 11}},
			{{X: position.X, Y: position.Y + 11}, {X: position.X, Y: position.Y + 23}},
		} {
			rl.DrawLineEx(line[0], line[1], 1.5, cross)
		}
		rl.DrawCircleLines(int32(position.X), int32(position.Y), 17, withAlpha(color, 180))
	}
}

func drawFT8ProvisionalMarker(position rl.Vector2, decode ft8.Decode) {
	color := withAlpha(colors.orange, 145)
	rl.DrawCircleV(position, 9, withAlpha(colors.panel, 75))
	for index := 0; index < 4; index++ {
		start := float32(index*90 + 10)
		rl.DrawRing(position, 6, 7.5, start, start+50, 4, color)
	}
	rl.DrawCircleV(position, 2, withAlpha(ft8MarkerColor(decode), 150))
}

func drawFT8RadarCluster(cluster ft8RadarCluster, now time.Time) {
	if len(cluster.contacts) == 0 {
		return
	}
	representative := ft8RadarClusterRepresentative(cluster)
	color := ft8RadarDisplayColor(representative.latest, now, false)
	center := cluster.center
	rl.DrawCircleV(center, 18, withAlpha(color, 28))
	rl.DrawCircleV(center, 12, withAlpha(colors.panel, 242))
	rl.DrawCircleLines(int32(center.X), int32(center.Y), 12, withAlpha(colors.text, 210))

	segments := min(len(cluster.contacts), 12)
	for index := 0; index < segments; index++ {
		start := float32(-90 + index*360/segments)
		end := start + float32(360/segments) - 5
		contact := cluster.contacts[index*len(cluster.contacts)/segments]
		segmentColor := ft8RadarDisplayColor(contact.station.latest, now, false)
		rl.DrawRing(center, 14, 16, start, end, 3, withAlpha(segmentColor, 225))
	}
	if age := ft8RadarAge(representative.latest, now); age <= 12*time.Second {
		progress := float32(math.Mod(age.Seconds(), 3)) / 3
		alpha := uint8(float32(125) * (1 - progress))
		rl.DrawCircleLines(int32(center.X), int32(center.Y), 19+progress*14, withAlpha(color, alpha))
	}
	label := fmt.Sprintf("%d", len(cluster.contacts))
	size := simpleui.MeasureTextStyled(label, 11, simpleui.FontSemiBold)
	simpleui.DrawTextStyled(label, center.X-size.X/2, center.Y-size.Y/2-1, 11, simpleui.FontSemiBold, colors.text)
}

func ft8DecodeIsCQ(decode ft8.Decode) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(decode.Message)), "CQ ")
}

func drawFT8RadarLegend() {
	bounds := rl.Rectangle{X: 44, Y: 154, Width: 540, Height: 32}
	rl.DrawRectangleRounded(bounds, .18, 6, withAlpha(colors.panel, 224))
	rl.DrawRectangleRoundedLinesEx(bounds, .18, 6, 1, withAlpha(colors.border, 220))
	simpleui.DrawTextStyled("RADAR", 55, 165, 9, simpleui.FontSemiBold, colors.muted)

	drawDot := func(x float32, color rl.Color) {
		rl.DrawCircleV(rl.Vector2{X: x, Y: 170}, 5, withAlpha(colors.panelAlt, 240))
		rl.DrawCircleV(rl.Vector2{X: x, Y: 170}, 3, color)
	}
	drawDot(105, colors.cyan)
	simpleui.DrawText("SEÑAL", 114, 165, 9, colors.text)
	drawDot(166, colors.orange)
	simpleui.DrawText("DÉBIL", 175, 165, 9, colors.text)

	cq := rl.Vector2{X: 231, Y: 170}
	rl.DrawCircleV(cq, 3, colors.cyan)
	rl.DrawRing(cq, 6, 7.5, 205, 335, 8, colors.cyan)
	simpleui.DrawText("CQ", 241, 165, 9, colors.text)

	drawDot(279, colors.green)
	simpleui.DrawText("QSO", 288, 165, 9, colors.text)

	provisional := rl.Vector2{X: 339, Y: 170}
	for index := 0; index < 4; index++ {
		start := float32(index*90 + 10)
		rl.DrawRing(provisional, 5.5, 7, start, start+50, 4, withAlpha(colors.orange, 165))
	}
	simpleui.DrawText("PROVISIONAL", 349, 165, 9, colors.text)
	simpleui.DrawText("PULSO = RECIENTE", 438, 165, 8, colors.muted)
}

func (p *FT8Panel) drawFT8LiveEvents() {
	locations := ft8StationLocations(p.mapDecodes)
	decodes := p.filterFT8DecodesByRegion(p.mapDecodes)
	now := time.Now()
	drawn := 0
	for _, decode := range decodes {
		event, ok := classifyFT8Activity(decode)
		if !ok || decode.ReceivedAt.IsZero() {
			continue
		}
		age := now.Sub(decode.ReceivedAt)
		if age < 0 || age > 5*time.Second {
			continue
		}
		progress := float32(age) / float32(5*time.Second)
		alpha := uint8(220 * (1 - progress))
		from, fromOK := locations[event.from]
		if event.kind == "cq" && fromOK {
			position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, from.lat, from.lon, ft8MapBounds)
			if rl.CheckCollisionPointRec(position, ft8MapBounds) {
				radius := float32(10) + progress*32
				rl.DrawCircleLines(int32(position.X), int32(position.Y), radius, rl.Color{R: colors.orange.R, G: colors.orange.G, B: colors.orange.B, A: alpha})
				rl.DrawCircleV(position, 5, colors.orange)
			}
			drawn++
			continue
		}
		to, toOK := locations[event.to]
		if !fromOK || !toOK {
			continue
		}
		color := ft8EventColor(event.kind)
		color.A = alpha
		drawFT8Route(from.lat, from.lon, to.lat, to.lon, p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds, color)
		pulse := ft8RoutePosition(from.lat, from.lon, to.lat, to.lon, float64(math.Mod(float64(progress)*1.7, 1)))
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, pulse.lat, pulse.lon, ft8MapBounds)
		if rl.CheckCollisionPointRec(position, ft8MapBounds) {
			rl.DrawCircleV(position, 7, color)
			rl.DrawCircleLines(int32(position.X), int32(position.Y), 12, rl.Color{R: color.R, G: color.G, B: color.B, A: alpha / 2})
		}
		drawn++
		if drawn >= 12 {
			break
		}
	}
}

type ft8Location struct{ lat, lon float64 }

func ft8StationLocations(decodes []ft8.Decode) map[string]ft8Location {
	result := make(map[string]ft8Location)
	for index := len(decodes) - 1; index >= 0; index-- {
		call, grid := ft8MessageStation(decodes[index].Message)
		if lat, lon, ok := maidenheadCenter(grid); ok && call != "" {
			result[call] = ft8Location{lat: lat, lon: lon}
		}
	}
	return result
}

func ft8RoutePosition(lat1, lon1, lat2, lon2, progress float64) ft8Location {
	return ft8Location{lat: lat1 + (lat2-lat1)*progress, lon: wrapLongitude(lon1 + wrapLongitude(lon2-lon1)*progress)}
}

func ft8EventColor(kind string) rl.Color {
	switch kind {
	case ft8QSOReport, ft8QSORogerReport:
		return colors.orange
	case ft8QSOConfirmed, ft8QSOComplete:
		return colors.green
	default:
		return colors.cyan
	}
}

func classifyFT8Activity(decode ft8.Decode) (ft8ActivityEvent, bool) {
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(decode.Message)))
	if len(fields) < 2 {
		return ft8ActivityEvent{}, false
	}
	if fields[0] == "CQ" {
		from := ft8MessageCall(decode.Message)
		return ft8ActivityEvent{decode: decode, from: from, kind: "cq"}, from != ""
	}
	from, to := strings.Trim(fields[1], "<>"), strings.Trim(fields[0], "<>")
	if !ft8PlausibleCall(from) || !ft8PlausibleCall(to) {
		return ft8ActivityEvent{}, false
	}
	phase := ft8QSOReply
	for _, token := range fields[2:] {
		token = strings.TrimSpace(strings.ToUpper(token))
		switch {
		case token == "RR73" || token == "73":
			phase = ft8QSOComplete
		case token == "RRR" && ft8QSOPhaseRank(phase) < ft8QSOPhaseRank(ft8QSOConfirmed):
			phase = ft8QSOConfirmed
		case (strings.HasPrefix(token, "R+") || strings.HasPrefix(token, "R-")) && ft8QSOPhaseRank(phase) < ft8QSOPhaseRank(ft8QSORogerReport):
			phase = ft8QSORogerReport
		case (strings.HasPrefix(token, "+") || strings.HasPrefix(token, "-")) && ft8QSOPhaseRank(phase) < ft8QSOPhaseRank(ft8QSOReport):
			phase = ft8QSOReport
		}
	}
	return ft8ActivityEvent{decode: decode, from: from, to: to, kind: phase, phase: phase}, true
}

func ft8QSOPhaseRank(phase string) int {
	switch phase {
	case ft8QSOReport:
		return 2
	case ft8QSORogerReport:
		return 3
	case ft8QSOConfirmed:
		return 4
	case ft8QSOComplete:
		return 5
	default:
		return 1
	}
}

type ft8RegionActivity struct {
	name               string
	stations, messages int
	bestSNR            int
	maxDistance        float64
}

func (p *FT8Panel) activityDecodes() []ft8.Decode {
	cutoff := time.Time{}
	if p.mapWindow > 0 {
		cutoff = time.Now().UTC().Add(-p.mapWindow)
	}
	result := make([]ft8.Decode, 0, len(p.mapDecodes))
	for _, decode := range p.mapDecodes {
		if cutoff.IsZero() || !decode.Slot.Before(cutoff) {
			upper := strings.ToUpper(strings.TrimSpace(decode.Message))
			if p.mapFilter == "cq" && !strings.HasPrefix(upper, "CQ ") {
				continue
			}
			if p.mapFilter == "calls" && strings.HasPrefix(upper, "CQ ") {
				continue
			}
			result = append(result, decode)
		}
	}
	return result
}

func (p *FT8Panel) drawFT8Dashboard() {
	decodes := p.activityDecodes()
	allStations := p.ft8MapStations()
	stations := p.filterFT8StationsByRegion(allStations)
	visibleDecodes := p.filterFT8DecodesByRegion(decodes)
	positioned, verified, provisional, maxDistance := 0, 0, 0, 0.0
	homeLat, homeLon := p.ft8HomePosition()
	for _, station := range stations {
		if station.provisional {
			provisional++
		} else {
			verified++
		}
		if station.hasPosition {
			positioned++
			if !station.provisional {
				maxDistance = math.Max(maxDistance, ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))
			}
		}
	}
	regions := p.stableFT8RegionActivity(allStations, time.Now())
	p.mapRegionOffset = min(p.mapRegionOffset, max(0, len(regions)-8))
	qso := calculateFT8QSOStats(visibleDecodes, time.Now())

	sidebar := rl.Rectangle{X: 1128, Y: 142, Width: 440, Height: 718}
	rl.DrawRectangleRounded(sidebar, .018, 8, colors.background)
	rl.DrawRectangleRoundedLinesEx(sidebar, .018, 8, 1, colors.border)
	metrics := []struct {
		label, value string
		color        rl.Color
	}{
		{"ESTACIONES", fmt.Sprintf("%d", verified), colors.green},
		{"MENSAJES", fmt.Sprintf("%d", len(visibleDecodes)), colors.cyan},
		{"QSO ACTIVOS", fmt.Sprintf("%d / %d", qso.active, qso.total), colors.orange},
		{"DX MÁXIMO", fmt.Sprintf("%.0f km", maxDistance), colors.blue},
	}
	for index, metric := range metrics {
		x := float32(1140 + (index%2)*208)
		y := float32(154 + (index/2)*70)
		box := rl.Rectangle{X: x, Y: y, Width: 196, Height: 60}
		rl.DrawRectangleRounded(box, .06, 6, mixColor(colors.panelAlt, metric.color, .07))
		rl.DrawRectangleRoundedLinesEx(box, .06, 6, 1, mixColor(colors.border, metric.color, .45))
		simpleui.DrawText(metric.label, x+12, y+9, 10, colors.muted)
		simpleui.DrawTextStyled(metric.value, x+12, y+27, 20, simpleui.FontSemiBold, metric.color)
		if index == 0 && provisional > 0 {
			simpleui.DrawText(fmt.Sprintf("+%d provisionales", provisional), x+65, y+35, 9, colors.orange)
		}
	}
	heading := "PAÍSES Y REGIONES"
	if p.mapRegionFilter != "" {
		heading = "REGIÓN: " + strings.ToUpper(clipFT8(p.mapRegionFilter, 18))
		drawModalAction(ft8RegionClear, "VER TODAS", colors.cyan, false)
	} else {
		simpleui.DrawText(fmt.Sprintf("%d regiones · %d QSO completados", len(regions), qso.completed), 1343, 307, 10, colors.muted)
	}
	simpleui.DrawTextStyled(heading, 1140, 304, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("REGIÓN", 1140, 334, 11, simpleui.FontSemiBold, colors.muted)
	drawFT8RightAligned("EST.", 1374, 334, 11, simpleui.FontSemiBold, colors.muted)
	drawFT8RightAligned("MSG", 1420, 334, 11, simpleui.FontSemiBold, colors.muted)
	drawFT8RightAligned("SNR", 1470, 334, 11, simpleui.FontSemiBold, colors.muted)
	drawFT8RightAligned("DIST.", 1544, 334, 11, simpleui.FontSemiBold, colors.muted)
	rl.DrawLine(1140, 354, 1556, 354, colors.border)
	for row, region := range regions[p.mapRegionOffset:] {
		if row >= 8 {
			break
		}
		y := float32(362 + row*25)
		selectedRegion := region.name == p.mapRegionFilter
		if selectedRegion {
			rl.DrawRectangle(1140, int32(y-3), 416, 24, mixColor(colors.panelAlt, colors.cyan, .18))
			rl.DrawRectangle(1140, int32(y-3), 3, 24, colors.cyan)
		} else if row%2 == 0 {
			rl.DrawRectangle(1140, int32(y-3), 416, 24, mixColor(colors.panelAlt, colors.blue, .04))
		}
		drawFT8RegionFlag(region.name, 1144, y+2)
		simpleui.DrawText(clipFT8(region.name, 16), 1167, y+2, 12, colors.text)
		drawFT8RightAligned(fmt.Sprintf("%d", region.stations), 1374, y+2, 12, simpleui.FontMono, colors.text)
		drawFT8RightAligned(fmt.Sprintf("%d", region.messages), 1420, y+2, 12, simpleui.FontMono, colors.text)
		drawFT8RightAligned(fmt.Sprintf("%+d", region.bestSNR), 1470, y+2, 12, simpleui.FontMono, ft8SNRColor(region.bestSNR))
		drawFT8RightAligned(fmt.Sprintf("%.0f km", region.maxDistance), 1544, y+2, 12, simpleui.FontMono, colors.muted)
	}
	if len(regions) > 8 {
		track := rl.Rectangle{X: 1552, Y: 360, Width: 4, Height: 198}
		rl.DrawRectangleRounded(track, 1, 4, colors.panelAlt)
		thumb := track
		thumb.Height = max(float32(24), track.Height*8/float32(len(regions)))
		thumb.Y += (track.Height - thumb.Height) * float32(p.mapRegionOffset) / float32(len(regions)-8)
		rl.DrawRectangleRounded(thumb, 1, 4, colors.cyan)
	}
	if len(regions) == 0 {
		simpleui.DrawText("Esperando estaciones identificables…", 1140, 370, uiBodyFontSize, colors.muted)
	}
	p.drawFT8SelectedCard(positioned)
	p.drawFT8Timeline(visibleDecodes)
}

func drawFT8RightAligned(text string, right, y float32, size int32, font simpleui.FontStyle, color rl.Color) {
	width := simpleui.MeasureTextStyled(text, size, font).X
	simpleui.DrawTextStyled(text, right-width, y, size, font, color)
}

func calculateFT8QSOStats(decodes []ft8.Decode, now time.Time) ft8QSOStats {
	conversations := reconstructFT8QSOs(decodes)
	result := ft8QSOStats{total: len(conversations)}
	for _, item := range conversations {
		if ft8QSOPhaseRank(item.phase) >= ft8QSOPhaseRank(ft8QSOConfirmed) {
			result.completed++
		}
		if ft8QSOPhaseRank(item.phase) < ft8QSOPhaseRank(ft8QSOConfirmed) && now.Sub(item.last) <= 2*time.Minute {
			result.active++
		}
	}
	return result
}

func reconstructFT8QSOs(decodes []ft8.Decode) map[string]ft8QSOConversation {
	ordered := append([]ft8.Decode(nil), decodes...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ft8DecodeTime(ordered[i]).Before(ft8DecodeTime(ordered[j]))
	})
	conversations := make(map[string]ft8QSOConversation)
	for _, decode := range ordered {
		event, ok := classifyFT8Activity(decode)
		if !ok || event.to == "" {
			continue
		}
		a, b := event.from, event.to
		if a > b {
			a, b = b, a
		}
		key := a + "|" + b
		item := conversations[key]
		when := ft8DecodeTime(decode)
		if item.messages == 0 {
			item.first, item.callA, item.callB = when, a, b
		}
		if when.After(item.last) {
			item.last = when
		}
		item.messages++
		if ft8QSOPhaseRank(event.phase) > ft8QSOPhaseRank(item.phase) {
			item.phase = event.phase
		}
		conversations[key] = item
	}
	return conversations
}

func ft8DecodeTime(decode ft8.Decode) time.Time {
	if !decode.ReceivedAt.IsZero() {
		return decode.ReceivedAt
	}
	return decode.Slot
}

func (p *FT8Panel) drawFT8SelectedCard(positioned int) {
	station := p.selectedFT8MapStation()
	panel := rl.Rectangle{X: 1140, Y: 578, Width: 416, Height: 130}
	rl.DrawRectangleRounded(panel, .035, 7, mixColor(colors.panelAlt, colors.cyan, .035))
	rl.DrawRectangleRoundedLinesEx(panel, .035, 7, 1, colors.border)
	cardTitle := "ESTACIÓN SELECCIONADA"
	cardTitleColor := colors.cyan
	if station.call == "" && p.mapRegionFilter != "" {
		cardTitle = "RESUMEN DE LA REGIÓN"
	} else if station.provisional {
		cardTitle = "ESTACIÓN PROVISIONAL · PENDIENTE DE CONFIRMACIÓN"
		cardTitleColor = colors.orange
	}
	simpleui.DrawTextStyled(cardTitle, 1152, 589, uiCaptionFontSize, simpleui.FontSemiBold, cardTitleColor)
	if station.call == "" {
		if p.mapRegionFilter != "" {
			stations := p.visibleFT8MapStations()
			messages, bestSNR := 0, -99
			for _, item := range stations {
				messages += item.messageCount
				bestSNR = max(bestSNR, item.latest.SNR)
			}
			drawFT8RegionFlag(p.mapRegionFilter, 1152, 619)
			simpleui.DrawTextStyled(clipFT8(p.mapRegionFilter, 27), 1177, 615, 18, simpleui.FontSemiBold, colors.cyan)
			simpleui.DrawText(fmt.Sprintf("%d estaciones · %d mensajes · mejor SNR %+d dB", len(stations), messages, bestSNR), 1152, 646, uiBodyFontSize, colors.text)
			simpleui.DrawText("Haz clic en un marcador para ver la estación.", 1152, 674, uiCaptionFontSize, colors.muted)
			return
		}
		simpleui.DrawTextStyled("NINGUNA", 1152, 614, uiSectionFontSize, simpleui.FontSemiBold, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d estaciones con posición", positioned), 1152, 641, uiCaptionFontSize, colors.muted)
		simpleui.DrawText("Haz clic en un marcador para ver sus datos.", 1152, 666, uiCaptionFontSize, colors.muted)
		return
	}
	backColor := colors.cyan
	rl.DrawLineEx(rl.Vector2{X: 1396, Y: 597}, rl.Vector2{X: 1401, Y: 592}, 1.5, backColor)
	rl.DrawLineEx(rl.Vector2{X: 1396, Y: 597}, rl.Vector2{X: 1401, Y: 602}, 1.5, backColor)
	simpleui.DrawText("VOLVER AL RANKING", 1407, 591, 10, backColor)
	region := ft8CallRegion(station.call)
	drawFT8RegionFlag(region, 1152, 619)
	simpleui.DrawTextStyled(region, 1177, 619, 11, simpleui.FontSemiBold, colors.text)
	callX := float32(1177) + simpleui.MeasureTextStyled(region, 11, simpleui.FontSemiBold).X + 12
	simpleui.DrawTextStyled(station.call+"  ·  "+station.gridOrDash(), callX, 615, 18, simpleui.FontSemiBold, colors.cyan)
	location := "Sin posición disponible"
	if station.hasPosition {
		homeLat, homeLon := p.ft8HomePosition()
		location = fmt.Sprintf("%.0f km  ·  %.0f°  ·  SNR %+d dB", ft8DistanceKM(homeLat, homeLon, station.lat, station.lon), ft8Bearing(homeLat, homeLon, station.lat, station.lon), station.latest.SNR)
	}
	if zone, ok := ft8CallCQZone(station.call); ok {
		location = fmt.Sprintf("CQ %d  ·  %s", zone, location)
	}
	simpleui.DrawText(location, 1152, 641, uiBodyFontSize, colors.text)
	localClock := "HORA LOCAL  NO DISPONIBLE"
	if station.hasPosition {
		if label, ok := localtime.Label(time.Now().UTC(), station.lat, station.lon); ok {
			localClock = "HORA LOCAL  " + label
		}
	}
	simpleui.DrawText(fmt.Sprintf("%d mensajes · %s", station.messageCount, station.latest.Slot.UTC().Format("15:04:05 UTC")), 1152, 665, 10, colors.muted)
	clockWidth := simpleui.MeasureTextStyled(localClock, 10, simpleui.FontRegular).X
	simpleui.DrawTextStyled(localClock, 1544-clockWidth, 665, 10, simpleui.FontRegular, colors.orange)
	messageX := float32(1152)
	if qso, ok := latestFT8QSOForCall(p.activityDecodes(), station.call); ok {
		label, color := ft8QSOPhasePresentation(qso.phase)
		label += "  ·  "
		simpleui.DrawTextStyled(label, messageX, 685, uiCaptionFontSize, simpleui.FontSemiBold, color)
		messageX += simpleui.MeasureTextStyled(label, uiCaptionFontSize, simpleui.FontSemiBold).X
	}
	simpleui.DrawText(clipFT8(station.latest.Message, 34), messageX, 685, uiBodyFontSize, colors.text)
}

func latestFT8QSOForCall(decodes []ft8.Decode, call string) (ft8QSOConversation, bool) {
	call = strings.ToUpper(strings.TrimSpace(call))
	var latest ft8QSOConversation
	found := false
	for _, qso := range reconstructFT8QSOs(decodes) {
		if qso.callA != call && qso.callB != call {
			continue
		}
		if !found || qso.last.After(latest.last) {
			latest, found = qso, true
		}
	}
	return latest, found
}

func ft8QSOPhasePresentation(phase string) (string, rl.Color) {
	switch phase {
	case ft8QSOReport:
		return "QSO · REPORTE", colors.orange
	case ft8QSORogerReport:
		return "QSO · REPORTE R", colors.orange
	case ft8QSOConfirmed:
		return "QSO · CONFIRMADO", colors.green
	case ft8QSOComplete:
		return "QSO · CERRADO", colors.green
	default:
		return "QSO · RESPUESTA", colors.cyan
	}
}

func (p *FT8Panel) drawFT8Timeline(decodes []ft8.Decode) {
	bounds := rl.Rectangle{X: 1140, Y: 718, Width: 416, Height: 130}
	rl.DrawRectangleRounded(bounds, .018, 7, colors.background)
	rl.DrawRectangleRoundedLinesEx(bounds, .018, 7, 1, colors.border)
	simpleui.DrawTextStyled("ÚLTIMOS 60 MIN", 1152, 730, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("Actividad por minuto", 1430, 733, 9, colors.muted)
	counts := make([]int, 60)
	now := time.Now().UTC().Truncate(time.Minute)
	maximum := 1
	for _, decode := range decodes {
		age := int(now.Sub(decode.Slot.UTC().Truncate(time.Minute)) / time.Minute)
		if age >= 0 && age < 60 {
			counts[59-age]++
			maximum = max(maximum, counts[59-age])
		}
	}
	chart := rl.Rectangle{X: 1152, Y: 765, Width: 392, Height: 58}
	rl.DrawLine(int32(chart.X), int32(chart.Y+chart.Height), int32(chart.X+chart.Width), int32(chart.Y+chart.Height), colors.border)
	barWidth := chart.Width / 60
	for index, count := range counts {
		height := chart.Height * float32(count) / float32(maximum)
		color := colors.cyan
		if index == 59 {
			color = colors.green
		}
		rl.DrawRectangle(int32(chart.X+float32(index)*barWidth+1), int32(chart.Y+chart.Height-height), int32(max(2, barWidth-2)), int32(height), color)
	}
	simpleui.DrawText("−60 min", 1152, 828, 9, colors.muted)
	simpleui.DrawText("AHORA", 1508, 828, 9, colors.green)
}

func (p *FT8Panel) ft8RegionActivity(stations []ft8MapStation) []ft8RegionActivity {
	byRegion := make(map[string]*ft8RegionActivity)
	homeLat, homeLon := p.ft8HomePosition()
	for _, station := range stations {
		if station.provisional {
			continue
		}
		region := ft8CallRegion(station.call)
		item := byRegion[region]
		if item == nil {
			item = &ft8RegionActivity{name: region, bestSNR: -99}
			byRegion[region] = item
		}
		item.stations++
		item.messages += station.messageCount
		item.bestSNR = max(item.bestSNR, station.latest.SNR)
		if station.hasPosition {
			item.maxDistance = math.Max(item.maxDistance, ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))
		}
	}
	result := make([]ft8RegionActivity, 0, len(byRegion))
	for _, item := range byRegion {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].messages != result[j].messages {
			return result[i].messages > result[j].messages
		}
		return result[i].name < result[j].name
	})
	return result
}

func (p *FT8Panel) stableFT8RegionActivity(stations []ft8MapStation, now time.Time) []ft8RegionActivity {
	regions := p.ft8RegionActivity(stations)
	if len(p.mapRegionOrder) == 0 || !now.Before(p.mapNextRegionRank) {
		p.mapRegionOrder = p.mapRegionOrder[:0]
		for _, region := range regions {
			p.mapRegionOrder = append(p.mapRegionOrder, region.name)
		}
		// Runtime emits the messages in one slot individually. Holding the row
		// order briefly prevents the ranking from jumping for every decode.
		p.mapNextRegionRank = now.Add(3 * time.Second)
		return regions
	}
	order := make(map[string]int, len(p.mapRegionOrder))
	for index, name := range p.mapRegionOrder {
		order[name] = index
	}
	sort.SliceStable(regions, func(i, j int) bool {
		left, leftKnown := order[regions[i].name]
		right, rightKnown := order[regions[j].name]
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown {
			return left < right
		}
		return regions[i].name < regions[j].name
	})
	return regions
}

func drawFT8RegionFlag(name string, x, y float32) {
	b := rl.Rectangle{X: x, Y: y, Width: 17, Height: 12}
	if code := ft8RegionCountryCode(name); code != countries.Unknown {
		loadFT8FlagAtlas()
		if index, ok := ft8FlagIndex[code.Alpha2()]; ok && ft8FlagAtlas.ID != 0 {
			source := rl.Rectangle{X: float32(index%16) * 24, Y: float32(index/16) * 18, Width: 24, Height: 18}
			rl.DrawTexturePro(ft8FlagAtlas, source, b, rl.Vector2{}, 0, rl.White)
			rl.DrawRectangleLinesEx(b, 1, withAlpha(colors.text, 90))
			return
		}
	}
	white := rl.Color{R: 245, G: 248, B: 250, A: 255}
	red := rl.Color{R: 215, G: 40, B: 52, A: 255}
	blue := rl.Color{R: 30, G: 77, B: 150, A: 255}
	yellow := rl.Color{R: 247, G: 196, B: 35, A: 255}
	green := rl.Color{R: 28, G: 150, B: 78, A: 255}
	stripeH := func(a, middle, c rl.Color) {
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: 17, Height: 4}, a)
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y + 4, Width: 17, Height: 4}, middle)
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y + 8, Width: 17, Height: 4}, c)
	}
	stripeV := func(a, middle, c rl.Color) {
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: 6, Height: 12}, a)
		rl.DrawRectangleRec(rl.Rectangle{X: x + 6, Y: y, Width: 5, Height: 12}, middle)
		rl.DrawRectangleRec(rl.Rectangle{X: x + 11, Y: y, Width: 6, Height: 12}, c)
	}
	switch name {
	case "España", "Spain", "Balearic Islands", "Canary Islands", "Ceuta & Melilla":
		stripeH(red, yellow, red)
	case "Alemania", "Fed. Rep. of Germany":
		stripeH(rl.Black, red, yellow)
	case "Francia", "France", "Corsica":
		stripeV(blue, white, red)
	case "Italia", "Italy", "Sardinia":
		stripeV(green, white, red)
	case "Países Bajos", "Netherlands":
		stripeH(red, white, blue)
	case "Bélgica", "Belgium":
		stripeV(rl.Black, yellow, red)
	case "Austria":
		stripeH(red, white, red)
	case "Polonia", "Poland":
		stripeH(white, white, red)
	case "Finlandia", "Finland":
		rl.DrawRectangleRec(b, white)
		rl.DrawRectangle(int32(x+5), int32(y), 3, 12, blue)
		rl.DrawRectangle(int32(x), int32(y+5), 17, 3, blue)
	case "Suecia", "Sweden":
		rl.DrawRectangleRec(b, blue)
		rl.DrawRectangle(int32(x+5), int32(y), 3, 12, yellow)
		rl.DrawRectangle(int32(x), int32(y+5), 17, 3, yellow)
	case "Suiza", "Switzerland":
		rl.DrawRectangleRec(b, red)
		rl.DrawRectangle(int32(x+7), int32(y+2), 3, 8, white)
		rl.DrawRectangle(int32(x+4), int32(y+5), 9, 3, white)
	case "Japón", "Japan":
		rl.DrawRectangleRec(b, white)
		rl.DrawCircle(int32(x+8), int32(y+6), 3, red)
	case "Portugal", "Azores", "Madeira Islands":
		stripeV(green, green, red)
	case "Canadá", "Canada":
		stripeV(red, white, red)
	case "Estados Unidos", "United States":
		for row := 0; row < 6; row++ {
			color := white
			if row%2 == 0 {
				color = red
			}
			rl.DrawRectangle(int32(x), int32(y)+int32(row*2), 17, 2, color)
		}
		rl.DrawRectangle(int32(x), int32(y), 7, 7, blue)
	case "Reino Unido", "England", "Scotland", "Wales", "Northern Ireland", "Isle of Man":
		rl.DrawRectangleRec(b, blue)
		rl.DrawRectangle(int32(x+7), int32(y), 3, 12, white)
		rl.DrawRectangle(int32(x), int32(y+4), 17, 4, white)
		rl.DrawRectangle(int32(x+8), int32(y), 1, 12, red)
		rl.DrawRectangle(int32(x), int32(y+5), 17, 2, red)
	case "Australia", "Nueva Zelanda", "New Zealand":
		rl.DrawRectangleRec(b, blue)
		rl.DrawCircle(int32(x+12), int32(y+4), 2, white)
	case "Brasil", "Brazil":
		rl.DrawRectangleRec(b, green)
		rl.DrawTriangle(rl.Vector2{X: x + 8.5, Y: y + 2}, rl.Vector2{X: x + 14, Y: y + 6}, rl.Vector2{X: x + 8.5, Y: y + 10}, yellow)
		rl.DrawTriangle(rl.Vector2{X: x + 8.5, Y: y + 2}, rl.Vector2{X: x + 3, Y: y + 6}, rl.Vector2{X: x + 8.5, Y: y + 10}, yellow)
		rl.DrawCircle(int32(x+8), int32(y+6), 2, blue)
	case "Argentina":
		stripeH(rl.SkyBlue, white, rl.SkyBlue)
		rl.DrawCircle(int32(x+8), int32(y+6), 1, yellow)
	case "Chile":
		stripeH(white, white, red)
		rl.DrawRectangle(int32(x), int32(y), 6, 6, blue)
	case "Uruguay":
		stripeH(white, rl.SkyBlue, white)
		rl.DrawCircle(int32(x+3), int32(y+3), 1, yellow)
	case "México", "Mexico":
		stripeV(green, white, red)
	case "Irlanda", "Ireland":
		stripeV(green, white, rl.Orange)
	case "Rumanía", "Romania":
		stripeV(blue, yellow, red)
	case "Ucrania", "Ukraine":
		rl.DrawRectangle(int32(x), int32(y), 17, 6, blue)
		rl.DrawRectangle(int32(x), int32(y+6), 17, 6, yellow)
	case "China":
		rl.DrawRectangleRec(b, red)
		rl.DrawCircle(int32(x+4), int32(y+4), 2, yellow)
	default:
		rl.DrawRectangleRec(b, colors.panelAlt)
		rl.DrawCircleLines(int32(x+8), int32(y+6), 5, colors.muted)
		rl.DrawLine(int32(x+3), int32(y+6), int32(x+13), int32(y+6), colors.muted)
	}
	rl.DrawRectangleLinesEx(b, 1, withAlpha(colors.text, 90))
}

func loadFT8FlagAtlas() {
	ft8FlagOnce.Do(func() {
		codes := countries.All()
		sort.Slice(codes, func(i, j int) bool { return codes[i].Alpha2() < codes[j].Alpha2() })
		ft8FlagIndex = make(map[string]int, len(codes))
		for index, code := range codes {
			ft8FlagIndex[code.Alpha2()] = index
		}
		image := rl.LoadImageFromMemory(".png", ft8FlagAtlasPNG, int32(len(ft8FlagAtlasPNG)))
		if image == nil {
			return
		}
		ft8FlagAtlas = rl.LoadTextureFromImage(image)
		rl.UnloadImage(image)
		rl.SetTextureFilter(ft8FlagAtlas, rl.FilterPoint)
	})
}

func ft8RegionCountryCode(name string) countries.CountryCode {
	ft8DXCCFlagOnce.Do(func() {
		ft8DXCCFlags = make(map[string]string)
		_ = json.Unmarshal(ft8DXCCFlagsJSON, &ft8DXCCFlags)
	})
	if code := ft8DXCCFlags[name]; code != "" {
		if country := countries.ByName(code); country != countries.Unknown {
			return country
		}
	}
	aliases := map[string]string{
		"España": "Spain", "Canary Islands": "Spain", "Balearic Islands": "Spain", "Ceuta & Melilla": "Spain",
		"Alemania": "Germany", "Fed. Rep. of Germany": "Germany",
		"Francia": "France", "Corsica": "France",
		"Italia": "Italy", "Sardinia": "Italy",
		"Países Bajos": "Netherlands", "Bélgica": "Belgium", "Polonia": "Poland",
		"Suecia": "Sweden", "Finlandia": "Finland", "Suiza": "Switzerland",
		"Japón": "Japan", "Canadá": "Canada", "Estados Unidos": "United States",
		"Reino Unido": "United Kingdom", "England": "United Kingdom", "Scotland": "United Kingdom",
		"Wales": "United Kingdom", "Northern Ireland": "United Kingdom", "Isle of Man": "Isle of Man",
		"Nueva Zelanda": "New Zealand", "Brasil": "Brazil", "México": "Mexico",
		"Irlanda": "Ireland", "Rumanía": "Romania", "Ucrania": "Ukraine",
		"Sudáfrica": "South Africa", "Rusia europea": "Russia", "Rusia asiática": "Russia",
		"Azores": "Portugal", "Madeira Islands": "Portugal",
		"Agalega & St. Brandon": "Mauritius", "Bouvet": "Bouvet Island", "Peter 1 Island": "Norway",
		"Dem. Rep. of the Congo": "CD", "Trinidad & Tobago": "Trinidad and Tobago",
		"San Felix & San Ambrosio": "Chile", "Juan Fernandez Islands": "Chile",
		"St. Barthelemy": "Saint Barthelemy", "Reunion Island": "Reunion", "St. Martin": "Saint Martin",
		"Juan de Nova, Europa": "France", "Galapagos Islands": "Ecuador",
		"San Andres & Providencia": "Colombia", "Republic of Korea": "South Korea",
		"African Italy": "Italy", "Sicily": "Italy", "Minami Torishima": "Japan", "Ogasawara": "Japan",
		"Bear Island": "Norway", "Saba & St. Eustatius": "Bonaire, Sint Eustatius and Saba",
		"St. Peter & St. Paul": "Brazil", "Trindade & Martim Vaz": "Brazil",
		"Asiatic Turkey": "Turkey", "European Turkey": "Turkey", "Heard Island": "Heard Island and McDonald Islands",
		"Revillagigedo": "Mexico",
	}
	if alias, ok := aliases[name]; ok {
		name = alias
	}
	return countries.ByName(name)
}

func ft8CallRegion(call string) string {
	initializeFT8DXCC()
	if ft8DXCCData == nil {
		return ft8FallbackCallRegion(call)
	}
	if entity, ok := ft8DXCCData.Lookup(call); ok {
		return ft8EntityDisplayName(entity.Name)
	}
	return ft8FallbackCallRegion(call)
}

func initializeFT8DXCC() {
	ft8DXCCOnce.Do(func() {
		ft8DXCCData, _ = dxcc.Load(resources.Path("tools", "ft8", "cty.dat"))
		if ft8DXCCData == nil {
			ft8DXCCData, _ = dxcc.LoadBundled()
		}
	})
}

func ft8CallCQZone(call string) (int, bool) {
	initializeFT8DXCC()
	if ft8DXCCData == nil {
		return 0, false
	}
	if entity, ok := ft8DXCCData.Lookup(call); ok {
		return entity.CQZone, entity.CQZone >= 1 && entity.CQZone <= 40
	}
	return 0, false
}

func ft8EntityDisplayName(name string) string {
	translations := map[string]string{
		"Spain": "España", "Fed. Rep. of Germany": "Alemania", "France": "Francia",
		"Italy": "Italia", "Netherlands": "Países Bajos", "Belgium": "Bélgica",
		"Poland": "Polonia", "Sweden": "Suecia", "Finland": "Finlandia",
		"Switzerland": "Suiza", "Japan": "Japón", "Canada": "Canadá",
		"United States": "Estados Unidos", "England": "Reino Unido",
		"New Zealand": "Nueva Zelanda",
		"Brazil":      "Brasil", "Mexico": "México", "Ireland": "Irlanda",
		"Romania": "Rumanía", "Ukraine": "Ucrania", "South Africa": "Sudáfrica",
		"European Russia": "Rusia europea", "Asiatic Russia": "Rusia asiática",
	}
	if translated, ok := translations[name]; ok {
		return translated
	}
	return name
}

// ft8FallbackCallRegion keeps useful identification when an installation has
// lost or damaged its local CTY database.
func ft8FallbackCallRegion(call string) string {
	call = strings.ToUpper(strings.TrimSpace(call))
	prefixes := []struct{ prefix, name string }{
		{"EA", "España"}, {"DL", "Alemania"}, {"G", "Reino Unido"}, {"M", "Reino Unido"},
		{"F", "Francia"}, {"I", "Italia"}, {"IK", "Italia"}, {"IZ", "Italia"}, {"CT", "Portugal"},
		{"PA", "Países Bajos"}, {"ON", "Bélgica"}, {"OE", "Austria"}, {"HB", "Suiza"},
		{"SP", "Polonia"}, {"SM", "Suecia"}, {"OH", "Finlandia"}, {"JA", "Japón"},
		{"VE", "Canadá"}, {"VK", "Australia"}, {"ZL", "Nueva Zelanda"},
	}
	for _, item := range prefixes {
		if strings.HasPrefix(call, item.prefix) {
			return item.name
		}
	}
	if len(call) > 0 && strings.Contains("KNAW", call[:1]) {
		return "Estados Unidos"
	}
	return "Otras regiones"
}

func (p *FT8Panel) drawFT8MapDetail() {
	panel := rl.Rectangle{X: 1120, Y: 126, Width: 390, Height: 492}
	rl.DrawRectangleRounded(panel, .025, 8, mixColor(colors.panelAlt, colors.blue, .05))
	rl.DrawRectangleRoundedLinesEx(panel, .025, 8, 1, colors.border)
	simpleui.DrawTextStyled("ESTACIÓN SELECCIONADA", 1143, 146, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	if p.mapSelected < 0 || p.mapSelected >= len(p.mapDecodes) {
		simpleui.DrawText("Sin estación seleccionada", 1143, 190, uiBodyFontSize, colors.muted)
		return
	}
	decode := p.mapDecodes[p.mapSelected]
	station := p.selectedFT8MapStation()
	call := station.call
	if call == "" {
		call = "ESTACIÓN SIN IDENTIFICAR"
	}
	simpleui.DrawTextStyled(call, 1143, 184, 28, simpleui.FontSemiBold, colors.text)
	location := "Sin locator en el mensaje"
	if station.hasPosition {
		location = fmt.Sprintf("%s  ·  %.3f°, %.3f°", station.grid, station.lat, station.lon)
	}
	simpleui.DrawText(location, 1143, 221, uiBodyFontSize, map[bool]rl.Color{true: colors.cyan, false: colors.orange}[station.hasPosition])
	rl.DrawLine(1143, 251, 1488, 251, colors.border)

	detailRows := []struct{ label, value string }{
		{"UTC", decode.Slot.UTC().Format("15:04:05")},
		{"SNR", fmt.Sprintf("%+d dB", decode.SNR)},
		{"DESFASE", fmt.Sprintf("%+.2f s", decode.DT)},
		{"AUDIO", fmt.Sprintf("%.0f Hz", decode.Frequency)},
	}
	if station.hasPosition {
		homeLat, homeLon := p.ft8HomePosition()
		detailRows = append(detailRows,
			struct{ label, value string }{"DISTANCIA", fmt.Sprintf("%.0f km", ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))},
			struct{ label, value string }{"RUMBO", fmt.Sprintf("%.0f°", ft8Bearing(homeLat, homeLon, station.lat, station.lon))},
		)
	}
	for index, row := range detailRows {
		y := float32(275 + index*45)
		simpleui.DrawText(row.label, 1143, y, uiCaptionFontSize, colors.muted)
		width := simpleui.MeasureTextStyled(row.value, uiBodyFontSize, simpleui.FontMono).X
		simpleui.DrawTextStyled(row.value, 1488-width, y-2, uiBodyFontSize, simpleui.FontMono, colors.text)
	}
	y := float32(275 + len(detailRows)*45 + 8)
	simpleui.DrawText("MENSAJE RECIBIDO", 1143, y, uiCaptionFontSize, colors.muted)
	drawWrapped(decode.Message, 1143, y+25, 335, uiBodyFontSize, colors.cyan)
	if !station.hasPosition {
		simpleui.DrawTextStyled("POSICIÓN NO DISPONIBLE", 1143, 568, uiCaptionFontSize, simpleui.FontSemiBold, colors.orange)
	}
	simpleui.DrawText("Arrastra para mover · rueda para zoom · ESC para cerrar", 1143, 598, 10, colors.muted)
}

func (p *FT8Panel) ft8MapStations() []ft8MapStation {
	decodes := p.activityDecodes()
	type location struct {
		grid     string
		lat, lon float64
	}
	locations := make(map[string]location)
	counts := make(map[string]int)
	// Oldest to newest builds a session memory; a later valid locator refreshes
	// the position used by reports, RRR, RR73 and 73 messages.
	for index := len(decodes) - 1; index >= 0; index-- {
		decode := decodes[index]
		call := ft8MessageCall(decode.Message)
		if call == "" {
			continue
		}
		counts[call]++
		_, grid := ft8MessageStation(decode.Message)
		if lat, lon, ok := maidenheadCenter(grid); ok {
			locations[call] = location{grid: grid, lat: lat, lon: lon}
		}
	}
	selectedCall := ""
	if p.mapSelected >= 0 && p.mapSelected < len(p.mapDecodes) {
		selectedCall = ft8MessageCall(p.mapDecodes[p.mapSelected].Message)
	}
	seen := make(map[string]bool)
	stations := make([]ft8MapStation, 0, len(locations))
	for _, decode := range decodes {
		call := ft8MessageCall(decode.Message)
		if call == "" || seen[call] {
			continue
		}
		seen[call] = true
		station := ft8MapStation{call: call, latest: decode, messageCount: counts[call], selected: call == selectedCall}
		station.provisional = ft8StationIsProvisional(station.messageCount, station.latest)
		if value, exists := locations[call]; exists {
			station.grid, station.lat, station.lon, station.hasPosition = value.grid, value.lat, value.lon, true
		}
		stations = append(stations, station)
	}
	for index := range stations {
		if stations[index].selected && index > 0 {
			selected := stations[index]
			copy(stations[1:index+1], stations[0:index])
			stations[0] = selected
			break
		}
	}
	if p.mapOnlyPosition {
		positioned := stations[:0]
		for _, station := range stations {
			if station.hasPosition {
				positioned = append(positioned, station)
			}
		}
		stations = positioned
	}
	return stations
}

// A single decode at the practical FT8 sensitivity floor is useful evidence,
// but not enough to promote a callsign into clusters and country rankings.
// A second reception confirms it regardless of SNR. The raw decode and its
// hollow map contact are always retained.
func ft8StationIsProvisional(messageCount int, latest ft8.Decode) bool {
	return messageCount < 2 && latest.SNR <= -24
}

func (p *FT8Panel) visibleFT8MapStations() []ft8MapStation {
	return p.filterFT8StationsByRegion(p.ft8MapStations())
}

func (p *FT8Panel) filterFT8StationsByRegion(stations []ft8MapStation) []ft8MapStation {
	if p.mapRegionFilter == "" {
		return stations
	}
	filtered := make([]ft8MapStation, 0, len(stations))
	for _, station := range stations {
		if ft8CallRegion(station.call) == p.mapRegionFilter {
			filtered = append(filtered, station)
		}
	}
	return filtered
}

func (p *FT8Panel) filterFT8DecodesByRegion(decodes []ft8.Decode) []ft8.Decode {
	if p.mapRegionFilter == "" {
		return decodes
	}
	filtered := make([]ft8.Decode, 0, len(decodes))
	for _, decode := range decodes {
		matched := false
		if event, ok := classifyFT8Activity(decode); ok {
			matched = (event.from != "" && ft8CallRegion(event.from) == p.mapRegionFilter) ||
				(event.to != "" && ft8CallRegion(event.to) == p.mapRegionFilter)
		} else if call := ft8MessageCall(decode.Message); call != "" {
			matched = ft8CallRegion(call) == p.mapRegionFilter
		}
		if matched {
			filtered = append(filtered, decode)
		}
	}
	return filtered
}

func (p *FT8Panel) selectedFT8MapStation() ft8MapStation {
	for _, station := range p.ft8MapStations() {
		if station.selected {
			return station
		}
	}
	return ft8MapStation{}
}

func (p *FT8Panel) drawFT8MapTable() {
	bounds := rl.Rectangle{X: 90, Y: 636, Width: 1420, Height: 180}
	rl.DrawRectangleRounded(bounds, .018, 7, colors.background)
	rl.DrawRectangleRoundedLinesEx(bounds, .018, 7, 1, colors.border)
	stations := p.ft8MapStations()
	positioned := 0
	for _, station := range stations {
		if station.hasPosition {
			positioned++
		}
	}
	simpleui.DrawTextStyled("ESTACIONES DE LA SESIÓN", 108, 649, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d estaciones · %d con posición", len(stations), positioned), 345, 651, uiCaptionFontSize, colors.muted)
	drawModalAction(rl.Rectangle{X: 1130, Y: 645, Width: 100, Height: 30}, "TODAS", colors.cyan, true)
	drawModalAction(rl.Rectangle{X: 1240, Y: 645, Width: 90, Height: 30}, "CQ", colors.border, false)
	drawModalAction(rl.Rectangle{X: 1340, Y: 645, Width: 150, Height: 30}, "CONTACTOS", colors.border, false)

	rl.DrawRectangle(91, 681, 1418, 27, colors.panelAlt)
	for _, header := range []struct {
		x    float32
		text string
	}{{108, "UTC"}, {220, "INDICATIVO"}, {390, "LOCATOR"}, {520, "SNR"}, {620, "DISTANCIA"}, {780, "MENSAJE MÁS RECIENTE"}, {1390, "RX"}} {
		simpleui.DrawText(header.text, header.x, 689, 10, colors.cyan)
	}
	homeLat, homeLon := p.ft8HomePosition()
	row := 0
	for _, station := range stations {
		if row >= 4 {
			break
		}
		y := float32(709 + row*26)
		if station.selected {
			selection, _ := tableSelectionColors()
			rl.DrawRectangle(91, int32(y), 1418, 25, selection)
		} else if row%2 == 0 {
			rl.DrawRectangle(91, int32(y), 1418, 25, mixColor(colors.panelAlt, colors.blue, .05))
		}
		distance := "—"
		if station.hasPosition {
			distance = fmt.Sprintf("%.0f km", ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))
		}
		simpleui.DrawTextStyled(station.latest.Slot.UTC().Format("15:04:05"), 108, y+6, 11, simpleui.FontMono, colors.muted)
		simpleui.DrawTextStyled(station.call, 220, y+6, 11, simpleui.FontSemiBold, map[bool]rl.Color{true: colors.text, false: colors.muted}[station.selected])
		simpleui.DrawText(station.gridOrDash(), 390, y+6, 11, colors.text)
		simpleui.DrawTextStyled(fmt.Sprintf("%+d", station.latest.SNR), 520, y+6, 11, simpleui.FontMono, ft8SNRColor(station.latest.SNR))
		simpleui.DrawText(distance, 620, y+6, 11, colors.text)
		simpleui.DrawText(clipFT8(station.latest.Message, 62), 780, y+6, 11, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d", station.messageCount), 1400, y+6, 11, colors.muted)
		row++
	}
}

func (station ft8MapStation) gridOrDash() string {
	if station.grid == "" {
		return "—"
	}
	return station.grid
}

func (p *FT8Panel) ft8HomePosition() (float64, float64) {
	if p.mapHomeSet {
		return p.mapHomeLat, p.mapHomeLon
	}
	if p.screen != nil && p.screen.voacapPanel != nil {
		settings := p.screen.voacapPanel.settings
		if settings.Latitude >= -90 && settings.Latitude <= 90 && settings.Longitude >= -180 && settings.Longitude <= 180 {
			return settings.Latitude, settings.Longitude
		}
	}
	return 40.4168, -3.7038
}

func ft8DecodeMapPoint(decode ft8.Decode) ft8MapPoint {
	call, grid := ft8MessageStation(decode.Message)
	lat, lon, ok := maidenheadCenter(grid)
	return ft8MapPoint{decode: decode, call: call, grid: grid, lat: lat, lon: lon, hasPosition: ok}
}

func ft8MessageStation(message string) (call, grid string) {
	fields := strings.Fields(strings.ToUpper(message))
	for index := len(fields) - 1; index >= 0; index-- {
		if _, _, ok := ft8LocatorToken(fields[index]); ok {
			grid = fields[index]
			if index > 0 {
				call = strings.Trim(fields[index-1], "<>")
			}
			return call, grid
		}
	}
	return "", ""
}

func ft8MessageCall(message string) string {
	if call, _ := ft8MessageStation(message); call != "" {
		return call
	}
	fields := strings.Fields(strings.ToUpper(message))
	if len(fields) == 0 {
		return ""
	}
	if fields[0] == "CQ" {
		for index := len(fields) - 1; index >= 1; index-- {
			if ft8PlausibleCall(fields[index]) {
				return strings.Trim(fields[index], "<>")
			}
		}
		return ""
	}
	// In a directed FT8 message the second callsign is the transmitting
	// station; the first is the addressee.
	if len(fields) >= 2 && ft8PlausibleCall(fields[1]) {
		return strings.Trim(fields[1], "<>")
	}
	if ft8PlausibleCall(fields[0]) {
		return strings.Trim(fields[0], "<>")
	}
	return ""
}

func ft8PlausibleCall(value string) bool {
	value = strings.Trim(strings.ToUpper(value), "<>")
	if len(value) < 3 || len(value) > 13 {
		return false
	}
	hasLetter, hasDigit := false, false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '/':
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

func ft8LocatorToken(token string) (lat, lon float64, ok bool) {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "RR73":
		return 0, 0, false
	}
	return maidenheadCenter(token)
}

func maidenheadCenter(locator string) (lat, lon float64, ok bool) {
	locator = strings.ToUpper(strings.TrimSpace(locator))
	if len(locator) != 4 && len(locator) != 6 && len(locator) != 8 {
		return 0, 0, false
	}
	for _, r := range locator {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return 0, 0, false
		}
	}
	a, b := int(locator[0]-'A'), int(locator[1]-'A')
	c, d := int(locator[2]-'0'), int(locator[3]-'0')
	if a < 0 || a >= 18 || b < 0 || b >= 18 || c < 0 || c > 9 || d < 0 || d > 9 {
		return 0, 0, false
	}
	lon, lat = -180+float64(a)*20+float64(c)*2, -90+float64(b)*10+float64(d)
	lonSize, latSize := 2.0, 1.0
	if len(locator) >= 6 {
		e, f := int(locator[4]-'A'), int(locator[5]-'A')
		if e < 0 || e >= 24 || f < 0 || f >= 24 {
			return 0, 0, false
		}
		lonSize, latSize = 2.0/24, 1.0/24
		lon += float64(e) * lonSize
		lat += float64(f) * latSize
	}
	if len(locator) == 8 {
		g, h := int(locator[6]-'0'), int(locator[7]-'0')
		if g < 0 || g > 9 || h < 0 || h > 9 {
			return 0, 0, false
		}
		lonSize, latSize = lonSize/10, latSize/10
		lon += float64(g) * lonSize
		lat += float64(h) * latSize
	}
	return lat + latSize/2, lon + lonSize/2, true
}

func ft8SNRColor(snr int) rl.Color {
	if snr >= -10 {
		return colors.green
	}
	if snr >= -18 {
		return colors.cyan
	}
	return colors.orange
}

func ft8DistanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earth = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := (lat2-lat1)*math.Pi/180, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return earth * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func ft8Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dl := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(dl) * math.Cos(p2)
	x := math.Cos(p1)*math.Sin(p2) - math.Sin(p1)*math.Cos(p2)*math.Cos(dl)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

func drawFT8Route(lat1, lon1, lat2, lon2, centerLat, centerLon, span float64, bounds rl.Rectangle, color rl.Color) {
	previous := projectMercator(centerLat, centerLon, span, lat1, lon1, bounds)
	dLon := wrapLongitude(lon2 - lon1)
	for step := 1; step <= 48; step++ {
		t := float64(step) / 48
		lat := lat1 + (lat2-lat1)*t
		lon := wrapLongitude(lon1 + dLon*t)
		current := projectMercator(centerLat, centerLon, span, lat, lon, bounds)
		if rl.CheckCollisionPointRec(previous, bounds) || rl.CheckCollisionPointRec(current, bounds) {
			rl.DrawLineEx(previous, current, 2, color)
		}
		previous = current
	}
}
