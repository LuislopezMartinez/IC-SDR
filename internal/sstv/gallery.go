package sstv

import (
	"encoding/json"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

const sstvMetadataSuffix = ".sstv.json"

// sstvFrameMetadata accompanies received PNGs. PNG deliberately remains a
// standard image file; this small sidecar keeps the per-line reception data
// needed for safe reconstruction in the gallery.
type sstvFrameMetadata struct {
	Version     int     `json:"version"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Lines       int     `json:"lines"`
	LineQuality []uint8 `json:"lineQuality"`
}

func writeSSTVFrameMetadata(imagePath string, frame Frame) error {
	metadata := sstvFrameMetadata{Version: 1, Width: frame.Width, Height: frame.Height, Lines: frame.Lines, LineQuality: frame.LineQuality}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(imagePath+sstvMetadataSuffix, data, 0o644)
}

func readSSTVFrameMetadata(imagePath string, width, height int) (lines int, quality []uint8) {
	data, err := os.ReadFile(imagePath + sstvMetadataSuffix)
	if err != nil {
		return height, nil
	}
	var metadata sstvFrameMetadata
	if json.Unmarshal(data, &metadata) != nil || metadata.Version != 1 || metadata.Width != width || metadata.Height != height || len(metadata.LineQuality) != height {
		return height, nil
	}
	return min(max(metadata.Lines, 0), height), append([]uint8(nil), metadata.LineQuality...)
}

// EnhancementOptions controls the non-destructive adjustments available in the
// SSTV gallery. The received file is never changed: callers always supply a
// different destination for the rendered copy.
type EnhancementOptions struct {
	ColorCorrection bool
	WhiteBalance    bool
	Contrast        bool
	AlignRows       bool
	RecoverLines    bool
	ReduceNoise     bool
	Smooth          bool
	ReduceBands     bool
	Sharpen         bool
	Contours        bool
}

// EnhancementResult describes the corrections applied while rendering a copy.
type EnhancementResult struct {
	CorrectedLines     int
	ReconstructedLines int
	DenoisedPixels     int
}

// EnhancePNG renders sourcePath with the selected SSTV adjustments into a PNG
// at destinationPath. Saved SSTV images do not retain their line confidence,
// so every imported row is treated as received; line reconstruction remains
// available for images containing transparent/missing rows in future formats.
func EnhancePNG(sourcePath, destinationPath string, options EnhancementOptions) (EnhancementResult, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return EnhancementResult{}, err
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		return EnhancementResult{}, err
	}
	bounds := decoded.Bounds()
	frame := Frame{Width: bounds.Dx(), Height: bounds.Dy(), Lines: bounds.Dy()}
	frame.RGB = make([]byte, frame.Width*frame.Height*3)
	frame.Lines, frame.LineQuality = readSSTVFrameMetadata(sourcePath, frame.Width, frame.Height)
	legacyImage := frame.LineQuality == nil
	if legacyImage {
		frame.LineQuality = make([]uint8, frame.Height)
	}
	for y := 0; y < frame.Height; y++ {
		if legacyImage {
			frame.LineQuality[y] = 1
		}
		for x := 0; x < frame.Width; x++ {
			red, green, blue, _ := decoded.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			i := (y*frame.Width + x) * 3
			frame.RGB[i], frame.RGB[i+1], frame.RGB[i+2] = byte(red>>8), byte(green>>8), byte(blue>>8)
		}
	}
	result := EnhancementResult{}
	if options.AlignRows {
		frame, result.CorrectedLines = alignSSTVRows(frame)
	}
	if options.RecoverLines {
		frame, result.ReconstructedLines = reconstructSSTVLines(frame, 6)
	}
	if options.ReduceNoise {
		frame, result.DenoisedPixels = reduceSSTVImpulseNoise(frame)
	}
	if options.Smooth {
		smoothSSTVImage(&frame)
	}
	if options.ReduceBands {
		reduceSSTVHorizontalBands(&frame)
	}
	if options.ColorCorrection {
		correctSSTVColors(&frame)
	}
	if options.WhiteBalance {
		balanceSSTVColors(&frame)
	}
	if options.Contrast {
		increaseSSTVContrast(&frame)
	}
	if options.Sharpen {
		sharpenSSTVImage(&frame)
	}
	if options.Contours {
		showSSTVContours(&frame)
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return result, err
	}
	output, err := os.Create(destinationPath)
	if err != nil {
		return result, err
	}
	imageOut := image.NewRGBA(image.Rect(0, 0, frame.Width, frame.Height))
	for y := 0; y < frame.Height; y++ {
		for x := 0; x < frame.Width; x++ {
			i := (y*frame.Width + x) * 3
			imageOut.SetRGBA(x, y, color.RGBA{R: frame.RGB[i], G: frame.RGB[i+1], B: frame.RGB[i+2], A: 255})
		}
	}
	if err := png.Encode(output, imageOut); err != nil {
		_ = output.Close()
		return result, err
	}
	if err := output.Close(); err != nil {
		return result, err
	}
	return result, writeSSTVFrameMetadata(destinationPath, frame)
}

func smoothSSTVImage(frame *Frame) {
	if frame == nil || frame.Width < 3 || frame.Height < 3 {
		return
	}
	source := append([]byte(nil), frame.RGB...)
	for y := 1; y < frame.Height-1; y++ {
		for x := 1; x < frame.Width-1; x++ {
			for c := 0; c < 3; c++ {
				sum := 0
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						sum += int(source[((y+dy)*frame.Width+x+dx)*3+c])
					}
				}
				frame.RGB[(y*frame.Width+x)*3+c] = byte(sum / 9)
			}
		}
	}
}

func reduceSSTVHorizontalBands(frame *Frame) {
	if frame == nil || frame.Width == 0 || frame.Height == 0 {
		return
	}
	means := [3]int{}
	for i := 0; i+2 < len(frame.RGB); i += 3 {
		for c := 0; c < 3; c++ {
			means[c] += int(frame.RGB[i+c])
		}
	}
	pixels := frame.Width * frame.Height
	for c := 0; c < 3; c++ {
		means[c] /= pixels
	}
	for y := 0; y < frame.Height; y++ {
		row := [3]int{}
		for x := 0; x < frame.Width; x++ {
			for c := 0; c < 3; c++ {
				row[c] += int(frame.RGB[(y*frame.Width+x)*3+c])
			}
		}
		for c := 0; c < 3; c++ {
			row[c] /= frame.Width
			row[c] = min(max(means[c]-row[c], -28), 28)
		}
		for x := 0; x < frame.Width; x++ {
			for c := 0; c < 3; c++ {
				i := (y*frame.Width+x)*3 + c
				frame.RGB[i] = byte(min(max(int(frame.RGB[i])+row[c], 0), 255))
			}
		}
	}
}

func balanceSSTVColors(frame *Frame) {
	if frame == nil || len(frame.RGB) == 0 {
		return
	}
	mean := [3]int{}
	pixels := len(frame.RGB) / 3
	for i := 0; i+2 < len(frame.RGB); i += 3 {
		for c := 0; c < 3; c++ {
			mean[c] += int(frame.RGB[i+c])
		}
	}
	for c := 0; c < 3; c++ {
		mean[c] /= pixels
	}
	target := (mean[0] + mean[1] + mean[2]) / 3
	for i := 0; i+2 < len(frame.RGB); i += 3 {
		for c := 0; c < 3; c++ {
			if mean[c] > 0 {
				frame.RGB[i+c] = byte(min(max(int(frame.RGB[i+c])*target/mean[c], 0), 255))
			}
		}
	}
}

func increaseSSTVContrast(frame *Frame) {
	if frame == nil {
		return
	}
	for i := range frame.RGB {
		frame.RGB[i] = byte(min(max((int(frame.RGB[i])-128)*12/10+128, 0), 255))
	}
}

func sharpenSSTVImage(frame *Frame) {
	if frame == nil || frame.Width < 3 || frame.Height < 3 {
		return
	}
	source := append([]byte(nil), frame.RGB...)
	for y := 1; y < frame.Height-1; y++ {
		for x := 1; x < frame.Width-1; x++ {
			for c := 0; c < 3; c++ {
				i := (y*frame.Width+x)*3 + c
				neighbours := int(source[i-3]) + int(source[i+3]) + int(source[i-frame.Width*3]) + int(source[i+frame.Width*3])
				frame.RGB[i] = byte(min(max(int(source[i])+(int(source[i])-neighbours/4)/2, 0), 255))
			}
		}
	}
}

func showSSTVContours(frame *Frame) {
	if frame == nil || frame.Width < 2 || frame.Height < 2 {
		return
	}
	source := append([]byte(nil), frame.RGB...)
	gray := func(i int) int { return (int(source[i])*30 + int(source[i+1])*59 + int(source[i+2])*11) / 100 }
	for y := 0; y < frame.Height-1; y++ {
		for x := 0; x < frame.Width-1; x++ {
			i := (y*frame.Width + x) * 3
			edge := absInt(gray(i)-gray(i+3)) + absInt(gray(i)-gray(i+frame.Width*3))
			v := byte(min(edge*2, 255))
			frame.RGB[i], frame.RGB[i+1], frame.RGB[i+2] = v, v, v
		}
	}
}

// correctSSTVColors uses conservative per-channel levels. It avoids stretching
// a nearly uniform image, which keeps weak SSTV transmissions natural instead
// of creating exaggerated colour noise.
func correctSSTVColors(frame *Frame) {
	if frame == nil || len(frame.RGB) == 0 {
		return
	}
	low := [3]byte{255, 255, 255}
	high := [3]byte{}
	for i := 0; i+2 < len(frame.RGB); i += 3 {
		for channel := 0; channel < 3; channel++ {
			value := frame.RGB[i+channel]
			if value < low[channel] {
				low[channel] = value
			}
			if value > high[channel] {
				high[channel] = value
			}
		}
	}
	for i := 0; i+2 < len(frame.RGB); i += 3 {
		for channel := 0; channel < 3; channel++ {
			rangeValue := int(high[channel]) - int(low[channel])
			if rangeValue < 72 {
				continue
			}
			value := (int(frame.RGB[i+channel]) - int(low[channel])) * 255 / rangeValue
			frame.RGB[i+channel] = byte(min(max(value, 0), 255))
		}
	}
}
