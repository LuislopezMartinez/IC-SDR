package sdr

import "go-zero/internal/i18n"

// SoapySDR keeps modules registered for the lifetime of the process.
func moduleLoadSucceeded(message, path string) bool {
	return message == "" || message == path+i18n.Source("text.8612457b32e8")
}
