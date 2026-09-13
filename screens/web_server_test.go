package screens

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-zero/simpleui"
)

func TestWebSpectrumUsesSelectedSpan(t *testing.T) {
	screen := &MainScreen{
		spanHz: 250_000, spectrumMinimumDB: -120, spectrumMaximumDB: 0,
		spectrum: make([]float32, 1024),
	}
	screen.stats.SampleRate = 2_000_000
	for i := range screen.spectrum {
		screen.spectrum[i] = -100
	}
	screen.spectrum[512] = -20 // Center frequency: visible.
	screen.spectrum[768] = 0   // 500 kHz away: outside the selected span.
	service := &WebServer{}
	service.PublishScreen(screen)
	var state webSnapshot
	if err := json.Unmarshal(service.snapshot, &state); err != nil {
		t.Fatal(err)
	}
	if got := state.Spectrum[0]; got != -1000 {
		t.Fatalf("left edge = %.1f dB, want -100 dB", float64(got)/10)
	}
	if got := state.Spectrum[len(state.Spectrum)/2]; got <= -1000 || got > -200 {
		t.Fatalf("center signal is missing or misplaced: %.1f dB", float64(got)/10)
	}
	for _, value := range state.Spectrum {
		if value > -200 {
			t.Fatalf("out-of-span signal leaked into web spectrum: %.1f dB", float64(value)/10)
		}
	}
}

func TestWebSMeterUsesPCDisplayedLevelAndPeak(t *testing.T) {
	screen := &MainScreen{sMeter: &SMeter{displayed: -73, peak: -63, initialized: true}}
	service := &WebServer{}
	service.PublishScreen(screen)
	var state webSnapshot
	if err := json.Unmarshal(service.snapshot, &state); err != nil {
		t.Fatal(err)
	}
	if state.SMeterDBm != -73 || state.SMeterPeakDBm != -63 {
		t.Fatalf("web meter differs from PC: level=%v peak=%v", state.SMeterDBm, state.SMeterPeakDBm)
	}
}

func TestWebSnapshotOffersDesktopFilterPresets(t *testing.T) {
	screen := &MainScreen{mode: simpleui.NewDropdown("webFilterTestMode", 0, 0, 100, 30, "MODE", []string{"NFM"}, 12), filterSelector: NewFilterSelector(nil)}
	screen.mode.SetSelected(0)
	service := &WebServer{}
	service.PublishScreen(screen)
	var state webSnapshot
	if err := json.Unmarshal(service.snapshot, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.FilterOptions) != 4 || state.FilterOptions[1].BandwidthHz != 12500 || state.FilterIndex != 1 {
		t.Fatalf("web filters do not match NFM desktop presets: %+v", state.FilterOptions)
	}
}

func TestWebMemoryMarkersFollowPCViewAndVisibleSpan(t *testing.T) {
	screen := &MainScreen{centerFrequencyHz: 100_000_000, spanHz: 250_000}
	screen.memoryPanel = &MemoryPanel{
		screen: screen, markersVisible: true,
		memories: []MemoryEntry{
			{Name: "Fuera", FrequencyHz: 101_000_000, ScanEnabled: true},
			{Name: "Derecha", FrequencyHz: 100_100_000, ScanEnabled: false},
			{Name: "Izquierda", FrequencyHz: 99_900_000, ScanEnabled: true},
		},
	}
	service := &WebServer{}
	service.PublishScreen(screen)
	var state webSnapshot
	if err := json.Unmarshal(service.snapshot, &state); err != nil {
		t.Fatal(err)
	}
	if !state.MemoryView || len(state.MemoryMarkers) != 2 || state.MemoryMarkers[0].Name != "Izquierda" || state.MemoryMarkers[1].Name != "Derecha" {
		t.Fatalf("visible markers do not match the PC: %+v", state.MemoryMarkers)
	}
	if state.MemoryMarkers[0].Color == "" || state.MemoryMarkers[1].ScanEnabled {
		t.Fatalf("marker style not published: %+v", state.MemoryMarkers)
	}
	screen.memoryPanel.markersVisible = false
	service.lastSnapshot = time.Time{}
	service.PublishScreen(screen)
	if err := json.Unmarshal(service.snapshot, &state); err != nil {
		t.Fatal(err)
	}
	if state.MemoryView || len(state.MemoryMarkers) != 0 {
		t.Fatalf("MEM VIEW off still shows markers: %+v", state.MemoryMarkers)
	}
}

