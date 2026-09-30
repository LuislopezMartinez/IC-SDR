package voacap

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type RouteLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type RoutePrediction struct {
	Reliability map[string][]float64 `json:"reliability,omitempty"`
	MUF         []float64            `json:"muf,omitempty"`
	DistanceKM  float64              `json:"distanceKm,omitempty"`
	Azimuth     float64              `json:"azimuth,omitempty"`
	Calculated  time.Time            `json:"calculated,omitempty"`
	Engine      string               `json:"engine,omitempty"`
	Error       string               `json:"error,omitempty"`
}

func CalculateRoute(ctx context.Context, runtimeRoot string, settings Settings, tx, rx RouteLocation) RoutePrediction {
	engineRunMu.Lock()
	defer engineRunMu.Unlock()
	result := RoutePrediction{Engine: "VOACAP 08.0121W"}
	if _, err := os.Stat(filepath.Join(runtimeRoot, "bin_win", "Voacapw.exe")); err != nil {
		result.Error = "MOTOR VOACAP PORTABLE NO ENCONTRADO"
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
	inputName, outputName := "gozroute.dat", "gozroute.out"
	inputPath := filepath.Join(stagedRoot, "run", inputName)
	outputPath := filepath.Join(stagedRoot, "run", outputName)
	if err = os.WriteFile(inputPath, []byte(buildRouteInput(settings, tx, rx)), 0o644); err != nil {
		result.Error = err.Error()
		return result
	}
	command := exec.CommandContext(ctx, filepath.Join(engineRoot, "bin_win", "Voacapw.exe"), "SILENT", engineRoot, inputName, outputName)
	command.Dir = filepath.Join(engineRoot, "run")
	if output, runErr := command.CombinedOutput(); runErr != nil {
		result.Error = fmt.Sprintf("VOACAP: %v %s", runErr, strings.TrimSpace(string(output)))
		return result
	}
	result, err = parseRouteOutput(outputPath)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Engine = "VOACAP 08.0121W"
	result.Calculated = time.Now().UTC()
	result.DistanceKM, result.Azimuth = routeGeometry(tx, rx)
	return result
}

func buildRouteInput(settings Settings, tx, rx RouteLocation) string {
	requiredSNR := 38
	if settings.Mode == "CW" {
		requiredSNR = 24
	} else if settings.Mode == "DIGITAL" {
		requiredSNR = 12
	}
	noise := 145
	if settings.Noise == "RESIDENCIAL" {
		noise = 155
	} else if settings.Noise == "URBANO" {
		noise = 165
	}
	frequencies := ""
	for _, band := range AmateurBands {
		frequencies += fmt.Sprintf("%6.2f", bandFrequency[band])
	}
	return fmt.Sprintf(`COMMENT    GO-Zero 24-hour route
LINEMAX      55
COEFFS    CCIR
TIME          1   24    1    1
MONTH      %4d%5.2f
SUNSPOT %7.1f
LABEL     TX                  RX
CIRCUIT   %6s%10s%10s%10s  S     0
SYSTEM       1. %3d. 3.00  90. %4.1f 3.00 0.10
FPROB      1.00 1.00 1.00 0.00
ANTENNA       1    1    2   30     0.000[default\CONST17.VOA  ]  0.0 %9.4f
ANTENNA       2    2    2   30     0.000[default\SWWHIP.VOA   ]  0.0    0.0000
FREQUENCY%s  0.00  0.00
METHOD       30    0
EXECUTE
QUIT
`, time.Now().UTC().Year(), float64(time.Now().UTC().Month()), float64(settings.SSN), coordinate(tx.Latitude, 'N', 'S'), coordinate(tx.Longitude, 'E', 'W'), coordinate(rx.Latitude, 'N', 'S'), coordinate(rx.Longitude, 'E', 'W'), noise, float64(requiredSNR), float64(settings.PowerW)/1000, frequencies)
}

func parseRouteOutput(path string) (RoutePrediction, error) {
	file, err := os.Open(path)
	if err != nil {
		return RoutePrediction{}, errors.New("VOACAP NO GENERÓ EL ANÁLISIS DE RUTA")
	}
	defer file.Close()
	result := RoutePrediction{Reliability: make(map[string][]float64), MUF: make([]float64, 24)}
	for _, band := range AmateurBands {
		result.Reliability[band] = make([]float64, 24)
	}
	pendingHour := -1
	parsedHours := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 12 && fields[len(fields)-1] == "FREQ" {
			hourValue, hourErr := strconv.ParseFloat(fields[0], 64)
			muf, mufErr := strconv.ParseFloat(fields[1], 64)
			if hourErr == nil && mufErr == nil {
				pendingHour = int(math.Round(hourValue)) % 24
				if pendingHour >= 0 && pendingHour < 24 {
					result.MUF[pendingHour] = muf
					parsedHours++
				}
			}
			continue
		}
		if pendingHour < 0 || pendingHour >= 24 || len(fields) < len(AmateurBands)+2 || fields[len(fields)-1] != "REL" {
			continue
		}
		for index, band := range AmateurBands {
			// Column zero is VOACAP's automatically selected optimum frequency;
			// the nine requested amateur-band frequencies start at column one.
			value, parseErr := strconv.ParseFloat(fields[index+1], 64)
			if parseErr == nil {
				result.Reliability[band][pendingHour] = min(max(value*100, 0), 100)
			}
		}
		pendingHour = -1
	}
	if err = scanner.Err(); err != nil {
		return RoutePrediction{}, err
	}
	if parsedHours != 24 {
		return RoutePrediction{}, errors.New("VOACAP DEVOLVIÓ UNA RUTA VACÍA")
	}
	return result, nil
}

func routeGeometry(tx, rx RouteLocation) (float64, float64) {
	lat1, lat2 := tx.Latitude*math.Pi/180, rx.Latitude*math.Pi/180
	dLon := (rx.Longitude - tx.Longitude) * math.Pi / 180
	a := math.Sin((lat2-lat1)/2)*math.Sin((lat2-lat1)/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	distance := 6371.0088 * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	bearing := math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
	return distance, bearing
}
