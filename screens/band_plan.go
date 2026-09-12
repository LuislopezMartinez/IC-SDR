package screens

import (
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type bandRange struct {
	category, name       string
	minimumHz, maximumHz int64
}

var bandPlan = []bandRange{
	{"HAM", "160 m", 1_810_000, 2_000_000}, {"HAM", "80 m", 3_500_000, 3_800_000},
	{"HAM", "60 m", 5_351_500, 5_366_500}, {"HAM", "40 m", 7_000_000, 7_200_000},
	{"HAM", "30 m", 10_100_000, 10_150_000}, {"HAM", "20 m", 14_000_000, 14_350_000},
	{"HAM", "17 m", 18_068_000, 18_168_000}, {"HAM", "15 m", 21_000_000, 21_450_000},
	{"HAM", "12 m", 24_890_000, 24_990_000}, {"HAM", "10 m", 28_000_000, 29_700_000},
	{"HAM", "6 m", 50_000_000, 52_000_000}, {"HAM", "4 m", 70_000_000, 70_500_000},
	{"HAM", "2 m", 144_000_000, 146_000_000}, {"HAM", "70 cm", 430_000_000, 440_000_000},
	{"HAM", "23 cm", 1_240_000_000, 1_300_000_000},
	{"COMMERCIAL", "LW", 148_500, 283_500}, {"COMMERCIAL", "MW / AM", 526_500, 1_606_500},
	{"COMMERCIAL", "SW 49 m", 5_900_000, 6_200_000}, {"COMMERCIAL", "SW 41 m", 7_200_000, 7_450_000},
	{"COMMERCIAL", "SW 31 m", 9_400_000, 9_900_000}, {"COMMERCIAL", "FM", 87_500_000, 108_000_000},
	{"COMMERCIAL", "AIR", 118_000_000, 136_975_000}, {"COMMERCIAL", "MARINE", 156_000_000, 162_000_000},
	{"COMMERCIAL", "DAB", 174_928_000, 239_200_000},
	{"COMMERCIAL", "RADIOSONDES", 400_000_000, 406_000_000},
	{"ISM", "CB 27", 26_965_000, 27_405_000}, {"ISM", "PMR446", 446_000_000, 446_200_000},
	{"ISM", "433 MHz", 433_050_000, 434_790_000}, {"ISM", "868 MHz", 863_000_000, 870_000_000},
	{"ISM", "915 MHz", 902_000_000, 928_000_000}, {"ISM", "2.4 GHz", 2_400_000_000, 2_483_500_000},
}

func findBandRange(frequencyHz int64, preferredCategory string) (bandRange, bool) {
	// Preserve the user's current context in overlapping allocations.
	if preferredCategory != "" && preferredCategory != "NONE" {
		for _, band := range bandPlan {
			if band.category == preferredCategory && frequencyHz >= band.minimumHz && frequencyHz <= band.maximumHz {
				return band, true
			}
		}
	}
	for _, category := range []string{"HAM", "COMMERCIAL", "ISM"} {
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
		screen.bandCategory, screen.bandName = "NONE", "OUT OF BAND"
	} else {
		screen.bandCategory, screen.bandName = band.category, band.name
	}
	if screen.band != nil {
		label := T("BAND") + "  " + screen.bandName
		if !found {
			label = T("BAND OUT")
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
	categories := []string{"COMMERCIAL", "ISM", "HAM"}
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
