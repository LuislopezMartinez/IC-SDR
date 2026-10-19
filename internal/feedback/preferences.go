package feedback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Preferences contains only the contact values the user asked IC-SDR to
// remember. It deliberately excludes reports, technical data and radio state.
type Preferences struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserDataPath keeps personal feedback details outside the portable DATA
// directory, alongside (but separate from) QRZ's local browser profile.
func UserDataPath(parts ...string) string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(append([]string{base, "IC-SDR", "Feedback"}, parts...)...)
}

func LoadPreferences(path string) (Preferences, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Preferences{}, err
	}
	var preferences Preferences
	if err := json.Unmarshal(data, &preferences); err != nil {
		return Preferences{}, err
	}
	preferences.Name = strings.TrimSpace(preferences.Name)
	preferences.Email = strings.TrimSpace(preferences.Email)
	return preferences, nil
}

func SavePreferences(path string, preferences Preferences) error {
	preferences.Name = strings.TrimSpace(preferences.Name)
	preferences.Email = strings.TrimSpace(preferences.Email)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(preferences, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
