package tetrapolruntime

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Event struct {
	Time    time.Time
	Kind    string
	Summary string
}

// Telemetry is the protocol information reported by tetrapol-kit. Sync and
// voice activity are derived from recent valid records because the upstream
// JSON format does not publish explicit acquisition/loss events.
type Telemetry struct {
	Synchronized   bool
	VoiceActive    bool
	FrameNumber    int
	FrameKnown     bool
	FrameType      string
	LogicalChannel string
	ValidFrames    uint64
	CRCFailures    uint64
	BitsFixed      uint64
	QualityPercent float64
	QualityKnown   bool
	CRCPerSecond   float64
	SCR            int
	SCRKnown       bool
	RXOffset       uint64
	ASB            [2]int
	ASBKnown       bool
	LastRXTime     string
	CipherState    string
	CipherKeyType  int
	CipherKeyIndex int
	ChannelAction  string
	ChannelKind    string
	ChannelID      int
	ChannelEvent   uint64
	LastRecord     time.Time
	RecentEvents   []Event
}

type telemetryTracker struct {
	mu                   sync.RWMutex
	value                Telemetry
	lastVoice            time.Time
	validTimes           []time.Time
	crcTimes             []time.Time
	explicitSync         bool
	explicitSynchronized bool
}

type kitRecord struct {
	Event    string `json:"event"`
	SCR      *int   `json:"scr"`
	RXOffset uint64 `json:"rx_offs"`
	RXTime   string `json:"rx_time"`
	State    string `json:"state"`
	Frame    struct {
		FrameNumber *int            `json:"frame_no"`
		State       json.RawMessage `json:"state"`
		Type        string          `json:"type"`
		BitsFixed   int             `json:"bits_fixed"`
		ASB         []int           `json:"asb"`
	} `json:"frame"`
	TSDU struct {
		FrameNumber *int   `json:"frame_no"`
		Logical     string `json:"log_ch"`
		Address     struct {
			Z int `json:"z"`
			Y int `json:"y"`
			X int `json:"x"`
		} `json:"addr"`
		TSAPID *int `json:"tsap_id"`
		Cipher *struct {
			State    string `json:"state"`
			KeyType  int    `json:"key_type"`
			KeyIndex int    `json:"key_index"`
		} `json:"cipher"`
		ChannelAction *struct {
			Action    string `json:"action"`
			Kind      string `json:"kind"`
			ChannelID int    `json:"channel_id"`
		} `json:"channel_action"`
	} `json:"tsdu"`
}

func (tracker *telemetryTracker) addEvent(now time.Time, kind, summary string) {
	tracker.value.RecentEvents = append(tracker.value.RecentEvents, Event{Time: now, Kind: kind, Summary: summary})
	if len(tracker.value.RecentEvents) > 12 {
		tracker.value.RecentEvents = append([]Event(nil), tracker.value.RecentEvents[len(tracker.value.RecentEvents)-12:]...)
	}
}

