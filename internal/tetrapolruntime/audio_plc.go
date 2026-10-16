package tetrapolruntime

import (
	"math"
	"sync"
	"time"
)

const (
	tetrapolAudioRate       = 8_000
	tetrapolVoiceFrame      = 20 * time.Millisecond
	tetrapolMaxConcealments = 3
)

// audioPLC hides isolated RP-CELP output gaps. It deliberately stops after
// three frames (60 ms): beyond that point silence is less misleading than
// synthesizing speech that was never received.
type audioPLC struct {
	mu                           sync.Mutex
	lastFrame                    []float32
	lastAt                       time.Time
	decoded, concealed, longGaps uint64
}

func (plc *audioPLC) reset() {
	plc.mu.Lock()
	plc.lastFrame, plc.lastAt = nil, time.Time{}
	plc.decoded, plc.concealed, plc.longGaps = 0, 0, 0
	plc.mu.Unlock()
}

func (plc *audioPLC) process(input []float32, now time.Time) []float32 {
	plc.mu.Lock()
	defer plc.mu.Unlock()
	if len(input) == 0 {
		return nil
	}
	current := append([]float32(nil), input...)
	missing := 0
	if len(plc.lastFrame) >= 80 && len(current) >= 80 && !plc.lastAt.IsZero() {
		gap := now.Sub(plc.lastAt)
		if gap > tetrapolVoiceFrame+tetrapolVoiceFrame/2 {
			missing = int(math.Round(float64(gap)/float64(tetrapolVoiceFrame))) - 1
			if missing > tetrapolMaxConcealments {
				plc.longGaps++
				missing = tetrapolMaxConcealments
			}
		}
	}
	output := make([]float32, 0, missing*len(plc.lastFrame)+len(current))
	for frame := 0; frame < missing; frame++ {
		gain := float32(math.Pow(.68, float64(frame+1)))
		for index, sample := range plc.lastFrame {
			// The slight within-frame fade prevents repeated RP-CELP pitch
			// periods from turning into a stationary buzz.
			taper := 1 - .18*float32(index)/float32(max(1, len(plc.lastFrame)-1))
			output = append(output, sample*gain*taper)
		}
		plc.concealed++
	}
	if missing > 0 {
		crossfade := min(24, len(current), len(plc.lastFrame))
		lastGain := float32(math.Pow(.68, float64(missing)))
		for index := 0; index < crossfade; index++ {
			mix := float32(index+1) / float32(crossfade+1)
			current[index] = plc.lastFrame[index]*lastGain*(1-mix) + current[index]*mix
		}
	}
	output = append(output, current...)
	plc.lastFrame = append(plc.lastFrame[:0], input...)
	plc.lastAt = now
	plc.decoded++
	return output
}

func (plc *audioPLC) stats() (decoded, concealed, longGaps uint64) {
	plc.mu.Lock()
	defer plc.mu.Unlock()
	return plc.decoded, plc.concealed, plc.longGaps
}
