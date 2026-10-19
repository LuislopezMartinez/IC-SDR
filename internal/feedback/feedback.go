// Package feedback implements IC-SDR's standalone reports and suggestions window.
package feedback

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

type TechnicalInfo struct {
	Version, BuildDate, Revision string
	Windows, Architecture        string
	SDR, SDRStatus               string
	LocalTime, TimeZone          string
	Mode, Band, Frequency        string
}

type Report struct {
	Type            string `json:"type"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Comment         string `json:"comment"`
	IncludeReceiver bool   `json:"includeReceiver"`
	Technical       string `json:"technical"`
}

type FieldConfig struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Comment   string `json:"comment"`
	Technical string `json:"technical"`
}

type Config struct {
	FormAction string      `json:"form_action"`
	Fields     FieldConfig `json:"fields"`
}

type Result struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	parsed, err := url.Parse(config.FormAction)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "docs.google.com" || !strings.HasSuffix(parsed.Path, "/formResponse") {
		return errors.New("la dirección de Google Forms no está configurada")
	}
	for _, field := range []string{config.Fields.Type, config.Fields.Name, config.Fields.Email, config.Fields.Comment, config.Fields.Technical} {
		if !strings.HasPrefix(field, "entry.") {
			return errors.New("los identificadores de los campos de Google Forms no están configurados")
		}
	}
	return nil
}

func Submit(config Config, report Report) Result {
	if err := config.Validate(); err != nil {
		return Result{Message: err.Error()}
	}
	report.Comment = strings.TrimSpace(report.Comment)
	if report.Comment == "" {
		return Result{Message: "Escribe un comentario antes de enviar el reporte."}
	}
	if len(report.Comment) > 2000 {
		return Result{Message: "El comentario no puede superar los 2000 caracteres."}
	}
	values := url.Values{
		config.Fields.Type:      {strings.TrimSpace(report.Type)},
		config.Fields.Name:      {strings.TrimSpace(report.Name)},
		config.Fields.Email:     {strings.TrimSpace(report.Email)},
		config.Fields.Comment:   {report.Comment},
		config.Fields.Technical: {strings.TrimSpace(report.Technical)},
	}
	request, err := http.NewRequest(http.MethodPost, config.FormAction, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return Result{Message: "No se pudo preparar el envío."}
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "IC-SDR feedback/"+runtime.GOARCH)
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return Result{Message: "No se pudo conectar con Google Forms. Comprueba tu conexión."}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return Result{Message: fmt.Sprintf("Google Forms respondió con el estado %s.", response.Status)}
	}
	return Result{OK: true, Message: "¡Gracias! Tu reporte se ha enviado correctamente."}
}

func TechnicalSummary(info TechnicalInfo, includeReceiver bool) string {
	lines := []string{
		"IC-SDR: " + valueOr(info.Version, "desarrollo"),
		"Compilación: " + valueOr(info.BuildDate, "desconocida"),
		"Revisión: " + valueOr(info.Revision, "desconocida"),
		"Sistema: " + valueOr(info.Windows, "Windows") + " · " + valueOr(info.Architecture, runtime.GOARCH),
		"SDR: " + valueOr(info.SDR, "No detectado") + " · " + valueOr(info.SDRStatus, "Desconectado"),
		"Fecha local: " + valueOr(info.LocalTime, time.Now().Format("02/01/2006 15:04")),
		"Zona horaria: " + valueOr(info.TimeZone, "desconocida"),
	}
	if includeReceiver {
		lines = append(lines,
			"Modo: "+valueOr(info.Mode, "desconocido"),
			"Banda: "+valueOr(info.Band, "desconocida"),
			"Frecuencia: "+valueOr(info.Frequency, "desconocida"),
		)
	}
	return strings.Join(lines, "\n")
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
