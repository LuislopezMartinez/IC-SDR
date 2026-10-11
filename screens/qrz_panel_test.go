package screens

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestDecodeQRZImageConvertsJPEGToRGBA(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 3, 2))
	source.SetRGBA(1, 1, color.RGBA{R: 220, G: 40, B: 30, A: 255})
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	width, height, pixels, ok := decodeQRZImage(encoded.Bytes())
	if !ok || width != 3 || height != 2 || len(pixels) != 6 {
		t.Fatalf("decoded JPEG = %dx%d, pixels=%d, ok=%v", width, height, len(pixels), ok)
	}
}
