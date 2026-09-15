package screens

import (
	"go-zero/internal/i18n"

	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net"
	"net/http"
	"sync"
	"time"
)

//go:embed web/index.html
var webFiles embed.FS

type webMemory struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
	FrequencyHz int64  `json:"frequencyHz"`
	Mode        string `json:"mode"`
	FilterHz    int    `json:"filterHz"`
	StepHz      int64  `json:"stepHz"`
	ScanEnabled bool   `json:"scanEnabled"`
	Priority    bool   `json:"priority"`
	CTCSSHz     string `json:"ctcssHz"`
	DCSCode     string `json:"dcsCode"`
	Color       string `json:"color"`
}

type webMemoryList struct {
	Groups        []string    `json:"groups"`
	SelectedGroup string      `json:"selectedGroup"`
	Selected      int         `json:"selected"`
	OnlyActive    bool        `json:"onlyActive"`
	Memories      []webMemory `json:"memories"`
}

type webMemoryMarker struct {
	Name        string `json:"name"`
	FrequencyHz int64  `json:"frequencyHz"`
	Color       string `json:"color"`
	ScanEnabled bool   `json:"scanEnabled"`
}

type webFilterOption struct {
	Index       int    `json:"index"`
	Label       string `json:"label"`
	BandwidthHz int    `json:"bandwidthHz"`
	MinimumHz   int    `json:"minimumHz,omitempty"`
	MaximumHz   int    `json:"maximumHz,omitempty"`
	StepHz      int    `json:"stepHz,omitempty"`
}

type webSnapshot struct {
	Online            bool              `json:"online"`
	FrequencyHz       int64             `json:"frequencyHz"`
	CenterHz          int64             `json:"centerHz"`
	SpanHz            int64             `json:"spanHz"`
	StepHz            int64             `json:"stepHz"`
	Mode              string            `json:"mode"`
	FilterIndex       int               `json:"filterIndex"`
	FilterOptions     []webFilterOption `json:"filterOptions"`
	BandwidthHz       int               `json:"bandwidthHz"`
	Fixed             bool              `json:"fixed"`
	SignalDBm         float32           `json:"signalDBm"`
	SMeterDBm         float32           `json:"sMeterDBm"`
	SMeterPeakDBm     float32           `json:"sMeterPeakDBm"`
	SquelchEnabled    bool              `json:"squelchEnabled"`
	SquelchOpen       bool              `json:"squelchOpen"`
	SquelchDBm        int               `json:"squelchDBm"`
	Tool              string            `json:"tool"`
	ScannerRunning    bool              `json:"scannerRunning"`
	Scanner           *webScanner       `json:"scanner,omitempty"`
	RecorderRecording bool              `json:"recorderRecording"`
	RecorderPaused    bool              `json:"recorderPaused"`
	MemoryView        bool              `json:"memoryView"`
	MemoryMarkers     []webMemoryMarker `json:"memoryMarkers"`
	Spectrum          []int16           `json:"spectrum"`
	SpectrumMin       float32           `json:"spectrumMin"`
	SpectrumMax       float32           `json:"spectrumMax"`
	TETRA             *webTETRA         `json:"tetra,omitempty"`
	DMR               *webDMR           `json:"dmr,omitempty"`
}

type webDMR struct {
	State       string `json:"state"`
	Detail      string `json:"detail"`
	Slot1       string `json:"slot1"`
	Slot2       string `json:"slot2"`
	AudioSlot   string `json:"audioSlot"`
	ColorCode   int    `json:"colorCode"`
	PLLLocked   bool   `json:"pllLocked"`
	SyncQuality int    `json:"syncQuality"`
}

