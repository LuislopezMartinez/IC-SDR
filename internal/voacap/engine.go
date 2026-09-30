package voacap

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var bandFrequency = map[string]float64{
	"80 m": 3.6, "60 m": 5.357, "40 m": 7.1, "30 m": 10.12,
	"20 m": 14.1, "17 m": 18.1, "15 m": 21.1, "12 m": 24.93, "10 m": 28.4,
}

var engineRunMu sync.Mutex

// Calculate runs the official NTIA/ITS VOACAP 08.0121W area engine bundled
// with GO-Zero. A temporary DOS drive handles portable paths with spaces.
func Calculate(ctx context.Context, runtimeRoot string, settings Settings) Prediction {
	engineRunMu.Lock()
	defer engineRunMu.Unlock()
	result := Prediction{Engine: "VOACAP 08.0121W"}
	if runtime.GOOS != "windows" {
		result.Error = "VOACAP 08.0121W REQUIERE WINDOWS"
		return result
	}
	if _, err := os.Stat(filepath.Join(runtimeRoot, "bin_win", "Voacapw.exe")); err != nil {
		result.Error = "MOTOR VOACAP PORTABLE NO ENCONTRADO"
		return result
	}
	selected := selectedFrequencies(settings)
	if len(selected) == 0 {
		result.Error = "SELECCIONA AL MENOS UNA BANDA"
		return result
	}
	stagedRoot, removeStage, err := stageRuntime(runtimeRoot)
	if err != nil {
		result.Error = "NO SE PUDO PREPARAR VOACAP: " + err.Error()
		return result
	}
	defer removeStage()
	engineRoot, err := shortEnginePath(stagedRoot)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	areaDir := filepath.Join(stagedRoot, "areadata", "gozero")
	if err = os.MkdirAll(areaDir, 0o755); err != nil {
		result.Error = err.Error()
		return result
	}
	for index, item := range selected {
		base := fmt.Sprintf("goz%02d", index)
		inputPath := filepath.Join(areaDir, base+".voa")
		gridPath := filepath.Join(areaDir, base+".vg1")
		defer os.Remove(inputPath)
		defer os.Remove(gridPath)
		_ = os.Remove(gridPath)
		if err = os.WriteFile(inputPath, []byte(buildAreaInput(settings, item.mhz)), 0o644); err != nil {
			result.Error = err.Error()
			return result
		}
		command := exec.CommandContext(ctx, filepath.Join(engineRoot, "bin_win", "Voacapw.exe"), "SILENT", engineRoot, "AREA", "CALC", "GOZERO\\"+base+".VOA")
		command.Dir = filepath.Join(engineRoot, "run")
		output, runErr := command.CombinedOutput()
		_ = os.Remove(inputPath)
		if runErr != nil {
			result.Error = fmt.Sprintf("VOACAP: %v %s", runErr, strings.TrimSpace(string(output)))
			return result
		}
		cells, parseErr := parseAreaGrid(gridPath, item.band)
		_ = os.Remove(gridPath)
		if parseErr != nil {
			result.Error = parseErr.Error()
			return result
		}
		mergeCells(&result.Cells, cells)
	}
	result.Calculated = time.Now().UTC()
	return result
}

