package screens

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
	qrcode "github.com/skip2/go-qrcode"
)

func TestWebLANURLAndQR(t *testing.T) {
	url := webLANURL("192.168.1.50", 8080)
	if url != "http://192.168.1.50:8080/" || strings.Contains(url, "password") {
		t.Fatalf("unexpected QR URL: %q", url)
	}
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil || len(png) < 100 {
		t.Fatalf("QR generation failed: %v", err)
	}
}

func TestPublicIPLookup(t *testing.T) {
	var response atomic.Value
	response.Store("8.8.8.8\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(response.Load().(string))) }))
	defer server.Close()
	address, err := lookupPublicIPv4From(context.Background(), server.Client(), server.URL)
	if err != nil || address != "8.8.8.8" || webLANURL(address, 8080) != "http://8.8.8.8:8080/" {
		t.Fatalf("public IP = %q, %v", address, err)
	}
	for _, invalid := range []string{"192.168.1.20", "100.64.0.1", "not-an-ip", "2001:4860:4860::8888"} {
		response.Store(invalid)
		if _, err := lookupPublicIPv4From(context.Background(), server.Client(), server.URL); err == nil {
			t.Fatalf("accepted %q as public IPv4", invalid)
		}
	}
}

// WEB_PANEL_RENDER_DIR enables visual QA without starting the SDR.
func TestWebPanelRender(t *testing.T) {
	dir := os.Getenv("WEB_PANEL_RENDER_DIR")
	if dir == "" {
		t.Skip("visual QA not requested")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1600, 900, "Web panel visual QA")
	defer rl.CloseWindow()
	simpleui.SetTextScale(1.25)
	screen := &MainScreen{webConfig: webConfig{Enabled: true, Port: 8080, PasswordHash: "configured"}, webServer: &WebServer{listeners: map[chan []byte]struct{}{}, opusListeners: map[chan []byte]struct{}{}}}
	panel := NewWebPanel(screen)
	defer panel.Close()
	panel.lanIP = "192.168.1.50"
	panel.lastAddressCheck = time.Now()
	panel.SetVisible(true)
	canvas := rl.LoadRenderTexture(1600, 900)
	defer rl.UnloadRenderTexture(canvas)
	rl.BeginTextureMode(canvas)
	rl.ClearBackground(colors.background)
	drawPanel(toolContentX, toolY, toolContentRight-toolContentX, toolH)
	panel.DrawPanel()
	for _, control := range panel.controls {
		control.Draw()
	}
	rl.EndTextureMode()
	img := rl.LoadImageFromTexture(canvas.Texture)
	defer rl.UnloadImage(img)
	rl.ImageFlipVertical(img)
	if !rl.ExportImage(*img, filepath.Join(dir, "web-panel.png")) {
		t.Fatal("failed to export panel")
	}
	panel.qrMode.SetActive(true)
	panel.publicIP = "8.8.8.8"
	panel.lastPublicCheck = time.Now()
	rl.BeginTextureMode(canvas)
	rl.ClearBackground(colors.background)
	drawPanel(toolContentX, toolY, toolContentRight-toolContentX, toolH)
	panel.DrawPanel()
	for _, control := range panel.controls {
		control.Draw()
	}
	rl.EndTextureMode()
	internetImage := rl.LoadImageFromTexture(canvas.Texture)
	defer rl.UnloadImage(internetImage)
	rl.ImageFlipVertical(internetImage)
	if !rl.ExportImage(*internetImage, filepath.Join(dir, "web-panel-internet.png")) {
		t.Fatal("failed to export internet panel")
	}
}
