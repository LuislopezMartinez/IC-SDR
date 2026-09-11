package tetra

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	DefaultFrequencyHz int64 = 391_662_724
	outputRate               = 72_000.0
	symbolRate               = 18_000.0
)

type Point struct{ I, Q float32 }

type Status struct {
	Running                                                bool
	State                                                  string
	InputRate, OutputRate, SymbolRate                      float64
	LevelDBFS, Quality, FrequencyErrorHz                   float32
	BER, FER                                               float32
	TimingPhase                                            uint8
	TimingError                                            float32
	Symbols, SyncHits, NormalBursts, BSCHFailures, Dropped uint64
	LastSync                                               time.Time
	System                                                 SystemInfo
	Constellation                                          []Point
	SlotBursts                                             [4]uint64
	FramedBursts                                           uint64
	SCHValid, SCHCRCFailures, MACResources                 uint64
	MACRejected, MACChannelAlloc, MACEncrypted             uint64
	LLCRejected, LLCNonCMCE, CMCEEvents                    uint64
	LLCTypes                                               [16]uint64
	MLEProtocols                                           [8]uint64
	LLCFragments, LLCReassembled                           uint64
	MACPDUTypes                                            [4]uint64
	Network                                                NetworkInfo
	SlotEncrypted                                          [4]int8
	SlotTraffic                                            [4]int8
	AACHValid, AACHRejected                                uint64
	ListenSlot, ActiveAudioSlot                            int8
	ClearAudioOnly                                         bool
	AudioFrames                                            uint64
	LastAudio                                              time.Time
	VoiceCodecReady                                        bool
	VoiceCodecError                                        string
	LastAudioDecision                                      string
	AudioRejectedEncrypted, AudioRejectedUnselected        uint64
	AudioRejectedInactive, AudioRejectedDamaged            uint64
}

type LiveSnapshot struct {
	Updated     time.Time   `json:"updated"`
	FrequencyHz int64       `json:"frequencyHz"`
	Status      Status      `json:"status"`
	Groups      []Group     `json:"groups"`
	Users       []User      `json:"users"`
	Messages    []Message   `json:"messages"`
	Positions   []Position  `json:"positions"`
	Calls       []Call      `json:"calls"`
	Neighbours  []Neighbour `json:"neighbours"`
}
type Group struct {
	ID        uint32    `json:"id"`
	Name      string    `json:"name,omitempty"`
	LastSeen  time.Time `json:"lastSeen"`
	Calls     uint64    `json:"calls"`
	LastEvent string    `json:"lastEvent,omitempty"`
}
type User struct {
	SSI         uint32    `json:"ssi"`
	AddressType uint8     `json:"addressType"`
	Slot        uint8     `json:"slot"`
	Encrypted   bool      `json:"encrypted"`
	LastSeen    time.Time `json:"lastSeen"`
	Seen        uint64    `json:"seen"`
}
type Message struct {
	Time       time.Time `json:"time"`
	Kind, Text string    `json:"kind"`
}
type Position struct {
	SSI       uint32    `json:"ssi"`
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lon"`
	Altitude  float64   `json:"alt,omitempty"`
	SpeedKmh  float64   `json:"speedKmh,omitempty"`
	Heading   float64   `json:"heading,omitempty"`
	AccuracyM float64   `json:"accuracyM,omitempty"`
	AgeCode   uint8     `json:"ageCode,omitempty"`
	Time      time.Time `json:"time"`
}
type Call struct {
	ID          uint16    `json:"id"`
	SSI         uint32    `json:"ssi"`
	UsageMarker uint8     `json:"usageMarker,omitempty"`
	Slot        uint8     `json:"slot,omitempty"`
	Encrypted   bool      `json:"encrypted"`
	Active      bool      `json:"active"`
	State       string    `json:"state"`
	LastSeen    time.Time `json:"lastSeen"`
	Events      uint64    `json:"events"`
}
type Neighbour struct {
	CellID       uint8     `json:"cellId"`
	Carrier      uint32    `json:"carrier"`
	FrequencyHz  int64     `json:"frequencyHz"`
	MCC          uint16    `json:"mcc"`
	MNC          uint16    `json:"mnc"`
	LocationArea uint16    `json:"locationArea"`
	Synchronized bool      `json:"synchronized"`
	ServiceLevel uint8     `json:"serviceLevel"`
	Priority     bool      `json:"priority"`
	Voice        bool      `json:"voice"`
	LastSeen     time.Time `json:"lastSeen"`
}

