package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"os/exec"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/simpleui"
)

type RecorderPanel struct {
	simpleui.BaseElement
	screen                                        *MainScreen
	recorder                                      *AudioRecorder
	controls                                      []simpleui.Element
	toolControls                                  []simpleui.Element
	record, pause, folder                         *simpleui.Button
	skip, format                                  *simpleui.Switch
	toolRecord, toolPause, toolFormat, toolFolder *simpleui.Button
	toolSkip                                      *simpleui.Switch
	deleteModal                                   bool
	deleteCandidate                               string
	modalPressed                                  int
}

func NewRecorderPanel(screen *MainScreen, recorder *AudioRecorder) *RecorderPanel {
	p := &RecorderPanel{BaseElement: simpleui.NewBaseElement("recorderDeleteOverlay", 0, 0, designWidth, designHeight), screen: screen, recorder: recorder}
	p.record = simpleui.NewButton("audioRecord", 1300, 445, 132, 36, i18n.Source("text.31d59748e4d1"), uiControlFontSize)
	p.record.SetColors(rl.Color{R: 145, G: 38, B: 42, A: 255}, rl.Color{R: 255, G: 95, B: 95, A: 255}, colors.text)
	p.pause = simpleui.NewButton("audioRecordPause", 1442, 445, 138, 36, i18n.Source("text.0b03bdac33fa"), uiControlFontSize)
	p.skip = simpleui.NewSwitch("audioRecordSkipSQL", 1300, 489, 280, 30, i18n.Source("text.d5a32f2a10ee"), screen.recorderSkipSilence, uiMinimumFontSize)
	p.format = simpleui.NewSwitch("audioRecordFormat", 1300, 525, 122, 32, "MP3", screen.recorderFormat == recorderFormatMP3, 13)
	p.format.SetTrackColors(colors.panelAlt, colors.green)
	p.folder = simpleui.NewButton("audioRecordFolder", 1430, 525, 150, 32, i18n.Source("text.f7aa514861ad"), 13)
	p.toolRecord = simpleui.NewButton("toolAudioRecord", 35, 715, 135, 40, i18n.Source("text.31d59748e4d1"), uiControlFontSize)
	p.toolRecord.SetColors(rl.Color{R: 145, G: 38, B: 42, A: 255}, rl.Color{R: 255, G: 95, B: 95, A: 255}, colors.text)
	p.toolPause = simpleui.NewButton("toolAudioPause", 180, 715, 135, 40, i18n.Source("text.0b03bdac33fa"), uiControlFontSize)
	p.toolSkip = simpleui.NewSwitch("toolAudioSkipSQL", 330, 715, 270, 40, i18n.Source("text.d5a32f2a10ee"), screen.recorderSkipSilence, 13)
	p.toolFormat = simpleui.NewButton("toolAudioFormat", 35, 766, 150, 42, i18n.Source("text.500312539e9c"), 13)
	p.toolFolder = simpleui.NewButton("toolAudioFolder", 195, 766, 405, 42, i18n.Source("text.771a20c38385"), 14)
	startStop := p.ToggleRecording
	togglePause := p.TogglePause
	setSkip := func(value bool) {
		screen.recorderSkipSilence = value
		recorder.SetSkipSquelchSilence(value)
		screen.markSettingsDirty()
		p.refresh()
	}
	openFolder := func() { openExplorerPath(recorder.Directory()) }
	setFormat := func(mp3 bool) {
		if recorder.State().Recording {
			p.refresh()
			return
		}
		screen.recorderFormat = recorderFormatWAV
		if mp3 {
			screen.recorderFormat = recorderFormatMP3
		}
		recorder.SetFormat(screen.recorderFormat)
		screen.markSettingsDirty()
		p.refresh()
	}
	toggleFormat := func() { setFormat(screen.recorderFormat != recorderFormatMP3) }
	p.record.OnClick(startStop)
	p.toolRecord.OnClick(startStop)
	p.pause.OnClick(togglePause)
	p.toolPause.OnClick(togglePause)
	p.skip.OnChange(setSkip)
	p.toolSkip.OnChange(setSkip)
	p.format.OnChange(setFormat)
	p.toolFormat.OnClick(toggleFormat)
	p.folder.OnClick(openFolder)
	p.toolFolder.OnClick(openFolder)
	p.controls = []simpleui.Element{p.record, p.pause, p.skip, p.format, p.folder}
	p.toolControls = []simpleui.Element{p.toolRecord, p.toolPause, p.toolSkip, p.toolFormat, p.toolFolder}
	p.SetToolVisible(false)
	p.refresh()
	return p
}

