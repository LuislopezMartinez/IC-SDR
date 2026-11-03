package screens

import (
	"context"
	"fmt"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/update"
	"go-zero/simpleui"
)

// UpdateDialog checks in the background so startup and radio reception are
// never delayed by the network.
type UpdateDialog struct {
	simpleui.BaseElement
	mu                                 sync.Mutex
	open, checking, installing, manual bool
	installable                        bool
	release                            update.Release
	message                            string
	notesScroll                        int
	check, later                       *simpleui.Button
}

const (
	updateNotesFontSize = int32(14)
	updateNotesLineGap  = float32(23)
	updateNotesVisible  = 11
)

var updateNotesBounds = rl.Rectangle{X: 390, Y: 310, Width: 820, Height: 278}

func NewUpdateDialog() *UpdateDialog {
	d := &UpdateDialog{BaseElement: simpleui.NewBaseElement("updateDialog", 0, 0, designWidth, designHeight)}
	d.check = simpleui.NewButton("installUpdate", 535, 714, 245, 45, "ACTUALIZAR", 15)
	d.later = simpleui.NewButton("laterUpdate", 800, 714, 245, 45, "MÁS TARDE", 15)
	d.check.OnClick(d.install)
	d.later.OnClick(func() { d.mu.Lock(); d.open = false; d.mu.Unlock() })
	return d
}
func (d *UpdateDialog) Check(manual bool) {
	d.mu.Lock()
	if d.checking || d.installing {
		d.mu.Unlock()
		return
	}
	d.checking, d.installable, d.manual, d.open, d.message = true, false, manual, manual, "Buscando actualizaciones…"
	d.notesScroll = 0
	d.mu.Unlock()
	go func() {
		release, available, err := update.Check(context.Background())
		d.mu.Lock()
		defer d.mu.Unlock()
		d.checking = false
		d.release = release
		d.installable = available
		if err != nil {
			if d.manual {
				d.open = true
				d.message = "No se ha podido comprobar la actualización: " + err.Error()
			}
			return
		}
		if available {
			d.open = true
			d.message = "Está disponible IC-SDR " + release.Version
		}
		if d.manual && !available {
			d.open = true
			d.message = "IC-SDR ya está actualizado."
		}
	}()
}

func (d *UpdateDialog) install() {
	d.mu.Lock()
	if d.checking || d.installing || !d.installable {
		d.mu.Unlock()
		return
	}
	d.installing = true
	d.message = "Preparando la descarga…"
	release := d.release
	d.mu.Unlock()

	go func() {
		err := update.Prepare(context.Background(), release, func(downloaded, total int64) {
			d.mu.Lock()
			if total > 0 {
				d.message = fmt.Sprintf("Descargando actualización… %.0f %%", float64(downloaded)*100/float64(total))
			} else {
				d.message = fmt.Sprintf("Descargando actualización… %.1f MB", float64(downloaded)/(1024*1024))
			}
			d.mu.Unlock()
		})
		d.mu.Lock()
		if err != nil {
			d.installing = false
			d.message = "No se ha podido preparar la actualización: " + err.Error()
			d.mu.Unlock()
			return
		}
		d.message = "Actualización preparada. Cerrando IC-SDR…"
		d.mu.Unlock()
		simpleui.RequestClose()
	}()
}