func stageRuntime(source string) (string, func(), error) {
	name := fmt.Sprintf("GZV%05d", os.Getpid()%100000)
	destination := filepath.Join(os.TempDir(), name)
	if err := os.RemoveAll(destination); err != nil {
		return "", func() {}, err
	}
	if err := copyTree(source, destination); err != nil {
		_ = os.RemoveAll(destination)
		return "", func() {}, err
	}
	// Empty directories are not retained by the portable release packaging.
	// VOACAP nevertheless requires RUN as its working directory before either
	// point-to-point or area calculations can create their input/output files.
	for _, directory := range []string{"run", "areadata", "area_inv"} {
		if err := os.MkdirAll(filepath.Join(destination, directory), 0o755); err != nil {
			_ = os.RemoveAll(destination)
			return "", func() {}, err
		}
	}
	return destination, func() { _ = os.RemoveAll(destination) }, nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

type frequency struct {
	band string
	mhz  float64
}

func selectedFrequencies(settings Settings) []frequency {
	result := make([]frequency, 0, len(settings.Bands))
	for _, band := range AmateurBands {
		if settings.Bands[band] {
			result = append(result, frequency{band, bandFrequency[band]})
		}
	}
	return result
}

func buildAreaInput(settings Settings, frequency float64) string {
	now := time.Now().UTC()
	noise := 145
	if settings.Noise == "RESIDENCIAL" {
		noise = 155
	}
	if settings.Noise == "URBANO" {
		noise = 165
	}
	requiredSNR := 38
	if settings.Mode == "CW" {
		requiredSNR = 24
	}
	if settings.Mode == "DIGITAL" {
		requiredSNR = 12
	}
	lat := coordinate(settings.Latitude, 'N', 'S')
	lon := coordinate(settings.Longitude, 'E', 'W')
	return fmt.Sprintf(`Model    :VOACAP
Colors   :Black    :Blue     :Ignore   :Ignore   :Red      :Black with shading
Cities   :Receive.cty
Nparms   :    4
Parameter:MUF      0
Parameter:DBU      0
Parameter:SNRxx    0
Parameter:REL      0
Transmit :%10s%10s%20s Short
Area     :    -180.0     180.0     -90.0      90.0
Gridsize :   31    1
Method   :   30
Coeffs   :CCIR
Months   :%7.2f   0.00   0.00   0.00   0.00   0.00   0.00   0.00   0.00
Ssns     :%7d      0      0      0      0      0      0      0      0
Hours    :%7d      0      0      0      0      0      0      0      0
Freqs    :%7.3f  0.000  0.000  0.000  0.000  0.000  0.000  0.000  0.000
System   :%5d     0.100   90%5d     3.000     0.100
Fprob    : 1.00 1.00 1.00 0.00
Rec Ants :[DEFAULT \SWWHIP.VOA  ]  gain=   0.0   0.0
Tx Ants  :[DEFAULT \CONST17.VOA ]  0.000   0.0%10.4f
`, lat, lon, truncate(settings.QTH, 20), float64(now.Month()), settings.SSN, settings.UTCHour, frequency, noise, requiredSNR, float64(settings.PowerW)/1000)
}

func parseAreaGrid(path, band string) ([]Cell, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("VOACAP NO GENERÓ LA CUADRÍCULA")
	}
	defer file.Close()
	cells := make([]Cell, 0, 961)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 104 {
			continue
		}
		fields := strings.Fields(line[:40])
		if len(fields) < 4 {
			continue
		}
		lat, latErr := strconv.ParseFloat(fields[2], 64)
		lon, lonErr := strconv.ParseFloat(fields[3], 64)
		reliability, relErr := strconv.ParseFloat(strings.TrimSpace(line[98:104]), 64)
		if latErr != nil || lonErr != nil || relErr != nil {
			continue
		}
		if lon > 180 {
			lon -= 360
		}
		cells = append(cells, Cell{Latitude: lat, Longitude: lon, Reliability: map[string]float64{band: reliability * 100}})
	}
	if len(cells) == 0 {
		return nil, errors.New("VOACAP DEVOLVIÓ UNA CUADRÍCULA VACÍA")
	}
	return cells, scanner.Err()
}

func mergeCells(destination *[]Cell, source []Cell) {
	if len(*destination) == 0 {
		*destination = source
		return
	}
	for index := range source {
		if index >= len(*destination) {
			*destination = append(*destination, source[index])
			continue
		}
		for band, value := range source[index].Reliability {
			(*destination)[index].Reliability[band] = value
		}
	}
}

func coordinate(value float64, positive, negative byte) string {
	direction := positive
	if value < 0 {
		value, direction = -value, negative
	}
	return fmt.Sprintf("%.2f%c", value, direction)
}
func truncate(value string, length int) string {
	if len(value) > length {
		return value[:length]
	}
	return value
}
