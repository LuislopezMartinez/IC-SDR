package screens

import (
	"go-zero/internal/i18n"

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
	i18n.Source("text.3b84edd06b03"): {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.32c5fd24567a"), 3000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.e3b19dc5779f"), 2400}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.754a0dc3b606"), 1800}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 2700}},
	"AM":                             {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.32c5fd24567a"), 10000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.e3b19dc5779f"), 6000}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.754a0dc3b606"), 4000}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 7500}},
	i18n.Source("text.0896d612d497"): {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.32c5fd24567a"), 15000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.e3b19dc5779f"), 12500}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.754a0dc3b606"), 8500}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 11500}},
	i18n.Source("text.6b742bac3eb4"): {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.32c5fd24567a"), 220000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.e3b19dc5779f"), 180000}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.754a0dc3b606"), 150000}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 200000}},
	i18n.Source("text.2604864ce4d3"): {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.32c5fd24567a"), 15000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.e630ba82d84a"), 12500}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.754a0dc3b606"), 10000}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 12500}},
	i18n.Source("text.7866f9f32e66"): {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.fc09f45c8252"), 1800000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.780cd5dd50e5"), 2000000}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.22c9628a35db"), 1500000}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 2000000}},
	i18n.Source("text.72c048cb5100"): {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.fc09f45c8252"), 1800000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.780cd5dd50e5"), 2000000}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.22c9628a35db"), 1500000}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 2000000}},
	i18n.Source("text.f69d86a86926"): {{i18n.Source("text.c4006a8004b9"), i18n.Source("text.a334ab1b51eb"), 25000}, {i18n.Source("text.eaef6ac938cc"), i18n.Source("text.e3b19dc5779f"), 22000}, {i18n.Source("text.5e6f400c402d"), i18n.Source("text.754a0dc3b606"), 18000}, {i18n.Source("text.7cd5885327fd"), i18n.Source("text.4fd9b0cb33bf"), 25000}},
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
	selector.cancel = simpleui.NewButton("filterCancel", 564, 590, 216, 50, i18n.Source("text.b1a5fe65d180"), 17)
	selector.apply = simpleui.NewButton("filterApply", 820, 590, 216, 50, i18n.Source("text.3db550736ade"), 17)
	selector.cancel.OnClick(selector.cancelChanges)
	selector.apply.OnClick(func() { selector.emit(); selector.open = false })
	return selector
}

func filterMode(mode string) string {
	if mode == i18n.Source("text.61f0acff1735") || mode == i18n.Source("text.6323db4948ad") || mode == "CW" {
		return i18n.Source("text.3b84edd06b03")
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
	drawCentered(i18n.Source("text.f69b30034a3f")+selector.mode, rl.Rectangle{X: 450, Y: 180, Width: 700, Height: 45}, 27, colors.text)
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
			name = i18n.Source("text.3eab586ddfc8")
		}
		labelColor := simpleui.EnsureTextContrast(colors.text, fill)
		bandwidthColor := simpleui.EnsureTextContrast(colors.cyan, fill)
		drawCentered(name, rl.Rectangle{X: bounds.X, Y: bounds.Y + 12, Width: bounds.Width, Height: 20}, 17, labelColor)
		drawCentered(preset.Description, rl.Rectangle{X: bounds.X, Y: bounds.Y + 36, Width: bounds.Width, Height: 18}, 11, labelColor)
		drawCentered(formatFilterBandwidth(preset.BandwidthHz), rl.Rectangle{X: bounds.X, Y: bounds.Y + 58, Width: bounds.Width, Height: 18}, 14, bandwidthColor)
	}
	drawCentered(i18n.Source("text.f285438f7395"), rl.Rectangle{X: 460, Y: 420, Width: 680, Height: 36}, 15, colors.text)
	if selector.selected[selector.mode] == 3 {
		selector.custom.Draw()
		minimum, maximum, _ := customFilterRange(selector.mode)
		drawCentered(fmt.Sprintf(i18n.Source("text.98f9e875bcdc"), formatFilterBandwidth(filterCatalog[selector.mode][3].BandwidthHz), formatFilterBandwidth(minimum), formatFilterBandwidth(maximum)), rl.Rectangle{X: 510, Y: 540, Width: 580, Height: 30}, 16, colors.text)
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
		return fmt.Sprintf(i18n.Source("text.b350b6c0de6c"), hz/1_000_000)
	}
	if hz%1000 == 0 {
		return fmt.Sprintf(i18n.Source("text.a47fa5c4f793"), hz/1000)
	}
	if hz >= 1000 {
		return strconv.FormatFloat(float64(hz)/1000, 'f', -1, 64) + i18n.Source("text.acaf5a32d70a")
	}
	return fmt.Sprintf(i18n.Source("text.3999a0ad05ce"), hz)
}
