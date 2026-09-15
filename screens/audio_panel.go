package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"math"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type AudioPanel struct {
	screen                          *MainScreen
	controls                        []simpleui.Element
	lowCut, highCut                 int
	eqEnabled                       bool
	eqGains                         [5]float32
	profile                         string
	deemphasisUs                    int
	pbtLow, pbtHigh                 int
	pbtLocked, pbtBypassed          bool
	nextSpectrum                    float64
	spectrum                        [96]float32
	eqSwitch                        *simpleui.Switch
	eqSliders                       [5]*simpleui.Slider
	cutoffs                         *simpleui.RangeSlider
	profileButton, deemphasisButton *simpleui.Button
	pbtRange                        *simpleui.RangeSlider
	pbtLock                         *simpleui.Switch
	pbtClear, pbtBypass             *simpleui.Button
	pbtDrag, audioDrag              int
	dragStartX                      float32
	dragStartLow, dragStartHigh     int
}

func NewAudioPanel(screen *MainScreen) *AudioPanel {
	p := &AudioPanel{screen: screen, lowCut: 100, highCut: 4000, eqEnabled: true, profile: i18n.Source("text.db2cb3fe28e2"), deemphasisUs: 50, pbtLow: 100, pbtHigh: 3250}
	for i := range p.spectrum {
		p.spectrum[i] = -80
	}
	p.pbtRange = simpleui.NewRangeSlider("pbtRange", 45, 710, 350, 20, 50, 5000, 100, 3250)
	p.pbtRange.SetStep(10)
	p.pbtRange.SetMinimumGap(200)
	p.pbtRange.SetRangeDragging(false)
	p.pbtRange.OnChange(func(low, high float32) { p.pbtLow, p.pbtHigh = int(low), int(high); p.applyPBT() })
	p.pbtLock = simpleui.NewSwitch("pbtLock", 42, 782, 104, 26, i18n.Source("text.74c4812d040a"), false, 11)
	p.pbtLock.OnChange(func(active bool) {
		p.pbtLocked = active
		if active {
			p.pbtLock.SetLabel(i18n.Source("text.fd1a9a18ddbb"))
		} else {
			p.pbtLock.SetLabel(i18n.Source("text.216e7fea416a"))
		}
	})
	p.pbtClear = simpleui.NewButton("pbtClear", 154, 782, 92, 26, i18n.Source("text.9cc3a043b6a9"), 11)
	p.pbtClear.OnClick(func() {
		p.pbtLow, p.pbtHigh = 100, min(max(p.screen.demodBandwidthHz, 300), 5000)
		p.applyPBT()
	})
	p.pbtBypass = simpleui.NewButton("pbtBypass", 254, 782, 118, 26, i18n.Source("text.7f84c8c9be1e"), 11)
	p.pbtBypass.OnClick(func() {
		p.pbtBypassed = !p.pbtBypassed
		if p.pbtBypassed {
			p.pbtBypass.SetLabel(i18n.Source("text.e68c10ac725c"))
		} else {
			p.pbtBypass.SetLabel(i18n.Source("text.16e473bccb0b"))
		}
		p.applyPBT()
	})

	p.eqSwitch = simpleui.NewSwitch("audioEQ", 676, 649, 66, 24, "EQ", true, 11)
	p.eqSwitch.OnChange(func(active bool) {
		p.eqEnabled = active
		p.eqSwitch.SetLabel(map[bool]string{true: i18n.Source("text.f82743605b47"), false: i18n.Source("text.b9958b5b0d93")}[active])
		p.apply()
	})
	flat := simpleui.NewButton("audioEQFlat", 744, 649, 48, 24, i18n.Source("text.988ca3f92f1a"), 10)
	flat.OnClick(func() {
		for i := range p.eqGains {
			p.eqGains[i] = 0
			p.eqSliders[i].SetValue(0)
		}
		p.apply()
	})
	for i := range p.eqSliders {
		x := float32(506 + i*53)
		slider := simpleui.NewSlider(fmt.Sprintf("audioEQ%d", i), x, 702, 16, 68, -12, 12, 0)
		slider.SetOrientation(simpleui.Vertical)
		slider.SetStep(.5)
		band := i
		slider.OnChange(func(value float32) { p.eqGains[band] = value; p.apply() })
		p.eqSliders[i] = slider
	}
	p.cutoffs = simpleui.NewRangeSlider("audioCutoffs", 700, 770, 390, 20, 0, 12000, 100, 4000)
	p.cutoffs.SetStep(10)
	p.cutoffs.SetMinimumGap(200)
	p.cutoffs.SetRangeDragging(false)
	p.cutoffs.OnChange(func(low, high float32) { p.lowCut, p.highCut = max(int(low), 20), min(int(high), 16000); p.apply() })
	p.profileButton = simpleui.NewButton("audioProfile", 818, 778, 176, 30, i18n.Source("text.6562bc0daf89"), 11)
	p.profileButton.OnClick(p.cycleProfile)
	p.deemphasisButton = simpleui.NewButton("audioDeemphasis", 1004, 778, 156, 30, i18n.Source("text.bd8a9dc191f1"), 11)
	p.deemphasisButton.OnClick(func() {
		if p.deemphasisUs == 50 {
			p.deemphasisUs = 75
		} else {
			p.deemphasisUs = 50
		}
		p.apply()
	})
	reset := simpleui.NewButton("audioReset", 1458, 648, 94, 25, i18n.Source("text.7ef2fad58d1f"), 10)
	reset.OnClick(func() { p.lowCut, p.highCut = 100, 4000; p.cutoffs.SetValues(100, 4000); p.apply() })
	p.controls = []simpleui.Element{p.pbtLock, p.pbtClear, p.pbtBypass, p.eqSwitch, flat, p.profileButton, p.deemphasisButton, reset}
	for _, slider := range p.eqSliders {
		p.controls = append(p.controls, slider)
	}
	p.SetVisible(false)
	p.apply()
	p.applyPBT()
	return p
}

