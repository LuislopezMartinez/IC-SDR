package voacap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	solarCycleURL = "https://services.swpc.noaa.gov/json/solar-cycle/observed-solar-cycle-indices.json"
	solarFluxURL  = "https://services.swpc.noaa.gov/json/f107_cm_flux.json"
	planetaryKURL = "https://services.swpc.noaa.gov/products/noaa-planetary-k-index.json"
	noaaScalesURL = "https://services.swpc.noaa.gov/products/noaa-scales.json"
	xrayFlareURL  = "https://services.swpc.noaa.gov/json/goes/primary/xray-flares-latest.json"
	drapStatusURL = "https://services.swpc.noaa.gov/products/animations/d-rap/global.json"
)

type SolarData struct {
	SSN       float64   `json:"ssn"`
	SFI       float64   `json:"sfi"`
	Kp        float64   `json:"kp"`
	Ap        int       `json:"ap"`
	RScale    int       `json:"rScale"`
	SScale    int       `json:"sScale"`
	GScale    int       `json:"gScale"`
	XRayClass string    `json:"xRayClass"`
	DRAPTime  time.Time `json:"drapTime"`
	Observed  time.Time `json:"observed"`
	FetchedAt time.Time `json:"fetchedAt"`
}

type solarCycleRecord struct {
	TimeTag string  `json:"time-tag"`
	SSN     float64 `json:"ssn"`
}

type solarFluxRecord struct {
	TimeTag string  `json:"time_tag"`
	Flux    float64 `json:"flux"`
}

type planetaryKRecord struct {
	TimeTag string  `json:"time_tag"`
	Kp      float64 `json:"Kp"`
	Ap      int     `json:"a_running"`
}

type noaaScale struct {
	Scale string `json:"Scale"`
}

type noaaScaleRecord struct {
	R noaaScale `json:"R"`
	S noaaScale `json:"S"`
	G noaaScale `json:"G"`
}

type xrayFlareRecord struct {
	CurrentClass string `json:"current_class"`
}

type drapRecord struct {
	TimeTag time.Time `json:"time_tag"`
}

func FetchSolarData(ctx context.Context) (SolarData, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	var cycles []solarCycleRecord
	var fluxes []solarFluxRecord
	var indices []planetaryKRecord
	var scales map[string]noaaScaleRecord
	var flares []xrayFlareRecord
	var drap []drapRecord
	if err := fetchJSON(ctx, client, solarCycleURL, &cycles); err != nil {
		return SolarData{}, err
	}
	if err := fetchJSON(ctx, client, solarFluxURL, &fluxes); err != nil {
		return SolarData{}, err
	}
	if err := fetchJSON(ctx, client, planetaryKURL, &indices); err != nil {
		return SolarData{}, err
	}
	// Live alerts enrich the confidence assessment but must never prevent the
	// core SSN/SFI update when an auxiliary NOAA product is briefly unavailable.
	_ = fetchJSON(ctx, client, noaaScalesURL, &scales)
	_ = fetchJSON(ctx, client, xrayFlareURL, &flares)
	_ = fetchJSON(ctx, client, drapStatusURL, &drap)
	if len(cycles) == 0 || len(fluxes) == 0 || len(indices) == 0 {
		return SolarData{}, errors.New("NOAA SWPC devolvió datos incompletos")
	}
	cycle := newestCycle(cycles)
	flux := newestFlux(fluxes)
	index := newestKIndex(indices)
	currentScale := scales["0"]
	xrayClass := "--"
	if len(flares) > 0 && flares[0].CurrentClass != "" {
		xrayClass = flares[0].CurrentClass
	}
	drapTime := time.Time{}
	for _, item := range drap {
		if item.TimeTag.After(drapTime) {
			drapTime = item.TimeTag
		}
	}
	observed, _ := time.Parse("2006-01-02T15:04:05", index.TimeTag)
	return SolarData{
		SSN: cycle.SSN, SFI: flux.Flux, Kp: index.Kp, Ap: index.Ap,
		RScale: scaleNumber(currentScale.R.Scale), SScale: scaleNumber(currentScale.S.Scale), GScale: scaleNumber(currentScale.G.Scale),
		XRayClass: xrayClass, DRAPTime: drapTime, Observed: observed, FetchedAt: time.Now().UTC(),
	}, nil
}

func scaleNumber(value string) int {
	number, _ := strconv.Atoi(value)
	return min(max(number, 0), 5)
}

func newestCycle(records []solarCycleRecord) solarCycleRecord {
	newest := records[0]
	for _, record := range records[1:] {
		if record.TimeTag > newest.TimeTag {
			newest = record
		}
	}
	return newest
}

func fetchJSON(ctx context.Context, client *http.Client, url string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "GO-Zero/VOACAP")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("NOAA SWPC no disponible: " + response.Status)
	}
	return json.NewDecoder(response.Body).Decode(destination)
}

func newestFlux(records []solarFluxRecord) solarFluxRecord {
	newest := records[0]
	for _, record := range records[1:] {
		if record.TimeTag > newest.TimeTag {
			newest = record
		}
	}
	return newest
}

func newestKIndex(records []planetaryKRecord) planetaryKRecord {
	newest := records[0]
	for _, record := range records[1:] {
		if record.TimeTag > newest.TimeTag {
			newest = record
		}
	}
	return newest
}

func LoadSolarData(path string) (SolarData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SolarData{}, err
	}
	var solar SolarData
	err = json.Unmarshal(data, &solar)
	return solar, err
}

func SaveSolarData(path string, solar SolarData) error {
	data, err := json.MarshalIndent(solar, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
