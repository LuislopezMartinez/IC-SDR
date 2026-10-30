package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-zero/internal/resources"
	"go-zero/internal/sstv"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type sstvGallerySnapshot struct {
	Folder       string `json:"folder"`
	Theme        string `json:"theme"`
	SelectedPath string `json:"selectedPath,omitempty"`
}

func (p *SSTVPanel) openSSTVGallery() {
	p.openSSTVGalleryFor(sstv.SavedImage{})
}

// openSSTVGalleryFor opens the full visual editor and, when an item is
// supplied, selects that exact saved frame instead of whichever file happens
// to be newest on disk.
func (p *SSTVPanel) openSSTVGalleryFor(item sstv.SavedImage) {
	if p.screen.receiver == nil {
		p.flash("El receptor SSTV todavía no está disponible")
		return
	}
	snapshot := sstvGallerySnapshot{Folder: p.screen.receiver.SSTVOutputFolder(), Theme: p.screen.themeName, SelectedPath: item.Path}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	path := resources.WritablePath("cache", fmt.Sprintf("sstv-gallery-%d.json", os.Getpid()))
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path, data, 0o644) != nil {
		return
	}
	if p.galleryViewer != nil && p.galleryViewer.Process != nil {
		_ = p.galleryViewer.Process.Kill()
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	command := exec.Command(executable, "--sstv-gallery", path)
	command.SysProcAttr = rtl433ViewerProcessAttributes()
	if command.Start() != nil {
		return
	}
	p.galleryViewer = command
	p.galleryDone = make(chan struct{})
	done := p.galleryDone
	go func() {
		_ = command.Wait()
		_ = os.Remove(path)
		close(done)
	}()
}

type sstvGalleryItem struct {
	path     string
	name     string
	modified time.Time
	size     int64
	texture  rl.Texture2D
	ready    bool
}

type sstvGallery struct {
	folder              string
	items               []sstvGalleryItem
	filter              int // 0 all, 1 received, 2 copies
	view                int // 0 gallery, 1 editor
	chosen              int
	scroll              int
	mode                int // 0 original, 1 compare, 2 enhanced
	split, zoom         float32
	pan, previous       rl.Vector2
	dragging, splitting bool
	preview             rl.Texture2D
	previewReady        bool
	previewPath         string
	options             sstv.EnhancementOptions
	message             string
	lastSelectionTime   float32
}

func RunSSTVGallery(snapshotPath string) {
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		return
	}
	var snapshot sstvGallerySnapshot
	if json.Unmarshal(data, &snapshot) != nil || snapshot.Folder == "" {
		return
	}
	colors = paletteForTheme(snapshot.Theme)
	simpleui.SetMode(1600, 900, simpleui.Stretch)
	simpleui.SetInitialWindowSize(1420, 820)
	simpleui.SetMinimumSize(1024, 640)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	// The editor is normally displayed scaled down from its 1600×900 canvas.
	// Its cards have generous vertical margins, so use them for larger labels
	// and descriptions instead of leaving the operational text undersized.
	simpleui.SetTextScale(1.20)
	simpleui.SetTitle("IC-SDR · GALERÍA SSTV")
	simpleui.SetTheme(simpleUITheme(colors, snapshot.Theme == themeLight))
	// The gallery opens directly in the visual comparator. The thumbnail rail
	// is the navigator, so the first thing a user sees is the received image
	// at a useful size rather than a dense file grid.
	gallery := &sstvGallery{folder: snapshot.Folder, chosen: -1, split: 718, zoom: 1, mode: 1, view: 1}
	gallery.reload()
	if snapshot.SelectedPath != "" {
		for index := range gallery.items {
			if filepath.Clean(gallery.items[index].path) == filepath.Clean(snapshot.SelectedPath) {
				gallery.chosen = index
				gallery.loadPreview()
				break
			}
		}
	}
	defer gallery.close()
	simpleui.Run(gallery.draw)
}

