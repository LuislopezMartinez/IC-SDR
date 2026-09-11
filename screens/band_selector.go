package screens

import (
	"fmt"
	"strings"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type BandDefinition struct {
	Category    string
	Name        string
	FrequencyHz int64
	SpanHz      int64
}

var bandsByCategory = map[string][]BandDefinition{
	"HAM": {
		{Category: "HAM", Name: "160 m", FrequencyHz: 1_900_000, SpanHz: 25_000},
		{Category: "HAM", Name: "80 m", FrequencyHz: 3_650_000, SpanHz: 50_000},
		{Category: "HAM", Name: "60 m", FrequencyHz: 5_354_000, SpanHz: 25_000},
		{Category: "HAM", Name: "40 m", FrequencyHz: 7_100_000, SpanHz: 50_000},
		{Category: "HAM", Name: "30 m", FrequencyHz: 10_125_000, SpanHz: 25_000},
		{Category: "HAM", Name: "20 m", FrequencyHz: 14_200_000, SpanHz: 100_000},
		{Category: "HAM", Name: "17 m", FrequencyHz: 18_100_000, SpanHz: 50_000},
		{Category: "HAM", Name: "15 m", FrequencyHz: 21_200_000, SpanHz: 100_000},
		{Category: "HAM", Name: "12 m", FrequencyHz: 24_950_000, SpanHz: 50_000},
		{Category: "HAM", Name: "10 m", FrequencyHz: 28_500_000, SpanHz: 200_000},
		{Category: "HAM", Name: "6 m", FrequencyHz: 50_150_000, SpanHz: 200_000},
		{Category: "HAM", Name: "4 m", FrequencyHz: 70_200_000, SpanHz: 200_000},
		{Category: "HAM", Name: "2 m", FrequencyHz: 145_000_000, SpanHz: 500_000},
		{Category: "HAM", Name: "70 cm", FrequencyHz: 433_500_000, SpanHz: 500_000},
		{Category: "HAM", Name: "23 cm", FrequencyHz: 1_296_000_000, SpanHz: 1_000_000},
	},
	"COMMERCIAL": {
		{Category: "COMMERCIAL", Name: "LW", FrequencyHz: 198_000, SpanHz: 100_000},
		{Category: "COMMERCIAL", Name: "MW / AM", FrequencyHz: 1_000_000, SpanHz: 500_000},
		{Category: "COMMERCIAL", Name: "SW 49 m", FrequencyHz: 6_100_000, SpanHz: 200_000},
		{Category: "COMMERCIAL", Name: "SW 41 m", FrequencyHz: 7_300_000, SpanHz: 200_000},
		{Category: "COMMERCIAL", Name: "SW 31 m", FrequencyHz: 9_600_000, SpanHz: 200_000},
		{Category: "COMMERCIAL", Name: "FM", FrequencyHz: 100_000_000, SpanHz: 2_000_000},
		{Category: "COMMERCIAL", Name: "AIR", FrequencyHz: 125_000_000, SpanHz: 1_000_000},
		{Category: "COMMERCIAL", Name: "MARINE", FrequencyHz: 156_800_000, SpanHz: 1_000_000},
		{Category: "COMMERCIAL", Name: "MARINE AIS", FrequencyHz: 162_000_000, SpanHz: 250_000},
		{Category: "COMMERCIAL", Name: "DAB", FrequencyHz: 220_352_000, SpanHz: 2_000_000},
		{Category: "COMMERCIAL", Name: "RADIOSONDES", FrequencyHz: 403_000_000, SpanHz: 250_000},
		{Category: "COMMERCIAL", Name: "UAT 978", FrequencyHz: 978_000_000, SpanHz: 2_000_000},
		{Category: "COMMERCIAL", Name: "ADS-B 1090", FrequencyHz: 1_090_000_000, SpanHz: 2_000_000},
	},
	"ISM": {
		{Category: "ISM", Name: "CB 27", FrequencyHz: 27_205_000, SpanHz: 500_000},
		{Category: "ISM", Name: "PMR446", FrequencyHz: 446_006_250, SpanHz: 500_000},
		{Category: "ISM", Name: "433 MHz", FrequencyHz: 433_920_000, SpanHz: 1_000_000},
		{Category: "ISM", Name: "868 MHz", FrequencyHz: 868_300_000, SpanHz: 2_000_000},
		{Category: "ISM", Name: "915 MHz", FrequencyHz: 915_000_000, SpanHz: 2_000_000},
		{Category: "ISM", Name: "2.4 GHz", FrequencyHz: 2_440_000_000, SpanHz: 2_000_000},
	},
}

type BandSelector struct {
	simpleui.BaseElement
	open             bool
	category         string
	selectedCategory string
	selectedName     string
	pressedCategory  int
	pressedBand      int
	pressedCancel    bool
	onSelect         func(BandDefinition)
}

func NewBandSelector(category, name string, onSelect func(BandDefinition)) *BandSelector {
	if _, exists := bandsByCategory[category]; !exists {
		category = "HAM"
	}
	return &BandSelector{
		BaseElement: simpleui.NewBaseElement("bandSelectorOverlay", 0, 0, designWidth, designHeight),
		category:    category, selectedCategory: category, selectedName: name,
		pressedCategory: -1, pressedBand: -1, onSelect: onSelect,
	}
}

func (selector *BandSelector) Open() {
	selector.category = selector.selectedCategory
	selector.open = true
}

func (selector *BandSelector) Close() {
	selector.open = false
	selector.pressedCategory = -1
	selector.pressedBand = -1
	selector.pressedCancel = false
}

func (selector *BandSelector) OverlayOpen() bool          { return selector.open }
func (selector *BandSelector) Update(simpleui.Input) bool { return false }
func (selector *BandSelector) Draw()                      {}

func (selector *BandSelector) UpdateOverlay(input simpleui.Input) bool {
	if !selector.open {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		selector.Close()
		return true
	}
	if input.Pressed {
		selector.pressedCategory = selector.categoryAt(input.Pointer)
		selector.pressedBand = selector.bandAt(input.Pointer)
		selector.pressedCancel = rl.CheckCollisionPointRec(input.Pointer, selector.cancelBounds())
	}
	if input.Released {
		category := selector.categoryAt(input.Pointer)
		bandIndex := selector.bandAt(input.Pointer)
		if rl.CheckCollisionPointRec(input.Pointer, selector.cancelBounds()) {
			selector.Close()
		} else if category >= 0 {
			selector.category = []string{"HAM", "COMMERCIAL", "ISM"}[category]
		} else if bandIndex >= 0 {
			band := bandsByCategory[selector.category][bandIndex]
			selector.selectedCategory, selector.selectedName = band.Category, band.Name
			selector.Close()
			if selector.onSelect != nil {
				selector.onSelect(band)
			}
		}
		selector.pressedCategory, selector.pressedBand = -1, -1
		selector.pressedCancel = false
	}
	return true
}

func (selector *BandSelector) DrawOverlay() {
	if !selector.open {
		return
	}
	accent := selector.accent()
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 200})
	modal := rl.Rectangle{X: 130, Y: 70, Width: 1020, Height: 530}
	rl.DrawRectangleRounded(modal, .022, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(modal, .022, 8, 2, colors.border)
	rl.DrawRectangleRounded(rl.Rectangle{X: 130, Y: 70, Width: 10, Height: 530}, .5, 8, accent)
	simpleui.DrawTextStyled("BAND SELECTION", 170, 91, 27, simpleui.FontRegular, colors.text)

	labels := []string{"RADIOAFICIONADO / HAM", "COMERCIALES", "ISM / LIBRE"}
	for index, label := range labels {
		bounds := selector.categoryBounds(index)
		active := []string{"HAM", "COMMERCIAL", "ISM"}[index] == selector.category
		categoryAccent := selector.categoryAccent(index)
		fill := blendRGBA(colors.panelAlt, categoryAccent, .24)
		if active {
			fill = blendRGBA(categoryAccent, rl.White, .05)
		}
		rl.DrawRectangleRounded(bounds, .12, 8, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .12, 8, 2, blendRGBA(categoryAccent, rl.White, .28))
		drawCentered(label, bounds, 17, simpleui.EnsureTextContrast(colors.text, fill))
	}

	bands := bandsByCategory[selector.category]
	for index, band := range bands {
		bounds := selector.bandBounds(index)
		selected := band.Category == selector.selectedCategory && band.Name == selector.selectedName
		fill := colors.panelAlt
		if selected {
			fill = accent
		}
		rl.DrawRectangleRounded(bounds, .14, 8, fill)
		rl.DrawRectangleRoundedLinesEx(bounds, .14, 8, 2, accent)
		labelColor := simpleui.EnsureTextContrast(colors.text, fill)
		drawCentered(band.Name, rl.Rectangle{X: bounds.X, Y: bounds.Y + 8, Width: bounds.Width, Height: 20}, 15, labelColor)
		drawCentered(formatBandFrequency(band.FrequencyHz), rl.Rectangle{X: bounds.X, Y: bounds.Y + 28, Width: bounds.Width, Height: 20}, 14, labelColor)
	}

	simpleui.DrawText("Select a band to change frequency and span.", 170, 540, 15, colors.text)
	cancel := selector.cancelBounds()
	cancelFill := colors.panelAlt
	rl.DrawRectangleRounded(cancel, .16, 8, cancelFill)
	rl.DrawRectangleRoundedLinesEx(cancel, .16, 8, 2, rl.Color{R: 135, G: 140, B: 145, A: 255})
	drawCentered("CANCEL", cancel, 16, simpleui.EnsureTextContrast(colors.text, cancelFill))
}

