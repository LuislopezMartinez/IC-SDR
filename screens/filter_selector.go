package screens

import (
	"fmt"
	"math"
	"strconv"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type FilterPreset struct {
	ID, Description string
	BandwidthHz     int
}

var filterCatalog = map[string][]FilterPreset{
	"SSB":      {{"FIL1", "ANCHO", 3000}, {"FIL2", "MEDIO", 2400}, {"FIL3", "ESTRECHO", 1800}, {"CUSTOM", "PERSONALIZADO", 2700}},
	"AM":       {{"FIL1", "ANCHO", 10000}, {"FIL2", "MEDIO", 6000}, {"FIL3", "ESTRECHO", 4000}, {"CUSTOM", "PERSONALIZADO", 7500}},
	"NFM":      {{"FIL1", "ANCHO", 15000}, {"FIL2", "MEDIO", 12500}, {"FIL3", "ESTRECHO", 8500}, {"CUSTOM", "PERSONALIZADO", 11500}},
	"WFM":      {{"FIL1", "ANCHO", 220000}, {"FIL2", "MEDIO", 180000}, {"FIL3", "ESTRECHO", 150000}, {"CUSTOM", "PERSONALIZADO", 200000}},
	"DMR BETA": {{"FIL1", "ANCHO", 15000}, {"FIL2", "DMR 12,5", 12500}, {"FIL3", "ESTRECHO", 10000}, {"CUSTOM", "PERSONALIZADO", 12500}},
	"ADS-B":    {{"FIL1", "AMPLIO", 1800000}, {"FIL2", "COMPLETO", 2000000}, {"FIL3", "REDUCIDO", 1500000}, {"CUSTOM", "PERSONALIZADO", 2000000}},
	"UAT":      {{"FIL1", "AMPLIO", 1800000}, {"FIL2", "COMPLETO", 2000000}, {"FIL3", "REDUCIDO", 1500000}, {"CUSTOM", "PERSONALIZADO", 2000000}},
	"TETRA":    {{"FIL1", "TETRA 25", 25000}, {"FIL2", "MEDIO", 22000}, {"FIL3", "ESTRECHO", 18000}, {"CUSTOM", "PERSONALIZADO", 25000}},
}

type FilterSelector struct {
	simpleui.BaseElement
	open                          bool
	mode                          string
	selected                      map[string]int
	originalIndex, originalCustom int
	custom                        *simpleui.Slider
	apply, cancel                 *simpleui.Button
	onSelect                      func(FilterPreset)
}

func NewFilterSelector(onSelect func(FilterPreset)) *FilterSelector {
	selector := &FilterSelector{BaseElement: simpleui.NewBaseElement("filterSelectorOverlay", 0, 0, designWidth, designHeight), selected: map[string]int{}, onSelect: onSelect}
	for mode := range filterCatalog {
		selector.selected[mode] = 1
	}
	selector.custom = simpleui.NewSlider("filterCustom", 510, 500, 580, 34, 0, 1, .5)
	selector.custom.SetStep(.001)
	selector.custom.OnChange(func(value float32) { selector.setCustomNormalized(value) })
	selector.cancel = simpleui.NewButton("filterCancel", 564, 590, 216, 50, "CANCELAR", 17)
	selector.apply = simpleui.NewButton("filterApply", 820, 590, 216, 50, "APLICAR", 17)
	selector.cancel.OnClick(selector.cancelChanges)
	selector.apply.OnClick(func() { selector.emit(); selector.open = false })
	return selector
}

func filterMode(mode string) string {
	if mode == "USB" || mode == "LSB" || mode == "CW" {
		return "SSB"
	}
	return mode
}
func (selector *FilterSelector) Current(mode string) FilterPreset {
	key := filterMode(mode)
	return filterCatalog[key][selector.selected[key]]
}

// SelectPreset changes the remembered preset for a mode without opening the
// overlay. Tools use it when they require a specific channel bandwidth.
func (selector *FilterSelector) SelectPreset(mode string, index int) FilterPreset {
	key := filterMode(mode)
	presets := filterCatalog[key]
	if len(presets) == 0 {
		return FilterPreset{}
	}
	index = min(max(index, 0), len(presets)-1)
	selector.selected[key] = index
	return presets[index]
}
func (selector *FilterSelector) OpenForMode(mode string) func() {
	return func() { selector.Open(mode) }
}
func (selector *FilterSelector) Open(mode string) {
	selector.mode = filterMode(mode)
	selector.originalIndex = selector.selected[selector.mode]
	selector.originalCustom = filterCatalog[selector.mode][3].BandwidthHz
	selector.open = true
	selector.syncSlider()
}
func (selector *FilterSelector) OverlayOpen() bool          { return selector.open }
func (selector *FilterSelector) Update(simpleui.Input) bool { return false }
func (selector *FilterSelector) Draw()                      {}

func (selector *FilterSelector) UpdateOverlay(input simpleui.Input) bool {
	if !selector.open {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		selector.cancelChanges()
		return true
	}
	if selector.selected[selector.mode] == 3 && selector.custom.Update(input) {
		return true
	}
	if selector.cancel.Update(input) || (selector.selected[selector.mode] == 3 && selector.apply.Update(input)) {
		return true
	}
	if input.Released {
		for index := 0; index < 4; index++ {
			if rl.CheckCollisionPointRec(input.Pointer, selector.presetBounds(index)) {
				selector.selected[selector.mode] = index
				if index == 3 {
					selector.syncSlider()
				} else {
					selector.emit()
					selector.open = false
				}
				return true
			}
		}
	}
	return true
}

func (selector *FilterSelector) DrawOverlay() {
	if !selector.open {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 200})
	panel := rl.Rectangle{X: 410, Y: 150, Width: 780, Height: 530}
	rl.DrawRectangleRounded(panel, .025, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(panel, .025, 8, 2, colors.blue)
	rl.DrawRectangleRounded(rl.Rectangle{X: 410, Y: 150, Width: 10, Height: 530}, .5, 8, colors.blue)
	drawCentered("FILTRO "+selector.mode, rl.Rectangle{X: 450, Y: 180, Width: 700, Height: 45}, 27, colors.text)
	for index, preset := range filterCatalog[selector.mode] {
		bounds := selector.presetBounds(index)
		fill := colors.panelAlt
		if selector.selected[selector.mode] == index {
			fill = colors.blue
		}
		rl.DrawRectangleRounded(bounds, .1, 8, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .1, 8, 2, colors.border)
		name := preset.ID
		if index == 3 {
			name = "FIL 4"
		}
		labelColor := simpleui.EnsureTextContrast(colors.text, fill)
		bandwidthColor := simpleui.EnsureTextContrast(colors.cyan, fill)
		drawCentered(name, rl.Rectangle{X: bounds.X, Y: bounds.Y + 12, Width: bounds.Width, Height: 20}, 17, labelColor)
		drawCentered(preset.Description, rl.Rectangle{X: bounds.X, Y: bounds.Y + 36, Width: bounds.Width, Height: 18}, 11, labelColor)
		drawCentered(formatFilterBandwidth(preset.BandwidthHz), rl.Rectangle{X: bounds.X, Y: bounds.Y + 58, Width: bounds.Width, Height: 18}, 14, bandwidthColor)
	}
	drawCentered("CUSTOM permite ajustar y recordar un ancho para cada modo.", rl.Rectangle{X: 460, Y: 420, Width: 680, Height: 36}, 15, colors.text)
	if selector.selected[selector.mode] == 3 {
		selector.custom.Draw()
		minimum, maximum, _ := customFilterRange(selector.mode)
		drawCentered(fmt.Sprintf("CUSTOM  %s    (%s - %s)", formatFilterBandwidth(filterCatalog[selector.mode][3].BandwidthHz), formatFilterBandwidth(minimum), formatFilterBandwidth(maximum)), rl.Rectangle{X: 510, Y: 540, Width: 580, Height: 30}, 16, colors.text)
		selector.apply.Draw()
	}
	selector.cancel.Draw()
}

func (selector *FilterSelector) presetBounds(index int) rl.Rectangle {
	return rl.Rectangle{X: 440 + float32(index)*180, Y: 260, Width: 164, Height: 100}
}
func (selector *FilterSelector) cancelChanges() {
	filterCatalog[selector.mode][3].BandwidthHz = selector.originalCustom
	selector.selected[selector.mode] = selector.originalIndex
	selector.open = false
}
func (selector *FilterSelector) emit() {
	if selector.onSelect != nil {
		selector.onSelect(selector.Current(selector.mode))
	}
}
func (selector *FilterSelector) syncSlider() {
	minimum, maximum, _ := customFilterRange(selector.mode)
	selector.custom.SetValue(float32(filterCatalog[selector.mode][3].BandwidthHz-minimum) / float32(maximum-minimum))
}
func (selector *FilterSelector) setCustomNormalized(value float32) {
	minimum, maximum, step := customFilterRange(selector.mode)
	raw := float64(minimum) + float64(value)*float64(maximum-minimum)
	bandwidth := minimum + int(math.Round((raw-float64(minimum))/float64(step)))*step
	filterCatalog[selector.mode][3].BandwidthHz = min(max(bandwidth, minimum), maximum)
}
func customFilterRange(mode string) (int, int, int) {
	switch mode {
	case "AM":
		return 2000, 15000, 250
	case "NFM":
		return 500, 25000, 500
	case "WFM":
		return 100000, 300000, 5000
	case "DMR BETA":
		return 8000, 18000, 500
	case "ADS-B", "UAT":
		return 1000000, 2048000, 16000
	default:
		return 300, 5000, 50
	}
}
func formatFilterBandwidth(hz int) string {
	if hz >= 1_000_000 && hz%1_000_000 == 0 {
		return fmt.Sprintf("%d MHz", hz/1_000_000)
	}
	if hz%1000 == 0 {
		return fmt.Sprintf("%d kHz", hz/1000)
	}
	if hz >= 1000 {
		return strconv.FormatFloat(float64(hz)/1000, 'f', -1, 64) + " kHz"
	}
	return fmt.Sprintf("%d Hz", hz)
}
