package screens

import "testing"

func TestNFMCustomFilterRange(t *testing.T) {
	minimum, maximum, step := customFilterRange("NFM")
	if minimum != 500 || maximum != 25_000 || step != 500 {
		t.Fatalf("NFM custom range = %d..%d step %d; want 500..25000 step 500", minimum, maximum, step)
	}
}
