// Package contestlog stores contest QSOs locally and exports Cabrillo logs.
package contestlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-zero/internal/i18n"
)

const (
	SchemaVersion            = 1
	OceaniaDXSSB             = "OCEANIA-DX-SSB"
	WarningDuplicate Warning = "duplicate"
	WarningMode      Warning = "mode"
	WarningTime      Warning = "time"
	WarningBand      Warning = "band"
)

type Warning string

type Config struct {
	Power, OperatorCategory, Assisted, Band, Transmitter string
	SentValue, Location                                  string
}

type Session struct {
	Version             int        `json:"version"`
	Contest             string     `json:"contest"`
	ContestName         string     `json:"contestName,omitempty"`
	ContestMode         string     `json:"contestMode,omitempty"`
	ContestStart        time.Time  `json:"contestStart,omitempty"`
	ContestEnd          time.Time  `json:"contestEnd,omitempty"`
	DuplicateRule       string     `json:"duplicateRule,omitempty"`
	PointsPerQSO        int        `json:"pointsPerQso,omitempty"`
	SentExchange        string     `json:"sentExchange,omitempty"`
	SentValue           string     `json:"sentValue,omitempty"`
	ReceivedExchange    string     `json:"receivedExchange,omitempty"`
	AllowedBands        []string   `json:"allowedBands,omitempty"`
	StationCall         string     `json:"stationCall"`
	Operator            string     `json:"operator,omitempty"`
	CategoryPower       string     `json:"categoryPower"`
	CategoryOperator    string     `json:"categoryOperator"`
	CategoryAssisted    string     `json:"categoryAssisted"`
	CategoryBand        string     `json:"categoryBand"`
	CategoryTransmitter string     `json:"categoryTransmitter"`
	Location            string     `json:"location,omitempty"`
	Name                string     `json:"name,omitempty"`
	Email               string     `json:"email,omitempty"`
	Address             string     `json:"address,omitempty"`
	City                string     `json:"city,omitempty"`
	Country             string     `json:"country,omitempty"`
	Club                string     `json:"club,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	ClosedAt            *time.Time `json:"closedAt,omitempty"`
	NextSerial          int        `json:"nextSerial"`
	QSOs                []QSO      `json:"qsos"`
}

func (s *Session) Close(now time.Time) {
	closed := now.UTC()
	s.ClosedAt = &closed
}

func (s *Session) Reopen() { s.ClosedAt = nil }

func (s *Session) Closed() bool { return s != nil && s.ClosedAt != nil }

type QSO struct {
	ID               string    `json:"id"`
	Time             time.Time `json:"time"`
	FrequencyHz      int64     `json:"frequencyHz"`
	Band             string    `json:"band"`
	Mode             string    `json:"mode"`
	StationCall      string    `json:"stationCall"`
	WorkedCall       string    `json:"workedCall"`
	RSTSent          string    `json:"rstSent"`
	SerialSent       int       `json:"serialSent"`
	RSTReceived      string    `json:"rstReceived"`
	SerialReceived   int       `json:"serialReceived"`
	ExchangeSent     string    `json:"exchangeSent,omitempty"`
	ExchangeReceived string    `json:"exchangeReceived,omitempty"`
	Duplicate        bool      `json:"duplicate"`
	Warnings         []Warning `json:"warnings,omitempty"`
}

type Entry struct {
	Time             time.Time
	FrequencyHz      int64
	Mode             string
	WorkedCall       string
	RSTSent          string
	RSTReceived      string
	SerialReceived   int
	ExchangeReceived string
}

func NewOceaniaDXSSB(stationCall, operator, power string, now time.Time) (*Session, error) {
	return NewOceaniaDXSSBWithConfig(stationCall, operator, Config{Power: power}, now)
}

func NewOceaniaDXSSBWithConfig(stationCall, operator string, config Config, now time.Time) (*Session, error) {
	return NewSession(OceaniaDXSSB, stationCall, operator, config, now)
}

func NewSession(profileID, stationCall, operator string, config Config, now time.Time) (*Session, error) {
	stationCall = NormalizeCall(stationCall)
	if stationCall == "" {
		return nil, errors.New(i18n.Source("text.log0075"))
	}
	profile, ok := ProfileByID(profileID)
	if !ok {
		return nil, errors.New(i18n.Source("text.log0084"))
	}
	return NewSessionFromProfile(profile, stationCall, operator, config, now)
}

func NewSessionFromProfile(profile Profile, stationCall, operator string, config Config, now time.Time) (*Session, error) {
	stationCall = NormalizeCall(stationCall)
	if stationCall == "" {
		return nil, errors.New(i18n.Source("text.log0075"))
	}
	if err := ValidateProfile(profile); err != nil {
		return nil, err
	}
	if profile.SentExchange == "" {
		profile.SentExchange = "SERIAL"
	}
	if profile.ReceivedExchange == "" {
		profile.ReceivedExchange = "SERIAL"
	}
	if len(profile.AllowedBands) == 0 {
		profile.AllowedBands = []string{"160M", "80M", "40M", "20M", "15M", "10M"}
	}
	power := strings.ToUpper(strings.TrimSpace(config.Power))
	if power != "HIGH" && power != "LOW" && power != "QRP" {
		power = "LOW"
	}
	sentValue := strings.ToUpper(strings.TrimSpace(profile.SentValue))
	if profile.SentExchange == "FIXED" && strings.TrimSpace(config.SentValue) != "" {
		sentValue = strings.ToUpper(strings.TrimSpace(config.SentValue))
	}
	location := strings.ToUpper(strings.TrimSpace(config.Location))
	if location == "" {
		location = "DX"
	}
	isCQWW := profile.CabrilloName == CQWWSSB || profile.CabrilloName == CQWWCW
	operatorCategory := choice(config.OperatorCategory, []string{"SINGLE-OP", "MULTI-OP"}, "SINGLE-OP")
	transmitter := choice(config.Transmitter, []string{"ONE", "TWO", "UNLIMITED"}, "ONE")
	if isCQWW {
		operatorCategory, transmitter = "SINGLE-OP", "ONE"
	}
	return &Session{Version: SchemaVersion, Contest: profile.CabrilloName, ContestName: profile.Name, ContestMode: profile.Mode,
		ContestStart: profile.Start, ContestEnd: profile.End, DuplicateRule: profile.DuplicateRule, PointsPerQSO: profile.PointsPerQSO,
		SentExchange: profile.SentExchange, SentValue: sentValue, ReceivedExchange: profile.ReceivedExchange,
		AllowedBands: append([]string(nil), profile.AllowedBands...), StationCall: stationCall,
		Operator: NormalizeCall(operator), CategoryPower: power,
		CategoryOperator:    operatorCategory,
		CategoryAssisted:    choice(config.Assisted, []string{"NON-ASSISTED", "ASSISTED"}, "NON-ASSISTED"),
		CategoryBand:        choice(config.Band, append([]string{"ALL"}, profile.AllowedBands...), "ALL"),
		CategoryTransmitter: transmitter,
		Location:            location,
		CreatedAt:           now.UTC(), NextSerial: 1}, nil
}

func choice(value string, allowed []string, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

func (s *Session) Warnings(entry Entry) []Warning {
	if s == nil {
		return nil
	}
	warnings := s.entryWarnings(entry)
	call, band := NormalizeCall(entry.WorkedCall), BandForFrequency(entry.FrequencyHz)
	for _, q := range s.QSOs {
		if call != "" && band != "" && q.WorkedCall == call && (s.DuplicateRule == "CONTEST" || q.Band == band) {
			warnings = append(warnings, WarningDuplicate)
			break
		}
	}
	return warnings
}

func (s *Session) entryWarnings(entry Entry) []Warning {
	var warnings []Warning
	mode := strings.ToUpper(strings.TrimSpace(entry.Mode))
	validMode := (s.ContestMode == "CW" && mode == "CW") ||
		(s.ContestMode == "DIGI" && (mode == "DIGI" || mode == "DATA" || mode == "RTTY")) ||
		((s.ContestMode == "" || s.ContestMode == "SSB") && (mode == "USB" || mode == "LSB" || mode == "SSB" || mode == "PH"))
	if !validMode {
		warnings = append(warnings, WarningMode)
	}
	if !s.ContestStart.IsZero() {
		when := entry.Time.UTC()
		if when.Before(s.ContestStart) || !when.Before(s.ContestEnd) {
			warnings = append(warnings, WarningTime)
		}
	}
	band := BandForFrequency(entry.FrequencyHz)
	if band != "" && !s.BandAllowed(band) {
		warnings = append(warnings, WarningBand)
	}
	return warnings
}

func (s *Session) Add(entry Entry) (QSO, error) {
	if s == nil {
		return QSO{}, errors.New(i18n.Source("text.log0076"))
	}
	entry.WorkedCall = NormalizeCall(entry.WorkedCall)
	if entry.WorkedCall == "" {
		return QSO{}, errors.New(i18n.Source("text.log0077"))
	}
	received, receivedNumber, err := s.normalizeReceived(entry)
	if err != nil {
		return QSO{}, err
	}
	band := BandForFrequency(entry.FrequencyHz)
	if band == "" {
		return QSO{}, errors.New(i18n.Source("text.log0079"))
	}
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	if s.NextSerial < 1 {
		s.NextSerial = 1
	}
	q := QSO{ID: fmt.Sprintf("%d-%03d", entry.Time.UTC().UnixNano(), s.NextSerial), Time: entry.Time.UTC(),
		FrequencyHz: entry.FrequencyHz, Band: band, Mode: s.cabrilloMode(entry.Mode), StationCall: s.StationCall,
		WorkedCall: entry.WorkedCall, RSTSent: normalizeRST(entry.RSTSent), SerialSent: s.NextSerial, ExchangeSent: s.sentExchangeText(s.NextSerial),
		RSTReceived: normalizeRST(entry.RSTReceived), SerialReceived: receivedNumber, ExchangeReceived: received, Warnings: s.entryWarnings(entry)}
	for _, existing := range s.QSOs {
		if existing.WorkedCall == q.WorkedCall && (s.DuplicateRule == "CONTEST" || existing.Band == q.Band) {
			q.Duplicate = true
			break
		}
	}
	s.QSOs = append(s.QSOs, q)
	s.NextSerial++
	return q, nil
}

// Update replaces the editable data of one QSO while preserving its internal
// identity and the serial that was actually transmitted on air.
func (s *Session) Update(index int, entry Entry) (QSO, error) {
	if s == nil || index < 0 || index >= len(s.QSOs) {
		return QSO{}, errors.New(i18n.Source("text.log0080"))
	}
	entry.WorkedCall = NormalizeCall(entry.WorkedCall)
	if entry.WorkedCall == "" {
		return QSO{}, errors.New(i18n.Source("text.log0077"))
	}
	received, receivedNumber, err := s.normalizeReceived(entry)
	if err != nil {
		return QSO{}, err
	}
	band := BandForFrequency(entry.FrequencyHz)
	if band == "" {
		return QSO{}, errors.New(i18n.Source("text.log0081"))
	}
	if entry.Time.IsZero() {
		return QSO{}, errors.New(i18n.Source("text.log0082"))
	}
	previous := s.QSOs[index]
	updated := QSO{ID: previous.ID, Time: entry.Time.UTC(), FrequencyHz: entry.FrequencyHz, Band: band,
		Mode: s.cabrilloMode(entry.Mode), StationCall: s.StationCall, WorkedCall: entry.WorkedCall,
		RSTSent: normalizeRST(entry.RSTSent), SerialSent: previous.SerialSent,
		ExchangeSent: s.sentExchangeText(previous.SerialSent), RSTReceived: normalizeRST(entry.RSTReceived), SerialReceived: receivedNumber,
		ExchangeReceived: received, Warnings: s.entryWarnings(entry)}
	s.QSOs[index] = updated
	s.recomputeDuplicates()
	return s.QSOs[index], nil
}

func (s *Session) Delete(index int) (QSO, error) {
	if s == nil || index < 0 || index >= len(s.QSOs) {
		return QSO{}, errors.New(i18n.Source("text.log0080"))
	}
	removed := s.QSOs[index]
	s.QSOs = append(s.QSOs[:index], s.QSOs[index+1:]...)
	s.recomputeDuplicates()
	return removed, nil
}

func (s *Session) recomputeDuplicates() {
	seen := make(map[string]bool, len(s.QSOs))
	for index := range s.QSOs {
		key := s.QSOs[index].Band + "\x00" + s.QSOs[index].WorkedCall
		if s.DuplicateRule == "CONTEST" {
			key = s.QSOs[index].WorkedCall
		}
		s.QSOs[index].Duplicate = seen[key]
		seen[key] = true
	}
}

func NormalizeCall(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), ""))
}

func BandForFrequency(hz int64) string {
	switch {
	case hz >= 1_800_000 && hz <= 2_000_000:
		return "160M"
	case hz >= 3_500_000 && hz <= 4_000_000:
		return "80M"
	case hz >= 7_000_000 && hz <= 7_300_000:
		return "40M"
	case hz >= 14_000_000 && hz <= 14_350_000:
		return "20M"
	case hz >= 21_000_000 && hz <= 21_450_000:
		return "15M"
	case hz >= 28_000_000 && hz <= 29_700_000:
		return "10M"
	default:
		return ""
	}
}

func Save(path string, session *Session) error {
	if session == nil {
		return errors.New(i18n.Source("text.log0083"))
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return replaceFile(tmp, path)
}

func Load(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	if session.Version != SchemaVersion || strings.TrimSpace(session.Contest) == "" {
		return nil, errors.New(i18n.Source("text.log0084"))
	}
	if session.NextSerial < 1 {
		session.NextSerial = len(session.QSOs) + 1
	}
	if session.CategoryOperator == "" {
		session.CategoryOperator = "SINGLE-OP"
	}
	if session.CategoryAssisted == "" {
		session.CategoryAssisted = "NON-ASSISTED"
	}
	if session.CategoryBand == "" {
		session.CategoryBand = "ALL"
	}
	if session.CategoryTransmitter == "" {
		session.CategoryTransmitter = "ONE"
	}
	if session.Location == "" {
		session.Location = "DX"
	}
	if profile, ok := ProfileByID(session.Contest); ok {
		if session.ContestName == "" {
			session.ContestName = profile.Name
		}
		if session.ContestMode == "" {
			session.ContestMode = profile.Mode
		}
		if session.ContestStart.IsZero() {
			session.ContestStart, session.ContestEnd = profile.Start, profile.End
		}
		if session.DuplicateRule == "" {
			session.DuplicateRule = profile.DuplicateRule
		}
		if session.PointsPerQSO < 1 {
			session.PointsPerQSO = profile.PointsPerQSO
		}
	}
	if session.ContestName == "" {
		session.ContestName = session.Contest
	}
	if session.ContestMode == "" {
		session.ContestMode = "SSB"
	}
	if session.DuplicateRule == "" {
		session.DuplicateRule = "BAND"
	}
	if session.PointsPerQSO < 1 {
		session.PointsPerQSO = 1
	}
	if session.SentExchange == "" {
		session.SentExchange = "SERIAL"
	}
	if session.ReceivedExchange == "" {
		session.ReceivedExchange = "SERIAL"
	}
	if len(session.AllowedBands) == 0 {
		session.AllowedBands = []string{"160M", "80M", "40M", "20M", "15M", "10M"}
	}
	for index := range session.QSOs {
		if session.QSOs[index].ExchangeSent == "" {
			session.QSOs[index].ExchangeSent = serial(session.QSOs[index].SerialSent)
		}
		if session.QSOs[index].ExchangeReceived == "" {
			session.QSOs[index].ExchangeReceived = serial(session.QSOs[index].SerialReceived)
		}
	}
	return &session, nil
}

func ExportCabrillo(path string, session *Session) error {
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
	operator := session.Operator
	if operator == "" {
		operator = session.StationCall
	}
	headers := []string{
		"START-OF-LOG: 3.0", "CREATED-BY: IC-SDR", "CALLSIGN: " + session.StationCall,
		"LOCATION: " + defaultLocation(session.Location),
		"CONTEST: " + session.Contest, "CATEGORY-OPERATOR: " + session.CategoryOperator, "CATEGORY-ASSISTED: " + session.CategoryAssisted,
		"CATEGORY-BAND: " + session.CategoryBand, "CATEGORY-MODE: " + session.ContestMode, "CATEGORY-POWER: " + session.CategoryPower,
		"CATEGORY-STATION: FIXED", "CATEGORY-TRANSMITTER: " + session.CategoryTransmitter, "OPERATORS: " + operator,
	}
	for _, header := range []struct{ key, value string }{{"NAME", session.Name}, {"EMAIL", session.Email}, {"ADDRESS", session.Address}, {"ADDRESS-CITY", session.City}, {"ADDRESS-COUNTRY", session.Country}, {"CLUB", session.Club}} {
		if strings.TrimSpace(header.value) != "" {
			headers = append(headers, header.key+": "+strings.TrimSpace(header.value))
		}
	}
	headers = append(headers, fmt.Sprintf("CLAIMED-SCORE: %d", session.Score().Total))
	for _, line := range headers {
		fmt.Fprintln(w, line)
	}
	for _, q := range session.QSOs {
		minutes := q.FrequencyHz / 1000
		fmt.Fprintf(w, "QSO: %5d %-2s %s %s %-13s %3s %6s %-13s %3s %6s\n",
			minutes, q.Mode, q.Time.UTC().Format("2006-01-02"), q.Time.UTC().Format("1504"), q.StationCall,
			q.RSTSent, q.SentExchangeText(), q.WorkedCall, q.RSTReceived, q.ReceivedExchangeText())
	}
	fmt.Fprintln(w, "END-OF-LOG:")
	if err := w.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return replaceFile(tmp, path)
}

func defaultLocation(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "DX"
	}
	return value
}

// replaceFile also works on Windows, where os.Rename cannot replace an
// existing destination. The short-lived backup lets us restore the prior log
// if installing the new file fails.
func replaceFile(source, destination string) error {
	backup := destination + ".bak"
	_ = os.Remove(backup)
	hadDestination := true
	if err := os.Rename(destination, backup); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		hadDestination = false
	}
	if err := os.Rename(source, destination); err != nil {
		if hadDestination {
			_ = os.Rename(backup, destination)
		}
		return err
	}
	if hadDestination {
		_ = os.Remove(backup)
	}
	return nil
}

func normalizeRST(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "59"
	}
	return value
}

func normalizePhoneMode(string) string { return "PH" }
func serial(value int) string          { return fmt.Sprintf("%03d", value) }

func (s *Session) cabrilloMode(string) string {
	switch s.ContestMode {
	case "CW":
		return "CW"
	case "DIGI":
		return "DG"
	default:
		return "PH"
	}
}

func (s *Session) sentExchangeText(serialNumber int) string {
	if s.SentExchange == "FIXED" {
		return strings.ToUpper(strings.TrimSpace(s.SentValue))
	}
	return serial(serialNumber)
}

func (s *Session) normalizeReceived(entry Entry) (string, int, error) {
	value := strings.ToUpper(strings.TrimSpace(entry.ExchangeReceived))
	if value == "" && entry.SerialReceived > 0 {
		value = serial(entry.SerialReceived)
	}
	if value == "" {
		return "", 0, errors.New(i18n.Source("text.log0078"))
	}
	number := entry.SerialReceived
	if s.ReceivedExchange == "SERIAL" || s.ReceivedExchange == "NUMBER" || s.ReceivedExchange == "CQ_ZONE" || s.ReceivedExchange == "ITU_ZONE" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return "", 0, errors.New(i18n.Source("text.log0078"))
		}
		number = parsed
		if s.ReceivedExchange == "SERIAL" {
			value = serial(parsed)
		}
	}
	return value, number, nil
}

func (q QSO) SentExchangeText() string {
	if strings.TrimSpace(q.ExchangeSent) != "" {
		return q.ExchangeSent
	}
	return serial(q.SerialSent)
}

func (q QSO) ReceivedExchangeText() string {
	if strings.TrimSpace(q.ExchangeReceived) != "" {
		return q.ExchangeReceived
	}
	return serial(q.SerialReceived)
}

func (s *Session) BandAllowed(band string) bool {
	if s == nil || len(s.AllowedBands) == 0 {
		return true
	}
	for _, allowed := range s.AllowedBands {
		if allowed == band {
			return true
		}
	}
	return false
}
