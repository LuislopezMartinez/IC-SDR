package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go-zero/internal/resources"
	"go-zero/internal/sstv"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type sstvCompareSnapshot struct {
	Image sstv.SavedImage `json:"image"`
	Theme string          `json:"theme"`
}

func (p *SSTVPanel) openSSTVComparator(item sstv.SavedImage) {
	snapshot := sstvCompareSnapshot{Image: item, Theme: p.screen.themeName}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	path := resources.WritablePath("cache", fmt.Sprintf("sstv-compare-%d.json", os.Getpid()))
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path, data, 0o644) != nil {
		return
	}
	if p.compareViewer != nil && p.compareViewer.Process != nil {
		_ = p.compareViewer.Process.Kill()
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	command := exec.Command(executable, "--sstv-compare", path)
	command.SysProcAttr = rtl433ViewerProcessAttributes()
	if command.Start() != nil {
		return
	}
	p.compareViewer = command
	p.compareDone = make(chan struct{})
	done := p.compareDone
	go func() {
		_ = command.Wait()
		_ = os.Remove(path)
		close(done)
	}()
}

type sstvComparator struct {
	item                         sstv.SavedImage
	original, enhanced           rl.Texture2D
	originalReady, enhancedReady bool
	mode                         int // 0 original, 1 compare, 2 enhanced
	split, zoom                  float32
	pan, previous                rl.Vector2
	dragging, splitting          bool
}

func RunSSTVComparator(snapshotPath string) {
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		return
	}
	var snapshot sstvCompareSnapshot
	if json.Unmarshal(data, &snapshot) != nil || snapshot.Image.Path == "" {
		return
	}
	colors = paletteForTheme(snapshot.Theme)
	simpleui.SetMode(1200, 800, simpleui.Stretch)
	simpleui.SetInitialWindowSize(1200, 800)
	simpleui.SetMinimumSize(760, 500)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.15)
	simpleui.SetTitle("IC-SDR · COMPARADOR SSTV")
	simpleui.SetMaximizeKey(0)
	simpleui.SetTheme(simpleUITheme(colors, snapshot.Theme == themeLight))
	viewer := &sstvComparator{item: snapshot.Image, split: 600, zoom: 1, mode: 1}
	viewer.original, viewer.originalReady = loadSSTVCompareTexture(snapshot.Image.Path)
	viewer.enhanced, viewer.enhancedReady = loadSSTVCompareTexture(snapshot.Image.EnhancedPath)
	if !viewer.enhancedReady {
		viewer.mode = 0
	}
	simpleui.Run(viewer.draw)
}

