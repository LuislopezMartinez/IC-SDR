package screens

import (
	"go-zero/internal/i18n"

	"image/color"
	"math"

	"go-zero/internal/sdr"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	waterfallWidth  = 1552
	waterfallHeight = 170
)

type WaterfallSettings struct {
	LinesPerSecond int
	Contrast       int
	ColorOffsetDB  int
	MinimumDBm     float32
	MaximumDBm     float32
	Palette        string
}

func defaultWaterfallSettings() WaterfallSettings {
	return WaterfallSettings{
		LinesPerSecond: 15,
		Contrast:       102,
		ColorOffsetDB:  -30,
		MinimumDBm:     -60,
		MaximumDBm:     0,
		Palette:        i18n.Source("text.2d71cca47c3a"),
	}
}

func factoryWaterfallSettings() WaterfallSettings {
	return WaterfallSettings{LinesPerSecond: 31, Contrast: 100, MinimumDBm: -80, MaximumDBm: -20, Palette: i18n.Source("text.24a866f4940f")}
}

type Waterfall struct {
	settings     *WaterfallSettings
	historyDBm   []float32
	pixels       []color.RGBA
	linePixels   []color.RGBA
	spectrum     []float32
	newestRow    int
	texture      rl.Texture2D
	textureReady bool
	colorsDirty  bool
	nextLineAt   float64
}

func NewWaterfall(settings *WaterfallSettings, spectrumSize int) *Waterfall {
	history := make([]float32, waterfallWidth*waterfallHeight)
	for index := range history {
		history[index] = -120
	}
	return &Waterfall{
		settings:    settings,
		historyDBm:  history,
		pixels:      make([]color.RGBA, waterfallWidth*waterfallHeight),
		linePixels:  make([]color.RGBA, waterfallWidth),
		spectrum:    make([]float32, spectrumSize),
		colorsDirty: true,
	}
}

func (waterfall *Waterfall) InvalidateColors() { waterfall.colorsDirty = true }

func (waterfall *Waterfall) Reset() {
	for index := range waterfall.historyDBm {
		waterfall.historyDBm[index] = -120
	}
	waterfall.newestRow = 0
	waterfall.nextLineAt = 0
	waterfall.colorsDirty = true
}

func (waterfall *Waterfall) Update(receiver *sdr.Receiver, sampleRate float64, spanHz int64) {
	if receiver == nil || sampleRate <= 0 || len(waterfall.spectrum) == 0 {
		return
	}
	now := rl.GetTime()
	if now < waterfall.nextLineAt {
		return
	}
	waterfall.nextLineAt = now + 1/float64(max(waterfall.settings.LinesPerSecond, 1))
	stats := receiver.Snapshot(waterfall.spectrum)
	if stats.FFTBlocks == 0 {
		return
	}

	waterfall.newestRow = (waterfall.newestRow - 1 + waterfallHeight) % waterfallHeight
	visibleBins := min(max(int(math.Round(float64(len(waterfall.spectrum))*float64(spanHz)/sampleRate)), 8), len(waterfall.spectrum))
	firstBin := (len(waterfall.spectrum) - visibleBins) / 2
	rowStart := waterfall.newestRow * waterfallWidth
	for pixel := 0; pixel < waterfallWidth; pixel++ {
		fraction := float64(pixel) / float64(waterfallWidth-1)
		bin := firstBin + int(math.Round(fraction*float64(visibleBins-1)))
		value := waterfall.spectrum[bin]
		waterfall.historyDBm[rowStart+pixel] = value
		mapped := waterfall.colorFor(value)
		waterfall.pixels[rowStart+pixel] = mapped
		waterfall.linePixels[pixel] = mapped
	}
	if waterfall.textureReady {
		rl.UpdateTextureRec(waterfall.texture,
			rl.Rectangle{X: 0, Y: float32(waterfall.newestRow), Width: waterfallWidth, Height: 1},
			waterfall.linePixels)
	}
}