type webTETRA struct {
	State            string  `json:"state"`
	Quality          float32 `json:"quality"`
	LevelDBFS        float32 `json:"levelDBFS"`
	FrequencyErrorHz float32 `json:"frequencyErrorHz"`
	CMCEEvents       uint64  `json:"cmceEvents"`
	AudioFrames      uint64  `json:"audioFrames"`
	Enabled          bool    `json:"enabled"`
	AutoCenter       bool    `json:"autoCenter"`
	ClearOnly        bool    `json:"clearOnly"`
	SlotTraffic      [4]int8 `json:"slotTraffic"`
	SlotEncrypted    [4]int8 `json:"slotEncrypted"`
}

type webScanner struct {
	Status         string `json:"status"`
	MinimumHz      int64  `json:"minimumHz"`
	MaximumHz      int64  `json:"maximumHz"`
	OverlayVisible bool   `json:"overlayVisible"`
	CenterToMemory bool   `json:"centerToMemory"`
	Resume         string `json:"resume"`
	DwellMs        int    `json:"dwellMs"`
	Policy         string `json:"policy"`
}

// WebServer only publishes a read-only, bounded copy of state and audio.
// It never opens the SDR device or calls a tuner/control method.
type WebServer struct {
	server             *http.Server
	listener           net.Listener
	password           string
	passwordSalt       []byte
	passwordHash       []byte
	controlSalt        []byte
	controlHash        []byte
	controlToken       string
	controlUntil       time.Time
	controlCommands    chan webControlCommand
	sessionToken       string
	mu                 sync.RWMutex
	snapshot           []byte
	memorySnapshot     []byte
	listeners          map[chan []byte]struct{}
	opusListeners      map[chan []byte]struct{}
	opusQueue          chan []float32
	opusStop           chan struct{}
	opusDone           chan struct{}
	opus               *opusEncoder
	closeOnce          sync.Once
	lastSnapshot       time.Time
	lastMemorySnapshot time.Time
}

// StartWebServer enables LAN access only with an explicitly configured password.
func StartWebServer(screen *MainScreen, address, password string) (*WebServer, error) {
	if password == "" {
		return nil, errors.New(i18n.Source("text.e4faa502f6e3"))
	}
	return startWebServer(screen, address, password, nil, nil)
}

func StartWebServerWithHash(screen *MainScreen, address string, salt, hash []byte) (*WebServer, error) {
	if len(salt) < 16 || len(hash) != 32 {
		return nil, errors.New(i18n.Source("text.69680b9bf1d5"))
	}
	return startWebServer(screen, address, "", salt, hash)
}

func startWebServer(screen *MainScreen, address, password string, salt, hash []byte) (*WebServer, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	service := &WebServer{listener: listener, password: password, passwordSalt: salt, passwordHash: hash, controlCommands: make(chan webControlCommand, 32), listeners: make(map[chan []byte]struct{}), opusListeners: make(map[chan []byte]struct{})}
	service.controlSalt, service.controlHash, _ = screen.webConfig.controlCredential()
	if encoder, encodeErr := newOpusEncoder(); encodeErr == nil {
		service.opus = encoder
		service.opusQueue = make(chan []float32, 32)
		service.opusStop = make(chan struct{})
		service.opusDone = make(chan struct{})
		go service.encodeAudio()
	} else {
		log.Printf(i18n.Source("text.ec483ce096d2"), encodeErr)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		listener.Close()
		return nil, err
	}
	service.sessionToken = hex.EncodeToString(token)
	mux := http.NewServeMux()
	mux.HandleFunc(i18n.Source("text.c767025d0edc"), service.page)
	mux.HandleFunc(i18n.Source("text.f4f696b992f6"), service.state)
	mux.HandleFunc(i18n.Source("text.7c707f47507f"), service.memories)
	mux.HandleFunc(i18n.Source("text.a4ceb9bda2e9"), service.audio)
	mux.HandleFunc(i18n.Source("text.94f6ebc47392"), service.audioOpus)
	mux.HandleFunc(i18n.Source("text.35a9ff7068f0"), service.audioFormats)
	mux.HandleFunc(i18n.Source("text.df5124adde68"), service.controlLogin)
	mux.HandleFunc(i18n.Source("text.49563974a835"), service.controlLogout)
	mux.HandleFunc(i18n.Source("text.ce296ffe10dd"), service.controlStatus)
	mux.HandleFunc(i18n.Source("text.2aafbcf56910"), service.controlAction)
	service.server = &http.Server{Handler: service.authorize(mux), ReadHeaderTimeout: 5 * time.Second}
	screen.webServer = service
	screen.audioPlayer.SetWebAudioSink(service.PublishAudio)
	go func() {
		if err := service.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf(i18n.Source("text.5fa445bb718b"), err)
		}
	}()
	return service, nil
}

