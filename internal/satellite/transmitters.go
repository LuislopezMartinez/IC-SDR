package satellite

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed transmitters_catalog.json
var embeddedTransmitterFile embed.FS

const satnogsTransmittersURL = "https://db.satnogs.org/api/transmitters/?format=json"

var (
	embeddedTransmittersOnce sync.Once
	embeddedTransmitterTable map[int][]Signal
)

type satnogsPage struct {
	Next    string               `json:"next"`
	Results []satnogsTransmitter `json:"results"`
}

type satnogsTransmitter struct {
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	Mode         string   `json:"mode"`
	NORAD        int      `json:"norad_cat_id"`
	DownlinkLow  *float64 `json:"downlink_low"`
	DownlinkHigh *float64 `json:"downlink_high"`
	Alive        *bool    `json:"alive"`
}

func knownSignals(norad int) []Signal {
	switch norad {
	case 25544:
		return []Signal{{"Voice / SSTV", "FM", 145800000}, {"APRS", "AFSK", 145825000}}
	case 43700:
		return []Signal{{"PSK beacon", "BPSK", 10489750000}, {"NB transponder", "SSB/CW", 10489500000}, {"WB transponder", "DVB-S2", 10491000000}}
	default:
		return nil
	}
}

func builtinTransmitters() map[int][]Signal {
	table := map[int][]Signal{
		25544: knownSignals(25544),
		43700: knownSignals(43700),
		25338: {{"APT", "AM", 137620000}},
		28654: {{"APT", "AM", 137912500}},
		33591: {{"APT", "AM", 137100000}},
		40069: {{"LRPT", "QPSK", 137100000}},
		44387: {{"LRPT", "QPSK", 137900000}},
		27607: {{"FM voice", "FM", 436795000}},
		43017: {{"FM voice", "FM", 145880000}},
		24278: {{"V/U transponder", "SSB/CW", 435800000}, {"Beacon", "CW", 435795000}},
		40908: {{"Telemetry", "BPSK", 145705000}},
		42761: {{"Telemetry", "BPSK", 145855000}},
		42759: {{"Telemetry", "BPSK", 145890000}},
		39444: {{"Telemetry", "BPSK", 145935000}},
		7530:  {{"V/U transponder", "SSB/CW", 145950000}},
		39427: {{"Telemetry", "BPSK", 145935000}},
		44331: {{"FM voice", "FM", 145840000}},
	}
	return table
}

