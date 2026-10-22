// Package feedback opens IC-SDR's public GitHub issue form with useful
// diagnostic information already included.
package feedback

import (
	"net/url"
	"runtime"
	"strings"
	"time"
)

const newIssueURL = "https://github.com/LuislopezMartinez/IC-SDR/issues/new"

type TechnicalInfo struct {
	Version, BuildDate, Revision string
	Windows, Architecture        string
	SDR, SDRStatus               string
	LocalTime, TimeZone          string
}

// IssueURL builds a GitHub issue URL without including personal information or
// the current tuned frequency. GitHub supplies the reporter's identity.
func IssueURL(info TechnicalInfo) string {
	values := url.Values{}
	values.Set("body", strings.Join([]string{
		"## Descripción",
		"",
		"Describe aquí el problema, la sugerencia o la pregunta.",
		"",
		"## Pasos para reproducirlo",
		"",
		"1. ",
		"2. ",
		"3. ",
		"",
		"## Información técnica de IC-SDR",
		"",
		"```text",
		TechnicalSummary(info),
		"```",
		"",
		"> Revisa el texto antes de publicar. No incluyas contraseñas, credenciales ni información privada.",
	}, "\n"))
	return newIssueURL + "?" + values.Encode()
}

func TechnicalSummary(info TechnicalInfo) string {
	lines := []string{
		"IC-SDR: " + valueOr(info.Version, "desarrollo"),
		"Compilación: " + valueOr(info.BuildDate, "desconocida"),
		"Revisión: " + valueOr(info.Revision, "desconocida"),
		"Sistema: " + valueOr(info.Windows, "Windows") + " · " + valueOr(info.Architecture, runtime.GOARCH),
		"SDR: " + valueOr(info.SDR, "No detectado") + " · " + valueOr(info.SDRStatus, "Desconectado"),
		"Fecha local: " + valueOr(info.LocalTime, time.Now().Format("02/01/2006 15:04")),
		"Zona horaria: " + valueOr(info.TimeZone, "desconocida"),
	}
	return strings.Join(lines, "\n")
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
