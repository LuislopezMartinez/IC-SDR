package screens

import (
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// RemoteLockOverlay is registered last so it intercepts all ordinary desktop
// controls while one mobile browser owns the remote-control lease.
type RemoteLockOverlay struct {
	simpleui.BaseElement
	screen *MainScreen
}

func NewRemoteLockOverlay(screen *MainScreen) *RemoteLockOverlay {
	return &RemoteLockOverlay{BaseElement: simpleui.NewBaseElement("remoteLockOverlay", 0, 0, designWidth, designHeight), screen: screen}
}

func (overlay *RemoteLockOverlay) OverlayOpen() bool {
	return overlay.screen != nil && overlay.screen.webServer != nil && overlay.screen.webServer.RemoteActive()
}
func (overlay *RemoteLockOverlay) Update(simpleui.Input) bool        { return false }
func (overlay *RemoteLockOverlay) Draw()                             {}
func (overlay *RemoteLockOverlay) UpdateOverlay(simpleui.Input) bool { return true }

func (overlay *RemoteLockOverlay) DrawOverlay() {
	if !overlay.OverlayOpen() {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{R: 5, G: 17, B: 29, A: 46})
	box := rl.Rectangle{X: 585, Y: 210, Width: 430, Height: 51}
	rl.DrawRectangleRounded(box, .18, 8, rl.Color{R: 13, G: 47, B: 69, A: 245})
	rl.DrawRectangleRoundedLinesEx(box, .18, 8, 2, colors.orange)
	drawCenteredStyled("CONEXIÓN REMOTA · PC BLOQUEADO", box, 18, simpleui.FontSemiBold, rl.White)
}
