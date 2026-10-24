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
	var previous int64
	for index, q := range s.QSOs {
		invalid := q.Time.IsZero() || q.FrequencyHz <= 0 || NormalizeCall(q.WorkedCall) == "" ||
			strings.TrimSpace(q.RSTSent) == "" || strings.TrimSpace(q.RSTReceived) == "" || q.SerialSent < 1 ||
			strings.TrimSpace(q.SentExchangeText()) == "" || strings.TrimSpace(q.ReceivedExchangeText()) == ""
		if invalid {
			issues = append(issues, ValidationIssue{Code: "qso_incomplete", QSOIndex: index})
		}
		if q.Time.UnixNano() < previous {
			issues = append(issues, ValidationIssue{Code: "qso_order", QSOIndex: index})
		}
		previous = q.Time.UnixNano()
		if BandForFrequency(q.FrequencyHz) != q.Band {
			issues = append(issues, ValidationIssue{Code: "qso_band", QSOIndex: index})
		}
		if isCQWWContest(s.Contest) && !validCQZone(q.ReceivedExchangeText()) {
			issues = append(issues, ValidationIssue{Code: "cq_zone_received", QSOIndex: index})
		}
	}
	return issues
}

func isCQWWContest(contest string) bool { return contest == CQWWSSB || contest == CQWWCW }

func validCQZone(value string) bool {
	zone, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil && zone >= 1 && zone <= 40
}
