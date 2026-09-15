package screens

import (
	"go-zero/internal/i18n"

	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"go-zero/internal/resources"
)

type webConfig struct {
	Enabled             bool   `json:"enabled"`
	Port                int    `json:"port"`
	QRInternet          bool   `json:"qrInternet,omitempty"`
	PasswordSalt        string `json:"passwordSalt,omitempty"`
	PasswordHash        string `json:"passwordHash,omitempty"`
	ControlPasswordSalt string `json:"controlPasswordSalt,omitempty"`
	ControlPasswordHash string `json:"controlPasswordHash,omitempty"`
}

func defaultWebConfigPath() string { return resources.WritablePath("config", "web.json") }

func loadWebConfig(path string) webConfig {
	config := webConfig{Port: 8080}
	data, err := os.ReadFile(path)
	if err != nil {
		return config
	}
	if json.Unmarshal(data, &config) != nil {
		return webConfig{Port: 8080}
	}
	if config.Port < 1024 || config.Port > 65535 {
		config.Port = 8080
	}
	return config
}

func saveWebConfig(path string, config webConfig) error {
	if config.Port < 1024 || config.Port > 65535 {
		return errors.New(i18n.Source("text.eabf428e6e93"))
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".web-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	// Windows cannot replace an existing file with os.Rename. The stored value
	// is a password verifier, never the clear-text password.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func (config *webConfig) setPassword(password string) error {
	if len([]rune(password)) < 12 {
		return errors.New(i18n.Source("text.514ababdf164"))
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, 60000, 32)
	if err != nil {
		return err
	}
	config.PasswordSalt = hex.EncodeToString(salt)
	config.PasswordHash = hex.EncodeToString(hash)
	return nil
}

func (config *webConfig) setControlPassword(password string) error {
	if len([]rune(password)) < 12 {
		return errors.New(i18n.Source("text.e35da3befc73"))
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, 60000, 32)
	if err != nil {
		return err
	}
	config.ControlPasswordSalt = hex.EncodeToString(salt)
	config.ControlPasswordHash = hex.EncodeToString(hash)
	return nil
}

func (config webConfig) controlCredential() ([]byte, []byte, error) {
	salt, err := hex.DecodeString(config.ControlPasswordSalt)
	if err != nil {
		return nil, nil, errors.New(i18n.Source("text.ad4630151469"))
	}
	hash, err := hex.DecodeString(config.ControlPasswordHash)
	if err != nil || len(salt) != 16 || len(hash) != 32 {
		return nil, nil, errors.New(i18n.Source("text.ad4630151469"))
	}
	return salt, hash, nil
}

func (config webConfig) credential() ([]byte, []byte, error) {
	salt, err := hex.DecodeString(config.PasswordSalt)
	if err != nil {
		return nil, nil, errors.New(i18n.Source("text.69680b9bf1d5"))
	}
	hash, err := hex.DecodeString(config.PasswordHash)
	if err != nil || len(salt) != 16 || len(hash) != 32 {
		return nil, nil, errors.New(i18n.Source("text.69680b9bf1d5"))
	}
	return salt, hash, nil
}

func (screen *MainScreen) StartConfiguredWebServer() error {
	if !screen.webConfig.Enabled {
		return nil
	}
	salt, hash, err := screen.webConfig.credential()
	if err != nil {
		return err
	}
	address := ":" + strconv.Itoa(screen.webConfig.Port)
	_, err = StartWebServerWithHash(screen, address, salt, hash)
	return err
}

func (screen *MainScreen) applyWebConfig(config webConfig) error {
	if config.Enabled {
		if _, _, err := config.credential(); err != nil {
			return err
		}
	}
	if err := saveWebConfig(screen.webConfigPath, config); err != nil {
		return err
	}
	if screen.webServer != nil {
		_ = screen.webServer.Close()
		screen.webServer = nil
		screen.audioPlayer.SetWebAudioSink(nil)
	}
	screen.webConfig = config
	if err := screen.StartConfiguredWebServer(); err != nil {
		return fmt.Errorf(i18n.Source("text.d80f041277c6"), err)
	}
	return nil
}