func (p *AudioPanel) applyPBT() {
	if p.screen.receiver != nil {
		p.screen.receiver.SetTwinPBT(p.pbtLow, p.pbtHigh, p.pbtBypassed)
	}
}

func (p *AudioPanel) SetVisible(visible bool) {
	for _, c := range p.controls {
		c.SetVisible(visible)
	}
}
func (p *AudioPanel) UpdateSpectrum() {
	p.handleGraphInput()
	if p.screen.audioPlayer != nil && rl.GetTime() >= p.nextSpectrum {
		p.screen.audioPlayer.Spectrum(p.spectrum[:])
		p.nextSpectrum = rl.GetTime() + .05
	}
}

func (p *AudioPanel) apply() {
	if p.screen.audioPlayer != nil {
		p.screen.audioPlayer.ConfigureProcessing(p.lowCut, p.highCut, p.eqEnabled, p.eqGains, p.profile)
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetFMDeemphasis(p.deemphasisUs)
	}
	p.profileButton.SetLabel(i18n.Source("text.373f005e130f") + p.profile)
	p.deemphasisButton.SetLabel(fmt.Sprintf(i18n.Source("text.427ce15ba57b"), p.deemphasisUs))
}

func (p *AudioPanel) cycleProfile() {
	switch p.profile {
	case "SUAVE":
		p.profile = i18n.Source("text.db2cb3fe28e2")
	case "NORMAL":
		p.profile = i18n.Source("text.e5ccf011d642")
	default:
		p.profile = i18n.Source("text.692233b9c713")
	}
	p.apply()
}

