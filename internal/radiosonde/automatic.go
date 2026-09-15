package radiosonde

import "go-zero/internal/i18n"

import "time"

type autoDetection struct {
	id    string
	frame int64
	count int
	last  time.Time
}

// All candidates remain active so a different family can be detected without
// restarting or cycling away while a short transmission is being received.
func (d *Decoder) startAutomatic(frequency, center int64) {
	d.mu.Lock()
	d.status = Status{Family: i18n.Source("text.6ea56fae9eac"), FrequencyHz: frequency, State: i18n.Source("text.f8bd2e05139c")}
	d.center = center
	d.detections = make(map[string]autoDetection)
	generation := d.generation
	d.mu.Unlock()
	children := make([]*Decoder, 0, len(Families))
	running := false
	for _, family := range Families {
		child := New(d.rate, d.directory)
		child.eventSink = func(e Event) { d.acceptAutomatic(generation, family, e) }
		child.Configure(true, family, frequency, center)
		running = running || child.Snapshot().Running
		children = append(children, child)
	}
	d.mu.Lock()
	d.children = children
	d.status.Running = running
	d.mu.Unlock()
}

func matchesFamily(family, typ string) bool {
	switch family {
	case "RS41":
		return typ == "RS41"
	case "DFM":
		return typ == i18n.Source("text.5945e8331ba7") || typ == i18n.Source("text.29ac0a15a8af") || typ == i18n.Source("text.7c83dc03fd40") || typ == i18n.Source("text.3b5b0b106f1c")
	case "M10/M20":
		return typ == "M10" || typ == "M20"
	}
	return false
}

func (d *Decoder) acceptAutomatic(generation uint64, family string, e Event) {
	if !matchesFamily(family, e.Type) || e.Lat == nil || e.Lon == nil || e.Alt == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if generation != d.generation {
		return
	}
	v := d.detections[family]
	if v.id != e.ID || time.Since(v.last) >= 15*time.Second {
		v = autoDetection{id: e.ID, frame: e.Frame, count: 1, last: time.Now()}
	} else if v.frame != e.Frame {
		v.count = min(v.count+1, 2)
		v.frame = e.Frame
		v.last = time.Now()
	}
	d.detections[family] = v
	d.events = append(d.events, e)
	if len(d.events) > 2000 {
		copy(d.events, d.events[len(d.events)-2000:])
		d.events = d.events[:2000]
	}
}
