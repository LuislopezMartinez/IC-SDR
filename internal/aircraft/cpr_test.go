package aircraft

import (
	"math"
	"testing"
	"time"
)

func encodeAirborne(lat, lon float64, odd bool) (int, int) {
	dlat := 360.0 / 60.0
	if odd {
		dlat = 360.0 / 59.0
	}
	yz := int(math.Floor(cprModFloat(lat, dlat)/dlat*131072+0.5)) % 131072
	if yz < 0 {
		yz += 131072
	}
	ni := cprN(lat, odd)
	dlon := 360.0 / float64(ni)
	xz := int(math.Floor(cprModFloat(lon, dlon)/dlon*131072+0.5)) % 131072
	if xz < 0 {
		xz += 131072
	}
	return yz, xz
}

func TestCPRAirborneBothHemispheres(t *testing.T) {
	points := []struct{ lat, lon float64 }{
		{52.25720, 3.91937},  // Netherlands
		{40.4168, -3.7038},   // Madrid
		{-33.8688, 151.2093}, // Sydney
		{-37.8136, 144.9631}, // Melbourne
		{-45.8788, 170.5028}, // Dunedin
		{61.2181, -149.9003}, // Anchorage
		{1.3521, 103.8198},   // Singapore
	}
	for _, p := range points {
		evenLat, evenLon := encodeAirborne(p.lat, p.lon, false)
		oddLat, oddLon := encodeAirborne(p.lat, p.lon, true)
		gotLat, gotLon, ok := decodeCPRAirborne(evenLat, evenLon, oddLat, oddLon, true)
		if !ok {
			t.Fatalf("global CPR failed at %.4f %.4f", p.lat, p.lon)
		}
		if math.Abs(gotLat-p.lat) > 0.002 || math.Abs(wrap180(gotLon-p.lon)) > 0.002 {
			t.Fatalf("global CPR mismatch at %+v: got %.5f %.5f", p, gotLat, gotLon)
		}
		relLat, relLon, ok := decodeCPRRelative(p.lat+0.02, p.lon-0.03, evenLat, evenLon, false, false)
		if !ok {
			t.Fatalf("relative CPR failed at %.4f %.4f", p.lat, p.lon)
		}
		if math.Abs(relLat-p.lat) > 0.002 || math.Abs(wrap180(relLon-p.lon)) > 0.002 {
			t.Fatalf("relative CPR mismatch at %+v: got %.5f %.5f", p, relLat, relLon)
		}
	}
}

func TestCPRNLIsSymmetricAboutEquator(t *testing.T) {
	for _, lat := range []float64{0, 10, 33.86, 45, 52.25, 80} {
		if cprNL(lat) != cprNL(-lat) {
			t.Fatalf("NL(%v)=%d NL(%v)=%d", lat, cprNL(lat), -lat, cprNL(-lat))
		}
	}
	if cprNL(0) != 59 {
		t.Fatalf("equator NL=%d, want 59", cprNL(0))
	}
}

func TestKnownEuropeanCPRPair(t *testing.T) {
	lat, lon, ok := decodeCPRAirborne(93000, 51372, 74158, 50194, true)
	if !ok {
		t.Fatal("expected a global CPR fix")
	}
	if math.Abs(lat-52.26578) > 0.001 || math.Abs(lon-3.93891) > 0.001 {
		t.Fatalf("got %.5f %.5f", lat, lon)
	}
	lat, lon, ok = decodeCPRAirborne(93000, 51372, 74158, 50194, false)
	if !ok {
		t.Fatal("expected an even-frame CPR fix")
	}
	if math.Abs(lat-52.25720) > 0.001 || math.Abs(lon-3.91937) > 0.001 {
		t.Fatalf("even frame got %.5f %.5f", lat, lon)
	}
}

func TestCPRSurfaceUsesReceiverLongitude(t *testing.T) {
	lat, lon := 40.489, -3.567
	evenLat, evenLon := encodeSurface(lat, lon, false)
	oddLat, oddLon := encodeSurface(lat, lon, true)
	gotLat, gotLon, ok := decodeCPRSurface(evenLat, evenLon, oddLat, oddLon, true, lon)
	if !ok {
		t.Fatal("surface CPR failed")
	}
	if math.Abs(gotLat-lat) > 0.002 || math.Abs(wrap180(gotLon-lon)) > 0.002 {
		t.Fatalf("surface CPR mismatch: got %.5f %.5f", gotLat, gotLon)
	}
}

func TestSurfacePositionUsesSouthernReceiverReference(t *testing.T) {
	lat, lon := -33.8688, 151.2093
	evenLat, evenLon := encodeSurface(lat, lon, false)
	track := &trackState{refLat: lat, refLon: lon, hasRef: true}
	a := &Aircraft{}
	track.applyCPR(a, evenLat, evenLon, false, true, time.Now())
	if a.Latitude == nil || a.Longitude == nil || math.Abs(*a.Latitude-lat) > .002 || math.Abs(wrap180(*a.Longitude-lon)) > .002 || !a.OnGround {
		t.Fatalf("southern surface position not decoded: %+v", a)
	}
}

func encodeSurface(lat, lon float64, odd bool) (int, int) {
	dlat := 90.0 / 60.0
	if odd {
		dlat = 90.0 / 59.0
	}
	yz := int(math.Floor(cprModFloat(lat, dlat)/dlat*131072+0.5)) % 131072
	if yz < 0 {
		yz += 131072
	}
	ni := cprN(lat, odd)
	dlon := 90.0 / float64(ni)
	xz := int(math.Floor(cprModFloat(lon, dlon)/dlon*131072+0.5)) % 131072
	if xz < 0 {
		xz += 131072
	}
	return yz, xz
}