func (g *sstvGallery) reload() {
	for i := range g.items {
		if g.items[i].ready {
			rl.UnloadTexture(g.items[i].texture)
		}
	}
	g.items = nil
	entries, err := os.ReadDir(g.folder)
	if err != nil {
		g.message = "No se pudo abrir la carpeta de imágenes SSTV"
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		g.items = append(g.items, sstvGalleryItem{path: filepath.Join(g.folder, entry.Name()), name: entry.Name(), modified: info.ModTime(), size: info.Size()})
	}
	sort.Slice(g.items, func(i, j int) bool { return g.items[i].modified.After(g.items[j].modified) })
	if len(g.items) > 0 {
		g.chosen = 0
		g.loadPreview()
	}
}

func (g *sstvGallery) current() *sstvGalleryItem {
	if g.chosen < 0 || g.chosen >= len(g.items) {
		return nil
	}
	return &g.items[g.chosen]
}

func (g *sstvGallery) ensureTexture(index int) bool {
	if index < 0 || index >= len(g.items) {
		return false
	}
	if g.items[index].ready {
		return true
	}
	g.items[index].texture, g.items[index].ready = loadSSTVCompareTexture(g.items[index].path)
	return g.items[index].ready
}

func (g *sstvGallery) loadPreview() {
	if g.previewReady {
		rl.UnloadTexture(g.preview)
		g.previewReady = false
	}
	item := g.current()
	if item == nil {
		return
	}
	g.preview, g.previewReady = loadSSTVCompareTexture(item.path)
	g.previewPath = item.path
	g.zoom, g.pan = 1, rl.Vector2{}
}

func (g *sstvGallery) renderPreview() {
	item := g.current()
	if item == nil {
		return
	}
	path := resources.WritablePath("cache", fmt.Sprintf("sstv-gallery-preview-%d.png", os.Getpid()))
	result, err := sstv.EnhancePNG(item.path, path, g.options)
	if err != nil {
		g.message = err.Error()
		return
	}
	if g.previewReady {
		rl.UnloadTexture(g.preview)
		g.previewReady = false
	}
	g.preview, g.previewReady = loadSSTVCompareTexture(path)
	g.previewPath = path
	g.message = fmt.Sprintf("Vista previa: %d alineadas · %d reconstruidas · %d píxeles", result.CorrectedLines, result.ReconstructedLines, result.DenoisedPixels)
}

func (g *sstvGallery) visible(index int) bool {
	if index < 0 || index >= len(g.items) {
		return false
	}
	copyImage := strings.Contains(strings.ToLower(g.items[index].name), "-mejorada") || strings.Contains(strings.ToLower(g.items[index].name), "-galeria-")
	return g.filter == 0 || (g.filter == 1 && !copyImage) || (g.filter == 2 && copyImage)
}

