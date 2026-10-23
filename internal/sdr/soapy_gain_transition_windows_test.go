//go:build windows

package sdr

import "testing"

func TestRTLSDRLeavingAGCReappliesUnchangedManualGain(t *testing.T) {
	agc := true
	var gains []float64
	api := &soapyAPI{
		getGainMode:    func(uintptr, int32, uintptr) bool { return agc },
		setGainMode:    func(_ uintptr, _ int32, _ uintptr, value bool) int32 { agc = value; return 0 },
		getGainElement: func(uintptr, int32, uintptr, string) float64 { return 40 },
		setGainElement: func(_ uintptr, _ int32, _ uintptr, _ string, value float64) int32 {
			gains = append(gains, value)
			return 0
		},
		getFrequencyCorrection: func(uintptr, int32, uintptr) float64 { return 0 },
		readSetting:            func(uintptr, string) uintptr { return 0 },
	}
	device := &soapyDevice{api: api, device: 1, driver: "rtlsdr"}
	if err := device.applyHardwareSettings(HardwareSettings{AGC: false, RFGain: 40}); err != nil {
		t.Fatal(err)
	}
	if agc || len(gains) != 1 || gains[0] != 40 {
		t.Fatalf("leaving AGC did not restore tuner gain: agc=%v gains=%v", agc, gains)
	}
}

func TestAirspyLeavingAGCReappliesAllManualGainStages(t *testing.T) {
	agc := true
	values := map[string]float64{"LNA": 8, "MIX": 7, "VGA": 6}
	var stages []string
	api := &soapyAPI{
		getGainMode: func(uintptr, int32, uintptr) bool { return agc },
		setGainMode: func(_ uintptr, _ int32, _ uintptr, value bool) int32 { agc = value; return 0 },
		getGainElement: func(_ uintptr, _ int32, _ uintptr, name string) float64 {
			return values[name]
		},
		setGainElement: func(_ uintptr, _ int32, _ uintptr, name string, value float64) int32 {
			stages = append(stages, name)
			values[name] = value
			return 0
		},
		readSetting: func(uintptr, string) uintptr { return 0 },
	}
	device := &soapyDevice{api: api, device: 1, driver: "airspy"}
	want := HardwareSettings{AGC: false, RFGain: 8, IFGain: 7, BasebandGain: 6}
	if err := device.applyHardwareSettings(want); err != nil {
		t.Fatal(err)
	}
	if agc || len(stages) != 3 || stages[0] != "LNA" || stages[1] != "MIX" || stages[2] != "VGA" {
		t.Fatalf("leaving AGC did not restore every Airspy gain stage: agc=%v stages=%v", agc, stages)
	}
}