func (p *AudioPanel) DrawPanel() {
	mode := p.screen.mode.SelectedText()
	ssb := mode == i18n.Source("text.61f0acff1735") || mode == i18n.Source("text.6323db4948ad")
	p.pbtLock.SetEnabled(ssb && !p.pbtBypassed)
	p.pbtClear.SetEnabled(ssb)
	p.pbtBypass.SetEnabled(ssb)
	drawCentered(i18n.Source("text.cdc6bff8f509"), rl.Rectangle{X: 30, Y: 634, Width: 410, Height: 24}, 13, colors.text)
	drawPanel(470, 642, 326, 174)
	drawPanel(808, 642, 756, 174)
	drawSmallText(i18n.Source("text.ab5f31a11801"), 482, 650, colors.text)
	drawSmallText(i18n.Source("text.08701518bfdd"), 818, 650, colors.text)
	p.drawPBT(ssb, mode)
	for i, hz := range audioEQFrequencies {
		x := float32(514 + i*53)
		drawSmallText(fmt.Sprintf("%+.1f", p.eqGains[i]), x-10, 684, colors.cyan)
		label := fmt.Sprintf("%.0f", hz)
		if hz >= 1000 {
			label = fmt.Sprintf("%.1fk", hz/1000)
		}
		drawSmallText(label, x-10, 782, colors.muted)
	}
	drawSmallText("+12", 476, 704, colors.muted)
	drawSmallText("0", 480, 733, colors.muted)
	drawSmallText("-12", 476, 762, colors.muted)
	p.drawSpectrum(818, 690, 736, 74, 12000)
	drawSmallText(fmt.Sprintf(i18n.Source("text.148eb8b51395"), formatAudioHz(p.lowCut)), 822, 670, colors.cyan)
	drawSmallText(fmt.Sprintf(i18n.Source("text.d96b30c0b24d"), formatAudioHz(p.highCut)), 952, 670, colors.orange)
	drawSmallText(fmt.Sprintf(i18n.Source("text.68c6f1d43d10"), p.screen.stats.AudioBuffered*1000/audioSampleRate, float32(p.screen.stats.AudioBuffered)*100/48000), 1045, 650, colors.muted)
	drawSmallText(fmt.Sprintf(i18n.Source("text.0429bae138cb"), p.screen.stats.AudioOverruns), 1260, 650, colors.muted)
	drawSmallText(fmt.Sprintf(i18n.Source("text.1197ea4c2f46"), formatAudioHz(p.highCut-p.lowCut)), 1422, 786, colors.text)
}

func (p *AudioPanel) drawPBT(enabled bool, mode string) {
	x, y, w, h := float32(42), float32(670), float32(390), float32(80)
	drawPanel(x, y, w, h)
	drawGrid(x, y, w, h, 8, 4)
	center := x + w/2
	rl.DrawLineEx(rl.Vector2{X: center, Y: y}, rl.Vector2{X: center, Y: y + h}, 1.5, colors.green)
	if !enabled {
		drawCentered(i18n.Source("text.85c3be5255f8"), rl.Rectangle{X: x, Y: y, Width: w, Height: h}, 10, colors.muted)
		return
	}
	sign := float32(1)
	if mode == i18n.Source("text.6323db4948ad") {
		sign = -1
	}
	lowX := center + sign*w*.5*float32(p.pbtLow)/5000
	highX := center + sign*w*.5*float32(p.pbtHigh)/5000
	left, right := min(lowX, highX), max(lowX, highX)
	rl.DrawRectangleRec(rl.Rectangle{X: left, Y: y + 12, Width: right - left, Height: h - 12}, rl.Color{R: 25, G: 115, B: 220, A: 100})
	p.drawPBTResponse(x, y, w, h, mode, 100, min(max(p.screen.demodBandwidthHz, 300), 5000), rl.Color{R: 125, G: 130, B: 140, A: 255})
	p.drawPBTResponse(x, y, w, h, mode, p.pbtLow, 5000, colors.cyan)
	p.drawPBTResponse(x, y, w, h, mode, 50, p.pbtHigh, colors.orange)
	rl.DrawLineEx(rl.Vector2{X: lowX, Y: y}, rl.Vector2{X: lowX, Y: y + h}, 2, colors.cyan)
	rl.DrawLineEx(rl.Vector2{X: highX, Y: y}, rl.Vector2{X: highX, Y: y + h}, 2, colors.orange)
	drawAudioDragHandle(lowX, y+8, colors.cyan)
	drawAudioDragHandle(highX, y+8, colors.orange)
	drawSmallText(fmt.Sprintf(i18n.Source("text.6d2a1f73e4b4"), p.pbtLow), 208, 650, colors.cyan)
	drawSmallText(fmt.Sprintf(i18n.Source("text.08ff963b2962"), p.pbtHigh), 330, 650, colors.orange)
	drawSmallText(i18n.Source("text.5b915f321d92"), x, y+h+2, colors.muted)
	drawSmallText("0", center-3, y+h+2, colors.muted)
	drawSmallText(i18n.Source("text.c1c18594305f"), x+w-34, y+h+2, colors.muted)
}