func (g *sstvGallery) filtered() []int {
	indices := make([]int, 0, len(g.items))
	for i := range g.items {
		if g.visible(i) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (g *sstvGallery) draw() {
	rl.ClearBackground(colors.background)
	if g.view == 0 {
		g.drawLibrary()
	} else {
		g.drawEditor()
	}
}

func (g *sstvGallery) drawHeader(title, subtitle string) {
	rl.DrawRectangle(0, 0, 1600, 66, colors.panel)
	rl.DrawLine(0, 65, 1600, 65, colors.border)
	simpleui.DrawTextStyled(title, 26, 19, 22, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(subtitle, 260, 26, 11, colors.muted)
}

func (g *sstvGallery) drawLibrary() {
	g.drawHeader("GALERÍA SSTV", fmt.Sprintf("%d imágenes · %s", len(g.filtered()), g.folder))
	pointer, released := simpleui.MousePosition(), rl.IsMouseButtonReleased(rl.MouseButtonLeft)
	rl.DrawRectangle(0, 66, 238, 834, colors.panel)
	rl.DrawLine(238, 66, 238, 900, colors.border)
	simpleui.DrawTextStyled("BIBLIOTECA", 24, 96, 13, simpleui.FontSemiBold, colors.cyan)
	filters := []struct {
		label string
		count int
	}{{"TODAS", len(g.filteredFor(0))}, {"RECIBIDAS", len(g.filteredFor(1))}, {"COPIAS MEJORADAS", len(g.filteredFor(2))}}
	for i, filter := range filters {
		bounds := rl.Rectangle{X: 18, Y: float32(128 + i*48), Width: 202, Height: 38}
		if released && rl.CheckCollisionPointRec(pointer, bounds) {
			g.filter, g.scroll = i, 0
		}
		g.drawButton(bounds, fmt.Sprintf("%s  %d", filter.label, filter.count), g.filter == i, true)
	}
	simpleui.DrawTextStyled("ACCIONES", 24, 304, 12, simpleui.FontSemiBold, colors.muted)
	reload := rl.Rectangle{X: 18, Y: 330, Width: 202, Height: 38}
	folder := rl.Rectangle{X: 18, Y: 378, Width: 202, Height: 38}
	if released && rl.CheckCollisionPointRec(pointer, reload) {
		g.reload()
	}
	if released && rl.CheckCollisionPointRec(pointer, folder) {
		_ = exec.Command("explorer.exe", g.folder).Start()
	}
	g.drawButton(reload, "ACTUALIZAR GALERÍA", false, true)
	g.drawButton(folder, "ABRIR CARPETA", false, true)

	indices := g.filtered()
	if rl.CheckCollisionPointRec(pointer, rl.Rectangle{X: 250, Y: 66, Width: 960, Height: 834}) {
		wheel := int(rl.GetMouseWheelMove())
		if wheel != 0 {
			g.scroll = max(g.scroll-wheel*3, 0)
		}
	}
	maxScroll := max(len(indices)-12, 0)
	g.scroll = min(g.scroll, maxScroll)
	start := g.scroll
	for cell := 0; cell < 12 && start+cell < len(indices); cell++ {
		index := indices[start+cell]
		column, row := cell%3, cell/3
		bounds := rl.Rectangle{X: 266 + float32(column)*300, Y: 94 + float32(row)*184, Width: 276, Height: 164}
		if released && rl.CheckCollisionPointRec(pointer, bounds) {
			g.chosen = index
			g.loadPreview()
			if rl.GetTime()-float64(g.lastSelectionTime) < .35 {
				g.view = 1
			}
			g.lastSelectionTime = float32(rl.GetTime())
		}
		g.drawThumbnail(index, bounds, index == g.chosen)
	}
	g.drawInspector(pointer, released)
}

func (g *sstvGallery) filteredFor(filter int) []int {
	previous := g.filter
	g.filter = filter
	result := g.filtered()
	g.filter = previous
	return result
}

func (g *sstvGallery) drawThumbnail(index int, bounds rl.Rectangle, selected bool) {
	fill := colors.panel
	border := colors.border
	if selected {
		fill, border = mixColor(colors.panel, colors.cyan, .16), colors.cyan
	}
	rl.DrawRectangleRounded(bounds, .06, 5, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .06, 5, 1, border)
	imageBounds := rl.Rectangle{X: bounds.X + 8, Y: bounds.Y + 8, Width: bounds.Width - 16, Height: 112}
	rl.DrawRectangleRec(imageBounds, rl.Color{R: 3, G: 6, B: 9, A: 255})
	if g.ensureTexture(index) {
		texture := g.items[index].texture
		destination := fitRectangle(imageBounds.X, imageBounds.Y, imageBounds.Width, imageBounds.Height, float32(texture.Width), float32(texture.Height))
		rl.DrawTexturePro(texture, rl.Rectangle{Width: float32(texture.Width), Height: float32(texture.Height)}, destination, rl.Vector2{}, 0, rl.White)
	}
	simpleui.DrawTextStyled(trimMemory(g.items[index].name, 34), bounds.X+10, bounds.Y+128, 10, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText(g.items[index].modified.Format("02/01/2006 15:04"), bounds.X+10, bounds.Y+145, 9, colors.muted)
}

func (g *sstvGallery) drawInspector(pointer rl.Vector2, released bool) {
	panel := rl.Rectangle{X: 1184, Y: 82, Width: 394, Height: 792}
	rl.DrawRectangleRounded(panel, .05, 5, colors.panel)
	rl.DrawRectangleRoundedLinesEx(panel, .05, 5, 1, colors.border)
	simpleui.DrawTextStyled("INSPECTOR", 1210, 110, 13, simpleui.FontSemiBold, colors.cyan)
	item := g.current()
	if item == nil {
		simpleui.DrawText("Selecciona una imagen", 1210, 150, 12, colors.muted)
		return
	}
	preview := rl.Rectangle{X: 1210, Y: 142, Width: 342, Height: 220}
	rl.DrawRectangleRec(preview, rl.Color{R: 3, G: 6, B: 9, A: 255})
	if g.ensureTexture(g.chosen) {
		texture := g.items[g.chosen].texture
		dest := fitRectangle(preview.X, preview.Y, preview.Width, preview.Height, float32(texture.Width), float32(texture.Height))
		rl.DrawTexturePro(texture, rl.Rectangle{Width: float32(texture.Width), Height: float32(texture.Height)}, dest, rl.Vector2{}, 0, rl.White)
	}
	simpleui.DrawTextStyled(trimMemory(item.name, 40), 1210, 386, 13, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText(item.modified.Format("Recibida el 02/01/2006 a las 15:04"), 1210, 414, 11, colors.muted)
	simpleui.DrawText(fmt.Sprintf("%.1f KB", float64(item.size)/1024), 1210, 437, 11, colors.muted)
	open := rl.Rectangle{X: 1210, Y: 486, Width: 342, Height: 44}
	compare := rl.Rectangle{X: 1210, Y: 540, Width: 342, Height: 40}
	if released && rl.CheckCollisionPointRec(pointer, open) {
		g.view = 1
	}
	if released && rl.CheckCollisionPointRec(pointer, compare) {
		g.view = 1
		g.mode = 1
	}
	g.drawButton(open, "ABRIR EN EL EDITOR", true, true)
	g.drawButton(compare, "ABRIR COMPARADOR", false, true)
	simpleui.DrawTextStyled("No se modifica nunca la recepción original.", 1210, 630, 10, simpleui.FontSemiBold, colors.green)
	simpleui.DrawText("Las mejoras se guardan como una copia PNG.", 1210, 650, 10, colors.muted)
}

func (g *sstvGallery) drawEditor() {
	pointer := simpleui.MousePosition()
	pressed := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	down := rl.IsMouseButtonDown(rl.MouseButtonLeft)
	released := rl.IsMouseButtonReleased(rl.MouseButtonLeft)
	// The gallery draws its own controls. Activating on press is more reliable
	// in a stretched auxiliary window than waiting for the release event.
	activated := pressed

	// Header, navigator, central comparison and improvements rail follow the
	// same visual hierarchy as the SSTV workspace: the image is dominant and
	// the controls remain large enough to use from a distance.
	rl.DrawRectangle(0, 0, 1600, 64, colors.panel)
	rl.DrawLine(0, 63, 1600, 63, colors.border)
	simpleui.DrawTextStyled("IC-SDR", 22, 16, 28, simpleui.FontSemiBold, colors.cyan)
	rl.DrawLine(206, 12, 206, 51, colors.border)
	simpleui.DrawTextStyled("COMPARADOR Y MEJORA SSTV", 230, 19, 22, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText("Original · mejoras no destructivas · arrastra el divisor para comparar", 230, 45, 11, colors.muted)

	g.drawFilmstrip(pointer, activated)
	canvas := rl.Rectangle{X: 220, Y: 76, Width: 1000, Height: 708}
	g.handleCanvas(canvas, pointer, pressed, down, released)
	rl.DrawRectangleRec(canvas, rl.Color{R: 3, G: 6, B: 9, A: 255})
	rl.DrawRectangleRoundedLinesEx(canvas, .015, 5, 1, colors.border)
	g.drawEditorImage(canvas)
	g.drawImageCaption(canvas, pointer, activated)
	g.drawEditorControls(pointer, activated)
	g.drawEditorFooter(pointer, activated)
}

func (g *sstvGallery) drawFilmstrip(pointer rl.Vector2, released bool) {
	rl.DrawRectangle(0, 64, 202, 746, colors.panel)
	rl.DrawLine(202, 64, 202, 810, colors.border)
	simpleui.DrawTextStyled(fmt.Sprintf("IMÁGENES SSTV (%d)", len(g.items)), 18, 82, 13, simpleui.FontSemiBold, colors.text)
	if released && rl.CheckCollisionPointRec(pointer, rl.Rectangle{X: 164, Y: 76, Width: 24, Height: 28}) {
		g.view = 0
	}
	g.drawButton(rl.Rectangle{X: 160, Y: 76, Width: 28, Height: 28}, "MENU", false, true)
	if rl.CheckCollisionPointRec(pointer, rl.Rectangle{X: 0, Y: 106, Width: 202, Height: 680}) {
		if wheel := int(rl.GetMouseWheelMove()); wheel != 0 {
			g.scroll = min(max(g.scroll-wheel, 0), max(len(g.items)-8, 0))
		}
	}
	limit := min(len(g.items)-g.scroll, 8)
	for position := 0; position < limit; position++ {
		i := g.scroll + position
		bounds := rl.Rectangle{X: 20, Y: 112 + float32(position)*84, Width: 162, Height: 73}
		if released && rl.CheckCollisionPointRec(pointer, bounds) {
			g.chosen = i
			g.loadPreview()
		}
		g.drawSmallThumbnail(i, bounds, i == g.chosen)
	}
}

func (g *sstvGallery) drawSmallThumbnail(index int, bounds rl.Rectangle, selected bool) {
	fill, border := colors.background, colors.border
	if selected {
		fill, border = mixColor(fill, colors.cyan, .16), colors.cyan
	}
	rl.DrawRectangleRounded(bounds, .06, 5, fill)
	lineWidth := float32(1)
	if selected {
		lineWidth = 2
	}
	rl.DrawRectangleRoundedLinesEx(bounds, .06, 5, lineWidth, border)
	if g.ensureTexture(index) {
		t := g.items[index].texture
		dest := fitRectangle(bounds.X+4, bounds.Y+4, bounds.Width-8, bounds.Height-8, float32(t.Width), float32(t.Height))
		rl.DrawTexturePro(t, rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)}, dest, rl.Vector2{}, 0, rl.White)
	}
}

func (g *sstvGallery) handleCanvas(canvas rl.Rectangle, pointer rl.Vector2, pressed, down, released bool) {
	if rl.CheckCollisionPointRec(pointer, canvas) {
		if wheel := rl.GetMouseWheelMove(); wheel != 0 {
			g.zoom = min(max(g.zoom*(1+wheel*.12), float32(.5)), float32(6))
		}
		if pressed {
			g.splitting = g.mode == 1 && absFloat32(pointer.X-g.split) <= 36
			g.dragging, g.previous = !g.splitting, pointer
		}
		if down && g.splitting {
			g.split = min(max(pointer.X, canvas.X+20), canvas.X+canvas.Width-20)
		} else if down && g.dragging {
			g.pan.X += pointer.X - g.previous.X
			g.pan.Y += pointer.Y - g.previous.Y
			g.previous = pointer
		}
	}
	if released {
		g.dragging, g.splitting = false, false
	}
}

func (g *sstvGallery) drawEditorImage(canvas rl.Rectangle) {
	item := g.current()
	if item == nil || !g.ensureTexture(g.chosen) {
		return
	}
	original := item.texture
	enhanced := g.preview
	if !g.previewReady {
		enhanced = original
	}
	base := fitRectangle(canvas.X, canvas.Y, canvas.Width, canvas.Height, float32(original.Width), float32(original.Height))
	destination := rl.Rectangle{X: base.X + g.pan.X - base.Width*(g.zoom-1)/2, Y: base.Y + g.pan.Y - base.Height*(g.zoom-1)/2, Width: base.Width * g.zoom, Height: base.Height * g.zoom}
	if g.mode == 0 {
		drawSSTVTextureSegment(original, destination, canvas.X, canvas.X+canvas.Width)
		return
	}
	if g.mode == 2 {
		drawSSTVTextureSegment(enhanced, destination, canvas.X, canvas.X+canvas.Width)
		return
	}
	drawSSTVTextureSegment(original, destination, canvas.X, g.split)
	drawSSTVTextureSegment(enhanced, destination, g.split, canvas.X+canvas.Width)
	rl.DrawLineEx(rl.Vector2{X: g.split, Y: canvas.Y}, rl.Vector2{X: g.split, Y: canvas.Y + canvas.Height}, 3, colors.cyan)
	rl.DrawCircleV(rl.Vector2{X: g.split, Y: canvas.Y + canvas.Height/2}, 24, colors.background)
	rl.DrawCircleLines(int32(g.split), int32(canvas.Y+canvas.Height/2), 24, colors.cyan)
	drawCentered("< >", rl.Rectangle{X: g.split - 21, Y: canvas.Y + canvas.Height/2 - 11, Width: 42, Height: 22}, 15, colors.cyan)
}

func (g *sstvGallery) drawImageCaption(canvas rl.Rectangle, pointer rl.Vector2, released bool) {
	modes := []struct {
		label  string
		mode   int
		bounds rl.Rectangle
	}{
		{"ORIGINAL", 0, rl.Rectangle{X: canvas.X + 18, Y: canvas.Y + 14, Width: 140, Height: 38}},
		{"COMPARAR", 1, rl.Rectangle{X: canvas.X + canvas.Width/2 - 70, Y: canvas.Y + 14, Width: 140, Height: 38}},
		{"VISTA MEJORADA", 2, rl.Rectangle{X: canvas.X + canvas.Width - 206, Y: canvas.Y + 14, Width: 188, Height: 38}},
	}
	for _, entry := range modes {
		if released && rl.CheckCollisionPointRec(pointer, entry.bounds) {
			g.mode = entry.mode
		}
		g.drawGalleryAction(entry.bounds, entry.label, g.mode == entry.mode, false)
	}
}

func (g *sstvGallery) drawEditorControls(pointer rl.Vector2, released bool) {
	rl.DrawRectangle(1240, 64, 360, 746, colors.panel)
	rl.DrawLine(1240, 64, 1240, 810, colors.border)
	simpleui.DrawTextStyled("MEJORAS", 1262, 82, 23, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("Ajustes sobre una copia", 1264, 112, 12, colors.muted)
	groups := []struct {
		title string
		rows  []struct {
			label, detail string
			value         *bool
		}
	}{
		{"COLOR", []struct {
			label, detail string
			value         *bool
		}{{"NIVELES DE COLOR", "ajuste RGB conservador", &g.options.ColorCorrection}, {"BALANCE DE COLOR", "neutraliza dominantes", &g.options.WhiteBalance}, {"CONTRASTE", "expande tonos medios", &g.options.Contrast}}},
		{"RECEPCIÓN", []struct {
			label, detail string
			value         *bool
		}{{"RECUPERAR LÍNEAS", "rellena huecos breves", &g.options.RecoverLines}, {"ALINEAR FILAS", "corrige desplazamientos horizontales", &g.options.AlignRows}}},
		{"RUIDO", []struct {
			label, detail string
			value         *bool
		}{{"REDUCCIÓN DE RUIDO", "elimina píxeles impulsivos", &g.options.ReduceNoise}, {"SUAVIZADO", "promedia ruido granular", &g.options.Smooth}, {"BANDAS HORIZONTALES", "compensa niveles por línea", &g.options.ReduceBands}}},
		{"DETALLE", []struct {
			label, detail string
			value         *bool
		}{{"NITIDEZ", "refuerza contornos suaves", &g.options.Sharpen}}},
		{"ANÁLISIS", []struct {
			label, detail string
			value         *bool
		}{{"DETECCIÓN DE CONTORNOS", "vista de bordes en gris", &g.options.Contours}}},
	}
	y := float32(145)
	for _, group := range groups {
		simpleui.DrawTextStyled(group.title, 1262, y, 12, simpleui.FontSemiBold, colors.cyan)
		y += 22
		for _, row := range group.rows {
			bounds := rl.Rectangle{X: 1258, Y: y, Width: 324, Height: 44}
			if released && rl.CheckCollisionPointRec(pointer, bounds) {
				*row.value = !*row.value
				g.renderPreview()
				g.mode = 1
			}
			rl.DrawRectangleRounded(bounds, .08, 6, colors.panelAlt)
			rl.DrawRectangleRoundedLinesEx(bounds, .08, 6, 1, colors.border)
			simpleui.DrawTextStyled(row.label, bounds.X+15, bounds.Y+6, 12, simpleui.FontSemiBold, colors.text)
			simpleui.DrawText(row.detail, bounds.X+15, bounds.Y+24, 10, colors.muted)
			g.drawGalleryToggle(rl.Rectangle{X: bounds.X + bounds.Width - 53, Y: bounds.Y + 12, Width: 36, Height: 20}, *row.value)
			y += 49
		}
		y += 8
	}
}

func (g *sstvGallery) drawEditorFooter(pointer rl.Vector2, released bool) {
	rl.DrawRectangle(0, 810, 1600, 90, colors.panel)
	rl.DrawLine(0, 810, 1600, 810, colors.border)
	fit := rl.Rectangle{X: 28, Y: 830, Width: 142, Height: 46}
	reset := rl.Rectangle{X: 806, Y: 830, Width: 196, Height: 46}
	save := rl.Rectangle{X: 1022, Y: 830, Width: 254, Height: 46}
	export := rl.Rectangle{X: 1296, Y: 830, Width: 254, Height: 46}
	if released && rl.CheckCollisionPointRec(pointer, fit) {
		g.zoom, g.pan = 1, rl.Vector2{}
	}
	if released && rl.CheckCollisionPointRec(pointer, reset) {
		g.options = sstv.EnhancementOptions{}
		g.loadPreview()
		g.message = "Ajustes restablecidos"
	}
	if released && rl.CheckCollisionPointRec(pointer, save) {
		g.saveCopy()
	}
	if released && rl.CheckCollisionPointRec(pointer, export) {
		if item := g.current(); item != nil {
			_ = exec.Command("explorer.exe", "/select,", item.path).Start()
		}
	}
	g.drawGalleryAction(fit, "100% · AJUSTAR", false, false)
	g.drawGalleryAction(reset, "RESTABLECER", false, false)
	g.drawGalleryAction(save, "GUARDAR COPIA", true, true)
	g.drawGalleryAction(export, "EXPORTAR", true, true)
	if g.message != "" {
		simpleui.DrawText(trimMemory(g.message, 66), 198, 848, 11, colors.muted)
	}
}

func (g *sstvGallery) drawImprovementThumbnails(bounds rl.Rectangle, index int) {
	left := rl.Rectangle{X: bounds.X + 16, Y: bounds.Y + 64, Width: 124, Height: 62}
	right := rl.Rectangle{X: bounds.X + 184, Y: bounds.Y + 64, Width: 124, Height: 62}
	for _, box := range []rl.Rectangle{left, right} {
		rl.DrawRectangleRounded(box, .06, 4, colors.background)
		rl.DrawRectangleRoundedLinesEx(box, .06, 4, 1, colors.border)
	}
	if g.ensureTexture(index) {
		t := g.items[index].texture
		d := fitRectangle(left.X+3, left.Y+3, left.Width-6, left.Height-6, float32(t.Width), float32(t.Height))
		rl.DrawTexturePro(t, rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)}, d, rl.Vector2{}, 0, rl.White)
	}
	if g.previewReady {
		t := g.preview
		d := fitRectangle(right.X+3, right.Y+3, right.Width-6, right.Height-6, float32(t.Width), float32(t.Height))
		rl.DrawTexturePro(t, rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)}, d, rl.Vector2{}, 0, rl.White)
	}
	drawCentered(">", rl.Rectangle{X: bounds.X + 145, Y: bounds.Y + 82, Width: 30, Height: 26}, 25, colors.cyan)
}

func (g *sstvGallery) drawGalleryToggle(bounds rl.Rectangle, on bool) {
	fill := colors.muted
	if on {
		fill = colors.cyan
	}
	rl.DrawRectangleRounded(bounds, .5, 8, fill)
	x := bounds.X + 10
	if on {
		x = bounds.X + bounds.Width - 10
	}
	rl.DrawCircleV(rl.Vector2{X: x, Y: bounds.Y + bounds.Height/2}, 7, colors.text)
}

func (g *sstvGallery) drawGalleryAction(bounds rl.Rectangle, label string, selected, warm bool) {
	fill, border := colors.panelAlt, colors.border
	if selected {
		if warm {
			fill, border = colors.orange, colors.orange
		} else {
			fill, border = mixColor(colors.panelAlt, colors.cyan, .28), colors.cyan
		}
	}
	rl.DrawRectangleRounded(bounds, .10, 7, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .10, 7, 1, border)
	drawCentered(label, bounds, 13, simpleui.EnsureTextContrast(colors.text, fill))
}

func (g *sstvGallery) saveCopy() {
	item := g.current()
	if item == nil {
		return
	}
	sourcePath := item.path
	base := strings.TrimSuffix(item.name, filepath.Ext(item.name))
	destination := filepath.Join(g.folder, base+"-galeria-mejorada-"+time.Now().Format("20060102-150405")+".png")
	result, err := sstv.EnhancePNG(item.path, destination, g.options)
	if err != nil {
		g.message = err.Error()
		return
	}
	g.message = fmt.Sprintf("Copia guardada: %s · %d líneas corregidas", filepath.Base(destination), result.CorrectedLines)
	g.reload()
	// Continue editing the received original. Automatically selecting the new
	// copy invites users to apply the same filters repeatedly without noticing.
	for index := range g.items {
		if filepath.Clean(g.items[index].path) == filepath.Clean(sourcePath) {
			g.chosen = index
			g.loadPreview()
			break
		}
	}
}

func (g *sstvGallery) drawButton(bounds rl.Rectangle, label string, selected, active bool) {
	fill, border, textColor := colors.panelAlt, colors.border, colors.text
	if selected {
		fill, border = mixColor(colors.panelAlt, colors.cyan, .25), colors.cyan
	}
	if !active {
		textColor = colors.muted
	}
	rl.DrawRectangleRounded(bounds, .10, 6, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .10, 6, 1, border)
	drawCentered(label, bounds, 10, textColor)
}

func (g *sstvGallery) close() {
	for i := range g.items {
		if g.items[i].ready {
			rl.UnloadTexture(g.items[i].texture)
		}
	}
	if g.previewReady {
		rl.UnloadTexture(g.preview)
	}
	if g.previewPath != "" && strings.Contains(g.previewPath, "sstv-gallery-preview-") {
		_ = os.Remove(g.previewPath)
	}
}