// Decoder is a streaming pi/4-DQPSK front end. It deliberately owns no SDR:
// the application's receiver fans its CF32 stream into this decoder.
type Decoder struct {
	mu                                                     sync.RWMutex
	rate                                                   float64
	queue                                                  chan []float32
	stop                                                   chan struct{}
	done                                                   chan struct{}
	running                                                bool
	state                                                  string
	dropped, symbols, syncHits, normalBursts, bschFailures uint64
	level, quality, freqErr                                float32
	timingPhase                                            uint8
	timingError                                            float32
	points                                                 []Point
	lastSync                                               time.Time
	system                                                 SystemInfo
	tuningOffsetHz, afcHz                                  float64
	slotBursts                                             [4]uint64
	framedBursts, totalBits, lastSyncBit                   uint64
	schValid, schCRCFailures, macResources                 uint64
	macRejected, macChannelAlloc, macEncrypted             uint64
	llcRejected, llcNonCMCE, cmceEvents                    uint64
	users                                                  map[uint32]User
	groups                                                 map[uint32]Group
	calls                                                  map[uint16]Call
	neighbours                                             map[uint8]Neighbour
	positions                                              map[uint32]Position
	messages                                               []Message
	llcTypes                                               [16]uint64
	mleProtocols                                           [8]uint64
	llcFragments, llcReassembled                           uint64
	fragments                                              map[uint32]llcFragment
	macPDUTypes                                            [4]uint64
	network                                                NetworkInfo
	slotEncrypted                                          [4]int8
	slotEncryptionSeen                                     [4]time.Time
	usageEncrypted                                         [64]int8
	usageEncryptionSeen                                    [64]time.Time
	usageCall                                              [64]uint16
	slotTraffic                                            [4]int8
	slotTrafficSeen                                        [4]time.Time
	aachValid, aachRejected                                uint64
	listenSlot, activeAudioSlot                            int8
	activeCallID                                           uint16
	clearAudioOnly                                         bool
	activeAudioSeen                                        time.Time
	audioFrames                                            uint64
	lastAudio                                              time.Time
	channelErrors, channelBits, channelFrames, channelBad  uint64
	lastAudioDecision                                      string
	audioRejectedEncrypted, audioRejectedUnselected        uint64
	audioRejectedInactive, audioRejectedDamaged            uint64
	voice                                                  [4]*voiceDecoder
	onAudio                                                func([]float32)
}

type llcFragment struct {
	NS, LastSS uint8
	Bits       []byte
	Updated    time.Time
}

func New(inputRate float64, codecPath string, onAudio func([]float32)) *Decoder {
	if inputRate <= 0 {
		inputRate = 2_048_000
	}
	d := &Decoder{rate: inputRate, state: "STOPPED", users: make(map[uint32]User), groups: make(map[uint32]Group), calls: make(map[uint16]Call), neighbours: make(map[uint8]Neighbour), positions: make(map[uint32]Position), fragments: make(map[uint32]llcFragment), onAudio: onAudio}
	for slot := range d.voice {
		path := codecPath
		if slot > 0 && codecPath != "" {
			ext := filepath.Ext(codecPath)
			path = codecPath[:len(codecPath)-len(ext)] + fmt.Sprintf("-ts%d%s", slot+1, ext)
		}
		d.voice[slot] = newVoiceDecoder(path)
	}
	d.slotEncrypted = [4]int8{-1, -1, -1, -1}
	for i := range d.usageEncrypted {
		d.usageEncrypted[i] = -1
	}
	d.slotTraffic = [4]int8{-1, -1, -1, -1}
	d.slotEncryptionSeen = [4]time.Time{}
	d.activeAudioSlot, d.activeAudioSeen = 0, time.Time{}
	d.activeCallID = 0
	d.audioFrames, d.lastAudio = 0, time.Time{}
	d.channelErrors, d.channelBits, d.channelFrames, d.channelBad = 0, 0, 0, 0
	d.lastAudioDecision = "WAITING FOR TRAFFIC"
	d.audioRejectedEncrypted, d.audioRejectedUnselected = 0, 0
	d.audioRejectedInactive, d.audioRejectedDamaged = 0, 0
	d.clearAudioOnly = true
	return d
}

// SetAudioPolicy selects 0 for automatic listening or 1..4 for a fixed
// timeslot. The policy is consumed by the TCH/ACELP audio stage.
func (d *Decoder) SetAudioPolicy(slot int, clearOnly bool) {
	if slot < 0 || slot > 4 {
		slot = 0
	}
	d.mu.Lock()
	d.listenSlot, d.clearAudioOnly = int8(slot), clearOnly
	d.activeAudioSlot, d.activeAudioSeen = 0, time.Time{}
	d.activeCallID = 0
	d.mu.Unlock()
}

func (d *Decoder) Configure(enabled bool) {
	d.mu.Lock()
	if enabled == d.running {
		d.mu.Unlock()
		return
	}
	if !enabled {
		stop, done := d.stop, d.done
		d.running = false
		d.state = "STOPPED"
		d.mu.Unlock()
		close(stop)
		<-done
		return
	}
	d.queue = make(chan []float32, 12)
	d.stop = make(chan struct{})
	d.done = make(chan struct{})
	d.running = true
	d.state = "SEARCHING FOR SYNC"
	d.totalBits, d.lastSyncBit = 0, 0
	queue, stop, done := d.queue, d.stop, d.done
	d.mu.Unlock()
	go d.run(queue, stop, done)
}

