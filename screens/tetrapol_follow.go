package screens

import "fmt"

const (
	tetrapolUHFChannelZeroHz = int64(358_400_000)
	tetrapolChannelStepHz    = int64(12_500)
)

// tetrapolChannelFrequency converts the common UHF TETRAPOL channel plan used
// by tetrapol-kit. VHF plans are deployment-specific, so they are deliberately
// not guessed.
func tetrapolChannelFrequency(band string, controlHz int64, channelID int) (int64, error) {
	if band != "UHF" || controlHz < 380_000_000 || controlHz > 400_000_000 {
		return 0, fmt.Errorf("seguimiento automático disponible para el plan UHF 380–400 MHz")
	}
	if channelID < 0 || channelID > 4095 {
		return 0, fmt.Errorf("canal TETRAPOL inválido: %d", channelID)
	}
	hz := tetrapolUHFChannelZeroHz + int64(channelID)*tetrapolChannelStepHz
	if hz < 380_000_000 || hz > 400_000_000 {
		return 0, fmt.Errorf("canal %d fuera del plan UHF 380–400 MHz", channelID)
	}
	return hz, nil
}
