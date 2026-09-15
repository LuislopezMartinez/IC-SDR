package screens

import (
	"go-zero/internal/i18n"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type bandRange struct {
	category, name       string
	minimumHz, maximumHz int64
}

var bandPlan = []bandRange{
	{i18n.Source("text.4fae663ae96a"), "160 m", 1_810_000, 2_000_000}, {i18n.Source("text.4fae663ae96a"), "80 m", 3_500_000, 3_800_000},
	{i18n.Source("text.4fae663ae96a"), "60 m", 5_351_500, 5_366_500}, {i18n.Source("text.4fae663ae96a"), "40 m", 7_000_000, 7_200_000},
	{i18n.Source("text.4fae663ae96a"), "30 m", 10_100_000, 10_150_000}, {i18n.Source("text.4fae663ae96a"), "20 m", 14_000_000, 14_350_000},
	{i18n.Source("text.4fae663ae96a"), "17 m", 18_068_000, 18_168_000}, {i18n.Source("text.4fae663ae96a"), "15 m", 21_000_000, 21_450_000},
	{i18n.Source("text.4fae663ae96a"), "12 m", 24_890_000, 24_990_000}, {i18n.Source("text.4fae663ae96a"), "10 m", 28_000_000, 29_700_000},
	{i18n.Source("text.4fae663ae96a"), "6 m", 50_000_000, 52_000_000}, {i18n.Source("text.4fae663ae96a"), "4 m", 70_000_000, 70_500_000},
	{i18n.Source("text.4fae663ae96a"), "2 m", 144_000_000, 146_000_000}, {i18n.Source("text.4fae663ae96a"), "70 cm", 430_000_000, 440_000_000},
	{i18n.Source("text.4fae663ae96a"), "23 cm", 1_240_000_000, 1_300_000_000},
	{i18n.Source("text.00c2ae96f694"), "LW", 148_500, 283_500}, {i18n.Source("text.00c2ae96f694"), i18n.Source("text.f27f69c7bee0"), 526_500, 1_606_500},
	{i18n.Source("text.00c2ae96f694"), i18n.Source("text.422aa045ffdc"), 5_900_000, 6_200_000}, {i18n.Source("text.00c2ae96f694"), i18n.Source("text.fd43177cd5c7"), 7_200_000, 7_450_000},
	{i18n.Source("text.00c2ae96f694"), i18n.Source("text.ee734a42f240"), 9_400_000, 9_900_000}, {i18n.Source("text.00c2ae96f694"), "FM", 87_500_000, 108_000_000},
	{i18n.Source("text.00c2ae96f694"), i18n.Source("text.9e0d5d5acc31"), 118_000_000, 136_975_000}, {i18n.Source("text.00c2ae96f694"), i18n.Source("text.4137a9ef7e64"), 156_000_000, 162_000_000},
	{i18n.Source("text.00c2ae96f694"), i18n.Source("text.bbcc8ec46f07"), 174_928_000, 239_200_000},
	{i18n.Source("text.00c2ae96f694"), i18n.Source("text.493b20493ea1"), 400_000_000, 406_000_000},
	{i18n.Source("text.3e250a199e99"), "CB 27", 26_965_000, 27_405_000}, {i18n.Source("text.3e250a199e99"), i18n.Source("text.9565c4e7fc84"), 446_000_000, 446_200_000},
	{i18n.Source("text.3e250a199e99"), i18n.Source("text.b76fd1fc7152"), 433_050_000, 434_790_000}, {i18n.Source("text.3e250a199e99"), i18n.Source("text.b22302813a6c"), 863_000_000, 870_000_000},
	{i18n.Source("text.3e250a199e99"), i18n.Source("text.116e50259e29"), 902_000_000, 928_000_000}, {i18n.Source("text.3e250a199e99"), i18n.Source("text.9ce5ecb522a9"), 2_400_000_000, 2_483_500_000},
}

func findBandRange(frequencyHz int64, preferredCategory string) (bandRange, bool) {
	// Preserve the user's current context in overlapping allocations.
	if preferredCategory != "" && preferredCategory != i18n.Source("text.c627c09c14e5") {
		for _, band := range bandPlan {
			if band.category == preferredCategory && frequencyHz >= band.minimumHz && frequencyHz <= band.maximumHz {
				return band, true
			}
		}
	}
	for _, category := range []string{i18n.Source("text.4fae663ae96a"), i18n.Source("text.00c2ae96f694"), i18n.Source("text.3e250a199e99")} {
		for _, band := range bandPlan {
			if band.category == category && frequencyHz >= band.minimumHz && frequencyHz <= band.maximumHz {
				return band, true
			}
		}
	}
	return bandRange{}, false
}

func (screen *MainScreen) updateBandForFrequency(frequencyHz int64) {
	band, found := findBandRange(frequencyHz, screen.bandCategory)
	if !found {
		screen.bandCategory, screen.bandName = i18n.Source("text.c627c09c14e5"), i18n.Source("text.34c6a2898d3d")
	} else {
		screen.bandCategory, screen.bandName = band.category, band.name
	}
	if screen.band != nil {
		label := i18n.Source("text.eae2605aec31") + screen.bandName
		if !found {
			label = i18n.Source("text.f808033a4b69")
		}
		screen.band.SetLabel(label)
	}
	if screen.bandSelector != nil {
		screen.bandSelector.selectedCategory = screen.bandCategory
		screen.bandSelector.selectedName = screen.bandName
		if found {
			screen.bandSelector.category = screen.bandCategory
		}
	}
}

func (screen *MainScreen) drawBandPlanStrip(x, y, width, height float32) {
	if screen.spanHz <= 0 {
		return
	}
	visibleMin, visibleMax := screen.centerFrequencyHz-screen.spanHz/2, screen.centerFrequencyHz+screen.spanHz/2
	rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: width, Height: height}, rl.Color{R: 10, G: 13, B: 20, A: 225})
	// The active category is painted last so overlaps such as HAM 70 cm / ISM
	// 433 MHz remain understandable.
	categories := []string{i18n.Source("text.00c2ae96f694"), i18n.Source("text.3e250a199e99"), i18n.Source("text.4fae663ae96a")}
	for pass := 0; pass < 2; pass++ {
		for _, category := range categories {
			activeCategory := category == screen.bandCategory
			if (pass == 0 && activeCategory) || (pass == 1 && !activeCategory) {
				continue
			}
			for _, band := range bandPlan {
				if band.category != category || band.maximumHz < visibleMin || band.minimumHz > visibleMax {
					continue
				}
				low, high := max(band.minimumHz, visibleMin), min(band.maximumHz, visibleMax)
				left := x + width*float32(low-visibleMin)/float32(screen.spanHz)
				right := x + width*float32(high-visibleMin)/float32(screen.spanHz)
				fill, border := bandPlanColors(category)
				active := activeCategory && band.name == screen.bandName
				if active {
					fill.A = 205
					border = rl.Color{R: 255, G: 245, B: 225, A: 255}
				}
				rect := rl.Rectangle{X: left, Y: y + 1, Width: max(1, right-left), Height: height - 2}
				rl.DrawRectangleRec(rect, fill)
				rl.DrawRectangleLinesEx(rect, map[bool]float32{true: 2, false: 1}[active], border)
				if rect.Width >= 70 {
					label := band.name
					if rect.Width >= 135 {
						label += "  " + category
					}
					size := int32(12)
					measured := simpleui.MeasureTextStyled(label, size, simpleui.FontSemiBold)
					if measured.X+8 <= rect.Width {
						simpleui.DrawTextStyled(label, rect.X+(rect.Width-measured.X)/2, rect.Y+(rect.Height-measured.Y)/2, size, simpleui.FontSemiBold, colors.text)
					}
				}
			}
		}
	}
	rl.DrawRectangleLinesEx(rl.Rectangle{X: x, Y: y, Width: width, Height: height}, 1, rl.Color{R: 90, G: 108, B: 128, A: 220})
}

func bandPlanColors(category string) (rl.Color, rl.Color) {
	switch category {
	case "COMMERCIAL":
		return rl.Color{R: 28, G: 72, B: 155, A: 150}, rl.Color{R: 75, G: 145, B: 245, A: 230}
	case "ISM":
		return rl.Color{R: 172, G: 78, B: 16, A: 155}, rl.Color{R: 245, G: 140, B: 48, A: 230}
	default:
		return rl.Color{R: 28, G: 112, B: 58, A: 150}, rl.Color{R: 75, G: 205, B: 110, A: 230}
	}
}