func (d *UpdateDialog) OverlayOpen() bool          { d.mu.Lock(); defer d.mu.Unlock(); return d.open }
func (d *UpdateDialog) Update(simpleui.Input) bool { return false }
func (d *UpdateDialog) UpdateOverlay(input simpleui.Input) bool {
	if !d.OverlayOpen() {
		return false
	}
	d.mu.Lock()
	busy := d.checking || d.installing
	d.mu.Unlock()
	if busy {
		return true
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		d.mu.Lock()
		d.open = false
		d.mu.Unlock()
		return true
	}
	d.mu.Lock()
	if d.installable && d.release.Notes != "" {
		lines := wrapUpdateNotes(d.release.Notes, updateNotesBounds.Width-40, updateNotesFontSize)
		maximum := max(0, len(lines)-updateNotesVisible)
		if input.Over(updateNotesBounds) && input.Wheel != 0 {
			d.notesScroll = min(max(d.notesScroll-int(input.Wheel*3), 0), maximum)
		}
		if rl.IsKeyPressed(rl.KeyPageUp) {
			d.notesScroll = max(0, d.notesScroll-updateNotesVisible)
		}
		if rl.IsKeyPressed(rl.KeyPageDown) {
			d.notesScroll = min(maximum, d.notesScroll+updateNotesVisible)
		}
	}
	d.mu.Unlock()
	if d.check.Update(input) || d.later.Update(input) {
		return true
	}
	return true
}
func (d *UpdateDialog) Draw() {}
func (d *UpdateDialog) DrawOverlay() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.open {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 205})
	box := rl.Rectangle{X: 350, Y: 105, Width: 900, Height: 690}
	rl.DrawRectangleRounded(box, .03, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(box, .03, 8, 2, colors.border)
	drawCenteredStyled("ACTUALIZACIONES", rl.Rectangle{X: 390, Y: 137, Width: 820, Height: 35}, 25, simpleui.FontSemiBold, colors.cyan)
	drawCenteredStyled(d.message, rl.Rectangle{X: 390, Y: 196, Width: 820, Height: 45}, 16, simpleui.FontRegular, colors.text)
	if d.installable && d.release.Notes != "" {
		simpleui.DrawTextStyled("NOVEDADES", updateNotesBounds.X, updateNotesBounds.Y-29, 13, simpleui.FontSemiBold, colors.cyan)
		d.drawNotes()
	}
	if d.checking || d.installing || !d.installable {
		d.later.SetLabel("CERRAR")
		d.later.SetBounds(rl.Rectangle{X: 677, Y: 714, Width: 245, Height: 45})
		d.later.Draw()
		return
	}
	d.later.SetLabel("MÁS TARDE")
	d.check.SetBounds(rl.Rectangle{X: 535, Y: 714, Width: 245, Height: 45})
	d.later.SetBounds(rl.Rectangle{X: 800, Y: 714, Width: 245, Height: 45})
	d.check.Draw()
	d.later.Draw()
}

func (d *UpdateDialog) drawNotes() {
	lines := wrapUpdateNotes(d.release.Notes, updateNotesBounds.Width-40, updateNotesFontSize)
	maximum := max(0, len(lines)-updateNotesVisible)
	d.notesScroll = min(max(d.notesScroll, 0), maximum)

	rl.DrawRectangleRounded(updateNotesBounds, .025, 6, colors.panelAlt)
	rl.DrawRectangleRoundedLinesEx(updateNotesBounds, .025, 6, 1, colors.border)
	rl.BeginScissorMode(int32(updateNotesBounds.X), int32(updateNotesBounds.Y), int32(updateNotesBounds.Width), int32(updateNotesBounds.Height))
	for index := d.notesScroll; index < len(lines) && index < d.notesScroll+updateNotesVisible; index++ {
		y := updateNotesBounds.Y + 13 + float32(index-d.notesScroll)*updateNotesLineGap
		simpleui.DrawTextStyled(lines[index], updateNotesBounds.X+16, y, updateNotesFontSize, simpleui.FontRegular, colors.text)
	}
	rl.EndScissorMode()

	if maximum > 0 {
		track := rl.Rectangle{X: updateNotesBounds.X + updateNotesBounds.Width - 11, Y: updateNotesBounds.Y + 10, Width: 3, Height: updateNotesBounds.Height - 20}
		rl.DrawRectangleRounded(track, 1, 4, colors.border)
		thumbHeight := max(float32(30), track.Height*float32(updateNotesVisible)/float32(len(lines)))
		thumbY := track.Y + (track.Height-thumbHeight)*float32(d.notesScroll)/float32(maximum)
		rl.DrawRectangleRounded(rl.Rectangle{X: track.X, Y: thumbY, Width: track.Width, Height: thumbHeight}, 1, 4, colors.cyan)
		simpleui.DrawTextStyled("Rueda o Re Pág/Av Pág para desplazarte", updateNotesBounds.X, updateNotesBounds.Y+updateNotesBounds.Height+9, 11, simpleui.FontRegular, colors.muted)
	}
}

func wrapUpdateNotes(value string, width float32, size int32) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	result := make([]string, 0)
	for _, raw := range strings.Split(value, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		if line == "" {
			if len(result) > 0 && result[len(result)-1] != "" {
				result = append(result, "")
			}
			continue
		}
		prefix := ""
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			prefix, line = "• ", strings.TrimSpace(line[2:])
		}
		words := strings.Fields(line)
		current := prefix
		continuation := strings.Repeat(" ", len([]rune(prefix)))
		for _, word := range words {
			candidate := word
			if strings.TrimSpace(current) != "" {
				candidate = current + " " + word
			} else if current != "" {
				candidate = current + word
			}
			if strings.TrimSpace(current) != "" && simpleui.MeasureTextStyled(candidate, size, simpleui.FontRegular).X > width {
				result = append(result, current)
				current = continuation + word
			} else {
				current = candidate
			}
		}
		if strings.TrimSpace(current) != "" {
			result = append(result, current)
		}
	}
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return result
}