func (p *RecorderPanel) ToggleRecording() {
	if p.recorder.State().Recording {
		p.recorder.Stop()
	} else {
		p.recorder.Configure(p.screen.frequencyHz, p.screen.bandName, p.screen.mode.SelectedText())
		p.recorder.Start()
	}
	p.refresh()
}

func (p *RecorderPanel) TogglePause() {
	p.recorder.TogglePause()
	p.refresh()
}

func (p *RecorderPanel) ToggleSkipSilence() {
	value := !p.recorder.State().SkipSquelchSilence
	p.screen.recorderSkipSilence = value
	p.recorder.SetSkipSquelchSilence(value)
	p.screen.markSettingsDirty()
	p.refresh()
}

func (p *RecorderPanel) OpenFolder() { openExplorerPath(p.recorder.Directory()) }

func (p *RecorderPanel) refresh() {
	state := p.recorder.State()
	if state.Recording {
		p.record.SetLabel(i18n.Source("text.42a572b1399e"))
		p.toolRecord.SetLabel(i18n.Source("text.42a572b1399e"))
	} else {
		p.record.SetLabel(i18n.Source("text.31d59748e4d1"))
		p.toolRecord.SetLabel(i18n.Source("text.31d59748e4d1"))
	}
	if state.Paused {
		p.pause.SetLabel(i18n.Source("text.fad8cbb4bb22"))
		p.toolPause.SetLabel(i18n.Source("text.fad8cbb4bb22"))
	} else {
		p.pause.SetLabel(i18n.Source("text.0b03bdac33fa"))
		p.toolPause.SetLabel(i18n.Source("text.0b03bdac33fa"))
	}
	p.pause.SetEnabled(state.Recording)
	p.toolPause.SetEnabled(state.Recording)
	p.format.SetEnabled(!state.Recording)
	p.toolFormat.SetEnabled(!state.Recording)
	p.format.SetLabel(state.Format)
	p.format.SetActive(state.Format == recorderFormatMP3)
	p.toolFormat.SetLabel(i18n.Source("text.4034091b82a3") + state.Format)
	p.skip.SetActive(state.SkipSquelchSilence)
	p.toolSkip.SetActive(state.SkipSquelchSilence)
}
func (p *RecorderPanel) SetToolVisible(visible bool) {
	for _, control := range p.toolControls {
		control.SetVisible(visible)
	}
}
func (p *RecorderPanel) Tick() {
	p.recorder.Configure(p.screen.frequencyHz, p.screen.bandName, p.screen.mode.SelectedText())
	p.refresh()
	if p.screen.webServer != nil && p.screen.webServer.RemoteActive() {
		return
	}
	if p.screen.activeTool != i18n.Source("text.e71378482f31") || p.screen.viewMode != 1 || !rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		return
	}
	mouse := simpleui.MousePosition()
	if mouse.X < 650 || mouse.X > 1560 || mouse.Y < 675 || mouse.Y > 815 {
		return
	}
	row := int((mouse.Y - 675) / 27)
	files := p.recorder.State().RecentFiles
	if row >= 0 && row < len(files) && row < 5 {
		if rl.CheckCollisionPointRec(mouse, recorderPlayBounds(row)) {
			simpleui.PlayActivationFeedback()
			openExplorerPath(files[row])
		} else if rl.CheckCollisionPointRec(mouse, recorderDeleteBounds(row)) {
			simpleui.PlayActivationFeedback()
			p.deleteCandidate, p.deleteModal, p.modalPressed = files[row], true, 0
		}
	}
}

