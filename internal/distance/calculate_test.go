package distance

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestGeographicCalculations(t *testing.T) {
	a, b := Point{0, 0}, Point{0, 1}
	if math.Abs(Distance(a, b)-111195.08) > 1 {
		t.Fatal("equatorial distance")
	}
	if Bearing(a, b) != 90 || Bearing(b, a) != 270 {
		t.Fatal("bearings")
	}
	if Distance(a, a) != 0 {
		t.Fatal("coincident pins")
	}
	p := Samples(Point{0, 179}, Point{0, -179}, 81)
	if math.Abs(math.Abs(p[40].Lon)-180) > 1e-6 || math.Abs(Distance(p[0], p[40])-Distance(p[40], p[80])) > 1 {
		t.Fatal("dateline sampling")
	}
	if Locator(Point{40.4168, -3.7038}) != "IN80dk" {
		t.Fatalf("locator %s", Locator(Point{40.4168, -3.7038}))
	}
	if math.Abs(FSPL(1000, 100)-72.4478) > 1e-6 {
		t.Fatal("FSPL units")
	}
}
func TestLinkTerrainAndMissingCoverage(t *testing.T) {
	zero, mountain := 0., 50.
	flat := []*float64{&zero, &zero, &zero}
	r := Evaluate(flat, 1000, 144, 10, 10)
	if !r.Valid || !r.Clear {
		t.Fatal("short flat link should be clear")
	}
	if r.MinFresnel >= r.MinLOS || r.Radius <= 0 {
		t.Fatal("Fresnel clearance")
	}
	r = Evaluate([]*float64{&zero, &mountain, &zero}, 1000, 144, 10, 10)
	if !r.Valid || r.Clear {
		t.Fatal("mountain not detected")
	}
	if Evaluate([]*float64{&zero, nil, &zero}, 1000, 144, 10, 10).Valid {
		t.Fatal("unknown terrain reported as valid")
	}
	if Evaluate(flat, 100000, 144, 10, 10).Clear {
		t.Fatal("Earth curvature ignored")
	}
}

type elevationTransport func(*http.Request) (*http.Response, error)

func (f elevationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestElevationRequestAndNullCoverage(t *testing.T) {
	c := &http.Client{Transport: elevationTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Query().Get("locations") != "40.000000,-3.000000|41.000000,-4.000000" || r.Header.Get("User-Agent") == "" {
			t.Error("incorrect elevation request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"OK","results":[{"elevation":657},{"elevation":null}]}`))}, nil
	})}
	e, err := Elevations(context.Background(), c, []Point{{40, -3}, {41, -4}})
	if err != nil || len(e) != 2 || *e[0] != 657 || e[1] != nil {
		t.Fatal("elevation/null coverage mishandled", err)
	}
}
