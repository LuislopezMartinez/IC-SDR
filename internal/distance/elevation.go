package distance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Mapzen terrain elevations are queried in one batch; absent coverage remains nil.
func Elevations(ctx context.Context, client *http.Client, points []Point) ([]*float64, error) {
	loc := make([]string, len(points))
	for i, p := range points {
		loc[i] = fmt.Sprintf("%.6f,%.6f", p.Lat, p.Lon)
	}
	q := url.Values{"locations": {strings.Join(loc, "|")}, "interpolation": {"bilinear"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.opentopodata.org/v1/mapzen?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "IC-SDR/0.6 (+https://github.com/LuislopezMartinez/IC-SDR)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("elevation HTTP %d", resp.StatusCode)
	}
	var data struct {
		Status  string
		Results []struct{ Elevation *float64 }
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
		return nil, err
	}
	if data.Status != "OK" || len(data.Results) != len(points) {
		return nil, fmt.Errorf("incomplete elevation response")
	}
	out := make([]*float64, len(points))
	for i, r := range data.Results {
		out[i] = r.Elevation
	}
	return out, nil
}