func loadSSTVCompareTexture(path string) (rl.Texture2D, bool) {
	if path == "" {
		return rl.Texture2D{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rl.Texture2D{}, false
	}
	image := rl.LoadImageFromMemory(filepath.Ext(path), data, int32(len(data)))
	if !rl.IsImageValid(image) {
		return rl.Texture2D{}, false
	}
	texture := rl.LoadTextureFromImage(image)
	rl.UnloadImage(image)
	if !rl.IsTextureValid(texture) {
		return rl.Texture2D{}, false
	}
	rl.SetTextureFilter(texture, rl.FilterBilinear)
	return texture, true
}

func (v *sstvComparator) draw() {
	pointer := simpleui.MousePosition()
	pressed := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	down := rl.IsMouseButtonDown(rl.MouseButtonLeft)
	released := rl.IsMouseButtonReleased(rl.MouseButtonLeft)
	canvas := rl.Rectangle{X: 24, Y: 112, Width: 1152, Height: 594}

	buttons := []struct {
		bounds rl.Rectangle
		label  string
		mode   int
		active bool
	}{
		{rl.Rectangle{X: 24, Y: 58, Width: 128, Height: 36}, "ORIGINAL", 0, true},
		{rl.Rectangle{X: 160, Y: 58, Width: 128, Height: 36}, "COMPARAR", 1, v.enhancedReady},
		{rl.Rectangle{X: 296, Y: 58, Width: 128, Height: 36}, "MEJORADA", 2, v.enhancedReady},
	}
	for _, button := range buttons {
		if released && button.active && rl.CheckCollisionPointRec(pointer, button.bounds) {
			v.mode = button.mode
		}
		drawSSTVCompareButton(button.bounds, button.label, v.mode == button.mode, button.active)
	}
	fitButton := rl.Rectangle{X: 440, Y: 58, Width: 116, Height: 36}
	folderButton := rl.Rectangle{X: 1010, Y: 58, Width: 166, Height: 36}
	if released && rl.CheckCollisionPointRec(pointer, fitButton) {
		v.zoom, v.pan = 1, rl.Vector2{}
	}
	if released && rl.CheckCollisionPointRec(pointer, folderButton) {
		_ = exec.Command("explorer.exe", "/select,", v.item.Path).Start()
	}
	drawSSTVCompareButton(fitButton, "AJUSTAR", false, true)
	drawSSTVCompareButton(folderButton, "ABRIR CARPETA", false, true)

	if rl.CheckCollisionPointRec(pointer, canvas) {
		wheel := rl.GetMouseWheelMove()
		if wheel != 0 {
			v.zoom = min(max(v.zoom*(1+wheel*.12), float32(.5)), float32(6))
		}
		if pressed {
			v.splitting = v.mode == 1 && absFloat32(pointer.X-v.split) <= 12
			v.dragging, v.previous = !v.splitting, pointer
		}
		if down && v.splitting {
			v.split = min(max(pointer.X, canvas.X+20), canvas.X+canvas.Width-20)
		} else if down && v.dragging {
			v.pan.X += pointer.X - v.previous.X
			v.pan.Y += pointer.Y - v.previous.Y
			v.previous = pointer
		}
	}
	if released {
		v.dragging, v.splitting = false, false
	}

	rl.DrawRectangleRec(canvas, rl.Color{R: 3, G: 6, B: 9, A: 255})
	rl.DrawRectangleLinesEx(canvas, 1, colors.border)
	v.drawImages(canvas)
	v.drawHeader()
	v.drawFooter()
}

func (v *sstvComparator) drawImages(canvas rl.Rectangle) {
	texture := v.original
	if v.mode == 2 && v.enhancedReady {
		texture = v.enhanced
	}
	if !rl.IsTextureValid(texture) {
		return
	}
	base := fitRectangle(canvas.X, canvas.Y, canvas.Width, canvas.Height, float32(texture.Width), float32(texture.Height))
	destination := rl.Rectangle{
		X:     base.X + v.pan.X - base.Width*(v.zoom-1)/2,
		Y:     base.Y + v.pan.Y - base.Height*(v.zoom-1)/2,
		Width: base.Width * v.zoom, Height: base.Height * v.zoom,
	}
	if v.mode != 1 || !v.enhancedReady {
		drawSSTVTextureSegment(texture, destination, canvas.X, canvas.X+canvas.Width)
		return
	}
	drawSSTVTextureSegment(v.original, destination, canvas.X, v.split)
	drawSSTVTextureSegment(v.enhanced, destination, v.split, canvas.X+canvas.Width)
	rl.DrawLineEx(rl.Vector2{X: v.split, Y: canvas.Y}, rl.Vector2{X: v.split, Y: canvas.Y + canvas.Height}, 3, colors.cyan)
	rl.DrawCircleV(rl.Vector2{X: v.split, Y: canvas.Y + canvas.Height/2}, 10, colors.cyan)
}

func drawSSTVTextureSegment(texture rl.Texture2D, destination rl.Rectangle, clipLeft, clipRight float32) {
	left := max(destination.X, clipLeft)
	right := min(destination.X+destination.Width, clipRight)
	if right <= left || destination.Width <= 0 {
		return
	}
	start := (left - destination.X) / destination.Width
	end := (right - destination.X) / destination.Width
	source := rl.Rectangle{X: start * float32(texture.Width), Width: (end - start) * float32(texture.Width), Height: float32(texture.Height)}
	target := rl.Rectangle{X: left, Y: destination.Y, Width: right - left, Height: destination.Height}
	rl.DrawTexturePro(texture, source, target, rl.Vector2{}, 0, rl.White)
}

func (v *sstvComparator) drawHeader() {
	simpleui.DrawTextStyled("COMPARADOR SSTV", 24, 18, 22, simpleui.FontSemiBold, colors.cyan)
	status := v.item.Quality
	if v.item.Partial {
		status = "PARCIAL · " + status
	} else {
		status = "COMPLETA · " + status
	}
	simpleui.DrawTextStyled(status, 250, 24, 12, simpleui.FontSemiBold, map[bool]rl.Color{true: colors.orange, false: colors.green}[v.item.Partial])
	simpleui.DrawText("Rueda: zoom · arrastra: desplazar · arrastra el divisor para comparar", 600, 25, 10, colors.muted)
}

func (v *sstvComparator) drawFooter() {
	y := float32(725)
	simpleui.DrawTextStyled(v.item.Mode, 24, y, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(v.item.SavedAt.Format("02/01/2006 15:04:05"), 150, y+1, 11, colors.text)
	simpleui.DrawText(fmt.Sprintf("LÍNEAS %d/%d", v.item.Lines, v.item.Height), 355, y+1, 11, colors.text)
	simpleui.DrawText(fmt.Sprintf("ALINEADAS %d", v.item.CorrectedLines), 505, y+1, 11, colors.orange)
	simpleui.DrawText(fmt.Sprintf("RECONSTRUIDAS %d", v.item.ReconstructedLines), 650, y+1, 11, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("RUIDO %d px", v.item.DenoisedPixels), 825, y+1, 11, colors.green)
	simpleui.DrawText(trimMemory(v.item.Reason, 28), 965, y+1, 11, colors.muted)
	if !v.enhancedReady {
		simpleui.DrawText("No fue necesaria una versión mejorada", 24, 757, 10, colors.muted)
	}
}

func drawSSTVCompareButton(bounds rl.Rectangle, label string, selected, active bool) {
	fill := colors.panelAlt
	border, textColor := colors.border, colors.text
	if selected {
		fill, border = mixColor(colors.panelAlt, colors.cyan, .22), colors.cyan
	}
	if !active {
		textColor = colors.muted
	}
	rl.DrawRectangleRounded(bounds, .12, 6, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .12, 6, 1, border)
	drawCentered(label, bounds, 10, textColor)
}
