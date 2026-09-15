package screens

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type tileTestTransport func(*http.Request) (*http.Response, error)

func (f tileTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTileDownloadWaitsForOccupiedSlots(t *testing.T) {
	key := tileKey{z: 19, x: 987654, y: 123456}
	oldTokens, oldClient := tileFetchTokens, httpTiles
	tileFetchTokens = make(chan struct{}, 1)
	tileFetchTokens <- struct{}{}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	called := make(chan struct{}, 1)
	httpTiles = &http.Client{Transport: tileTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.Header.Get("User-Agent") != mapTileUA {
			t.Error("incorrect tile request")
		}
		called <- struct{}{}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(png)), Header: make(http.Header)}, nil
	})}
	done := make(chan struct{})
	defer func() {
		tileFetchTokens = oldTokens
		httpTiles = oldClient
		os.Remove(tilePath(key))
		tilesMu.Lock()
		delete(tileBytes, key)
		delete(tileRetryAfter, key)
		tilesMu.Unlock()
	}()
	go func() { fetchTile(key); close(done) }()
	select {
	case <-done:
		t.Fatal("occupied slots discarded a tile")
	case <-time.After(50 * time.Millisecond):
	}
	<-tileFetchTokens
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queued tile did not finish")
	}
	select {
	case <-called:
	default:
		t.Fatal("queued tile was not downloaded")
	}
	tilesMu.Lock()
	defer tilesMu.Unlock()
	if len(tileBytes[key]) == 0 || !tileRetryAfter[key].IsZero() {
		t.Fatal("queued tile marked as failed")
	}
}
func TestCachedTileDoesNotWaitForNetworkSlot(t *testing.T) {
	key := tileKey{z: 19, x: 987654, y: 123457}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	path := tilePath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, png, 0644); err != nil {
		t.Fatal(err)
	}
	old := tileFetchTokens
	tileFetchTokens = make(chan struct{}, 1)
	tileFetchTokens <- struct{}{}
	defer func() {
		tileFetchTokens = old
		os.Remove(path)
		tilesMu.Lock()
		delete(tileBytes, key)
		delete(tileRetryAfter, key)
		tilesMu.Unlock()
	}()
	done := make(chan struct{})
	go func() { fetchTile(key); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		<-tileFetchTokens
		<-done
		t.Fatal("cache waited for network")
	}
	tilesMu.Lock()
	defer tilesMu.Unlock()
	if len(tileBytes[key]) == 0 {
		t.Fatal("cached tile not loaded")
	}
}
