package contestlog

import "strings"

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
	}
	return issues
}