func (d *Decoder) Close() { d.Configure(false) }

func (d *Decoder) SetTuningOffset(offsetHz float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if math.Abs(offsetHz-d.tuningOffsetHz) > 100 {
		d.afcHz, d.freqErr = 0, 0
	}
	d.tuningOffsetHz = offsetHz
}

func (d *Decoder) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.symbols, d.syncHits, d.normalBursts, d.bschFailures, d.dropped = 0, 0, 0, 0, 0
	d.totalBits, d.lastSyncBit = 0, 0
	d.freqErr, d.afcHz = 0, 0
	d.timingPhase, d.timingError = 0, 0
	d.system = SystemInfo{}
	d.slotBursts = [4]uint64{}
	d.framedBursts = 0
	d.schValid, d.schCRCFailures, d.macResources = 0, 0, 0
	d.cmceEvents = 0
	d.macRejected, d.macChannelAlloc, d.macEncrypted = 0, 0, 0
	d.llcRejected, d.llcNonCMCE = 0, 0
	d.llcTypes, d.mleProtocols = [16]uint64{}, [8]uint64{}
	d.llcFragments, d.llcReassembled = 0, 0
	d.fragments = make(map[uint32]llcFragment)
	d.macPDUTypes, d.network = [4]uint64{}, NetworkInfo{}
	d.slotEncrypted = [4]int8{-1, -1, -1, -1}
	d.slotEncryptionSeen = [4]time.Time{}
	for i := range d.usageEncrypted {
		d.usageEncrypted[i] = -1
		d.usageEncryptionSeen[i] = time.Time{}
	}
	d.slotTraffic = [4]int8{-1, -1, -1, -1}
	d.slotTrafficSeen = [4]time.Time{}
	d.aachValid, d.aachRejected = 0, 0
	d.activeAudioSlot, d.activeAudioSeen = 0, time.Time{}
	d.activeCallID = 0
	d.audioFrames, d.lastAudio = 0, time.Time{}
	for _, voice := range d.voice {
		if voice != nil {
			voice.reset()
		}
	}
	d.users = make(map[uint32]User)
	d.groups = make(map[uint32]Group)
	d.calls = make(map[uint16]Call)
	d.neighbours = make(map[uint8]Neighbour)
	d.positions = make(map[uint32]Position)
	d.messages = nil
	d.points = nil
	d.lastSync = time.Time{}
}

func (d *Decoder) ProcessIQ(iq []float32) {
	d.mu.RLock()
	running, q := d.running, d.queue
	d.mu.RUnlock()
	if !running || len(iq) < 2 {
		return
	}
	copyIQ := append([]float32(nil), iq...)
	select {
	case q <- copyIQ:
	default:
		d.mu.Lock()
		d.dropped++
		d.mu.Unlock()
	}
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()
	points := append([]Point(nil), d.points...)
	slotEncrypted := d.slotEncrypted
	slotTraffic := d.slotTraffic
	for i, seen := range d.slotEncryptionSeen {
		if seen.IsZero() || time.Since(seen) > 15*time.Second {
			slotEncrypted[i] = -1
		}
	}
	for i, seen := range d.slotTrafficSeen {
		if seen.IsZero() || time.Since(seen) > 2*time.Second {
			slotTraffic[i] = -1
		}
	}
	activeAudioSlot := d.activeAudioSlot
	if time.Since(d.activeAudioSeen) > 5*time.Second {
		activeAudioSlot = 0
	}
	codecReady, codecError := false, "CODEC NOT CONFIGURED"
	codecReady = true
	for _, voice := range d.voice {
		if voice == nil || !voice.ready {
			codecReady = false
			if voice != nil && codecError == "" {
				codecError = voice.errText
			}
		}
	}
	ber, fer := float32(0), float32(0)
	if d.channelBits > 0 {
		ber = 100 * float32(d.channelErrors) / float32(d.channelBits)
	}
	if d.channelFrames > 0 {
		fer = 100 * float32(d.channelBad) / float32(d.channelFrames)
	}
	return Status{Running: d.running, State: d.state, InputRate: d.rate, OutputRate: outputRate, SymbolRate: symbolRate, LevelDBFS: d.level, Quality: d.quality, FrequencyErrorHz: d.freqErr, BER: ber, FER: fer, TimingPhase: d.timingPhase, TimingError: d.timingError, Symbols: d.symbols, SyncHits: d.syncHits, NormalBursts: d.normalBursts, BSCHFailures: d.bschFailures, Dropped: d.dropped, LastSync: d.lastSync, System: d.system, Constellation: points, SlotBursts: d.slotBursts, FramedBursts: d.framedBursts, SCHValid: d.schValid, SCHCRCFailures: d.schCRCFailures, MACResources: d.macResources, MACRejected: d.macRejected, MACChannelAlloc: d.macChannelAlloc, MACEncrypted: d.macEncrypted, LLCRejected: d.llcRejected, LLCNonCMCE: d.llcNonCMCE, CMCEEvents: d.cmceEvents, LLCTypes: d.llcTypes, MLEProtocols: d.mleProtocols, LLCFragments: d.llcFragments, LLCReassembled: d.llcReassembled, MACPDUTypes: d.macPDUTypes, Network: d.network, SlotEncrypted: slotEncrypted, SlotTraffic: slotTraffic, AACHValid: d.aachValid, AACHRejected: d.aachRejected, ListenSlot: d.listenSlot, ActiveAudioSlot: activeAudioSlot, ClearAudioOnly: d.clearAudioOnly, AudioFrames: d.audioFrames, LastAudio: d.lastAudio, VoiceCodecReady: codecReady, VoiceCodecError: codecError, LastAudioDecision: d.lastAudioDecision, AudioRejectedEncrypted: d.audioRejectedEncrypted, AudioRejectedUnselected: d.audioRejectedUnselected, AudioRejectedInactive: d.audioRejectedInactive, AudioRejectedDamaged: d.audioRejectedDamaged}
}

