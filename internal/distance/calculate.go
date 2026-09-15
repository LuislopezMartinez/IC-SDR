package distance

import "math"

const EarthRadius = 6371008.8
const rad = math.Pi / 180

type Point struct{ Lat, Lon float64 }

func Distance(a, b Point) float64 {
	dlat, dlon := (b.Lat-a.Lat)*rad, (b.Lon-a.Lon)*rad
	h := math.Pow(math.Sin(dlat/2), 2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Pow(math.Sin(dlon/2), 2)
	return 2 * EarthRadius * math.Asin(math.Sqrt(math.Min(1, math.Max(0, h))))
}
func Bearing(a, b Point) float64 {
	dl := (b.Lon - a.Lon) * rad
	return math.Mod(math.Atan2(math.Sin(dl)*math.Cos(b.Lat*rad), math.Cos(a.Lat*rad)*math.Sin(b.Lat*rad)-math.Sin(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Cos(dl))/rad+360, 360)
}
func Samples(a, b Point, n int) []Point {
	out := make([]Point, n)
	angle := Distance(a, b) / EarthRadius
	for i := range out {
		t := float64(i) / float64(n-1)
		if angle < 1e-9 || math.Abs(math.Sin(angle)) < 1e-9 {
			out[i] = Point{a.Lat + (b.Lat-a.Lat)*t, a.Lon + math.Remainder(b.Lon-a.Lon, 360)*t}
			continue
		}
		u, v := math.Sin((1-t)*angle)/math.Sin(angle), math.Sin(t*angle)/math.Sin(angle)
		x := u*math.Cos(a.Lat*rad)*math.Cos(a.Lon*rad) + v*math.Cos(b.Lat*rad)*math.Cos(b.Lon*rad)
		y := u*math.Cos(a.Lat*rad)*math.Sin(a.Lon*rad) + v*math.Cos(b.Lat*rad)*math.Sin(b.Lon*rad)
		z := u*math.Sin(a.Lat*rad) + v*math.Sin(b.Lat*rad)
		out[i] = Point{math.Atan2(z, math.Hypot(x, y)) / rad, math.Atan2(y, x) / rad}
	}
	out[0] = a
	out[n-1] = b
	return out
}
func Locator(p Point) string {
	x := math.Min(359.999999, math.Max(0, p.Lon+180))
	y := math.Min(179.999999, math.Max(0, p.Lat+90))
	return string([]byte{'A' + byte(x/20), 'A' + byte(y/10), '0' + byte(math.Mod(x, 20)/2), '0' + byte(math.Mod(y, 10)), 'a' + byte(math.Mod(x, 2)*12), 'a' + byte(math.Mod(y, 1)*24)})
}
func FSPL(meters, mhz float64) float64 {
	if meters <= 0 || mhz <= 0 {
		return math.NaN()
	}
	return 20*math.Log10(meters/1000) + 20*math.Log10(mhz) + 32.4478
}

type Link struct {
	Valid                      bool
	MinLOS, MinFresnel, Radius float64
	Clear                      bool
}

func Evaluate(e []*float64, d, mhz, ha, hb float64) Link {
	r := Link{MinLOS: math.Inf(1), MinFresnel: math.Inf(1)}
	if len(e) < 3 || d <= 0 || mhz <= 0 {
		return r
	}
	for _, v := range e {
		if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
			return r
		}
	}
	a, b := *e[0]+ha, *e[len(e)-1]+hb
	for i := 1; i < len(e)-1; i++ {
		t := float64(i) / float64(len(e)-1)
		d1, d2 := d*t, d*(1-t)
		bulge := d1 * d2 / (2 * EarthRadius * 4 / 3)
		f := math.Sqrt((299.792458 / mhz) * d1 * d2 / d)
		c := a + (b-a)*t - *e[i] - bulge
		r.MinLOS = math.Min(r.MinLOS, c)
		r.MinFresnel = math.Min(r.MinFresnel, c-.6*f)
	}
	r.Radius = math.Sqrt((299.792458 / mhz) * d / 4)
	r.Valid = true
	r.Clear = r.MinLOS > 0
	return r
}