func (service *WebServer) Close() error {
	var err error
	service.closeOnce.Do(func() {
		if service.server != nil {
			err = service.server.Close()
		}
		if service.opusStop != nil {
			close(service.opusStop)
			<-service.opusDone
			service.opus.Close()
		}
	})
	return err
}

func (service *WebServer) ListenerCount() int {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return len(service.listeners) + len(service.opusListeners)
}

func (service *WebServer) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("gozero_session"); err == nil && service.sessionToken != "" && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(service.sessionToken)) == 1 {
			service.writeSecurityHeaders(w)
			next.ServeHTTP(w, r)
			return
		}
		_, supplied, ok := r.BasicAuth()
		valid := false
		if ok {
			if len(service.passwordHash) == 32 {
				derived, err := pbkdf2.Key(sha256.New, supplied, service.passwordSalt, 60000, 32)
				valid = err == nil && subtle.ConstantTimeCompare(derived, service.passwordHash) == 1
			} else {
				valid = subtle.ConstantTimeCompare([]byte(supplied), []byte(service.password)) == 1
			}
		}
		if !valid {
			w.Header().Set("WWW-Authenticate", i18n.Source("text.7c43ac9473a2"))
			http.Error(w, i18n.Source("text.7c0d7d7d6042"), http.StatusUnauthorized)
			return
		}
		if service.sessionToken != "" {
			http.SetCookie(w, &http.Cookie{Name: "gozero_session", Value: service.sessionToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		}
		service.writeSecurityHeaders(w)
		next.ServeHTTP(w, r)
	})
}

func (service *WebServer) writeSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", i18n.Source("text.342caa2dcca5"))
}

func (service *WebServer) controlLogin(w http.ResponseWriter, r *http.Request) {
	if len(service.controlHash) != 32 {
		http.Error(w, i18n.Source("text.acfc6ba6127a"), http.StatusConflict)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var request struct {
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		http.Error(w, i18n.Source("text.34585d6f2f81"), http.StatusBadRequest)
		return
	}
	derived, err := pbkdf2.Key(sha256.New, request.Password, service.controlSalt, 60000, 32)
	if err != nil || subtle.ConstantTimeCompare(derived, service.controlHash) != 1 {
		http.Error(w, i18n.Source("text.306486a14e83"), http.StatusUnauthorized)
		return
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		http.Error(w, i18n.Source("text.190877e714b2"), http.StatusInternalServerError)
		return
	}
	service.mu.Lock()
	if service.controlToken != "" && time.Now().Before(service.controlUntil) {
		service.mu.Unlock()
		http.Error(w, i18n.Source("text.b7869df40e79"), http.StatusConflict)
		return
	}
	service.controlToken = hex.EncodeToString(token)
	service.controlUntil = time.Now().Add(remoteControlLease)
	controlToken := service.controlToken
	service.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "icsdr_control", Value: controlToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (service *WebServer) controlStatus(w http.ResponseWriter, r *http.Request) {
	service.mu.Lock()
	active := service.controlToken != "" && time.Now().Before(service.controlUntil)
	owner := false
	if cookie, err := r.Cookie("icsdr_control"); err == nil && active {
		owner = subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(service.controlToken)) == 1
	}
	if owner {
		service.controlUntil = time.Now().Add(remoteControlLease)
	}
	configured := len(service.controlHash) == 32
	service.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"configured": configured, "active": active, "owner": owner})
}