func (p *AudioPanel) drawPBTResponse(x, y, w, h float32, mode string, low, high int, color rl.Color) {
	response := func(audioHz float32) float32 {
		roll := max(float32(80), min(float32(260), float32(high-low)*.16))
		rise := min(max((audioHz-(float32(low)-roll))/roll, 0), 1)
		fall := min(max(((float32(high)+roll)-audioHz)/roll, 0), 1)
		return min(rise, fall)
	}
	var previous rl.Vector2
	for i := 0; i <= 160; i++ {
		rf := -5000 + 10000*float32(i)/160
		audio := rf
		if mode == i18n.Source("text.6323db4948ad") {
			audio = -rf
		}
		current := rl.Vector2{X: x + w*float32(i)/160, Y: y + h - response(audio)*(h-13)}
		if i > 0 {
			rl.DrawLineEx(previous, current, 2, color)
		}
		previous = current
	}
}

func (p *AudioPanel) handleGraphInput() {
	if p.screen.activeTool != i18n.Source("text.a42c60257b01") || p.screen.viewMode != 1 || p.screen.overlayOpen() {
		p.pbtDrag, p.audioDrag = 0, 0
		return
	}
	mouse := simpleui.MousePosition()
	// The graphs are drawn through drawCompactedTool's horizontal transform;
	// hit testing must use the same legacy coordinates as their drawing code.
	mouse.X = legacyToolPointerX(mouse.X)
	pressed := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	down := rl.IsMouseButtonDown(rl.MouseButtonLeft)
	released := rl.IsMouseButtonReleased(rl.MouseButtonLeft)
	mode := ""
	if p.screen.mode != nil {
		mode = p.screen.mode.SelectedText()
	}
	ssb := mode == i18n.Source("text.61f0acff1735") || mode == i18n.Source("text.6323db4948ad")
	if pressed && ssb && !p.pbtBypassed && mouse.X >= 42 && mouse.X <= 432 && mouse.Y >= 650 && mouse.Y <= 750 {
		center := float32(237)
		sign := float32(1)
		if mode == i18n.Source("text.6323db4948ad") {
			sign = -1
		}
		lowX := center + sign*195*float32(p.pbtLow)/5000
		highX := center + sign*195*float32(p.pbtHigh)/5000
		if float32(math.Abs(float64(mouse.X-lowX))) <= 18 || float32(math.Abs(float64(mouse.X-highX))) <= 18 {
			if math.Abs(float64(mouse.X-lowX)) <= math.Abs(float64(mouse.X-highX)) {
				p.pbtDrag = 1
			} else {
				p.pbtDrag = 2
			}
			p.dragStartX, p.dragStartLow, p.dragStartHigh = mouse.X, p.pbtLow, p.pbtHigh
		}
	}
	if p.pbtDrag != 0 && down {
		if p.pbtLocked {
			sign := float32(1)
			if mode == i18n.Source("text.6323db4948ad") {
				sign = -1
			}
			delta := int(math.Round(float64((mouse.X-p.dragStartX)*5000/195*sign/10))) * 10
			width := p.dragStartHigh - p.dragStartLow
			p.pbtLow = min(max(p.dragStartLow+delta, 50), 5000-width)
			p.pbtHigh = p.pbtLow + width
		} else {
			value := int(math.Round(math.Abs(float64(mouse.X-237))*5000/195/10)) * 10
			value = min(max(value, 50), 5000)
			if p.pbtDrag == 1 {
				p.pbtLow = min(value, p.pbtHigh-200)
			} else {
				p.pbtHigh = max(value, p.pbtLow+200)
			}
		}
		p.applyPBT()
	}
	if pressed && mouse.X >= 818 && mouse.X <= 1554 && mouse.Y >= 684 && mouse.Y <= 764 {
		lowX := 818 + 736*float32(p.lowCut)/12000
		highX := 818 + 736*float32(p.highCut)/12000
		if math.Abs(float64(mouse.X-lowX)) <= 16 {
			p.audioDrag = 1
		} else if math.Abs(float64(mouse.X-highX)) <= 16 {
			p.audioDrag = 2
		}
	}
	if p.audioDrag != 0 && down {
		value := int(math.Round(float64((mouse.X-818)*12000/736/10))) * 10
		value = min(max(value, 20), 12000)
		if p.audioDrag == 1 {
			p.lowCut = min(value, p.highCut-200)
		} else {
			p.highCut = max(value, p.lowCut+200)
		}
		p.apply()
	}
	if released {
		p.pbtDrag, p.audioDrag = 0, 0
	}
}

