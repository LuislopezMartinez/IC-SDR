package bandplan

// Range is a painted allocation on the spectrum strip.
type Range struct {
	Category, Name string
	MinHz, MaxHz   int64
}

// Tune is a band-selector center/span preset.
type Tune struct {
	Category, Name string
	FrequencyHz    int64
	SpanHz         int64
}

// Plan is the amateur/commercial/ISM map for a region or country.
type Plan struct {
	ID, Name, ITU string
	APRSHz        int64
	Default       Tune
	Ranges        []Range
	Selector      map[string][]Tune
}

var registry = map[string]Plan{}

func register(plan Plan) { registry[plan.ID] = plan }

func Get(id string) (Plan, bool) {
	plan, ok := registry[Normalize(id)]
	return plan, ok
}

func Must(id string) Plan {
	plan, ok := Get(id)
	if !ok {
		plan, _ = Get("itu-r1")
	}
	return plan
}

func All() []Plan {
	order := []string{
		"itu-r1", "itu-r2", "itu-r3",
		"uk", "de", "es", "fr", "it", "pl", "nl", "be", "at", "ch", "ie", "pt",
		"se", "no", "fi", "dk", "cz", "hu", "ro", "tr", "ua", "gr", "bg", "za", "ru",
		"us", "ca", "mx", "br", "ar", "cl", "co", "pe", "ve", "uy",
		"au", "nz", "jp", "in", "cn", "kr", "id", "ph", "sg", "my", "tw", "vn", "th", "hk",
	}
	out := make([]Plan, 0, len(order))
	for _, id := range order {
		if plan, ok := registry[id]; ok {
			out = append(out, plan)
		}
	}
	return out
}

func Normalize(id string) string {
	if id == "" {
		return "itu-r1"
	}
	if _, ok := registry[id]; ok {
		return id
	}
	switch id {
	case "R1", "r1", "ITU-R1", "europe":
		return "itu-r1"
	case "R2", "r2", "ITU-R2", "americas":
		return "itu-r2"
	case "R3", "r3", "ITU-R3", "asia", "pacific":
		return "itu-r3"
	}
	return "itu-r1"
}

// Resolve picks the most specific shipped plan. Country wins when we have one.
func Resolve(region, country string) string {
	if country != "" && country != "auto" {
		if _, ok := registry[country]; ok {
			return country
		}
	}
	switch Normalize(region) {
	case "itu-r2":
		return "itu-r2"
	case "itu-r3":
		return "itu-r3"
	default:
		return "itu-r1"
	}
}

func RegionOf(id string) string {
	plan, ok := Get(id)
	if !ok {
		return "R1"
	}
	return plan.ITU
}

func CountriesFor(region string) []Plan {
	want := "R1"
	switch Normalize(region) {
	case "itu-r2":
		want = "R2"
	case "itu-r3":
		want = "R3"
	}
	var out []Plan
	for _, plan := range All() {
		if plan.ITU != want {
			continue
		}
		if plan.ID == "itu-r1" || plan.ID == "itu-r2" || plan.ID == "itu-r3" {
			continue
		}
		out = append(out, plan)
	}
	return out
}

func clone(plan Plan, id, name, itu string) Plan {
	next := plan
	next.ID, next.Name, next.ITU = id, name, itu
	next.Ranges = append([]Range(nil), plan.Ranges...)
	next.Selector = map[string][]Tune{}
	for category, list := range plan.Selector {
		next.Selector[category] = append([]Tune(nil), list...)
	}
	return next
}

func replaceRange(plan *Plan, category, name string, minHz, maxHz int64) {
	for i := range plan.Ranges {
		if plan.Ranges[i].Category == category && plan.Ranges[i].Name == name {
			plan.Ranges[i].MinHz, plan.Ranges[i].MaxHz = minHz, maxHz
			return
		}
	}
	plan.Ranges = append(plan.Ranges, Range{Category: category, Name: name, MinHz: minHz, MaxHz: maxHz})
}

func removeRange(plan *Plan, category, name string) {
	filtered := plan.Ranges[:0]
	for _, item := range plan.Ranges {
		if item.Category == category && item.Name == name {
			continue
		}
		filtered = append(filtered, item)
	}
	plan.Ranges = filtered
}

func replaceTune(plan *Plan, category, name string, freq, span int64) {
	list := plan.Selector[category]
	for i := range list {
		if list[i].Name == name {
			list[i].FrequencyHz, list[i].SpanHz = freq, span
			plan.Selector[category] = list
			return
		}
	}
	plan.Selector[category] = append(list, Tune{Category: category, Name: name, FrequencyHz: freq, SpanHz: span})
}

func removeTune(plan *Plan, category, name string) {
	list := plan.Selector[category]
	filtered := list[:0]
	for _, item := range list {
		if item.Name == name {
			continue
		}
		filtered = append(filtered, item)
	}
	plan.Selector[category] = filtered
}