func (d *Decoder) Live(frequencyHz int64) LiveSnapshot {
	d.mu.RLock()
	users := make([]User, 0, len(d.users))
	for _, u := range d.users {
		users = append(users, u)
	}
	groups := make([]Group, 0, len(d.groups))
	for _, g := range d.groups {
		groups = append(groups, g)
	}
	messages := append([]Message(nil), d.messages...)
	calls := make([]Call, 0, len(d.calls))
	for _, call := range d.calls {
		if call.Active && time.Since(call.LastSeen) > 8*time.Second {
			call.Active = false
		}
		calls = append(calls, call)
	}
	neighbours := make([]Neighbour, 0, len(d.neighbours))
	for _, neighbour := range d.neighbours {
		neighbours = append(neighbours, neighbour)
	}
	positions := make([]Position, 0, len(d.positions))
	for _, position := range d.positions {
		positions = append(positions, position)
	}
	d.mu.RUnlock()
	sort.Slice(users, func(i, j int) bool { return users[i].LastSeen.After(users[j].LastSeen) })
	sort.Slice(groups, func(i, j int) bool { return groups[i].LastSeen.After(groups[j].LastSeen) })
	sort.Slice(calls, func(i, j int) bool { return calls[i].LastSeen.After(calls[j].LastSeen) })
	sort.Slice(neighbours, func(i, j int) bool { return neighbours[i].CellID < neighbours[j].CellID })
	sort.Slice(positions, func(i, j int) bool { return positions[i].Time.After(positions[j].Time) })
	return LiveSnapshot{Updated: time.Now(), FrequencyHz: frequencyHz, Status: d.Snapshot(), Groups: groups, Users: users, Messages: messages, Positions: positions, Calls: calls, Neighbours: neighbours}
}

