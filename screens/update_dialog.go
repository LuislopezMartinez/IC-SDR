package screens

import (
	"context"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/update"
	"go-zero/simpleui"
)

// UpdateDialog checks in the background so startup and radio reception are
// never delayed by the network.
type UpdateDialog struct {
	simpleui.BaseElement
	mu                     sync.Mutex
	open, checking, manual bool
	release                update.Release
	message                string
	check, later           *simpleui.Button
}

func NewUpdateDialog() *UpdateDialog {
	d := &UpdateDialog{BaseElement: simpleui.NewBaseElement("updateDialog", 0, 0, designWidth, designHeight)}
	d.check = simpleui.NewButton("installUpdate", 545, 545, 245, 45, "ACTUALIZAR", 15)
	d.later = simpleui.NewButton("laterUpdate", 810, 545, 245, 45, "MÁS TARDE", 15)
	d.check.OnClick(func() { d.Check(true) })
	d.later.OnClick(func() { d.mu.Lock(); d.open = false; d.mu.Unlock() })
	return d
}
func (d *UpdateDialog) Check(manual bool) {
	d.mu.Lock()
	if d.checking {
		d.mu.Unlock()
		return
	}
	d.checking, d.manual, d.open, d.message = true, manual, manual, "Buscando actualizaciones…"
	d.mu.Unlock()
	go func() {
		release, available, err := update.Check(context.Background())
		d.mu.Lock()
		defer d.mu.Unlock()
		d.checking = false
		d.release = release
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
func (d *UpdateDialog) OverlayOpen() bool          { d.mu.Lock(); defer d.mu.Unlock(); return d.open }
func (d *UpdateDialog) Update(simpleui.Input) bool { return false }
func (d *UpdateDialog) UpdateOverlay(input simpleui.Input) bool {
	if !d.OverlayOpen() {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		d.mu.Lock()
		d.open = false
		d.mu.Unlock()
		return true
	}
	if d.checking {
		return true
	}
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
	box := rl.Rectangle{X: 450, Y: 300, Width: 700, Height: 320}
	rl.DrawRectangleRounded(box, .03, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(box, .03, 8, 2, colors.border)
	drawCenteredStyled("ACTUALIZACIONES", rl.Rectangle{X: 480, Y: 332, Width: 640, Height: 35}, 25, simpleui.FontSemiBold, colors.cyan)
	drawCenteredStyled(d.message, rl.Rectangle{X: 490, Y: 400, Width: 620, Height: 45}, 16, simpleui.FontRegular, colors.text)
	if d.release.Notes != "" {
		drawCenteredStyled("Consulta las notas de la versión antes de instalar.", rl.Rectangle{X: 490, Y: 465, Width: 620, Height: 30}, 13, simpleui.FontRegular, colors.muted)
	}
	if d.checking || d.release.PackageURL == "" {
		d.later.SetLabel("CERRAR")
		d.later.SetBounds(rl.Rectangle{X: 677, Y: 545, Width: 245, Height: 45})
		d.later.Draw()
		return
	}
	d.later.SetLabel("MÁS TARDE")
	d.later.SetBounds(rl.Rectangle{X: 810, Y: 545, Width: 245, Height: 45})
	d.check.Draw()
	d.later.Draw()
}
