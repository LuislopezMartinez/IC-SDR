package contestlog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const GenericSSB = "GENERIC-SSB"

type Profile struct {
	ID, Name, CabrilloName, Mode string
	Start, End                   time.Time
	DuplicateRule                string
	PointsPerQSO                 int
	SentExchange                 string
	SentValue                    string
	ReceivedExchange             string
	AllowedBands                 []string
	OfficialScoring              bool
	BuiltIn                      bool
}

var profiles = []Profile{
	{ID: OceaniaDXSSB, Name: "Oceania DX SSB 2026", CabrilloName: OceaniaDXSSB, Mode: "SSB",
		Start: time.Date(2026, 10, 3, 6, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 4, 6, 0, 0, 0, time.UTC),
		DuplicateRule: "BAND", PointsPerQSO: 1, SentExchange: "SERIAL", ReceivedExchange: "SERIAL",
		AllowedBands: []string{"160M", "80M", "40M", "20M", "15M", "10M"}, OfficialScoring: true, BuiltIn: true},
	{ID: GenericSSB, Name: "Generic SSB", CabrilloName: GenericSSB, Mode: "SSB", DuplicateRule: "BAND", PointsPerQSO: 1,
		SentExchange: "SERIAL", ReceivedExchange: "SERIAL", AllowedBands: []string{"160M", "80M", "40M", "20M", "15M", "10M"}, BuiltIn: true},
}

func LoadProfiles(dir string) ([]Profile, error) {
	result := Profiles()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return result, err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var profile Profile
		if json.Unmarshal(data, &profile) == nil && ValidateProfile(profile) == nil {
			profile = normalizeProfile(profile)
			profile.BuiltIn = false
			result = append(result, profile)
		}
	}
	return result, nil
}

func SaveProfile(dir string, profile Profile) error {
	profile = normalizeProfile(profile)
	if err := ValidateProfile(profile); err != nil {
		return err
	}
	profile.BuiltIn = false
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, strings.ToLower(profile.ID)+".json"), append(data, '\n'), 0o644)
}

func normalizeProfile(profile Profile) Profile {
	if profile.SentExchange == "" {
		profile.SentExchange = "SERIAL"
	}
	if profile.ReceivedExchange == "" {
		profile.ReceivedExchange = "SERIAL"
	}
	if len(profile.AllowedBands) == 0 {
		profile.AllowedBands = []string{"160M", "80M", "40M", "20M", "15M", "10M"}
	}
	return profile
}

func DeleteProfile(dir, id string) error {
	if profile, ok := ProfileByID(id); ok && profile.BuiltIn {
		return errors.New("built-in profile")
	}
	return os.Remove(filepath.Join(dir, strings.ToLower(id)+".json"))
}

func ValidateProfile(profile Profile) error {
	profile.ID = strings.TrimSpace(profile.ID)
	if profile.ID == "" || strings.TrimSpace(profile.Name) == "" || strings.TrimSpace(profile.CabrilloName) == "" {
		return errors.New("name and Cabrillo identifier are required")
	}
	for _, value := range []string{profile.ID, profile.CabrilloName} {
		for _, r := range value {
			if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return errors.New("invalid profile identifier")
			}
		}
	}
	if profile.Mode != "SSB" && profile.Mode != "CW" && profile.Mode != "DIGI" {
		return errors.New("invalid mode")
	}
	if !profile.Start.IsZero() && (profile.End.IsZero() || !profile.End.After(profile.Start)) {
		return errors.New("invalid contest period")
	}
	if profile.DuplicateRule != "BAND" && profile.DuplicateRule != "CONTEST" {
		return errors.New("invalid duplicate rule")
	}
	if profile.PointsPerQSO < 1 {
		return errors.New("points must be positive")
	}
	if profile.SentExchange == "" {
		profile.SentExchange = "SERIAL"
	}
	if profile.ReceivedExchange == "" {
		profile.ReceivedExchange = "SERIAL"
	}
	if profile.SentExchange != "SERIAL" && profile.SentExchange != "FIXED" {
		return errors.New("invalid sent exchange")
	}
	if profile.SentExchange == "FIXED" && strings.TrimSpace(profile.SentValue) == "" {
		return errors.New("fixed sent exchange is required")
	}
	allowedReceived := map[string]bool{"SERIAL": true, "CQ_ZONE": true, "ITU_ZONE": true, "SECTION": true, "LOCATOR": true, "NUMBER": true, "TEXT": true}
	if !allowedReceived[profile.ReceivedExchange] {
		return errors.New("invalid received exchange")
	}
	allowedBands := map[string]bool{"160M": true, "80M": true, "40M": true, "20M": true, "15M": true, "10M": true}
	if len(profile.AllowedBands) == 0 {
		profile.AllowedBands = []string{"160M", "80M", "40M", "20M", "15M", "10M"}
	}
	for _, band := range profile.AllowedBands {
		if !allowedBands[band] {
			return errors.New("invalid band")
		}
	}
	return nil
}

func Profiles() []Profile { return append([]Profile(nil), profiles...) }

func ProfileByID(id string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}