func (d *Decoder) run(queue <-chan []float32, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	// Fractional boxcar resampler. At this first-stage detector its integration
	// also acts as the anti-alias filter before the 72 kS/s symbol front end.
	step := d.rate / outputRate
	var phase float64
	var sumI, sumQ float64
	var count int
	var agc float64 = 1
	var prevI, prevQ float64
	var havePrev bool
	var samplePhase, timingPhase, timingSamples int
	timingPhase = 3
	var timingScore [4]float64
	var timingPrevI, timingPrevQ [4]float64
	var timingHavePrev [4]bool
	for i := range timingScore {
		timingScore[i] = 1
	}
	var ringI, ringQ [4]float64
	var ringIndex, ringCount int
	var rollingI, rollingQ float64
	var phaseErrorSum, freqSum, powerSum float64
	var metricCount int
	var ncoPhase float64
	points := make([]Point, 0, 96)
	bits := make([]byte, 0, 2048)
	phases := make([]float64, 0, 64)
	var lastTrainingBit uint64
	type pendingBurst struct {
		start uint64
		slot  int
		ndb2  bool
	}
	pending := make([]pendingBurst, 0, 4)
	for {
		select {
		case <-stop:
			return
		case iq := <-queue:
			d.mu.RLock()
			mixHz := d.tuningOffsetHz
			d.mu.RUnlock()
			phaseStep := -2 * math.Pi * mixHz / d.rate
			for n := 0; n+1 < len(iq); n += 2 {
				i, q := float64(iq[n]), float64(iq[n+1])
				cs, sn := math.Cos(ncoPhase), math.Sin(ncoPhase)
				i, q = i*cs-q*sn, i*sn+q*cs
				ncoPhase += phaseStep
				if ncoPhase > math.Pi {
					ncoPhase -= 2 * math.Pi
				} else if ncoPhase < -math.Pi {
					ncoPhase += 2 * math.Pi
				}
				sumI += i
				sumQ += q
				count++
				phase++
				if phase < step {
					continue
				}
				phase -= step
				i = sumI / float64(count)
				q = sumQ / float64(count)
				sumI, sumQ, count = 0, 0, 0
				p := i*i + q*q
				if p > 1e-12 {
					target := 1 / math.Sqrt(p)
					agc += .002 * (target - agc)
					agc = math.Max(.05, math.Min(100, agc))
				}
				i *= agc
				q *= agc
				powerSum += p
				rollingI += i - ringI[ringIndex]
				rollingQ += q - ringQ[ringIndex]
				ringI[ringIndex], ringQ[ringIndex] = i, q
				ringIndex = (ringIndex + 1) & 3
				if ringCount < 4 {
					ringCount++
				}
				phaseIndex := samplePhase
				samplePhase = (samplePhase + 1) & 3
				if ringCount < 4 {
					continue
				}
				candidateI, candidateQ := rollingI/4, rollingQ/4
				candidateMag := math.Hypot(candidateI, candidateQ)
				if candidateMag > 0 {
					candidateI, candidateQ = candidateI/candidateMag, candidateQ/candidateMag
				}
				if timingHavePrev[phaseIndex] {
					delta := math.Atan2(candidateQ*timingPrevI[phaseIndex]-candidateI*timingPrevQ[phaseIndex], candidateI*timingPrevI[phaseIndex]+candidateQ*timingPrevQ[phaseIndex])
					nearest := math.Round((delta-math.Pi/4)/(math.Pi/2))*(math.Pi/2) + math.Pi/4
					err := math.Atan2(math.Sin(delta-nearest), math.Cos(delta-nearest))
					timingScore[phaseIndex] = .995*timingScore[phaseIndex] + .005*err*err
				}
				timingPrevI[phaseIndex], timingPrevQ[phaseIndex], timingHavePrev[phaseIndex] = candidateI, candidateQ, true
				timingSamples++
				if timingSamples >= 720 {
					timingPhase = selectTimingPhase(timingScore, timingPhase)
					timingSamples = 0
				}
				if phaseIndex != timingPhase {
					continue
				}
				i, q = candidateI, candidateQ
				mag := math.Hypot(i, q)
				if mag > 0 {
					i /= mag
					q /= mag
				}
				if havePrev {
					dp := math.Atan2(q*prevI-i*prevQ, i*prevI+q*prevQ)
					nearest := math.Round((dp-math.Pi/4)/(math.Pi/2))*(math.Pi/2) + math.Pi/4
					err := math.Abs(math.Atan2(math.Sin(dp-nearest), math.Cos(dp-nearest)))
					phaseErrorSum += err
					freqSum += math.Atan2(math.Sin(dp-nearest), math.Cos(dp-nearest)) * symbolRate / (2 * math.Pi)
					metricCount++
					var b1, b2 byte
					if dp >= 0 {
						if dp >= math.Pi/2 {
							b2 = 1
						}
					} else {
						b1 = 1
						if dp < -math.Pi/2 {
							b2 = 1
						}
					}
					bits = append(bits, b1, b2)
					phases = append(phases, dp)
					if len(phases) > 64 {
						phases = append(phases[:0], phases[len(phases)-48:]...)
					}
					d.totalBits += 2
					if len(bits) > 2048 {
						bits = append(bits[:0], bits[len(bits)-1024:]...)
					}
					training, _ := detectTraining(phases)
					if training != trainingNone && lastTrainingBit != 0 && d.totalBits-lastTrainingBit < 400 {
						training = trainingNone
					}
					if training == trainingSync {
						lastTrainingBit = d.totalBits
						d.mu.Lock()
						d.syncHits++
						d.lastSyncBit = d.totalBits
						d.lastSync = time.Now()
						d.state = "LIVE TETRA SYNC"
						start := len(bits) - len(syncTraining) - 120
						if start >= 0 {
							if si, ok := decodeBSCH(bits[start : start+120]); ok {
								d.system = si
								d.state = "BSCH DECODED"
							} else {
								d.bschFailures++
							}
						}
						d.mu.Unlock()
					} else if training == trainingNDB1 || training == trainingNDB2 {
						lastTrainingBit = d.totalBits
						isNDB2 := training == trainingNDB2
						detectedSlot := -1
						d.mu.Lock()
						d.normalBursts++
						if d.lastSyncBit > 0 && d.totalBits > d.lastSyncBit {
							delta := d.totalBits - d.lastSyncBit
							burstDelta := int((delta + 255) / 510)
							base := 0
							if d.system.Valid {
								base = int(d.system.Timeslot) - 1
							}
							slot := (base + burstDelta) % 4
							d.slotBursts[slot]++
							d.framedBursts++
							detectedSlot = slot
						}
						d.lastSync = time.Now()
						if isNDB2 {
							d.state = "TETRA NDB2 BURST"
						} else {
							d.state = "TETRA NDB1 BURST"
						}
						d.mu.Unlock()
						if detectedSlot >= 0 && d.totalBits >= 266 {
							pending = append(pending, pendingBurst{start: d.totalBits - 266, slot: detectedSlot, ndb2: isNDB2})
						}
					}
					for len(pending) > 0 && d.totalBits >= pending[0].start+510 {
						pnd := pending[0]
						pending = pending[1:]
						bufferStart := d.totalBits - uint64(len(bits))
						if pnd.start >= bufferStart {
							idx := int(pnd.start - bufferStart)
							if idx+510 <= len(bits) {
								d.processNormalBurst(bits[idx:idx+510], pnd.slot, pnd.ndb2)
							}
						}
					}
				}
				prevI, prevQ, havePrev = i, q, true
				points = append(points, Point{float32(i), float32(q)})
				if len(points) > 96 {
					points = points[len(points)-96:]
				}
			}
			if metricCount >= 360 {
				meanErr := phaseErrorSum / float64(metricCount)
				quality := math.Max(0, math.Min(100, 100*(1-meanErr/(math.Pi/4))))
				level := 10 * math.Log10(math.Max(powerSum/float64(metricCount*4), 1e-12))
				d.mu.Lock()
				d.symbols += uint64(metricCount)
				d.level = float32(level)
				d.quality = float32(quality)
				d.freqErr = float32(freqSum / float64(metricCount))
				d.timingPhase = uint8(timingPhase)
				d.timingError = float32(math.Sqrt(math.Max(timingScore[timingPhase], 0)))
				d.points = append(d.points[:0], points...)
				if time.Since(d.lastSync) > 2*time.Second {
					d.state = "SEARCHING SYNC / NTS1"
				}
				d.mu.Unlock()
				phaseErrorSum, freqSum, powerSum = 0, 0, 0
				metricCount = 0
			}
		}
	}
}