func (p *RecorderPanel) Update(simpleui.Input) bool { return false }
func (p *RecorderPanel) Draw()                      {}
func (p *RecorderPanel) OverlayOpen() bool          { return p.deleteModal }
func (p *RecorderPanel) UpdateOverlay(input simpleui.Input) bool {
	if !p.deleteModal {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		p.closeDeleteModal()
		return true
	}
	if input.Pressed {
		p.modalPressed = 0
		if input.Over(p.confirmDeleteBounds()) {
			p.modalPressed = 1
		}
		if input.Over(p.cancelDeleteBounds()) {
			p.modalPressed = 2
		}
	}
	if input.Released {
		if p.modalPressed == 1 && input.Over(p.confirmDeleteBounds()) {
			simpleui.PlayActivationFeedback()
			_ = p.recorder.DeleteFile(p.deleteCandidate)
			p.closeDeleteModal()
		}
		if p.modalPressed == 2 && input.Over(p.cancelDeleteBounds()) {
			simpleui.PlayActivationFeedback()
			p.closeDeleteModal()
		}
		p.modalPressed = 0
	}
	return true
}
func (p *RecorderPanel) DrawOverlay() {
	if !p.deleteModal {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 220})
	modal := rl.Rectangle{X: 470, Y: 300, Width: 660, Height: 250}
	rl.DrawRectangleRounded(modal, .04, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(modal, .04, 8, 2, colors.red)
	drawCentered(i18n.Source("text.8eee8f2e7f12"), rl.Rectangle{X: 490, Y: 325, Width: 620, Height: 30}, 22, colors.red)
	drawCentered(trimMemory(filepath.Base(p.deleteCandidate), 72), rl.Rectangle{X: 505, Y: 380, Width: 590, Height: 28}, 14, colors.text)
	drawCentered(i18n.Source("text.21ae79dcd38b"), rl.Rectangle{X: 505, Y: 420, Width: 590, Height: 24}, uiMinimumFontSize, colors.muted)
	p.drawModalButton(p.cancelDeleteBounds(), i18n.Source("text.b1a5fe65d180"), p.modalPressed == 2, rl.Color{R: 45, G: 55, B: 65, A: 255}, colors.border)
	p.drawModalButton(p.confirmDeleteBounds(), i18n.Source("text.b243b05a8aa4"), p.modalPressed == 1, rl.Color{R: 125, G: 30, B: 35, A: 255}, colors.red)
}
func (p *RecorderPanel) closeDeleteModal() {
	p.deleteModal = false
	p.deleteCandidate = ""
	p.modalPressed = 0
}
func (p *RecorderPanel) cancelDeleteBounds() rl.Rectangle {
	return rl.Rectangle{X: 530, Y: 475, Width: 245, Height: 48}
}
func (p *RecorderPanel) confirmDeleteBounds() rl.Rectangle {
	return rl.Rectangle{X: 825, Y: 475, Width: 245, Height: 48}
}
func (p *RecorderPanel) drawModalButton(bounds rl.Rectangle, label string, pressed bool, background, border rl.Color) {
	if pressed {
		background.R = min(background.R+uint8(25), uint8(255))
		background.G = min(background.G+uint8(25), uint8(255))
		background.B = min(background.B+uint8(25), uint8(255))
	}
	rl.DrawRectangleRounded(bounds, .15, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .15, 6, 2, border)
	drawCentered(label, bounds, uiControlFontSize, simpleui.EnsureTextContrast(colors.text, background))
}

func (p *RecorderPanel) DrawSidebar() {
	state := p.recorder.State()
	drawSidebarSection(1292, 324, 296, 260, i18n.Source("text.e71378482f31"), colors.red)
	drawStatusDot(1560, 340, state.Recording, i18n.Source("text.ac0a7f145139"), i18n.Source("text.c2e3ac47f4a3"))
	simpleui.DrawTextStyled(formatRecordingDuration(state.DurationSeconds), 1372, 354, 25, simpleui.FontMono, colors.text)
	simpleui.DrawTextStyled(recorderFormatDescription(state.Format), 1330, 389, 12, simpleui.FontRegular, colors.muted)
	drawRecorderMeter(1308, 405, 264, 29, state.PeakDBFS, state.Recording && !state.Paused)
	simpleui.DrawTextStyled(i18n.Source("text.94d166731b44"), 1308, 562, 12, simpleui.FontRegular, colors.muted)
	if state.DroppedChunks > 0 {
		simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.0100b243750a"), state.DroppedChunks), 1500, 562, 12, simpleui.FontSemiBold, colors.orange)
	} else {
		simpleui.DrawTextStyled(i18n.Source("text.b57d70039d76"), 1508, 562, 12, simpleui.FontSemiBold, colors.green)
	}
}

