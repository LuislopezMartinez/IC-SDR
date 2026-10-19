package feedback

import (
	"path/filepath"
	"testing"
)

func TestPreferencesRoundTripOutsidePortableData(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	path := UserDataPath("contact.json")
	if want := filepath.Join(local, "IC-SDR", "Feedback", "contact.json"); path != want {
		t.Fatalf("UserDataPath() = %q, want %q", path, want)
	}
	if err := SavePreferences(path, Preferences{Name: "  EA4ABC  ", Email: " test@example.com "}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "EA4ABC" || got.Email != "test@example.com" {
		t.Fatalf("unexpected preferences: %+v", got)
	}
}