func (d *Decoder) processNormalBurst(burst []byte, slot int, ndb2 bool) {
	if len(burst) < 510 {
		return
	}
	coded := make([]byte, 0, 432)
	coded = append(coded, burst[14:230]...)
	coded = append(coded, burst[282:498]...)
	d.mu.RLock()
	si := d.system
	d.mu.RUnlock()
	usage, aachOK := decodeAACH(burst, si)
	d.mu.Lock()
	if aachOK {
		d.aachValid++
		d.slotTraffic[slot] = 0
		if usage > 3 {
			d.slotTraffic[slot] = 1
			// AACH identifies traffic by its usage marker. Encryption belongs to
			// that marker, not to the timeslot where MAC-RESOURCE was received.
			encryption := int8(-1)
			if seen := d.usageEncryptionSeen[usage]; !seen.IsZero() && time.Since(seen) <= 30*time.Second {
				encryption = d.usageEncrypted[usage]
			}
			d.slotEncrypted[slot] = encryption
			d.slotEncryptionSeen[slot] = time.Now()
			// In clear-only mode, unknown traffic is deliberately silent. A
			// vocoder can produce scrambled-sounding output from encrypted TCH.
			allowed := !d.clearAudioOnly || encryption == 0
			selected := d.listenSlot == 0 || d.listenSlot == int8(slot+1)
			activeExpired := d.activeAudioSlot == 0 || time.Since(d.activeAudioSeen) > 1500*time.Millisecond
			callID := d.usageCall[usage]
			call, confirmed := d.calls[callID]
			confirmed = confirmed && call.Active && time.Since(call.LastSeen) <= 8*time.Second
			currentCall, currentConfirmed := d.calls[d.activeCallID]
			currentConfirmed = currentConfirmed && currentCall.Active && time.Since(currentCall.LastSeen) <= 2*time.Second
			canTakeAuto := d.listenSlot != 0 || d.activeAudioSlot == int8(slot+1) || (!currentConfirmed && (confirmed || activeExpired))
			if allowed && selected && canTakeAuto {
				d.activeAudioSlot = int8(slot + 1)
				d.activeAudioSeen = time.Now()
				if confirmed {
					d.activeCallID = callID
					call.Slot = uint8(slot + 1)
					call.Encrypted = encryption == 1
					call.LastSeen = time.Now()
					d.calls[callID] = call
				} else if activeExpired {
					d.activeCallID = 0
				}
			}
		}
		d.slotTrafficSeen[slot] = time.Now()
	} else {
		d.aachRejected++
	}
	if ndb2 {
		firstHalf := coded[:216]
		payload, ok, bitErrors, channelBits := decodeSCHHMetrics(firstHalf, si)
		d.recordChannelResultLocked(ok, bitErrors, channelBits)
		secondHalfStolen := false
		if ok {
			d.schValid++
			if len(payload) >= 13 && bitsToUint(payload, 0, 2) == 0 {
				length := bitsToUint(payload, 7, 6)
				secondHalfStolen = length == 62 || length == 63
			}
			d.processMACPayloadLocked(payload, slot)
		} else {
			d.schCRCFailures++
		}
		if aachOK && usage > 3 && !secondHalfStolen {
			if d.activeAudioSlot != int8(slot+1) || time.Since(d.activeAudioSeen) > 5*time.Second {
				d.audioRejectedUnselected++
				d.lastAudioDecision = fmt.Sprintf("TS%d NDB2 NOT SELECTED", slot+1)
			} else if d.clearAudioOnly && d.slotEncrypted[slot] != 0 {
				d.audioRejectedEncrypted++
				d.lastAudioDecision = fmt.Sprintf("TS%d NDB2 CIPHER OR UNKNOWN", slot+1)
			} else if half := descrambleBlock(coded[216:], si); half != nil {
				voiceBits := make([]byte, 432)
				copy(voiceBits[216:], half)
				d.processVoiceBitsLocked(voiceBits, slot, true)
			}
		}
		d.mu.Unlock()
		return
	}
	if aachOK && usage > 3 {
		d.processVoiceLocked(coded, si, slot)
		d.mu.Unlock()
		return
	}
	d.mu.Unlock()
	payload, ok, bitErrors, channelBits := decodeSCHFMetrics(coded, si)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.recordChannelResultLocked(ok, bitErrors, channelBits)
	if !ok {
		d.schCRCFailures++
		return
	}
	d.schValid++
	d.processMACPayloadLocked(payload, slot)
}