func (p *AudioPanel) drawSpectrum(x, y, w, h float32, maximumHz int) {
	drawPanel(x, y, w, h)
	drawGrid(x, y, w, h, 4, 4)
	lowX := x + w*float32(p.lowCut)/float32(maximumHz)
	highX := x + w*float32(p.highCut)/float32(maximumHz)
	rl.DrawRectangleRec(rl.Rectangle{X: lowX, Y: y, Width: max(highX-lowX, 0), Height: h}, rl.Color{R: 20, G: 125, B: 190, A: 48})
	count := min(len(p.spectrum), int(math.Ceil(float64(len(p.spectrum)-1)*float64(maximumHz)/16000))+1)
	var previous rl.Vector2
	for i := 0; i < count; i++ {
		px := x + w*float32(i)/float32(count-1)
		py := y + h - (min(max(p.spectrum[i], -80), 0)+80)/80*h
		current := rl.Vector2{X: px, Y: py}
		if i > 0 {
			rl.DrawLineEx(previous, current, 1.4, colors.cyan)
		}
		previous = current
	}
	rl.DrawLineEx(rl.Vector2{X: lowX, Y: y}, rl.Vector2{X: lowX, Y: y + h}, 1.5, colors.cyan)
	rl.DrawLineEx(rl.Vector2{X: highX, Y: y}, rl.Vector2{X: highX, Y: y + h}, 1.5, colors.orange)
	drawAudioDragHandle(lowX, y+8, colors.cyan)
	drawAudioDragHandle(highX, y+8, colors.orange)
}

func legacyToolPointerX(x float32) float32 {
	return legacyToolX + (x-toolContentX)/toolContentScaleX
}

func drawAudioDragHandle(x, y float32, accent rl.Color) {
	const width, height = float32(16), float32(20)
	bounds := rl.Rectangle{X: x - width/2, Y: y, Width: width, Height: height}
	rl.DrawRectangleRounded(bounds, .3, 4, colors.panelAlt)
	rl.DrawRectangleRoundedLinesEx(bounds, .3, 4, 1.5, accent)
	for _, offset := range []float32{7, 12} {
		rl.DrawLineEx(rl.Vector2{X: x - 3, Y: y + offset}, rl.Vector2{X: x + 3, Y: y + offset}, 1.5, accent)
	}
}

func formatAudioHz(hz int) string {
	if hz >= 1000 {
		return fmt.Sprintf(i18n.Source("text.c3e70ea0286a"), float64(hz)/1000)
	}
	return fmt.Sprintf(i18n.Source("text.3999a0ad05ce"), hz)
}
