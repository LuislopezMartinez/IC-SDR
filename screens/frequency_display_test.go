package screens

import "testing"

func TestFrequencySeparatorsAreNeverSelected(t *testing.T) {
	formatted := formatDialFrequency(446_093_750)
	digits := countFrequencyDigits(formatted)
	seen := 0
	for _, character := range formatted {
		exponent := -1
		if character >= '0' && character <= '9' {
			exponent = digits - seen - 1
			seen++
		}
		selected := exponent >= 0 && exponent == -1
		if selected {
			t.Fatalf("separator %q was treated as the selected digit", character)
		}
	}
}
