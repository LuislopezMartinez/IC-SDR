// Package localtime resolves civil time from geographic coordinates without
// network access. It uses IANA timezone boundaries and Go's embedded tzdata so
// daylight-saving rules also work in portable Windows builds.
package localtime

import (
	"fmt"
	"math"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/ringsaturn/tzf/v2"
)

type Info struct {
	Time      time.Time
	Zone      string
	UTCOffset string
}

var (
	finderOnce sync.Once
	finder     tzf.F
)

func At(now time.Time, lat, lon float64) (Info, bool) {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return Info{}, false
	}
	finderOnce.Do(func() { finder, _ = tzf.NewDefaultFinder() })
	if finder == nil {
		return Info{}, false
	}
	zone := finder.GetTimezoneName(lon, lat)
	if zone == "" {
		return Info{}, false
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return Info{}, false
	}
	local := now.In(location)
	_, seconds := local.Zone()
	return Info{Time: local, Zone: zone, UTCOffset: formatOffset(seconds)}, true
}

func formatOffset(seconds int) string {
	if seconds == 0 {
		return "UTC"
	}
	sign := "+"
	if seconds < 0 {
		sign, seconds = "-", -seconds
	}
	hours, minutes := seconds/3600, (seconds%3600)/60
	if minutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, hours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, hours, minutes)
}

func Label(now time.Time, lat, lon float64) (string, bool) {
	info, ok := At(now, lat, lon)
	if !ok {
		return "", false
	}
	return info.Time.Format("15:04") + " · " + info.UTCOffset, true
}