func (service *WebServer) controlLogout(w http.ResponseWriter, r *http.Request) {
	service.mu.Lock()
	if cookie, err := r.Cookie("icsdr_control"); err == nil && service.controlToken != "" && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(service.controlToken)) == 1 {
		service.controlToken = ""
		service.controlUntil = time.Time{}
	}
	service.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "icsdr_control", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (service *WebServer) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, i18n.Source("text.0e64c0bd03c3"), 500)
		return
	}
	w.Header().Set("Content-Type", i18n.Source("text.5e30ae15588f"))
	_, _ = w.Write(data)
}

func (service *WebServer) state(w http.ResponseWriter, r *http.Request) {
	service.mu.RLock()
	data := service.snapshot
	service.mu.RUnlock()
	if len(data) == 0 {
		data = []byte(`{"online":false}`)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func (service *WebServer) memories(w http.ResponseWriter, r *http.Request) {
	service.mu.RLock()
	data := service.memorySnapshot
	service.mu.RUnlock()
	if len(data) == 0 {
		data = []byte(`{"groups":["TODAS"],"selectedGroup":"TODAS","selected":-1,"memories":[]}`)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func (service *WebServer) publishMemories(screen *MainScreen) {
	if time.Since(service.lastMemorySnapshot) < 500*time.Millisecond {
		return
	}
	list := webMemoryList{Groups: []string{i18n.Source("text.201f15dab8b3")}, SelectedGroup: i18n.Source("text.201f15dab8b3"), Selected: -1, Memories: []webMemory{}}
	if panel := screen.memoryPanel; panel != nil {
		list.Groups = append([]string(nil), panel.groups...)
		if len(list.Groups) == 0 {
			list.Groups = []string{i18n.Source("text.201f15dab8b3")}
		}
		list.SelectedGroup, list.Selected, list.OnlyActive = panel.selectedGroup, panel.selected, panel.onlyActive
		for _, memory := range panel.memories {
			list.Memories = append(list.Memories, webMemory{
				Name: memory.Name, Description: memory.Description, Group: memoryGroup(memory), FrequencyHz: memory.FrequencyHz,
				Mode: memory.Mode, FilterHz: memory.FilterBandwidthHz, StepHz: memory.StepHz,
				ScanEnabled: memory.ScanEnabled, Priority: memory.Priority, CTCSSHz: memory.CTCSSHz, DCSCode: memory.DCSCode,
				Color: colorHex(panel.groupColor(memoryGroup(memory))),
			})
		}
	}
	data, err := json.Marshal(list)
	if err != nil {
		return
	}
	service.mu.Lock()
	service.memorySnapshot = data
	service.mu.Unlock()
	service.lastMemorySnapshot = time.Now()
}

// PublishAudio is called from raylib's audio callback. Slow listeners drop
// frames instead of blocking the local playback thread.
func (service *WebServer) PublishAudio(samples []float32) {
	if len(samples) == 0 {
		return
	}
	service.mu.RLock()
	if len(service.listeners) == 0 && len(service.opusListeners) == 0 {
		service.mu.RUnlock()
		return
	}
	if len(service.listeners) > 0 {
		pcm := make([]byte, len(samples)*2)
		for i, sample := range samples {
			value := int16(math.Round(float64(max(-1, min(1, sample)) * 32767)))
			binary.LittleEndian.PutUint16(pcm[2*i:], uint16(value))
		}
		for channel := range service.listeners {
			offerLatestAudio(channel, pcm)
		}
	}
	if len(service.opusListeners) > 0 && service.opusQueue != nil {
		copySamples := append([]float32(nil), samples...)
		select {
		case service.opusQueue <- copySamples:
		default:
		}
	}
	service.mu.RUnlock()
}

// Keep a slow PCM listener near the live edge instead of making it replay an
// ever-growing queue of stale audio. Called while service.mu is read-locked.
func offerLatestAudio(channel chan []byte, data []byte) {
	select {
	case channel <- data:
		return
	default:
	}
	select {
	case <-channel:
	default:
	}
	select {
	case channel <- data:
	default:
	}
}

func (service *WebServer) encodeAudio() {
	defer close(service.opusDone)
	frame := make([]float32, 0, 1920)
	for {
		select {
		case <-service.opusStop:
			return
		case samples := <-service.opusQueue:
			frame = append(frame, samples...)
			for len(frame) >= 960 {
				packet, err := service.opus.Encode(frame[:960])
				frame = frame[960:]
				if err != nil {
					continue
				}
				service.mu.RLock()
				for channel := range service.opusListeners {
					select {
					case channel <- packet:
					default:
					}
				}
				service.mu.RUnlock()
			}
		}
	}
}

func (service *WebServer) audioFormats(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if service.opus != nil {
		_, _ = w.Write([]byte(`{"opus":true}`))
	} else {
		_, _ = w.Write([]byte(`{"opus":false}`))
	}
}

func (service *WebServer) audioOpus(w http.ResponseWriter, r *http.Request) {
	if service.opus == nil {
		http.Error(w, i18n.Source("text.0446c422e054"), http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, i18n.Source("text.3953068f2132"), 500)
		return
	}
	channel := make(chan []byte, 24)
	service.mu.Lock()
	service.opusListeners[channel] = struct{}{}
	service.mu.Unlock()
	defer func() { service.mu.Lock(); delete(service.opusListeners, channel); service.mu.Unlock() }()
	serialBytes := make([]byte, 4)
	if _, err := rand.Read(serialBytes); err != nil {
		http.Error(w, i18n.Source("text.9f0dd7c0e3aa"), 500)
		return
	}
	stream := oggOpusStream{serial: binary.LittleEndian.Uint32(serialBytes), preSkip: service.opus.preSkip}
	w.Header().Set("Content-Type", i18n.Source("text.2b7cf730cdeb"))
	w.Header().Set("X-Accel-Buffering", "no")
	head, tags := stream.headers()
	if _, err := w.Write(head); err != nil {
		return
	}
	if _, err := w.Write(tags); err != nil {
		return
	}
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case packet := <-channel:
			if _, err := w.Write(stream.audio(packet)); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (service *WebServer) audio(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, i18n.Source("text.3953068f2132"), 500)
		return
	}
	// Twenty-four 25 ms device frames absorb brief Wi-Fi/HTTP scheduling pauses.
	channel := make(chan []byte, 24)
	service.mu.Lock()
	service.listeners[channel] = struct{}{}
	service.mu.Unlock()
	defer func() { service.mu.Lock(); delete(service.listeners, channel); service.mu.Unlock() }()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Audio-Format", "s16le;rate=48000;channels=1")
	for {
		select {
		case <-r.Context().Done():
			return
		case pcm := <-channel:
			if _, err := w.Write(pcm); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (service *WebServer) PublishScreen(screen *MainScreen) {
	if time.Since(service.lastSnapshot) < 50*time.Millisecond {
		return
	}
	service.lastSnapshot = time.Now()
	service.publishMemories(screen)
	mode := ""
	if screen.mode != nil {
		mode = screen.mode.SelectedText()
	}
	state := webSnapshot{
		Online:      screen.stats.FFTBlocks > 0 && screen.stats.Status != i18n.Source("text.d98ee0e5f939"),
		FrequencyHz: screen.frequencyHz, CenterHz: screen.centerFrequencyHz,
		SpanHz: screen.spanHz, StepHz: screen.tuningStepHz,
		Mode: mode, BandwidthHz: screen.demodBandwidthHz, Fixed: !screen.centerMode,
		SignalDBm: screen.stats.SignalDBm, SquelchEnabled: screen.squelchEnabled,
		SquelchOpen: screen.stats.SquelchOpen, SquelchDBm: screen.squelchThreshold,
		Tool: screen.activeTool, SpectrumMin: screen.spectrumMinimumDB,
		SpectrumMax: screen.spectrumMaximumDB,
		Spectrum:    make([]int16, 512),
	}
	if screen.filterSelector != nil {
		state.FilterIndex = screen.filterSelector.selected[filterMode(mode)]
	}
	for index, preset := range filterCatalog[filterMode(mode)] {
		option := webFilterOption{Index: index, Label: preset.ID, BandwidthHz: preset.BandwidthHz}
		if index == 3 {
			option.MinimumHz, option.MaximumHz, option.StepHz = customFilterRange(filterMode(mode))
		}
		state.FilterOptions = append(state.FilterOptions, option)
	}
	state.SMeterDBm = calibrateSMeter(screen.stats.SignalDBm)
	state.SMeterPeakDBm = state.SMeterDBm
	if screen.sMeter != nil && screen.sMeter.initialized {
		state.SMeterDBm = screen.sMeter.displayed
		state.SMeterPeakDBm = screen.sMeter.peak
	}
	if screen.scanPanel != nil {
		p := screen.scanPanel
		state.ScannerRunning = p.running
		state.Scanner = &webScanner{
			Status: p.displayStatus(), MinimumHz: p.minimumHz, MaximumHz: p.maximumHz,
			OverlayVisible: p.overlayVisible, CenterToMemory: p.centerToMemory,
			Resume: p.resume, DwellMs: p.dwellMs, Policy: p.policy,
		}
	}
	if screen.activeTool == i18n.Source("text.f69d86a86926") && screen.receiver != nil {
		status := screen.receiver.TETRAStatus()
		state.TETRA = &webTETRA{State: status.State, Quality: status.Quality, LevelDBFS: status.LevelDBFS, FrequencyErrorHz: status.FrequencyErrorHz, CMCEEvents: status.CMCEEvents, AudioFrames: status.AudioFrames, SlotTraffic: status.SlotTraffic, SlotEncrypted: status.SlotEncrypted}
		if screen.tetraPanel != nil {
			state.TETRA.Enabled = screen.tetraPanel.enabled
			state.TETRA.AutoCenter = screen.tetraPanel.autoCenter
			state.TETRA.ClearOnly = screen.tetraPanel.clearOnly
		}
	}
	if screen.activeTool == i18n.Source("text.93239b223632") && screen.receiver != nil {
		status := screen.receiver.DMRStatus()
		state.DMR = &webDMR{State: status.State, Detail: status.Detail, Slot1: status.Slot1, Slot2: status.Slot2, AudioSlot: screen.dmrAudioSlot, ColorCode: status.ColorCode, PLLLocked: status.PLLLocked, SyncQuality: status.SyncQuality}
	}
	if screen.recorder != nil {
		record := screen.recorder.State()
		state.RecorderRecording, state.RecorderPaused = record.Recording, record.Paused
	}
	if screen.memoryPanel != nil {
		state.MemoryView = screen.memoryPanel.markersVisible
		if state.MemoryView && screen.spanHz > 0 {
			for _, index := range screen.memoryPanel.visibleMarkerIndices() {
				memory := screen.memoryPanel.memories[index]
				state.MemoryMarkers = append(state.MemoryMarkers, webMemoryMarker{
					Name: memory.Name, FrequencyHz: memory.FrequencyHz,
					Color:       colorHex(screen.memoryPanel.groupColor(memoryGroup(memory))),
					ScanEnabled: memory.ScanEnabled,
				})
			}
		}
	}
	if len(screen.spectrum) > 0 {
		for i := range state.Spectrum {
			value, visible := screen.interpolatedSpectrumValue(screen.spectrum, float32(i)/float32(len(state.Spectrum)-1))
			if !visible {
				value = state.SpectrumMin
			}
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				value = state.SpectrumMin
			}
			state.Spectrum[i] = int16(math.Round(float64(value * 10)))
		}
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	service.mu.Lock()
	service.snapshot = data
	service.mu.Unlock()
}
