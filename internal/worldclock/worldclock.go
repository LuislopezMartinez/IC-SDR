// Package worldclock stores the locations shown by the international clock
// tool and provides a small, deterministic offline place resolver.
package worldclock

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-zero/internal/localtime"
)

type Location struct {
	City    string  `json:"city"`
	Country string  `json:"country,omitempty"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Zone    string  `json:"zone,omitempty"`
}

type Config struct {
	Version   int        `json:"version"`
	Locations []Location `json:"locations"`
}

type City struct {
	Name, Country string
	Lat, Lon      float64
}

// NearestCityMaxKM is deliberately conservative. Beyond this distance a
// major-city label would look authoritative while actually describing a
// different place (especially at sea and in sparsely populated regions).
const NearestCityMaxKM = 250.0

var cities = []City{
	{"Madrid", "España", 40.4168, -3.7038}, {"Barcelona", "España", 41.3874, 2.1686}, {"Valencia", "España", 39.4699, -0.3763}, {"Sevilla", "España", 37.3891, -5.9845}, {"Bilbao", "España", 43.2630, -2.9350}, {"Lisboa", "Portugal", 38.7223, -9.1393}, {"Oporto", "Portugal", 41.1579, -8.6291},
	{"Londres", "Reino Unido", 51.5074, -0.1278}, {"Dublín", "Irlanda", 53.3498, -6.2603}, {"París", "Francia", 48.8566, 2.3522}, {"Bruselas", "Bélgica", 50.8503, 4.3517}, {"Ámsterdam", "Países Bajos", 52.3676, 4.9041}, {"Luxemburgo", "Luxemburgo", 49.6116, 6.1319},
	{"Berlín", "Alemania", 52.5200, 13.4050}, {"Hamburgo", "Alemania", 53.5511, 9.9937}, {"Múnich", "Alemania", 48.1351, 11.5820}, {"Zúrich", "Suiza", 47.3769, 8.5417}, {"Ginebra", "Suiza", 46.2044, 6.1432}, {"Viena", "Austria", 48.2082, 16.3738}, {"Roma", "Italia", 41.9028, 12.4964}, {"Milán", "Italia", 45.4642, 9.1900},
	{"Copenhague", "Dinamarca", 55.6761, 12.5683}, {"Oslo", "Noruega", 59.9139, 10.7522}, {"Estocolmo", "Suecia", 59.3293, 18.0686}, {"Helsinki", "Finlandia", 60.1699, 24.9384}, {"Reikiavik", "Islandia", 64.1466, -21.9426}, {"Varsovia", "Polonia", 52.2297, 21.0122}, {"Praga", "Chequia", 50.0755, 14.4378}, {"Budapest", "Hungría", 47.4979, 19.0402}, {"Atenas", "Grecia", 37.9838, 23.7275}, {"Bucarest", "Rumanía", 44.4268, 26.1025}, {"Sofía", "Bulgaria", 42.6977, 23.3219}, {"Belgrado", "Serbia", 44.7866, 20.4489}, {"Zagreb", "Croacia", 45.8150, 15.9819}, {"Kiev", "Ucrania", 50.4501, 30.5234}, {"Moscú", "Rusia", 55.7558, 37.6173}, {"Estambul", "Turquía", 41.0082, 28.9784},
	{"Nueva York", "Estados Unidos", 40.7128, -74.0060}, {"Washington", "Estados Unidos", 38.9072, -77.0369}, {"Boston", "Estados Unidos", 42.3601, -71.0589}, {"Miami", "Estados Unidos", 25.7617, -80.1918}, {"Chicago", "Estados Unidos", 41.8781, -87.6298}, {"Denver", "Estados Unidos", 39.7392, -104.9903}, {"Los Ángeles", "Estados Unidos", 34.0522, -118.2437}, {"San Francisco", "Estados Unidos", 37.7749, -122.4194}, {"Seattle", "Estados Unidos", 47.6062, -122.3321}, {"Honolulu", "Estados Unidos", 21.3069, -157.8583}, {"Anchorage", "Estados Unidos", 61.2181, -149.9003},
	{"Ciudad de México", "México", 19.4326, -99.1332}, {"Toronto", "Canadá", 43.6532, -79.3832}, {"Vancouver", "Canadá", 49.2827, -123.1207}, {"Montreal", "Canadá", 45.5019, -73.5674}, {"Ottawa", "Canadá", 45.4215, -75.6972}, {"La Habana", "Cuba", 23.1136, -82.3666}, {"Panamá", "Panamá", 8.9824, -79.5199},
	{"Bogotá", "Colombia", 4.7110, -74.0721}, {"Caracas", "Venezuela", 10.4806, -66.9036}, {"Quito", "Ecuador", -0.1807, -78.4678}, {"Lima", "Perú", -12.0464, -77.0428}, {"La Paz", "Bolivia", -16.4897, -68.1193}, {"Santiago", "Chile", -33.4489, -70.6693}, {"Buenos Aires", "Argentina", -34.6037, -58.3816}, {"Montevideo", "Uruguay", -34.9011, -56.1645}, {"São Paulo", "Brasil", -23.5505, -46.6333}, {"Río de Janeiro", "Brasil", -22.9068, -43.1729}, {"Brasilia", "Brasil", -15.7939, -47.8828},
	{"Rabat", "Marruecos", 34.0209, -6.8416}, {"Casablanca", "Marruecos", 33.5731, -7.5898}, {"Argel", "Argelia", 36.7538, 3.0588}, {"Túnez", "Túnez", 36.8065, 10.1815}, {"El Cairo", "Egipto", 30.0444, 31.2357}, {"Dakar", "Senegal", 14.7167, -17.4677}, {"Lagos", "Nigeria", 6.5244, 3.3792}, {"Nairobi", "Kenia", -1.2921, 36.8219}, {"Adís Abeba", "Etiopía", 8.9806, 38.7578}, {"Ciudad del Cabo", "Sudáfrica", -33.9249, 18.4241}, {"Johannesburgo", "Sudáfrica", -26.2041, 28.0473},
	{"Jerusalén", "Israel", 31.7683, 35.2137}, {"Riad", "Arabia Saudí", 24.7136, 46.6753}, {"Dubái", "Emiratos Árabes Unidos", 25.2048, 55.2708}, {"Doha", "Catar", 25.2854, 51.5310}, {"Teherán", "Irán", 35.6892, 51.3890}, {"Delhi", "India", 28.6139, 77.2090}, {"Bombay", "India", 19.0760, 72.8777}, {"Karachi", "Pakistán", 24.8607, 67.0011},
	{"Pekín", "China", 39.9042, 116.4074}, {"Shanghái", "China", 31.2304, 121.4737}, {"Hong Kong", "China", 22.3193, 114.1694}, {"Taipéi", "Taiwán", 25.0330, 121.5654}, {"Seúl", "Corea del Sur", 37.5665, 126.9780}, {"Tokio", "Japón", 35.6762, 139.6503}, {"Osaka", "Japón", 34.6937, 135.5023}, {"Bangkok", "Tailandia", 13.7563, 100.5018}, {"Singapur", "Singapur", 1.3521, 103.8198}, {"Yakarta", "Indonesia", -6.2088, 106.8456}, {"Manila", "Filipinas", 14.5995, 120.9842},
	{"Perth", "Australia", -31.9523, 115.8613}, {"Adelaida", "Australia", -34.9285, 138.6007}, {"Melbourne", "Australia", -37.8136, 144.9631}, {"Sídney", "Australia", -33.8688, 151.2093}, {"Brisbane", "Australia", -27.4698, 153.0251}, {"Auckland", "Nueva Zelanda", -36.8509, 174.7645}, {"Wellington", "Nueva Zelanda", -41.2866, 174.7756},
}

func Defaults() Config {
	return Config{Version: 1, Locations: []Location{
		{City: "Madrid", Country: "España", Lat: 40.4168, Lon: -3.7038},
		{City: "Londres", Country: "Reino Unido", Lat: 51.5074, Lon: -0.1278},
		{City: "Nueva York", Country: "Estados Unidos", Lat: 40.7128, Lon: -74.0060},
		{City: "Tokio", Country: "Japón", Lat: 35.6762, Lon: 139.6503},
		{City: "Sídney", Country: "Australia", Lat: -33.8688, Lon: 151.2093},
	}}
}

func Load(path string) Config {
	config := Defaults()
	data, err := os.ReadFile(path)
	if err == nil {
		var saved Config
		if json.Unmarshal(data, &saved) == nil && saved.Version == 1 && len(saved.Locations) == 5 {
			config = saved
		}
	}
	for index := range config.Locations {
		config.Locations[index] = Normalize(config.Locations[index], time.Now().UTC())
	}
	return config
}

func Save(path string, config Config) error {
	config.Version = 1
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".world-clocks-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(append(data, '\n')); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func Normalize(location Location, now time.Time) Location {
	location.Lat = math.Max(-85.05112878, math.Min(85.05112878, location.Lat))
	location.Lon = location.Lon - math.Floor((location.Lon+180)/360)*360
	if info, ok := localtime.At(now, location.Lat, location.Lon); ok {
		location.Zone = info.Zone
	}
	if strings.TrimSpace(location.City) == "" {
		city, km := Resolve(location.Lat, location.Lon, location.Zone)
		if km <= NearestCityMaxKM {
			location.City, location.Country = city.Name, city.Country
		} else {
			location.City = strings.ReplaceAll(location.Zone, "_", " ")
			location.Country = ""
		}
	}
	return location
}

// Resolve chooses the nearest known major city, preferring candidates in the
// exact IANA timezone under the pin. It never changes the timezone itself.
func Resolve(lat, lon float64, zone string) (City, float64) {
	best, bestKM := City{}, math.MaxFloat64
	for _, city := range cities {
		if zone != "" {
			info, ok := localtime.At(time.Now().UTC(), city.Lat, city.Lon)
			if !ok || info.Zone != zone {
				continue
			}
		}
		if km := distanceKM(lat, lon, city.Lat, city.Lon); km < bestKM {
			best, bestKM = city, km
		}
	}
	if best.Name == "" && zone != "" {
		return Resolve(lat, lon, "")
	}
	return best, bestKM
}

func distanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	const radius = 6371.0088
	toRad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*toRad, (lon2-lon1)*toRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*toRad)*math.Cos(lat2*toRad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return radius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
