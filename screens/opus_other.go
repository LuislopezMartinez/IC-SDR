//go:build !windows

package screens

import "errors"

type opusEncoder struct{ preSkip uint16 }

func newOpusEncoder() (*opusEncoder, error) {
	return nil, errors.New("Opus disponible solo en Windows en esta distribución")
}
func (e *opusEncoder) Encode([]float32) ([]byte, error) { return nil, errors.New("Opus no disponible") }
func (e *opusEncoder) Close()                           {}