func (tracker *telemetryTracker) update(line []byte, now time.Time) {
	var record kitRecord
	if json.Unmarshal(line, &record) != nil {
		return
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	value := &tracker.value
	switch record.Event {
	case "frame":
		value.LastRecord = now
		if record.RXOffset > 0 {
			value.RXOffset = record.RXOffset
		}
		if record.RXTime != "" {
			value.LastRXTime = record.RXTime
		}
		if record.Frame.Type != "" {
			value.FrameType = record.Frame.Type
		}
		if record.Frame.FrameNumber != nil {
			value.FrameNumber, value.FrameKnown = *record.Frame.FrameNumber, true
		}
		var state string
		_ = json.Unmarshal(record.Frame.State, &state)
		if state == "ok" {
			value.ValidFrames++
			tracker.validTimes = append(tracker.validTimes, now)
			if record.Frame.BitsFixed > 0 {
				value.BitsFixed += uint64(record.Frame.BitsFixed)
			}
			if record.Frame.Type == "VOICE" {
				tracker.lastVoice = now
			}
			if len(record.Frame.ASB) >= 2 {
				value.ASB, value.ASBKnown = [2]int{record.Frame.ASB[0], record.Frame.ASB[1]}, true
			}
			tracker.addEvent(now, record.Frame.Type, fmt.Sprintf("trama %s · %d bits corregidos", record.Frame.Type, record.Frame.BitsFixed))
		} else if state == "bad_CRC" {
			value.CRCFailures++
			tracker.crcTimes = append(tracker.crcTimes, now)
			tracker.addEvent(now, "CRC", "trama descartada por CRC")
		}
	case "tsdu":
		value.LastRecord = now
		if record.RXOffset > 0 {
			value.RXOffset = record.RXOffset
		}
		value.LogicalChannel = record.TSDU.Logical
		if record.TSDU.FrameNumber != nil {
			value.FrameNumber, value.FrameKnown = *record.TSDU.FrameNumber, true
		}
		summary := fmt.Sprintf("%s · dirección %d.%d.%d", valueOr(record.TSDU.Logical, "TSDU"), record.TSDU.Address.Z, record.TSDU.Address.Y, record.TSDU.Address.X)
		if record.TSDU.TSAPID != nil {
			summary += fmt.Sprintf(" · TSAP %d", *record.TSDU.TSAPID)
		}
		if record.TSDU.Cipher != nil {
			value.CipherState = record.TSDU.Cipher.State
			value.CipherKeyType = record.TSDU.Cipher.KeyType
			value.CipherKeyIndex = record.TSDU.Cipher.KeyIndex
			tracker.addEvent(now, "CIPHER", fmt.Sprintf("%s · tipo %d índice %d", value.CipherState, value.CipherKeyType, value.CipherKeyIndex))
		}
		if record.TSDU.ChannelAction != nil {
			value.ChannelAction = record.TSDU.ChannelAction.Action
			value.ChannelKind = record.TSDU.ChannelAction.Kind
			value.ChannelID = record.TSDU.ChannelAction.ChannelID
			value.ChannelEvent++
			if value.ChannelAction == "follow" {
				tracker.addEvent(now, "FOLLOW", fmt.Sprintf("canal %d · %s", value.ChannelID, valueOr(value.ChannelKind, "tráfico")))
			} else if value.ChannelAction == "return" {
				tracker.addEvent(now, "FOLLOW", "retorno al canal de control")
			}
		}
		tracker.addEvent(now, "TSDU", summary)
	case "scr":
		if record.SCR != nil {
			value.SCR, value.SCRKnown = *record.SCR, true
			value.LastRecord = now
			tracker.addEvent(now, "SCR", fmt.Sprintf("scrambling %d detectado", *record.SCR))
		}
	case "sync":
		tracker.explicitSync = true
		tracker.explicitSynchronized = record.State == "acquired"
		value.LastRecord = now
		if record.RXOffset > 0 {
			value.RXOffset = record.RXOffset
		}
		if tracker.explicitSynchronized {
			tracker.addEvent(now, "SYNC", "sincronismo adquirido")
		} else {
			tracker.addEvent(now, "SYNC", "sincronismo perdido")
		}
	}
}

func (tracker *telemetryTracker) voiceAllowed(clearOnly bool) bool {
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	return !clearOnly || tracker.value.CipherState == "clear"
}

func (tracker *telemetryTracker) snapshot(now time.Time) Telemetry {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	cutoff := now.Add(-5 * time.Second)
	validFirst := 0
	for validFirst < len(tracker.validTimes) && tracker.validTimes[validFirst].Before(cutoff) {
		validFirst++
	}
	tracker.validTimes = append([]time.Time(nil), tracker.validTimes[validFirst:]...)
	first := 0
	for first < len(tracker.crcTimes) && tracker.crcTimes[first].Before(cutoff) {
		first++
	}
	tracker.crcTimes = append([]time.Time(nil), tracker.crcTimes[first:]...)
	value := tracker.value
	value.Synchronized = !value.LastRecord.IsZero() && now.Sub(value.LastRecord) < 2*time.Second
	if tracker.explicitSync {
		value.Synchronized = tracker.explicitSynchronized
	}
	value.VoiceActive = !tracker.lastVoice.IsZero() && now.Sub(tracker.lastVoice) < 500*time.Millisecond
	windowFrames := len(tracker.validTimes) + len(tracker.crcTimes)
	// A single good packet is not enough evidence to label reception quality.
	// The UI only exposes this short rolling CRC-integrity measure after five
	// observations collected during the current five-second window.
	if windowFrames >= 5 {
		value.QualityKnown = true
		value.QualityPercent = float64(len(tracker.validTimes)) * 100 / float64(windowFrames)
	} else {
		value.QualityKnown = false
		value.QualityPercent = 0
	}
	value.CRCPerSecond = float64(len(tracker.crcTimes)) / 5
	value.RecentEvents = append([]Event(nil), value.RecentEvents...)
	return value
}

func (tracker *telemetryTracker) reset() {
	tracker.mu.Lock()
	tracker.value, tracker.lastVoice, tracker.validTimes, tracker.crcTimes = Telemetry{}, time.Time{}, nil, nil
	tracker.explicitSync, tracker.explicitSynchronized = false, false
	tracker.mu.Unlock()
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
