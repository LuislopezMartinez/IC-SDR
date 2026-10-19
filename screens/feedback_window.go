package screens

import (
	"fmt"
	"log"
	"strings"
	"time"

	"go-zero/internal/buildinfo"
	"go-zero/internal/feedback"
	"go-zero/internal/resources"
)

func (screen *MainScreen) openFeedbackWindow() {
	if screen.feedbackWindow == nil {
		screen.feedbackWindow = feedback.NewWindow(resources.WritablePath("config", "feedback.json"))
	}
	windowsName, architecture := feedback.PlatformDescription()
	now := time.Now()
	zone, _ := now.Zone()
	info := feedback.TechnicalInfo{
		Version:      buildinfo.DisplayVersion(),
		BuildDate:    buildinfo.BuildDate,
		Revision:     buildinfo.Revision,
		Windows:      windowsName,
		Architecture: architecture,
		LocalTime:    now.Format("02/01/2006 · 15:04"),
		TimeZone:     zone,
		Band:         screen.bandName,
		Frequency:    fmt.Sprintf("%.6f MHz", float64(screen.frequencyHz)/1e6),
	}
	if screen.mode != nil {
		info.Mode = screen.mode.SelectedText()
	}
	if screen.receiver != nil {
		hardware := screen.receiver.HardwareSettings()
		info.SDR = strings.TrimSpace(hardware.Device)
		if info.SDR == "" {
			info.SDR = strings.ToUpper(hardware.Driver)
		}
		if hardware.Available {
			info.SDRStatus = "Conectado"
		} else {
			info.SDRStatus = "No detectado"
		}
	}
	if err := screen.feedbackWindow.Open(info); err != nil {
		log.Printf("No se pudo abrir la ventana de reportes: %v", err)
	}
}