func (selector *BandSelector) categoryAt(point rl.Vector2) int {
	for index := 0; index < 3; index++ {
		if rl.CheckCollisionPointRec(point, selector.categoryBounds(index)) {
			return index
		}
	}
	return -1
}

func (selector *BandSelector) bandAt(point rl.Vector2) int {
	for index := range bandsByCategory[selector.category] {
		if rl.CheckCollisionPointRec(point, selector.bandBounds(index)) {
			return index
		}
	}
	return -1
}

func (selector *BandSelector) categoryBounds(index int) rl.Rectangle {
	return []rl.Rectangle{
		{X: 170, Y: 148, Width: 280, Height: 52},
		{X: 466, Y: 148, Width: 260, Height: 52},
		{X: 742, Y: 148, Width: 260, Height: 52},
	}[index]
}

func (selector *BandSelector) bandBounds(index int) rl.Rectangle {
	return rl.Rectangle{X: 170 + float32(index%5)*168, Y: 230 + float32(index/5)*72, Width: 152, Height: 54}
}

func (selector *BandSelector) cancelBounds() rl.Rectangle {
	return rl.Rectangle{X: 850, Y: 526, Width: 152, Height: 46}
}

func (selector *BandSelector) categoryAccent(index int) rl.Color {
	return []rl.Color{
		{R: 35, G: 155, B: 86, A: 255},
		{R: 36, G: 113, B: 163, A: 255},
		{R: 230, G: 126, B: 34, A: 255},
	}[index]
}

func (selector *BandSelector) accent() rl.Color {
	if selector.category == "COMMERCIAL" {
		return selector.categoryAccent(1)
	}
	if selector.category == "ISM" {
		return selector.categoryAccent(2)
	}
	return selector.categoryAccent(0)
}

func formatBandFrequency(frequencyHz int64) string {
	var formatted string
	if frequencyHz >= 1_000_000_000 {
		formatted = fmt.Sprintf("%.3f GHz", float64(frequencyHz)/1e9)
	} else if frequencyHz >= 1_000_000 {
		formatted = fmt.Sprintf("%.5f MHz", float64(frequencyHz)/1e6)
	} else if frequencyHz >= 1_000 {
		formatted = fmt.Sprintf("%.3f kHz", float64(frequencyHz)/1e3)
	} else {
		return fmt.Sprintf("%d Hz", frequencyHz)
	}
	formatted = strings.TrimRight(strings.TrimRight(strings.Split(formatted, " ")[0], "0"), ".") + " " + strings.Split(formatted, " ")[1]
	return strings.Replace(formatted, ".", ",", 1)
}
