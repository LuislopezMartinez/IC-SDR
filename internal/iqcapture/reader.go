package iqcapture

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// WAVReader streams stereo PCM16 IQ WAV files written by Writer.
type WAVReader struct {
	file *os.File
	rate int
}

func Open(path string) (*WAVReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	header := make([]byte, 44)
	if _, err = io.ReadFull(file, header); err != nil {
		file.Close()
		return nil, fmt.Errorf("leer cabecera IQ WAV: %w", err)
	}
	if string(header[:4]) != "RIFF" || string(header[8:12]) != "WAVE" || string(header[12:16]) != "fmt " || string(header[36:40]) != "data" {
		file.Close()
		return nil, fmt.Errorf("formato IQ WAV no compatible")
	}
	if binary.LittleEndian.Uint16(header[20:22]) != 1 || binary.LittleEndian.Uint16(header[22:24]) != 2 || binary.LittleEndian.Uint16(header[34:36]) != 16 {
		file.Close()
		return nil, fmt.Errorf("IQ WAV debe ser PCM16 estéreo")
	}
	return &WAVReader{file: file, rate: int(binary.LittleEndian.Uint32(header[24:28]))}, nil
}

func (reader *WAVReader) SampleRate() int { return reader.rate }

func (reader *WAVReader) ReadFrames(maxFrames int) ([]float32, error) {
	if maxFrames < 1 {
		maxFrames = 4096
	}
	raw := make([]byte, maxFrames*4)
	n, err := reader.file.Read(raw)
	n -= n % 4
	if n == 0 {
		return nil, err
	}
	samples := make([]float32, n/2)
	for index := range samples {
		value := int16(binary.LittleEndian.Uint16(raw[index*2:]))
		samples[index] = float32(value) / 32768
	}
	if err == io.EOF {
		err = nil
	}
	return samples, err
}

func (reader *WAVReader) Close() error { return reader.file.Close() }