func (waterfall *Waterfall) Draw(x, y, width, height float32, tuningFraction float32) {
	waterfall.ensureTexture()
	if waterfall.colorsDirty {
		waterfall.recolorHistory()
	}
	tailRows := waterfallHeight - waterfall.newestRow
	if tailRows > 0 {
		destinationHeight := height * float32(tailRows) / waterfallHeight
		rl.DrawTexturePro(waterfall.texture,
			rl.Rectangle{X: 0, Y: float32(waterfall.newestRow), Width: waterfallWidth, Height: float32(tailRows)},
			rl.Rectangle{X: x, Y: y, Width: width, Height: destinationHeight}, rl.Vector2{}, 0, rl.White)
	}
	if waterfall.newestRow > 0 {
		tailHeight := height * float32(tailRows) / waterfallHeight
		rl.DrawTexturePro(waterfall.texture,
			rl.Rectangle{X: 0, Y: 0, Width: waterfallWidth, Height: float32(waterfall.newestRow)},
			rl.Rectangle{X: x, Y: y + tailHeight, Width: width, Height: height - tailHeight}, rl.Vector2{}, 0, rl.White)
	}
	rl.DrawRectangleLinesEx(rl.Rectangle{X: x, Y: y, Width: width, Height: height}, 1, colors.border)
	cursorX := x + width*min(max(tuningFraction, 0), 1)
	rl.DrawLineEx(rl.Vector2{X: cursorX, Y: y}, rl.Vector2{X: cursorX, Y: y + height}, 1.5, colors.cyan)
}

func (waterfall *Waterfall) ensureTexture() {
	if waterfall.textureReady {
		return
	}
	image := rl.GenImageColor(waterfallWidth, waterfallHeight, color.RGBA{R: 4, G: 7, B: 38, A: 255})
	waterfall.texture = rl.LoadTextureFromImage(image)
	rl.UnloadImage(image)
	rl.SetTextureFilter(waterfall.texture, rl.FilterPoint)
	waterfall.textureReady = true
}

func (waterfall *Waterfall) recolorHistory() {
	for index, value := range waterfall.historyDBm {
		waterfall.pixels[index] = waterfall.colorFor(value)
	}
	if waterfall.textureReady {
		rl.UpdateTexture(waterfall.texture, waterfall.pixels)
	}
	waterfall.colorsDirty = false
}

func (waterfall *Waterfall) colorFor(dbm float32) color.RGBA {
	settings := waterfall.settings
	rangeDB := max(settings.MaximumDBm-settings.MinimumDBm, 1)
	level := (dbm + float32(settings.ColorOffsetDB) - settings.MinimumDBm) / rangeDB
	level = (level-.5)*float32(settings.Contrast)/100 + .5
	level = min(max(level, 0), 1)
	switch settings.Palette {
	case "FIRE":
		if level < .4 {
			return lerpRGBA(rgba(5, 2, 12), rgba(145, 15, 15), level/.4)
		}
		if level < .75 {
			return lerpRGBA(rgba(145, 15, 15), rgba(255, 125, 10), (level-.4)/.35)
		}
		return lerpRGBA(rgba(255, 125, 10), rgba(255, 255, 180), (level-.75)/.25)
	case "VIRIDIS":
		if level < .5 {
			return lerpRGBA(rgba(35, 5, 70), rgba(15, 145, 125), level/.5)
		}
		return lerpRGBA(rgba(15, 145, 125), rgba(245, 235, 65), (level-.5)/.5)
	case "GRAY":
		gray := uint8(math.Round(8 + float64(level)*247))
		return rgba(gray, gray, gray)
	default:
		if level < .35 {
			return lerpRGBA(rgba(4, 7, 38), rgba(25, 45, 180), level/.35)
		}
		if level < .7 {
			return lerpRGBA(rgba(25, 45, 180), rgba(0, 235, 255), (level-.35)/.35)
		}
		return lerpRGBA(rgba(0, 235, 255), rgba(255, 255, 115), (level-.7)/.3)
	}
}

func rgba(red, green, blue uint8) color.RGBA {
	return color.RGBA{R: red, G: green, B: blue, A: 255}
}

func lerpRGBA(from, to color.RGBA, amount float32) color.RGBA {
	amount = min(max(amount, 0), 1)
	return color.RGBA{
		R: uint8(float32(from.R) + (float32(to.R)-float32(from.R))*amount),
		G: uint8(float32(from.G) + (float32(to.G)-float32(from.G))*amount),
		B: uint8(float32(from.B) + (float32(to.B)-float32(from.B))*amount),
		A: 255,
	}
}
