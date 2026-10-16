//go:build ignore

// Generates the compact flag atlas embedded by the FT8 activity viewer.
// Run from the repository root with: go run scripts/generate_flag_atlas.go
package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/biter777/countries"
)

const (
	columns = 16
	cellW   = 24
	cellH   = 18
)

func main() {
	codes := countries.All()
	sort.Slice(codes, func(i, j int) bool { return codes[i].Alpha2() < codes[j].Alpha2() })
	atlas := image.NewNRGBA(image.Rect(0, 0, columns*cellW, ((len(codes)+columns-1)/columns)*cellH))
	client := &http.Client{Timeout: 15 * time.Second}
	for index, code := range codes {
		alpha2 := strings.ToLower(code.Alpha2())
		response, err := client.Get("https://flagcdn.com/w20/" + alpha2 + ".png")
		if err != nil {
			panic(err)
		}
		flag, decodeErr := png.Decode(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || decodeErr != nil {
			fmt.Printf("skip %s: HTTP %d, %v\n", alpha2, response.StatusCode, decodeErr)
			continue
		}
		x := (index%columns)*cellW + (cellW-flag.Bounds().Dx())/2
		y := (index/columns)*cellH + (cellH-flag.Bounds().Dy())/2
		draw.Draw(atlas, image.Rect(x, y, x+flag.Bounds().Dx(), y+flag.Bounds().Dy()), flag, flag.Bounds().Min, draw.Src)
	}
	file, err := os.Create("screens/assets/flags/atlas.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if err := png.Encode(file, atlas); err != nil {
		panic(err)
	}
	fmt.Printf("generated %d flags\n", len(codes))
}
