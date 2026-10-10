package screens

import rl "github.com/gen2brain/raylib-go/raylib"

type semanticActionPalette struct {
	start, stop, clear, export rl.Color
}

var defaultActionColors = semanticActionColors(darkPalette)

var (
	actionStartFill  = defaultActionColors.start
	actionStopFill   = defaultActionColors.stop
	actionClearFill  = defaultActionColors.clear
	actionExportFill = defaultActionColors.export
)

// semanticActionColors gives action colors one stable meaning across every
// theme: green starts/confirms, red stops, muted red is destructive and teal
// represents a non-destructive export or save operation.
func semanticActionColors(p uiPalette) semanticActionPalette {
	return semanticActionPalette{
		start:  mixColor(p.panel, p.green, .48),
		stop:   mixColor(p.panel, p.red, .52),
		clear:  mixColor(p.panel, p.red, .28),
		export: mixColor(p.panel, p.green, .38),
	}
}
