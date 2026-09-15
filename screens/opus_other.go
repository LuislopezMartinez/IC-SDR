//go:build !windows

package screens

import "go-zero/internal/i18n"

import "errors"

type opusEncoder struct{ preSkip uint16 }

func newOpusEncoder() (*opusEncoder, error) {
	return nil, errors.New(i18n.Source("text.7973bee0de21"))
}
func (e *opusEncoder) Encode([]float32) ([]byte, error) {
	return nil, errors.New(i18n.Source("text.0446c422e054"))
}
func (e *opusEncoder) Close() {}
