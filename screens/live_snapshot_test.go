package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-zero/internal/aprs"
)

func TestReplaceLiveSnapshotUpdatesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "captures.json")
	if err := replaceLiveSnapshot(path, []byte(`[]`)); err != nil {
		t.Fatal(err)
	}
	if err := replaceLiveSnapshot(path, []byte(`[{"source":"ICSDR1"}]`)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `[{"source":"ICSDR1"}]` {
		t.Fatalf("stale snapshot: %s", data)
	}
}

func TestAPRSPacketSnapshotSupportsPacketsWithoutPosition(t *testing.T) {
	panel := &APRSPanel{snapshotPath: filepath.Join(t.TempDir(), "aprs.json")}
	packets := []aprs.Packet{{Received: time.Now(), Source: "EA1TEST", Type: "STATUS", Coordinates: "—", Summary: "en route"}}
	panel.writeSnapshot(packets)
	data, err := os.ReadFile(panel.snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []aprs.Packet
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].Source != "EA1TEST" {
		t.Fatalf("snapshot lost packet: %+v", decoded)
	}
}