func (p *RecorderPanel) DrawPanel() {
	state := p.recorder.State()
	status := i18n.Source("text.d78afe9b19e4")
	dot := colors.muted
	if state.Recording {
		status = i18n.Source("text.edd74dc734e6")
		dot = colors.red
		if state.Paused {
			status = i18n.Source("text.033d2e9017bb")
			dot = colors.orange
		} else if state.WaitingForSquelch {
			status = i18n.Source("text.ec0df1f33fe4")
			dot = colors.blue
		}
	}
	rl.DrawCircle(48, 660, 7, dot)
	simpleui.DrawTextStyled(status, 64, 649, 16, simpleui.FontSemiBold, colors.text)
	simpleui.DrawTextStyled(formatRecordingDuration(state.DurationSeconds), 64, 676, 24, simpleui.FontMono, colors.text)
	drawRecorderMeter(330, 650, 270, 38, state.PeakDBFS, state.Recording && !state.Paused)
	detail := recorderFormatDescription(state.Format) + i18n.Source("text.2786cd13543c")
	if state.Encoding {
		detail = i18n.Source("text.ed5eccdd8f60")
	} else if state.LastError != "" {
		detail = trimMemory(state.LastError, 72)
	}
	simpleui.DrawTextStyled(detail, 270, 694, 12, simpleui.FontRegular, colors.muted)
	drawPanel(650, 638, 910, 182)
	simpleui.DrawTextStyled(i18n.Source("text.d0ac19dbf215"), 665, 650, 14, simpleui.FontSemiBold, colors.text)
	if len(state.RecentFiles) == 0 {
		simpleui.DrawText(i18n.Source("text.d8d2071efc27"), 665, 686, 12, colors.muted)
	}
	for i, file := range state.RecentFiles {
		if i >= 5 {
			break
		}
		yy := float32(680 + i*27)
		if i%2 == 0 {
			rl.DrawRectangle(660, int32(yy-5), 888, 22, rl.Color{R: 18, G: 25, B: 34, A: 255})
		}
		simpleui.DrawTextStyled(trimMemory(filepath.Base(file), 82), 668, yy, 12, simpleui.FontMono, colors.text)
		drawRecorderRowButton(recorderPlayBounds(i), "▶", colors.green)
		drawRecorderRowButton(recorderDeleteBounds(i), i18n.Source("text.9b89497fcb0b"), colors.red)
	}
}

func recorderFormatDescription(format string) string {
	if format == recorderFormatMP3 {
		return i18n.Source("text.0a35a010e6cc")
	}
	return i18n.Source("text.e1856d735af0")
}

func recorderPlayBounds(row int) rl.Rectangle {
	return rl.Rectangle{X: 1454, Y: 675 + float32(row)*27, Width: 38, Height: 23}
}
func recorderDeleteBounds(row int) rl.Rectangle {
	return rl.Rectangle{X: 1500, Y: 675 + float32(row)*27, Width: 48, Height: 23}
}
func drawRecorderRowButton(bounds rl.Rectangle, label string, accent rl.Color) {
	background := rl.Color{R: 25, G: 34, B: 43, A: 255}
	if rl.CheckCollisionPointRec(simpleui.MousePosition(), bounds) {
		background = rl.Color{R: 42, G: 56, B: 68, A: 255}
	}
	rl.DrawRectangleRounded(bounds, .18, 5, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .18, 5, 1, accent)
	drawCentered(label, bounds, 12, accent)
}

func drawRecorderMeter(x, y, w, h, db float32, active bool) {
	drawPanel(x, y, w, h)
	for i, label := range []string{"-60", "-45", "-30", "-15", "0"} {
		px := x + 8 + (w-16)*float32(i)/4
		simpleui.DrawTextStyled(label, px-9, y+19, 12, simpleui.FontMono, colors.muted)
	}
	if !active {
		db = -60
	}
	fraction := min(max((db+60)/60, 0), 1)
	color := colors.cyan
	if db > -12 {
		color = colors.orange
	}
	if db > -3 {
		color = colors.red
	}
	rl.DrawRectangleRounded(rl.Rectangle{X: x + 8, Y: y + 7, Width: (w - 16) * fraction, Height: 5}, 1, 4, color)
}
func formatRecordingDuration(seconds uint64) string {
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, (seconds/60)%60, seconds%60)
}
func openExplorerPath(path string) { _ = exec.Command("explorer.exe", path).Start() }
