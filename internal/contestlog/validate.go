package contestlog

import (
	"strconv"
	"strings"
)

type ValidationIssue struct {
	Code     string
	QSOIndex int
}

func (s *Session) ValidateSubmission() []ValidationIssue {
	var issues []ValidationIssue
	add := func(code string) { issues = append(issues, ValidationIssue{Code: code, QSOIndex: -1}) }
	if s == nil {
		add("no_session")
		return issues
	}
	if NormalizeCall(s.StationCall) == "" {
		add("station_call")
	}
	if strings.TrimSpace(s.Name) == "" {
		add("name")
	}
	if strings.TrimSpace(s.Email) == "" || !strings.Contains(s.Email, "@") {
		add("email")
	}
	if strings.TrimSpace(s.Address) == "" {
		add("address")
	}
	if strings.TrimSpace(s.Country) == "" {
		add("country")
	}
	if isCQWWContest(s.Contest) {
		if s.CategoryOperator != "SINGLE-OP" || s.CategoryTransmitter != "ONE" {
			add("cqww_single_operator")
		}
		if !validCQZone(s.SentValue) {
			add("cq_zone_sent")
		}
		if strings.TrimSpace(s.Location) == "" {
			add("location")
		}
	}
	if len(s.QSOs) == 0 {
		add("no_qsos")
	}
	issues = append(issues, s.validateQSOs(true)...)
	return issues
}

// ValidateADIF checks only fields required to create useful ADIF records. It
// intentionally does not require Cabrillo participant/address information.
func (s *Session) ValidateADIF() []ValidationIssue {
	var issues []ValidationIssue
	add := func(code string) { issues = append(issues, ValidationIssue{Code: code, QSOIndex: -1}) }
	if s == nil {
		add("no_session")
		return issues
	}
	if NormalizeCall(s.StationCall) == "" {
		add("station_call")
	}
	if len(s.QSOs) == 0 {
		add("no_qsos")
	}
	issues = append(issues, s.validateQSOs(false)...)
	return issues
}

func (s *Session) validateQSOs(requireContestExchange bool) []ValidationIssue {
	var issues []ValidationIssue
	var previous int64
	for index, q := range s.QSOs {
		add := func(code string) { issues = append(issues, ValidationIssue{Code: code, QSOIndex: index}) }
		if q.Time.IsZero() {
			add("qso_time")
		}
		if q.FrequencyHz <= 0 {
			add("qso_frequency")
		}
		if NormalizeCall(q.WorkedCall) == "" {
			add("qso_call")
		}
		if strings.TrimSpace(q.RSTSent) == "" {
			add("qso_rst_sent")
		}
		if strings.TrimSpace(q.RSTReceived) == "" {
			add("qso_rst_received")
		}
		if requireContestExchange {
			if q.SerialSent < 1 {
				add("qso_serial_sent")
			}
			if strings.TrimSpace(q.SentExchangeText()) == "" {
				add("qso_exchange_sent")
			}
			if strings.TrimSpace(q.ReceivedExchangeText()) == "" {
				add("qso_exchange_received")
			}
		}
		if requireContestExchange && q.Time.UnixNano() < previous {
			add("qso_order")
		}
		previous = q.Time.UnixNano()
		if BandForFrequency(q.FrequencyHz) != q.Band {
			add("qso_band")
		}
		if requireContestExchange && isCQWWContest(s.Contest) && strings.TrimSpace(q.ReceivedExchangeText()) != "" && !validCQZone(q.ReceivedExchangeText()) {
			add("cq_zone_received")
		}
	}
	return issues
}

func isCQWWContest(contest string) bool { return contest == CQWWSSB || contest == CQWWCW }

func validCQZone(value string) bool {
	zone, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil && zone >= 1 && zone <= 40
}