func mergeSignals(groups ...[]Signal) []Signal {
	out := make([]Signal, 0)
	seen := map[int64]struct{}{}
	for _, group := range groups {
		for _, sig := range group {
			if sig.DownlinkHz <= 0 {
				continue
			}
			if _, ok := seen[sig.DownlinkHz]; ok {
				continue
			}
			if strings.TrimSpace(sig.Name) == "" {
				sig.Name = "Downlink"
			}
			if strings.TrimSpace(sig.Mode) == "" {
				sig.Mode = "FM"
			}
			seen[sig.DownlinkHz] = struct{}{}
			out = append(out, sig)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := amateurRank(out[i].DownlinkHz), amateurRank(out[j].DownlinkHz)
		if ri != rj {
			return ri < rj
		}
		return out[i].DownlinkHz < out[j].DownlinkHz
	})
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

func amateurRank(hz int64) int {
	switch {
	case hz >= 144_000_000 && hz <= 148_000_000:
		return 0
	case hz >= 430_000_000 && hz <= 440_000_000:
		return 1
	case hz >= 1_260_000_000 && hz <= 1_300_000_000:
		return 2
	case hz >= 137_000_000 && hz <= 138_000_000:
		return 3
	case hz >= 10_489_000_000 && hz <= 10_500_000_000:
		return 4
	default:
		return 8
	}
}

func defaultTransmitters() map[int][]Signal {
	return mergeTransmitterTables(builtinTransmitters(), embeddedTransmitters())
}

func embeddedTransmitters() map[int][]Signal {
	embeddedTransmittersOnce.Do(func() {
		data, err := embeddedTransmitterFile.ReadFile("transmitters_catalog.json")
		if err != nil {
			embeddedTransmitterTable = map[int][]Signal{}
			return
		}
		table, err := parseTransmitterTable(data)
		if err != nil || len(table) == 0 {
			embeddedTransmitterTable = map[int][]Signal{}
			return
		}
		embeddedTransmitterTable = table
	})
	return embeddedTransmitterTable
}

func (t *Tracker) attachSignalsLocked() {
	for i := range t.satellites {
		norad := t.satellites[i].NORAD
		t.satellites[i].Signals = mergeSignals(knownSignals(norad), t.transmitters[norad], t.satellites[i].Signals)
	}
}

func (t *Tracker) loadTransmitters() error {
	data, err := os.ReadFile(t.transmitterPath)
	if err != nil {
		return err
	}
	table, err := parseTransmitterTable(data)
	if err != nil || len(table) == 0 {
		return fmt.Errorf("invalid transmitter cache")
	}
	t.transmitters = mergeTransmitterTables(defaultTransmitters(), table)
	return nil
}

func (t *Tracker) saveTransmitters() error {
	if t.transmitterPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(t.transmitters, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(t.transmitterPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(t.transmitterPath, data, 0o644)
}

func mergeTransmitterTables(base, extra map[int][]Signal) map[int][]Signal {
	out := make(map[int][]Signal, len(base)+len(extra))
	for norad, list := range base {
		out[norad] = append([]Signal(nil), list...)
	}
	for norad, list := range extra {
		out[norad] = mergeSignals(out[norad], list)
	}
	return out
}

func parseTransmitterTable(data []byte) (map[int][]Signal, error) {
	var stored map[int][]Signal
	if json.Unmarshal(data, &stored) == nil && len(stored) > 0 {
		return stored, nil
	}
	return parseSatnogsTransmitters(data)
}

func parseSatnogsTransmitters(data []byte) (map[int][]Signal, error) {
	list, err := decodeSatnogsList(data)
	if err != nil {
		return nil, err
	}
	out := map[int][]Signal{}
	for _, item := range list {
		if item.NORAD <= 0 || !satnogsUsable(item) {
			continue
		}
		hz := satnogsDownlink(item)
		if hz < 1_000_000 || hz > 40_000_000_000 {
			continue
		}
		name := strings.TrimSpace(item.Description)
		if name == "" {
			name = strings.TrimSpace(item.Type)
		}
		if name == "" {
			name = "Downlink"
		}
		mode := strings.TrimSpace(item.Mode)
		if mode == "" {
			mode = "FM"
		}
		out[item.NORAD] = append(out[item.NORAD], Signal{Name: name, Mode: mode, DownlinkHz: hz})
	}
	for norad, list := range out {
		out[norad] = mergeSignals(list)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no usable transmitters")
	}
	return out, nil
}

func decodeSatnogsList(data []byte) ([]satnogsTransmitter, error) {
	var page satnogsPage
	if json.Unmarshal(data, &page) == nil && len(page.Results) > 0 {
		return page.Results, nil
	}
	var list []satnogsTransmitter
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func satnogsUsable(item satnogsTransmitter) bool {
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "invalid" {
		return false
	}
	if item.Alive != nil && !*item.Alive && status != "active" {
		return false
	}
	return status == "" || status == "active" || status == "inactive"
}

func satnogsDownlink(item satnogsTransmitter) int64 {
	switch {
	case item.DownlinkLow != nil && item.DownlinkHigh != nil && *item.DownlinkHigh > *item.DownlinkLow:
		return int64((*item.DownlinkLow + *item.DownlinkHigh) / 2)
	case item.DownlinkLow != nil && *item.DownlinkLow > 0:
		return int64(*item.DownlinkLow)
	case item.DownlinkHigh != nil && *item.DownlinkHigh > 0:
		return int64(*item.DownlinkHigh)
	default:
		return 0
	}
}

func fetchSatnogsTransmitters(ctx context.Context, client *http.Client) (map[int][]Signal, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	url := satnogsTransmittersURL
	var all []satnogsTransmitter
	for page := 0; page < 20 && url != ""; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "IC-SDR-English/0.5")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("SatNOGS HTTP %d", resp.StatusCode)
		}
		var p satnogsPage
		if json.Unmarshal(data, &p) == nil && len(p.Results) > 0 {
			all = append(all, p.Results...)
			url = strings.TrimSpace(p.Next)
			continue
		}
		list, err := decodeSatnogsList(data)
		if err != nil {
			return nil, err
		}
		all = append(all, list...)
		url = ""
	}
	encoded, err := json.Marshal(all)
	if err != nil {
		return nil, err
	}
	return parseSatnogsTransmitters(encoded)
}

func DemodForSignal(mode string) string {
	m := strings.ToUpper(mode)
	switch {
	case strings.Contains(m, "LSB"):
		return "LSB"
	case strings.Contains(m, "USB") || strings.Contains(m, "SSB") || strings.Contains(m, "CW") || strings.Contains(m, "BPSK") || strings.Contains(m, "QPSK") || strings.Contains(m, "PSK") || strings.Contains(m, "GMSK") || strings.Contains(m, "DVB"):
		return "USB"
	case strings.Contains(m, "APT") || m == "AM":
		return "AM"
	case strings.Contains(m, "WFM"):
		return "WFM"
	default:
		return "NFM"
	}
}

func FormatSignalLabel(sig Signal) string {
	freq := fmt.Sprintf("%.3f MHz", float64(sig.DownlinkHz)/1e6)
	if sig.DownlinkHz >= 1_000_000_000 {
		freq = fmt.Sprintf("%.4f GHz", float64(sig.DownlinkHz)/1e9)
	}
	return sig.Name + " · " + sig.Mode + " · " + freq
}
