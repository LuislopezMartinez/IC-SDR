package screens

import "fmt"

const (
	tetrapolPlanUHF125 = "UHF-12.5"
	tetrapolPlanUHF10  = "UHF-10"
	tetrapolPlanVHF    = "VHF"
)

type tetrapolChannelPlan struct {
	id, label     string
	band          string
	channelZeroHz int64
	stepHz        int64
	minimumHz     int64
	maximumHz     int64
}

var tetrapolChannelPlans = map[string]tetrapolChannelPlan{
	tetrapolPlanUHF125: {tetrapolPlanUHF125, "UHF 12,5 kHz", "UHF", 358_400_000, 12_500, 380_000_000, 400_000_000},
	tetrapolPlanUHF10:  {tetrapolPlanUHF10, "UHF 10 kHz", "UHF", 368_645_000, 10_000, 380_000_000, 400_000_000},
	tetrapolPlanVHF:    {tetrapolPlanVHF, "VHF", "VHF", 0, 12_500, 0, 0},
}

func tetrapolPlan(id string) tetrapolChannelPlan {
	if plan, exists := tetrapolChannelPlans[id]; exists {
		return plan
	}
	return tetrapolChannelPlans[tetrapolPlanUHF125]
}

// tetrapolChannelFrequency converts the common UHF TETRAPOL channel plan used
// by tetrapol-kit. VHF plans are deployment-specific, so they are deliberately
// not guessed.
func tetrapolChannelFrequency(band string, controlHz int64, channelID int) (int64, error) {
	planID := tetrapolPlanUHF125
	if band == "VHF" {
		planID = tetrapolPlanVHF
	}
	return tetrapolChannelFrequencyForPlan(planID, controlHz, channelID)
}

func tetrapolChannelFrequencyForPlan(planID string, controlHz int64, channelID int) (int64, error) {
	plan := tetrapolPlan(planID)
	if plan.band != "UHF" || controlHz < plan.minimumHz || controlHz > plan.maximumHz {
		return 0, fmt.Errorf("seguimiento automático no disponible para el plan %s", plan.label)
	}
	if channelID < 0 || channelID > 4095 {
		return 0, fmt.Errorf("canal TETRAPOL inválido: %d", channelID)
	}
	hz := plan.channelZeroHz + int64(channelID)*plan.stepHz
	if hz < plan.minimumHz || hz > plan.maximumHz {
		return 0, fmt.Errorf("canal %d fuera del plan %s", channelID, plan.label)
	}
	return hz, nil
}
