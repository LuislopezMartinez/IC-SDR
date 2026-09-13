package screens

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebConfigStoresVerifierNotPassword(t *testing.T) {
	config := webConfig{Enabled: true, Port: 8080, QRInternet: true}
	if err := config.setPassword("larga-contraseña-de-prueba"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "web.json")
	if err := saveWebConfig(path, config); err != nil {
		t.Fatal(err)
	}
	loaded := loadWebConfig(path)
	if !loaded.Enabled || loaded.Port != 8080 || !loaded.QRInternet {
		t.Fatalf("settings not restored: %+v", loaded)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "larga-contraseña-de-prueba") {
		t.Fatal("plain password stored")
	}
	salt, hash, err := loaded.credential()
	if err != nil {
		t.Fatal(err)
	}
	derived, err := pbkdf2.Key(sha256.New, "larga-contraseña-de-prueba", salt, 60000, 32)
	if err != nil || subtle.ConstantTimeCompare(derived, hash) != 1 {
		t.Fatal("password verifier mismatch")
	}
}

func TestWebConfigRequiresStrongPassword(t *testing.T) {
	config := webConfig{Port: 8080}
	if err := config.setPassword("short"); err == nil {
		t.Fatal("accepted short password")
	}
	if _, _, err := config.credential(); err == nil {
		t.Fatal("accepted missing password")
	}
}

func TestWebConfigStoresSeparateControlVerifier(t *testing.T) {
	config := webConfig{Port: 8080}
	if err := config.setPassword("clave-espectador-123"); err != nil {
		t.Fatal(err)
	}
	if err := config.setControlPassword("clave-control-456"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "web.json")
	if err := saveWebConfig(path, config); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "clave-control-456") {
		t.Fatal("control password stored in plain text")
	}
	salt, hash, err := loadWebConfig(path).controlCredential()
	if err != nil {
		t.Fatal(err)
	}
	derived, err := pbkdf2.Key(sha256.New, "clave-control-456", salt, 60000, 32)
	if err != nil || subtle.ConstantTimeCompare(derived, hash) != 1 {
		t.Fatal("control verifier mismatch")
	}
}
