package voacap

import (
	"encoding/json"
	"os"
	"time"
)

var AmateurBands = []string{"80 m", "60 m", "40 m", "30 m", "20 m", "17 m", "15 m", "12 m", "10 m"}

type Settings struct {
	QTH       string          `json:"qth"`
	Latitude  float64         `json:"latitude"`
	Longitude float64         `json:"longitude"`
	Bands     map[string]bool `json:"bands"`
	Mode      string          `json:"mode"`
	PowerW    int             `json:"powerW"`
	Antenna   string          `json:"antenna"`
	Noise     string          `json:"noise"`
	UTCHour   int             `json:"utcHour"`
	SSN       int             `json:"ssn"`
	AutoSolar bool            `json:"autoSolar"`
}

type Cell struct {
	Latitude    float64            `json:"latitude"`
	Longitude   float64            `json:"longitude"`
	Reliability map[string]float64 `json:"reliability"`
}

type Prediction struct {
	Cells      []Cell    `json:"cells,omitempty"`
	Calculated time.Time `json:"calculated,omitempty"`
	Engine     string    `json:"engine,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type Snapshot struct {
	Settings   Settings   `json:"settings"`
	Theme      string     `json:"theme"`
	Updated    time.Time  `json:"updated"`
	Engine     string     `json:"engine"`
	Prediction Prediction `json:"prediction,omitempty"`
	Solar      SolarData  `json:"solar,omitempty"`
}

func Defaults() Settings {
	return Settings{
		QTH: "Madrid", Latitude: 40.4168, Longitude: -3.7038,
		Bands: map[string]bool{"20 m": true, "15 m": true},
		Mode:  "SSB", PowerW: 100, Antenna: "DIPOLO", Noise: "RURAL", UTCHour: time.Now().UTC().Hour(), SSN: 100, AutoSolar: true,
	}
}

func Load(path string) Settings {
	settings := Defaults()
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	settings.Normalize()
	return settings
}

func (settings *Settings) Normalize() {
	if settings.QTH == "" {
		settings.QTH = "QTH"
	}
	if settings.Latitude < -90 || settings.Latitude > 90 {
		settings.Latitude = 0
	}
	if settings.Longitude < -180 || settings.Longitude > 180 {
		settings.Longitude = 0
	}
	if settings.Bands == nil {
		settings.Bands = map[string]bool{}
	}
	if settings.Mode != "CW" && settings.Mode != "SSB" && settings.Mode != "DIGITAL" {
		settings.Mode = "SSB"
	}
	if settings.PowerW <= 0 || settings.PowerW > 5000 {
		settings.PowerW = 100
	}
	if settings.Antenna == "" {
		settings.Antenna = "DIPOLO"
	}
	if settings.Noise == "" {
		settings.Noise = "RURAL"
	}
	settings.UTCHour = min(max(settings.UTCHour, 0), 23)
	settings.SSN = min(max(settings.SSN, 0), 300)
}
