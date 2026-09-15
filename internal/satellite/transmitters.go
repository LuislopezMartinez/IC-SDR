package satellite

import (
	"go-zero/internal/i18n"

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
		return []Signal{{i18n.Source("text.a9d54c84b79e"), "FM", 145800000}, {i18n.Source("text.4c4310fd27fd"), i18n.Source("text.c049f537536c"), 145825000}}
	case 43700:
		return []Signal{{i18n.Source("text.8ad7a21466a5"), i18n.Source("text.51a733b740d2"), 10489750000}, {i18n.Source("text.978cf75f6b20"), i18n.Source("text.a4c2d32fa824"), 10489500000}, {i18n.Source("text.903f1970c1d3"), i18n.Source("text.75d9d19a4ec6"), 10491000000}}
	default:
		return nil
	}
}

func builtinTransmitters() map[int][]Signal {
	table := map[int][]Signal{
		25544: knownSignals(25544),
		43700: knownSignals(43700),
		25338: {{i18n.Source("text.6e23a3697616"), "AM", 137620000}},
		28654: {{i18n.Source("text.6e23a3697616"), "AM", 137912500}},
		33591: {{i18n.Source("text.6e23a3697616"), "AM", 137100000}},
		40069: {{i18n.Source("text.2db1a87e1d05"), i18n.Source("text.bf956544920b"), 137100000}},
		44387: {{i18n.Source("text.2db1a87e1d05"), i18n.Source("text.bf956544920b"), 137900000}},
		27607: {{i18n.Source("text.02d98c5db46a"), "FM", 436795000}},
		43017: {{i18n.Source("text.02d98c5db46a"), "FM", 145880000}},
		24278: {{i18n.Source("text.b960614a7528"), i18n.Source("text.a4c2d32fa824"), 435800000}, {"Beacon", "CW", 435795000}},
		40908: {{"Telemetry", i18n.Source("text.51a733b740d2"), 145705000}},
		42761: {{"Telemetry", i18n.Source("text.51a733b740d2"), 145855000}},
		42759: {{"Telemetry", i18n.Source("text.51a733b740d2"), 145890000}},
		39444: {{"Telemetry", i18n.Source("text.51a733b740d2"), 145935000}},
		7530:  {{i18n.Source("text.b960614a7528"), i18n.Source("text.a4c2d32fa824"), 145950000}},
		39427: {{"Telemetry", i18n.Source("text.51a733b740d2"), 145935000}},
		44331: {{i18n.Source("text.02d98c5db46a"), "FM", 145840000}},
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
		return fmt.Errorf("%s", i18n.Source("text.141f4e071af5"))
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
		return nil, fmt.Errorf("%s", i18n.Source("text.4b8084281364"))
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
		req.Header.Set("User-Agent", i18n.Source("text.8be1e322b32b"))
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
			return nil, fmt.Errorf(i18n.Source("text.6c2deb4e51d8"), resp.StatusCode)
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
		return i18n.Source("text.6323db4948ad")
	case strings.Contains(m, "USB") || strings.Contains(m, "SSB") || strings.Contains(m, "CW") || strings.Contains(m, "BPSK") || strings.Contains(m, "QPSK") || strings.Contains(m, "PSK") || strings.Contains(m, "GMSK") || strings.Contains(m, "DVB"):
		return i18n.Source("text.61f0acff1735")
	case strings.Contains(m, "APT") || m == "AM":
		return "AM"
	case strings.Contains(m, "WFM"):
		return i18n.Source("text.6b742bac3eb4")
	default:
		return i18n.Source("text.0896d612d497")
	}
}

func FormatSignalLabel(sig Signal) string {
	freq := fmt.Sprintf(i18n.Source("text.a433787084ce"), float64(sig.DownlinkHz)/1e6)
	if sig.DownlinkHz >= 1_000_000_000 {
		freq = fmt.Sprintf(i18n.Source("text.06fbdd021efd"), float64(sig.DownlinkHz)/1e9)
	}
	return sig.Name + " · " + sig.Mode + " · " + freq
}
