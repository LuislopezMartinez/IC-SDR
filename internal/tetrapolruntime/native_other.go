//go:build !windows

package tetrapolruntime

import "fmt"

type nativeAPI struct{}

func loadNativeAPI(string) (nativeAPI, error) {
	return nativeAPI{}, fmt.Errorf("liliPOL native is only available on Windows")
}

func (nativeAPI) setContext(uintptr, string, int)            {}
func (*nativeAPI) close()                                    {}
func (*nativeAPI) create(int) uintptr                        { return 0 }
func (*nativeAPI) createConfigured(int, int, int) uintptr    { return 0 }
func (*nativeAPI) destroy(uintptr)                           {}
func (*nativeAPI) setSampleRate(uintptr, float64)            {}
func (*nativeAPI) setAudioCallback(uintptr, func([]float32)) {}
func (*nativeAPI) clearAudioCallback(uintptr)                {}
func (*nativeAPI) processIQ(uintptr, []float32)              {}
func (*nativeAPI) snapshot(uintptr) (Telemetry, error)       { return Telemetry{}, fmt.Errorf("unavailable") }
