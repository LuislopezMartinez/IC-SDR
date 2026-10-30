package screens

import "testing"

func TestNativeMemoryCatalogIntegrity(t *testing.T) {
	items := nativeMemoryCatalog()
	if len(items) < 35 {
		t.Fatalf("catalog too small: %d", len(items))
	}
	ids := map[string]bool{}
	kinds := map[string]int{}
	for _, item := range items {
		if item.ID == "" || ids[item.ID] {
			t.Fatalf("missing or duplicate id %q", item.ID)
		}
		ids[item.ID] = true
		if item.FrequencyHz <= 0 || item.Name == "" || item.Category == "" || item.Source == "" || item.Verified == "" {
			t.Fatalf("incomplete catalog item: %+v", item)
		}
		if item.Kind != "official" && item.Kind != "swl" {
			t.Fatalf("unexpected kind %q", item.Kind)
		}
		kinds[item.Kind]++
	}
	if kinds["official"] == 0 || kinds["swl"] == 0 {
		t.Fatalf("missing catalog sections: %#v", kinds)
	}
}

func TestCatalogMemoryProducesEditablePersonalCopy(t *testing.T) {
	m := &memoryManager{}
	item := nativeMemoryCatalog()[0]
	memory := m.catalogMemory(item)
	if memory.Name != item.Name || memory.Group != item.Category || !memory.ScanEnabled {
		t.Fatalf("bad personal copy: %+v", memory)
	}
	if memory.FrequencyHz != item.FrequencyHz || memory.Mode != item.Mode {
		t.Fatalf("tuning data changed: %+v", memory)
	}
}