func TestWebScannerPublishesPCControlsAsState(t *testing.T) {
	screen := &MainScreen{spanHz: 250_000}
	screen.scanPanel = &ScanPanel{
		screen: screen, running: true, status: "SCANNING",
		minimumHz: 100_000_000, maximumHz: 100_200_000,
		overlayVisible: true, centerToMemory: true,
		resume: "DELAY", dwellMs: 3000, policy: "STRONGER",
	}
	service := &WebServer{}
	service.PublishScreen(screen)
	var state webSnapshot
	if err := json.Unmarshal(service.snapshot, &state); err != nil {
		t.Fatal(err)
	}
	if !state.ScannerRunning || state.Scanner == nil || state.Scanner.Status != "BUSCANDO TRANSMISIONES" ||
		state.Scanner.MinimumHz != 100_000_000 || state.Scanner.MaximumHz != 100_200_000 ||
		!state.Scanner.OverlayVisible || !state.Scanner.CenterToMemory || state.Scanner.Resume != "DELAY" ||
		state.Scanner.DwellMs != 3000 || state.Scanner.Policy != "STRONGER" {
		t.Fatalf("web scanner does not match PC: %+v", state.Scanner)
	}
}

func TestWebMemoriesEndpointReturnsFullReadOnlyList(t *testing.T) {
	screen := &MainScreen{spanHz: 250_000}
	entries := make([]MemoryEntry, 120)
	for i := range entries {
		entries[i] = MemoryEntry{Name: "Canal", Group: "PMR", FrequencyHz: 446_000_000 + int64(i)*12_500, ScanEnabled: i%2 == 0}
	}
	screen.memoryPanel = &MemoryPanel{screen: screen, memories: entries, groups: []string{"TODAS", "PMR"}, selectedGroup: "PMR", selected: 119}
	service := &WebServer{}
	service.PublishScreen(screen)
	response := httptest.NewRecorder()
	service.memories(response, httptest.NewRequest(http.MethodGet, "/api/memories", nil))
	var list webMemoryList
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || len(list.Memories) != 120 || list.SelectedGroup != "PMR" || list.Selected != 119 || len(list.Groups) != 2 {
		t.Fatalf("incomplete memory list: status=%d list=%+v", response.Code, list)
	}
	if list.Memories[0].Color == "" || !list.Memories[0].ScanEnabled {
		t.Fatalf("memory metadata missing: %+v", list.Memories[0])
	}
}

