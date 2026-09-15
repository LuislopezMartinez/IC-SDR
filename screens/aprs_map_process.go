package screens

import (
	"go-zero/internal/i18n"

	"encoding/json"
	"go-zero/internal/aprs"
	"os"
	"os/exec"
)

func (p *APRSPanel) writeMapSnapshot(packets []aprs.Packet) {
	aprs.UpdateMapStations(p.stations, packets)
	data, err := json.Marshal(aprs.MapStationList(p.stations))
	if err != nil || string(data) == p.lastMapSnapshot {
		return
	}
	if replaceLiveSnapshot(p.mapPath, data) == nil {
		p.lastMapSnapshot = string(data)
	}
}
func (p *APRSPanel) openMap() {
	p.writeMapSnapshot(p.packets())
	if p.mapViewer != nil && p.mapViewer.Process != nil {
		focusRTL433Viewer(p.mapViewer.Process.Pid)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		p.say(i18n.Source("text.590655f681fb"))
		return
	}
	cmd := exec.Command(exe, "--aprs-map", p.mapPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if cmd.Start() != nil {
		p.say(i18n.Source("text.590655f681fb"))
		return
	}
	p.mapViewer = cmd
	p.mapDone = make(chan struct{})
	done := p.mapDone
	go func() { _ = cmd.Wait(); close(done) }()
	p.say(i18n.Source("text.2731df8cb51c"))
}