func (d *Decoder) processMACPayloadLocked(payload []byte, slot int) {
	for offset := 0; offset+16 <= len(payload); {
		pdu := payload[offset:]
		pduType := bitsToUint(pdu, 0, 2)
		d.macPDUTypes[pduType]++
		switch pduType {
		case 0:
			lengthField := bitsToUint(pdu, 7, 6)
			lengthBits := int(lengthField) * 8
			d.processMACResourceLocked(pdu, slot)
			if lengthField < 1 || lengthField > 0x3a || lengthBits > len(pdu) {
				return
			}
			offset += lengthBits
			if pdu[2] != 0 {
				return
			}
		case 2:
			if network, valid := parseMACSysinfo(pdu); valid {
				d.network = network
				d.state = "SYSINFO DECODED"
			}
			// Broadcast PDUs occupy their logical channel; remaining decoded
			// bits are padding rather than another concatenated MAC PDU.
			return
		default:
			return
		}
	}
}

func (d *Decoder) processVoiceLocked(coded []byte, si SystemInfo, slot int) {
	if d.onAudio == nil || slot < 0 || slot > 3 {
		d.lastAudioDecision = "AUDIO OUTPUT UNAVAILABLE"
		return
	}
	if d.clearAudioOnly && d.slotEncrypted[slot] != 0 {
		d.audioRejectedEncrypted++
		if d.slotEncrypted[slot] < 0 {
			d.lastAudioDecision = fmt.Sprintf("TS%d UNKNOWN CIPHER", slot+1)
		} else {
			d.lastAudioDecision = fmt.Sprintf("TS%d CIPHER", slot+1)
		}
		return
	}
	if d.activeAudioSlot != int8(slot+1) || time.Since(d.activeAudioSeen) > 5*time.Second {
		d.audioRejectedUnselected++
		d.lastAudioDecision = fmt.Sprintf("TS%d NOT SELECTED", slot+1)
		return
	}
	voice := d.voice[slot]
	if voice == nil || !voice.ready {
		d.audioRejectedInactive++
		d.lastAudioDecision = "VOICE CODEC UNAVAILABLE"
		return
	}
	type4 := descrambleFullSlot(coded, si)
	if type4 == nil {
		d.audioRejectedDamaged++
		d.lastAudioDecision = "FRAME WITHOUT VALID SCRAMBLER"
		return
	}
	d.processVoiceBitsLocked(type4, slot, false)
}

func (d *Decoder) processVoiceBitsLocked(type4 []byte, slot int, stolen bool) {
	voice := d.voice[slot]
	pcm, ok := voice.decode(type4, stolen)
	if !ok {
		d.audioRejectedDamaged++
		d.lastAudioDecision = fmt.Sprintf("TS%d VOICE FRAME REJECTED", slot+1)
		return
	}
	d.audioFrames++
	d.lastAudio = time.Now()
	d.lastAudioDecision = fmt.Sprintf("PLAYING TS%d", slot+1)
	d.onAudio(pcm)
}

func (d *Decoder) recordChannelResultLocked(ok bool, errors, bits int) {
	d.channelFrames++
	if !ok {
		d.channelBad++
	}
	if errors > 0 {
		d.channelErrors += uint64(errors)
	}
	if bits > 0 {
		d.channelBits += uint64(bits)
	}
}

