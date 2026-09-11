package screens

import "testing"

func TestCatalogMaxScrollUsesFullExpandedContentHeight(t *testing.T) {
	const viewport = float32(667)
	short := catalogMaxScroll(600, viewport)
	long := catalogMaxScroll(2800, viewport)
	if short != 0 {
		t.Fatalf("short catalog max scroll = %d; want 0", short)
	}
	if long != 89 {
		t.Fatalf("expanded catalog max scroll = %d; want 89", long)
	}
}
