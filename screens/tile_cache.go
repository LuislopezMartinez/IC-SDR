package screens

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/resources"
)

type tileKey struct{ z, x, y int }

var (
	tilesMu         sync.Mutex
	tileMemory      = map[tileKey]rl.Texture2D{}
	tilePending     = map[tileKey]bool{}
	tileBytes       = map[tileKey][]byte{}
	worldTex        rl.Texture2D
	httpTiles       = &http.Client{Timeout: 8 * time.Second}
	tileFetchTokens = make(chan struct{}, 6)
)

func worldMapTexture() rl.Texture2D {
	if worldTex.ID != 0 {
		return worldTex
	}
	img := rl.LoadImageFromMemory(".png", aisWorldPNG, int32(len(aisWorldPNG)))
	if img != nil && img.Data != nil {
		worldTex = rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		if worldTex.ID != 0 {
			rl.SetTextureFilter(worldTex, rl.FilterBilinear)
		}
	}
	return worldTex
}

func drawMapTiles(centerLat, centerLon, lonSpan float64, b rl.Rectangle) bool {
	z := mapZoom(lonSpan, b.Width)
	zInt := int(math.Floor(z))
	if zInt < 2 {
		zInt = 2
	}
	if zInt > 18 {
		zInt = 18
	}
	n := 1 << zInt
	drawScale := float32(math.Exp2(z - float64(zInt)))
	tilePx := 256 * drawScale
	cx := mercatorX(centerLon, float64(zInt))
	cy := mercatorY(centerLat, float64(zInt))
	halfW := b.Width / 2
	halfH := b.Height / 2
	minX := int(math.Floor(float64(cx) - float64(halfW)/float64(tilePx)))
	maxX := int(math.Floor(float64(cx) + float64(halfW)/float64(tilePx)))
	minY := int(math.Floor(float64(cy) - float64(halfH)/float64(tilePx)))
	maxY := int(math.Floor(float64(cy) + float64(halfH)/float64(tilePx)))
	drew := false
	drawn := 0
	for x := minX; x <= maxX; x++ {
		tx := ((x % n) + n) % n
		for y := minY; y <= maxY; y++ {
			if y < 0 || y >= n {
				continue
			}
			dest := rl.Rectangle{
				X:      b.X + halfW + (float32(x)-float32(cx))*tilePx,
				Y:      b.Y + halfH + (float32(y)-float32(cy))*tilePx,
				Width:  tilePx + 0.75,
				Height: tilePx + 0.75,
			}
			if !rl.CheckCollisionRecs(dest, b) {
				continue
			}
			tex, ok := tileTexture(zInt, tx, y)
			if !ok {
				continue
			}
			if tex.Width <= 0 || tex.Height <= 0 {
				continue
			}
			rl.DrawTexturePro(tex, rl.Rectangle{Width: float32(tex.Width), Height: float32(tex.Height)}, dest, rl.Vector2{}, 0, rl.White)
			drew = true
			drawn++
			if drawn >= 48 {
				return drew
			}
		}
	}
	return drew
}

func tileTexture(z, x, y int) (rl.Texture2D, bool) {
	key := tileKey{z, x, y}
	tilesMu.Lock()
	if tex, ok := tileMemory[key]; ok {
		tilesMu.Unlock()
		return tex, tex.ID != 0
	}
	if data, ok := tileBytes[key]; ok {
		delete(tileBytes, key)
		tilesMu.Unlock()
		if !isPNG(data) {
			requestTile(key)
			return rl.Texture2D{}, false
		}
		img := rl.LoadImageFromMemory(".png", data, int32(len(data)))
		if img == nil || img.Data == nil {
			requestTile(key)
			return rl.Texture2D{}, false
		}
		tex := rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		if tex.ID != 0 {
			rl.SetTextureFilter(tex, rl.FilterBilinear)
		}
		tilesMu.Lock()
		tileMemory[key] = tex
		tilesMu.Unlock()
		return tex, tex.ID != 0
	}
	pending := tilePending[key]
	tilesMu.Unlock()
	if !pending {
		requestTile(key)
	}
	return rl.Texture2D{}, false
}

func requestTile(key tileKey) {
	tilesMu.Lock()
	if tilePending[key] {
		tilesMu.Unlock()
		return
	}
	tilePending[key] = true
	tilesMu.Unlock()
	go fetchTile(key)
}

func fetchTile(key tileKey) {
	defer func() {
		tilesMu.Lock()
		delete(tilePending, key)
		tilesMu.Unlock()
	}()
	select {
	case tileFetchTokens <- struct{}{}:
		defer func() { <-tileFetchTokens }()
	default:
		return
	}
	if data, err := os.ReadFile(tilePath(key)); err == nil && isPNG(data) {
		storeTileBytes(key, data)
		return
	}
	url := fmt.Sprintf("https://basemaps.cartocdn.com/rastertiles/voyager/%d/%d/%d.png", key.z, key.x, key.y)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "IC-SDR/0.5 (+https://github.com/blkph0x/IC-SDR)")
	resp, err := httpTiles.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || !isPNG(data) {
		return
	}
	_ = os.MkdirAll(filepath.Dir(tilePath(key)), 0o755)
	_ = os.WriteFile(tilePath(key), data, 0o644)
	storeTileBytes(key, data)
}

func storeTileBytes(key tileKey, data []byte) {
	tilesMu.Lock()
	tileBytes[key] = data
	tilesMu.Unlock()
}

func tilePath(key tileKey) string {
	return resources.WritablePath("cache", "map-tiles", fmt.Sprintf("%d", key.z), fmt.Sprintf("%d", key.x), fmt.Sprintf("%d.png", key.y))
}

func isPNG(data []byte) bool {
	return len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47 &&
		data[4] == 0x0d && data[5] == 0x0a && data[6] == 0x1a && data[7] == 0x0a
}