func TestWebServerRequiresPassword(t *testing.T) {
	service := &WebServer{password: "secret"}
	handler := service.authorize(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		password string
		want     int
	}{
		{"", http.StatusUnauthorized},
		{"incorrect", http.StatusUnauthorized},
		{"secret", http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.password != "" {
			req.SetBasicAuth("viewer", tc.password)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != tc.want {
			t.Fatalf("password %q: got %d, want %d", tc.password, response.Code, tc.want)
		}
	}
}

func TestConfiguredWebServerServesOnlyAuthenticatedReaders(t *testing.T) {
	config := webConfig{Enabled: true, Port: 8080}
	if err := config.setPassword("contraseña-de-prueba-larga"); err != nil {
		t.Fatal(err)
	}
	salt, hash, err := config.credential()
	if err != nil {
		t.Fatal(err)
	}
	screen := &MainScreen{audioPlayer: NewAudioPlayer(nil, nil)}
	server, err := StartWebServerWithHash(screen, "127.0.0.1:0", salt, hash)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	url := "http://" + server.listener.Addr().String() + "/api/state"
	request, _ := http.NewRequest(http.MethodGet, url, nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous status: %d", response.StatusCode)
	}
	request, _ = http.NewRequest(http.MethodGet, url, nil)
	request.SetBasicAuth("viewer", "contraseña-de-prueba-larga")
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authenticated status: %d", response.StatusCode)
	}
}

func TestWebAudioFansOutWithoutBlocking(t *testing.T) {
	a := make(chan []byte, 1)
	b := make(chan []byte, 1)
	service := &WebServer{listeners: map[chan []byte]struct{}{a: {}, b: {}}}
	service.PublishAudio([]float32{0, 1, -1})
	for _, ch := range []chan []byte{a, b} {
		pcm := <-ch
		if len(pcm) != 6 || pcm[2] != 255 || pcm[3] != 127 || pcm[4] != 1 || pcm[5] != 128 {
			t.Fatalf("unexpected PCM: %v", pcm)
		}
	}
	// A full subscriber queue must not block the local audio callback.
	a <- []byte{1}
	service.PublishAudio([]float32{.5})
	if len(b) != 1 {
		t.Fatal("second subscriber did not receive audio")
	}
}

func TestOfferLatestAudioDropsStaleFrame(t *testing.T) {
	channel := make(chan []byte, 2)
	channel <- []byte("old-1")
	channel <- []byte("old-2")
	offerLatestAudio(channel, []byte("live"))
	if string(<-channel) != "old-2" || string(<-channel) != "live" {
		t.Fatal("audio queue did not retain the latest frames")
	}
}

func TestWebPageServed(t *testing.T) {
	service := &WebServer{password: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	service.page(response, req)
	content, _ := io.ReadAll(response.Body)
	if response.Code != http.StatusOK || len(content) < 1000 || !strings.Contains(string(content), "GRABADOR · MÓVIL") || !strings.Contains(string(content), "<title>IC-SDR · Visor</title>") || !strings.Contains(string(content), `id="wakeButton"`) || !strings.Contains(string(content), `id="sqlSwitch"`) || !strings.Contains(string(content), `id="sqlRange"`) || !strings.Contains(string(content), `id="smeter"`) || !strings.Contains(string(content), `id="contactButton"`) || !strings.Contains(string(content), `id="copyContact"`) || !strings.Contains(string(content), `id="controlButton"`) || !strings.Contains(string(content), `id="demodButton"`) || !strings.Contains(string(content), `id="filterButton"`) || !strings.Contains(string(content), `id="choiceDialog"`) {
		t.Fatal("web page not served")
	}
}

func TestControlLoginIsSeparateAndExclusive(t *testing.T) {
	config := webConfig{}
	if err := config.setControlPassword("clave-control-segura"); err != nil {
		t.Fatal(err)
	}
	salt, hash, err := config.controlCredential()
	if err != nil {
		t.Fatal(err)
	}
	service := &WebServer{controlSalt: salt, controlHash: hash}
	login := func(password string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/control/login", strings.NewReader(`{"password":"`+password+`"}`))
		response := httptest.NewRecorder()
		service.controlLogin(response, req)
		return response
	}
	if got := login("clave-incorrecta").Code; got != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", got)
	}
	first := login("clave-control-segura")
	if first.Code != http.StatusNoContent || len(first.Result().Cookies()) != 1 {
		t.Fatalf("first login failed: %d", first.Code)
	}
	if got := login("clave-control-segura").Code; got != http.StatusConflict {
		t.Fatalf("second controller accepted: %d", got)
	}
	status := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/control/status", nil)
	req.AddCookie(first.Result().Cookies()[0])
	service.controlStatus(status, req)
	if !strings.Contains(status.Body.String(), `"owner":true`) {
		t.Fatalf("control owner missing: %s", status.Body.String())
	}
}

func TestControlLogoutUnlocksAndLeaseExpires(t *testing.T) {
	service := &WebServer{controlToken: "owner", controlUntil: time.Now().Add(remoteControlLease)}
	if !service.RemoteActive() {
		t.Fatal("new control session is not active")
	}
	wrong := httptest.NewRequest(http.MethodPost, "/api/control/logout", nil)
	wrong.AddCookie(&http.Cookie{Name: "icsdr_control", Value: "other"})
	service.controlLogout(httptest.NewRecorder(), wrong)
	if !service.RemoteActive() {
		t.Fatal("another client's logout released control")
	}
	owner := httptest.NewRequest(http.MethodPost, "/api/control/logout", nil)
	owner.AddCookie(&http.Cookie{Name: "icsdr_control", Value: "owner"})
	service.controlLogout(httptest.NewRecorder(), owner)
	if service.RemoteActive() {
		t.Fatal("owner logout did not release control")
	}
	service.controlToken = "owner"
	service.controlUntil = time.Now().Add(-time.Millisecond)
	if service.RemoteActive() {
		t.Fatal("expired control lease still locks PC")
	}
}

type slowControlResponse struct {
	header  http.Header
	writing chan struct{}
	release chan struct{}
}

func (response *slowControlResponse) Header() http.Header { return response.header }
func (response *slowControlResponse) WriteHeader(int)     {}
func (response *slowControlResponse) Write(data []byte) (int, error) {
	close(response.writing)
	<-response.release
	return len(data), nil
}

func TestControlStatusDoesNotHoldSnapshotLockDuringSlowResponse(t *testing.T) {
	service := &WebServer{controlHash: make([]byte, 32), controlToken: "owner", controlUntil: time.Now().Add(time.Minute)}
	response := &slowControlResponse{header: make(http.Header), writing: make(chan struct{}), release: make(chan struct{})}
	req := httptest.NewRequest(http.MethodGet, "/api/control/status", nil)
	req.AddCookie(&http.Cookie{Name: "icsdr_control", Value: "owner"})
	done := make(chan struct{})
	go func() { service.controlStatus(response, req); close(done) }()
	select {
	case <-response.writing:
	case <-time.After(time.Second):
		t.Fatal("status response did not start")
	}
	if !service.mu.TryLock() {
		close(response.release)
		<-done
		t.Fatal("slow client holds the FFT snapshot lock")
	}
	service.mu.Unlock()
	close(response.release)
	<-done
}
