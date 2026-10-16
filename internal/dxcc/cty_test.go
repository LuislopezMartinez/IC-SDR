package dxcc

import (
	"strings"
	"testing"
)

const sample = `Spain: 14: 37: EU: 40.37: 4.88: -1.0: EA:
    EA,EA6,=AM500ESP;
Fed. Rep. of Germany: 14: 28: EU: 51.00: -10.00: -1.0: DL:
    DA,DB,DC,DD,DE,DF,DG,DH,DI,DJ,DK,DL;
Canary Islands: 33: 36: AF: 28.32: 15.85: 0.0: EA8:
    EA8;
`

func TestLookupLongestPrefixAndExactException(t *testing.T) {
	db, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	for call, want := range map[string]string{
		"EA1ABC":       "Spain",
		"EA8XYZ":       "Canary Islands",
		"AM500ESP":     "Spain",
		"EA8/DL1ABC":   "Canary Islands",
		"DL1ABC/EA8":   "Canary Islands",
		"DL1ABC/P":     "Fed. Rep. of Germany",
		" dl1abc/qrp ": "Fed. Rep. of Germany",
	} {
		got, ok := db.Lookup(call)
		if !ok || got.Name != want {
			t.Errorf("Lookup(%q) = %q, %v; want %q", call, got.Name, ok, want)
		}
	}
}

func TestCTYLongitudeUsesMapConvention(t *testing.T) {
	db, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	entity, _ := db.Lookup("DL1ABC")
	if entity.Longitude != 10 {
		t.Fatalf("longitude = %v; want 10", entity.Longitude)
	}
}

func TestBundledDatabase(t *testing.T) {
	db, err := LoadBundled()
	if err != nil {
		t.Fatal(err)
	}
	for call, want := range map[string]string{
		"EA8ABC": "Canary Islands",
		"PY2BRU": "Brazil",
		"JA1ABC": "Japan",
	} {
		entity, ok := db.Lookup(call)
		if !ok || entity.Name != want {
			t.Errorf("Lookup(%q) = %q, %v; want %q", call, entity.Name, ok, want)
		}
	}
}
