package screens

type tuningCenterPolicy int

const (
	tuningKeepVisible tuningCenterPolicy = iota
	tuningCenter
)

type tuningRequest struct {
	FrequencyHz int64
	Center      tuningCenterPolicy
	UpdateBand  bool
}

// applyTuning is the common low-level tuning transaction. Callers remain
// responsible for mode- or decoder-specific preparation, while frequency,
// center, band identity, receiver state and persistence move together.
func (screen *MainScreen) applyTuning(request tuningRequest) {
	if request.FrequencyHz < 1_000 {
		return
	}
	if screen.scanPanel != nil && screen.scanPanel.running {
		screen.scanPanel.Stop()
	}
	previousFrequency, previousCenter := screen.frequencyHz, screen.centerFrequencyHz
	screen.frequencyHz = request.FrequencyHz
	if request.UpdateBand {
		screen.updateBandForFrequency(request.FrequencyHz)
	}
	if request.Center == tuningCenter || screen.centerMode || screen.spanHz <= 0 {
		screen.centerFrequencyHz = request.FrequencyHz
	} else {
		halfSpan := screen.spanHz / 2
		guard := max(int64(screen.demodBandwidthHz), screen.spanHz/20)
		guard = min(guard, screen.spanHz*2/5)
		if request.FrequencyHz < screen.centerFrequencyHz-halfSpan+guard {
			screen.centerFrequencyHz = request.FrequencyHz + halfSpan - guard
		} else if request.FrequencyHz > screen.centerFrequencyHz+halfSpan-guard {
			screen.centerFrequencyHz = request.FrequencyHz - halfSpan + guard
		}
		screen.centerFrequencyHz = max(screen.centerFrequencyHz, halfSpan)
	}
	if screen.receiver != nil {
		if screen.centerFrequencyHz != previousCenter {
			screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
		}
		screen.receiver.SetDemodulator(screen.receiverDemodMode(), screen.frequencyHz, screen.demodBandwidthHz)
	}
	if screen.centerFrequencyHz != previousCenter && screen.waterfall != nil {
		screen.waterfall.Reset()
	}
	screen.resetTETRAAfterManualSpectrumTune(previousFrequency)
	screen.markSettingsDirty()
}
