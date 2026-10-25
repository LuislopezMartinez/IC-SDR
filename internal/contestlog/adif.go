package contestlog

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-zero/internal/buildinfo"
	"go-zero/internal/i18n"
)

const adifVersion = "3.1.7"

// ExportADIF writes every QSO in the session as an ADIF record. The output is
// replaced atomically so an interrupted export cannot damage an older file.
func ExportADIF(path string, session *Session) error {
	if session == nil || session.StationCall == "" {
		return errors.New(i18n.Source("text.log0076"))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(file)
	version := strings.TrimPrefix(buildinfo.DisplayVersion(), "v")
	for _, field := range []struct{ name, value string }{
		{"ADIF_VER", adifVersion}, {"PROGRAMID", "IC-SDR"}, {"PROGRAMVERSION", version},
		{"CREATED_TIMESTAMP", time.Now().UTC().Format("20060102 150405")},
	} {
		if err := writeADIFField(w, field.name, field.value); err != nil {
			file.Close()
			return err
		}
		if _, err := io.WriteString(w, " "); err != nil {
			file.Close()
			return err
		}
	}
	if _, err := io.WriteString(w, "<EOH>\n"); err != nil {
		file.Close()
		return err
	}
	for _, qso := range session.QSOs {
		if err := writeADIFQSO(w, session, qso); err != nil {
			file.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return replaceFile(tmp, path)
}

func writeADIFQSO(w io.Writer, session *Session, qso QSO) error {
	operator := session.Operator
	if operator == "" {
		operator = session.StationCall
	}
	fields := []struct{ name, value string }{
		{"CALL", qso.WorkedCall}, {"STATION_CALLSIGN", session.StationCall}, {"OPERATOR", operator},
		{"QSO_DATE", qso.Time.UTC().Format("20060102")}, {"TIME_ON", qso.Time.UTC().Format("150405")},
		{"BAND", strings.ToLower(qso.Band)}, {"FREQ", strconv.FormatFloat(float64(qso.FrequencyHz)/1e6, 'f', 6, 64)},
		{"MODE", adifMode(qso.Mode)}, {"RST_SENT", qso.RSTSent}, {"RST_RCVD", qso.RSTReceived}, {"CONTEST_ID", session.Contest},
	}
	if session.SentExchange == "SERIAL" {
		fields = append(fields, struct{ name, value string }{"STX", strconv.Itoa(qso.SerialSent)})
	} else if sent := qso.SentExchangeText(); sent != "" {
		fields = append(fields, struct{ name, value string }{"STX_STRING", sent})
	}
	received := qso.ReceivedExchangeText()
	if session.ReceivedExchange == "SERIAL" || session.ReceivedExchange == "NUMBER" {
		fields = append(fields, struct{ name, value string }{"SRX", strconv.Itoa(qso.SerialReceived)})
	} else if received != "" {
		fields = append(fields, struct{ name, value string }{"SRX_STRING", received})
	}
	if session.ReceivedExchange == "CQ_ZONE" {
		if zone, err := strconv.Atoi(received); err == nil && zone >= 1 && zone <= 40 {
			fields = append(fields, struct{ name, value string }{"CQZ", strconv.Itoa(zone)})
		}
	}
	if zone, err := strconv.Atoi(strings.TrimSpace(session.SentValue)); session.SentExchange == "FIXED" && err == nil && zone >= 1 && zone <= 40 {
		fields = append(fields, struct{ name, value string }{"MY_CQ_ZONE", strconv.Itoa(zone)})
	}
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		if err := writeADIFField(w, field.name, field.value); err != nil {
			return err
		}
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "<EOR>\n")
	return err
}

func writeADIFField(w io.Writer, name, value string) error {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")
	_, err := fmt.Fprintf(w, "<%s:%d>%s", name, len(value), value)
	return err
}

func adifMode(mode string) string {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "PH", "USB", "LSB", "SSB":
		return "SSB"
	case "DG", "DIGI":
		return "DATA"
	default:
		return strings.ToUpper(strings.TrimSpace(mode))
	}
}