// processMACResourceLocked consumes one already delimited MAC-RESOURCE. The
// decoder mutex is held by processNormalBurst while this method updates state.
func (d *Decoder) processMACResourceLocked(payload []byte, slot int) {
	address, reason, ok := parseMACResourceDetailed(payload)
	if !ok {
		d.macRejected++
		if reason == "ENCRYPTED ALLOCATION" {
			d.macEncrypted++
		}
		return
	}
	d.macResources++
	if slot >= 0 && slot < 4 {
		if address.HasUsageMarker && address.UsageMarker > 3 {
			if address.Encrypted {
				d.usageEncrypted[address.UsageMarker] = 1
			} else {
				d.usageEncrypted[address.UsageMarker] = 0
			}
			d.usageEncryptionSeen[address.UsageMarker] = time.Now()
		}
	}
	if address.ChannelAllocation {
		d.macChannelAlloc++
	}
	if address.Encrypted {
		d.macEncrypted++
	}
	u := d.users[address.SSI]
	fresh := userFromResource(address, slot+1)
	fresh.Seen = u.Seen + 1
	d.users[address.SSI] = fresh
	d.state = "MAC-RESOURCE DECODED"
	pdu, llcOK := parseLLC(payload, address)
	if !llcOK || pdu.FCSInvalid {
		d.llcRejected++
		return
	}
	d.llcTypes[pdu.Type]++
	tl := pdu.TL
	if pdu.Fragment {
		d.llcFragments++
		key := address.SSI<<8 | uint32(pdu.NS)
		frag := d.fragments[key]
		if len(frag.Bits) == 0 || pdu.SS == frag.LastSS+1 {
			frag.NS, frag.LastSS, frag.Updated = pdu.NS, pdu.SS, time.Now()
			frag.Bits = append(frag.Bits, pdu.TL...)
			d.fragments[key] = frag
		} else {
			delete(d.fragments, key)
			d.llcRejected++
			return
		}
		if !pdu.Final {
			return
		}
		tl = frag.Bits
		delete(d.fragments, key)
		if len(tl) >= 32 {
			tl = tl[:len(tl)-32]
		}
		d.llcReassembled++
	}
	cmce, pdisc, valid := parseTLSDU(tl)
	if pdisc < 8 {
		d.mleProtocols[pdisc]++
	}
	if valid {
		d.cmceEvents++
		now := time.Now()
		text := cmce.Kind
		if cmce.CallID != 0 {
			text += fmt.Sprintf(" · CALL %d", cmce.CallID)
		}
		d.messages = append([]Message{{Time: now, Kind: cmce.Kind, Text: fmt.Sprintf("SSI %08d · TS%d · %s", address.SSI, slot+1, text)}}, d.messages...)
		if len(d.messages) > 200 {
			d.messages = d.messages[:200]
		}
		if cmce.Code == 0 || cmce.Code == 1 || cmce.Code == 2 || cmce.Code == 7 || cmce.Code == 11 {
			g := d.groups[address.SSI]
			g.ID = address.SSI
			g.Name = "CMCE DESTINATION"
			g.LastSeen = now
			g.Calls++
			g.LastEvent = cmce.Kind
			d.groups[address.SSI] = g
		}
		d.updateCallLocked(cmce, address, slot, now)
		if len(cmce.SDS) > 0 {
			caller := cmce.CallingSSI
			if caller == 0 {
				caller = address.SSI
			}
			if message, position, ok := parseSDS(cmce.SDS, caller, now); ok {
				d.messages = append([]Message{message}, d.messages...)
				if position != nil {
					d.positions[position.SSI] = *position
				}
				if len(d.messages) > 200 {
					d.messages = d.messages[:200]
				}
			}
		}
		d.state = "CMCE " + cmce.Kind
	} else {
		if parsed, ok := parseNeighbourBroadcast(tl, d.system, d.network); ok {
			for _, neighbour := range parsed {
				d.neighbours[neighbour.CellID] = neighbour
			}
			d.state = "NEIGHBOUR CELLS DECODED"
			return
		}
		d.llcNonCMCE++
	}
}

func (d *Decoder) updateCallLocked(cmce cmceInfo, address resourceAddress, slot int, now time.Time) {
	if cmce.CallID == 0 {
		return
	}
	call := d.calls[cmce.CallID]
	call.ID = cmce.CallID
	call.SSI = address.SSI
	call.State = cmce.Kind
	call.LastSeen = now
	call.Events++
	call.Encrypted = address.Encrypted
	if address.HasUsageMarker && address.UsageMarker > 3 {
		call.UsageMarker = address.UsageMarker
		d.usageCall[address.UsageMarker] = cmce.CallID
	}
	if slot >= 0 && slot < 4 {
		call.Slot = uint8(slot + 1)
	}
	switch cmce.Code {
	case 4, 6, 9: // disconnect, release, transmitter ceased
		call.Active = false
		if d.activeCallID == call.ID {
			d.activeCallID = 0
			d.activeAudioSlot = 0
		}
	default:
		call.Active = true
	}
	d.calls[call.ID] = call
}
