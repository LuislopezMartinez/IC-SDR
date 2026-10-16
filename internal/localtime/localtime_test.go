package localtime

import (
	"testing"
	"time"
)

func TestAtUsesGeographicZoneAndDaylightSaving(t *testing.T) {
	winter := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
	summer := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	winterMadrid, ok := At(winter, 40.4168, -3.7038)
	if !ok || winterMadrid.Zone != "Europe/Madrid" || winterMadrid.Time.Hour() != 13 || winterMadrid.UTCOffset != "UTC+1" {
		t.Fatalf("Madrid winter = %+v, %v", winterMadrid, ok)
	}
	summerMadrid, ok := At(summer, 40.4168, -3.7038)
	if !ok || summerMadrid.Time.Hour() != 14 || summerMadrid.UTCOffset != "UTC+2" {
		t.Fatalf("Madrid summer = %+v, %v", summerMadrid, ok)
	}
}

func TestAtRejectsInvalidCoordinates(t *testing.T) {
	if _, ok := At(time.Now(), 95, 0); ok {
		t.Fatal("invalid latitude was accepted")
	}
}
