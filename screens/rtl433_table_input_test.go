package screens

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"testing"
)

func TestRTL433TableHitAreas(t *testing.T) {
	for _, x := range []float32{70, 140, 215, 285} {
		if rtl433TableRowAt(rl.Vector2{X: x, Y: 755}, 4) != -1 {
			t.Fatal("bandwidth control overlaps table input")
		}
	}
	for row := 0; row < 4; row++ {
		if got := rtl433TableRowAt(rl.Vector2{X: 500, Y: 746 + float32(row)*14}, 4); got != row {
			t.Fatalf("row %d hit %d", row, got)
		}
	}
	if rtl433TableRowAt(rl.Vector2{X: 500, Y: 725}, 4) != -1 {
		t.Fatal("header selects row")
	}
}
