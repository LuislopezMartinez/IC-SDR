package screens

import (
	"go-zero/internal/i18n"

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
	i18n.Source("text.4fae663ae96a"): {
		{Category: i18n.Source("text.4fae663ae96a"), Name: "160 m", FrequencyHz: 1_900_000, SpanHz: 25_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "80 m", FrequencyHz: 3_650_000, SpanHz: 50_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "60 m", FrequencyHz: 5_354_000, SpanHz: 25_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "40 m", FrequencyHz: 7_100_000, SpanHz: 50_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "30 m", FrequencyHz: 10_125_000, SpanHz: 25_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "20 m", FrequencyHz: 14_200_000, SpanHz: 100_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "17 m", FrequencyHz: 18_100_000, SpanHz: 50_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "15 m", FrequencyHz: 21_200_000, SpanHz: 100_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "12 m", FrequencyHz: 24_950_000, SpanHz: 50_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "10 m", FrequencyHz: 28_500_000, SpanHz: 200_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "6 m", FrequencyHz: 50_150_000, SpanHz: 200_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "4 m", FrequencyHz: 70_200_000, SpanHz: 200_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "2 m", FrequencyHz: 145_000_000, SpanHz: 500_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "70 cm", FrequencyHz: 433_500_000, SpanHz: 500_000},
		{Category: i18n.Source("text.4fae663ae96a"), Name: "23 cm", FrequencyHz: 1_296_000_000, SpanHz: 1_000_000},
	},
	i18n.Source("text.00c2ae96f694"): {
		{Category: i18n.Source("text.00c2ae96f694"), Name: "LW", FrequencyHz: 198_000, SpanHz: 100_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.f27f69c7bee0"), FrequencyHz: 1_000_000, SpanHz: 500_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.422aa045ffdc"), FrequencyHz: 6_100_000, SpanHz: 200_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.fd43177cd5c7"), FrequencyHz: 7_300_000, SpanHz: 200_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.ee734a42f240"), FrequencyHz: 9_600_000, SpanHz: 200_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: "FM", FrequencyHz: 100_000_000, SpanHz: 2_000_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.9e0d5d5acc31"), FrequencyHz: 125_000_000, SpanHz: 1_000_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.4137a9ef7e64"), FrequencyHz: 156_800_000, SpanHz: 1_000_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.9fbabbb6fe61"), FrequencyHz: 162_000_000, SpanHz: 250_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.bbcc8ec46f07"), FrequencyHz: 220_352_000, SpanHz: 2_000_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.493b20493ea1"), FrequencyHz: 403_000_000, SpanHz: 250_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.5de9168480ce"), FrequencyHz: 978_000_000, SpanHz: 2_000_000},
		{Category: i18n.Source("text.00c2ae96f694"), Name: i18n.Source("text.2bfb0a95d742"), FrequencyHz: 1_090_000_000, SpanHz: 2_000_000},
	},
	i18n.Source("text.3e250a199e99"): {
		{Category: i18n.Source("text.3e250a199e99"), Name: "CB 27", FrequencyHz: 27_205_000, SpanHz: 500_000},
		{Category: i18n.Source("text.3e250a199e99"), Name: i18n.Source("text.9565c4e7fc84"), FrequencyHz: 446_006_250, SpanHz: 500_000},
		{Category: i18n.Source("text.3e250a199e99"), Name: i18n.Source("text.b76fd1fc7152"), FrequencyHz: 433_920_000, SpanHz: 1_000_000},
		{Category: i18n.Source("text.3e250a199e99"), Name: i18n.Source("text.b22302813a6c"), FrequencyHz: 868_300_000, SpanHz: 2_000_000},
		{Category: i18n.Source("text.3e250a199e99"), Name: i18n.Source("text.116e50259e29"), FrequencyHz: 915_000_000, SpanHz: 2_000_000},
		{Category: i18n.Source("text.3e250a199e99"), Name: i18n.Source("text.9ce5ecb522a9"), FrequencyHz: 2_440_000_000, SpanHz: 2_000_000},
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
		category = i18n.Source("text.4fae663ae96a")
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
			selector.category = []string{i18n.Source("text.4fae663ae96a"), i18n.Source("text.00c2ae96f694"), i18n.Source("text.3e250a199e99")}[category]
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
	simpleui.DrawTextStyled(i18n.Source("text.63955630f3d8"), 170, 91, 27, simpleui.FontRegular, colors.text)

	labels := []string{i18n.Source("text.4d6d3d18b060"), i18n.Source("text.8d85ede58085"), i18n.Source("text.75de1f15f7ee")}
	for index, label := range labels {
		bounds := selector.categoryBounds(index)
		active := []string{i18n.Source("text.4fae663ae96a"), i18n.Source("text.00c2ae96f694"), i18n.Source("text.3e250a199e99")}[index] == selector.category
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

	simpleui.DrawText(i18n.Source("text.bd1e9b6c9b71"), 170, 540, 15, colors.text)
	cancel := selector.cancelBounds()
	cancelFill := colors.panelAlt
	rl.DrawRectangleRounded(cancel, .16, 8, cancelFill)
	rl.DrawRectangleRoundedLinesEx(cancel, .16, 8, 2, rl.Color{R: 135, G: 140, B: 145, A: 255})
	drawCentered(i18n.Source("text.b1a5fe65d180"), cancel, 16, simpleui.EnsureTextContrast(colors.text, cancelFill))
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
	if selector.category == i18n.Source("text.00c2ae96f694") {
		return selector.categoryAccent(1)
	}
	if selector.category == i18n.Source("text.3e250a199e99") {
		return selector.categoryAccent(2)
	}
	return selector.categoryAccent(0)
}

func formatBandFrequency(frequencyHz int64) string {
	var formatted string
	if frequencyHz >= 1_000_000_000 {
		formatted = fmt.Sprintf(i18n.Source("text.51020b68758d"), float64(frequencyHz)/1e9)
	} else if frequencyHz >= 1_000_000 {
		formatted = fmt.Sprintf(i18n.Source("text.4d983ee4ca80"), float64(frequencyHz)/1e6)
	} else if frequencyHz >= 1_000 {
		formatted = fmt.Sprintf(i18n.Source("text.474f3123592c"), float64(frequencyHz)/1e3)
	} else {
		return fmt.Sprintf(i18n.Source("text.3999a0ad05ce"), frequencyHz)
	}
	formatted = strings.TrimRight(strings.TrimRight(strings.Split(formatted, " ")[0], "0"), ".") + " " + strings.Split(formatted, " ")[1]
	return strings.Replace(formatted, ".", ",", 1)
}
